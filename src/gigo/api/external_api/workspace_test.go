package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestHTTPServer_CreateWorkspace(t *testing.T) {

	body := bytes.NewReader([]byte(`{"test":true, "name": "test-name", "latest_build": "420", "workspace_path": "test-path", "template_id": "69", "template_name": "test-name", "template_icon": "test-icon"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/workspace/create", body)
	if err != nil {
		t.Errorf("\nCreateNewTemplate failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nCreateNewTemplate failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nCreateNewTemplate failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nCreateNewTemplate HTTP succeeded")
}

func TestHTTPServer_GetWorkspaceStatus(t *testing.T) {
	// Prepare workspace ID
	workspaceID := map[string]interface{}{
		"id":   "1",
		"test": true, // This will make the function return immediately after the test flag check
	}

	testWorkspace := &models.Workspace{
		ID:             1,
		CodeSourceID:   2,
		CodeSourceType: models.CodeSourcePost,
		RepoID:         3,
		CreatedAt:      time.Now(),
		OwnerID:        callingUser.ID,
		Expiration:     time.Now().Add(time.Hour),
		Commit:         "abcdef",
		State:          1,
		InitState:      1,
		InitFailure:    nil,
		Ports:          []models.WorkspacePort{},
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

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from workspace")
	}()

	body, err := json.Marshal(workspaceID)
	if err != nil {
		t.Fatalf("Failed to marshal workspace info: %v", err)
	}

	req, err := http.NewRequest("GET", "http://localhost:1818/api/workspace/status", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetWorkspaceStatus failed\n    Error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetWorkspaceStatus failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetWorkspaceStatus failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_GetWorkspaceStatus succeeded")
}

//func TestHTTPServer_InitializeWorkspace(t *testing.T) {
//	defer func() {
//		testTiDB.DB.Exec("drop table workspaces")
//	}()
//
//	user, err := models.CreateUser(69, "test", "test", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Errorf("\nInitializeWorkspace HTTP failed\n    Error: %v\n", err)
//		return
//	}
//
//	statements, err := user.ToSQLNative()
//
//	for _, statement := range statements {
//		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nInitializeWorkspace HTTPn failed\n    Error: %v\n", err)
//			return
//		}
//	}
//
//	id := int64(64)
//	repoId := int64(420)
//	createdAt := time.Unix(1677485673, 0)
//	ownerId := int64(1231)
//	coderId := uuid.New()
//	templateId := int64(42069)
//	expiration := time.Unix(1677485673, 0)
//	commit := hex.EncodeToString(sha1.New().Sum([]byte("test")))
//	postId := int64(47284)
//
//	ws, err := models.CreateWorkspace(id, repoId, postId, models.CodeSourcePost, createdAt, ownerId, coderId, templateId, expiration, commit, &models.DefaultWorkspaceSettings)
//	if err != nil {
//		t.Errorf("\nfailed to create workspace\n   Error: %v\n", err)
//	}
//
//	stmt, err := ws.ToSQLNative()
//	if err != nil {
//		t.Error("\nInitializeWorkspace HTTP failed\n    Error: ", err)
//		return
//	}
//
//	for _, s := range stmt {
//		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
//		if err != nil {
//			t.Error("\nInitializeWorkspace HTTP failed\n    Error: ", err)
//			return
//		}
//	}
//
//	body := bytes.NewReader([]byte(`{"test":true, "coder_id":""}`))
//	req, err := http.NewRequest("POST", "http://localhost:1818/api/workspace/initializeWorkspace", body)
//	if err != nil {
//		t.Errorf("\nInitializeWorkspace HTTP failed\n    Error: %v", err)
//		return
//	}
//
//	req.AddCookie(&http.Cookie{
//		Name:  "gigoAuthToken",
//		Value: testUserAuth,
//	})
//
//	res, err := client.Do(req)
//	if err != nil {
//		t.Errorf("\nInitializeWorkspace HTTP failed\n    Error: %v", err)
//		return
//	}
//
//	fmt.Println("res is: ", res)
//
//	if res.StatusCode != http.StatusOK {
//		fmt.Println(res.StatusCode)
//		body, _ := ioutil.ReadAll(res.Body)
//		fmt.Println(string(body))
//		t.Error("\nInitializeWorkspace HTTP failed\n    Error: incorrect response code")
//		return
//	}
//
//	fmt.Println(req)
//
//	t.Log("\nInitializeWorkspace HTTP succeeded")
//
//	return
//}
//
//func TestHTTPServer_ExtendExpiration(t *testing.T) {
//
//	user, err := models.CreateUser(69, "test", "test", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v\n", err)
//		return
//	}
//
//	statements, err := user.ToSQLNative()
//
//	for _, statement := range statements {
//		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v\n", err)
//			return
//		}
//	}
//
//	id := int64(64)
//	repoId := int64(420)
//	createdAt := time.Unix(1677485673, 0)
//	ownerId := int64(1231)
//	coderId := uuid.New()
//	templateId := int64(42069)
//	expiration := time.Unix(1677485673, 0)
//	commit := hex.EncodeToString(sha1.New().Sum([]byte("test")))
//	postId := int64(757284)
//
//	ws, err := models.CreateWorkspace(id, repoId, postId, models.CodeSourcePost, createdAt, ownerId, coderId, templateId, expiration, commit, nil)
//	if err != nil {
//		t.Errorf("\nfailed to create workspace\n   Error: %v\n", err)
//	}
//
//	stmt, err := ws.ToSQLNative()
//	if err != nil {
//		t.Error("\nExtendExpiration HTTP failed\n    Error: ", err)
//		return
//	}
//
//	for _, s := range stmt {
//		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
//		if err != nil {
//			t.Error("\nExtendExpiration HTTP failed\n    Error: ", err)
//			return
//		}
//	}
//
//	body := bytes.NewReader([]byte(`{"test":true, "coder_id":"test", "secret":"74657374"}`))
//	req, err := http.NewRequest("POST", "http://localhost:1818/api/workspace/extendExpiration", body)
//	if err != nil {
//		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v", err)
//		return
//	}
//
//	req.AddCookie(&http.Cookie{
//		Name:  "gigoAuthToken",
//		Value: testUserAuth,
//	})
//
//	res, err := client.Do(req)
//	if err != nil {
//		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v", err)
//		return
//	}
//
//	fmt.Println("res is: ", res)
//
//	if res.StatusCode != http.StatusOK {
//		fmt.Println(res.StatusCode)
//		body, _ := ioutil.ReadAll(res.Body)
//		fmt.Println(string(body))
//		t.Error("\nExtendExpiration HTTP failed\n    Error: incorrect response code")
//		return
//	}
//	fmt.Println(req)
//
//	t.Log("\nExtendExpiration HTTP succeeded")
//
//	return
//}
//
//func TestHTTPServer_WorkspaceAFK(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	id := int64(64)
//	repoId := int64(420)
//	createdAt := time.Unix(1677485673, 0)
//	ownerId := int64(1231)
//	coderId := uuid.New()
//	templateId := int64(42069)
//	expiration := time.Unix(1677485673, 0)
//	commit := hex.EncodeToString(sha1.New().Sum([]byte("test")))
//	attemptId := int64(858284)
//
//	ws, err := models.CreateWorkspace(id, repoId, attemptId, models.CodeSourceAttempt, createdAt, ownerId, coderId, templateId, expiration, commit, &models.DefaultWorkspaceSettings)
//	if err != nil {
//		t.Errorf("\nfailed to create workspace\n   Error: %v\n", err)
//	}
//
//	tx, err := testTiDB.DB.Begin()
//	if err != nil {
//		t.Errorf("\nWorkspaceAFK failed\n    Error: %v\n", err)
//		return
//	}
//
//	wsStmt, err := ws.ToSQLNative()
//	if err != nil {
//		t.Error("\nTestHTTPServer_WorkspaceAFK failed\n    Error: ", err)
//		return
//	}
//
//	for _, statement := range wsStmt {
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\n ws failed\n    Error: %v", err)
//			return
//		}
//	}
//
//	body := bytes.NewReader([]byte(`{"test":true, "coder_id":"test", "secret":"74657374", "add_min":"60"}`))
//	req, err := http.NewRequest("POST", "http://localhost:1818/api/workspace/workspaceAfk", body)
//	if err != nil {
//		t.Errorf("\nExtendExpiration HTTP failed\n    Error: %v", err)
//		return
//	}
//
//	req.AddCookie(&http.Cookie{
//		Name:  "gigoAuthToken",
//		Value: testUserAuth,
//	})
//
//	res, err := client.Do(req)
//	if err != nil {
//		t.Errorf("\nTestHTTPServer_GetActiveProjects failed\n    Error: %v", err)
//		return
//	}
//
//	if res.StatusCode != http.StatusOK {
//		fmt.Println(res.StatusCode)
//		body, _ := ioutil.ReadAll(res.Body)
//		fmt.Println(string(body))
//		t.Error("\nTestHTTPServer_WorkspaceAFK failed\n    Error: incorrect response code")
//		return
//	}
//	fmt.Println(req)
//
//	t.Log("\nTestHTTPServer_WorkspaceAFK HTTP succeeded")
//
//	err = tx.Commit()
//	if err != nil {
//		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
//		return
//	}
//
//	return
//}
