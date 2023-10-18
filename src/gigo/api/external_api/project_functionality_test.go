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

func TestHTTPServer_DeleteProject(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestDeleteProject failed\n    Error: %v\n", err)
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

	// Assume we have a project with id 1
	projectId := 1

	// Send a request to the DeleteProject endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"project_id": "%v", "test": true}`, projectId)))
	req, err := http.NewRequest("DELETE", "http://localhost:1818/api/project/delete", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeleteProject failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_DeleteProject failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_DeleteProject failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_DeleteProject failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_DeleteProject failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_DeleteProject failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_DeleteProject succeeded")
}

func TestHTTPServer_StartAttempt(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestStartAttempt failed\n    Error: %v\n", err)
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

	// Assume we have a project with id 1
	projectId := 1

	// Send a request to the StartAttempt endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"project_id": "%v", "parent_attempt": "0", "test": true}`, projectId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/attempt/start", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_StartAttempt failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_StartAttempt failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_StartAttempt failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_StartAttempt failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_StartAttempt failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_StartAttempt failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_StartAttempt succeeded")
}

func TestHTTPServer_PublishProject(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestPublishProject failed\n    Error: %v\n", err)
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

	//// Create a test post
	//testPost := models.Post{
	//	ID:       1,
	//	AuthorID: 1,
	//	Title:    "Test Post",
	//}
	//
	//postStmt, err := testPost.ToSQLNative()
	//if err != nil {
	//	t.Fatalf("Failed to convert post to SQL: %v", err)
	//}
	//
	//for _, stmt := range postStmt {
	//	_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
	//	if err != nil {
	//		t.Fatalf("Failed to insert test post: %v", err)
	//	}
	//}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("delete from post")
	}()

	// Assume we have a project with id 1
	projectId := 1

	// Send a request to the PublishProject endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"project_id": "%v", "test": true}`, projectId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/publish", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_PublishProject failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_PublishProject failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_PublishProject failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_PublishProject failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_PublishProject failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_PublishProject failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_PublishProject succeeded")
}

func TestHTTPServer_EditConfig(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestEditConfig failed\n    Error: %v\n", err)
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

	// Assume we have a repository with id 1
	repoId := 1
	// Sample configuration content
	configContent := "Sample configuration content"

	// Send a request to the EditConfig endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"content": "%s", "repo": "%v"}`, configContent, repoId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/editConfig", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_EditConfig failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_EditConfig failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_EditConfig failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_EditConfig failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_EditConfig failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_EditConfig failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_EditConfig succeeded")
}

func TestHTTPServer_GetConfig(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetConfig failed\n    Error: %v\n", err)
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

	// Assume we have a repository with id 1 and a commit
	repoId := 1
	commit := "sampleCommit"

	// Send a request to the GetConfig endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"commit": "%s", "repo": "%v"}`, commit, repoId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/config", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetConfig failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetConfig failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetConfig failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetConfig failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetConfig failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetConfig failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetConfig succeeded")
}

func TestHTTPServer_CloseAttempt(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestCloseAttempt failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from attempt")
	}()

	// You need to use an existing attempt for testing. Replace `attemptId` with the ID of an existing attempt.
	attemptId := int64(1)

	attempt, err := models.CreateAttempt(1, "title", "description", testUser.UserName, testUser.ID, time.Now(), time.Now(), 1, testUser.Tier, nil, 0, 420, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	statement, err := attempt.ToSQLNative()
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	for _, stmt := range statement {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
	}

	// Send a request to the CloseAttempt endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"attempt_id": "%v"}`, attemptId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/attempt/closeAttempt", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CloseAttempt failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_CloseAttempt failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CloseAttempt failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CloseAttempt failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CloseAttempt failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CloseAttempt failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CloseAttempt succeeded")
}

func TestHTTPServer_MarkSuccess(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestMarkSuccess failed\n    Error: %v\n", err)
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

	// Assume we have an attempt with id 1
	attemptId := 1

	// Send a request to the MarkSuccess endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"attempt_id": "%v"}`, attemptId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/attempt/markSuccess", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_MarkSuccess failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_MarkSuccess failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_MarkSuccess failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_MarkSuccess failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_MarkSuccess failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_MarkSuccess failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_MarkSuccess succeeded")
}
