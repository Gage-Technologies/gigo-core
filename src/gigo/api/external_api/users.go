package external_api

import (
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	_ "github.com/gage-technologies/gigo-lib/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) CreateNewUser(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-new-user-http")
	defer parentSpan.End()

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateNewUser", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	userName, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "user_name", reflect.String, nil, false, "", "")
	if userName == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load email from body
	email, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "email", reflect.String, nil, false, "", "")
	if email == nil || !ok {
		return
	}

	// attempt to load phone from body
	phone, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "phone", reflect.String, nil, false, "", "")
	if phone == nil || !ok {
		return
	}

	// attempt to load pfpPath from body
	firstName, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "first_name", reflect.String, nil, false, "", "")
	if firstName == nil || !ok {
		return
	}

	// attempt to load pfpPath from body
	lastName, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "last_name", reflect.String, nil, false, "", "")
	if lastName == nil || !ok {
		return
	}

	// attempt to load bio from body
	bio, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "bio", reflect.String, nil, false, "", "")
	if bio == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	userInitFormI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "start_user_info", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var userInitForm models.UserStart

	// conditionally attempt to marshall and unmarshall the user init form
	buf, err := json.Marshal(userInitFormI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(buf)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(buf, &userInitForm)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(buf)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "upload_id", reflect.String, nil, false, "", "")
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "avatar_settings", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(buf)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(buf)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load key from body
	referralI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "referral_user", reflect.String, nil, true, "", "")
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
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// check if user wants to force their password or not
	forcePass, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "force_pass", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}

	// execute core function logic
	res, err := core.CreateNewUser(ctx, s.tiDB, s.meili, s.stripeSubscriptions, s.streakEngine, s.sf, s.domain, userName.(string), password.(string),
		email.(string), phone.(string), bio.(string), firstName.(string), lastName.(string),
		s.vscClient, userInitForm, timeZoneI.(string), thumbnailTempPath, s.storageEngine, avatarSetting,
		s.passwordFilter, forcePass.(bool), s.initialRecUrl, s.logger, s.mailGunKey, s.mailGunDomain, referral)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateNewUser core failed", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-new-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) ValidateUserInfo(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "validate-user-info-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ValidateUserInfo", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	userName, ok := s.loadValue(w, r, reqJson, "ValidateUserInfo", "user_name", reflect.String, nil, false, "", "")
	if userName == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "ValidateUserInfo", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load email from body
	email, ok := s.loadValue(w, r, reqJson, "ValidateUserInfo", "email", reflect.String, nil, false, "", "")
	if email == nil || !ok {
		return
	}

	// attempt to load phone from body
	phone, ok := s.loadValue(w, r, reqJson, "ValidateUserInfo", "phone", reflect.String, nil, false, "", "")
	if phone == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "ValidateUserInfo", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "ValidateUserInfo", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ValidateUserInfo", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// check if user wants to force their password or not
	forcePass, ok := s.loadValue(w, r, reqJson, "ValidateUserInfo", "force_pass", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}

	// execute core function logic
	res, err := core.ValidateUserInfo(ctx, s.tiDB, userName.(string), password.(string), email.(string), phone.(string), timeZoneI.(string), s.passwordFilter, forcePass.(bool))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ValidateUserInfo core failed", r.URL.Path, "ValidateUserInfo", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"validate-user-info",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ValidateUserInfo", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) ForgotPasswordValidation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "forgot-password-validation-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ForgotPasswordValidation", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load email from body
	email, ok := s.loadValue(w, r, reqJson, "ForgotPasswordValidation", "email", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// attempt to load email from body
	url, ok := s.loadValue(w, r, reqJson, "ForgotPasswordValidation", "url", reflect.String, nil, true, "", "")
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ForgotPasswordValidation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ForgotPasswordValidation(ctx, s.tiDB, s.mailGunKey, s.mailGunDomain, email.(string), url.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ForgotPasswordValidation core failed", r.URL.Path, "ForgotPasswordValidation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"forgot-password-validation",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ForgotPasswordValidation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) ResetForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "reset-forgot-password-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ResetForgotPassword", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load userId from body
	userId, ok := s.loadValue(w, r, reqJson, "ResetForgotPassword", "user_id", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// attempt to load newPassword from body
	newPassword, ok := s.loadValue(w, r, reqJson, "ResetForgotPassword", "new_password", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// attempt to load retypedPassword from body
	retypedPassword, ok := s.loadValue(w, r, reqJson, "ResetForgotPassword", "retyped_password", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// attempt to load forcePass from body
	forcePassI, ok := s.loadValue(w, r, reqJson, "ResetForgotPassword", "force_pass", reflect.Bool, nil, true, "", "")
	if !ok {
		return
	}

	forcePass := false
	if forcePassI != nil {
		forcePass = forcePassI.(bool)
	}

	// attempt to load forcePass from body
	validToken, ok := s.loadValue(w, r, reqJson, "ResetForgotPassword", "valid_token", reflect.Bool, nil, false, "", "")
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ResetForgotPassword", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ResetForgotPassword(ctx, s.tiDB, s.vscClient, userId.(string), newPassword.(string), retypedPassword.(string), s.passwordFilter, forcePass, validToken.(bool))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ResetForgotPassword core failed", r.URL.Path, "ResetForgotPassword", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"reset-forgot-password",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ResetForgotPassword", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) UserProjects(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "user-projects-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.jsonResponse(r, w, map[string]interface{}{"response": "not logged in"}, r.URL.Path, "UserProjects", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "no-login", network.GetRequestIP(r), http.StatusOK)
		return
	}

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UserProjects", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "UserProjects", "skip", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "UserProjects", "limit", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UserProjects(ctx, callingUser.(*models.User), s.tiDB, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UserProjects failed", r.URL.Path, "UserProjects", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"user-projects",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UserProjects", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) UserProfilePage(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "user-profile-page-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingUsername := ""
	callingId := ""
	var callingIdInt int64
	var callingUserModel *models.User

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		callingId = network.GetRequestIP(r)
		callingUsername = network.GetRequestIP(r)
	} else {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		callingUsername = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
		callingUserModel = callingUser.(*models.User)
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UserProfilePage", false, callingUsername, callingIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load season number from body
	rawClient, ok := s.loadValue(w, r, reqJson, "UserProfilePage", "author_id", reflect.String, nil, true, callingUsername, callingId)
	if !ok {
		return
	}

	// load season number if it was passed
	var client *int64
	if rawClient != nil {
		// parse post id to integer
		clientID, err := strconv.ParseInt(rawClient.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse client id string to integer: %s", rawClient.(string)), r.URL.Path, "UserProfilePage", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}

		client = &clientID
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UserProfilePage", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
		return
	}
	// execute core function logic
	res, err := core.UserProfilePage(ctx, callingUserModel, s.tiDB, client)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "UserProfilePage failed", r.URL.Path, "UserProfilePage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUsername, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"user-profile-page",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUsername),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UserProfilePage", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUsername, callingId, http.StatusOK)
}

func (s *HTTPServer) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "change-email-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ChangeEmail", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ChangeEmail", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load email from body
	newEmail, ok := s.loadValue(w, r, reqJson, "ChangeEmail", "new_email", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if newEmail == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}
	// execute core function logic
	res, err := core.ChangeEmail(ctx, callingUser.(*models.User), s.tiDB, newEmail.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ChangeEmail core failed", r.URL.Path, "ChangeEmail", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"change-email-page",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ChangeEmail", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) ChangeUsername(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "change-username-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ChangeUsername", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ChangeUsername", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load email from body
	newUsername, ok := s.loadValue(w, r, reqJson, "ChangeUsername", "new_username", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if newUsername == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ChangeUsername(ctx, callingUser.(*models.User), s.tiDB, newUsername.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ChangeUsername core failed", r.URL.Path, "ChangeUsername", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"change-username",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ChangeUsername", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) ChangePhoneNumber(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "change-phonenumber-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ChangePhoneNumber", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ChangePhoneNumber", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load phone number from body
	newPhone, ok := s.loadValue(w, r, reqJson, "ChangePhoneNumber", "new_phone", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if newPhone == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ChangePhoneNumber(ctx, callingUser.(*models.User), s.tiDB, newPhone.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ChangePhoneNumber core failed", r.URL.Path, "ChangePhoneNumber", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"change-phonenumber",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ChangePhoneNumber", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "change-password-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ChangePassword", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ChangePassword", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load password from body
	oldPassword, ok := s.loadValue(w, r, reqJson, "ChangePassword", "old_password", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if oldPassword == nil || !ok {
		return
	}

	// attempt to load new password from body
	newPassword, ok := s.loadValue(w, r, reqJson, "ChangePassword", "new_password", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if newPassword == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ChangePassword(ctx, callingUser.(*models.User), s.tiDB, oldPassword.(string), newPassword.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ChangePassword core failed", r.URL.Path, "ChangePassword", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"change-password",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ChangePassword", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)

}

func (s *HTTPServer) ChangeUserPicture(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "change-user-picture-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "ChangeUserPicture", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ChangeUserPicture", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load email from body
	newImagePath, ok := s.loadValue(w, r, reqJson, "ChangeUserPicture", "new_image_path", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if newImagePath == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ChangeUserPicture(ctx, callingUser.(*models.User), s.tiDB, newImagePath.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ChangeUserPicture core failed", r.URL.Path, "ChangeUserPicture", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"change-user-picture",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "ChangeUserPicture", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

// func (s *HTTPServer) InitTempUser(w http.ResponseWriter, r *http.Request) {
//	// retrieve calling user from context
//	callingUser := r.Context().Value(CtxKeyUser)
//
//	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)
//
//	// return if calling user was not retrieved in authentication
//	if callingUser == nil {
//		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
//			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
//		return
//	}
//
//	// attempt to load JSON from request body
//	reqJson := s.jsonRequest(w, r, "InitTempUser", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
//	if reqJson == nil {
//		return
//	}
//
//	// attempt to load first name from body
//	firstName, ok := s.loadValue(w, r, reqJson, "InitTempUser", "username", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
//	if firstName == nil || !ok {
//		return
//	}
//
//	// attempt to load email from body
//	email, ok := s.loadValue(w, r, reqJson, "InitTempUser", "email", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
//	if email == nil || !ok {
//		return
//	}
//
//	// check if this is a test
//	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
//		// return success for test
//		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "InitTempUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
//		return
//	}
//
//	// execute core function logic
//	res, err := core.InitTempUser(s.tiDB, callingUser.(*models.User), firstName.(string), email.(string))
//	if err != nil {
//
//		// select error message dependent on if there was one returned from the function
//		responseMessage := selectErrorResponse("internal server error occurred", res)
//		// handle error internally
//		s.handleError(w, "add season core failed", r.URL.Path, "InitTempUser", r.Method, r.Context().Value(CtxKeyRequestID),
//			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
//		// exit
//		return
//	}
//
//	// return response
//	s.jsonResponse(r, w, res, r.URL.Path, "InitTempUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
// }

func (s *HTTPServer) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "delete-user-account-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "DeleteUserAccount", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "DeleteUser", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "DeleteUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.DeleteUserAccount(ctx, s.tiDB, s.meili, s.vscClient, callingUser.(*models.User), s.mailGunKey, s.mailGunDomain)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "DeleteUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// revoke cookie
	s.revokeCookie(w, network.GetRequestIP(r))

	parentSpan.AddEvent(
		"delete-user-account",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "DeleteUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) CreateNewGoogleUser(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-new-google-user-http")
	defer parentSpan.End()

	reqJson := s.receiveUpload(w, r, "CreateNewUser", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load bio from body
	externalAuth, ok := s.loadValue(w, r, reqJson, "CreateNewGoogleUser", "external_auth", reflect.String, nil, false, "", "")
	if externalAuth == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "CreateNewGoogleUser", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "CreateNewGoogleUser", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "CreateNewGoogleUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "start_user_info", reflect.Map, nil, true, "", "")
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.UserStart

	if workspaceSettingsI != nil {
		// conditionally attempt to marshall and unmarshall the workspace settings
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
	}

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "avatar_settings", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(bufs)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(bufs)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "upload_id", reflect.String, nil, false, "", "")
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// attempt to load key from body
	referralI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "referral_user", reflect.String, nil, true, "", "")
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
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateNewGoogleUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateNewGoogleUser(ctx, s.tiDB, s.meili, s.sf, s.stripeSubscriptions, s.streakEngine, s.domain, externalAuth.(string),
		password.(string), s.vscClient, *workspaceSettings, timeZoneI.(string), avatarSetting, thumbnailTempPath,
		s.storageEngine, s.mailGunKey, s.mailGunDomain, s.initialRecUrl, referral, s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateNewGoogleUser core failed", r.URL.Path, "CreateNewGoogleUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-new-google-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateNewGoogleUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) CreateNewGithubUser(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-new-github-user-http")
	defer parentSpan.End()

	// attempt to load JSON from request body

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "CreateNewUser", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load bio from body
	externalAuth, ok := s.loadValue(w, r, reqJson, "CreateNewGithubUser", "external_auth", reflect.String, nil, false, "", "")
	if externalAuth == nil || !ok {
		return
	}

	// attempt to load new password from body
	password, ok := s.loadValue(w, r, reqJson, "CreateNewGithubUser", "password", reflect.String, nil, false, "", "")
	if password == nil || !ok {
		return
	}

	// attempt to load timezone from body
	timeZoneI, ok := s.loadValue(w, r, reqJson, "CreateNewGithubUser", "timezone", reflect.String, nil, false, "", "")
	if !ok {
		return
	}

	// ensure that the timezone is a valid IANA string
	_, err := time.LoadLocation(timeZoneI.(string))
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate IANA timezone: %s", timeZoneI.(string)), r.URL.Path, "CreateNewGithubUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "CreateNewGithubUser", "start_user_info", reflect.Map, nil, true, "", "")
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.UserStart

	if workspaceSettingsI != nil {
		// conditionally attempt to marshall and unmarshall the workspace settings
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateNewGithubUser", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		err = json.Unmarshal(buf, &workspaceSettings)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to decode rank from string: %s", workspaceSettingsI.(string)), r.URL.Path, "CreateNewGithubUser", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
	}

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "avatar_settings", reflect.Map, nil, false, "", "")
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(bufs)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(bufs)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "upload_id", reflect.String, nil, false, "", "")
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// attempt to load key from body
	referralI, ok := s.loadValue(w, r, reqJson, "CreateNewUser", "referral_user", reflect.String, nil, true, "", "")
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
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateNewGithubUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, token, err := core.CreateNewGithubUser(ctx, s.tiDB, s.meili, s.sf, s.stripeSubscriptions, s.streakEngine, s.domain, externalAuth.(string),
		password.(string), s.vscClient, *workspaceSettings, timeZoneI.(string), avatarSetting, s.githubSecret,
		thumbnailTempPath, s.storageEngine, s.mailGunKey, s.mailGunDomain, s.initialRecUrl, referral, network.GetRequestIP(r), s.logger)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateNewGithubUser core failed", r.URL.Path, "CreateNewGithubUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// conditionally set cookie with insecure settings
	if s.developmentMode {
		// set cookie in response if the token was created
		http.SetCookie(w, &http.Cookie{
			Name:     "gigoAuthToken",
			Value:    token,
			Expires:  time.Now().Add(time.Hour * 24 * 30),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Domain:   fmt.Sprintf(".%s", s.domain),
		})
	} else {
		// set cookie in response if the token was created
		http.SetCookie(w, &http.Cookie{
			Name:     "gigoAuthToken",
			Value:    token,
			Expires:  time.Now().Add(time.Hour * 24 * 30),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   true,
			Domain:   fmt.Sprintf(".%s", s.domain),
		})
	}

	parentSpan.AddEvent(
		"create-new-github-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateNewGithubUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
}

func (s *HTTPServer) GetSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-subscription-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetSubscription", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetSubscription", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetSubscription(ctx, callingUser.(*models.User))
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "GetSubscription", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-subscription-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetSubscription", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetUserInformation(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-information-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserInformation", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetUserInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetUserInformation(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "GetUserInformation", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-user-information",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetUserInformation", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) SetUserWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-user-workspace-settings-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SetUserWorkspaceSettings", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	workspaceSettingsI, ok := s.loadValue(w, r, reqJson, "SetUserWorkspaceSettings", "workspace_settings", reflect.Map, nil, true, callingUser.(*models.User).UserName, userId)
	if !ok {
		return
	}

	// create variable to hold workspace settings
	var workspaceSettings *models.WorkspaceSettings

	fmt.Println("workspace settings I are: \n", workspaceSettingsI)

	// conditionally attempt to marshall and unmarshall the workspace settings
	if workspaceSettingsI != nil {
		buf, err := json.Marshal(workspaceSettingsI)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to marshal workspace settings: %s", err), r.URL.Path, "SetUserWorkspaceSettings", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}

		// if len(buf) <= 0 || buf == nil {
		//	s.handleError(w, fmt.Sprintf("failed to marshal workspace settings: %s", errors.New(fmt.Sprintf("buffer is empty"))), r.URL.Path, "SetUserWorkspaceSettings", r.Method,
		//		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError,
		//		"internal server error occurred", err)
		//	return
		// }

		var ws models.WorkspaceSettings
		err = json.Unmarshal(buf, &ws)
		if err != nil {
			s.handleError(w, fmt.Sprintf("failed to unmarshal workspace settings: %s", err), r.URL.Path, "SetUserWorkspaceSettings", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError,
				"internal server error occurred", err)
			return
		}
		workspaceSettings = &ws

		fmt.Println("workspace settings commit: \n", ws)

		// // ensure the validity of the settings
		// if workspaceSettings.AutoGit.CommitMessage == "" {
		//	s.handleError(w, fmt.Sprintf("invalid commit message for workspace settings: %s", workspaceSettings.AutoGit.CommitMessage), r.URL.Path, "SetUserWorkspaceSettings", r.Method,
		//		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusUnprocessableEntity,
		//		"invalid commit message", nil)
		//	return
		// }
	}

	fmt.Println("workspace settings marshalled: \n", workspaceSettings)

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SetUserWorkspaceSettings", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SetUserWorkspaceSettings(ctx, callingUser.(*models.User), s.tiDB, workspaceSettings)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "SetUserWorkspaceSettings", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-user-workspace-settings",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SetUserWorkspaceSettings", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetUserWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-information-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserWorkspaceSettings", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetUserWorkspaceSettings", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetUserWorkspaceSettings(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "GetUserWorkspaceSettings", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-user-information",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetUserWorkspaceSettings", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) UpdateAvatarSettings(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "update-avatar-settings-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// receive upload part and handle file assemble
	reqJson := s.receiveUpload(w, r, "UpdateAvatarSettings", "File Part Uploaded.", "anon", int64(-1))
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	avatarSettingI, ok := s.loadValue(w, r, reqJson, "UpdateAvatarSettings", "avatar_settings", reflect.Map, nil, false, callingUser.(*models.User).UserName, userId)
	if !ok {
		return
	}

	// create variable to hold user initialization form
	var avatarSetting models.AvatarSettings

	// conditionally attempt to marshall and unmarshall the user init form
	bufs, err := json.Marshal(avatarSettingI)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to marshall user init form: %s", string(bufs)), r.URL.Path, "UpdateAvatarSettings", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	err = json.Unmarshal(bufs, &avatarSetting)
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to unmarshall user init form: %s", string(bufs)), r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// attempt to load parameter from body
	uploadId, ok := s.loadValue(w, r, reqJson, "UpdateAvatarSettings", "upload_id", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
	if uploadId == nil || !ok {
		return
	}

	// create thumbnail temp path
	thumbnailTempPath := filepath.Join("temp", uploadId.(string))

	// defer removal of thumbnail temp file
	defer s.storageEngine.DeleteFile(thumbnailTempPath)

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UpdateAvatarSettings", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UpdateAvatarSettings(ctx, callingUser.(*models.User), s.tiDB, avatarSetting, thumbnailTempPath, s.storageEngine)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "UpdateAvatarSettings", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"update-avatar-settings",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UpdateAvatarSettings", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) FollowUser(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "follow-user-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "FollowUser", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "FollowUser", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	attemptId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "FollowUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "FollowUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.FollowUser(ctx, callingUser.(*models.User), s.tiDB, attemptId)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "FollowUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"follow-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "FollowUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) UnFollowUser(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "un-follow-user-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateNewUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UnFollowUser", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	id, ok := s.loadValue(w, r, reqJson, "UnFollowUser", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
	if id == nil || !ok {
		return
	}

	// parse post id to integer
	attemptId, err := strconv.ParseInt(id.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse post id string to integer: %s", id.(string)), r.URL.Path, "UnFollowUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UnFollowUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UnFollowUser(ctx, callingUser.(*models.User), s.tiDB, attemptId)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "add season core failed", r.URL.Path, "UnFollowUser", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"un-follow-user",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UnFollowUser", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) UpdateUserExclusiveAgreement(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-user-exclusive-agreement-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UpdateUserExclusiveAgreement", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UpdateUserExclusiveAgreement", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UpdateUserExclusiveAgreement", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UpdateUserExclusiveAgreement(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "update user exclsuive agreement failed", r.URL.Path, "UpdateUserExclusiveAgreement", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-user-exclusive-agreement",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UpdateUserExclusiveAgreement", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) UpdateHolidayPreference(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-user-holiday-preference-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "UpdateHolidayPreference", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "UpdateHolidayPreference", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "UpdateHolidayPreference", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.UpdateHolidayPreference(ctx, callingUser.(*models.User), s.tiDB)
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "update user exclsuive agreement failed", r.URL.Path, "UpdateHolidayPreference", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-user-holiday-preference",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "UpdateUserExclusiveAgreement", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) MarkTutorialAsCompleted(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "mark-tutorial-as-completed-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	userId := fmt.Sprintf("%v", callingUser.(*models.User).ID)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "MarkTutorialAsCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "MarkTutorialAsCompleted", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// load the tutorial key
	tutorialKey, ok := s.loadValue(w, r, reqJson, "MarkTutorialAsCompleted", "tutorial_key", reflect.String, nil, false, callingUser.(*models.User).UserName, userId)
	if !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "MarkTutorialAsCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.MarkTutorialAsCompleted(ctx, callingUser.(*models.User), s.tiDB, tutorialKey.(string))
	if err != nil {

		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "update user exclsuive agreement failed", r.URL.Path, "MarkTutorialAsCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"mark-tutorial-as-completed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
			attribute.String("tutorial_key", tutorialKey.(string)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "MarkTutorialAsCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, userId, http.StatusOK)
}

func (s *HTTPServer) GetUserID(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-id-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingUserId := int64(-1)
	callingUserIdString := "-1"
	callingUserName := "anon"

	// load calling user values if the caller is logged in
	if callingUser != nil {
		callingUserId = callingUser.(*models.User).ID
		callingUserIdString = fmt.Sprintf("%v", callingUser.(*models.User).ID)
		callingUserName = callingUser.(*models.User).UserName
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserID", false, callingUserName, callingUserId)
	if reqJson == nil {
		return
	}

	// attempt to load parameter from body
	username, ok := s.loadValue(w, r, reqJson, "GetUserID", "username", reflect.String, nil, false, callingUserName, callingUserIdString)
	if username == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetUserID", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName, callingUserIdString, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetUserID(ctx, s.tiDB, username.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, fmt.Sprintf("get user id core failed: %s", username.(string)), r.URL.Path, "GetUserID", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingUserIdString, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-user-id",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", username.(string)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetUserID", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName, callingUserIdString, http.StatusOK)
}
