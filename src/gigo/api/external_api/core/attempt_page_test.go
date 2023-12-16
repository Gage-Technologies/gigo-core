package core

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/search"
)

func TestProjectAttemptInformation(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestMostChallengingActive failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Insert sample post data
	samplePost := &models.Post{
		ID:          1,
		Title:       "Test Post",
		Description: "Test Description",
		Tier:        1,
		Coffee:      69,
		Author:      "test",
		AuthorID:    user.ID,
		CreatedAt:   time.Now().Add(-6 * 24 * time.Hour), // 6 days ago
		UpdatedAt:   time.Now().Add(-6 * 24 * time.Hour),
	}
	postStmt, err := samplePost.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample post: %v", err)
		}
	}

	// Insert sample attempt data
	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    user.ID,
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
		UpdatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		PostID:      1,
	}
	attemptStmt, err := sampleAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert attempt to SQL: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample attempt: %v", err)
		}
	}

	tests := []struct {
		name       string
		tidb       *ti.Database
		vcsClient  *git.VCSClient
		attemptId  int64
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name:      "Test ProjectAttemptInformation",
			tidb:      testTiDB,
			vcsClient: vcsClient,
			attemptId: 1, // Set the attemptId according to your test data
			wantErr:   false,
			wantResult: map[string]interface{}{
				"description": "", // Expected description
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ProjectAttemptInformation(context.Background(), tt.tidb, tt.vcsClient, tt.attemptId)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProjectAttemptInformation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("ProjectAttemptInformation() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}

	defer func() {
		_, err := testTiDB.DB.Exec("DELETE FROM attempt")
		if err != nil {
			t.Fatalf("Failed to cleanup sample attempt: %v", err)
		}
		_, err = testTiDB.DB.Exec("DELETE FROM post")
		if err != nil {
			t.Fatalf("Failed to cleanup sample attempt: %v", err)
		}
		_, err = testTiDB.DB.Exec("DELETE FROM users")
		if err != nil {
			t.Fatalf("Failed to cleanup sample attempt: %v", err)
		}
	}()

}

func TestGetAttemptCode(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(1000, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 1000, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetAttemptCode failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	tests := []struct {
		name        string
		vcsClient   *git.VCSClient
		callingUser *models.User
		repo        string
		ref         string
		filePath    string
		wantErr     bool
		wantResult  map[string]interface{}
	}{
		{
			name:        "Test GetAttemptCode",
			vcsClient:   vcsClient,
			callingUser: user,
			repo:        "testRepo", // Replace with actual repo name
			ref:         "main",     // Replace with actual ref
			filePath:    "testFile", // Replace with actual file path
			wantErr:     false,
			wantResult: map[string]interface{}{
				"message": "Test project contents", // Replace with expected project contents
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := GetAttemptCode(context.Background(), tt.vcsClient, tt.callingUser, tt.repo, tt.ref, tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAttemptCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("GetAttemptCode() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}

	defer func() {
		_, err = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", user.ID)
		if err != nil {
			t.Fatalf("Failed to cleanup test user: %v", err)
		}
	}()
}

//func TestAttemptInformation(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
//	if err != nil {
//		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
//	}
//
//	var ava models.AvatarSettings
//
//	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
//	if err != nil {
//		t.Errorf("\nTestMostChallengingActive failed\n    Error: %v\n", err)
//		return
//	}
//
//	userStmt, err := user.ToSQLNative()
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
//	samplePost := &models.Post{
//		ID:          1,
//		Title:       "Test Post",
//		Description: "Test Description",
//		Tier:        1,
//		Coffee:      69,
//		Author:      "test",
//		AuthorID:    user.ID,
//		CreatedAt:   time.Now().Add(-6 * 24 * time.Hour), // 6 days ago
//		UpdatedAt:   time.Now().Add(-6 * 24 * time.Hour),
//	}
//	postStmt, err := samplePost.ToSQLNative()
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
//	sampleAttempt := &models.Attempt{
//		ID:          1,
//		PostTitle:   "Test Attempt",
//		Description: "Test Description",
//		Author:      "test",
//		AuthorID:    user.ID,
//		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
//		UpdatedAt:   time.Now().Add(-5 * 24 * time.Hour),
//		PostID:      1,
//	}
//	attemptStmt, err := sampleAttempt.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert attempt to SQL: %v", err)
//	}
//
//	for _, stmt := range attemptStmt {
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert sample attempt: %v", err)
//		}
//	}
//
//	// Set up test cases
//	tests := []struct {
//		name       string
//		tidb       *ti.Database
//		vcsClient  *git.VCSClient
//		attemptId  int64
//		wantErr    bool
//		wantResult map[string]interface{}
//	}{
//		{
//			name:      "Test AttemptInformation",
//			tidb:      testTiDB,
//			vcsClient: vcsClient,
//			attemptId: 1,
//			wantErr:   false,
//			wantResult: map[string]interface{}{
//				"post":        models.AttemptFrontend{}, // Replace with expected AttemptPostMergeFrontend object
//				"description": "Test description",       // Replace with expected description
//			},
//		},
//		// Add more test cases if needed
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			result, err := AttemptInformation(context.Background(), tt.tidb, tt.vcsClient, tt.attemptId)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("AttemptInformation() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			fmt.Println(fmt.Sprintf("%v", result))
//		})
//	}
//
//	// Deferred removal of inserted data
//	defer func() {
//		_, _ = testTiDB.DB.Exec("DELETE FROM attempt WHERE _id = 1")
//		_, _ = testTiDB.DB.Exec("DELETE FROM post WHERE _id = 1")
//		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 1")
//	}()
//}

func TestEditDescription(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"posts": {
				Name:                 "posts",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"title", "description", "author"},
				FilterableAttributes: []string{"author_id"},
				SortableAttributes:   []string{"created_at"},
			},
		},
	}
	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestMostChallengingActive failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Insert sample post data
	samplePost := &models.Post{
		ID:          1,
		Title:       "Test Post",
		Description: "Test Description",
		Tier:        1,
		Coffee:      69,
		Author:      "testuser",
		AuthorID:    user.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	postStmt, err := samplePost.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample post: %v", err)
		}
	}

	newDescription := "Updated Test Description"
	result, err := EditDescription(context.Background(), samplePost.ID, meili, true, newDescription, testTiDB)
	if err != nil {
		t.Errorf("EditDescription() error = %v", err)
		return
	}

	expectedResult := map[string]interface{}{"message": "Edit successful"}
	if !reflect.DeepEqual(result, expectedResult) {
		t.Errorf("EditDescription() = %v, want %v", result, expectedResult)
	}

	// Check if the post description was updated
	var updatedDescription string
	err = testTiDB.DB.QueryRow("SELECT description FROM post WHERE _id = ?", samplePost.ID).Scan(&updatedDescription)
	if err != nil {
		t.Errorf("Failed to fetch updated description: %v", err)
		return
	}

	if updatedDescription != newDescription {
		t.Errorf("Updated description mismatch, got: %v, want: %v", updatedDescription, newDescription)
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM post WHERE _id = 1")
	}()
}
