package external_api

import (
	"encoding/json"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

func (s *HTTPServer) CheckUnsubscribeEmail(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "check-unsubscribe-email-http")
	defer parentSpan.End()

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CheckUnsubscribeEmail", false, "", -1)
	if reqJson == nil {
		return
	}

	// Attempt to load email from body
	email, ok := s.loadValue(w, r, reqJson, "CheckUnsubscribeEmail", "email", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// Check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// Return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CheckUnsubscribeEmail", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.CheckUnsubscribeEmail(ctx, s.tiDB, email.(string))
	if err != nil {
		// Handle error internally
		s.handleError(w, "CheckUnsubscribeEmail core failed", r.URL.Path, "CheckUnsubscribeEmail", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	parentSpan.AddEvent(
		"check-unsubscribe-email",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "CheckUnsubscribeEmail", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) UpdateEmailPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "update-email-preferences-http")
	defer parentSpan.End()

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UpdateEmailPreferences", false, "", -1)
	if reqJson == nil {
		return
	}

	// Attempt to load user ID and email preferences from body
	userID, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "userID", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// Convert userID to int64
	userIDInt64, err := strconv.ParseInt(userID.(string), 10, 64)
	if err != nil {
		s.handleError(w, "Invalid user ID format", r.URL.Path, "UpdateEmailPreferences", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusBadRequest, "invalid user ID", err)
		return
	}

	// Load all boolean preferences with proper checking
	allEmails, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "allEmails", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	streak, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "streak", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	pro, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "pro", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	newsletter, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "newsletter", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	inactivity, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "inactivity", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	messages, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "messages", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	referrals, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "referrals", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}
	promotional, ok := s.loadValue(w, r, reqJson, "UpdateEmailPreferences", "promotional", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}

	// Execute core function logic
	err = core.UpdateEmailPreferences(ctx, s.tiDB, userIDInt64, allEmails.(bool), streak.(bool), pro.(bool), newsletter.(bool), inactivity.(bool), messages.(bool), referrals.(bool), promotional.(bool))
	if err != nil {
		// Handle error internally
		s.handleError(w, "UpdateEmailPreferences core failed", r.URL.Path, "UpdateEmailPreferences", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	parentSpan.AddEvent(
		"update-email-preferences",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// Return success response
	s.jsonResponse(r, w, map[string]interface{}{"success": true}, r.URL.Path, "UpdateEmailPreferences", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) GetUserEmailPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-email-preferences-http")
	defer parentSpan.End()

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserEmailPreferences", false, "", -1)
	if reqJson == nil {
		return
	}

	// Attempt to load user ID from body
	userID, ok := s.loadValue(w, r, reqJson, "GetUserEmailPreferences", "userID", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// Convert userID to int64
	userIDInt64, err := strconv.ParseInt(userID.(string), 10, 64)
	if err != nil {
		s.handleError(w, "Invalid user ID format", r.URL.Path, "GetUserEmailPreferences", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusBadRequest, "invalid user ID", err)
		return
	}

	// Execute core function logic
	preferences, err := core.GetUserEmailPreferences(ctx, s.tiDB, userIDInt64)
	if err != nil {
		// Handle error internally
		s.handleError(w, "GetUserEmailPreferences core failed", r.URL.Path, "GetUserEmailPreferences", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	parentSpan.AddEvent(
		"get-user-email-preferences",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// Return response
	s.jsonResponse(r, w, preferences, r.URL.Path, "GetUserEmailPreferences", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) InitializeNewMailingListInBulk(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "initialize-new-mailing-list-in-bulk-http")
	defer parentSpan.End()

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "InitializeNewMailingListInBulk", false, "", -1)
	if reqJson == nil {
		return
	}

	// Attempt to load mailing list name from body
	mailingList, ok := s.loadValue(w, r, reqJson, "InitializeNewMailingListInBulk", "mailingList", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "InitializeNewMailingListInBulk", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// Execute core function logic
	err := core.InitializeNewMailingListInBulk(ctx, s.tiDB, s.mailGunKey, s.mailGunDomain, mailingList.(string))
	if err != nil {
		// Handle error internally
		s.handleError(w, "InitializeNewMailingListInBulk core failed", r.URL.Path, "InitializeNewMailingListInBulk", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	parentSpan.AddEvent(
		"initialize-new-mailing-list-in-bulk-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// Return success response
	s.jsonResponse(r, w, map[string]interface{}{"success": true}, r.URL.Path, "InitializeNewMailingListInBulk", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}
