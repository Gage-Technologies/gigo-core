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

func (s *HTTPServer) AddXP(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "add-xp-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "AddXP", false, callingUser.(*models.User).UserName, -1)
	if reqJson == nil {
		return
	}

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "AddXP", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingIdInt, err := strconv.ParseInt(callingId, 10, 64)
	if err != nil {
		s.handleError(w, "improper calling id", r.URL.Path, "AddXP", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load new password from body
	rawSource, ok := s.loadValue(w, r, reqJson, "AddXP", "source", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// load email if it was passed
	var source string
	if rawSource != nil {
		tempSource := rawSource.(string)
		source = tempSource
	}

	// attempt to load new password from body
	rawRenownOfChallenge, ok := s.loadValue(w, r, reqJson, "AddXP", "renown_of_challenge", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// load email if it was passed
	var renownOfChallenge *models.TierType
	if rawRenownOfChallenge != nil {
		tempRenown := rawSource.(string)
		renown, err := models.TierTypeFromString(tempRenown)
		if err != nil {
			renownOfChallenge = nil
		} else {
			renownOfChallenge = &renown
		}
	}

	// attempt to load new password from body
	rawNemesisBasesCaptured, ok := s.loadValue(w, r, reqJson, "AddXP", "nemesis_bases_captured", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	// load email if it was passed
	var nemesisBasesCaptured *int
	if rawNemesisBasesCaptured != nil {
		tempNemesisBasesCaptured := int(rawNemesisBasesCaptured.(float64))
		nemesisBasesCaptured = &tempNemesisBasesCaptured
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "AddXP", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.AddXP(ctx, s.tiDB, s.jetstreamClient, s.rdb, s.sf, callingIdInt, source, renownOfChallenge, nemesisBasesCaptured, s.logger, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "AddXP core failed", r.URL.Path, "AddXP", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"add-xp",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	s.logger.Infof("AddXP: %v", res)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "AddXP", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetUserRewardsInventory(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-rewards-inventory-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserRewardsInventory", false, callingUser.(*models.User).UserName, -1)
	if reqJson == nil {
		return
	}

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetUserRewardsInventory", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingIdInt, err := strconv.ParseInt(callingId, 10, 64)
	if err != nil {
		s.handleError(w, "improper calling id", r.URL.Path, "GetUserRewardsInventory", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetUserRewardsInventory", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetUserRewardsInventory(ctx, s.tiDB, callingIdInt)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetUserRewardsInventory core failed", r.URL.Path, "GetUserRewardsInventory", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-user-rewards-inventory",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetUserRewardsInventory", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) SetUserReward(w http.ResponseWriter, r *http.Request) {

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-user-reward-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SetUserReward", false, callingUser.(*models.User).UserName, -1)
	if reqJson == nil {
		return
	}

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "SetUserReward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingIdInt, err := strconv.ParseInt(callingId, 10, 64)
	if err != nil {
		s.handleError(w, "improper calling id", r.URL.Path, "SetUserReward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load phone from body
	rawRewardId, ok := s.loadValue(w, r, reqJson, "SetUserReward", "reward_id", reflect.String, nil, true, callingUser.(*models.User).UserName, callingId)
	if !ok {
		return
	}

	var rewardId *int64 = nil
	if rawRewardId != nil {
		// parse post ownerCount to integer
		tempAttemptsMin, err := strconv.ParseInt(rawRewardId.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", rawRewardId.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		rewardId = &tempAttemptsMin
	}

	// // parse post id to integer
	// rewardId, err := strconv.ParseInt(rawRewardId.(string), 10, 64)
	// if err != nil {
	//	// handle error internally
	//	s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", rawRewardId.(string)), r.URL.Path, "AttemptInformation", r.Method, r.Context().Value(CtxKeyRequestID),
	//		network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
	//	// exit
	//	return
	// }

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SetUserReward", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	err = core.SetUserReward(ctx, s.tiDB, callingIdInt, rewardId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "SetUserReward core failed", r.URL.Path, "SetUserReward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-user-reward",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, nil, r.URL.Path, "GetUserRewardsInventory", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
