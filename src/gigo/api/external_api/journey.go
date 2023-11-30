package external_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"strconv"
)

func (s *HTTPServer) SaveJourneyInfo(w http.ResponseWriter, r *http.Request) {
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
	reqJson := s.jsonRequest(w, r, "SaveJourneyInfo", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	journeyInfoI, ok := s.loadValue(w, r, reqJson, "SaveJourneyInfo", "journey_info", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold journey info initialization form
	var journeyInfo models.JourneyInfo

	// conditionally attempt to marshall and unmarshall the info init form
	bufs, err := json.Marshal(journeyInfoI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall journey info init form: %s", string(bufs)), r.URL.Path, "SaveJourneyInfo", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &journeyInfo)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall journey info init form: %s", string(bufs)), r.URL.Path, "SaveJourneyInfo", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	aptitude, ok := s.loadValue(w, r, reqJson, "SaveJourneyInfo", "aptitude_level", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if aptitude == nil || !ok {
		return
	}

	// load the aptitude level into the journey info
	journeyInfo.AptitudeLevel = aptitude.(string)

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SaveJourneyInfo", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SaveJourneyInfo(ctx, s.tiDB, s.sf, callingUser.(*models.User), journeyInfo)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "SaveJourneyInfo core failed", r.URL.Path, "SaveJourneyInfo", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"save-journey-info",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SaveJourneyInfo", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-unit-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		if callingUser.(*models.User).AuthRole != models.Admin {
			s.handleError(w, fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID), r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "naughty naughty, you shouldn't be here nerd!", errors.New(fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID)))
			return
		}

		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateJourneyUnit", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	title, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "title", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if title == nil || !ok {
		return
	}

	unitFocusT, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "unit_focus", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitFocusT == nil || !ok {
		return
	}
	unitFocus := models.UnitFocus(int(unitFocusT.(float64)))

	// attempt to load parameter from body
	description, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tier, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "tier", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if tier == nil || !ok {
		return
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "languages", reflect.Slice, &langsType, false, callingUser.(*models.User).UserName, callingId)
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
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
	if tagsI == nil || !ok {
		return
	}

	// create a slice to hold tags loaded from the http parameter
	tags := make([]string, 0)

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, tagI := range tagsI.([]interface{}) {
		tag := tagI.(map[string]interface{})

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateJourneyUnit", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag value type - should be string", nil,
			)
			return
		}

		tags = append(tags, val)
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if id == nil || !ok {
		return
	}

	// attempt to load parameter from body
	workspaceConfigContentI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_content", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	workspaceConfigContent := ""
	if workspaceConfigContentI != nil {
		workspaceConfigContent = workspaceConfigContentI.(string)
	}

	// attempt to load parameter from body
	workspaceConfigTitleI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_title", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	workspaceConfigTitle := ""
	if workspaceConfigTitleI != nil {
		workspaceConfigTitle = workspaceConfigTitleI.(string)
	}

	// attempt to load parameter from body
	workspaceConfigDescI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_desc", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	workspaceConfigDesc := ""
	if workspaceConfigDescI != nil {
		workspaceConfigDesc = workspaceConfigDescI.(string)
	}

	// attempt to load parameter from body
	wsCfgLanguagesI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_languages", reflect.Slice, &langsType, true, callingUser.(*models.User).UserName, callingId)
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
	wsCfgTagsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_tags", reflect.Slice, &tagsType, true, callingUser.(*models.User).UserName, callingId)
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
				s.handleError(w, "missing tag id", r.URL.Path, "CreateJourneyUnit", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"internal server error occurred", fmt.Errorf("missing tag id"))
				return
			}

			// assert id as string and parse into int64
			idS, ok := idI.(string)
			if !ok {
				s.handleError(w, fmt.Sprintf("invalid tag id type: %v", reflect.TypeOf(idS)), r.URL.Path,
					"CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"invalid tag id type - should be string", nil)
				return
			}

			id, err := strconv.ParseInt(idS, 10, 64)
			if !ok {
				s.handleError(w, "failed to parse tag id as int64", r.URL.Path, "CreateJourneyUnit", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
					"internal server error occurred", err)
				return
			}

			// load value from tag map
			valS, ok := tag["value"]
			if !ok {
				s.handleError(w, "missing tag value", r.URL.Path, "CreateJourneyUnit", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"internal server error occurred", nil)
				return
			}

			// assert val as string
			val, ok := valS.(string)
			if !ok {
				s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
					"CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
					"invalid tag value type - should be string", nil,
				)
				return
			}

			wsCfgTags = append(wsCfgTags, models.CreateTag(id, val))
		}
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_settings", reflect.Map, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.WorkspaceSettings

	// conditionally attempt to marshall and unmarshall the workspace settings
	if workspaceSettingsI != nil {
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to marshal workspace settings: %s", err), r.URL.Path, "CreateJourneyUnit", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to unmarshal workspace settings: %s", err), r.URL.Path, "CreateJourneyUnit", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// ensure the validity of the settings
		if workspaceSettings.AutoGit.CommitMessage == "" {
			s.handleError(w, fmt.Sprintf("invalid commit message for workspace settings: %s", workspaceSettings.AutoGit.CommitMessage), r.URL.Path, "CreateJourneyUnit", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid commit message", nil)
			return
		}
	}

	// attempt to load parameter from body
	unitCostI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "project_cost", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable to hold parent attempt
	var unitCost *string
	// conditionally parse parent attempt from string
	if unitCostI != nil {
		uns := fmt.Sprintf("%v", unitCostI.(string))
		unitCost = &uns
	}

	// attempt to load parameter from body
	visibilityI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "project_visibility", reflect.Float64, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	visibility := models.PublicVisibility
	if visibilityI != nil {
		visibility = models.PostVisibility(visibilityI.(float64))
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateJourneyUnit(ctx, s.tiDB, s.sf, callingUser.(*models.User), title.(string), unitFocus,
		visibility, languages, description.(string), tags, models.TierType(int(tier.(float64))), unitCost,
		s.vscClient, workspaceConfigContent, workspaceConfigTitle, workspaceConfigDesc, languages, workspaceSettings,
		nil, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "CreateJourneyUnit core failed", r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-journey-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteJourneyUnit(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-journey-unit-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		if callingUser.(*models.User).AuthRole != models.Admin {
			s.handleError(w, fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID), r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "naughty naughty, you shouldn't be here nerd!", errors.New(fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID)))
			return
		}

		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteJourneyUnit", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	unitIDS, ok := s.loadValue(w, r, reqJson, "DeleteJourneyUnit", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIDS == nil || !ok {
		return
	}

	unitID, err := strconv.ParseInt(unitIDS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse unit id as int64", r.URL.Path, "CreateJourneyUnit", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteJourneyUnit(ctx, s.tiDB, callingUser.(*models.User), unitID, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "DeleteJourneyUnit core failed", r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-journey-unit",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyUnitAttempt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-unit-attempt-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	userSession := r.Context().Value("userSession")

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateJourneyUnitAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
	userName = callingUser.(*models.User).UserName
	callingIdInt = callingUser.(*models.User).ID

	if userSession == nil {
		s.handleError(w, "user session missing from context", r.URL.Path, "CreateJourneyUnitAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateJourneyUnitAttempt", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	title, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "title", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if title == nil || !ok {
		return
	}

	unitFocusT, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "unit_focus", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitFocusT == nil || !ok {
		return
	}
	unitFocus := models.UnitFocus(int(unitFocusT.(float64)))

	// attempt to load parameter from body
	description, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tier, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "tier", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if tier == nil || !ok {
		return
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "languages", reflect.Slice, &langsType, false, callingUser.(*models.User).UserName, callingId)
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
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
	if tagsI == nil || !ok {
		return
	}

	// create a slice to hold tags loaded from the http parameter
	tags := make([]string, 0)

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, tagI := range tagsI.([]interface{}) {
		tag := tagI.(map[string]interface{})

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateJourneyUnitAttempt", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateJourneyUnitAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag value type - should be string", nil,
			)
			return
		}

		tags = append(tags, val)
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "workspace_config_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if id == nil || !ok {
		return
	}

	// attempt to load parameter from body
	workspaceConfigRevisionS, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "workspace_config_revision", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspaceConfigRevisionS == nil || !ok {
		return
	}

	workspaceConfigRevision, err := strconv.ParseInt(workspaceConfigRevisionS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse workspace config revision as int64", r.URL.Path, "CreateJourneyUnit", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	workspaceConfigIDS, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "workspace_config_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspaceConfigIDS == nil || !ok {
		return
	}

	workspaceConfigID, err := strconv.ParseInt(workspaceConfigIDS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse workspace config id as int64", r.URL.Path, "CreateJourneyUnit", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	repoIDS, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "repo_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if repoIDS == nil || !ok {
		return
	}

	repoID, err := strconv.ParseInt(repoIDS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse repo id as int64", r.URL.Path, "CreateJourneyUnitAttempt", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	parentIDS, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "parent_unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if parentIDS == nil || !ok {
		return
	}

	parentUnit, err := strconv.ParseInt(parentIDS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse parent unit id as int64", r.URL.Path, "CreateJourneyUnitAttempt", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "workspace_settings", reflect.Map, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.WorkspaceSettings

	// conditionally attempt to marshall and unmarshall the workspace settings
	if workspaceSettingsI != nil {
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to marshal workspace settings: %s", err), r.URL.Path, "CreateJourneyUnitAttempt", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to unmarshal workspace settings: %s", err), r.URL.Path, "CreateJourneyUnitAttempt", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// ensure the validity of the settings
		if workspaceSettings.AutoGit.CommitMessage == "" {
			s.handleError(w, fmt.Sprintf("invalid commit message for workspace settings: %s", workspaceSettings.AutoGit.CommitMessage), r.URL.Path, "CreateJourneyUnitAttempt", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid commit message", nil)
			return
		}
	}

	// attempt to load parameter from body
	visibilityI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnitAttempt", "project_visibility", reflect.Float64, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}
	visibility := models.PublicVisibility
	if visibilityI != nil {
		visibility = models.PostVisibility(visibilityI.(float64))
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyUnitAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateJourneyUnitAttempt(ctx, s.tiDB, s.vscClient, userSession.(*models.UserSession), s.sf, callingUser.(*models.User), parentUnit, title.(string), unitFocus,
		languages, description.(string), repoID, tags, models.TierType(int(tier.(float64))), workspaceConfigID, visibility, int(workspaceConfigRevision), workspaceSettings,
		nil)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "CreateJourneyUnitAttempt core failed", r.URL.Path, "CreateJourneyUnitAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-journey-unit-attempt",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyUnitAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyProject(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-project-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		if callingUser.(*models.User).AuthRole != models.Admin {
			s.handleError(w, fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID), r.URL.Path, "CreateJourneyProject", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "naughty naughty, you shouldn't be here nerd!", errors.New(fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID)))
			return
		}

		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateJourneyProject", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	title, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "title", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if title == nil || !ok {
		return
	}

	// attempt to load parameter from body
	description, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tier, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "tier", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if tier == nil || !ok {
		return
	}

	languageS, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "languages", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if languageS == nil || !ok {
		return
	}

	language := models.ProgrammingLanguageFromString(languageS.(string))

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
	if tagsI == nil || !ok {
		return
	}

	// create a slice to hold tags loaded from the http parameter
	tags := make([]string, 0)

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, tagI := range tagsI.([]interface{}) {
		tag := tagI.(map[string]interface{})

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateJourneyProject", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateJourneyProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag value type - should be string", nil,
			)
			return
		}

		tags = append(tags, val)
	}

	// attempt to load parameter from body
	unitIds, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIds == nil || !ok {
		return
	}

	unitID, err := strconv.ParseInt(unitIds.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse unit id as int64", r.URL.Path, "CreateJourneyProject", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	workingDirectory, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "working_directory", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	if workingDirectory == nil {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	dependencies := make([]int64, 0)

	// attempt to load parameter from body
	depType := reflect.String
	depsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyProject", "deps", reflect.Slice, &depType, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	if depsI != nil {
		// iterate through tagsI asserting each value as a map and create a new tag
		for _, deps := range depsI.([]interface{}) {

			dep, err := strconv.ParseInt(deps.(string), 10, 64)
			if err != nil {
				s.handleError(w, "failed to parse dependency id as int64", r.URL.Path, "CreateJourneyProject", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
					"internal server error occurred", err)
				return
			}

			dependencies = append(dependencies, dep)
		}
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateJourneyProject(ctx, s.tiDB, s.sf, callingUser.(*models.User), unitID, workingDirectory.(string), title.(string), description.(string),
		language, tags, dependencies, nil)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "CreateJourneyProject core failed", r.URL.Path, "CreateJourneyProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-journey-project",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) DeleteJourneyUnitProject(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-journey-unit-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		if callingUser.(*models.User).AuthRole != models.Admin {
			s.handleError(w, fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID), r.URL.Path, "DeleteJourneyUnitProject", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "naughty naughty, you shouldn't be here nerd!", errors.New(fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID)))
			return
		}

		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteJourneyUnitProject", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	unitIDS, ok := s.loadValue(w, r, reqJson, "DeleteJourneyUnitProject", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIDS == nil || !ok {
		return
	}

	unitID, err := strconv.ParseInt(unitIDS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse unit id as int64", r.URL.Path, "CreateJourneyUnit", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	projectIDS, ok := s.loadValue(w, r, reqJson, "DeleteJourneyUnitProject", "project_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if projectIDS == nil || !ok {
		return
	}

	projectID, err := strconv.ParseInt(unitIDS.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse project id as int64", r.URL.Path, "DeleteJourneyUnitProject", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteJourneyUnitProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteJourneyProject(ctx, s.tiDB, callingUser.(*models.User), projectID, unitID, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "DeleteJourneyUnitProject core failed", r.URL.Path, "DeleteJourneyUnitProject", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"delete-journey-unit-project",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteJourneyUnitProject", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateJourneyProjectAttempt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-journey-project-attempt-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		if callingUser.(*models.User).AuthRole != models.Admin {
			s.handleError(w, fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID), r.URL.Path, "CreateJourneyProjectAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "naughty naughty, you shouldn't be here nerd!", errors.New(fmt.Sprintf("incorrect permissions for acessing user: %v", callingUser.(*models.User).ID)))
			return
		}

		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateJourneyProjectAttempt", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	title, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "title", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if title == nil || !ok {
		return
	}

	// attempt to load parameter from body
	description, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tier, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "tier", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if tier == nil || !ok {
		return
	}

	languageS, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "languages", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if languageS == nil || !ok {
		return
	}

	language := models.ProgrammingLanguageFromString(languageS.(string))

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
	if tagsI == nil || !ok {
		return
	}

	// create a slice to hold tags loaded from the http parameter
	tags := make([]string, 0)

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, tagI := range tagsI.([]interface{}) {
		tag := tagI.(map[string]interface{})

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateJourneyProjectAttempt", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateJourneyProjectAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag value type - should be string", nil,
			)
			return
		}

		tags = append(tags, val)
	}

	// attempt to load parameter from body
	unitIds, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "unit_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if unitIds == nil || !ok {
		return
	}

	unitID, err := strconv.ParseInt(unitIds.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse unit id as int64", r.URL.Path, "CreateJourneyProjectAttempt", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	parentIds, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "parent_project_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if parentIds == nil || !ok {
		return
	}

	parentProject, err := strconv.ParseInt(unitIds.(string), 10, 64)
	if !ok {
		s.handleError(w, "failed to parse parent project id as int64", r.URL.Path, "CreateJourneyProjectAttempt", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
			"internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	workingDirectory, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "working_directory", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	if workingDirectory == nil {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyProjectAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	dependencies := make([]int64, 0)

	// attempt to load parameter from body
	depType := reflect.Map
	depsI, ok := s.loadValue(w, r, reqJson, "CreateJourneyProjectAttempt", "tags", reflect.Slice, &depType, false, callingUser.(*models.User).UserName, callingId)
	if depsI == nil || !ok {
		return
	}

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, deps := range depsI.([]interface{}) {

		dep, err := strconv.ParseInt(deps.(string), 10, 64)
		if !ok {
			s.handleError(w, "failed to parse dependency id as int64", r.URL.Path, "CreateJourneyProjectAttempt", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		dependencies = append(dependencies, dep)
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyProjectAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateJourneyProjectAttempt(ctx, s.tiDB, s.sf, callingUser.(*models.User), unitID, parentProject, workingDirectory.(string), title.(string), description.(string),
		language, tags, dependencies, nil)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "CreateJourneyProjectAttempt core failed", r.URL.Path, "CreateJourneyProjectAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-journey-project-attempt",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateJourneyProjectAttempt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}
