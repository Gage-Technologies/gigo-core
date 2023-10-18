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

func (s *HTTPServer) SendFriendRequest(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "send-friend-request-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "SendFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SendFriendRequest", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load phone from body
	rawFriendId, ok := s.loadValue(w, r, reqJson, "SendFriendRequest", "friend_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var friendID int64
	if rawFriendId != nil {
		tempId, err := strconv.ParseInt(rawFriendId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid antagonist id", r.URL.Path, "SendFriendRequest",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid antagonist id", nil)
			return
		}
		friendID = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SendFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SendFriendRequest(ctx, s.tiDB, s.sf, s.jetstreamClient, callingUser.(*models.User), friendID)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "send freind request core failed", r.URL.Path, "SendFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"send-friend-request",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SendFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) AcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "accept-friend-request-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "AcceptFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "AcceptFriendRequest", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load phone from body
	rawRequesterId, ok := s.loadValue(w, r, reqJson, "AcceptFriendRequest", "requester_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var requesterID int64
	if rawRequesterId != nil {
		tempId, err := strconv.ParseInt(rawRequesterId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid antagonist id", r.URL.Path, "AcceptFriendRequest",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid antagonist id", nil)
			return
		}
		requesterID = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "AcceptFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.AcceptFriendRequest(ctx, s.tiDB, s.sf, s.jetstreamClient, callingUser.(*models.User), requesterID)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "accept friend request core failed", r.URL.Path, "AcceptFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"accept-friend-request",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "AcceptFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) DeclineFriendRequest(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "decline-friend-request-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeclineFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeclineFriendRequest", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load phone from body
	rawRequesterId, ok := s.loadValue(w, r, reqJson, "DeclineFriendRequest", "requester_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var requesterID int64
	if rawRequesterId != nil {
		tempId, err := strconv.ParseInt(rawRequesterId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid request id", r.URL.Path, "DeclineFriendRequest",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid request id", nil)
			return
		}
		requesterID = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeclineFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeclineFriendRequest(ctx, s.tiDB, s.sf, s.jetstreamClient, callingUser.(*models.User), requesterID)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "decline friend request core failed", r.URL.Path, "DeclineFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"decline-friend-request",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeclineFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetFriendsList(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-friends-list-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetFriendsList", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "calling user nil occurred", nil)
		return
	}

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetFriendsList", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetFriendsList", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetFriendsList(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "get friends list core failed", r.URL.Path, "GetFriendsList", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-friends-list",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetFriendsList", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetFriendRequests(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-friend-requests-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetFriendRequests", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetFriendRequests", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetFriendRequests", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetFriendRequests(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "get friends requests core failed", r.URL.Path, "GetFriendRequests", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-friend-requests",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetFriendRequests", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) CheckFriend(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "check-friend-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CheckFriend", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CheckFriend", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load phone from body
	rawRequesterId, ok := s.loadValue(w, r, reqJson, "CheckFriend", "profile_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var requesterID int64
	if rawRequesterId != nil {
		tempId, err := strconv.ParseInt(rawRequesterId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid antagonist id", r.URL.Path, "CheckFriend",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid antagonist id", nil)
			return
		}
		requesterID = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CheckFriend", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CheckFriend(ctx, s.tiDB, callingUser.(*models.User), requesterID)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "decline friend request core failed", r.URL.Path, "CheckFriend", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"check-friend",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CheckFriend", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) CheckFriendRequest(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "check-friend-request-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CheckFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CheckFriendRequest", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load id from body
	rawUserId, ok := s.loadValue(w, r, reqJson, "CheckFriendRequest", "user_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load userID if it was passed
	var userID int64
	if rawUserId != nil {
		tempId, err := strconv.ParseInt(rawUserId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid userID id", r.URL.Path, "CheckFriendRequest",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid userID id", nil)
			return
		}
		userID = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CheckFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CheckFriendRequest(ctx, s.tiDB, callingUser.(*models.User), userID)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "decline friend request core failed", r.URL.Path, "CheckFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"check-friend-request",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CheckFriendRequest", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}
