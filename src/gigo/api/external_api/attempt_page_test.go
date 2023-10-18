package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestHTTPServer_ProjectAttemptInformation(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
		testHttpServer.tiDB.DB.Exec("delete from attempt")
	}()

	// Add user to DB
	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: ", err)
			return
		}
	}

	// Insert sample attempt data
	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    user.ID,
		PostID:      1,
	}
	attemptStmt, err := sampleAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert attempt to SQL: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample attempt: %v", err)
		}
	}

	// Insert data related to challenging posts and attempts here

	body := bytes.NewReader([]byte(`{"attempt_id":"1", "test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/getProject", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_ProjectAttemptInformation failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_ProjectAttemptInformation succeeded")
}

func TestHTTPServer_AttemptInformation(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
		testHttpServer.tiDB.DB.Exec("delete from attempt")
	}()

	// Add user to DB
	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_AttemptInformation failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_AttemptInformation failed\n    Error: ", err)
			return
		}
	}

	// Insert sample attempt data
	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    user.ID,
		PostID:      1,
	}
	attemptStmt, err := sampleAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert attempt to SQL: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample attempt: %v", err)
		}
	}

	// Insert data related to challenging posts and attempts here

	body := bytes.NewReader([]byte(`{"test":true, "attempt_id": "1"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/attempt/information", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AttemptInformation failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AttemptInformation failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_AttemptInformation failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_AttemptInformation failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_AttemptInformation failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} //
	// Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_AttemptInformation failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_AttemptInformation succeeded")
}

func TestHTTPServer_GetAttemptCode(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	// Add user to DB
	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetAttemptCode failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetAttemptCode failed\n    Error: ", err)
			return
		}
	}

	body := bytes.NewReader([]byte(`{
		"repo":"testRepo",
		"ref":"testRef",
		"filepath":"testPath",
		"test":true
	}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/attempt/code", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetAttemptCode failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetAttemptCode failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetAttemptCode failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetAttemptCode failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetAttemptCode failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetAttemptCode failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetAttemptCode succeeded")
}

func TestHTTPServer_EditDescription(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	// Add user to DB
	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_EditDescription failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_EditDescription failed\n    Error: ", err)
			return
		}
	}

	// Insert sample post data
	samplePost := &models.Post{
		ID:          1,
		Title:       "Test Post",
		Description: "Test Description",
		Tier:        1,
		Coffee:      69,
		Author:      "testuser",
		AuthorID:    user.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	postStmt, err := samplePost.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample post: %v", err)
		}
	}

	body := bytes.NewReader([]byte(`{
		"id":"1",
		"project":true,
		"description":"test description",
		"test":true
	}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/attempt/editdescription", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_EditDescription failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_EditDescription failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_EditDescription failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_EditDescription failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_EditDescription failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_EditDescription failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_EditDescription succeeded")
}
