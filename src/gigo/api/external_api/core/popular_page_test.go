package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
)

func TestPopularPageFeed(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	// Create test user
	testUser, err := models.CreateUser(1, "testuser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestPopularPageFeed failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
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

	// Call the PopularPageFeed function
	result, err := PopularPageFeed(context.Background(), 0, 5, testTiDB)
	if err != nil {
		t.Errorf("PopularPageFeed() error = %v", err)
		return
	}

	feed, ok := result["feed"].([]*models.PostFrontend)
	if !ok {
		t.Fatalf("PopularPageFeed() result did not contain feed")
	}

	if len(feed) != 5 {
		t.Errorf("PopularPageFeed() = %v, want 5 posts", len(feed))
	}

	for i := 0; i < len(feed)-1; i++ {
		if feed[i].Coffee < feed[i+1].Coffee || (feed[i].Coffee == feed[i+1].Coffee && feed[i].Attempts < feed[i+1].Attempts) {
			t.Errorf("PopularPageFeed() posts not sorted correctly by coffee and attempts")
		}
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		for i := 1; i <= 5; i++ {
			_, _ = testTiDB.DB.Exec("DELETE FROM post")
		}
	}()
}
