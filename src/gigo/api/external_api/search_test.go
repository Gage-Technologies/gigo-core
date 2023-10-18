package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPServer_SearchBarResults(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nrecc projects home failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nrecc projects home failed\n    Error: ", err)
			return
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "keyword": "test", "skip": 2, "limit": 5}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search", body)
	if err != nil {
		t.Errorf("\nadd feedback HTTP failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nadd feedback HTTP failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nadd feedback HTTP failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nsearch bar result HTTP succeeded")
}

func TestHTTPServer_SearchUsers(t *testing.T) {
	// Create test users
	testUsers := []models.User{
		{ID: 1, UserName: "testUser1"},
		{ID: 2, UserName: "testUser2"},
		{ID: 3, UserName: "testUser3"},
	}

	for _, testUser := range testUsers {
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
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	// Send a request to the SearchUsers endpoint
	payload := `{"query": "testUser", "skip": 0, "limit": 10}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search/users", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchUsers failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_SearchUsers failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SearchUsers failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchUsers failed\n    Error: %v", err)
		return
	}

	// Unmarshal the response
	var users []models.User
	err = json.Unmarshal(body, &users)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchUsers failed\n    Error: %v", err)
		return
	}

	// Check the users
	if len(users) != 3 {
		t.Errorf("\nTestHTTPServer_SearchUsers failed\n    Error: expected 3 users, got %d", len(users))
		return
	}

	t.Log("\nTestHTTPServer_SearchUsers succeeded")
}

func TestHTTPServer_SearchTags(t *testing.T) {
	// Create test tags
	testTags := []models.Tag{
		{ID: 1, Value: "testTag1", UsageCount: 69},
		{ID: 2, Value: "testTag2", UsageCount: 69},
		{ID: 3, Value: "testTag3", UsageCount: 69},
	}

	for _, testTag := range testTags {
		tagStmt := testTag.ToSQLNative()

		for _, stmt := range tagStmt {
			_, err := testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test tag: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from tags")
	}()

	// Send a request to the SearchTags endpoint
	payload := `{"query": "testTag", "skip": 0, "limit": 10}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search/tags", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchTags failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_SearchTags failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SearchTags failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchTags failed\n    Error: %v", err)
		return
	}

	// Unmarshal the response
	var tags []models.Tag
	err = json.Unmarshal(body, &tags)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchTags failed\n    Error: %v", err)
		return
	}

	// Check the tags
	if len(tags) != 3 {
		t.Errorf("\nTestHTTPServer_SearchTags failed\n    Error: expected 3 tags, got %d", len(tags))
		return
	}

	t.Log("\nTestHTTPServer_SearchTags succeeded")
}

func TestHTTPServer_SearchDiscussions(t *testing.T) {
	// Create test discussions
	testDiscussions := []models.Discussion{
		{ID: 1, Title: "testDiscussion1", PostId: 69},
		{ID: 2, Title: "testDiscussion2", PostId: 70},
		{ID: 3, Title: "testDiscussion3", PostId: 71},
	}

	for _, testDiscussion := range testDiscussions {
		discussionStmt := testDiscussion.ToSQLNative()

		for _, stmt := range discussionStmt {
			_, err := testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test discussion: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from discussions")
	}()

	// Send a request to the SearchDiscussions endpoint
	payload := `{"query": "testDiscussion", "skip": 0, "limit": 10}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search/discussions", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchDiscussions failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_SearchDiscussions failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SearchDiscussions failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchDiscussions failed\n    Error: %v", err)
		return
	}

	// Unmarshal the response
	var discussions []models.Discussion
	err = json.Unmarshal(body, &discussions)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchDiscussions failed\n    Error: %v", err)
		return
	}

	// Check the discussions
	if len(discussions) != 3 {
		t.Errorf("\nTestHTTPServer_SearchDiscussions failed\n    Error: expected 3 discussions, got %d", len(discussions))
		return
	}

	t.Log("\nTestHTTPServer_SearchDiscussions succeeded")
}

func TestHTTPServer_SearchComments(t *testing.T) {
	// Create test comments
	testComments := []models.Comment{
		{ID: 1, Body: "testComment1", DiscussionId: 69},
		{ID: 2, Body: "testComment2", DiscussionId: 70},
		{ID: 3, Body: "testComment3", DiscussionId: 71},
	}

	for _, testComment := range testComments {
		commentStmt := testComment.ToSQLNative()

		for _, stmt := range commentStmt {
			_, err := testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test comment: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from comments")
	}()

	// Send a request to the SearchComments endpoint
	payload := `{"query": "testComment", "skip": 0, "limit": 10}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search/comments", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchComments failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_SearchComments failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SearchComments failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchComments failed\n    Error: %v", err)
		return
	}

	// Unmarshal the response
	var comments []models.Comment
	err = json.Unmarshal(body, &comments)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchComments failed\n    Error: %v", err)
		return
	}

	// Check the comments
	if len(comments) != 3 {
		t.Errorf("\nTestHTTPServer_SearchComments failed\n    Error: expected 3 comments, got %d", len(comments))
		return
	}

	t.Log("\nTestHTTPServer_SearchComments succeeded")
}

func TestHTTPServer_SearchWorkspaceConfigs(t *testing.T) {
	// Create test workspace configurations
	testWorkspaceConfigs := []models.WorkspaceConfig{
		{ID: 1, Title: "config1", Languages: []models.ProgrammingLanguage{1, 2}, Description: "test", Content: "test", AuthorID: 69, Tags: []int64{1, 2, 3}},
		{ID: 2, Title: "config2", Languages: []models.ProgrammingLanguage{3, 4}, Description: "test", Content: "test", AuthorID: 69, Tags: []int64{4, 5, 6}},
		{ID: 3, Title: "config3", Languages: []models.ProgrammingLanguage{5, 6}, Description: "test", Content: "test", AuthorID: 69, Tags: []int64{7, 8, 9}},
	}

	for _, config := range testWorkspaceConfigs {
		configStmt, err := config.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert config to SQL: %v", err)
		}

		for _, stmt := range configStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test config: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from workspace_configs")
	}()

	// Send a request to the SearchWorkspaceConfigs endpoint
	payload := `{
		"query": "config",
		"skip": 0,
		"limit": 10,
		"languages": [1, 2, 3, 4, 5, 6],
		"tags": [{"_id": "1"}, {"_id": "2"}, {"_id": "3"}]
	}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search/workspaceConfigs", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchWorkspaceConfigs failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_SearchWorkspaceConfigs failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SearchWorkspaceConfigs failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchWorkspaceConfigs failed\n    Error: %v", err)
		return
	}

	// Unmarshal the response
	var configs []models.WorkspaceConfig
	err = json.Unmarshal(body, &configs)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SearchWorkspaceConfigs failed\n    Error: %v", err)
		return
	}

	// Check the tags
	if len(configs) != 3 {
		t.Errorf("\nTestHTTPServer_SearchWorkspaceConfigs failed\n    Error: expected 3 configs, got %d", len(configs))
		return
	}

	t.Log("\nTestHTTPServer_SearchWorkspaceConfigs succeeded")
}
