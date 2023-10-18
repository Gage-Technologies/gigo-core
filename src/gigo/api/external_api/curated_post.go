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

func (s *HTTPServer) AddPostToCurated(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "add-post-to-curated-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "AddPostToCurated", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "AddPostToCurated", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load phone from body
	rawPostId, ok := s.loadValue(w, r, reqJson, "AddPostToCurated", "post_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var postID int64
	if rawPostId != nil {
		tempId, err := strconv.ParseInt(rawPostId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid antagonist id", r.URL.Path, "AddPostToCurated",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid antagonist id", nil)
			return
		}
		postID = tempId
	}

	// attempt to load parameter from body
	language, ok := s.loadValue(w, r, reqJson, "AddPostToCurated", "language", reflect.Float64, nil, false, callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID))
	if language == nil || !ok {
		return
	}

	// attempt to load parameter from body
	proficiencyType := reflect.Float64
	proficiencyI, ok := s.loadValue(w, r, reqJson, "AddPostToCurated", "proficiency_type", reflect.Slice, &proficiencyType, false, callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID))
	if proficiencyI == nil || !ok {
		return
	}

	// create a slice to hold languages loaded from the http parameter
	proficiency := make([]models.ProficiencyType, 0)

	// attempt to load parameter from body
	for _, lang := range proficiencyI.([]interface{}) {
		proficiency = append(proficiency, models.ProficiencyType(lang.(float64)))
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "AddPostToCurated", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.AddPostToCurated(ctx, s.tiDB, s.sf, callingUser.(*models.User), postID, proficiency, models.ProgrammingLanguage(language.(float64)))
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "AddPostToCurated core failed", r.URL.Path, "AddPostToCurated", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"add-post-to-curated-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "AddPostToCurated", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) RemoveCuratedPost(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "remove-curated-post-http")
	defer parentSpan.End()

	// Retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// Return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "RemoveCuratedPost", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "RemoveCuratedPost", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// Attempt to load curatedPostID from body
	rawCuratedPostID, ok := s.loadValue(w, r, reqJson, "RemoveCuratedPost", "curated_post_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	var curatedPostID int64
	if rawCuratedPostID != nil {
		id, err := strconv.ParseInt(rawCuratedPostID.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid curated post id", r.URL.Path, "RemoveCuratedPost",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid curated post id", nil)
			return
		}
		curatedPostID = id
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "RemoveCuratedPost", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.RemoveCuratedPost(ctx, s.tiDB, callingUser.(*models.User), curatedPostID)
	if err != nil {
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		s.handleError(w, "RemoveCuratedPost core failed", r.URL.Path, "RemoveCuratedPost", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"remove-curated-post-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "RemoveCuratedPost", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetCuratedPostsForAdmin(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-curated-posts-for-admin-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetCuratedPostsForAdmin", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetCuratedPostsForAdmin", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	proficiencyFilter, ok := s.loadValue(w, r, reqJson, "GetCuratedPostsForAdmin", "proficiency_filter", reflect.Float64, nil, false, "", "")
	if !ok {
		return
	}

	languageFilter, ok := s.loadValue(w, r, reqJson, "GetCuratedPostsForAdmin", "language_filter", reflect.Float64, nil, false, "", "")
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetCuratedPostsForAdmin", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetCuratedPostsAdmin(ctx, s.tiDB, callingUser.(*models.User), models.ProficiencyType(proficiencyFilter.(float64)), models.ProgrammingLanguage(languageFilter.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("Internal sever error : %v", nil)
		// handle error internally
		s.handleError(w, "GetCuratedPostsForAdmin core failed", r.URL.Path, "GetCuratedPostsForAdmin", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-curated-posts-for-admin-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetCuratedPostsForAdmin", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) CurationAuth(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "curation-auth-http")
	defer parentSpan.End()

	// Retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// Return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CurationAuth", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CurationAuth", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// Attempt to load curatedPostID from body
	password, ok := s.loadValue(w, r, reqJson, "CurationAuth", "password", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CurationAuth", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.CurationAuth(ctx, callingUser.(*models.User), s.curatedSecret, password.(string))
	if err != nil {
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		s.handleError(w, "CurationAuth core failed", r.URL.Path, "CurationAuth", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"curation-auth-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "CurationAuth", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}
