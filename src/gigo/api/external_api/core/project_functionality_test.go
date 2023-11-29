package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/gigo-lib/config"
	config2 "github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/go-redis/redis/v8"
	"testing"
	"time"
)

func TestPublishProject(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test post
	testPost := models.Post{
		ID:       1,
		AuthorID: 1,
		Title:    "Test Post",
	}

	postStmt, err := testPost.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test post: %v", err)
		}
	}

	// Create a MeiliSearch client
	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"posts": {
				Name:                 "posts",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"title", "description", "languages", "tags"},
				FilterableAttributes: []string{"languages", "tags"},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	// Test the PublishProject function
	postID := int64(1)

	response, err := PublishProject(context.Background(), testTiDB, meili, postID)
	if err != nil {
		t.Errorf("PublishProject() error = %v", err)
		return
	}

	if response["message"] != "Post published successfully." {
		t.Errorf("PublishProject() unexpected message: %v", response["message"])
	}

	if response["post"] != "1" {
		t.Errorf("PublishProject() unexpected post ID: %v", response["post"])
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM post WHERE _id = 1")
	}()
}

func TestEditConfig(t *testing.T) {
	// Create a user
	callingUser := &models.User{
		ID: 1,
	}

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	// todo: need to use an existing repository for testing. Replace `repoId` with the ID of an existing repository.
	repoId := int64(1)

	// Use an example workspace configuration for testing.
	content := `version: 0.1
base_container: "python:3.9"
working_directory: "/app"
resources:
  cpu: 1
  mem: 1
  disk: 1
`

	response, err := EditConfig(context.Background(), vcsClient, callingUser, repoId, content, "commit message")
	if err != nil {
		t.Errorf("EditConfig() error = %v", err)
		return
	}

	if response["message"] != "repo config updated successfully." {
		t.Errorf("EditConfig() unexpected message: %v", response["message"])
	}

	if response["repo"] != fmt.Sprintf("%d", repoId) {
		t.Errorf("EditConfig() unexpected repo ID: %v", response["repo"])
	}

}

func TestGetConfig(t *testing.T) {
	//todo: create test git repository for test to work properly
	// Create a user
	callingUser := &models.User{
		ID: 1,
	}

	// Create a VCS client
	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	// You need to use an existing repository for testing. Replace `repoId` with the ID of an existing repository.
	repoId := int64(1)

	// Set the commit to test. You can use "main" or the specific commit hash.
	commit := "main"

	response, err := GetConfig(context.Background(), vcsClient, callingUser, repoId, commit)
	if err != nil {
		t.Errorf("GetConfig() error = %v", err)
		return
	}

	if wsConfig, ok := response["ws_config"].(string); !ok || wsConfig == "" {
		t.Errorf("GetConfig() unexpected or empty workspace config: %v", response["ws_config"])
	}

	// Add additional test cases for other scenarios as needed.
}

//func TestCloseAttempt(t *testing.T) {
//	tidb, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	// Create a user
//	callingUser := &models.User{
//		ID:       1,
//		UserName: "gigo",
//		Timezone: "UTC",
//		Tier:     1,
//	}
//
//	// Create a VCS client
//	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
//	if err != nil {
//		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
//	}
//
//	// You need to use an existing attempt for testing. Replace `attemptId` with the ID of an existing attempt.
//	attemptId := int64(1)
//
//	attempt, err := models.CreateAttempt(1, "title", "description", callingUser.UserName, callingUser.ID, time.Now(), time.Now(), 1, callingUser.Tier, nil, 0, 420, 1, nil, 0)
//	if err != nil {
//		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
//		return
//	}
//
//	statement, err := attempt.ToSQLNative()
//	if err != nil {
//		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
//		return
//	}
//
//	for _, stmt := range statement {
//		_, err = tidb.DB.Exec(stmt.Statement, stmt.Values...)
//	}
//
//	// Add cleanup for the attempt
//	defer func() {
//		_, err = tidb.DB.Exec(`DELETE FROM attempt`)
//		if err != nil {
//			t.Logf("Failed to delete sample attempt: %v", err)
//		}
//	}()
//
//	response, err := CloseAttempt(context.Background(), tidb, vcsClient, callingUser, attemptId)
//	if err != nil {
//		t.Errorf("CloseAttempt() error = %v", err)
//		return
//	}
//
//	if msg, ok := response["message"].(string); !ok || msg != "Attempt Closed Successfully" {
//		t.Errorf("CloseAttempt() unexpected message: %v", response["message"])
//	}
//
//	// Add additional test cases for other scenarios as needed.
//}

func TestMarkSuccess(t *testing.T) {
	tidb, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
	if err != nil {
		t.Fatal(err)
	}

	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
		Host:        "mq://gigo-dev-nats:4222",
		Username:    "gigo-dev",
		Password:    "gigo-dev",
		MaxPubQueue: 256,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	defer js.Close()

	// Create a Snowflake node
	sf, _ := snowflake.NewNode(1)

	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestDontGiveUpActive failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := callingUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = tidb.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	attempt, err := models.CreateAttempt(69, "title", "description", callingUser.UserName, callingUser.ID, time.Now(), time.Now(), 1, callingUser.Tier, nil, 0, 420, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	statement, err := attempt.ToSQLNative()
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	for _, stmt := range statement {
		_, err = tidb.DB.Exec(stmt.Statement, stmt.Values...)
	}

	_, err = tidb.DB.Exec("UPDATE attempt SET closed = true WHERE _id = ?", attempt.ID)
	if err != nil {
		t.Errorf("\nTestDontGiveUpActive failed\n    Error: %v\n", err)
		return
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = tidb.DB.Exec(`DELETE FROM attempt WHERE _id =?`, attempt.ID)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
		_, err = tidb.DB.Exec(`DELETE FROM users where _id =?`, callingUser.ID)
		if err != nil {
			t.Logf("Failed to delete sample user: %v", err)
		}
	}()

	// create variable to hold redis client
	var rdb redis.UniversalClient

	// create local client
	rdb = redis.NewClient(&redis.Options{})

	response, err := MarkSuccess(context.Background(), tidb, js, rdb, sf, attempt.ID, logger, callingUser)
	if err != nil {
		t.Errorf("MarkSuccess() error = %v", err)
		return
	}

	if msg, ok := response["message"].(string); !ok || msg != "Attempt Marked as a Success" {
		t.Errorf("MarkSuccess() unexpected message: %v", response["message"])
	}

}
