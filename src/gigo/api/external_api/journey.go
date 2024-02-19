package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

type CreateJourneyUnitRequest struct {
	Name        string                       `json:"name" validate:"required,lte=35"`
	UnitAbove   *int64                       `json:"unit_above"`
	UnitBelow   *int64                       `json:"unit_below"`
	Description string                       `json:"description" validate:"required"`
	Langs       []models.ProgrammingLanguage `json:"langs" validate:"required"`
	Tags        []string                     `json:"tags" validate:"required"`
	UploadID    string                       `json:"upload_id" validate:"required"`
}

type CreateJourneyTaskRequest struct {
	JourneyUnitID  int64                      `json:"journey_unit_id" validate:"required"`
	Name           string                     `json:"name" validate:"required"`
	NodeAbove      *int64                     `json:"node_above" validate:"required"`
	NodeBelow      *int64                     `json:"node_below" validate:"required"`
	Description    string                     `json:"description" validate:"required"`
	CodeSourceType models.CodeSource          `json:"code_source_type" validate:"required"`
	CodeSourceID   int64                      `json:"code_source_id" validate:"required"`
	Lang           models.ProgrammingLanguage `json:"lang" validate:"required"`
}

type CreateJourneyDetourRequest struct {
	DetourUnitID int64 `json:"detour_unit_id" validate:"required"`
	UserID       int64 `json:"user_id" validate:"required"`
	TaskID       int64 `json:"task_id" validate:"required"`
}

type CreateJourneyDetourRecommendationRequest struct {
	RecUnitID  int64 `json:"rec_unit_id" validate:"required"`
	UserID     int64 `json:"user_id" validate:"required"`
	FromTaskID int64 `json:"from_task_id" validate:"required"`
}

type CreateJourneyUserMapRequest struct {
	UserID int64                `json:"user_id" validate:"required"`
	Units  []models.JourneyUnit `json:"units" validate:"required"`
}

func (s *HTTPServer) CreateJourneyUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-unit-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// require that the user is admin
	if callingUser.(*models.User).AuthRole != models.Admin {
		s.handleError(w, "only admins can perform this action", r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateJourneyUnit", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// marshall the reqJson then send through the validation system
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshal reqjson", r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	var JourneyUnitReq CreateJourneyUnitRequest
	if ok := s.validateRequest(w, r, callingUser.(*models.User), bytes.NewBuffer(buf), &JourneyUnitReq); !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", JourneyUnitReq.UploadID)

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// call the core
	res, err := core.CreateJourneyUnit(core.CreateJourneyUnitParams{
		Ctx:           ctx,
		TiDB:          s.tiDB,
		Sf:            s.sf,
		StorageEngine: s.storageEngine,
		Meili:         s.meili,
		Name:          JourneyUnitReq.Name,
		UnitAbove:     JourneyUnitReq.UnitAbove,
		UnitBelow:     JourneyUnitReq.UnitBelow,
		Thumbnail:     thumbnailTempPath,
		Langs:         JourneyUnitReq.Langs,
		Description:   JourneyUnitReq.Description,
		Tags:          JourneyUnitReq.Tags,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "core failed", r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"create-journey-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) PublishJourneyUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "publish-journey-attempt-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "PublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "PublishJourneyUnit", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	unitIdI, ok := s.loadValue(w, r, reqJson, "PublishJourneyUnit", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUnitId, err := strconv.ParseInt(unitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", unitIdI.(string)), r.URL.Path, "PublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PublishJourneyUnit(core.PublishJourneyUnitParams{
		Ctx:       ctx,
		TiDB:      s.tiDB,
		JourneyID: journeyUnitId,
		Meili:     s.meili,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "PublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"publish-journey-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UnPublishJourneyUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "un-publish-journey-unit-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UnPublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UnPublishJourneyUnit", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	unitIdI, ok := s.loadValue(w, r, reqJson, "UnPublishJourneyUnit", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUnitId, err := strconv.ParseInt(unitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", unitIdI.(string)), r.URL.Path, "UnPublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UnPublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UnPublishJourneyUnit(core.UnPublishJourneyUnitParams{
		Ctx:       ctx,
		TiDB:      s.tiDB,
		JourneyID: journeyUnitId,
		Meili:     s.meili,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "UnPublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"un-publish-journey-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UnPublishJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteJourneyUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-journey-unit-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteJourneyUnit", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	unitIdI, ok := s.loadValue(w, r, reqJson, "DeleteJourneyUnit", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUnitId, err := strconv.ParseInt(unitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", unitIdI.(string)), r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteJourneyUnit(core.DeleteJourneyUnitParams{
		Ctx:       ctx,
		TiDB:      s.tiDB,
		JourneyID: journeyUnitId,
		Meili:     s.meili,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-journey-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyTask(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-task-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// require that the user is admin
	if callingUser.(*models.User).AuthRole != models.Admin {
		s.handleError(w, "only admins can perform this action", r.URL.Path, "CreateJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateJourneyTask", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// marshall the reqJson then send through the validation system
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshal reqjson", r.URL.Path, "CreateJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	var journeyTaskReq CreateJourneyTaskRequest
	if ok := s.validateRequest(w, r, callingUser.(*models.User), bytes.NewBuffer(buf), &journeyTaskReq); !ok {
		return
	}

	// call the core
	res, err := core.CreateJourneyTask(core.CreateJourneyTaskParams{
		Ctx:            ctx,
		TiDB:           s.tiDB,
		Sf:             s.sf,
		JourneyUnitID:  journeyTaskReq.JourneyUnitID,
		Name:           journeyTaskReq.Name,
		NodeBelow:      journeyTaskReq.NodeBelow,
		NodeAbove:      journeyTaskReq.NodeAbove,
		Description:    journeyTaskReq.Description,
		CodeSourceType: journeyTaskReq.CodeSourceType,
		CodeSourceID:   journeyTaskReq.CodeSourceID,
		Lang:           journeyTaskReq.Lang,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "core failed", r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"create-journey-task",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetUserJourneyTask(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-journey-task-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetUserJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserJourneyTask", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	taskIdI, ok := s.loadValue(w, r, reqJson, "GetUserJourneyTask", "task_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if taskIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyTaskId, err := strconv.ParseInt(taskIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", taskIdI.(string)), r.URL.Path, "GetUserJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	userIdI, ok := s.loadValue(w, r, reqJson, "GetUserJourneyTask", "user_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if userIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUserId, err := strconv.ParseInt(userIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", userIdI.(string)), r.URL.Path, "GetUserJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetUserJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetUserJourneyTask(core.GetUserJourneyTaskParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		TaskID: journeyTaskId,
		UserID: journeyUserId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "GetUserJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-user-journey-task",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetUserJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) PublishJourneyTask(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "publish-journey-task-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "PublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "PublishJourneyTask", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	taskIdI, ok := s.loadValue(w, r, reqJson, "PublishJourneyTask", "task_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if taskIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyTaskId, err := strconv.ParseInt(taskIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", taskIdI.(string)), r.URL.Path, "PublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PublishJourneyTask(core.PublishJourneyTaskParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		TaskID: journeyTaskId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "PublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"publish-journey-task",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UnPublishJourneyTask(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "un-publish-journey-task-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UnPublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UnPublishJourneyTask", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	taskIdI, ok := s.loadValue(w, r, reqJson, "UnPublishJourneyTask", "task_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if taskIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyTaskId, err := strconv.ParseInt(taskIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", taskIdI.(string)), r.URL.Path, "UnPublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UnPublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UnPublishJourneyTask(core.UnPublishJourneyTaskParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		TaskID: journeyTaskId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UnPublishJourneyTask core failed", r.URL.Path, "UnPublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"un-publish-journey-task",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UnPublishJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteJourneyTask(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-journey-task-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeleteJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteJourneyTask", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	taskIdI, ok := s.loadValue(w, r, reqJson, "DeleteJourneyTask", "task_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if taskIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyTaskId, err := strconv.ParseInt(taskIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", taskIdI.(string)), r.URL.Path, "DeleteJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteJourneyTask(core.DeleteJourneyTaskParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		TaskID: journeyTaskId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "DeleteJourneyTask core failed", r.URL.Path, "DeleteJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-journey-task",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteJourneyTask", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyDetour(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-detour-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// require that the user is admin
	if callingUser.(*models.User).AuthRole != models.Admin {
		s.handleError(w, "only admins can perform this action", r.URL.Path, "CreateJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateJourneyDetour", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// marshall the reqJson then send through the validation system
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshal reqjson", r.URL.Path, "CreateJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	var journeyDetourReq CreateJourneyDetourRequest
	if ok := s.validateRequest(w, r, callingUser.(*models.User), bytes.NewBuffer(buf), &journeyDetourReq); !ok {
		return
	}

	// call the core
	res, err := core.CreateJourneyDetour(core.CreateJourneyDetourParams{
		Ctx:          ctx,
		TiDB:         s.tiDB,
		Sf:           s.sf,
		DetourUnitID: journeyDetourReq.DetourUnitID,
		UserID:       journeyDetourReq.UserID,
		TaskID:       journeyDetourReq.TaskID,
		StartedAt:    time.Now(),
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "core failed", r.URL.Path, "CreateJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"create-journey-detour",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteJourneyDetour(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-journey-detour-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteJourneyDetour", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	detourUnitIdI, ok := s.loadValue(w, r, reqJson, "DeleteJourneyDetour", "task_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if detourUnitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyDetourUnitId, err := strconv.ParseInt(detourUnitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", detourUnitIdI.(string)), r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteJourneyDetour(core.DeleteJourneyDetourParams{
		Ctx:          ctx,
		TiDB:         s.tiDB,
		DetourUnitID: journeyDetourUnitId,
		UserID:       callingUser.(*models.User).ID,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "DeleteJourneyTask core failed", r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-journey-detour",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyDetourRecommendation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-detour-rec-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// require that the user is admin
	if callingUser.(*models.User).AuthRole != models.Admin {
		s.handleError(w, "only admins can perform this action", r.URL.Path, "CreateJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateJourneyDetourRecommendation", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// marshall the reqJson then send through the validation system
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshal reqjson", r.URL.Path, "CreateJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	var journeyDetourReq CreateJourneyDetourRecommendationRequest
	if ok := s.validateRequest(w, r, callingUser.(*models.User), bytes.NewBuffer(buf), &journeyDetourReq); !ok {
		return
	}

	// call the core
	res, err := core.CreateJourneyDetourRecommendation(core.CreateDetourRecommendationParams{
		Ctx:        ctx,
		TiDB:       s.tiDB,
		Sf:         s.sf,
		RecUnitID:  journeyDetourReq.RecUnitID,
		UserID:     journeyDetourReq.UserID,
		FromTaskID: journeyDetourReq.FromTaskID,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "core failed", r.URL.Path, "CreateJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"create-journey-detour-rec",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteJourneyDetourRecommendation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-journey-detour-rec-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeleteJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteJourneyDetourRecommendation", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	recUnitIdI, ok := s.loadValue(w, r, reqJson, "DeleteJourneyDetourRecommendation", "rec_unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if recUnitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	recUnitId, err := strconv.ParseInt(recUnitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", recUnitIdI.(string)), r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	userIdI, ok := s.loadValue(w, r, reqJson, "DeleteJourneyDetourRecommendation", "rec_unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if userIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	userId, err := strconv.ParseInt(recUnitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", recUnitIdI.(string)), r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteJourneyDetour", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteJourneyDetourRecommendation(core.DeleteDetourRecommendationParams{
		Ctx:     ctx,
		TiDB:    s.tiDB,
		RecUnit: recUnitId,
		UserID:  userId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "DeleteJourneyTask core failed", r.URL.Path, "DeleteJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-journey-detour-rec",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteJourneyDetourRecommendation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyUserMap(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-user-map-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// require that the user is admin
	if callingUser.(*models.User).AuthRole != models.Admin {
		s.handleError(w, "only admins can perform this action", r.URL.Path, "CreateJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateJourneyUserMap", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// marshall the reqJson then send through the validation system
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshal reqjson", r.URL.Path, "CreateJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	var journeyMapReq CreateJourneyUserMapRequest
	if ok := s.validateRequest(w, r, callingUser.(*models.User), bytes.NewBuffer(buf), &journeyMapReq); !ok {
		return
	}

	// call the core
	res, err := core.CreateJourneyUserMap(core.CreateJourneyUserMapParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		UserID: journeyMapReq.UserID,
		Units:  journeyMapReq.Units,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "core failed", r.URL.Path, "CreateJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"create-journey-user-map",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetAllTasksInUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-all-tasks-in-unit-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetAllTasksInUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetAllTasksInUnit", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	unitIdI, ok := s.loadValue(w, r, reqJson, "GetAllTasksInUnit", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUnitId, err := strconv.ParseInt(unitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", unitIdI.(string)), r.URL.Path, "GetAllTasksInUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	userIdI, ok := s.loadValue(w, r, reqJson, "GetAllTasksInUnit", "user_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if userIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUserId, err := strconv.ParseInt(userIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", userIdI.(string)), r.URL.Path, "GetAllTasksInUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetAllTasksInUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetAllTasksInUnit(core.GetAllTasksInUnitParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		UnitID: journeyUnitId,
		UserID: journeyUserId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "GetAllTasksInUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-all-tasks-in-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetAllTasksInUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetJourneyUserMap(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-journey-user-map-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetJourneyUserMap", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	userIdI, ok := s.loadValue(w, r, reqJson, "GetJourneyUserMap", "user_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if userIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUserId, err := strconv.ParseInt(userIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", userIdI.(string)), r.URL.Path, "GetJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetJourneyUserMap(core.GetJourneyUserMapParams{
		Ctx:    ctx,
		TiDB:   s.tiDB,
		UserID: journeyUserId,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetJourneyUserMap core failed", r.URL.Path, "GetJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-journey-user-map",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UpdateJourneyUnitTree(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "update-journey-unit-tree-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UpdateJourneyUnitTree", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UpdateJourneyUnitTree", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	unitIdI, ok := s.loadValue(w, r, reqJson, "UpdateJourneyUnitTree", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyUnitId, err := strconv.ParseInt(unitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", unitIdI.(string)), r.URL.Path, "UpdateJourneyUnitTree", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	unitAboveIdI, ok := s.loadValue(w, r, reqJson, "UpdateJourneyUnitTree", "unit_above", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	var unitAbove *int64 = nil
	if unitAboveIdI != nil {
		temp, err := strconv.ParseInt(unitAboveIdI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", unitAboveIdI), r.URL.Path, "UpdateJourneyUnitTree", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		unitAbove = &temp
	}

	// attempt to load code source id from body
	unitBelowIdI, ok := s.loadValue(w, r, reqJson, "UpdateJourneyUnitTree", "unit_below", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	var unitBelow *int64 = nil
	if unitBelowIdI != nil {
		temp, err := strconv.ParseInt(unitBelowIdI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", unitBelowIdI), r.URL.Path, "UpdateJourneyUnitTree", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		unitBelow = &temp
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetJourneyUserMap", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UpdateJourneyUnitTree(core.UpdateJourneyUnitTreeParams{
		Ctx:       ctx,
		TiDB:      s.tiDB,
		UnitID:    journeyUnitId,
		UnitAbove: unitAbove,
		UnitBelow: unitBelow,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UpdateJourneyUnitTree core failed", r.URL.Path, "UpdateJourneyUnitTree", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"update-journey-unit-tree",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UpdateJourneyUnitTree", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UpdateJourneyTaskTree(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "update-journey-task-tree-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UpdateJourneyTaskTree", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	unitIdI, ok := s.loadValue(w, r, reqJson, "UpdateJourneyTaskTree", "task_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	journeyTaskId, err := strconv.ParseInt(unitIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", unitIdI.(string)), r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	nodeAboveIdI, ok := s.loadValue(w, r, reqJson, "UpdateJourneyTaskTree", "node_above", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if nodeAboveIdI == nil || !ok {
		return
	}

	var nodeAbove *int64 = nil
	if nodeAboveIdI != nil {
		temp, err := strconv.ParseInt(nodeAboveIdI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", nodeAboveIdI), r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		nodeAbove = &temp
	}

	// attempt to load code source id from body
	nodeBelowIdI, ok := s.loadValue(w, r, reqJson, "UpdateJourneyTaskTree", "node_below", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if nodeBelowIdI == nil || !ok {
		return
	}

	var nodeBelow *int64 = nil
	if nodeBelowIdI != nil {
		temp, err := strconv.ParseInt(nodeBelowIdI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", nodeBelowIdI), r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		nodeBelow = &temp
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UpdateJourneyTaskTree(core.UpdateJourneyTaskUnitTreeParams{
		Ctx:       ctx,
		TiDB:      s.tiDB,
		TaskID:    journeyTaskId,
		TaskBelow: nodeBelow,
		TaskAbove: nodeAbove,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UpdateJourneyTaskTree core failed", r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"update-journey-task-tree",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UpdateJourneyTaskTree", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
