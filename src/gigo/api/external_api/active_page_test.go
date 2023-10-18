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

func TestHTTPServer_PastWeekActive(t *testing.T) {
	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from post")
		testHttpServer.tiDB.DB.Exec("delete from attempt")
	}()

	// Insert sample post data
	location, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	// Insert sample post data
	samplePost := &models.Post{
		ID:          1,
		Title:       "Test Post",
		Description: "Test Description",
		Tier:        1,
		Coffee:      69,
		Author:      "test",
		AuthorID:    callingUser.ID,
		CreatedAt:   time.Now().Add(-6 * 24 * time.Hour), // 6 days ago
		UpdatedAt:   time.Date(1, 1, 1, 1, 1, 1, 1, location),
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

	// Insert sample attempt data
	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    callingUser.ID,
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
		UpdatedAt:   time.Date(1, 1, 1, 1, 1, 1, 1, location),
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

	body := bytes.NewReader([]byte(`{"test":true, "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/active/pastWeek", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_PastWeekActive failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_PastWeekActive failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_PastWeekActive failed\n    Error: incorrect response code")
		time.Sleep(time.Hour)
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_PastWeekActive failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_PastWeekActive failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_PastWeekActive failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_PastWeekActive succeeded")
}

func TestHTTPServer_MostChallengingActive(t *testing.T) {
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
		t.Error("\nTestHTTPServer_MostChallengingActive failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_MostChallengingActive failed\n    Error: ", err)
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

	body := bytes.NewReader([]byte(`{"test":true, "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/active/challenging", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_MostChallengingActive failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_MostChallengingActive failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_MostChallengingActive failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_MostChallengingActive failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_MostChallengingActive failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_MostChallengingActive failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_MostChallengingActive succeeded")
}

func TestHTTPServer_DontGiveUpActive(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
		testHttpServer.tiDB.DB.Exec("delete from post")
		testHttpServer.tiDB.DB.Exec("delete from attempt")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_DontGiveUpActive failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_DontGiveUpActive failed\n    Error: ", err)
			return
		}
	}

	location, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    user.ID,
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour), // 7 days ago
		UpdatedAt:   time.Date(1, 1, 1, 1, 1, 1, 1, location),
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

	body := bytes.NewReader([]byte(`{"test":true, "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/active/dontGiveUp", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DontGiveUpActive failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DontGiveUpActive failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_DontGiveUpActive failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_DontGiveUpActive failed\n    Error: ", err)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(resBody, &data)
	if err != nil {
		t.Error("\nTestHTTPServer_DontGiveUpActive failed\n    Error: ", err)
		return
	}

	if _, ok := data["projects"]; !ok {
		t.Error("\nTestHTTPServer_DontGiveUpActive failed\n    Error: expected 'projects' field in the response")
		return
	}
}
