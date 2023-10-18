/*
 *
 *  *  *********************************************************************************
 *  *   GAGE TECHNOLOGIES CONFIDENTIAL
 *  *   __________________
 *  *
 *  *    Gage Technologies
 *  *    Copyright (c) 2021
 *  *    All Rights Reserved.
 *  *
 *  *   NOTICE:  All information contained herein is, and remains
 *  *   the property of Gage Technologies and its suppliers,
 *  *   if any.  The intellectual and technical concepts contained
 *  *   herein are proprietary to Gage Technologies
 *  *   and its suppliers and may be covered by U.S. and Foreign Patents,
 *  *   patents in process, and are protected by trade secret or copyright law.
 *  *   Dissemination of this information or reproduction of this material
 *  *   is strictly forbidden unless prior written permission is obtained
 *  *   from Gage Technologies.
 *
 *
 */

package external_api

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

func (s *HTTPServer) CreateEphemeral(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-new-user-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "VerifyCaptcha", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	challengeIdI, ok := s.loadValue(w, r, reqJson, "CreateEphemeral", "challenge_id", reflect.String, nil, false, "", "")
	if challengeIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	challengeID, err := strconv.ParseInt(challengeIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse challenege id string to integer: %s", challengeIdI.(string)), r.URL.Path, "CreateEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), network.GetRequestIP(r), network.GetRequestIP(r), http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	ipString := network.GetRequestIP(r)
	ip := net.ParseIP(ipString)
	if ip == nil {
		s.handleError(w, fmt.Sprintf("failed to parse ip address: %s", ipString), r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), network.GetRequestIP(r), network.GetRequestIP(r), http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	ipInt := binary.BigEndian.Uint32(ip.To4())

	// attempt to load workspacePath from body
	workspacePath, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "commit", reflect.String, nil, false, network.GetRequestIP(r), network.GetRequestIP(r))
	if workspacePath == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateEphemeral(ctx, s.tiDB, s.storageEngine, s.meili, s.sf, s.domain, s.vscClient, s.masterKey, s.jetstreamClient, s.wsStatusUpdater, s.rdb, challengeID, int64(ipInt), workspacePath.(string), s.accessUrl.String(), s.hostname, s.useTls, network.GetRequestIP(r), s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateEphemeral core failed", r.URL.Path, "CreateEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	s.logger.Debugf("CreateEphemeral response: %v", res)

	if res["message"] == "ephemeral system has been used on this network before" {
		s.jsonResponse(r, w, res, r.URL.Path, "CreateEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gigoTempToken",
		Value:    res["token"].(string),
		Expires:  time.Now().Add(time.Hour * 24),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		Domain:   fmt.Sprintf(".%s", s.domain),
	})

	parentSpan.AddEvent(
		"create-new-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) CreateAccountFromEphemeral(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-new-user-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.jsonResponse(r, w, map[string]interface{}{"response": "not logged in"}, r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	if !callingUser.(models.User).IsEphemeral {
		s.jsonResponse(r, w, map[string]interface{}{"response": "this feature is not available for your user"}, r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateAccountFromEphemeral", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	userName, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "user_name", reflect.String, nil, false, "", "")
	if userName == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load email from body
	email, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "email", reflect.String, nil, false, "", "")
	if email == nil || !ok {
		return
	}

	// attempt to load phone from body
	phone, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "phone", reflect.String, nil, false, "", "")
	if phone == nil || !ok {
		return
	}

	// attempt to load pfpPath from body
	firstName, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "first_name", reflect.String, nil, false, "", "")
	if firstName == nil || !ok {
		return
	}

	// attempt to load pfpPath from body
	lastName, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "last_name", reflect.String, nil, false, "", "")
	if lastName == nil || !ok {
		return
	}

	// attempt to load bio from body
	bio, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "bio", reflect.String, nil, false, "", "")
	if bio == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	userInitFormI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "start_user_info", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var userInitForm models.UserStart

	// conditionally attempt to marshall and unmarshall the user init form
	buf, err := json.Marshal(userInitFormI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(buf)), r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(buf, &userInitForm)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(buf)), r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "upload_id", reflect.String, nil, false, "", "")
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "avatar_settings", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(buf)), r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(buf)), r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load key from body
	referralI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "referral_user", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// create variable for content and conditionally load from interface
	var referral *string
	if referralI != nil {
		c := referralI.(string)
		referral = &c
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// check if user wants to force their password or not
	forcePass, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeral", "force_pass", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}

	//user := &models.User{}

	res, err := core.CreateAccountFromEphemeral(ctx, s.tiDB, s.meili, s.streakEngine, s.domain, userName.(string), password.(string),
		email.(string), phone.(string), bio.(string), firstName.(string), lastName.(string),
		s.vscClient, userInitForm, timeZoneI.(string), thumbnailTempPath, s.storageEngine, avatarSetting,
		s.passwordFilter, forcePass.(bool), s.initialRecUrl, s.logger, s.mailGunKey, s.mailGunDomain, referral, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateAccountFromEphemeral core failed", r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-account-from-ephemeral",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateAccountFromEphemeral", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)

}

func (s *HTTPServer) CreateAccountFromEphemeralGoogle(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-account-from-ephemeral-google-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.jsonResponse(r, w, map[string]interface{}{"response": "not logged in"}, r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	if !callingUser.(models.User).IsEphemeral {
		s.jsonResponse(r, w, map[string]interface{}{"response": "this feature is not available for your user"}, r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	reqJson := s.receiveUpload(w, r, "CreateAccountFromEphemeralGoogle", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load bio from body
	externalAuth, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "external_auth", reflect.String, nil, false, "", "")
	if externalAuth == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "start_user_info", reflect.Map, nil, true, "", "")
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.UserStart

	if workspaceSettingsI != nil {
		// conditionally attempt to marshall and unmarshall the workspace settings
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
	}

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "avatar_settings", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(bufs)), r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(bufs)), r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "upload_id", reflect.String, nil, false, "", "")
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// attempt to load key from body
	referralI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGoogle", "referral_user", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// create variable for content and conditionally load from interface
	var referral *string
	if referralI != nil {
		c := referralI.(string)
		referral = &c
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateAccountFromEphemeralGoogle(ctx, s.tiDB, s.meili, s.sf, s.streakEngine, s.domain, externalAuth.(string),
		password.(string), s.vscClient, *workspaceSettings, timeZoneI.(string), avatarSetting, thumbnailTempPath,
		s.storageEngine, s.mailGunKey, s.mailGunDomain, s.initialRecUrl, referral, callingUser.(*models.User), s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateNewGoogleUser core failed", r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-account-from-ephemeral-google",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateAccountFromEphemeralGoogle", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) CreateAccountFromEphemeralGithub(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-account-from-ephemeral-github-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.jsonResponse(r, w, map[string]interface{}{"response": "not logged in"}, r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	if !callingUser.(models.User).IsEphemeral {
		s.jsonResponse(r, w, map[string]interface{}{"response": "this feature is not available for your user"}, r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	// attempt to load JSON from request body

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateAccountFromEphemeralGithub", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load bio from body
	externalAuth, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "external_auth", reflect.String, nil, false, "", "")
	if externalAuth == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "start_user_info", reflect.Map, nil, true, "", "")
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.UserStart

	if workspaceSettingsI != nil {
		// conditionally attempt to marshall and unmarshall the workspace settings
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
	}

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "avatar_settings", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(bufs)), r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(bufs)), r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "upload_id", reflect.String, nil, false, "", "")
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// attempt to load key from body
	referralI, ok := s.loadValue(w, r, reqJson, "CreateAccountFromEphemeralGithub", "referral_user", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// create variable for content and conditionally load from interface
	var referral *string
	if referralI != nil {
		c := referralI.(string)
		referral = &c
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateAccountFromEphemeralGithub(ctx, s.tiDB, s.meili, s.sf, s.streakEngine, s.domain, externalAuth.(string),
		password.(string), s.vscClient, *workspaceSettings, timeZoneI.(string), avatarSetting, s.githubSecret,
		thumbnailTempPath, s.storageEngine, s.mailGunKey, s.mailGunDomain, s.initialRecUrl, referral, callingUser.(*models.User), s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateNewGithubUser core failed", r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-account-from-ephemeral-github",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateAccountFromEphemeralGithub", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}
