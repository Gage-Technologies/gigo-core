package external_api

import (
	"errors"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) Login(w http.ResponseWriter, r *http.Request) {
	// derive trace span from context for telem
	// span := trace.SpanFromContext(r.Context())
	// span.SetName("login http server")
	// defer span.End()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "login-http")
	defer parentSpan.End()

	// retrieve basic authentication information from request
	username, password, ok := r.BasicAuth()

	// return error if authentication information is missing
	if !ok {
		s.handleError(w, "authentication information missing", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "basic auth missing", nil)
		return
	}

	// retrieve IP address of caller
	ip := network.GetRequestIP(r)

	// format the redis key for failure checks
	failureCheckKey := fmt.Sprintf("%s:%s", "failedAttempts", ip)

	// Check failed attempts in Redis
	failedAttempts, err := s.rdb.Get(ctx, failureCheckKey).Int()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			s.handleError(w, "Unable to grab the failed attempts", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "Can't grab failed attempts", err)
			return
		}
		// set failed attempts to 0 if the key does not exist
		failedAttempts = 0
	}
	s.logger.Info("failed attempts: ", failedAttempts)

	// If there are more than 5 failed attempts, block the user
	if failedAttempts >= 5 {
		ttl, err := s.rdb.TTL(ctx, failureCheckKey).Result()
		if err != nil {
			s.handleError(w, "failed to retrieve ttl of failed login key", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "Too many failed attempts, please try again in 10 minutes", err)
			return
		}

		responseMessage := fmt.Sprintf("Too many failed attempts. %v left", ttl)
		s.handleError(w, "Too many failed attempts, please try again in 10 minutes", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, responseMessage, nil)
		return
	}

	// execute core function logic
	res, token, err := core.Login(ctx, s.tiDB, s.jetstreamClient, s.rdb, s.sf, s.storageEngine, s.domain, strings.ToLower(username), password, ip, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "login core failed", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	s.logger.Info("failed attempts token: ", token)

	// check if token was created
	if token != "" {
		// Reset failed attempts in Redis
		err = s.rdb.Del(ctx, failureCheckKey).Err()
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to delete the failed attempt redis key", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, "internal server error", err)
			// exit
			return
		}
		// conditionally set cookie with insecure settings
		if s.developmentMode {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Hour * 24 * 30),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   false,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		} else {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Hour * 24 * 30),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   true,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		}
	} else {
		s.logger.Info("failed attempts final wrong: ", failedAttempts)
		err = s.rdb.Incr(ctx, failureCheckKey).Err()
		if err != nil {
			fmt.Println("failed to create/increment cookie: ", err)
			s.handleError(w, "Failed to create/increment redis key", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "Failed to create/increment redis key", err)
			return
		}

		// set expiration time to 10 minutes
		err = s.rdb.Expire(ctx, failureCheckKey, 10*time.Minute).Err()
		if err != nil {
			fmt.Println("failed to set redis key expiration: ", err)
			s.handleError(w, "Failed to set redis key expiration", r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "Failed to set redis key expiration", err)
			return
		}

		responseMessage := fmt.Sprintf("%v attempts left", 5-failedAttempts)

		// return JSON response
		s.jsonResponse(r, w, map[string]interface{}{"message": responseMessage}, r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), username, "n/a", http.StatusOK)
		return
	}

	// register the login event
	parentSpan.AddEvent(
		"login",
		trace.WithAttributes(
			attribute.Bool("success", token != ""),
			attribute.String("ip", ip),
			attribute.String("username", username),
		),
	)

	// return JSON response
	s.jsonResponse(r, w, res, r.URL.Path, "Login", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), username, "n/a", http.StatusOK)
}

func (s *HTTPServer) Logout(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "logout-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "Logout", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// revoke cookie
	s.revokeCookie(w, network.GetRequestIP(r))

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	parentSpan.AddEvent(
		"logout",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return JSON response
	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "Logout", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) ValidateSession(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "validate-session-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ValidateSession", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	parentSpan.AddEvent(
		"login",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return JSON response
	s.jsonResponse(r, w, map[string]interface{}{"message": "valid"}, r.URL.Path, "ValidateSession", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) LoginWithGoogle(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "login-with-google-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "LoginWithGoogle", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	externalAuth, ok := s.loadValue(w, r, reqJson, "LoginWithGoogle", "external_auth", reflect.String, nil, false, "", "")
	if externalAuth == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "LoginWithGoogle", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// retrieve IP address of caller
	ip := network.GetRequestIP(r)

	// execute core function logic
	res, token, err := core.LoginWithGoogle(ctx, s.tiDB, s.jetstreamClient, s.rdb, s.sf, s.storageEngine, s.domain, externalAuth.(string), password.(string), ip, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "LoginWithGoogle core failed", r.URL.Path, "LoginWithGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// check if token was created
	if token != "" {
		// conditionally set cookie with insecure settings
		if s.developmentMode {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Hour * 24 * 30),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   false,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		} else {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Hour * 24 * 30),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   true,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		}
	}

	parentSpan.AddEvent(
		"login-with-google",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return JSON response
	s.jsonResponse(r, w, res, r.URL.Path, "LoginWithGoogle", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "n/a", http.StatusOK)
}

func (s *HTTPServer) LoginWithGithub(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "login-with-github-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "LoginWithGithub", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	externalAuth, ok := s.loadValue(w, r, reqJson, "LoginWithGithub", "external_auth", reflect.String, nil, false, "", "")
	if externalAuth == nil || !ok {
		return
	}

	// retrieve IP address of caller
	ip := network.GetRequestIP(r)

	// execute core function logic
	res, token, err := core.LoginWithGithub(ctx, s.tiDB, s.storageEngine, externalAuth.(string), ip, s.githubSecret)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "LoginWithGithub core failed", r.URL.Path, "LoginWithGithub", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// check if token was created
	if token != "" {
		// conditionally set cookie with insecure settings
		if s.developmentMode {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Minute * 5),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   false,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		} else {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Minute * 5),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   true,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		}
	}

	parentSpan.AddEvent(
		"login-with-github",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return JSON response
	s.jsonResponse(r, w, res, r.URL.Path, "LoginWithGithub", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "n/a", http.StatusOK)
}

func (s *HTTPServer) ConfirmGithubLogin(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "login-with-github-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ConfirmGithubLogin", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1",
			http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ConfirmGithubLogin", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "ConfirmGithubLogin", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// retrieve IP address of caller
	ip := network.GetRequestIP(r)

	// execute core function logic
	res, token, err := core.ConfirmGithubLogin(ctx, s.tiDB, s.rdb, s.jetstreamClient, s.sf, s.storageEngine, callingUser.(*models.User), password.(string), ip, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ConfirmGithubLogin core failed", r.URL.Path, "ConfirmGithubLogin", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// check if token was created
	if token != "" {
		// conditionally set cookie with insecure settings
		if s.developmentMode {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Hour * 24 * 30),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   false,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		} else {
			// set cookie in response if the token was created
			http.SetCookie(w, &http.Cookie{
				Name:     "gigoAuthToken",
				Value:    token,
				Expires:  time.Now().Add(time.Hour * 24 * 30),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   true,
				Domain:   fmt.Sprintf(".%s", s.domain),
			})
		}
	}

	parentSpan.AddEvent(
		"confirm-github-login",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return JSON response
	s.jsonResponse(r, w, res, r.URL.Path, "ConfirmGithubLogin", r.Method, r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r), "", "n/a", http.StatusOK)
}

func (s *HTTPServer) ReferralUserInfo(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "referral-user-info-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ReferralUserInfo", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new password from body
	name, ok := s.loadValue(w, r, reqJson, "ReferralUserInfo", "user_name", reflect.String, nil, false, "", "")
	if name == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ReferralUserInfo", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}
	// execute core function logic
	res, err := core.ReferralUserInfo(ctx, s.tiDB, name.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ReferralUserInfo failed", r.URL.Path, "ReferralUserInfo", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"referral-user-info",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", ""),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ReferralUserInfo", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}
