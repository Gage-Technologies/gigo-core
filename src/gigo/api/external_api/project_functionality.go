package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
)

func (s *HTTPServer) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-project-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// read request body into byte slice
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to read request body", r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// create a new buffer for the response and re-assign to the request body
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// create variables for json payload and temp image file
	var reqJson map[string]interface{}
	var thumbnailTempPath string

	// handle gen images by loading json otherwise receive upload
	if _, err := jsonparser.GetString(body, "gen_image_id"); err == nil {
		// attempt to load JSON from request body
		reqJson = s.jsonRequest(w, r, "CreateProject", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
		if reqJson == nil {
			return
		}

		// attempt to load gen_image_id from body
		genImageId, ok := s.loadValue(w, r, reqJson, "CreateProject", "gen_image_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
		if genImageId == nil || !ok {
			return
		}

		// create thumbnail temp path
		thumbnailTempPath = fmt.Sprintf("temp_proj_images/%v/%v.jpg", callingUser.(*models.User).ID, genImageId.(string))
	} else {
		// receive upload part and handle file assemble
		reqJson = s.receiveUpload(w, r, "CreateProject", "File Part Uploaded.", callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
		if reqJson == nil {
			return
		}

		// attempt to load parameter from body
		uploadId, ok := s.loadValue(w, r, reqJson, "CreateProject", "upload_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
		if uploadId == nil || !ok {
			return
		}

		// create thumbnail temp path
		thumbnailTempPath = filepath.Join("temp", uploadId.(string))

		// defer removal of thumbnail temp file
		defer s.storageEngine.DeleteFile(thumbnailTempPath)
	}

	// attempt to load parameter from body
	name, ok := s.loadValue(w, r, reqJson, "CreateProject", "name", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if name == nil || !ok {
		return
	}

	// attempt to load parameter from body
	description, ok := s.loadValue(w, r, reqJson, "CreateProject", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// attempt to load parameter from body
	challengeType, ok := s.loadValue(w, r, reqJson, "CreateProject", "challenge_type", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if challengeType == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tier, ok := s.loadValue(w, r, reqJson, "CreateProject", "tier", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if tier == nil || !ok {
		return
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "CreateProject", "languages", reflect.Slice, &langsType, false, callingUser.(*models.User).UserName, callingId)
	if languagesI == nil || !ok {
		return
	}

	// create a slice to hold languages loaded from the http parameter
	languages := make([]models.ProgrammingLanguage, 0)

	// attempt to load parameter from body
	for _, lang := range languagesI.([]interface{}) {
		languages = append(languages, models.ProgrammingLanguage(lang.(float64)))
	}

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateProject", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
	if tagsI == nil || !ok {
		return
	}

	// create a slice to hold tags loaded from the http parameter
	tags := make([]*models.Tag, 0)

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, tagI := range tagsI.([]interface{}) {
		tag := tagI.(map[string]interface{})

		// load id from tag map
		idI, ok := tag["_id"]
		if !ok {
			s.handleError(w, "missing tag id", r.URL.Path, "CreateProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", fmt.Errorf("missing tag id"))
			return
		}

		// assert id as string and parse into int64
		idS, ok := idI.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag id type: %v", reflect.TypeOf(idS)), r.URL.Path,
				"CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag id type - should be string", nil)
			return
		}

		id, err := strconv.ParseInt(idS, 10, 64)
		if !ok {
			s.handleError(w, "failed to parse tag id as int64", r.URL.Path, "CreateProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag value type - should be string", nil,
			)
			return
		}

		tags = append(tags, models.CreateTag(id, val))
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if id == nil || !ok {
		return
	}

	// parse workspace config id to integer
	workspaceConfigId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to workspace config id string to integer: %s", id.(string)), r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	workspaceConfigRevision, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_revision", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspaceConfigRevision == nil || !ok {
		return
	}

	// attempt to load parameter from body
	workspaceConfigContentI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_content", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	workspaceConfigContent := ""
	if workspaceConfigContentI != nil {
		workspaceConfigContent = workspaceConfigContentI.(string)
	}

	// attempt to load parameter from body
	workspaceConfigTitleI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_title", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	workspaceConfigTitle := ""
	if workspaceConfigTitleI != nil {
		workspaceConfigTitle = workspaceConfigTitleI.(string)
	}

	// attempt to load parameter from body
	workspaceConfigDescI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_desc", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	workspaceConfigDesc := ""
	if workspaceConfigDescI != nil {
		workspaceConfigDesc = workspaceConfigDescI.(string)
	}

	// attempt to load parameter from body
	createWorkspaceConfigI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_create", reflect.Bool, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	createWorkspaceConfig := false
	if createWorkspaceConfigI != nil {
		createWorkspaceConfig = createWorkspaceConfigI.(bool)
	}

	// attempt to load parameter from body
	wsCfgLanguagesI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_languages", reflect.Slice, &langsType, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create a slice to hold workspace config languages loaded from the http parameter
	wsCfgLanguages := make([]models.ProgrammingLanguage, 0)

	// attempt to load parameter from body
	if wsCfgLanguagesI != nil {
		for _, lang := range wsCfgLanguagesI.([]interface{}) {
			wsCfgLanguages = append(wsCfgLanguages, models.ProgrammingLanguage(lang.(float64)))
		}
	}

	// attempt to load parameter from body
	wsCfgTagsI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_config_tags", reflect.Slice, &tagsType, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create a slice to hold workspace config tags loaded from the http parameter
	wsCfgTags := make([]*models.Tag, 0)

	// iterate through wsCfgTagsI asserting each value as a map and create a new tag
	if wsCfgTagsI != nil {
		for _, tagI := range tagsI.([]interface{}) {
			tag := tagI.(map[string]interface{})

			// load id from tag map
			idI, ok := tag["_id"]
			if !ok {
				s.handleError(w, "missing tag id", r.URL.Path, "CreateProject", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"internal server error occurred", fmt.Errorf("missing tag id"))
				return
			}

			// assert id as string and parse into int64
			idS, ok := idI.(string)
			if !ok {
				s.handleError(w, fmt.Sprintf("invalid tag id type: %v", reflect.TypeOf(idS)), r.URL.Path,
					"CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"invalid tag id type - should be string", nil)
				return
			}

			id, err := strconv.ParseInt(idS, 10, 64)
			if !ok {
				s.handleError(w, "failed to parse tag id as int64", r.URL.Path, "CreateProject", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
					"internal server error occurred", err)
				return
			}

			// load value from tag map
			valS, ok := tag["value"]
			if !ok {
				s.handleError(w, "missing tag value", r.URL.Path, "CreateProject", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"internal server error occurred", nil)
				return
			}

			// assert val as string
			val, ok := valS.(string)
			if !ok {
				s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
					"CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"invalid tag value type - should be string", nil,
				)
				return
			}

			wsCfgTags = append(wsCfgTags, models.CreateTag(id, val))
		}
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_settings", reflect.Map, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.WorkspaceSettings

	// conditionally attempt to marshall and unmarshall the workspace settings
	if workspaceSettingsI != nil {
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to marshal workspace settings: %s", err), r.URL.Path, "CreateProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to unmarshal workspace settings: %s", err), r.URL.Path, "CreateProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// ensure the validity of the settings
		if workspaceSettings.AutoGit.CommitMessage == "" {
			s.handleError(w, fmt.Sprintf("invalid commit message for workspace settings: %s", workspaceSettings.AutoGit.CommitMessage), r.URL.Path, "CreateProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid commit message", nil)
			return
		}
	}

	// attempt to load parameter from body
	projectCostI, ok := s.loadValue(w, r, reqJson, "CreateProject", "project_cost", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable to hold parent attempt
	var projectCost *int64
	// conditionally parse parent attempt from string
	if projectCostI != nil {
		// parse post id to integer
		pa, err := strconv.ParseInt(projectCostI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse parent attempt id string to integer: %s", projectCostI.(string)), r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		projectCost = &pa
	}

	// attempt to load parameter from body
	projectVisibilityI, ok := s.loadValue(w, r, reqJson, "CreateProject", "project_visibility", reflect.Float64, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	projectVisibility := models.PublicVisibility
	if projectVisibilityI != nil {
		projectVisibility = models.PostVisibility(projectVisibilityI.(float64))
	}

	// attempt to load parameter from body
	workspaceEvaluationI, ok := s.loadValue(w, r, reqJson, "CreateProject", "workspace_evaluation", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	var workspaceEvaluation *string
	if workspaceEvaluationI != nil {
		tempWorkspace := workspaceEvaluationI.(string)
		workspaceEvaluation = &tempWorkspace
	}

	// attempt to load parameter from body
	exclusiveDescriptionI, ok := s.loadValue(w, r, reqJson, "CreateProject", "exclusive_description", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	var exclusiveDescription *string
	if exclusiveDescriptionI != nil {
		tempWorkspace := exclusiveDescriptionI.(string)
		exclusiveDescription = &tempWorkspace
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateProject(
		ctx,
		s.tiDB,
		s.meili,
		s.vscClient,
		s.storageEngine,
		s.rdb,
		s.jetstreamClient,
		callingUser.(*models.User),
		name.(string),
		description.(string),
		s.sf,
		languages,
		models.ChallengeType(challengeType.(float64)),
		models.TierType(tier.(float64)),
		tags,
		thumbnailTempPath,
		workspaceConfigId,
		int(workspaceConfigRevision.(float64)),
		workspaceConfigContent,
		workspaceConfigTitle,
		workspaceConfigDesc,
		wsCfgTags,
		wsCfgLanguages,
		projectVisibility,
		createWorkspaceConfig,
		workspaceSettings,
		workspaceEvaluation,
		projectCost,
		s.logger,
		exclusiveDescription,
	)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "CreateProject core failed", r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-project",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-project-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeleteProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteProject", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	pId, ok := s.loadValue(w, r, reqJson, "DeleteProject", "project_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if pId == nil || !ok {
		return
	}

	// parse post id to integer
	projectId, err := strconv.ParseInt(pId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", pId.(string)), r.URL.Path, "DeleteProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteProject(
		ctx,
		s.tiDB,
		callingUser.(*models.User),
		s.meili,
		projectId,
		s.logger,
	)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "DeleteProject core failed", r.URL.Path, "DeleteProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-project",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) StartAttempt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "start-attempt-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	userSession := r.Context().Value("userSession")

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	if userSession == nil {
		s.handleError(w, "user session missing from context", r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StartAttempt", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "StartAttempt", "project_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	parentAttemptI, ok := s.loadValue(w, r, reqJson, "StartAttempt", "parent_attempt", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable to hold parent attempt
	var parentAttempt *int64
	// conditionally parse parent attempt from string
	if parentAttemptI != nil {
		// parse post id to integer
		pa, err := strconv.ParseInt(parentAttemptI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse parent attempt id string to integer: %s", parentAttemptI.(string)), r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		parentAttempt = &pa
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StartAttempt(ctx, s.tiDB, s.vscClient, s.jetstreamClient, s.rdb, callingUser.(*models.User), userSession.(*models.UserSession), s.sf, postId, parentAttempt, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "StartAttempt core failed", r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"start-attempt",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) PublishProject(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "publish-project-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "PublishProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "PublishProject", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "PublishProject", "project_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "PublishProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PublishProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PublishProject(ctx, s.tiDB, s.meili, postId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "PublishProject core failed", r.URL.Path, "PublishProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"publish-project",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PublishProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) EditConfig(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "edit-config-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "EditConfig", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load workspacePath from body
	content, ok := s.loadValue(w, r, reqJson, "EditConfig", "content", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if content == nil || !ok {
		return
	}

	// attempt to load workspacePath from body
	commit, ok := s.loadValue(w, r, reqJson, "EditConfig", "commit", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if commit == nil || !ok {
		return
	}

	// attempt to load repo id from body
	repoIdI, ok := s.loadValue(w, r, reqJson, "EditConfig", "repo", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if repoIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	repoId, err := strconv.ParseInt(repoIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse repo id string to integer: %s", repoIdI.(string)), r.URL.Path, "EditConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// execute core function logic
	res, err := core.EditConfig(ctx, s.vscClient, callingUser.(*models.User), repoId, content.(string), commit.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "EditConfig core failed", r.URL.Path, "EditConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"edit-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "EditConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) ConfirmEditConfig(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "confirm-edit-config-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ConfirmEditConfig", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	projectIdI, ok := s.loadValue(w, r, reqJson, "ConfirmEditConfig", "project", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if projectIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	projectId, err := strconv.ParseInt(projectIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", projectIdI.(string)), r.URL.Path, "ConfirmEditConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// execute core function logic
	res, err := core.ConfirmEditConfig(ctx, s.tiDB, s.jetstreamClient, s.wsStatusUpdater, callingUser.(*models.User), projectId, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ConfirmEditConfig core failed", r.URL.Path, "ConfirmEditConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"confirm-edit-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ConfirmEditConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-config-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetConfig", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load workspacePath from body
	commit, ok := s.loadValue(w, r, reqJson, "GetConfig", "commit", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if commit == nil || !ok {
		return
	}

	// attempt to load repo id from body
	repoIdI, ok := s.loadValue(w, r, reqJson, "GetConfig", "repo", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if repoIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	repoId, err := strconv.ParseInt(repoIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse repo id string to integer: %s", repoIdI.(string)), r.URL.Path, "GetConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// execute core function logic
	res, err := core.GetConfig(ctx, s.vscClient, callingUser.(*models.User), repoId, commit.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GetConfig core failed", r.URL.Path, "GetConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CloseAttempt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "close-attempt-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CloseAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CloseAttempt", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	attemptIdI, ok := s.loadValue(w, r, reqJson, "CloseAttempt", "attempt_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if attemptIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	attemptId, err := strconv.ParseInt(attemptIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse attempt id string to integer: %s", attemptIdI.(string)), r.URL.Path, "CloseAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// execute core function logic
	res, err := core.CloseAttempt(ctx, s.tiDB, s.vscClient, callingUser.(*models.User), attemptId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "CloseAttempt core failed", r.URL.Path, "CloseAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"close-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CloseAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) MarkSuccess(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "mark-success-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "MarkSuccess", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "MarkSuccess", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	attemptIdI, ok := s.loadValue(w, r, reqJson, "MarkSuccess", "attempt_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if attemptIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	attemptId, err := strconv.ParseInt(attemptIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse attempt id string to integer: %s", attemptIdI.(string)), r.URL.Path, "MarkSuccess", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}
	// execute core function logic
	res, err := core.MarkSuccess(ctx, s.tiDB, s.jetstreamClient, s.rdb, s.sf, attemptId, s.logger, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "MarkSuccess core failed", r.URL.Path, "MarkSuccess", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"mark-success",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "MarkSuccess", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)

}

func (s *HTTPServer) ShareLink(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "share-link-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ShareLink", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ShareLink", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	postIdI, ok := s.loadValue(w, r, reqJson, "ShareLink", "post_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if postIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	postId, err := strconv.ParseInt(postIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse attempt id string to integer: %s", postIdI.(string)), r.URL.Path, "ShareLink", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// execute core function logic
	res, err := core.ShareLink(ctx, s.tiDB, postId, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "ShareLink core failed", r.URL.Path, "ShareLink", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"share-link",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ShareLink", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)

}

func (s *HTTPServer) VerifyLink(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "verify-link-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "VerifyLink", false, "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	postIdI, ok := s.loadValue(w, r, reqJson, "VerifyLink", "post_id", reflect.String, nil, false, "", "")
	if postIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	postId, err := strconv.ParseInt(postIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse attempt id string to integer: %s", postIdI.(string)), r.URL.Path, "VerifyLink", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load share link hash from body
	shareLink, ok := s.loadValue(w, r, reqJson, "VerifyLink", "share_link", reflect.String, nil, false, "", "")
	if shareLink == nil || !ok {
		return
	}

	// execute core function logic
	res, err := core.VerifyLink(ctx, s.tiDB, postId, shareLink.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "VerifyLink core failed", r.URL.Path, "VerifyLink", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"verify-link",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "VerifyLink", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)

}
