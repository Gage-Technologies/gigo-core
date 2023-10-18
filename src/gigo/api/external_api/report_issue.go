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

func (s *HTTPServer) CreateReportIssue(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-rewards-inventory-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateReportIssue", false, callingUser.(*models.User).UserName, -1)
	if reqJson == nil {
		return
	}

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateReportIssue", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingIdInt, err := strconv.ParseInt(callingId, 10, 64)
	if err != nil {
		s.handleError(w, "improper calling id", r.URL.Path, "CreateReportIssue", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateReportIssue", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// attempt to load new username from body
	page, ok := s.loadValue(w, r, reqJson, "CreateReportIssue", "page", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if page == nil || !ok {
		return
	}

	// attempt to load new username from body
	issue, ok := s.loadValue(w, r, reqJson, "CreateReportIssue", "issue", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if issue == nil || !ok {
		return
	}

	// execute core function logic
	res, err := core.CreateReportIssue(ctx, callingIdInt, s.tiDB, page.(string), issue.(string), s.sf)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateReportIssue core failed", r.URL.Path, "CreateReportIssue", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-issue-report",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateReportIssue", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}
