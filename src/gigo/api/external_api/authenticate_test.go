package external_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/gage-technologies/gigo-lib/db/models"
)

func TestHTTPServer_Login(t *testing.T) {
	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")
	}()

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_Login failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_Login failed\n    Error: ", err)
			return
		}
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/auth/login", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_Login failed\n    Error: %v", err)
		return
	}

	req.SetBasicAuth("test", "password")

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_Login failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_Login failed\n    Error: incorrect response code")
		return
	}

	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Error("\nTestHTTPServer_Login failed\n    Error: No cookies returned")
		return
	}

	cookie := cookies[0]
	if cookie.Name != "gigoAuthToken" {
		t.Errorf("\nTestHTTPServer_Login failed\n    Error: cookie name not as expected, got: %v, want: %v", cookie.Name, "gigoAuthToken")
		return
	}

	t.Log("\nTestHTTPServer_Login succeeded")
}

func TestHTTPServer_ValidateSession(t *testing.T) {
	// Create a new request
	req, err := http.NewRequest("GET", "http://localhost:1818/api/auth/validate", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ValidateSession failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Simulate a valid user in the context
	ctx := context.WithValue(req.Context(), "callingUser", &models.User{
		// Fill out a user model here with test data
	})

	req = req.WithContext(ctx)

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ValidateSession failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_ValidateSession failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_ValidateSession failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_ValidateSession failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{
		"message": "valid", // Update the expectedJson to reflect the actual expected data.
	}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_ValidateSession failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_ValidateSession succeeded")
}

func TestHTTPServer_LoginWithGoogle(t *testing.T) {
	// Create a new request with JSON body
	body := bytes.NewReader([]byte(`{"external_auth":"test@gmail.com", "password":"testpassword"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/auth/loginWithGoogle", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_LoginWithGoogle failed\n    Error: %v", err)
		return
	}

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_LoginWithGoogle failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_LoginWithGoogle failed\n    Error: incorrect response code")
		return
	}

	// Check for Set-Cookie header
	cookies := res.Cookies()
	var gigoAuthCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "gigoAuthToken" {
			gigoAuthCookie = cookie
			break
		}
	}

	if gigoAuthCookie == nil {
		t.Error("\nTestHTTPServer_LoginWithGoogle failed\n    Error: no gigoAuthToken cookie set")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_LoginWithGoogle failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_LoginWithGoogle failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{
		// Update the expectedJson to reflect the actual expected data.
	}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_LoginWithGoogle failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_LoginWithGoogle succeeded")
}

func TestHTTPServer_LoginWithGithub(t *testing.T) {
	// Create a new request with JSON body
	body := bytes.NewReader([]byte(`{"external_auth":"test_github_username"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/auth/loginWithGithub", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_LoginWithGithub failed\n    Error: %v", err)
		return
	}

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_LoginWithGithub failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_LoginWithGithub failed\n    Error: incorrect response code")
		return
	}

	// Check for Set-Cookie header
	cookies := res.Cookies()
	var gigoAuthCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "gigoAuthToken" {
			gigoAuthCookie = cookie
			break
		}
	}

	if gigoAuthCookie == nil {
		t.Error("\nTestHTTPServer_LoginWithGithub failed\n    Error: no gigoAuthToken cookie set")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_LoginWithGithub failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_LoginWithGithub failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{
		// Update the expectedJson to reflect the actual expected data.
	}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_LoginWithGithub failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_LoginWithGithub succeeded")
}
