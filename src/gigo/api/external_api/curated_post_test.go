package external_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestHTTPServer_AddPostToCurated(t *testing.T) {
	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	// Clean up test data
	defer func() {
		testHttpServer.tiDB.DB.Exec("DELETE FROM users WHERE user_name = ?;", callingUser.UserName)
	}()

	// Create HTTP Request
	body := bytes.NewReader([]byte(`{"post_id": "somePostID", "language": 1.0, "proficiency_type": [1.0, 2.0], "test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/curated/add", body)
	if err != nil {
		t.Errorf("failed to create request, err: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform HTTP Request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("failed to perform request, err: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		t.Errorf("Incorrect response code: got %v", res.StatusCode)
		t.Errorf("Body: %s", string(body))
		return
	}

	// Verify Response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("failed to read response body, err: %v", err)
		return
	}

	expectedJson := map[string]interface{}{}
	var resJson map[string]interface{}
	json.Unmarshal(resBody, &resJson)
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Errorf("response JSON does not match expected JSON")
		return
	}

	t.Log("TestHTTPServer_AddPostToCurated succeeded")
}

func TestHTTPServer_RemoveCuratedPost(t *testing.T) {
	var ava models.AvatarSettings

	// Create callingUser and necessary data
	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser1")
	}()

	// Add user to DB
	userStmt, err := callingUser.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: ", err)
			return
		}
	}

	// Build request body
	body := bytes.NewReader([]byte(`{"curated_post_id":42}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/curated/remove", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Inject user into request context
	ctx := context.WithValue(req.Context(), CtxKeyUser, callingUser)
	req = req.WithContext(ctx)

	// Perform request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: %v", err)
		return
	}

	// Check response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: incorrect response code")
		return
	}

	// Check response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_RemoveCuratedPost failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_RemoveCuratedPost succeeded")
}

func TestHTTPServer_GetCuratedPostsForAdmin(t *testing.T) {
	var ava models.AvatarSettings

	// Create callingUser and necessary data
	callingUser, err := models.CreateUser(2, "testadmin", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testadmin")
	}()

	// Add user to DB
	userStmt, err := callingUser.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: ", err)
			return
		}
	}

	// Build request body
	body := bytes.NewReader([]byte(`{"proficiency_filter": 1, "language_filter": 2, "test": true}`))
	req, err := http.NewRequest("GET", "http://localhost:1818/api/admin/curated/posts", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Inject user into request context
	ctx := context.WithValue(req.Context(), CtxKeyUser, callingUser)
	req = req.WithContext(ctx)

	// Perform request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: %v", err)
		return
	}

	// Check response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: incorrect response code")
		return
	}

	// Check response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: ", err)
		return
	}

	// Since it's a test run, expecting an empty response JSON
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetCuratedPostsForAdmin failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetCuratedPostsForAdmin succeeded")
}
