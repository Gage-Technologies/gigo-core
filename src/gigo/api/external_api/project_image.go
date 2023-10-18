package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
)

func (s *HTTPServer) GenerateProjectImage(w http.ResponseWriter, r *http.Request) {

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GenerateProjectImage", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load prompt from request
	prompt, ok := s.loadValue(w, r, reqJson, "GenerateProjectImage", "prompt", reflect.String, nil, false, callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID))
	if prompt == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID), http.StatusOK)
		return
	}

	s.logger.Debugf("GenerateProjectImage Prompt: %v\n Host: %v\n Key: %v", prompt, s.stableDiffusionHost, s.stableDiffusionKey)

	// execute core function logic
	res, err := core.GenerateProjectImage(s.storageEngine, s.rdb, s.sf, callingUser.(*models.User).ID, s.stableDiffusionHost, s.stableDiffusionKey, prompt.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "GenerateProjectImage core failed", r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID), http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID), http.StatusOK)
}
