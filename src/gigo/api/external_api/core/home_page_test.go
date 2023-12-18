package core

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/utils"
)

//func TestActiveProjectsHome(t *testing.T) {
//	// Setup
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
//	if err != nil {
//		t.Fatalf("Failed to initialize test database: %v", err)
//	}
//
//	id := int64(5)
//	post, err := models.CreatePost(
//		69420, "test", "content", "autor", 42069, time.Now(),
//		time.Now(), 69, 1, []int64{}, &id, 6969, 20, 40, 24, 27,
//		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil,
//		64752, 3, &models.DefaultWorkspaceSettings, false, false, nil,
//	)
//	if err != nil {
//		t.Fatalf("Failed to create test post: %v", err)
//	}
//	defer testTiDB.DB.Exec("delete from post where _id = ?;", post.ID)
//
//	postStmt, err := post.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert post to SQL: %v", err)
//	}
//
//	for _, stmt := range postStmt {
//		if _, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...); err != nil {
//			t.Fatalf("Failed to insert test post: %v", err)
//		}
//	}
//
//	attempt, err := models.CreateAttempt(69420, "test", "test", "author", 42069, time.Now(), time.Now(), 69, 2, []int64{}, uint64(3), 6969, 2, nil, 0)
//	if err != nil {
//		t.Fatalf("Failed to create test attempt: %v", err)
//	}
//	defer testTiDB.DB.Exec("delete from attempt where _id = ?;", attempt.ID)
//
//	attemptStmt, err := attempt.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert attempt to SQL: %v", err)
//	}
//
//	for _, stmt := range attemptStmt {
//		if _, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...); err != nil {
//			t.Fatalf("Failed to insert test attempt: %v", err)
//		}
//	}
//
//	res, err := ActiveProjectsHome(context.Background(), &models.User{
//		ID: 42069,
//	}, testTiDB)
//
//	if err != nil {
//		t.Fatalf("ActiveProjectsHome failed: %v", err)
//	}
//
//	projects, ok := res["projects"].([]*models.AttemptFrontend)
//	if !ok {
//		t.Fatalf("ActiveProjectsHome failed: expected []*models.AttemptFrontend")
//	}
//
//	if len(projects) != 1 {
//		t.Fatalf("ActiveProjectsHome failed: expected 1 project, got %v", len(projects))
//	}
//
//	project := projects[0]
//
//	frontattempt := attempt.ToFrontend()
//
//	if project.ID != frontattempt.ID {
//		t.Errorf("ActiveProjectsHome failed: expected ID %v, got %v", attempt.ID, project.ID)
//	}
//
//	if project.PostTitle != frontattempt.PostTitle {
//		t.Errorf("ActiveProjectsHome failed: expected Title %v, got %v", frontattempt.PostTitle, project.PostTitle)
//	}
//
//	if project.Author != frontattempt.Author {
//		t.Errorf("ActiveProjectsHome failed: expected Author %v, got %v", attempt.Author, project.Author)
//	}
//}

func TestRecommendedProjectsHome(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
	if err != nil {
		t.Fatal(err)
	}

	id := int64(5)
	post, err := models.CreatePost(69420, "test", "content", "autor", 42069, time.Now(),
		time.Now(), 69, 3, []int64{}, &id, 6969, 20, 40, 24, 27,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil,
		5862, 6, nil, false, false, nil)
	if err != nil {
		t.Error("\nrecc projects home Failed")
		return
	}

	defer testTiDB.DB.Exec("delete from post where _id = ?;", post.ID)

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestMostChallengingActive failed\n    Error: %v\n", err)
		return
	}

	stmt, err := post.ToSQLNative()
	if err != nil {
		t.Error("\nrecc projects home failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nrecc projects home failed\n    Error: ", err)
			return
		}
	}

	attempt, err := models.CreateRecommendedPost(6969, 42069, 69420, 32, 69420, 7403, time.Now(), time.Now(), models.Tier3)
	if err != nil {
		t.Error("\nCreate Attempt Post Failed")
		return
	}

	defer testTiDB.DB.Exec("delete from recommended_post where _id = ?;", attempt.ID)

	// turn the takedown struct into match sql
	statement := attempt.ToSQLNative()

	tx, err := testTiDB.DB.BeginTx(context.TODO(), nil)
	if err != nil {
		t.Error("\nget takedown templates\n    Error: ", err)
		return
	}

	stmts, err := tx.Prepare(statement.Statement)
	if err != nil {
		t.Error("\nget takedown templates\n    Error: ", err)
		return
	}

	_, err = stmts.Exec(statement.Values...)
	if err != nil {
		t.Error("\nget takedown templates\n    Error: ", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		t.Error("\nget takedown templates\n    Error: ", err)
		return
	}

	res, err := RecommendedProjectsHome(context.Background(), user, testTiDB, logger, &ReccommendedProjectsHomeRequest{
		Skip: 0,
	})
	if err != nil {
		t.Error("\nrecc projects home failed\n    Error: ", err)
		return
	}

	b, _ := json.Marshal(res["projects"])

	hash, err := utils.HashData(b)
	if err != nil {
		t.Error("recc projects home failed")
		return
	}

	fmt.Println("hash is: ", hash)

	if hash != "587a69dcbc0120e49b899b4d242c8155d831d5e27c4854cab42e192d71870e57" {
		t.Errorf("\nActive projects home failed\n    Error: answer did not equal correct response\n%s", hash)
		return
	}
}

// func TestFeedProjectsHome(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
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
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
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
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
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
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert sample post: %v", err)
//		}
//	}
//
//	// Add a follower
//	_, err = testTiDB.DB.Exec("INSERT INTO follower(follower, following) VALUES (?, ?)", testUser2.ID, testUser1.ID)
//	if err != nil {
//		t.Fatalf("Failed to insert follower: %v", err)
//	}
//
//	// Call the FeedProjectsHome function
//	result, err := FeedProjectsHome(context.Background(), testUser2, testTiDB)
//	if err != nil {
//		t.Errorf("FeedProjectsHome() error = %v", err)
//		return
//	}
//
//	projects, ok := result["projects"].([]*models.PostFrontend)
//	if !ok {
//		t.Fatalf("FeedProjectsHome() result did not contain projects")
//	}
//
//	if len(projects) != 1 {
//		t.Errorf("FeedProjectsHome() = %v, want 1 project", len(projects))
//	}
//
//	if projects[0].Title != testPost.Title {
//		t.Errorf("FeedProjectsHome() project title = %v, want %v", projects[0].Title, testPost.Title)
//	}
//
//	// Deferred removal of inserted data
//	defer func() {
//		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 1")
//		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 2")
//		_, _ = testTiDB.DB.Exec("DELETE FROM post WHERE _id = ?", testPost.ID)
//		_, _ = testTiDB.DB.Exec("DELETE FROM follower WHERE follower = ? AND following = ?", testUser2.ID, testUser1.ID)
//	}()
// }

//func TestTopRecommendations(t *testing.T) {
//	// Setup
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
//	if err != nil {
//		t.Fatalf("Failed to initialize test database: %v", err)
//	}
//
//	user := &models.User{
//		ID: 42069,
//		StartUserInfo: &models.StartUserInfo{
//			PreferredLanguage: "Go",
//			Proficiency:       3,
//		},
//	}
//
//	// Create a sample curated post
//	curatedPost, err := models.CreateCuratedPost(/* Add your required parameters here */)
//	if err != nil {
//		t.Fatalf("Failed to create test curated post: %v", err)
//	}
//	defer testTiDB.DB.Exec("delete from curated_post where _id = ?;", curatedPost.ID)
//
//	curatedPostStmt, err := curatedPost.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert curated post to SQL: %v", err)
//	}
//
//	for _, stmt := range curatedPostStmt {
//		if _, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...); err != nil {
//			t.Fatalf("Failed to insert test curated post: %v", err)
//		}
//	}
//
//	// Test the TopRecommendations function
//	ctx := context.Background()
//	res, err := TopRecommendations(ctx, user, testTiDB)
//
//	if err != nil {
//		t.Fatalf("TopRecommendations failed: %v", err)
//	}
//
//	projects, ok := res["projects"].([]*models.PostFrontend)
//	if !ok {
//		t.Fatalf("TopRecommendations failed: expected []*models.PostFrontend")
//	}
//
//	if len(projects) == 0 {
//		t.Fatalf("TopRecommendations failed: expected at least 1 project")
//	}
//
//	// For simplicity, let's check just the first project
//	project := projects[0]
//
//	if project.ID != curatedPost.PostID {
//		t.Errorf("TopRecommendations failed: expected ID %v, got %v", curatedPost.PostID, project.ID)
//	}
//
//}
