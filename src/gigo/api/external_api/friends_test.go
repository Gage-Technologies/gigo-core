package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
)

func TestHTTPServer_SendFriendRequest(t *testing.T) {
	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	friendUser, err := models.CreateUser(2, "testuser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create friend user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser1")
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser2")
	}()

	// Add users to DB
	for _, user := range []*models.User{callingUser, friendUser} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Error("\nTestHTTPServer_SendFriendRequest failed\n    Error: ", err)
			return
		}

		for _, s := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
			if err != nil {
				t.Error("\nTestHTTPServer_SendFriendRequest failed\n    Error: ", err)
				return
			}
		}
	}

	body := bytes.NewReader([]byte(`{"friend_id":"2","test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/friends/request", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SendFriendRequest failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SendFriendRequest failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SendFriendRequest failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_SendFriendRequest failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_SendFriendRequest failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_SendFriendRequest failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_SendFriendRequest succeeded")
}

func TestHTTPServer_AcceptFriendRequest(t *testing.T) {
	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create calling user, err: %v", err)
		return
	}

	requesterUser, err := models.CreateUser(2, "testuser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create requester user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser1")
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser2")
	}()

	// Add users to DB
	for _, user := range []*models.User{callingUser, requesterUser} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Error("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: ", err)
			return
		}

		for _, s := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
			if err != nil {
				t.Error("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: ", err)
				return
			}
		}
	}

	friendReq, err := models.CreateFriendRequests(69, callingUser.ID, callingUser.UserName, requesterUser.ID, requesterUser.UserName, time.Now(), 420)

	reqStmt := friendReq.ToSQLNative()

	_, err = testHttpServer.tiDB.DB.Exec(reqStmt.Statement, reqStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	body := bytes.NewReader([]byte(`{"requester_id":"2", "test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/friends/accept", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_AcceptFriendRequest failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_AcceptFriendRequest succeeded")
}

func TestHTTPServer_DeclineFriendRequest(t *testing.T) {
	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create calling user, err: %v", err)
		return
	}

	requesterUser, err := models.CreateUser(2, "testuser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("failed to create requester user, err: %v", err)
		return
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser1")
		testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testuser2")
	}()

	for _, user := range []*models.User{callingUser, requesterUser} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Error("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: ", err)
			return
		}

		for _, s := range userStmt {
			_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
			if err != nil {
				t.Error("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: ", err)
				return
			}
		}
	}

	friendReq, err := models.CreateFriendRequests(69, callingUser.ID, callingUser.UserName, requesterUser.ID, requesterUser.UserName, time.Now(), 420)

	reqStmt := friendReq.ToSQLNative()

	_, err = testHttpServer.tiDB.DB.Exec(reqStmt.Statement, reqStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	body := bytes.NewReader([]byte(`{"requester_id":"2", "test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/friends/decline", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_DeclineFriendRequest failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_DeclineFriendRequest succeeded")
}

func TestHTTPServer_GetFriendsList(t *testing.T) {
	// Insert test users
	testUser1 := &models.User{
		ID:       1,
		UserName: "testuser1",
	}
	testUser2 := &models.User{
		ID:       2,
		UserName: "testuser2",
	}

	users := []*models.User{testUser1, testUser2}
	for _, user := range users {
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

	friend, err := models.CreateFriends(69, testUser1.ID, testUser1.UserName, testUser2.ID, testUser2.UserName, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	friendStmt := friend.ToSQLNative()

	_, err = testHttpServer.tiDB.DB.Exec(friendStmt.Statement, friendStmt.Values...)

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/friends/list", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetFriendsList failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetFriendsList failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetFriendsList failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetFriendsList failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetFriendsList failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetFriendsList failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetFriendsList succeeded")
}
