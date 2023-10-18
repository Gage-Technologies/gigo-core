package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestHTTPServer_RecordImplicitAction(t *testing.T) {
	var ava models.AvatarSettings

	// Create a test user
	user, err := models.CreateUser(2, "testUser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("Failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testUser2")
	}()

	// Add user to DB
	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Errorf("\nTestHTTPServer_RecordImplicitAction failed\n    Error: %v", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Errorf("\nTestHTTPServer_RecordImplicitAction failed\n    Error: %v", err)
			return
		}
	}

	// Send a request to the RecordImplicitAction endpoint
	sessionId := uuid.New()                                                                                                      // generate a new UUID for the session
	body := bytes.NewReader([]byte(`{"post_id": "1", "action": 0.5, "session_id": "` + sessionId.String() + `", "test": true}`)) // The request body may need to be adjusted based on the actual data requirements.
	req, err := http.NewRequest("POST", "http://localhost:1818/api/recordImplicitAction", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_RecordImplicitAction failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_RecordImplicitAction failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_RecordImplicitAction failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_RecordImplicitAction failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_RecordImplicitAction failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_RecordImplicitAction failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_RecordImplicitAction succeeded")
}
