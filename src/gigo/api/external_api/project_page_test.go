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

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
)

func TestHTTPServer_ProjectPageFrontend(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: ", err)
			return
		}
	}

	awards := make([]int64, 0)

	post, err := models.CreatePost(69, "title", "content", "author", 420,
		time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
		time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
		69, 5, awards, nil, 42069, 2, 6969, 6900, 4206969,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil,
		58382, 2, nil, false, false, nil)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectPageFrontend Failed")
		return
	}

	postStmt, err := post.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: ", err)
		return
	}

	for _, s := range postStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: ", err)
			return
		}
	}

	attemptArray := []models.Attempt{
		{
			ID:          69,
			Description: "Test 1",
			Author:      "Test Author 1",
			AuthorID:    69,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  4,
			// Awards:      awards,
			Coffee: 5,
			PostID: post.ID,
			Tier:   1,
		},
		{
			ID:          420,
			Description: "Test 1",
			Author:      "Test Author 2",
			AuthorID:    69,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(520, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  3,
			// Awards:      awards,
			Coffee: 12,
			PostID: post.ID,
			Tier:   2,
		},
		{
			ID:          42069,
			Description: "Test 1",
			Author:      "Test Author 3",
			AuthorID:    69,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(620, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  1,
			// Awards:      awards,
			Coffee: 7,
			PostID: 420,
			Tier:   3,
		},
	}

	attempts := make([]*models.Attempt, 0)

	for i := 0; i < len(attemptArray); i++ {
		var attempt *models.Attempt

		attempt, err = models.CreateAttempt(attemptArray[i].ID, attemptArray[i].PostTitle, attemptArray[i].Description, attemptArray[i].Author, attemptArray[i].AuthorID,
			attemptArray[i].CreatedAt, attemptArray[i].UpdatedAt, attemptArray[i].RepoID, attemptArray[i].AuthorTier,
			nil, attemptArray[i].Coffee, attemptArray[i].PostID, attemptArray[i].Tier, attemptArray[i].ParentAttempt, 0)
		if err != nil {
			t.Errorf("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: %v\n", err)
			return
		}

		attempts = append(attempts, attempt)
	}

	for _, attempt := range attempts {
		attemptStmt, err := attempt.ToSQLNative()
		// will break here if you add awards because stmt index hard coded to 0 because im being lazy
		for _, statement := range attemptStmt {
			_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
			if err != nil {
				t.Errorf("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: %v", err)
				return
			}
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "post_id": "69"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/frontend", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_ProjectPageFrontend failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_ProjectPageFrontend HTTP succeeded")
}

func TestHTTPServer_ProjectInformation(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestProjectInformation failed\n    Error: %v\n", err)
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

	awards := make([]int64, 0)

	post, err := models.CreatePost(69, "title", "content", "author", 420, time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
		time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC), 69, 5, awards, nil, 42069, 2, 6969, 6900, 4206969,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil, 0, 0, nil, false, false, nil)
	if err != nil {
		t.Error("\nTestProjectInformation Failed")
		return
	}

	stmt, err := post.ToSQLNative()
	if err != nil {
		t.Error("\nTestProjectInformation failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectInformation failed\n    Error: ", err)
			return
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("delete from post")
	}()

	// Assume we have a post with id 1
	postId := 69

	// Send a request to the ProjectInformation endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"post_id": "%v"}`, postId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/get", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ProjectInformation failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_ProjectInformation failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ProjectInformation failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectInformation failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectInformation failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_ProjectInformation failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_ProjectInformation succeeded")
}

func TestHTTPServer_ProjectAttempts(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
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

	post, err := models.CreatePost(69, "title", "content", "author", testUser.ID, time.Now(), time.Now(), 69, 5, nil, nil, 42069, 2, 6969, 6900, 4206969, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil, 0, 0, nil, false, false, nil)
	if err != nil {
		t.Error("\nTestProjectAttempts Failed")
		return
	}

	stmt, err := post.ToSQLNative()
	if err != nil {
		t.Error("\nTestProjectAttempts failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectAttempts failed\n    Error: ", err)
			return
		}
	}

	awards := []int64{
		1,
	}

	attempt1, err := models.CreateAttempt(1, post.Title, "description", testUser.UserName, testUser.ID, time.Now(), time.Now(), post.RepoID, testUser.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	attempt2, err := models.CreateAttempt(2, post.Title, "description", testUser.UserName, testUser.ID, time.Now(), time.Now(), post.RepoID, testUser.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	stmt1, err := attempt1.ToSQLNative()
	stmt2, err := attempt2.ToSQLNative()

	for _, s := range stmt1 {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectAttempts failed\n    Error: ", err)
			return
		}
	}

	for _, s := range stmt2 {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectAttempts failed\n    Error: ", err)
			return
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("delete from post")
		testHttpServer.tiDB.DB.Exec("delete from attempt")
	}()

	// Assume we have a project with id 1
	projectID := 69

	// Define limit and skip values
	limit := 10
	skip := 0

	// Send a request to the ProjectAttempts endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"project_id": "%v", "limit": %v, "skip": %v}`, projectID, limit, skip)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/attempts", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ProjectAttempts failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_ProjectAttempts failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ProjectAttempts failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectAttempts failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_ProjectAttempts failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_ProjectAttempts failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_ProjectAttempts succeeded")
}

func TestHTTPServer_GetProjectCode(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetProjectCode failed\n    Error: %v\n", err)
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

	// Assume we have a repo with id 1
	repoID := 1
	// Assume we are looking at 'master' ref
	ref := "master"
	// Assume the file path
	filepath := "path/to/code"

	// Send a request to the GetProjectCode endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"repo_id": "%v", "ref": "%v", "filepath": "%v"}`, repoID, ref, filepath)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/getProjectCode", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetProjectCode failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetProjectCode failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetProjectCode failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectCode failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectCode failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetProjectCode failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetProjectCode succeeded")
}

func TestHTTPServer_GetClosedAttempts(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetClosedAttempts failed\n    Error: %v\n", err)
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

	post, err := models.CreatePost(69, "title", "content", "author", testUser.ID, time.Now(), time.Now(), 69, 5, nil, nil, 42069, 2, 6969, 6900, 4206969, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil, 0, 0, nil, false, false, nil)
	if err != nil {
		t.Error("\nTestGetClosedAttempts Failed")
		return
	}

	stmt, err := post.ToSQLNative()
	if err != nil {
		t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
			return
		}
	}

	awards := []int64{
		1,
	}

	attempt1, err := models.CreateAttempt(1, post.Title, "description", testUser.UserName, testUser.ID, time.Now(), time.Now(), post.RepoID, testUser.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestGetClosedAttempts failed\n    Error: %v\n", err)
		return
	}

	attempt2, err := models.CreateAttempt(2, post.Title, "description", testUser.UserName, testUser.ID, time.Now(), time.Now(), post.RepoID, testUser.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestGetClosedAttempts failed\n    Error: %v\n", err)
		return
	}

	attempt2.Closed = true // Set one of the attempts as closed

	stmt1, err := attempt1.ToSQLNative()
	if err != nil {
		t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
		return
	}

	stmt2, err := attempt2.ToSQLNative()
	if err != nil {
		t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
		return
	}
	for _, s := range stmt1 {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
			return
		}
	}

	for _, s := range stmt2 {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
			return
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("delete from attempt")
		testHttpServer.tiDB.DB.Exec("delete from post")
	}()

	// Assume we have a project with id 1
	projectID := 1
	// Assume we want to skip 0 records and limit to 10
	skip := 0
	limit := 10

	// Send a request to the GetClosedAttempts endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"project_id": "%v", "skip": %v, "limit": %v}`, projectID, skip, limit)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/closed_attempts", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetClosedAttempts failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetClosedAttempts failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetClosedAttempts failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetClosedAttempts failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetClosedAttempts failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetClosedAttempts failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetClosedAttempts succeeded")
}

func TestHTTPServer_GetProjectFile(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetProjectFile failed\n    Error: %v\n", err)
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

	// Assume we have a repo with id 1, a ref "master" and a file path "README.md"
	repoID := 1
	ref := "master"
	filePath := "README.md"

	// Send a request to the GetProjectFile endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"repo_id": "%v", "ref": "%v", "filepath": "%v"}`, repoID, ref, filePath)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/getProjectFiles", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetProjectFile failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetProjectFile failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetProjectFile failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectFile failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectFile failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetProjectFile failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetProjectFile succeeded")
}

func TestHTTPServer_GetProjectDirectories(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetProjectDirectories failed\n    Error: %v\n", err)
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

	// Assume we have a repo with id 1, a ref "master" and a file path "README.md"
	repoID := 1
	ref := "master"
	filePath := "README.md"

	// Send a request to the GetProjectDirectories endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"repo_id": "%v", "ref": "%v", "filepath": "%v"}`, repoID, ref, filePath)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/project/getProjectDirectories", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetProjectDirectories failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetProjectDirectories failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetProjectDirectories failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectDirectories failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectDirectories failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetProjectDirectories failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetProjectDirectories succeeded")
}
