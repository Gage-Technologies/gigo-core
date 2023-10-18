package external_api

import (
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"strconv"
)

func (s *HTTPServer) RecordImplicitAction(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "record-implicit-action-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "RecordImplicitAction", false, callingUser.(*models.User).UserName, -1)
	if reqJson == nil {
		return
	}

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "RecordImplicitAction", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	rawPostId, ok := s.loadValue(w, r, reqJson, "RecordImplicitAction", "post_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if rawPostId == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(rawPostId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", rawPostId.(string)), r.URL.Path, "RecordImplicitAction", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	rawAction, ok := s.loadValue(w, r, reqJson, "RecordImplicitAction", "action", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if rawAction == nil || !ok {
		return
	}

	rawSessionId, ok := s.loadValue(w, r, reqJson, "RecordImplicitAction", "session_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if rawSessionId == nil || !ok {
		return
	}

	sessionId, err := uuid.Parse(rawSessionId.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse session id string to UUID: %s", rawSessionId.(string)), r.URL.Path, "RecordImplicitAction", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "RecordImplicitAction", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	err = core.RecordImplicitAction(ctx, s.tiDB, s.sf, callingUser.(*models.User), postId, sessionId, models.ImplicitAction(rawAction.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "RecordImplicitAction core failed", r.URL.Path, "RecordImplicitAction", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"record-implicit-action",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, nil, r.URL.Path, "RecordImplicitAction", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
