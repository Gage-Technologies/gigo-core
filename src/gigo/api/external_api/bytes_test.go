package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestHTTPServer_StartByteAttempt(t *testing.T) {
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nStartByteAttempt failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
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
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	testByteAttempt := models.ByteAttempts{
		ID:       1,
		ByteID:   2,
		AuthorID: 3,
		Content:  "Test Byte Attempt",
	}

	attemptStmt, err := testByteAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert byte attempt to SQL native: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test byte attempt: %v", err)
		}

		defer func() {
			testHttpServer.tiDB.DB.Exec("delete from byte_attempts")
		}()
	}
	// Assume we have a project with id 1
	byteId := 1

	// Send a request to the DeleteProject endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"byte_id": "%v", "test": true}`, byteId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/bytes/startByteAttempt", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_StartByteAttempt failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_StartByteAttempt failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_StartByteAttempt failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_StartByteAttempt failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_StartByteAttempt failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_StartByteAttempt failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_StartByteAttempt succeeded")
}

func TestHTTPServer_GetByteAttempt(t *testing.T) {
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nStartByteAttempt failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
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
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	testByteAttempt := models.ByteAttempts{
		ID:       1,
		ByteID:   2,
		AuthorID: 3,
		Content:  "Test Byte Attempt",
	}

	attemptStmt, err := testByteAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert byte attempt to SQL native: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test byte attempt: %v", err)
		}

		defer func() {
			testHttpServer.tiDB.DB.Exec("delete from byte_attempts")
		}()
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/bytes/getByteAttempt", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetByteAttempt failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetByteAttempt failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetByteAttempt failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetByteAttempt failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetByteAttempt failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetByteAttempt failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetByteAttempt succeeded")
}

func TestHTTPServer_GetRecommendedBytes(t *testing.T) {
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nStartByteAttempt failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
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
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	req, err := http.NewRequest("POST", "http://localhost:1818/api/bytes/getRecommendedBytes", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetRecommendedBytes failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetRecommendedBytes failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetRecommendedBytes failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetRecommendedBytes failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetRecommendedBytes failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetRecommendedBytes failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetRecommendedBytes succeeded")
}

func TestHTTPServer_GetByte(t *testing.T) {
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nGetByte failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
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
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	testByteAttempt := models.ByteAttempts{
		ID:       1,
		ByteID:   2,
		AuthorID: 3,
		Content:  "Test Byte Attempt",
	}

	attemptStmt, err := testByteAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert byte attempt to SQL native: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test byte attempt: %v", err)
		}

		defer func() {
			testHttpServer.tiDB.DB.Exec("delete from byte_attempts")
		}()
	}
	// Assume we have a project with id 1
	byteId := 1

	// Send a request to the DeleteProject endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"byte_id": "%v", "test": true}`, byteId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/bytes/getByte", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetByte failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetByte failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetByte failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetByte failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetByte failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetByte failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetByte succeeded")
}
