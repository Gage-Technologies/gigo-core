package external_api

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/gigo/api/external_api/core"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/google/uuid"
)

func (s *HTTPServer) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load workspacePath from body
	workspacePath, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "commit", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspacePath == nil || !ok {
		return
	}

	// attempt to load repo id from body
	repoIdI, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "repo", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if repoIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	repoId, err := strconv.ParseInt(repoIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse repo id string to integer: %s", repoIdI.(string)), r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	csIdI, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "code_source_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if csIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	csId, err := strconv.ParseInt(csIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", repoIdI.(string)), r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source type from body
	csType, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "code_source_type", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if csType == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateWorkspace(ctx, s.tiDB, s.vscClient, s.jetstreamClient, s.sf, s.wsStatusUpdater, callingUser.(*models.User),
		s.accessUrl.String(), repoId, workspacePath.(string), csId, models.CodeSource(csType.(float64)),
		s.hostname, s.useTls)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateWorkspace core failed", r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-workspace-status-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load ownerCount from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "GetWorkspace", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post ownerCount to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", wsIdI.(string)), r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetWorkspaceStatus(ctx, s.tiDB, s.vscClient, callingUser.(*models.User), wsId, s.hostname, s.useTls)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetWorkspace core failed", r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-workspace-status",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) StreakCheck(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "streak-check-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StreakCheck", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to load coder workspace id from body
	workspaceId, ok := s.loadValue(w, r, reqJson, "StreakCheck", "workspace_id", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	wsId, err := strconv.ParseInt(workspaceId.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load coder workspace id from body
	secret, ok := s.loadValue(w, r, reqJson, "StreakCheck", "secret", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	decodedSecret, err := uuid.Parse(secret.(string))
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StreakCheck(ctx, s.tiDB, wsId, decodedSecret)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StreakCheck core failed", r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"streak-check",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, res, r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) ExtendExpiration(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "extend-expiration-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ExtendExpiration", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to load coder workspace id from body
	workspaceId, ok := s.loadValue(w, r, reqJson, "ExtendExpiration", "workspace_id", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	wsId, err := strconv.ParseInt(workspaceId.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load coder workspace id from body
	secret, ok := s.loadValue(w, r, reqJson, "ExtendExpiration", "secret", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	decodedSecret, err := uuid.Parse(secret.(string))
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ExtendExpiration(ctx, s.tiDB, s.wsStatusUpdater, wsId, decodedSecret, s.jetstreamClient)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ExtendExpiration core failed", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"extend-expiration",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, res, r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceAFK(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-afk-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceAFK", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to load coder workspace id from body
	workspaceId, ok := s.loadValue(w, r, reqJson, "WorkspaceAFK", "workspace_id", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	wsId, err := strconv.ParseInt(workspaceId.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load minutes added from body
	rawMinutes, ok := s.loadValue(w, r, reqJson, "WorkspaceAFK", "add_min", reflect.String, nil, true, "workspace", "")
	if !ok {
		return
	}

	// load minutes added if it was passed
	addMin, err := strconv.ParseInt(rawMinutes.(string), 10, 64)
	if err != nil {
		return
	}

	// attempt to load coder workspace id from body
	secret, ok := s.loadValue(w, r, reqJson, "WorkspaceAFK", "secret", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	// decode the secret
	decodedSecret, err := uuid.Parse(secret.(string))
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.WorkspaceAFK(ctx, s.tiDB, s.wsStatusUpdater, wsId, decodedSecret, addMin)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "WorkspaceAFK core failed", r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-afk",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, res, r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceInitializationStepCompleted(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-initialization-step-completed-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceInitializationStepCompleted", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceInitializationStepCompleted",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to load step int from the json body
	state, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationStepCompleted", "state", reflect.Float64, nil, false, "workspace", "")
	if state == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WorkspaceInitializationStepCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	err := core.WorkspaceInitializationStepCompleted(ctx, s.tiDB, s.wsStatusUpdater, workspaceId.(int64), models.WorkspaceInitState(state.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "WorkspaceInitializationStepCompleted core failed", r.URL.Path,
			"WorkspaceInitializationStepCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", fmt.Sprintf("%d", workspaceId),
			http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-initialization-step-completed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "WorkspaceInitializationStepCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceInitializationFailure(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-initialization-failure-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceInitializationFailure", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceInitializationFailure",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to load step int from the json body
	state, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "state", reflect.Float64, nil, false, "workspace", "")
	if state == nil || !ok {
		return
	}

	// attempt to load command string from the json body
	command, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "command", reflect.String, nil, false, "workspace", "")
	if command == nil || !ok {
		return
	}

	// attempt to load status int from the json body
	status, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "status", reflect.Float64, nil, false, "workspace", "")
	if status == nil || !ok {
		return
	}

	// attempt to load stdout string from the json body
	stdout, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "stdout", reflect.String, nil, false, "workspace", "")
	if stdout == nil || !ok {
		return
	}

	// attempt to load stderr string from the json body
	stderr, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "stderr", reflect.String, nil, false, "workspace", "")
	if stderr == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	err := core.WorkspaceInitializationFailure(ctx, s.tiDB, s.wsStatusUpdater, workspaceId.(int64), models.WorkspaceInitState(state.(float64)),
		command.(string), int(status.(float64)), stdout.(string), stderr.(string), s.jetstreamClient)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "WorkspaceInitializationFailure core failed", r.URL.Path, "WorkspaceInitializationFailure", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-initialization-failure",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "WorkspaceInitializationFailure", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceGetExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-extension-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("ext/gigo-developer.vsix")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-extension",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetCtExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-ct-extension-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetCtExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("ext/code-teacher.vsix")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetCtExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetCtExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-ct-extension",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetCtExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetThemeExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-theme-extension-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetThemeExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("ext/gigo-theme.vsix")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-theme-extension-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetThemeExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetHolidayThemeExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-holiday-theme-extension-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceGetHolidayThemeExtension", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetHolidayThemeExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to load step int from the json body
	holidayS, ok := s.loadValue(w, r, reqJson, "WorkspaceGetHolidayThemeExtension", "holiday", reflect.String, nil, false, "workspace", "")
	if holidayS == nil || !ok {
		return
	}

	// parse post id to integer
	holiday, err := strconv.ParseInt(holidayS.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse holiday string to integer: %s", holidayS.(string)), r.URL.Path, "WorkspaceGetHolidayThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}
	if holiday < 1 {
		s.handleError(w, "invalid holiday passed in", r.URL.Path, "WorkspaceGetHolidayThemeExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile(fmt.Sprintf("ext/%v.vsix", agentsdk.Holiday(holiday).String()))
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetHolidayThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get--holiday-theme-extension-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetHolidayThemeExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetAgent(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-agent-http")
	defer parentSpan.End()

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("bin/agent")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve agent binary", r.URL.Path, "WorkspaceGetAgent", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetAgent", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-agent",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetAgent",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) StartWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "start-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StartWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "StartWorkspace", "workspace_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", wsIdI.(string)), r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StartWorkspace(ctx, s.tiDB, s.jetstreamClient, s.wsStatusUpdater, callingUser.(*models.User), wsId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartWorkspace core failed", r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"start-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) StopWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "stop-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StopWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "StopWorkspace", "workspace_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", wsIdI.(string)), r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StopWorkspace(ctx, s.tiDB, s.jetstreamClient, s.wsStatusUpdater, callingUser.(*models.User), wsId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StopWorkspace core failed", r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"stop-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetHighestScore(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-highest-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetHighestScore", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetHighestScore(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetHighestScore core failed", r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-highest-score",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) SetHighestScore(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-highest-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SetHighestScore", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "SetHighestScore", "highest_score", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse highest score string into int: %s", wsIdI.(string)), r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SetHighestScore(ctx, s.tiDB, callingUser.(*models.User), wsId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SetHighestScore core failed", r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-highest-score",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CodeServerPullThroughCache(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "code-server-pull-through-cache-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "CodeServerPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve the version, arch and install type from the query params
	version := r.URL.Query().Get("version")
	arch := r.URL.Query().Get("arch")
	os := r.URL.Query().Get("os")
	installType := r.URL.Query().Get("type")

	// ensure both values are provided
	if version == "" || arch == "" || os == "" || installType == "" {
		s.handleError(w, "version, arch, os and type are required", r.URL.Path, "CodeServerPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "version, arch, os and type are required", nil)
		return
	}

	// validate the install type
	if installType != "tar" && installType != "deb" && installType != "rpm" {
		s.handleError(w, "invalid install type provided", r.URL.Path, "CodeServerPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "invalid install type provided", nil)
		return
	}

	// attempt to retrieve file from storage
	buf, err := s.storageEngine.GetFile(fmt.Sprintf("ext/gigo-code-server-cache/%s-%s-%s-%s", version, arch, os, installType))
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "CodeServerPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// if we miss the cache then download the file from the source
	download := false
	if buf == nil {
		// if we have a cache miss then download the file from the source
		buf, err = core.CodeServerPullThroughCache(r.Context(), s.storageEngine, version, arch, os, installType)
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to retrieve extension file", r.URL.Path, "CodeServerPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		download = true
	}

	// ensure closure of file
	defer buf.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, buf)
	if err != nil {
		// if we are downloading then we need to invalidate the cache
		if download {
			// delete the file from storage
			_ = s.storageEngine.DeleteFile(fmt.Sprintf("ext/gigo-code-server-cache/%s-%s-%s-%s", version, arch, os, installType))
		}

		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "CodeServerPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-code-server",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "CodeServerPullThroughCache",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
	return
}

func (s *HTTPServer) OpenVsxPullThroughCache(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "openvsx-pull-through-cache-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "OpenVsxPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve the extension id and version from the query params
	extensionId := r.URL.Query().Get("ext")
	version := r.URL.Query().Get("version")
	vscVersion := r.URL.Query().Get("vscVersion")

	// ensure the extension id is provided
	if extensionId == "" {
		s.handleError(w, "extension id required", r.URL.Path, "OpenVsxPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "extension id required", nil)
		return
	}

	// split the extension id into the publisher and name
	extensionIdSplit := strings.Split(extensionId, ".")
	if len(extensionIdSplit) != 2 {
		s.handleError(w, "invalid extension id provided", r.URL.Path, "OpenVsxPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "invalid extension id provided", nil)
		return
	}

	// retrieve the extension
	buf, download, err := core.OpenVsxPullThroughCache(r.Context(), s.storageEngine, s.rdb, extensionId, version, vscVersion)
	if err != nil {
		if strings.Contains(err.Error(), "extension not found") {
			s.handleError(w, fmt.Sprintf("extension not found: %s - %s", extensionId, version), r.URL.Path, "OpenVsxPullThroughCache",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusNotFound, "extension not found", nil)
			return
		}
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "OpenVsxPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer buf.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, buf)
	if err != nil {
		// if we are downloading then we need to invalidate the cache
		if download {
			// delete the file from storage
			_ = s.storageEngine.DeleteFile(fmt.Sprintf("ext/open-vsx-cache/%s/%s.%s.vsix", extensionIdSplit[0], extensionIdSplit[1], version))
		}

		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "OpenVsxPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-extension-cache",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("extension", extensionId),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "OpenVsxPullThroughCache",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
	return
}

func (s *HTTPServer) CreateByteWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-byte-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateByteWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateByteWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	byteIdI, ok := s.loadValue(w, r, reqJson, "CreateByteWorkspace", "byte_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if byteIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	byteId, err := strconv.ParseInt(byteIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", byteIdI.(string)), r.URL.Path, "CreateByteWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateByteWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateByteWorkspace(ctx, s.tiDB, s.jetstreamClient, s.sf, s.wsStatusUpdater, callingUser.(*models.User),
		s.accessUrl.String(), byteId, s.hostname, s.useTls)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateByteWorkspace core failed", r.URL.Path, "CreateByteWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-byte-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateByteWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
