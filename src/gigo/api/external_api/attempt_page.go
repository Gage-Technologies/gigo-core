package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) ProjectAttemptInformation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "project-attempt-information-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ProjectAttemptInformation", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "ProjectAttemptInformation", "attempt_id", reflect.String, nil, false, userName, callingId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	attemptId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectAttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ProjectAttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ProjectAttemptInformation(ctx, s.tiDB, s.vscClient, attemptId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "ProjectAttemptInformation core failed", r.URL.Path, "ProjectAttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"project-attempt-information",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ProjectAttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) AttemptInformation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "attempt-information-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "AttemptInformation", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "AttemptInformation", "attempt_id", reflect.String, nil, false, userName, callingId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	attemptId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "AttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "AttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.AttemptInformation(ctx, s.tiDB, s.vscClient, attemptId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "AttemptInformation core failed", r.URL.Path, "AttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"attempt-information",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "AttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetAttemptCode(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-attempt-code-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetAttemptCode", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetAttemptCode", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	repo, ok := s.loadValue(w, r, reqJson, "GetAttemptCode", "repo", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if repo == nil || !ok {
		return
	}

	// attempt to load parameter from body
	ref, ok := s.loadValue(w, r, reqJson, "GetAttemptCode", "ref", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if ref == nil || !ok {
		return
	}

	// attempt to load parameter from body
	filepath, ok := s.loadValue(w, r, reqJson, "GetAttemptCode", "filepath", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if filepath == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetAttemptCode", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetAttemptCode(ctx, s.vscClient, callingUser.(*models.User), repo.(string), ref.(string), filepath.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GetAttemptCode core failed", r.URL.Path, "GetAttemptCode", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-attempt-code",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetAttemptCode", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) EditDescription(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "edit-description-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "EditDescription", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "EditDescription", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "AttemptInformation", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	attemptId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "AttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	project, ok := s.loadValue(w, r, reqJson, "EditDescription", "project", reflect.Bool, nil, false, callingUser.(*models.User).UserName, callingId)
	if project == nil || !ok {
		return
	}

	// attempt to load parameter from body
	description, ok := s.loadValue(w, r, reqJson, "EditDescription", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "EditDescription", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.EditDescription(ctx, attemptId, s.meili, project.(bool), description.(string), s.tiDB)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "EditDescription core failed", r.URL.Path, "EditDescription", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"edit-description",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "EditDescription", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
