package external_api

import (
	"fmt"
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

func (s *HTTPServer) CreateWorkspaceConfig(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-workspace-config-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateWorkspaceConfig", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load key from body
	title, ok := s.loadValue(w, r, reqJson, "CreateWorkspaceConfig", "title", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if title == nil || !ok {
		return
	}

	// attempt to load key from body
	description, ok := s.loadValue(w, r, reqJson, "CreateWorkspaceConfig", "description", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if description == nil || !ok {
		return
	}

	// attempt to load key from body
	content, ok := s.loadValue(w, r, reqJson, "CreateWorkspaceConfig", "content", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if content == nil || !ok {
		return
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "CreateWorkspaceConfig", "languages", reflect.Slice, &langsType, false, callingUser.(*models.User).UserName, callingId)
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
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateWorkspaceConfig", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
	if tagsI == nil || !ok {
		return
	}

	// create a slice to hold tags loaded from the http parameter
	tags := make([]*models.Tag, 0)

	// iterate through tagsI asserting each value as a map and create a new tag
	for _, tagI := range tagsI.([]interface{}) {
		tag := tagI.(map[string]interface{})

		// load id from tag map
		idI, ok := tag["id"]
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

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateWorkspaceConfig(ctx,
		s.tiDB, s.meili, s.sf, callingUser.(*models.User), title.(string), description.(string), content.(string),
		tags, languages,
	)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateWorkspaceConfig core failed", r.URL.Path, "CreateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-workspace-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UpdateWorkspaceConfig(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "update-workspace-config-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UpdateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UpdateWorkspaceConfig", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load workspace config id from body
	workspaceConfigIdI, ok := s.loadValue(w, r, reqJson, "UpdateWorkspaceConfig", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspaceConfigIdI == nil || !ok {
		return
	}

	// parse workspace config id to integer
	workspaceConfigId, err := strconv.ParseInt(workspaceConfigIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace config id string to integer: %s", workspaceConfigIdI.(string)), r.URL.Path, "UpdateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load key from body
	descriptionI, ok := s.loadValue(w, r, reqJson, "UpdateWorkspaceConfig", "description", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable for description and conditionally load from interface
	var description *string
	if descriptionI != nil {
		c := descriptionI.(string)
		description = &c
	}

	// attempt to load key from body
	contentI, ok := s.loadValue(w, r, reqJson, "UpdateWorkspaceConfig", "content", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable for content and conditionally load from interface
	var content *string
	if contentI != nil {
		c := contentI.(string)
		content = &c
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "UpdateWorkspaceConfig", "languages", reflect.Slice, &langsType, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable for languages and conditionally load from interface
	var languages []models.ProgrammingLanguage
	if languagesI != nil {
		languages = make([]models.ProgrammingLanguage, 0)

		// attempt to load parameter from body
		for _, lang := range languagesI.([]interface{}) {
			languages = append(languages, models.ProgrammingLanguage(lang.(float64)))
		}
	}

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "UpdateWorkspaceConfig", "tags", reflect.Slice, &tagsType, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// create variable for tags and conditionally load from interface
	var tags []*models.Tag
	if tagsI != nil {
		tags = make([]*models.Tag, 0)

		// iterate through tagsI asserting each value as a map and create a new tag
		for _, tagI := range tagsI.([]interface{}) {
			tag := tagI.(map[string]interface{})

			// load id from tag map
			idI, ok := tag["id"]
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
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UpdateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UpdateWorkspaceConfig(ctx,
		s.tiDB, s.meili, s.sf, callingUser.(*models.User), workspaceConfigId, description, content, tags, languages,
	)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UpdateWorkspaceConfig core failed", r.URL.Path, "UpdateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"update-workspace-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UpdateWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetWorkspaceConfig(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-workspace-config-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetWorkspaceConfig", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load workspace config id from body
	workspaceConfigIdI, ok := s.loadValue(w, r, reqJson, "GetWorkspaceConfig", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspaceConfigIdI == nil || !ok {
		return
	}

	// parse workspace config id to integer
	workspaceConfigId, err := strconv.ParseInt(workspaceConfigIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace config id string to integer: %s", workspaceConfigIdI.(string)), r.URL.Path, "GetWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetWorkspaceConfig(ctx, s.tiDB, workspaceConfigId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetWorkspaceConfig core failed", r.URL.Path, "GetWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-workspace-config",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetWorkspaceConfig", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
