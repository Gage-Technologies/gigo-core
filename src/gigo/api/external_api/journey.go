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
	journeyInfoI, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "journey_info", reflect.Map, nil, false, "", "")
	if !ok {
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
	workspaceConfigRevision, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "workspace_config_revision", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspaceConfigRevision == nil || !ok {
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
	var unitCost *int64
	// conditionally parse parent attempt from string
	if unitCostI != nil {
		// parse post id to integer
		pa, err := strconv.ParseInt(unitCostI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse parent attempt id string to integer: %s", unitCostI.(string)), r.URL.Path, "StartAttempt", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		unitCost = &pa
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

	// create variable to hold journey info initialization form
	var journeyInfo models.JourneyInfo

	// conditionally attempt to marshall and unmarshall the info init form
	bufs, err := json.Marshal(journeyInfoI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall journey info init form: %s", string(bufs)), r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &journeyInfo)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall journey info init form: %s", string(bufs)), r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	aptitude, ok := s.loadValue(w, r, reqJson, "CreateJourneyUnit", "aptitude_level", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if aptitude == nil || !ok {
		return
	}

	// load the aptitude level into the journey info
	journeyInfo.AptitudeLevel = aptitude.(string)

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateJourneyUnit", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	unitCostString := fmt.Sprintf("%v", unitCost)

	// execute core function logic
	res, err := core.CreateJourneyUnit(ctx, s.tiDB, s.sf, callingUser.(*models.User), title.(string), unitFocus,
		visibility, languages, description.(string), tags, models.TierType(int(tier.(float64))), &unitCostString,
		s.vscClient, workspaceConfigContent, workspaceConfigTitle, workspaceConfigDesc, languages, workspaceSettings,
		nil)
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
