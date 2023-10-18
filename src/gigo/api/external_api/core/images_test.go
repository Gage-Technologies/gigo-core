package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/storage"
	"io/ioutil"
	"testing"
)

func TestSiteImages(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestSiteImages failed\n    Error: %v\n", err)
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

	// Create a storage engine for the test
	storageEngine := new(storage.FileSystemStorage)

	// Test the SiteImages function for user images
	id := testUser.ID
	post := false

	rc, err := SiteImages(context.Background(), testUser, testTiDB, id, "test", post, storageEngine)
	if err != nil {
		t.Errorf("SiteImages() error = %v", err)
		return
	}

	data, err := ioutil.ReadAll(rc)
	rc.Close()

	if err != nil {
		t.Fatalf("Failed to read from ReadCloser: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("SiteImages() returned empty data for user images")
	}

	// Add additional test cases for post images and other scenarios as needed.

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}

func TestGitImages(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGitImages failed\n    Error: %v\n", err)
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

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	// Test the GitImages function
	id := int64(1)
	post := false
	path := "test-image.png"

	imgBytes, err := GitImages(context.Background(), testUser, testTiDB, id, post, path, vcsClient)
	if err != nil {
		t.Errorf("GitImages() error = %v", err)
		return
	}

	if len(imgBytes) == 0 {
		t.Errorf("GitImages() returned empty data")
	}

	// Add additional test cases for post images and other scenarios as needed.

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}

func TestGetGeneratedImage(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetGeneratedImage failed\n    Error: %v\n", err)
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

	// Create a storage engine for the test
	storageEngine := new(storage.FileSystemStorage)

	// Test the GetGeneratedImage function
	imageId := int64(1)

	_, err = GetGeneratedImage(testUser, imageId, storageEngine)
	if err != nil {
		t.Errorf("GetGeneratedImage() error = %v", err)
		return
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}
