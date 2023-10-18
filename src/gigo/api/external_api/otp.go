package external_api

import (
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"time"
)

func (s *HTTPServer) GenerateUserOtpUri(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "generate-otp-uri-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GenerateUserOtpUri", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GenerateUserOtpUri(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GenerateUserOtpUri core failed", r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"generate-otp-uri",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) VerifyUserOtp(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "verify-user-otp-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "VerifyUserOtp", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "n/a",
			http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "VerifyUserOtp", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load otp code from body
	code, ok := s.loadValue(w, r, reqJson, "VerifyUserOtp", "otp_code", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
	if code == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "VerifyUserOtp", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, token, err := core.VerifyUserOtp(ctx, callingUser.(*models.User), s.tiDB, s.storageEngine, code.(string), network.GetRequestIP(r))
	if err != nil {
		// handle true failures
		if res == nil {
			// handle error internally
			s.handleError(w, "VerifyUserOtp core failed", r.URL.Path, "VerifyUserOtp", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId,
				http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
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
		"verify-user-otp",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "VerifyUserOtp", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}
