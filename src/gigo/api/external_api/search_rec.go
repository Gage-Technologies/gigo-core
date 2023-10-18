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

func (s *HTTPServer) CompleteSearch(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "complete-search-http")
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
	reqJson := s.jsonRequest(w, r, "CompleteSearch", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	rawSearchRecId, ok := s.loadValue(w, r, reqJson, "CompleteSearch", "search_rec_id", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var searchRecId *int64 = nil
	if rawSearchRecId != nil {
		// parse post ownerCount to integer
		tempSearchRec, err := strconv.ParseInt(rawSearchRecId.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse search rec id string to integer: %s", rawSearchRecId.(string)), r.URL.Path, "CompleteSearch", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		searchRecId = &tempSearchRec
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "CompleteSearch", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load video id from body
	postId, ok := s.loadValue(w, r, reqJson, "CompleteSearch", "post_id", reflect.String, nil, false, userName, userId)
	if postId == nil || !ok {
		return
	}

	postID, err := strconv.ParseInt(postId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", postId.(string)), r.URL.Path, "EditDiscussions", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CompleteSearch", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}
	// execute core function logic
	err = core.CompleteSearch(ctx, s.tiDB, s.sf, userIdInt, searchRecId, postID, query.(string), s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "CompleteSearch core failed", r.URL.Path, "CompleteSearch", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"complete-search",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CompleteSearch", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}
