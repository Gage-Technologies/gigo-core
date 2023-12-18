package core

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"gigo-core/gigo/api/external_api/core/query_models"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
)

func TestProjectInformation(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestProjectInformation failed\n    Error: %v\n", err)
		return
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
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectInformation failed\n    Error: ", err)
			return
		}
	}

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	projInfo, err := ProjectInformation(context.Background(), testTiDB, vcsClient, callingUser, post.ID)
	if err != nil {
		t.Error("\nTestProjectInformation failed\n    Error: ", err)
		return
	}

	if projInfo == nil {
		t.Error("\nTestProjectInformation failed\n    Error: ", err)
		return
	}

	if !reflect.DeepEqual(projInfo["post"].(models.PostFrontend).ID, fmt.Sprintf("%v", post.ID)) {
		t.Error("\nTestProjectInformation failed\n    Error: ", err)
		return
	}

	if !reflect.DeepEqual(projInfo["post"].(models.PostFrontend).Title, post.Title) {
		t.Error("\nTestProjectInformation failed\n    Error: ", err)
		return
	}

	t.Log("\nTestProjectInformation succeeded")
}

func TestProjectAttempts(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings
	user, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	post, err := models.CreatePost(69, "title", "content", "author", user.ID, time.Now(), time.Now(), 69, 5, nil, nil, 42069, 2, 6969, 6900, 4206969, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil, 0, 0, nil, false, false, nil)
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
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectAttempts failed\n    Error: ", err)
			return
		}
	}

	awards := []int64{
		1,
	}

	attempt1, err := models.CreateAttempt(1, post.Title, "description", user.UserName, user.ID, time.Now(), time.Now(), post.RepoID, user.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	attempt2, err := models.CreateAttempt(2, post.Title, "description", user.UserName, user.ID, time.Now(), time.Now(), post.RepoID, user.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}

	stmt1, err := attempt1.ToSQLNative()
	stmt2, err := attempt2.ToSQLNative()

	for _, s := range stmt1 {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectAttempts failed\n    Error: ", err)
			return
		}
	}

	for _, s := range stmt2 {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestProjectAttempts failed\n    Error: ", err)
			return
		}
	}

	skip := 0
	limit := 10

	attemptsInfo, err := ProjectAttempts(context.Background(), testTiDB, post.ID, skip, limit)
	if err != nil {
		t.Error("\nTestProjectAttempts failed\n    Error: ", err)
		return
	}

	if attemptsInfo == nil {
		t.Error("\nTestProjectAttempts failed\n    Error: ", err)
		return
	}

	attempts := attemptsInfo["attempts"].([]*query_models.AttemptUserBackgroundFrontend)

	if len(attempts) != 2 {
		t.Errorf("\nTestProjectAttempts failed\n    Expected: 2 attempts, Got: %v attempts\n", len(attempts))
		return
	}

	for _, attempt := range attempts {
		if attempt.PostID != fmt.Sprintf("%v", post.ID) {
			t.Error("\nTestProjectAttempts failed\n    Error: Incorrect post ID")
			return
		}
		if attempt.PostTitle != post.Title {
			t.Error("\nTestProjectAttempts failed\n    Error: Incorrect post title")
			return
		}
	}

	t.Log("\nTestProjectAttempts succeeded")
}

func TestGetProjectCode(t *testing.T) {
	// Setup test VCS client with a public repo
	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatalf("failed to create VCS client: %v", err)
	}

	// Prepare test data
	repoID := int64(1) // Use a valid public repo ID
	ref := "master"    // Use a valid reference (branch or commit)
	filePath := ""     // Use a valid file path

	// Call the GetProjectCode function
	projectCode, err := GetProjectCode(context.Background(), vcsClient, repoID, ref, filePath)
	if err != nil {
		t.Errorf("TestGetProjectCode failed: %v", err)
		return
	}

	// Check the project code result
	if projectCode == nil {
		t.Error("TestGetProjectCode failed: nil result")
		return
	}

	// Check if the returned result contains the expected key
	if _, ok := projectCode["message"]; !ok {
		t.Error("TestGetProjectCode failed: 'message' key not found in the result")
		return
	}

	// You can add more specific checks based on the expected structure of the project code data
	// For example, if you know that the test repo has specific files, you can check if they are present in the result

	t.Log("TestGetProjectCode succeeded")
}

func TestGetProjectFile(t *testing.T) {
	// Setup test VCS client with a public repo
	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatalf("failed to create VCS client: %v", err)
	}

	// Prepare test data
	repoID := int64(1) // Use a valid public repo ID
	ref := "master"    // Use a valid reference (branch or commit)
	filePath := ""     // Use a valid file path

	// Call the GetProjectFile function
	projectFile, err := GetProjectFile(context.Background(), vcsClient, repoID, ref, filePath)
	if err != nil {
		t.Errorf("TestGetProjectFile failed: %v", err)
		return
	}

	// Check the project file result
	if projectFile == nil {
		t.Error("TestGetProjectFile failed: nil result")
		return
	}

	// Check if the returned result contains the expected key
	if _, ok := projectFile["message"]; !ok {
		t.Error("TestGetProjectFile failed: 'message' key not found in the result")
		return
	}

	// You can add more specific checks based on the expected structure of the project file data
	// For example, if you know that the test repo has a specific file, you can check if the returned file matches the expected content

	t.Log("TestGetProjectFile succeeded")
}

func TestGetProjectDirectories(t *testing.T) {
	// Setup test VCS client with a public repo
	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatalf("failed to create VCS client: %v", err)
	}

	// Prepare test data
	repoID := int64(1) // Use a valid public repo ID
	ref := "master"    // Use a valid reference (branch or commit)
	filePath := ""     // Use a valid file path (use root path or a valid directory path)

	// Call the GetProjectDirectories function
	projectDirectories, err := GetProjectDirectories(context.Background(), vcsClient, repoID, ref, filePath)
	if err != nil {
		t.Errorf("TestGetProjectDirectories failed: %v", err)
		return
	}

	// Check the project directories result
	if projectDirectories == nil {
		t.Error("TestGetProjectDirectories failed: nil result")
		return
	}

	// Check if the returned result contains the expected key
	if _, ok := projectDirectories["message"]; !ok {
		t.Error("TestGetProjectDirectories failed: 'message' key not found in the result")
		return
	}

	// You can add more specific checks based on the expected structure of the project directories data
	// For example, you can check if the returned directories/files match the expected content

	t.Log("TestGetProjectDirectories succeeded")
}

func TestGetClosedAttempts(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings
	user, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetClosedAttempts failed\n    Error: %v\n", err)
		return
	}

	post, err := models.CreatePost(69, "title", "content", "author", user.ID, time.Now(), time.Now(), 69, 5, nil, nil, 42069, 2, 6969, 6900, 4206969, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil, 0, 0, nil, false, false, nil)
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
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
			return
		}
	}

	awards := []int64{
		1,
	}

	attempt1, err := models.CreateAttempt(1, post.Title, "description", user.UserName, user.ID, time.Now(), time.Now(), post.RepoID, user.Tier, awards, 0, post.ID, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestGetClosedAttempts failed\n    Error: %v\n", err)
		return
	}

	attempt2, err := models.CreateAttempt(2, post.Title, "description", user.UserName, user.ID, time.Now(), time.Now(), post.RepoID, user.Tier, awards, 0, post.ID, 1, nil, 0)
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
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
			return
		}
	}

	for _, s := range stmt2 {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
			return
		}
	}

	skip := 0
	limit := 10

	closedAttemptsInfo, err := GetClosedAttempts(context.Background(), testTiDB, post.ID, skip, limit)
	if err != nil {
		t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
		return
	}

	if closedAttemptsInfo == nil {
		t.Error("\nTestGetClosedAttempts failed\n    Error: ", err)
		return
	}

	closedAttempts := closedAttemptsInfo["attempts"].([]*query_models.AttemptUserBackgroundFrontend)

	if len(closedAttempts) != 1 {
		t.Errorf("\nTestGetClosedAttempts failed\n    Expected: 1 closed attempt, Got: %v closed attempts\n", len(closedAttempts))
		return
	}

	for _, attempt := range closedAttempts {
		if attempt.PostID != fmt.Sprintf("%v", post.ID) {
			t.Error("\nTestGetClosedAttempts failed\n    Error: Incorrect post ID")
			return
		}
		if attempt.PostTitle != post.Title {
			t.Error("\nTestGetClosedAttempts failed\n    Error: Incorrect post title")
			return
		}
		if !attempt.Closed {
			t.Error("\nTestGetClosedAttempts failed\n    Error: Incorrect attempt closed status")
			return
		}
	}

	t.Log("\nTestGetClosedAttempts succeeded")
}
