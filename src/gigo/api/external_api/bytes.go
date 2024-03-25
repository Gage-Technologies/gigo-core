package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/types"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type CreateByteRequest struct {
	Name                   string                     `json:"name" validate:"required,lte=100"`
	DescriptionEasy        string                     `json:"description_easy" validate:"required,lte=500"`
	DescriptionMedium      string                     `json:"description_medium" validate:"required,lte=500"`
	DescriptionHard        string                     `json:"description_hard" validate:"required,lte=500"`
	EasyFiles              []types.CodeFile           `json:"easy_files" validate:"required"`
	MediumFiles            []types.CodeFile           `json:"medium_files" validate:"required"`
	HardFiles              []types.CodeFile           `json:"hard_files" validate:"required"`
	DevelopmentStepsEasy   string                     `json:"development_steps_easy" validate:"required"`
	DevelopmentStepsMedium string                     `json:"development_steps_medium" validate:"required"`
	DevelopmentStepsHard   string                     `json:"development_steps_hard" validate:"required"`
	QuestionsEasy          []string                   `json:"questions_easy" validate:"required"`
	QuestionsMedium        []string                   `json:"questions_medium" validate:"required"`
	QuestionsHard          []string                   `json:"questions_hard" validate:"required"`
	Language               models.ProgrammingLanguage `json:"language" validate:"required"`
	UploadID               string                     `json:"upload_id" validate:"required"`
}

func (s *HTTPServer) CreateByte(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-byte-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// require that the user is admin
	if callingUser.(*models.User).AuthRole != models.Admin {
		s.handleError(w, "only admins can perform this action", r.URL.Path, "CreateByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateByte", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// marshall the reqJson then send through the validation system
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshal reqjson", r.URL.Path, "CreateByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	var byteReq CreateByteRequest
	if ok := s.validateRequest(w, r, callingUser.(*models.User), bytes.NewBuffer(buf), &byteReq); !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", byteReq.UploadID)

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// call the core
	res, err := core.CreateByte(core.CreateByteParams{
		Ctx:                    ctx,
		Tidb:                   s.tiDB,
		Sf:                     s.sf,
		CallingUser:            callingUser.(*models.User),
		StorageEngine:          s.storageEngine,
		Meili:                  s.meili,
		Name:                   byteReq.Name,
		DescriptionEasy:        byteReq.DescriptionEasy,
		DescriptionMedium:      byteReq.DescriptionMedium,
		DescriptionHard:        byteReq.DescriptionHard,
		FilesEasy:              byteReq.EasyFiles,
		FilesMedium:            byteReq.MediumFiles,
		FilesHard:              byteReq.HardFiles,
		DevelopmentStepsEasy:   byteReq.DevelopmentStepsEasy,
		DevelopmentStepsMedium: byteReq.DevelopmentStepsMedium,
		DevelopmentStepsHard:   byteReq.DevelopmentStepsHard,
		QuestionsEasy:          byteReq.QuestionsEasy,
		QuestionsMedium:        byteReq.QuestionsMedium,
		QuestionsHard:          byteReq.QuestionsHard,
		Language:               byteReq.Language,
		Thumbnail:              thumbnailTempPath,
	})
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "core failed", r.URL.Path, "CreateByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"create-byte",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) StartByteAttempt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "start-byte-attempt-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StartByteAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StartByteAttempt", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	byteIdI, ok := s.loadValue(w, r, reqJson, "StartByteAttempt", "byte_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if byteIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	byteId, err := strconv.ParseInt(byteIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", byteIdI.(string)), r.URL.Path, "StartByteAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StartByteAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StartByteAttempt(ctx, s.tiDB, s.sf, callingUser.(*models.User), byteId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartByteAttempt core failed", r.URL.Path, "StartByteAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"start-byte-attempt",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "StartByteAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetByteAttempt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-byte-attempts-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetByteAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetByteAttempts", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetByteAttempts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetByteAttempt(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetByteAttempts core failed", r.URL.Path, "GetByteAttempts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-byte-attempts",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetByteAttempts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetRecommendedBytes(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-recommended-bytes-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUserI := r.Context().Value(CtxKeyUser)

	userName := ""
	userId := ""
	var userIdInt int64
	var callingUser *models.User

	// return if calling user was not retrieved in authentication
	if callingUserI == nil {
		userName = network.GetRequestIP(r)
		userId = network.GetRequestIP(r)
	} else {
		callingUser = callingUserI.(*models.User)
		userName = callingUser.UserName
		userId = fmt.Sprintf("%d", callingUser.ID)
		userIdInt = callingUser.ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetRecommendedBytes", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetRecommendedBytes", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	var userIdPointer *int64

	if callingUser != nil {
		userIdPointer = &userIdInt
	}

	// execute core function logic
	res, err := core.GetRecommendedBytes(ctx, s.tiDB, userIdPointer)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetRecommendedBytes core failed", r.URL.Path, "GetRecommendedBytes", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-recommended-bytes",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetRecommendedBytes", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) GetByte(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-byte-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUserI := r.Context().Value(CtxKeyUser)

	userName := ""
	userId := ""
	var userIdInt int64
	var callingUser *models.User

	// return if calling user was not retrieved in authentication
	if callingUserI == nil {
		userName = network.GetRequestIP(r)
		userId = network.GetRequestIP(r)
	} else {
		callingUser = callingUserI.(*models.User)
		userName = callingUser.UserName
		userId = fmt.Sprintf("%d", callingUser.ID)
		userIdInt = callingUser.ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetByte", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	byteIdI, ok := s.loadValue(w, r, reqJson, "GetByte", "byte_id", reflect.String, nil, false, userName, userId)
	if byteIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	byteId, err := strconv.ParseInt(byteIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", byteIdI.(string)), r.URL.Path, "GetByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetByte", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetByte(ctx, s.tiDB, byteId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetByte core failed", r.URL.Path, "GetByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-byte",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetByte", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SetByteCompleted(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-byte-completed-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "SetByteCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SetByteCompleted", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	byteIdI, ok := s.loadValue(w, r, reqJson, "SetByteCompleted", "byte_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if byteIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	byteId, err := strconv.ParseInt(byteIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", byteIdI.(string)), r.URL.Path, "SetByteCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	difficultyI, ok := s.loadValue(w, r, reqJson, "SetByteCompleted", "difficulty", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if difficultyI == nil || !ok {
		return
	}

	// attempt to load parameter from body
	journeyI, ok := s.loadValue(w, r, reqJson, "SetByteCompleted", "journey", reflect.Bool, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	var journey *bool = nil
	if journeyI != nil {
		tempJourney := journeyI.(bool)
		journey = &tempJourney
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SetByteCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SetByteCompleted(ctx, s.tiDB, s.sf, s.stripeSubscriptions, callingUser.(*models.User), byteId, difficultyI.(string), journey, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SetByteCompleted core failed", r.URL.Path, "SetByteCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-byte-completed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SetByteCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) PublishByte(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "publish-byte-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "PublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// ensure that only admins can publish bytes
	if callingUser.(*models.User).AuthRole != models.Admin {
		// return unauthorized
		s.handleError(w, "non-admin attempted to publish a byte", r.URL.Path, "PublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusUnauthorized, "only admins can publish vytes", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "PublishByte", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	byteIdI, ok := s.loadValue(w, r, reqJson, "PublishByte", "byte_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if byteIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	byteId, err := strconv.ParseInt(byteIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse byte id string to integer: %s", byteIdI.(string)), r.URL.Path, "PublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PublishByte", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PublishByte(ctx, s.tiDB, s.meili, byteId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "PublishByte core failed", r.URL.Path, "PublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"publish-byte-completed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.Int64("byte", byteId),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PublishByte", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UnPublishByte(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "unpublish-byte-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// ensure that only admins can publish bytes
	if callingUser.(*models.User).AuthRole != models.Admin {
		// return unauthorized
		s.handleError(w, "non-admin attempted to unpublish a byte", r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusUnauthorized, "only admins can publish vytes", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UnPublishByte", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	byteIdI, ok := s.loadValue(w, r, reqJson, "UnPublishByte", "byte_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if byteIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	byteId, err := strconv.ParseInt(byteIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse byte id string to integer: %s", byteIdI.(string)), r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UnPublishByte(ctx, s.tiDB, s.meili, byteId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UnPublishByte core failed", r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"unpublish-byte-completed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.Int64("byte", byteId),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CheckByteHelpMobile(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "check-byte-help-mobile-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UnPublishByte", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// Execute core function logic
	res, err := core.CheckByteHelpMobile(ctx, s.tiDB, callingUser.(*models.User).ID)
	if err != nil {
		// Handle potential errors from the core function
		responseMessage := "internal server error occurred"
		if res != nil {
			if msg, ok := res["error"].(string); ok {
				responseMessage = msg
			}
		}
		s.handleError(w, "core function checkByteHelpMobile failed", r.URL.Path, "CheckByteHelpMobile", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"checked-byte-help-mobile",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "CheckByteHelpMobile", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DisableByteHelpMobile(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "disable-byte-help-mobile-http")
	defer parentSpan.End()

	// Retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DisableByteHelpMobile", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingUserTyped := callingUser.(*models.User)
	callingId := strconv.FormatInt(callingUserTyped.ID, 10)

	// Execute core function logic
	res, err := core.DisableByteHelpMobile(ctx, s.tiDB, callingUserTyped.ID)
	if err != nil {
		// Handle potential errors from the core function
		responseMessage := "internal server error occurred"
		if res != nil {
			if msg, ok := res["error"].(string); ok {
				responseMessage = msg
			}
		}
		s.handleError(w, "core function DisableByteHelpMobile failed", r.URL.Path, "DisableByteHelpMobile", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserTyped.UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"disabled-byte-help-mobile",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUserTyped.UserName),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "DisableByteHelpMobile", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserTyped.UserName, callingId, http.StatusOK)
}
