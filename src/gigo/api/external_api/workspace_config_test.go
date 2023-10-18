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

func TestHTTPServer_CreateWorkspaceConfig(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test":        true,
		"title":       "TestWorkspace",
		"description": "This is a test workspace",
		"content":     "Test content",
		"languages":   []float64{1.0, 2.0, 3.0}, // Substitute with actual language codes
		"tags": []map[string]interface{}{
			{
				"id":    "1",
				"value": "TestTag1",
			},
			{
				"id":    "2",
				"value": "TestTag2",
			},
		},
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/workspace/config/create", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateWorkspaceConfig failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateWorkspaceConfig failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CreateWorkspaceConfig failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateWorkspaceConfig failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateWorkspaceConfig failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CreateWorkspaceConfig failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CreateWorkspaceConfig succeeded")
}

func TestHTTPServer_UpdateWorkspaceConfig(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test":        true,
		"id":          "1", // ID of the workspace to update
		"description": "Updated description",
		"content":     "Updated content",
		"languages":   []float64{1.0, 2.0, 3.0}, // Substitute with actual language codes
		"tags": []map[string]interface{}{
			{
				"id":    "1",
				"value": "UpdatedTag1",
			},
			{
				"id":    "2",
				"value": "UpdatedTag2",
			},
		},
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("PUT", "http://localhost:1818/api/workspace/config/update", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateWorkspaceConfig failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateWorkspaceConfig failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_UpdateWorkspaceConfig failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateWorkspaceConfig failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateWorkspaceConfig failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_UpdateWorkspaceConfig failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_UpdateWorkspaceConfig succeeded")
}

func TestHTTPServer_GetWorkspaceConfig(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test": true,
		"id":   "1", // ID of the workspace to get
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("GET", "http://localhost:1818/api/workspace/config/get", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetWorkspaceConfig failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetWorkspaceConfig failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetWorkspaceConfig failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetWorkspaceConfig failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetWorkspaceConfig failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetWorkspaceConfig failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetWorkspaceConfig succeeded")
}
