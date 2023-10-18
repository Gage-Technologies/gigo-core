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

func (s *HTTPServer) ProjectInformation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "project-information-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUserI := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	var callingUser *models.User
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUserI != nil {
		callingUser = callingUserI.(*models.User)
		userName = callingUser.UserName
		userId = fmt.Sprintf("%d", callingUser.ID)
		userIdInt = callingUser.ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ProjectInformation", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "ProjectInformation", "post_id", reflect.String, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ProjectInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ProjectInformation(ctx, s.tiDB, s.vscClient, callingUser, postId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "ProjectInformation core failed", r.URL.Path, "ProjectInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"project-information",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ProjectInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) ProjectAttempts(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "project-attempt-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ProjectAttempts", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "ProjectAttempts", "project_id", reflect.String, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load limit from request
	limit, ok := s.loadValue(w, r, reqJson, "ProjectAttempts", "limit", reflect.Float64, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// attempt to load skip from request
	skip, ok := s.loadValue(w, r, reqJson, "ProjectAttempts", "skip", reflect.Float64, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ProjectAttempts(ctx, s.tiDB, postId, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "ProjectAttempts core failed", r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"project-attempt",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) GetProjectCode(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-project-code-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetProjectCode", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "GetProjectCode", "repo_id", reflect.String, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	repo, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	ref, ok := s.loadValue(w, r, reqJson, "GetProjectCode", "ref", reflect.String, nil, false, userName, userId)
	if ref == nil || !ok {
		return
	}

	// attempt to load parameter from body
	filepath, ok := s.loadValue(w, r, reqJson, "GetProjectCode", "filepath", reflect.String, nil, false, userName, userId)
	if filepath == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetProjectCode", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetProjectCode(ctx, s.vscClient, repo, ref.(string), filepath.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GetProjectCode core failed", r.URL.Path, "GetProjectCode", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-project-code",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetProjectCode", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) GetClosedAttempts(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-closed-attempts-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetClosedAttempts", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "GetClosedAttempts", "project_id", reflect.String, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "GetClosedAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load limit from request
	limit, ok := s.loadValue(w, r, reqJson, "GetClosedAttempts", "limit", reflect.Float64, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// attempt to load skip from request
	skip, ok := s.loadValue(w, r, reqJson, "GetClosedAttempts", "skip", reflect.Float64, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetClosedAttempts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetClosedAttempts(ctx, s.tiDB, postId, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GetClosedAttempts core failed", r.URL.Path, "GetClosedAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-closed-attempts",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetClosedAttempts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) GetProjectFile(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-project-file-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetProjectFile", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "GetProjectFile", "repo_id", reflect.String, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	repo, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	ref, ok := s.loadValue(w, r, reqJson, "GetProjectFile", "ref", reflect.String, nil, false, userName, userId)
	if ref == nil || !ok {
		return
	}

	// attempt to load parameter from body
	filepath, ok := s.loadValue(w, r, reqJson, "GetProjectFile", "filepath", reflect.String, nil, false, userName, userId)
	if filepath == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetProjectFile", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetProjectFile(ctx, s.vscClient, repo, ref.(string), filepath.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GetProjectFile core failed", r.URL.Path, "GetProjectFile", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-project-file",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetProjectFile", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) GetProjectDirectories(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-project-directories-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetProjectDirectories", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "GetProjectDirectories", "repo_id", reflect.String, nil, false, userName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	repo, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	ref, ok := s.loadValue(w, r, reqJson, "GetProjectDirectories", "ref", reflect.String, nil, false, userName, userId)
	if ref == nil || !ok {
		return
	}

	// attempt to load parameter from body
	filepath, ok := s.loadValue(w, r, reqJson, "GetProjectDirectories", "filepath", reflect.String, nil, false, userName, userId)
	if filepath == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetProjectDirectories", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetProjectDirectories(ctx, s.vscClient, repo, ref.(string), filepath.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GetProjectDirectories core failed", r.URL.Path, "GetProjectDirectories", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-project-directories",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetProjectDirectories", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}
