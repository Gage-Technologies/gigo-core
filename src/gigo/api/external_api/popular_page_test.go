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

func TestHTTPServer_PopularPageFeed(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestPopularPageFeed failed\n    Error: %v\n", err)
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

	// Insert sample posts
	for i := 1; i <= 5; i++ {
		testPost := &models.Post{
			ID:          int64(i),
			Title:       fmt.Sprintf("Test Post %d", i),
			Description: fmt.Sprintf("Test post description %d", i),
			Author:      testUser.UserName,
			AuthorID:    testUser.ID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			RepoID:      int64(i),
			Tier:        1,
			PostType:    1,
			Views:       10,
			Completions: 5,
			Attempts:    int64(20 - i),    // Vary attempts for sorting validation
			Coffee:      uint64(int64(i)), // Vary coffee for sorting validation
		}

		postStmt, err := testPost.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert post to SQL: %v", err)
		}

		for _, stmt := range postStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert sample post: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("delete from post")
	}()

	// Assume we want to skip the first 10 entries and limit the output to 20 entries
	skip := 10
	limit := 20

	// Send a request to the PopularPageFeed endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"limit": "%v", "skip": "%v", "test": true}`, limit, skip)))
	req, err := http.NewRequest("GET", "http://localhost:1818/api/popular", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_PopularPageFeed failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_PopularPageFeed failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_PopularPageFeed failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_PopularPageFeed failed\n    Error: ", err)
		return
	}

	var resJson []models.Post
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_PopularPageFeed failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	// expectedJson should reflect the actual structure and data of the feeds in your application.
	expectedJson := []models.Post{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_PopularPageFeed failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_PopularPageFeed succeeded")
}
