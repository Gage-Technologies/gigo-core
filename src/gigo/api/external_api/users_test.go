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

func TestHTTPServer_CreateNewUser(t *testing.T) {
	var ava *models.AvatarSettings

	newUser := models.User{
		UserName:       "testUserName",
		Email:          "testEmail@test.com",
		Phone:          "1234567890",
		Bio:            "This is a test bio",
		FirstName:      "Test",
		LastName:       "User",
		Timezone:       "America/Los_Angeles",
		Password:       "testPassword",
		AvatarSettings: ava,
	}

	body, err := json.Marshal(newUser)
	if err != nil {
		t.Fatalf("Failed to marshal new user: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/createNewUser", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewUser failed\n    Error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewUser failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CreateNewUser failed\n    Error: incorrect response code")
		return
	}

	// Make sure the user was actually added to the database
	check, err := testHttpServer.tiDB.DB.Query("select _id from users where user_name = ?", newUser.UserName)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewUser failed\n    Error: %v", err)
		return
	}

	if !check.Next() {
		t.Errorf("\nTestHTTPServer_CreateNewUser failed\n    Error: user was not added to the database")
		return
	}

	t.Log("\nTestHTTPServer_CreateNewUser succeeded")
}

func TestHTTPServer_ValidateUserInfo(t *testing.T) {

	newUser := models.User{
		UserName: "testUserName",
		Email:    "testEmail@test.com",
		Phone:    "1234567890",
		Timezone: "America/Los_Angeles",
		Password: "testPassword",
	}

	// User info to be validated
	userInfo := map[string]interface{}{
		"user_name":  newUser.UserName,
		"password":   newUser.Password,
		"email":      newUser.Email,
		"phone":      newUser.Phone,
		"timezone":   newUser.Timezone,
		"force_pass": true,
		"test":       true, // This will make the function return immediately after the test flag check
	}

	body, err := json.Marshal(userInfo)
	if err != nil {
		t.Fatalf("Failed to marshal user info: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/validateUser", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("\nTestHTTPServer_ValidateUserInfo failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_ValidateUserInfo failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ValidateUserInfo failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_ValidateUserInfo succeeded")
}

func TestHTTPServer_ForgotPasswordValidation(t *testing.T) {
	// Prepare request data
	data := map[string]interface{}{
		"email":     "testUser@gmail.com",
		"user_name": "testUser",
		"url":       "http://localhost/resetPassword",
		"test":      true, // This will make the function return immediately after the test flag check
	}

	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/forgotPasswordValidation", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("\nTestHTTPServer_ForgotPasswordValidation failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_ForgotPasswordValidation failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ForgotPasswordValidation failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_ForgotPasswordValidation succeeded")
}

func TestHTTPServer_ResetForgotPassword(t *testing.T) {
	// Prepare request data
	data := map[string]interface{}{
		"user_id":      "testUserID",
		"new_password": "newTestPassword",
		"force_pass":   true,
		"test":         true, // This will make the function return immediately after the test flag check
	}

	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/forgotPassword", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("\nTestHTTPServer_ResetForgotPassword failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_ResetForgotPassword failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ResetForgotPassword failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_ResetForgotPassword succeeded")
}

func TestHTTPServer_ChangeEmail(t *testing.T) {

	body := bytes.NewReader([]byte(`{"test":true, "new_email": "changed_email"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/changeEmail", body)
	if err != nil {
		t.Errorf("\nChangeEmail failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nChangeEmail failed\n    Error: %v", err)
		return
	}

	fmt.Println("res is: ", res)

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nChangeEmails failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nChangeEmail succeeded")
}

func TestHTTPServer_ChangePassword(t *testing.T) {

	body := bytes.NewReader([]byte(`{"test":true, "old_password": "testpass","new_password": "newpassword"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/changePassword", body)
	if err != nil {
		t.Errorf("\nChangePassword failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nChangePassword failed\n    Error: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nChangePassword failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nChangePassword succeeded")
}

func TestHTTPServer_ChangeUsername(t *testing.T) {

	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test123", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("drop table users")
	}()

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nChangeUsername failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nChangeUsername failed\n    Error: ", err)
			return
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "new_username":"test-succeeded"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/changeUsername", body)
	if err != nil {
		t.Errorf("\nChangeUsername failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nclient.Do failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nChangeUsername failed\n    Error: incorrect response code")
		return
	}

	check, err := testHttpServer.tiDB.DB.Query("select user_name from users where _id = ?", user.ID)
	if err != nil {
		t.Errorf("\nTestChangeUsername failed\n    Error: %v\n", err)
		return
	}

	nameCheck := ""

	for check.Next() {
		var name string
		err = check.Scan(&name)
		if err != nil {
			t.Errorf("\nTestChangeUsername failed\n    Error: %v", err)
		}
		nameCheck = name
	}

	// ensure the closure of the rows
	defer check.Close()

	fmt.Println(nameCheck)

	if nameCheck != "test-succeeded" {
		t.Errorf("\nTestChangeUsername failed\n      Error: Name was incorrect\n")
	}

	fmt.Println(req)

	t.Log("\nChangeUsername succeeded")
}

func TestHTTPServer_ChangePhoneNumber(t *testing.T) {

	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test123", "testpass", "testemail",
		"oldphone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("drop table users")
	}()

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nChangePhoneNumber failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nChangePhoneNumber failed\n    Error: ", err)
			return
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "new_phone":"newphone"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/changePhoneNumber", body)
	if err != nil {
		t.Errorf("\nChangePhoneNumber failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nclient.Do failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nChangePhoneNumber failed\n    Error: incorrect response code")
		return
	}

	check, err := testHttpServer.tiDB.DB.Query("select phone from users where _id = ?", user.ID)
	if err != nil {
		t.Errorf("\nTestChangePhoneNumber failed\n    Error: %v\n", err)
		return
	}

	phoneCheck := ""

	for check.Next() {
		var phone string
		err = check.Scan(&phone)
		if err != nil {
			t.Errorf("\nTestChangePhoneNumber failed\n    Error: %v", err)
		}
		phoneCheck = phone
	}

	// ensure the closure of the rows
	defer check.Close()

	fmt.Println(phoneCheck)

	if phoneCheck != "newphone" {
		t.Errorf("\nTestChangePhoneNumber failed\n      Error: Phone number was incorrect\n")
	}

	fmt.Println(req)

	t.Log("\nChangePhoneNumber succeeded")
}

func TestHTTPServer_ChangeUserPicture(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer testHttpServer.tiDB.DB.Exec("delete from users where _id = ?;", user.ID)

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nChangeUserPicture failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nChangeUserPicture failed\n    Error: ", err)
			return
		}
	}

	body := bytes.NewReader([]byte(`{"test":true, "new_image_path": "changed_image_path"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/changeUserPicture", body)
	if err != nil {
		t.Errorf("\nChangeUserPicture failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nChangeUserPicture failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nChangeUserPicture failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nChangeUserPicture succeeded")
}

func TestHTTPServer_DeleteUserAccount(t *testing.T) {

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/deleteUserAccount", body)
	if err != nil {
		t.Errorf("\nDeleteUserAccount failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nDeleteUserAccount failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nDeleteUserAccount failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nDeleteUserAccount succeeded")
}

func TestHTTPServer_UserProjects(t *testing.T) {

	body := bytes.NewReader([]byte(`{"test":true, "skip": 0, limit": 1}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/userProjects", body)
	if err != nil {
		t.Errorf("\nUserProjects failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nChangeUsername failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nUserProjects failed\n    Error: incorrect response code")
		return
	}
	fmt.Println(req)

	t.Log("\nUserProjects succeeded")
}

func TestHTTPServer_UserProfilePage(t *testing.T) {

	// Insert sample user data
	sampleUser := &models.User{
		ID:       1,
		UserName: "test",
		Email:    "testUser@gmail.com",
		Password: "testPassword",
	}
	userStmt, err := sampleUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample user: %v", err)
		}
	}

	body := bytes.NewReader([]byte(`{"author_id":"1", "test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/profilePage", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UserProfilePage failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UserProfilePage failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_UserProfilePage failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_UserProfilePage failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_UserProfilePage failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_UserProfilePage failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_UserProfilePage succeeded")
}

func TestHTTPServer_CreateNewGoogleUser(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"external_auth":   "testExternalAuth",
		"password":        "testPassword",
		"timezone":        "America/Chicago",
		"start_user_info": map[string]interface{}{}, // fill as necessary
		"avatar_settings": map[string]interface{}{}, // fill as necessary
		"test":            true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/createNewGoogleUser", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewGoogleUser failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewGoogleUser failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_CreateNewGoogleUser failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateNewGoogleUser failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateNewGoogleUser failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CreateNewGoogleUser failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CreateNewGoogleUser succeeded")
}

func TestHTTPServer_CreateNewGithubUser(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"external_auth":   "testExternalAuth",
		"password":        "testPassword",
		"timezone":        "America/Chicago",
		"start_user_info": map[string]interface{}{}, // fill as necessary
		"avatar_settings": map[string]interface{}{}, // fill as necessary
		"test":            true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/createNewGithubUser", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewGithubUser failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateNewGithubUser failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_CreateNewGithubUser failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateNewGithubUser failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateNewGithubUser failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CreateNewGithubUser failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CreateNewGithubUser succeeded")
}

func TestHTTPServer_GetSubscription(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("GET", "http://localhost:1818/api/user/subscription", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetSubscription failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetSubscription failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetSubscription failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetSubscription failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetSubscription failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetSubscription failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetSubscription succeeded")
}

func TestHTTPServer_GetUserInformation(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("GET", "http://localhost:1818/api/user/get", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetUserInformation failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetUserInformation failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_GetUserInformation failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetUserInformation failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetUserInformation failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetUserInformation failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetUserInformation succeeded")
}

func TestHTTPServer_SetUserWorkspaceSettings(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"workspace_settings": map[string]interface{}{
			"AutoGit": map[string]interface{}{
				"CommitMessage": "Test Commit",
			},
		},
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/updateWorkspace", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SetUserWorkspaceSettings failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SetUserWorkspaceSettings failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_SetUserWorkspaceSettings failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_SetUserWorkspaceSettings failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_SetUserWorkspaceSettings failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_SetUserWorkspaceSettings failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_SetUserWorkspaceSettings succeeded")
}

func TestHTTPServer_UpdateAvatarSettings(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"avatar_settings": map[string]interface{}{
			// put actual avatar_settings fields here
		},
		"upload_id": "test-upload-id",
		"test":      true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/updateAvatar", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateAvatarSettings failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateAvatarSettings failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_UpdateAvatarSettings failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateAvatarSettings failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateAvatarSettings failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_UpdateAvatarSettings failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_UpdateAvatarSettings succeeded")
}

func TestHTTPServer_FollowUser(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"id":   "1", // User id to follow
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/follow", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_FollowUser failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_FollowUser failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_FollowUser failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_FollowUser failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_FollowUser failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_FollowUser failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_FollowUser succeeded")
}

func TestHTTPServer_UnFollowUser(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"id":   "1", // User id to unfollow
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/unfollow", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UnFollowUser failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UnFollowUser failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_UnFollowUser failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_UnFollowUser failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_UnFollowUser failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_UnFollowUser failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_UnFollowUser succeeded")
}

func TestHTTPServer_UpdateUserExclusiveAgreement(t *testing.T) {
	// Prepare request data
	reqData := map[string]interface{}{
		"test": true,
	}

	reqBytes, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("Failed to marshal request data: %v", err)
	}

	// Create a new request
	body := bytes.NewReader(reqBytes)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/updateExclusiveAgreement", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateUserExclusiveAgreement failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateUserExclusiveAgreement failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nTestHTTPServer_UpdateUserExclusiveAgreement failed\n    Error: incorrect response code")
		return
	}

	// Check the response body
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateUserExclusiveAgreement failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateUserExclusiveAgreement failed\n    Error: ", err)
		return
	}

	// Update the expectedJson to reflect the actual expected data.
	expectedJson := map[string]interface{}{}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_UpdateUserExclusiveAgreement failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_UpdateUserExclusiveAgreement succeeded")
}
