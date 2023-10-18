package external_api

import (
	"encoding/json"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
)

func (s *HTTPServer) VerifyEmailToken(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "verify-email-token-http")
	defer parentSpan.End()

	// attempt to retrieve target id and token from url
	vars := mux.Vars(r)
	token, tokOk := vars["token"]
	userId, idOk := vars["userId"]

	if !tokOk || !idOk {
		// handle error internally
		s.handleError(w, "no token or user id found in path", r.URL.Path, "VerifyEmailToken", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", userId, http.StatusMethodNotAllowed, "invalid path", nil)
		return
	}

	// execute core function logic
	result, err := core.VerifyEmailToken(ctx, s.tiDB, userId, token)
	if err != nil {
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"errorMessage": err})
		// handle error internally
		s.handleError(w, "VerifyEmailToken core failed", r.URL.Path, "VerifyEmailToken", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// Prepare the response JSON
	jsonResponse, err := json.Marshal(result)
	if err != nil {
		// handle error internally
		s.handleError(w, "json marshal failed", r.URL.Path, "VerifyEmailToken", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", userId, http.StatusInternalServerError, "json marshal failed", err)
		return
	}

	// Set the Content-Type header
	w.Header().Set("Content-Type", "application/json")

	// Write the JSON response to the response
	_, err = w.Write(jsonResponse)
	if err != nil {
		// handle error internally
		s.handleError(w, "write response failed", r.URL.Path, "VerifyEmailToken", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", userId, http.StatusInternalServerError, "write response failed", err)
		return
	}

	parentSpan.AddEvent(
		"verify-email-token",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", ""),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "VerifyEmailToken", r.Method,
		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", userId, http.StatusOK, nil)
}

func (s *HTTPServer) EmailVerification(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "email-verification-http")
	defer parentSpan.End()

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "EmailVerification", false, "", -1)
	if reqJson == nil {
		return
	}

	// Attempt to load curatedPostID from body
	address, ok := s.loadValue(w, r, reqJson, "EmailVerification", "email", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "EmailVerification", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.EmailVerification(ctx, s.mailGunKey, address.(string))
	if err != nil {
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		s.handleError(w, "EmailVerification core failed", r.URL.Path, "EmailVerification", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		return
	}

	parentSpan.AddEvent(
		"email-verification-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", ""),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "EmailVerification", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}
