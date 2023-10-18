package external_api

import (
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/gigo/api/external_api/core"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
)

func (s *HTTPServer) ActiveProjectsHome(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "active-projects-home-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.jsonResponse(r, w, map[string]interface{}{"response": "not logged in"}, r.URL.Path, "ActiveProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return

		// s.handleError(w, "calling user missing from context", r.URL.Path, "ActiveProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID),
		//	network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		// return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ActiveProjectsHome", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ActiveProjectsHome(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "RecommendedProjectsHome failed", r.URL.Path, "RecommendedProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"active-projects-home",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ActiveProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) RecommendedProjectsHome(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "recommended-projects-home-http")
	defer parentSpan.End()

	// retrieve calling user from context
	var callingUser *models.User
	callingUserI := r.Context().Value(CtxKeyUser)
	if callingUserI != nil {
		callingUser = callingUserI.(*models.User)
	}

	// parse and validate request body
	var req core.ReccommendedProjectsHomeRequest
	if !s.validateRequest(w, r, callingUser, r.Body, &req) {
		return
	}

	// check if this is a test
	if req.Test {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "RecommendedProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), network.GetRequestIP(r), network.GetRequestIP(r), http.StatusOK)
		return
	}

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		// execute core function logic
		res, err := core.RecommendedProjectsHome(ctx, nil, s.tiDB, s.logger, &req)
		if err != nil {
			// select error message dependent on if there was one returned from the function
			responseMessage := selectErrorResponse("internal server error occurred", res)
			// handle error internally
			s.handleError(w, "RecommendedProjectsHome failed", r.URL.Path, "RecommendedProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusInternalServerError, responseMessage, err)
			// exit
			return
		}

		parentSpan.AddEvent(
			"recommended-projects-home",
			trace.WithAttributes(
				attribute.Bool("success", true),
				attribute.String("ip", network.GetRequestIP(r)),
			),
		)

		// return response
		s.jsonResponse(r, w, res, r.URL.Path, "RecommendedProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	callingId := strconv.FormatInt(callingUser.ID, 10)

	// execute core function logic
	res, err := core.RecommendedProjectsHome(ctx, callingUser, s.tiDB, s.logger, &req)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "RecommendedProjectsHome failed", r.URL.Path, "RecommendedProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"recommended-projects-home",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "RecommendedProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) TopRecommendations(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "top-recommendations-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.jsonResponse(r, w, map[string]interface{}{"response": "not logged in"}, r.URL.Path, "ActiveProjectsHome", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "TopRecommendations", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.TopRecommendations(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "TopRecommendations failed", r.URL.Path, "TopRecommendations", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"top-recommendations-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "TopRecommendations", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
