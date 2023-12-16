package external_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	config2 "github.com/gage-technologies/gigo-lib/config"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
)

func TestHTTPServer_DeclareNemesis(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	// Send a request to the DeclareNemesis endpoint
	body := bytes.NewReader([]byte(`{"protag_id": "2", "end_time": "2024-06-01", "test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/declare", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeclareNemesis failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeclareNemesis failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_DeclareNemesis failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_DeclareNemesis failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_DeclareNemesis failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_DeclareNemesis failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_DeclareNemesis succeeded")
}

func TestHTTPServer_AcceptNemesis(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
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

	// Call the DeclareNemesis function to create a nemesis request
	_, _ = core.DeclareNemesis(context.Background(), testTiDB, sf, js, testUser1.ID, testUser2.ID)

	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
	}()

	// Send a request to the AcceptNemesis endpoint
	body := bytes.NewReader([]byte(`{"antagonist_id": "2", "test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/accept", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AcceptNemesis failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AcceptNemesis failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_AcceptNemesis failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_AcceptNemesis failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_AcceptNemesis failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_AcceptNemesis failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_AcceptNemesis succeeded")
}

func TestHTTPServer_DeclineNemesis(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
		return
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

	// Call the DeclareNemesis function to create a nemesis request
	_, err = core.DeclareNemesis(context.Background(), testTiDB, sf, js, testUser1.ID, testUser2.ID)
	if err != nil {
		t.Errorf("DeclareNemesis() error = %v", err)
		return
	}
	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
	}()

	// Send a request to the DeclineNemesis endpoint
	body := bytes.NewReader([]byte(`{"antagonist_id": "2", "test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/decline", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeclineNemesis failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeclineNemesis failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_DeclineNemesis failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_DeclineNemesis failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_DeclineNemesis failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_DeclineNemesis failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_DeclineNemesis succeeded")
}

func TestHTTPServer_GetActiveNemesis(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetActiveNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetActiveNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}
	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
		return
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

	_, err = core.DeclareNemesis(context.Background(), testTiDB, sf, js, testUser1.ID, testUser2.ID)
	if err != nil {
		t.Errorf("DeclareNemesis() error = %v", err)
		return
	}

	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
	}()

	// Send a request to the GetActiveNemesis endpoint
	req, err := http.NewRequest("GET", "http://localhost:1818/api/nemesis/active", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetActiveNemesis failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetActiveNemesis failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetActiveNemesis failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetActiveNemesis failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetActiveNemesis failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetActiveNemesis failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetActiveNemesis succeeded")
}

func TestHTTPServer_GetNemesisBattlegrounds(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	acceptedNemesisReq := models.Nemesis{
		ID:            1,
		AntagonistID:  testUser1.ID,
		ProtagonistID: testUser2.ID,
		IsAccepted:    true,
		EndTime:       nil,
	}

	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()

	for _, stmt := range nemesisReqStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM nemesis")
	}()

	// Build request body
	body := bytes.NewReader([]byte(`{"match_id":"1", "antagonist_id":"2", "protagonist_id":"3", "test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/battleground", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetNemesisBattlegrounds failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Execute request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetNemesisBattlegrounds failed\n    Error: %v", err)
		return
	}

	// Check the response status
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetNemesisBattlegrounds failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetNemesisBattlegrounds failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetNemesisBattlegrounds failed\n    Error: ", err)
		return
	}

	// Since the test attribute is set to true, the response should be an empty JSON object.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetNemesisBattlegrounds failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetNemesisBattlegrounds succeeded")
}

func TestHTTPServer_RecentNemesisBattleground(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	acceptedNemesisReq := models.Nemesis{
		ID:                        1,
		AntagonistID:              testUser1.ID,
		ProtagonistID:             testUser2.ID,
		TimeOfVillainy:            time.Now(),
		AntagonistTowersCaptured:  5,
		ProtagonistTowersCaptured: 3,
		IsAccepted:                true,
	}

	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert accepted nemesis request to SQL: %v", err)
	}

	for _, stmt := range nemesisReqStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	acceptedNemesisHistory := models.NemesisHistory{
		ID:            1,
		AntagonistID:  testUser1.ID,
		ProtagonistID: testUser2.ID,
	}

	nemesisHistoryStmt := acceptedNemesisHistory.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert accepted nemesis request to SQL: %v", err)
	}

	for _, stmt := range nemesisHistoryStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()

	// Send a request to the RecentNemesisBattleground endpoint
	body := bytes.NewReader([]byte(`{"test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/battleground/recent", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RecentNemesisBattleground failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RecentNemesisBattleground failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_RecentNemesisBattleground failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_RecentNemesisBattleground failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_RecentNemesisBattleground failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_RecentNemesisBattleground failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_RecentNemesisBattleground succeeded")
}

func TestHTTPServer_WarHistory(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	// Insert test data
	nemesisMatches := []models.Nemesis{
		{
			ID:                        1,
			AntagonistID:              testUser1.ID,
			ProtagonistID:             testUser2.ID,
			Victor:                    &testUser1.ID,
			AntagonistTowersCaptured:  5,
			ProtagonistTowersCaptured: 3,
		},
		{
			ID:                        2,
			AntagonistID:              testUser1.ID,
			ProtagonistID:             testUser2.ID,
			Victor:                    &testUser2.ID,
			AntagonistTowersCaptured:  2,
			ProtagonistTowersCaptured: 7,
		},
	}

	for _, match := range nemesisMatches {
		nemesisStmt := match.ToSQLNative()

		for _, stmt := range nemesisStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert nemesis match: %v", err)
			}
		}
	}

	nemesisHistoryData := []models.NemesisHistory{
		{
			MatchID:            1,
			AntagonistTotalXP:  120,
			ProtagonistTotalXP: 80,
		},
		{
			MatchID:            2,
			AntagonistTotalXP:  70,
			ProtagonistTotalXP: 130,
		},
	}

	for _, history := range nemesisHistoryData {
		historyStmt := history.ToSQLNative()

		for _, stmt := range historyStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert nemesis history: %v", err)
			}
		}
	}

	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM nemesis")
	}()

	// Send a request to the WarHistory endpoint
	body := bytes.NewReader([]byte(`{"test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/history", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_WarHistory failed\n    Error: %v", err)
		return
	}

	// Add user authentication token to the request
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_WarHistory failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_WarHistory failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_WarHistory failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_WarHistory failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_WarHistory failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_WarHistory succeeded")
}

func TestHTTPServer_PendingNemesis(t *testing.T) {
	// Create test user
	testUser1, err := models.CreateUser(1, "nemesis", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestPendingNemesis failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser1.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	defer func() {
		_, _ = testHttpServer.tiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()

	// Send a request to the PendingNemesis endpoint
	body := bytes.NewReader([]byte(`{"test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/nemesis/pending", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_PendingNemesis failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_PendingNemesis failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_PendingNemesis failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_PendingNemesis failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_PendingNemesis failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_PendingNemesis failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_PendingNemesis succeeded")
}

func TestHTTPServer_GetAllUsers(t *testing.T) {
	// Create test users
	testUser1, err := models.CreateUser(1, "test1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetAllUsers failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "test2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetAllUsers failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	// Send a request to the GetAllUsers endpoint
	req, err := http.NewRequest("GET", "http://localhost:1818/api/users", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetAllUsers failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetAllUsers failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetAllUsers failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetAllUsers failed\n    Error: ", err)
		return
	}

	var resJson []models.User
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetAllUsers failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := []*models.User{testUser1, testUser2} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetAllUsers failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetAllUsers succeeded")
}
