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
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateRandomString generates a random string of the given length with at least one character from each category.
func GenerateRandomUsername(length int) (string, error) {
	if length < 3 {
		return "", fmt.Errorf("length should be at least 3 to contain at least one character from each category")
	}

	const (
		lowerSet  = "abcdefghijklmnopqrstuvwxyz"
		upperSet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numberSet = "0123456789"
	)
	allSet := lowerSet + upperSet + numberSet

	result := make([]byte, length)

	// Ensure one character from each category
	randomLower, err := rand.Int(rand.Reader, big.NewInt(int64(len(lowerSet))))
	if err != nil {
		return "", err
	}
	result[0] = lowerSet[randomLower.Int64()]

	randomUpper, err := rand.Int(rand.Reader, big.NewInt(int64(len(upperSet))))
	if err != nil {
		return "", err
	}
	result[1] = upperSet[randomUpper.Int64()]

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(numberSet))))
	if err != nil {
		return "", err
	}
	result[2] = numberSet[randomNumber.Int64()]

	// Fill in the rest
	for i := 3; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(allSet))))
		if err != nil {
			return "", err
		}
		result[i] = allSet[randomIndex.Int64()]
	}

	return string(result), nil
}

// GenerateRandomPassword generates a random string of the given length with at least one character from each category.
func GenerateRandomPassword(length int) (string, error) {
	if length < 3 {
		return "", fmt.Errorf("length should be at least 3 to contain at least one character from each category")
	}

	const (
		lowerSet  = "abcdefghijklmnopqrstuvwxyz"
		upperSet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numberSet = "0123456789"
	)
	allSet := lowerSet + upperSet + numberSet

	result := make([]byte, length)

	// Ensure one character from each category
	randomLower, err := rand.Int(rand.Reader, big.NewInt(int64(len(lowerSet))))
	if err != nil {
		return "", err
	}
	result[0] = lowerSet[randomLower.Int64()]

	randomUpper, err := rand.Int(rand.Reader, big.NewInt(int64(len(upperSet))))
	if err != nil {
		return "", err
	}
	result[1] = upperSet[randomUpper.Int64()]

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(numberSet))))
	if err != nil {
		return "", err
	}
	result[2] = numberSet[randomNumber.Int64()]

	// Fill in the rest
	for i := 3; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(allSet))))
		if err != nil {
			return "", err
		}
		result[i] = allSet[randomIndex.Int64()]
	}

	return string(result), nil
}
