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

func TestHTTPServer_FeedPage(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
		testHttpServer.tiDB.DB.Exec("delete from post")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_FeedPage failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_FeedPage failed\n    Error: ", err)
			return
		}
	}

	// Insert sample post data
	location, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	samplePost := &models.Post{
		ID:          1,
		Title:       "Test Post",
		Description: "Test Description",
		Tier:        1,
		Coffee:      69,
		Author:      "test",
		AuthorID:    user.ID,
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

	body := bytes.NewReader([]byte(`{"test":true, "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/following/feed", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_FeedPage failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_FeedPage failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_FeedPage failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_FeedPage failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_FeedPage failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_FeedPage failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_FeedPage succeeded")
}
