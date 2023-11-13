package external_api

import (
	"encoding/json"
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
