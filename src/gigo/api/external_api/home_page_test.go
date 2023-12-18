package external_api

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gage-technologies/gigo-lib/db/models"
)

func TestHTTPServer_ActiveProjectsHome(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
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

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/home/active", body)
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

	t.Log("\ndetermine liferaft integration status HTTP succeeded")
}

func TestHTTPServer_RecommendedProjectsHome(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
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

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/home/recommended", body)
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

	t.Log("\ndetermine liferaft integration status HTTP succeeded")
}

//func TestHTTPServer_FeedProjectsHome(t *testing.T) {
//	var ava models.AvatarSettings
//
//	// Create test users
//	testUser1, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
//	if err != nil {
//		t.Errorf("\nTestFeedProjectsHome failed\n    Error: %v\n", err)
//		return
//	}
//	testUser2, err := models.CreateUser(2, "testuser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
//	if err != nil {
//		t.Errorf("\nTestFeedProjectsHome failed\n    Error: %v\n", err)
//		return
//	}
//
//	userStmt, err := testUser1.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert user to SQL: %v", err)
//	}
//
//	for _, stmt := range userStmt {
//		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert test user: %v", err)
//		}
//	}
//
//	userStmt2, err := testUser2.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert user to SQL: %v", err)
//	}
//
//	for _, stmt := range userStmt2 {
//		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert test user: %v", err)
//		}
//	}
//
//	// Create a post from testUser1
//	testPost := &models.Post{
//		Title:       "Test Post",
//		Description: "Test post description",
//		Author:      testUser1.UserName,
//		AuthorID:    testUser1.ID,
//		CreatedAt:   time.Now(),
//		UpdatedAt:   time.Now(),
//		RepoID:      1,
//		Tier:        1,
//		PostType:    1,
//		Views:       10,
//		Completions: 5,
//		Attempts:    15,
//	}
//
//	postStmt, err := testPost.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert post to SQL: %v", err)
//	}
//
//	for _, stmt := range postStmt {
//		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert sample post: %v", err)
//		}
//	}
//
//	// Add a follower
//	_, err = testHttpServer.tiDB.DB.Exec("INSERT INTO follower(follower, following) VALUES (?, ?)", testUser2.ID, testUser1.ID)
//	if err != nil {
//		t.Fatalf("Failed to insert follower: %v", err)
//	}
//
//	defer func() {
//		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 1")
//		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 2")
//		_, _ = testTiDB.DB.Exec("DELETE FROM post WHERE _id = ?", testPost.ID)
//		_, _ = testTiDB.DB.Exec("DELETE FROM follower WHERE follower = ? AND following = ?", testUser2.ID, testUser1.ID)
//	}()
//
//	// Send a request to the FeedProjectsHome endpoint
//	body := bytes.NewReader([]byte(`{"test": true}`)) // The request body may need to be adjusted based on the actual data requirements.
//	req, err := http.NewRequest("POST", "http://localhost:1818/api/home/", body)
//	if err != nil {
//		t.Errorf("\nTestHTTPServer_FeedProjectsHome failed\n    Error: %v", err)
//		return
//	}
//
//	// Add user authentication token to the request
//	req.AddCookie(&http.Cookie{
//		Name:  "authToken",
//		Value: "your-auth-token", // Replace with the actual authentication token for the test user.
//	})
//
//	// Perform the request
//	client := &http.Client{}
//	res, err := client.Do(req)
//	if err != nil {
//		t.Errorf("\nTestHTTPServer_FeedProjectsHome failed\n    Error: %v", err)
//		return
//	}
//
//	// Check the response status code
//	if res.StatusCode != http.StatusOK {
//		body, _ := ioutil.ReadAll(res.Body)
//		fmt.Println(string(body))
//		fmt.Println(res.StatusCode)
//		t.Error("\nTestHTTPServer_FeedProjectsHome failed\n    Error: incorrect response code")
//		return
//	}
//
//	// Parse the response
//	resBody, err := ioutil.ReadAll(res.Body)
//	if err != nil {
//		t.Error("\nTestHTTPServer_FeedProjectsHome failed\n    Error: ", err)
//		return
//	}
//
//	var resJson map[string]interface{}
//	err = json.Unmarshal(resBody, &resJson)
//	if err != nil {
//		t.Error("\nTestHTTPServer_FeedProjectsHome failed\n    Error: ", err)
//		return
//	}
//
//	// Compare the response to the expected result
//	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
//	if !reflect.DeepEqual(resJson, expectedJson) {
//		t.Error("\nTestHTTPServer_FeedProjectsHome failed\n    Error: response JSON does not match expected JSON")
//		return
//	}
//
//	t.Log("\nTestHTTPServer_FeedProjectsHome succeeded")
//}
