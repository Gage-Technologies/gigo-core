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
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
)

func (s *HTTPServer) VerifyCaptcha(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "verify-captcha")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "VerifyCaptcha", false, "", -1)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	captchaResponse, ok := s.loadValue(w, r, reqJson, "VerifyCaptcha", "captcha_response", reflect.String, nil, false, "", "")
	if captchaResponse == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "VerifyCaptcha", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)
		return
	}

	//user := &models.User{}

	isVerified, score, err := core.VerifyRecaptcha(captchaResponse.(string), s.captchaSecret)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"success": false, "message": "failed to call VerifyRecaptcha", "error": err.Error()})
		// handle error internally
		s.handleError(w, "VerifyRecaptcha core failed", r.URL.Path, "VerifyRecaptcha", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	if !isVerified {
		s.logger.Debugf("captcha score for %s is %f", network.GetRequestIP(r), score)
	}

	parentSpan.AddEvent(
		"verify-captcha",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.Float64("score", score),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	res := map[string]interface{}{}

	// TODO: fix this
	res = map[string]interface{}{"success": true, "message": "Captcha successfully verified"}
	//if !isVerified {
	//	res = map[string]interface{}{"success": false, "message": "Invalid captcha response"}
	//} else {
	//	res = map[string]interface{}{"success": true, "message": "Captcha successfully verified"}
	//}

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "VerifyCaptcha", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "", "", http.StatusOK)

}
