package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
)

func TestHTTPServer_BroadcastMessage(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where _id = ?;", user.ID)
	}()

	body := bytes.NewReader([]byte(`{
		"message": "Hello, world!",
		"test": true
	}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/broadcast/message", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_BroadcastMessage failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_BroadcastMessage failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		t.Errorf("\nTestHTTPServer_BroadcastMessage failed\n    Status code: %d, response: %s", res.StatusCode, string(body))
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_BroadcastMessage failed\n    Error: %v", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Errorf("\nTestHTTPServer_BroadcastMessage failed\n    Error: %v", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Errorf("\nTestHTTPServer_BroadcastMessage failed\n    Expected: %+v, got: %+v", expectedJson, resJson)
		return
	}

	t.Log("\nTestHTTPServer_BroadcastMessage succeeded")
}

func TestHTTPServer_GetBroadcastMessages(t *testing.T) {

	// Insert some test data into the broadcast_event table
	events := []models.BroadcastEvent{
		{ID: 1, UserID: 1, UserName: "user1", Message: "Test message 1", BroadcastType: 0, TimePosted: time.Now()},
		{ID: 2, UserID: 2, UserName: "user2", Message: "Test message 2", BroadcastType: 0, TimePosted: time.Now()},
		{ID: 3, UserID: 3, UserName: "user3", Message: "Test message 3", BroadcastType: 0, TimePosted: time.Now()},
	}

	for _, event := range events {
		event, err := models.CreateBroadcastEvent(event.ID, event.UserID, event.UserName, event.Message, event.BroadcastType, event.TimePosted)
		if err != nil {
			t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Error: %v\n", err)
			return
		}

		statement := event.ToSQLNative()

		_, err = testHttpServer.tiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Error: %v", err)
			return
		}

	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM broadcast_event")
	}()

	body := bytes.NewReader([]byte(`{
		"test": true
	}`))

	req, err := http.NewRequest("POST", "http://localhost:1818/api/broadcast/get", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Status code: %d, response: %s", res.StatusCode, string(body))
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Error: %v", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Error: %v", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Errorf("\nTestHTTPServer_GetBroadcastMessages failed\n    Expected: %+v, got: %+v", expectedJson, resJson)
		return
	}

	t.Log("\nTestHTTPServer_GetBroadcastMessages succeeded")
}

func TestHTTPServer_CheckBroadcastAward(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestCheckBroadcastAward failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	// Send a request to the CheckBroadcastAward endpoint
	body := bytes.NewReader([]byte(`{"test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/broadcast/check", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CheckBroadcastAward failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CheckBroadcastAward failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CheckBroadcastAward failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CheckBroadcastAward failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CheckBroadcastAward failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CheckBroadcastAward failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CheckBroadcastAward succeeded")
}

func TestHTTPServer_RevertBroadcastAward(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestRevertBroadcastAward failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	// Send a request to the RevertBroadcastAward endpoint
	body := bytes.NewReader([]byte(`{"test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/broadcast/revert", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RevertBroadcastAward failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RevertBroadcastAward failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_RevertBroadcastAward failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_RevertBroadcastAward failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_RevertBroadcastAward failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_RevertBroadcastAward failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_RevertBroadcastAward succeeded")
}
