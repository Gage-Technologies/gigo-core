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

func (s *HTTPServer) DeclareNemesis(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "declare-nemesis-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeclareNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeclareNemesis", false, "", -1)
	if reqJson == nil {
		return
	}

	rawProtagId, ok := s.loadValue(w, r, reqJson, "DeclareNemesis", "protag_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	var protagID int64
	if rawProtagId != nil {
		tempId, err := strconv.ParseInt(rawProtagId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "DeclareNemesis",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		protagID = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeclareNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeclareNemesis(ctx, s.tiDB, s.sf, s.jetstreamClient, callingUser.(*models.User).ID, protagID)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "DeclareNemesis core failed", r.URL.Path, "DeclareNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"declare-nemesis",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeclareNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) AcceptNemesis(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "accept-nemesis-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "AcceptNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "AcceptNemesis", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load new password from body
	rawAntagId, ok := s.loadValue(w, r, reqJson, "AcceptNemesis", "antagonist_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load id if it was passed
	var antagId int64
	if rawAntagId != nil {
		tempId, err := strconv.ParseInt(rawAntagId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "AcceptNemesis",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		antagId = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "AcceptNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	err := core.AcceptNemesis(ctx, s.tiDB, s.sf, s.jetstreamClient, callingUser.(*models.User).ID, antagId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "AcceptNemesis core failed", r.URL.Path, "AcceptNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"accept-nemesis",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, nil, r.URL.Path, "AcceptNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) DeclineNemesis(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "decline-nemesis-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeclineNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeclineNemesis", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load new password from body
	rawAntagId, ok := s.loadValue(w, r, reqJson, "DeclineNemesis", "antagonist_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load id if it was passed
	var antagId int64
	if rawAntagId != nil {
		tempId, err := strconv.ParseInt(rawAntagId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "DeclineNemesis",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		antagId = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeclineNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	err := core.DeclineNemesis(ctx, s.tiDB, s.sf, s.jetstreamClient, callingUser.(*models.User).ID, antagId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "DeclineNemesis core failed", r.URL.Path, "DeclineNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"decline-nemesis",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, nil, r.URL.Path, "DeclineNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) GetActiveNemesis(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-active-nemesis-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetActiveNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetActiveNemesis", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetActiveNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetActiveNemesis(ctx, s.tiDB, callingUser.(*models.User).ID)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetActiveNemesis core failed", r.URL.Path, "GetActiveNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-active-nemesis",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetActiveNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

// func (s *HTTPServer) GetPendingNemesisRequests(w http.ResponseWriter, r *http.Request) {
//	// retrieve calling user from context
//	callingUser := r.Context().Value(CtxKeyUser)
//
//	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)
//
//	// return if calling user was not retrieved in authentication
//	if callingUser == nil {
//		s.handleError(w, "calling user missing from context", r.URL.Path, "GetPendingNemesisRequests", r.Method, r.Context().Value(CtxKeyRequestID),
//			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
//		return
//	}
//
//	// attempt to load JSON from request body
//	reqJson := s.jsonRequest(w, r, "GetPendingNemesisRequests", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
//	if reqJson == nil {
//		return
//	}
//	// check if this is a test
//	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
//		// return success for test
//		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetPendingNemesisRequests", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
//		return
//	}
//
//	// execute core function logic
//	res, err := core.GetPendingNemesisRequests(s.tiDB, callingUser.(*models.User).ID)
//	if err != nil {
//		// select error message dependent on if there was one returned from the function
//		responseMessage := selectErrorResponse("internal server error occurred", res)
//		// handle error internally
//		s.handleError(w, "GetPendingNemesisRequests core failed", r.URL.Path, "GetPendingNemesisRequests", r.Method, r.Context().Value(CtxKeyRequestID),
//			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
//		// exit
//		return
//	}
//
//	// return response
//	s.jsonResponse(r, w, res, r.URL.Path, "GetPendingNemesisRequests", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
// }

func (s *HTTPServer) GetNemesisBattlegrounds(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-nemesis-battleground-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetNemesisBattlegrounds", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load phone from body
	rawMatchId, ok := s.loadValue(w, r, reqJson, "GetNemesisBattlegrounds", "match_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load match id if it was passed
	var matchID int64
	if rawMatchId != nil {
		tempId, err := strconv.ParseInt(rawMatchId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "GetNemesisBattlegrounds",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		matchID = tempId
	}

	// attempt to load phone from body
	rawAntagonistId, ok := s.loadValue(w, r, reqJson, "GetNemesisBattlegrounds", "antagonist_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var antagonistId int64
	if rawAntagonistId != nil {
		tempId, err := strconv.ParseInt(rawAntagonistId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid antagonist id", r.URL.Path, "GetNemesisBattlegrounds",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid antagonist id", nil)
			return
		}
		antagonistId = tempId
	}

	// attempt to load phone from body
	rawProtagonistId, ok := s.loadValue(w, r, reqJson, "GetNemesisBattlegrounds", "protagonist_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load email if it was passed
	var protagonistId int64
	if rawProtagonistId != nil {
		tempId, err := strconv.ParseInt(rawProtagonistId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "GetNemesisBattlegrounds",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		protagonistId = tempId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetNemesisBattlegrounds", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetNemesisBattleground(ctx, s.tiDB, matchID, antagonistId, protagonistId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetNemesisBattlegrounds core failed", r.URL.Path, "GetNemesisBattlegrounds", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-nemesis-battleground",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetNemesisBattlegrounds", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) RecentNemesisBattleground(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "recent-nemesis-battleground-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "RecentNemesisBattleground", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "RecentNemesisBattleground", false, "", -1)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "RecentNemesisBattleground", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.RecentNemesisBattleground(ctx, s.tiDB, callingUser.(*models.User).ID)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "RecentNemesisBattleground core failed", r.URL.Path, "RecentNemesisBattleground", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"recent-nemesis-battleground",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "RecentNemesisBattleground", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) WarHistory(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "war-history-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "WarHistory", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WarHistory", false, "", -1)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WarHistory", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.WarHistory(ctx, s.tiDB, callingUser.(*models.User).ID)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "WarHistory core failed", r.URL.Path, "WarHistory", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"war-history",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "WarHistory", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) PendingNemesis(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "pending-nemesis-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "PendingNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "PendingNemesis", false, "", -1)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PendingNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PendingNemesis(ctx, s.tiDB, callingUser.(*models.User).ID)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "PendingNemesis core failed", r.URL.Path, "PendingNemesis", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"pending-nemesis",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PendingNemesis", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) DeclareVictor(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "declare-victor-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeclareVictor", false, "", -1)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeclareVictor", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	matchID, ok := s.loadValue(w, r, reqJson, "DeclareVictor", "match_id", reflect.Float64, nil, false, "", "")
	if !ok {
		return
	}

	rawVictorId, ok := s.loadValue(w, r, reqJson, "DeclareVictor", "victor", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	var victor int64
	if rawVictorId != nil {
		tempId, err := strconv.ParseInt(rawVictorId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "DeclareVictor",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		victor = tempId
	}

	// execute core function logic
	err := core.DeclareVictor(ctx, s.tiDB, int64(matchID.(float64)), victor)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "DeclareVictor core failed", r.URL.Path, "DeclareVictor", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"declare-victor",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, nil, r.URL.Path, "DeclareVictor", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-all-users-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetAllUsers", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetAllUsers", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetAllUsers", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetAllUsers(ctx, s.tiDB, callingUser.(*models.User).ID)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "get friends list core failed", r.URL.Path, "GetAllUsers", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-all-users",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetAllUsers", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetDailyXPGain(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "declare-victor-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetDailyXPGain", false, "", -1)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetDailyXPGain", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// attempt to load phone from body
	rawMatchId, ok := s.loadValue(w, r, reqJson, "GetNemesisBattlegrounds", "match_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// load match id if it was passed
	var matchID int64
	if rawMatchId != nil {
		tempId, err := strconv.ParseInt(rawMatchId.(string), 10, 64)
		if err != nil {
			s.handleError(w, "invalid protagonist id", r.URL.Path, "GetNemesisBattlegrounds",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid protagonist id", nil)
			return
		}
		matchID = tempId
	}

	// execute core function logic
	res, err := core.GetDailyXPGain(ctx, s.tiDB, matchID)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "get nemesis daily xp core failed", r.URL.Path, "GetDailyXPGain", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-daily-xp-gain",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetDailyXPGain", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}
