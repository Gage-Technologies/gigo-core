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

func TestHTTPServer_AcknowledgeNotification(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestAcknowledgeNotification failed\n    Error: %v\n", err)
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

	// Insert a test notification into the notifications table
	notificationID := int64(1)
	userID := int64(1)
	message := "Test notification message"
	notificationType := models.NotificationType(1)
	interactingUserID := int64(2)

	testNotification := models.Notification{
		ID:                notificationID,
		UserID:            userID,
		Message:           message,
		NotificationType:  notificationType,
		CreatedAt:         time.Now(),
		Acknowledged:      false,
		InteractingUserID: &interactingUserID,
	}

	notificationStmt := testNotification.ToSQLNative()
	_, err = testHttpServer.tiDB.DB.Exec(notificationStmt.Statement, notificationStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test notification: %v", err)
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("DELETE FROM notification")
	}()

	// Assume we have a notification with id 123
	notificationId := 123

	// Send a request to the AcknowledgeNotification endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"notification_id": "%v", "test": true}`, notificationId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/notification/acknowledge", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AcknowledgeNotification failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_AcknowledgeNotification failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_AcknowledgeNotification failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_AcknowledgeNotification failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_AcknowledgeNotification failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_AcknowledgeNotification failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_AcknowledgeNotification succeeded")
}

func TestHTTPServer_AcknowledgeUserNotificationGroup(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestAcknowledgeUserNotificationGroup failed\n    Error: %v\n", err)
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

	// Assume we have a notification type
	notificationType := 123

	// Send a request to the AcknowledgeUserNotificationGroup endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"notification_type": "%v", "test": true}`, notificationType)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/notification/acknowledgeGroup", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_AcknowledgeUserNotificationGroup failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_AcknowledgeUserNotificationGroup failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_AcknowledgeUserNotificationGroup failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_AcknowledgeUserNotificationGroup failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_AcknowledgeUserNotificationGroup failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_AcknowledgeUserNotificationGroup failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_AcknowledgeUserNotificationGroup succeeded")
}

func TestHTTPServer_ClearUserNotifications(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestClearUserNotifications failed\n    Error: %v\n", err)
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

	// Send a request to the ClearUserNotifications endpoint
	body := bytes.NewReader([]byte(`{"test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/notification/clear", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_ClearUserNotifications failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_ClearUserNotifications failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_ClearUserNotifications failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_ClearUserNotifications failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_ClearUserNotifications failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_ClearUserNotifications failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_ClearUserNotifications succeeded")
}

func TestHTTPServer_GetUserNotifications(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestGetUserNotifications failed\n    Error: %v\n", err)
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

	// Insert a test notification into the notifications table
	notificationID := int64(1)
	userID := int64(1)
	message := "Test notification message"
	notificationType := models.NotificationType(1)
	interactingUserID := int64(2)

	testNotification := models.Notification{
		ID:                notificationID,
		UserID:            userID,
		Message:           message,
		NotificationType:  notificationType,
		CreatedAt:         time.Now(),
		Acknowledged:      false,
		InteractingUserID: &interactingUserID,
	}

	notificationStmt := testNotification.ToSQLNative()
	_, err = testHttpServer.tiDB.DB.Exec(notificationStmt.Statement, notificationStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test notification: %v", err)
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("DELETE FROM notification")
	}()

	// Send a request to the GetUserNotifications endpoint
	req, err := http.NewRequest("GET", "http://localhost:1818/api/notification/get", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetUserNotifications failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetUserNotifications failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetUserNotifications failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetUserNotifications failed\n    Error: ", err)
		return
	}

	var resJson []models.Notification
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetUserNotifications failed\n    Error: ", err)
		return
	}

	// Compare the response to the expected result
	expectedJson := []models.Notification{testNotification}
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetUserNotifications failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetUserNotifications succeeded")
}
