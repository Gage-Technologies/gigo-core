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

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Google's reCAPTCHA API URL
const recaptchaServerName = "https://www.google.com/recaptcha/api/siteverify"

// RecaptchaResponse holds the response from Google's reCAPTCHA API
type RecaptchaResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	Score       float64  `json:"score"`
	ErrorCodes  []string `json:"error-codes"`
}

// VerifyRecaptcha verifies the user's reCAPTCHA response
func VerifyRecaptcha(response string, secretKey string) (bool, float64, error) {
	// Create POST request to Google reCAPTCHA API
	resp, err := http.PostForm(recaptchaServerName,
		url.Values{"secret": {secretKey}, "response": {response}})

	if err != nil {
		return false, 0, fmt.Errorf("failed to create POST request to Google reCAPTCHA API: %s", err)
	}
	defer resp.Body.Close()

	// Read response from Google reCAPTCHA
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, 0, fmt.Errorf("failed to read response from Google reCAPTCHA API: %s", err)
	}

	var recaptchaResponse RecaptchaResponse
	err = json.Unmarshal(body, &recaptchaResponse)
	if err != nil {
		return false, 0, errors.New(fmt.Sprintf("Error unmarshalling JSON: %v", err.Error()))
	}

	if recaptchaResponse.Success {
		if recaptchaResponse.Score < .6 {
			return false, recaptchaResponse.Score, nil
		}
	}

	return recaptchaResponse.Success, 0, nil
}
