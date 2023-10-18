package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestHTTPServer_GetXPBoostCount(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("GET", "http://localhost:1818/api/xp/getXPBoost", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetXPBoostCount failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetXPBoostCount failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetXPBoostCount failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetXPBoostCount failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetXPBoostCount failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetXPBoostCount failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetXPBoostCount succeeded")
}

func TestHTTPServer_StartXPBoost(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test": true,
		"_id":  "12345", // Substitute with a valid user ID for actual tests
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/xp/startXPBoost", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_StartXPBoost failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_StartXPBoost failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_StartXPBoost failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_StartXPBoost failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_StartXPBoost failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_StartXPBoost failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_StartXPBoost succeeded")
}
