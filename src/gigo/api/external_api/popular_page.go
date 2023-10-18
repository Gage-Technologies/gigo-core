package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"reflect"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) PopularPageFeed(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "popular-page-feed-http")
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
	reqJson := s.jsonRequest(w, r, "PopularPageFeed", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load limit from request
	limit, ok := s.loadValue(w, r, reqJson, "PopularPageFeed", "limit", reflect.Float64, nil, false, userName, userId)
	if limit == nil || !ok {
		return
	}

	// attempt to load skip from request
	skip, ok := s.loadValue(w, r, reqJson, "PopularPageFeed", "skip", reflect.Float64, nil, false, userName, userId)
	if skip == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "PopularPageFeed", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.PopularPageFeed(ctx, int(skip.(float64)), int(limit.(float64)), s.tiDB)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"message": err})
		// handle error internally
		s.handleError(w, "PopularPageFeed core failed", r.URL.Path, "PopularPageFeed", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"popular-page-feed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "PopularPageFeed", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}
