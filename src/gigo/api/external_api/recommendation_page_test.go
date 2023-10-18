package external_api

import (
	"bytes"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestHTTPServer_TopRecommendation(t *testing.T) {
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
		t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: ", err)
			return
		}
	}

	ids := []int64{42069, 69420, 6942069}

	//authorIds := []int64{6969, 420420, 42069420}
	//
	//similarities := []int64{5, 25, 50}
	//
	//postIds := []int64{69420, 42069, 6942069}
	//
	//tx, err := testTiDB.DB.BeginTx(context.TODO(), nil)
	//if err != nil {
	//	t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: ", err)
	//	return
	//}

	//for i := 0; i < len(ids); i++ {
	//	rp, err := models.CreateRecommendedPost(ids[i], authorIds[i], postIds[i], similarities[i])
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed")
	//		return
	//	}
	//
	//	statement := rp.ToSQLNative()
	//
	//	stmts, err := tx.Prepare(statement.Statement)
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed\n    Error: ", err)
	//		return
	//	}
	//
	//	_, err = stmts.Exec(statement.Values...)
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed\n    Error: ", err)
	//		return
	//	}
	//}

	var topReply *int64

	for i := 0; i < len(ids); i++ {
		p, err := models.CreatePost(ids[i], "test", "content", "author", 42069, time.Now(),
			time.Now(), 69, 3, []int64{}, topReply, 6969, 20, 40, 24,
			27, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil,
			nil, 57593, 3, nil, false, false, nil)
		if err != nil {
			t.Error("\nTestHTTPServer_TopRecommendation Failed")
			return
		}

		stmt, err := p.ToSQLNative()
		if err != nil {
			t.Error("\nTestHTTPServer_TopRecommendation Failed\n    Error: ", err)
			return
		}

		for _, s := range stmt {
			_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
			if err != nil {
				t.Error("\nTestProjectInformation failed\n    Error: ", err)
				return
			}
		}
	}

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/active/get", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_TopRecommendation failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_TopRecommendation failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_TopRecommendation HTTP succeeded")
}

func TestHTTPServer_RecommendByAttempt(t *testing.T) {

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, uint64(0))
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_RecommendByAttempt failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_RecommendByAttempt failed\n    Error: ", err)
			return
		}
	}

	awards := make([]int64, 0)

	attempts := []models.Attempt{
		{
			ID:          69,
			Description: "Test 1",
			Author:      "Test Author 1",
			AuthorID:    42069,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  4,
			// Awards:      awards,
			Coffee: 5,
			PostID: 69,
			Tier:   1,
		},
		{
			ID:          420,
			Description: "Test 1",
			Author:      "Test Author 2",
			AuthorID:    42069,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(520, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  3,
			// Awards:      awards,
			Coffee: 12,
			PostID: 420,
			Tier:   2,
		},
		{
			ID:          42069,
			Description: "Test 1",
			Author:      "Test Author 3",
			AuthorID:    42069,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(620, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  1,
			// Awards:      awards,
			Coffee: 7,
			PostID: 6942069,
			Tier:   3,
		},
	}

	tx, err := testHttpServer.tiDB.DB.Begin()
	if err != nil {
		t.Errorf("\nRecommendByAttempt failed\n    Error: %v\n", err)
		return
	}

	for _, attempt := range attempts {
		stmt, err := attempt.ToSQLNative()
		// will break here if you add awards because stmt index hard coded to 0 because im being lazy
		for _, statement := range stmt {
			_, err = testHttpServer.tiDB.DB.Exec(statement.Statement, statement.Values...)
			if err != nil {
				t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
				return
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		t.Errorf("\nRecommendByAttempt failed\n    Error: %v\n", err)
		return
	}

	var topReply *int64

	posts := []models.Post{
		{
			ID:          69,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
		{
			ID:          420,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
		{
			ID:          6942069,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
	}

	for i := 0; i < len(posts); i++ {
		p, err := models.CreatePost(posts[i].ID, posts[i].Title, posts[i].Description, posts[i].Author,
			posts[i].AuthorID, posts[i].CreatedAt, posts[i].UpdatedAt, posts[i].RepoID, posts[i].Tier,
			posts[i].Awards, posts[i].TopReply, posts[i].Coffee, posts[i].PostType, 69, posts[i].Completions,
			posts[i].Attempts, posts[i].Languages, posts[i].Visibility, posts[i].Tags, nil, nil,
			4275482, 3, &models.DefaultWorkspaceSettings, false, false, nil)
		if err != nil {
			t.Error("\nRecommendByAttempt failed")
			return
		}

		statements, err := p.ToSQLNative()

		for _, statement := range statements {
			_, err = testHttpServer.tiDB.DB.Exec(statement.Statement, statement.Values...)
			if err != nil {
				t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
				return
			}
		}
	}

	//recommendedPosts := []models.RecommendedPost{
	//	{
	//		ID:         69,
	//		PostID:     posts[0].ID,
	//	},
	//	{
	//		ID:         69,
	//		PostID:     posts[1].ID,
	//	},
	//	{
	//		ID:         69,
	//		PostID:     posts[2].ID,
	//	},
	//}

	//for i := 0; i < len(recommendedPosts); i++ {
	//	rp, err := models.CreateRecommendedPost(recommendedPosts[i].ID, recommendedPosts[i].PostID)
	//	if err != nil {
	//		t.Error("\nRecommendByAttempt failed")
	//		return
	//	}
	//
	//	statement := rp.ToSQLNative()
	//
	//	_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
	//	if err != nil {
	//		t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
	//		return
	//	}
	//
	//}

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/recommendation/attempt", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RecommendByAttempt failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RecommendByAttempt failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_RecommendByAttempt failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_RecommendByAttempt HTTP succeeded")
}

func TestHTTPServer_HarderRecommendation(t *testing.T) {
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
		t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: ", err)
			return
		}
	}

	//ids := []int64{42069, 69420, 6942069}
	//
	//authorIds := []int64{6969, 420420, 42069420}
	//
	//similarities := []int64{5, 25, 50}
	//
	//postIds := []int64{69420, 42069, 6942069}
	//
	//tx, err := testTiDB.DB.BeginTx(context.TODO(), nil)
	//if err != nil {
	//	t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: ", err)
	//	return
	//}

	//for i := 0; i < len(ids); i++ {
	//	//rp, err := models.CreateRecommendedPost(ids[i], authorIds[i], postIds[i], similarities[i])
	//	//if err != nil {
	//	//	t.Error("\nTestHTTPServer_TopRecommendation Failed")
	//	//	return
	//	//}
	//
	//	statement := rp.ToSQLNative()
	//
	//	stmts, err := tx.Prepare(statement.Statement)
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed\n    Error: ", err)
	//		return
	//	}
	//
	//	_, err = stmts.Exec(statement.Values...)
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed\n    Error: ", err)
	//		return
	//	}
	//}

	//var topReply *int64
	//
	//tiers := []models.TierType{1, 2, 3}

	//for i := 0; i < len(ids); i++ {
	//	p, err := models.CreatePost(ids[i], "test", "content", "author", 42069, time.Now(),
	//		time.Now(), 69, tiers[i], []int64{}, topReply, 6969, 20, 40, 24,
	//		27, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil,
	//		nil, 27584, 2, &models.DefaultWorkspaceSettings, false, false)
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed")
	//		return
	//	}
	//
	//	stmt, err := p.ToSQLNative()
	//	if err != nil {
	//		t.Error("\nTestHTTPServer_TopRecommendation Failed\n    Error: ", err)
	//		return
	//	}
	//
	//	for _, s := range stmt {
	//		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
	//		if err != nil {
	//			t.Error("\nTestProjectInformation failed\n    Error: ", err)
	//			return
	//		}
	//	}
	//}

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/recommendation/harder", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_TopRecommendation failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_TopRecommendation failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_TopRecommendation failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nTestHTTPServer_TopRecommendation HTTP succeeded")
}
