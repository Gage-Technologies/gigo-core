package external_api

import (
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"strconv"
)

func (s *HTTPServer) PastWeekActive(w http.ResponseWriter, r *http.Request) {

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "past-week-active-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "PastWeekActive", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "PastWeekActive", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "PastWeekActive", "skip", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "PastWeekActive", "limit", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PastWeekActive", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PastWeekActive(ctx, callingUser.(*models.User), s.tiDB, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "PastWeekActive core failed", r.URL.Path, "PastWeekActive", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// register the login event
	parentSpan.AddEvent(
		"past-week-active",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PastWeekActive", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) MostChallengingActive(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "most-challenging-active-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "MostChallengingActive", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "MostChallengingActive", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "MostChallengingActive", "skip", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "MostChallengingActive", "limit", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "MostChallengingActive", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.MostChallengingActive(ctx, callingUser.(*models.User), s.tiDB, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "MostChallengingActive core failed", r.URL.Path, "MostChallengingActive", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// register the login event
	parentSpan.AddEvent(
		"most-challenging-active",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "MostChallengingActive", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DontGiveUpActive(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "cont-give-up-active-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DontGiveUpActive", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DontGiveUpActive", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "DontGiveUpActive", "skip", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "DontGiveUpActive", "limit", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DontGiveUpActive", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DontGiveUpActive(ctx, callingUser.(*models.User), s.tiDB, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "DontGiveUpActive core failed", r.URL.Path, "DontGiveUpActive", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// register the login event
	parentSpan.AddEvent(
		"dont-give-up-active",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DontGiveUpActive", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
