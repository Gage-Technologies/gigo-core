package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestHTTPServer_GetDiscussions(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
		testHttpServer.tiDB.DB.Exec("DELETE FROM discussion")
		testHttpServer.tiDB.DB.Exec("DELETE FROM discussion_awards")
		testHttpServer.tiDB.DB.Exec("DELETE FROM discussion_tags")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: ", err)
			return
		}
	}

	awards := make([]int64, 0)

	tags := []int64{1, 2}

	discussions := []models.Discussion{
		{
			ID:         69,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:  time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Awards:     awards,
			Coffee:     5,
			PostId:     420,
			Title:      "title",
			Tags:       tags,
			Revision:   0,
		},
		{
			ID:         69,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:  time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Awards:     awards,
			Coffee:     5,
			PostId:     420,
			Title:      "title",
			Tags:       tags,
			Revision:   1,
		},
		{
			ID:         6969,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:  time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Awards:     awards,
			Coffee:     5,
			PostId:     420,
			Title:      "title",
			Tags:       tags,
			Revision:   0,
		},
	}

	for i := 0; i < len(discussions); i++ {
		discussion, err := models.CreateDiscussion(discussions[i].ID, discussions[i].Body, discussions[i].Author, discussions[i].AuthorID,
			discussions[i].CreatedAt, discussions[i].UpdatedAt, discussions[i].AuthorTier, discussions[i].Awards, discussions[i].Coffee,
			discussions[i].PostId, discussions[i].Title, discussions[i].Tags, discussions[i].Leads, discussions[i].Revision, discussions[i].DiscussionLevel)
		if err != nil {
			t.Errorf("\nTestHTTPServer_GetDiscussions failed\n    Error: %v\n", err)
			return
		}

		statement := discussion.ToSQLNative()

		for _, stmt := range statement {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestHTTPServer_GetDiscussions failed\n    Error: %v", err)
				return
			}
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "post_id": "420", "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/discussion/getDiscussions", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussions failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussions failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: incorrect response code")
		return
	}
	// Check response JSON.
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: ", err)
		return
	}

	// Check that the response JSON contains the expected data.
	// You need to replace the expected JSON below with the actual expected JSON.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetDiscussions HTTP succeeded")
}

func TestHTTPServer_GetComments(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, uint64(0))
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetDiscussionCommentLeads failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetDiscussionCommentLeads failed\n    Error: ", err)
			return
		}
	}

	awards := make([]int64, 0)

	comments := []models.Comment{
		{
			ID:           69,
			Body:         "Test 1",
			Author:       "Test Author 1",
			AuthorID:     69,
			CreatedAt:    time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:   1,
			Awards:       awards,
			Coffee:       5,
			DiscussionId: 42069,
			Leads:        true,
			Revision:     0,
		},
		{
			ID:           69,
			Body:         "Test 2",
			Author:       "Test Author 1",
			AuthorID:     69,
			CreatedAt:    time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:   1,
			Awards:       awards,
			Coffee:       5,
			DiscussionId: 42069,
			Leads:        true,
			Revision:     1,
		},
		{
			ID:           420,
			Body:         "Test 1",
			Author:       "Test Author 1",
			AuthorID:     69,
			CreatedAt:    time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:   1,
			Awards:       awards,
			Coffee:       5,
			DiscussionId: 42069,
			Leads:        true,
			Revision:     0,
		},
	}

	for i := 0; i < len(comments); i++ {
		comment, err := models.CreateComment(comments[i].ID, comments[i].Body, comments[i].Author, comments[i].AuthorID, comments[i].CreatedAt, comments[i].AuthorTier,
			comments[i].Awards, comments[i].Coffee, comments[i].DiscussionId, comments[i].Leads, comments[i].Revision, 1)
		if err != nil {
			t.Errorf("\nTestGetDiscussionComments failed\n    Error: %v\n", err)
			return
		}

		statement := comment.ToSQLNative()

		for _, stmt := range statement {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestGetDiscussionComments failed\n    Error: %v", err)
				return
			}
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "discussion_id": "42069", "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/discussion/getComments", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussionCommentLeads failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussionCommentLeads failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetDiscussionCommentLeads failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_GetDiscussionCommentLeads HTTP succeeded")
}

func TestHTTPServer_GetDiscussionCommentThread(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, uint64(0))
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: ", err)
			return
		}
	}

	// awards := make([]int64, 0)

	// comments := []models.Comment{
	//	{
	//		ID:              69,
	//		Body:            "Test 1",
	//		Author:          "Test Author 1",
	//		AuthorID:        69,
	//		CreatedAt:       time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
	//		AuthorTier:      1,
	//		Awards:          awards,
	//		Coffee:          5,
	//		DiscussionId:    420,
	//		Lead:            true,
	//		CommentNumber:   69420,
	//		CommentThreadId: 42069,
	//	},
	//	{
	//		ID:              69,
	//		Body:            "Test 1",
	//		Author:          "Test Author 1",
	//		AuthorID:        69,
	//		CreatedAt:       time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
	//		AuthorTier:      1,
	//		Awards:          awards,
	//		Coffee:          5,
	//		DiscussionId:    420,
	//		Lead:            true,
	//		CommentNumber:   69420,
	//		CommentThreadId: 42069,
	//	},
	//	{
	//		ID:              69,
	//		Body:            "Test 1",
	//		Author:          "Test Author 1",
	//		AuthorID:        69,
	//		CreatedAt:       time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
	//		AuthorTier:      1,
	//		Awards:          awards,
	//		Coffee:          5,
	//		DiscussionId:    420,
	//		Lead:            true,
	//		CommentNumber:   69420,
	//		CommentThreadId: 42069,
	//	},
	// }

	// for i := 0; i < len(comments); i++ {
	//	comment, err := models.CreateComment(comments[i].ID, comments[i].Body, comments[i].Author, comments[i].AuthorID, comments[i].CreatedAt, comments[i].AuthorTier,
	//		comments[i].Awards, comments[i].Coffee, comments[i].DiscussionId, comments[i].Lead, comments[i].CommentNumber, comments[i].CommentThreadId)
	//	if err != nil {
	//		t.Errorf("\nGetDiscussionCommentLeads failed\n    Error: %v\n", err)
	//		return
	//	}
	//
	//	statement := comment.ToSQLNative()
	//
	//	for _, stmt := range statement {
	//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
	//		if err != nil {
	//			t.Errorf("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: %v", err)
	//			return
	//		}
	//	}
	// }

	body := bytes.NewReader([]byte(`{"test":true, "comment_thread_id": "42069", "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/projectDiscussion/getThread", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_GetDiscussionCommentThread HTTP succeeded")
}

func TestHTTPServer_GetCommentSideThread(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, uint64(0))
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetCommentSideThread failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetCommentSideThread failed\n    Error: ", err)
			return
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "comment_id": "42069", "skip": 0, "limit": 10}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/projectDiscussion/getSideThread", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetCommentSideThread failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetCommentSideThread failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetCommentSideThread failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_GetCommentSideThread HTTP succeeded")
}

func TestHTTPServer_EditDiscussions(t *testing.T) {
	badges := []int64{1, 2}
	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, uint64(0))
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: ", err)
			return
		}
	}

	comment, err := models.CreateComment(69, "body", user.UserName, user.ID, time.Now(), user.Tier,
		[]int64{1, 2, 3}, 0, 55, false, 0, 1)
	if err != nil {
		t.Errorf("\nGetDiscussionCommentLeads failed\n    Error: %v\n", err)
		return
	}

	statement := comment.ToSQLNative()

	for _, stmt := range statement {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Errorf("\nTestHTTPServer_GetDiscussionCommentThread failed\n    Error: %v", err)
			return
		}
	}

	// comment, err := core.CreateComment(testHttpServer.tiDB, testMeili, user, testSnowflakeNode, 69, "body")
	// if err != nil {
	//	t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
	//	return
	// }
	//
	// thread, err := core.CreateThreadComment(testHttpServer.tiDB, testMeili, user, testSnowflakeNode, 69, "body")
	// if err != nil {
	//	t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
	//	return
	// }

	body := bytes.NewReader([]byte(`{"test":true, "_id": "69", "discussion_type": "1", "body": "edited body, tags: ""}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/discussion/editDiscussions", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussions failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetDiscussions failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetDiscussions failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_GetDiscussions HTTP succeeded")
}
