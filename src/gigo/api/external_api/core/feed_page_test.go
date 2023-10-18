package core

import (
	"context"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"testing"
)

func TestFeedPage(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}
	defer testTiDB.DB.Close()

	// Insert some test data into the post and follower tables
	callingUser := &models.User{ID: 1}
	followingUser := &models.User{ID: 2}

	posts := []models.Post{
		{ID: 1, AuthorID: followingUser.ID, Title: "Test Post 1"},
		{ID: 2, AuthorID: followingUser.ID, Title: "Test Post 2"},
	}

	follower := models.Follower{Follower: callingUser.ID, Following: followingUser.ID}

	// Insert follower into the database
	followerStmt := follower.ToSQLNative()
	_, err = testTiDB.DB.Exec(followerStmt.Statement, followerStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert follower: %v", err)
	}

	// Insert posts into the database
	for _, post := range posts {
		postStmt, err := post.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to insert post: %v", err)
		}
		for _, stmt := range postStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert post: %v", err)
			}
		}
	}

	// Call the function being tested
	skip := 0
	limit := 2

	response, err := FeedPage(context.Background(), callingUser, testTiDB, skip, limit)
	if err != nil {
		t.Fatalf("Failed to get feed page: %v", err)
	}

	// Assert the expected response
	expectedKey := "projects"
	val, ok := response[expectedKey]
	if !ok {
		t.Fatalf("Expected key '%s' in response, but not found", expectedKey)
	}

	// Assert the type of the value is a slice of *models.PostFrontend
	projects, ok := val.([]*models.PostFrontend)
	if !ok {
		t.Fatalf("Expected value of type []*models.PostFrontend, but got %T", val)
	}

	// Assert the length of the projects slice is equal to the number of test data inserted
	expectedLength := len(posts)
	if expectedLength != len(projects) {
		t.Fatalf("Expected %d projects, but got %d", expectedLength, len(projects))
	}

	// Assert the titles of the projects match the inserted data
	for i, post := range posts {
		if post.Title != projects[i].Title {
			t.Fatalf("Expected project title '%s', but got '%s'", post.Title, projects[i].Title)
		}
	}

	t.Log("TestFeedPage succeeded")
}
