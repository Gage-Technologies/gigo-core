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

package utils

import (
	"testing"
)

func TestGenerateRandomUsername(t *testing.T) {
	username, err := GenerateRandomUsername(16)
	if err != nil {
		t.Fatalf("\nGenerateRandomUsername failed\n    Error: %v", err)
	}

	if username == "" {
		t.Fatalf("\nGenerateRandomUsername failed\n    Error: %v", err)
	}

	t.Logf("\nGenerateRandomUsername succeeded: %v", username)
}

