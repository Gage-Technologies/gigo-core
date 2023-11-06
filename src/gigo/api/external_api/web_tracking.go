package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
)

func (s *HTTPServer) RecordWebUsage(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "record-web-usage-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUserI := r.Context().Value(CtxKeyUser)
	// load calling user values if the caller is logged in
	var callingUser *models.User
	callingUserName := "anon"
	callingUserIdString := "-1"
	if callingUserI != nil {
		callingUser = callingUserI.(*models.User)
		callingUserName = callingUser.UserName
		callingUserIdString = fmt.Sprintf("%v", callingUser.ID)
	}

	// validate the input
	var params core.RecordWebUsageParams
	if !s.validateRequest(w, r, callingUser, r.Body, &params) {
		return
	}

	// add the IP and user id
	params.IP = network.GetRequestIP(r)
	if callingUser != nil {
		params.UserID = &callingUser.ID
	}

	// execute core function logic
	err := core.RecordWebUsage(ctx, s.tiDB, s.sf, &params)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to execute core function", r.URL.Path, "RecordWebUsage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingUserIdString, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// return response
	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "RecordWebUsage", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName, callingUserIdString, http.StatusOK)
}
