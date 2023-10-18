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

func (s *HTTPServer) GetDiscussions(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-discussions-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := ""
	callingUsername := ""
	var callingIdInt int64
	var callingUserModel *models.User
	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		callingId = network.GetRequestIP(r)
		callingUsername = network.GetRequestIP(r)
	} else {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		callingUsername = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
		callingUserModel = callingUser.(*models.User)
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetDiscussions", false, callingUsername, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "ProjectPageFrontend", "post_id", reflect.String, nil, false, callingUsername, callingId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectPageFrontend", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "GetDiscussions", "skip", reflect.Float64, nil, false, callingUsername, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "GetDiscussions", "limit", reflect.Float64, nil, false, callingUsername, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetDiscussions(ctx, s.tiDB, callingUserModel, postId, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetDiscussions core failed", r.URL.Path, "GetDiscussions", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-discussions",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetDiscussions", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
}

func (s *HTTPServer) GetDiscussionComments(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-discussion-comments-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingUsername := ""
	callingId := ""
	var callingIdInt int64
	var callingUserModel *models.User

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		callingId = network.GetRequestIP(r)
		callingUsername = network.GetRequestIP(r)
	} else {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		callingUsername = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
		callingUserModel = callingUser.(*models.User)
	}

	// // return if calling user was not retrieved in authentication
	// if callingUser == nil {
	//	s.handleError(w, "calling user missing from context", r.URL.Path, "GetDiscussionComments", r.Method, r.Context().Value(CtxKeyRequestID),
	//		network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
	//	return
	// }

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetDiscussionComments", false, callingUsername, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	discIdType := reflect.String
	id, ok := s.loadValue(w, r, reqJson, "GetDiscussionComments", "discussion_id", reflect.Slice, &discIdType, false, callingUsername, callingId)
	if id == nil || !ok {
		return
	}

	// create array to hold discussion ids
	discussionIds := make([]int64, 0)

	// iterate discussion ids interface slice asserting each value as a string and parsing to an int64
	for _, discussionId := range id.([]interface{}) {
		// parse post id to integer
		dId, err := strconv.ParseInt(discussionId.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "ProjectPageFrontend", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		// append to outer slice
		discussionIds = append(discussionIds, dId)
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "GetDiscussionComments", "skip", reflect.Float64, nil, false, callingUsername, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "GetDiscussionComments", "limit", reflect.Float64, nil, false, callingUsername, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetDiscussionComments(ctx, s.tiDB, callingUserModel, discussionIds, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetDiscussionComments core failed", r.URL.Path, "GetDiscussionCommentLeads", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-discussion-comments",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUsername),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetDiscussionComments", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
}

func (s *HTTPServer) GetCommentThreads(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-comment-threads-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingUsername := ""
	callingId := ""
	var callingIdInt int64
	var callingUserModel *models.User

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		callingId = network.GetRequestIP(r)
		callingUsername = network.GetRequestIP(r)
	} else {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		callingUsername = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
		callingUserModel = callingUser.(*models.User)
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetCommentThreads", false, callingUsername, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	commentIdType := reflect.String
	id, ok := s.loadValue(w, r, reqJson, "GetCommentThreads", "comment_id", reflect.Slice, &commentIdType, false, callingUsername, callingId)
	if id == nil || !ok {
		return
	}

	// create array to hold discussion ids
	commentIds := make([]int64, 0)

	// iterate discussion ids interface slice asserting each value as a string and parsing to an int64
	for _, commentId := range id.([]interface{}) {
		// parse post id to integer
		dId, err := strconv.ParseInt(commentId.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse comment id string to integer: %s", id.(string)), r.URL.Path, "GetCommentThreads", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		// append to outer slice
		commentIds = append(commentIds, dId)
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "GetCommentThreads", "skip", reflect.Float64, nil, false, callingUsername, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "GetCommentThreads", "limit", reflect.Float64, nil, false, callingUsername, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetCommentThreads(ctx, s.tiDB, callingUserModel, commentIds, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetCommentThreads core failed", r.URL.Path, "GetCommentThreads", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-comment-threads",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUsername),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetCommentThreads", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
}

func (s *HTTPServer) GetThreadReply(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-thread-reply-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingUsername := ""
	callingId := ""
	var callingIdInt int64
	var callingUserModel *models.User

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		callingId = network.GetRequestIP(r)
		callingUsername = network.GetRequestIP(r)
	} else {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		callingUsername = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
		callingUserModel = callingUser.(*models.User)
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetThreadReply", false, callingUsername, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	threadIdType := reflect.String
	id, ok := s.loadValue(w, r, reqJson, "GetThreadReply", "thread_id", reflect.Slice, &threadIdType, false, callingUsername, callingId)
	if id == nil || !ok {
		return
	}

	// create array to hold discussion ids
	threadIds := make([]int64, 0)

	// iterate discussion ids interface slice asserting each value as a string and parsing to an int64
	for _, threadId := range id.([]interface{}) {
		// parse post id to integer
		dId, err := strconv.ParseInt(threadId.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse thread id string to integer: %s", id.(string)), r.URL.Path, "GetThreadReply", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		// append to outer slice
		threadIds = append(threadIds, dId)
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "GetThreadReply", "skip", reflect.Float64, nil, false, callingUsername, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "GetThreadReply", "limit", reflect.Float64, nil, false, callingUsername, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetThreadReply(ctx, s.tiDB, callingUserModel, threadIds, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetThreadReply core failed", r.URL.Path, "GetThreadReply", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-thread-reply",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUsername),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetThreadReply", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateDiscussion(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-discussion-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateDiscussion", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "CreateDiscussion", "post_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	postId, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load repo id from body
	title, ok := s.loadValue(w, r, reqJson, "CreateDiscussion", "title", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if title == nil || !ok {
		return
	}

	// attempt to load repo id from body
	body, ok := s.loadValue(w, r, reqJson, "CreateDiscussion", "body", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if body == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateDiscussion", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
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
			s.handleError(w, "missing tag id", r.URL.Path, "CreateDiscussion", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", fmt.Errorf("missing tag id"))
			return
		}

		// assert id as string and parse into int64
		idS, ok := idI.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag id type: %v", reflect.TypeOf(idS)), r.URL.Path,
				"CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag id type - should be string", nil)
			return
		}

		id, err := strconv.ParseInt(idS, 10, 64)
		if !ok {
			s.handleError(w, "failed to parse tag id as int64", r.URL.Path, "CreateDiscussion", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateDiscussion", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
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
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateDiscussion(ctx, s.tiDB, s.meili, callingUser.(*models.User), s.sf, postId, title.(string), body.(string), tags)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateDiscussion core failed", r.URL.Path, "CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-discussion",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateComment(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-comment-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateComment", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateComment", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "CreateComment", "discussion_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	discussionId, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "CreateComment", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load repo id from body
	body, ok := s.loadValue(w, r, reqJson, "CreateComment", "body", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if body == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateComment(ctx, s.tiDB, s.meili, callingUser.(*models.User), s.sf, discussionId, body.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateComment core failed", r.URL.Path, "CreateComment", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-comment",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateComment", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateThreadComment(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-thread-comment-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateThreadComment", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateThreadComment", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "CreateThreadComment", "comment_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	commentId, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "CreateThreadComment", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load repo id from body
	body, ok := s.loadValue(w, r, reqJson, "CreateThreadComment", "body", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if body == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateThreadComment(ctx, s.tiDB, s.meili, callingUser.(*models.User), s.sf, commentId, body.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateThreadComment core failed", r.URL.Path, "CreateThreadComment", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-thread-comment",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateThreadComment", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CreateThreadReply(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-thread-reply-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateThreadReply", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateThreadReply", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "CreateThreadReply", "thread_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	threadId, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "CreateThreadReply", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load repo id from body
	body, ok := s.loadValue(w, r, reqJson, "CreateThreadReply", "body", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if body == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateThreadReply(ctx, s.tiDB, callingUser.(*models.User), s.sf, threadId, body.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateThreadReply core failed", r.URL.Path, "CreateThreadReply", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-thread-reply",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateThreadReply", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) EditDiscussions(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "edit-discussions-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "EditDiscussions", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "EditDiscussions", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "EditDiscussions", "_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	mainId, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "EditDiscussions", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load repo id from body
	discussionType, ok := s.loadValue(w, r, reqJson, "EditDiscussions", "discussion_type", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if discussionType == nil || !ok {
		return
	}

	// attempt to load repo id from body
	rawTitle, ok := s.loadValue(w, r, reqJson, "EditDiscussions", "title", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if rawTitle == nil || !ok {
		return
	}

	// load title if it was passed
	var title *string = nil
	if rawTitle != nil {
		tempTitle := rawTitle.(string)
		title = &tempTitle
	}

	// attempt to load repo id from body
	body, ok := s.loadValue(w, r, reqJson, "EditDiscussions", "body", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if body == nil || !ok {
		return
	}

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "CreateDiscussion", "tags", reflect.Slice, &tagsType, false, callingUser.(*models.User).UserName, callingId)
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
			s.handleError(w, "missing tag id", r.URL.Path, "CreateDiscussion", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", fmt.Errorf("missing tag id"))
			return
		}

		// assert id as string and parse into int64
		idS, ok := idI.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag id type: %v", reflect.TypeOf(idS)), r.URL.Path,
				"CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"invalid tag id type - should be string", nil)
			return
		}

		id, err := strconv.ParseInt(idS, 10, 64)
		if !ok {
			s.handleError(w, "failed to parse tag id as int64", r.URL.Path, "CreateDiscussion", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// load value from tag map
		valS, ok := tag["value"]
		if !ok {
			s.handleError(w, "missing tag value", r.URL.Path, "CreateDiscussion", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				callingUser.(*models.User).UserName, callingId, http.StatusUnprocessableEntity,
				"internal server error occurred", nil)
			return
		}

		// assert val as string
		val, ok := valS.(string)
		if !ok {
			s.handleError(w, fmt.Sprintf("invalid tag value type: %v", reflect.TypeOf(valS)), r.URL.Path,
				"CreateDiscussion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
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
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.EditDiscussions(ctx, s.tiDB, callingUser.(*models.User), s.meili, s.sf, discussionType.(string), mainId, title, body.(string), tags)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "EditDiscussions core failed", r.URL.Path, "EditDiscussions", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"edit-discussions",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "EditDiscussions-", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) AddDiscussionCoffee(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "add-discussion-coffee-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "AddDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "AddDiscussionCoffee", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "AddDiscussionCoffee", "_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	id, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "AddDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load discussion type from body
	discussionType, ok := s.loadValue(w, r, reqJson, "AddDiscussionCoffee", "discussion_type", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if discussionType == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.AddDiscussionCoffee(ctx, s.tiDB, callingUser.(*models.User), s.sf, id, discussionType.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "AddDiscussionCoffee core failed", r.URL.Path, "AddDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"add-discussion-coffee",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "AddDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) RemoveDiscussionCoffee(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "remove-discussion-coffee-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "RemoveDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "RemoveDiscussionCoffee", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	tempId, ok := s.loadValue(w, r, reqJson, "RemoveDiscussionCoffee", "_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if tempId == nil || !ok {
		return
	}

	// parse post id to integer
	id, err := strconv.ParseInt(tempId.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", tempId.(string)), r.URL.Path, "RemoveDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load discussion type from body
	discussionType, ok := s.loadValue(w, r, reqJson, "RemoveDiscussionCoffee", "discussion_type", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if discussionType == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.RemoveDiscussionCoffee(ctx, s.tiDB, callingUser.(*models.User), id, discussionType.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "RemoveDiscussionCoffee core failed", r.URL.Path, "RemoveDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"remove-discussion-coffee",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "RemoveDiscussionCoffee", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
