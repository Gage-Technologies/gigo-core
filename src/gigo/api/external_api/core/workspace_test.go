package core

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	config2 "github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"testing"
	"time"
)

func TestCreateWorkspace(t *testing.T) {
	return
}

func TestGetWorkspaceStatus(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	// Create test data
	testUser := &models.User{
		ID:        1,
		FirstName: "Test",
		LastName:  "User",
		Email:     "testuser@example.com",
	}

	testWorkspace := &models.Workspace{
		ID:             1,
		CodeSourceID:   2,
		CodeSourceType: models.CodeSourcePost,
		RepoID:         3,
		CreatedAt:      time.Now(),
		OwnerID:        testUser.ID,
		Expiration:     time.Now().Add(time.Hour),
		Commit:         "abcdef",
		State:          1,
		InitState:      1,
		InitFailure:    nil,
		Ports:          []models.WorkspacePort{},
	}

	testPost := &models.Post{
		ID:          2,
		Title:       "Test Post",
		Description: "This is a test post",
		Author:      "Test Author",
		AuthorID:    4,
	}

	postStatement, err := testPost.ToSQLNative()
	if err != nil {
		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v\n", err)
		return
	}

	for _, stmt := range postStatement {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample post: %v", err)
		}
	}

	workspaceStatement, err := testWorkspace.ToSQLNative()
	if err != nil {
		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v\n", err)
		return
	}

	for _, stmt := range workspaceStatement {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample workspace: %v", err)
		}
	}

	// Call GetWorkspaceStatus with the test data
	result, err := GetWorkspaceStatus(context.Background(), testTiDB, vcsClient, testUser, testWorkspace.ID, "localhost", false)

	// Check for any unexpected errors
	assert.NoError(t, err, "GetWorkspaceStatus should not return an error")

	// Validate the result
	workspace := result["workspace"].(*models.WorkspaceFrontend)
	assert.Equal(t, fmt.Sprintf("%v", testWorkspace.ID), workspace.ID, "Workspace ID should match")
	assert.Equal(t, testPost.Title, result["code_source"].(map[string]interface{})["name"], "Code source name should match")
}

// TODO finish test
func TestInitializeWorkspace(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	user, err := models.CreateUser(69, "", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v\n", err)
		return
	}

	statements, err := user.ToSQLNative()

	for _, statement := range statements {
		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v\n", err)
			return
		}
	}

	id := int64(64)
	repoId := int64(420)
	codeSourceId := int64(69420)
	createdAt := time.Unix(1677485673, 0)
	ownerId := int64(1231)
	templateId := int64(42069)
	expiration := time.Unix(1677485673, 0)
	commit := hex.EncodeToString(sha1.New().Sum([]byte("test")))

	ws, err := models.CreateWorkspace(id, repoId, codeSourceId, 1, createdAt, ownerId, templateId, expiration, commit, &models.DefaultWorkspaceSettings, &models.OverAllocated{}, []models.WorkspacePort{})
	if err != nil {
		t.Errorf("\nfailed to create workspace\n   Error: %v\n", err)
	}

	stmt, err := ws.ToSQLNative()
	if err != nil {
		t.Error("\nExtendExpiration HTTP failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nExtendExpiration HTTP failed\n    Error: ", err)
			return
		}
	}

	// // insert sensitive info when testing
	// vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	// if err != nil {
	//	log.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	// }
	//
	// snowflakeNode, err := snowflake.NewNode(69420)
	// if err != nil {
	//	log.Fatal(fmt.Sprintf("failed to create snowflake node, %v", err))
	// }

	// res, err := InitializeWork(testTiDB, vcsClient, snowflakeNode, "gage.intranet", ws.ID)
	// if err != nil {
	//	t.Errorf("failed to initialize workspace, err: %v", err)
	// }

	// fmt.Println(res)

	return
}

func TestExtendExpiration(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	id := int64(64)
	repoId := int64(420)
	createdAt := time.Unix(1677485673, 0)
	ownerId := int64(1231)
	templateId := int64(42069)
	expiration := time.Unix(1677485673, 0)
	commit := hex.EncodeToString(sha1.New().Sum([]byte("test")))
	postId := int64(27494)

	ws, err := models.CreateWorkspace(id, repoId, postId, 1, createdAt, ownerId, templateId, expiration, commit, &models.DefaultWorkspaceSettings, &models.OverAllocated{}, []models.WorkspacePort{})
	if err != nil {
		t.Errorf("failed to create workspace model, err: %v", err)
		return
	}

	tx, err := testTiDB.DB.Begin()
	if err != nil {
		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
		return
	}

	wsStmt, err := ws.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetActiveProjects failed\n    Error: ", err)
		return
	}

	for _, statement := range wsStmt {
		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\n ws failed\n    Error: %v", err)
			return
		}
	}

	// res, err := ExtendExpiration(testTiDB, ws.ID, ws.Secret)
	// if err != nil {
	//	t.Errorf("\n ExtendExpiration failed\n    Error: %v", err)
	// }

	err = tx.Commit()
	if err != nil {
		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
		return
	}

	defer func() {
		testTiDB.DB.Exec("drop table workspaces")
	}()

	// fmt.Println(res)

	return

}

func TestWorkspaceAFK(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	id := int64(64)
	repoId := int64(420)
	codeSourceId := int64(69420)
	createdAt := time.Unix(1677485673, 0)
	ownerId := int64(1231)
	templateId := int64(42069)
	expiration := time.Unix(1677485673, 0)
	commit := hex.EncodeToString(sha1.New().Sum([]byte("test")))

	ws, err := models.CreateWorkspace(id, repoId, codeSourceId, 1, createdAt, ownerId, templateId, expiration, commit, &models.DefaultWorkspaceSettings, &models.OverAllocated{}, []models.WorkspacePort{})
	if err != nil {
		t.Errorf("failed to create workspace model, err: %v", err)
		return
	}

	tx, err := testTiDB.DB.Begin()
	if err != nil {
		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
		return
	}

	wsStmt, err := ws.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_GetActiveProjects failed\n    Error: ", err)
		return
	}

	for _, statement := range wsStmt {
		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\n ws failed\n    Error: %v", err)
			return
		}
	}

	// _, err = WorkspaceAFK(testTiDB, ws.ID, ws.Secret, 3)
	// if err != nil {
	//	t.Errorf("\n ExtendExpiration failed\n    Error: %v", err)
	// }

	err = tx.Commit()
	if err != nil {
		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
		return
	}

	defer func() {
		testTiDB.DB.Exec("drop table workspaces")
	}()

	return
}

func TestCodeServerPullThroughCache(t *testing.T) {
	storageEngine, err := storage.CreateMinioObjectStorage(config2.StorageS3Config{
		Bucket:    "gigo-dev",
		AccessKey: "gigo-dev",
		SecretKey: "gigo-dev",
		Endpoint:  "gigo-dev-minio:9000",
		UseSSL:    false,
	})
	if err != nil {
		t.Fatalf("failed to create storage engine: %v", err)
	}

	defer storageEngine.DeleteFile("ext/gigo-code-server-cache/0.1.0-amd64-linux-tar")

	buf, err := CodeServerPullThroughCache(context.TODO(), storageEngine, "0.1.0", "amd64", "linux", "tar")
	if err != nil {
		t.Fatalf("failed to pull through cache: %v", err)
	}

	// read the buffer and hash it
	hash := sha1.New()
	_, err = io.Copy(hash, buf)
	if err != nil {
		t.Fatalf("failed to hash buffer: %v", err)
	}
	_ = buf.Close()

	// time.Sleep(5 * time.Second)

	// retrieve the cached file
	buf, err = storageEngine.GetFile("ext/gigo-code-server-cache/0.1.0-amd64-linux-tar")
	if err != nil {
		t.Fatalf("failed to retrieve cached file: %v", err)
	}
	if buf == nil {
		t.Fatalf("cached file is nil")
	}

	// read the buffer and hash it
	hash2 := sha1.New()
	_, err = io.Copy(hash2, buf)
	if err != nil {
		t.Fatalf("failed to hash buffer: %v", err)
	}

	// compare the hashes
	assert.Equal(t, hex.EncodeToString(hash.Sum(nil)), hex.EncodeToString(hash2.Sum(nil)), "hashes should match")
}

func TestOpenVsxPullThroughCache(t *testing.T) {
	storageEngine, err := storage.CreateMinioObjectStorage(config2.StorageS3Config{
		Bucket:    "gigo-dev",
		AccessKey: "gigo-dev",
		SecretKey: "gigo-dev",
		Endpoint:  "gigo-dev-minio:9000",
		UseSSL:    false,
	})
	if err != nil {
		t.Fatalf("failed to create storage engine: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	defer rdb.Del(context.TODO(), "vsc:ext:version:ms-python:python:1.80.2")
	defer storageEngine.DeleteFile("ext/open-vsx-cache/ms-python/python.2023.14.0.vsix")

	buf, _, err := OpenVsxPullThroughCache(context.TODO(), storageEngine, rdb, "ms-python.python", "latest", "1.80.2")
	if err != nil {
		t.Fatalf("failed to pull through cache: %v", err)
	}

	// read the buffer and hash it
	hash := sha1.New()
	_, err = io.Copy(hash, buf)
	if err != nil {
		t.Fatalf("failed to hash buffer: %v", err)
	}

	_ = buf.Close()

	time.Sleep(5 * time.Second)

	// retrieve the cached file
	buf, err = storageEngine.GetFile("ext/open-vsx-cache/ms-python/python.2023.14.0.vsix")
	if err != nil {
		t.Fatalf("failed to retrieve cached file: %v", err)
	}

	if buf == nil {
		t.Fatalf("cached file is nil")
	}

	// read the buffer and hash it
	hash2 := sha1.New()
	_, err = io.Copy(hash2, buf)
	if err != nil {
		t.Fatalf("failed to hash buffer: %v", err)
	}

	// compare the hashes
	assert.Equal(t, hex.EncodeToString(hash.Sum(nil)), hex.EncodeToString(hash2.Sum(nil)), "hashes should match")
}
