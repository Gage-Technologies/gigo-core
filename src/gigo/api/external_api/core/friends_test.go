package core

import (
	"context"
	"fmt"
	config2 "github.com/gage-technologies/gigo-lib/config"
	"reflect"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
)

func TestSendFriendRequest(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
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

	defer js.Close()

	var ava models.AvatarSettings

	callingUser, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestSendFriendRequest failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := callingUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert callingUser to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert callingUser: %v", err)
		}
	}

	friend, err := models.CreateUser(2, "testuser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestSendFriendRequest failed\n    Error: %v\n", err)
		return
	}

	friendStmt, err := friend.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert friend to SQL: %v", err)
	}

	for _, stmt := range friendStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert friend: %v", err)
		}
	}

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nSendFriendRequest failed\n    Error: %v\n", err)
		return
	}

	result, err := SendFriendRequest(context.Background(), testTiDB, sf, js, callingUser, friend.ID)
	if err != nil {
		t.Errorf("SendFriendRequest() error = %v", err)
		return
	}
	expectedResult := map[string]interface{}{"message": "friend request sent"}
	if !reflect.DeepEqual(result, expectedResult) {
		t.Errorf("SendFriendRequest() = %v, want %v", result, expectedResult)
	}

	// Check if the friend request was inserted into the database
	var friendRequest models.FriendRequests
	err = testTiDB.DB.QueryRow("SELECT user_id, friend FROM friend_requests WHERE user_id = ? AND friend = ?", callingUser.ID, friend.ID).Scan(&friendRequest.UserID, &friendRequest.Friend)
	if err != nil {
		t.Errorf("Failed to retrieve friend request from database: %v", err)
	}

	if friendRequest.UserID != callingUser.ID || friendRequest.Friend != friend.ID {
		t.Errorf("Friend request not inserted correctly: expected user_id = %d, friend = %d; got user_id = %d, friend = %d", callingUser.ID, friend.ID, friendRequest.UserID, friendRequest.Friend)
	}

	// Check if the notification was inserted into the database
	var notification models.Notification
	err = testTiDB.DB.QueryRow("SELECT user_id, message, notification_type, interacting_user_id FROM notification WHERE user_id = ? AND interacting_user_id = ?", friend.ID, callingUser.ID).Scan(&notification.UserID, &notification.Message, &notification.NotificationType, &notification.InteractingUserID)
	if err != nil {
		t.Errorf("Failed to retrieve notification from database: %v", err)
	}

	if notification.UserID != friend.ID || *notification.InteractingUserID != callingUser.ID || notification.Message != "New Friend Request" || notification.NotificationType != models.FriendRequest {
		t.Errorf("Notification not inserted correctly")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM friend_requests WHERE user_id = 1 AND friend = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM notification WHERE user_id = 2 AND interacting_user_id = 1")
	}()
}

func TestAcceptFriendRequest(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Setup logger and Jetstream client
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

	defer js.Close()

	// Create a snowflake node
	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nSendFriendRequest failed\n    Error: %v\n", err)
		return
	}

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
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	friendReq, err := models.CreateFriendRequests(69, testUser1.ID, testUser1.UserName, testUser2.ID, testUser2.UserName, time.Now(), 69)

	reqStmt := friendReq.ToSQLNative()

	_, err = testTiDB.DB.Exec(reqStmt.Statement, reqStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Call the AcceptFriendRequest function
	result, err := AcceptFriendRequest(context.Background(), testTiDB, sf, js, testUser2, testUser1.ID)
	if err != nil {
		t.Errorf("AcceptFriendRequest() error = %v", err)
		return
	}

	expectedResult := map[string]interface{}{"message": "friend request accepted"}
	if !reflect.DeepEqual(result, expectedResult) {
		t.Errorf("AcceptFriendRequest() = %v, want %v", result, expectedResult)
	}
	// Check if friend relationships were created in the friends table
	var count int
	err = testTiDB.DB.QueryRow("SELECT COUNT(*) FROM friends WHERE (user_id = ? AND friend = ?) OR (user_id = ? AND friend = ?)", testUser1.ID, testUser2.ID, testUser2.ID, testUser1.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query friends table: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected friend relationship count to be 2, got %v", count)
	}

	// Check if the friend request was marked as accepted in the friend_requests table
	var response bool
	err = testTiDB.DB.QueryRow("SELECT response FROM friend_requests WHERE user_id = ? AND friend = ?", testUser1.ID, testUser2.ID).Scan(&response)
	if err != nil {
		t.Fatalf("Failed to query friend_requests table: %v", err)
	}

	if !response {
		t.Errorf("Expected friend request to be marked as accepted, but it was not")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM friends WHERE (user_id = 1 AND friend = 2) OR (user_id = 2 AND friend = 1)")
		_, _ = testTiDB.DB.Exec("DELETE FROM friend_requests WHERE user_id = 1 AND friend = 2")
	}()
}

func TestDeclineFriendRequest(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
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

	defer js.Close()

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nDeclineFriendRequest failed\n    Error: %v\n", err)
		return
	}

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
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	friendReq, err := models.CreateFriendRequests(69, testUser1.ID, testUser1.UserName, testUser2.ID, testUser2.UserName, time.Now(), 69)

	reqStmt := friendReq.ToSQLNative()

	_, err = testTiDB.DB.Exec(reqStmt.Statement, reqStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	result, err := DeclineFriendRequest(context.Background(), testTiDB, sf, js, testUser2, testUser1.ID)
	if err != nil {
		t.Errorf("DeclineFriendRequest() error = %v", err)
		return
	}

	expectedResult := map[string]interface{}{"message": "friend request declined"}
	if !reflect.DeepEqual(result, expectedResult) {
		t.Errorf("DeclineFriendRequest() = %v, want %v", result, expectedResult)
	}

	// Check if the friend request was marked as declined in the friend_requests table
	var response bool
	err = testTiDB.DB.QueryRow("SELECT response FROM friend_requests WHERE user_id = ? AND friend = ?", testUser1.ID, testUser2.ID).Scan(&response)
	if err != nil {
		t.Fatalf("Failed to query friend_requests table: %v", err)
	}

	if response {
		t.Errorf("Expected friend request to be marked as declined, but it was not")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", testUser1.ID)
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", testUser2.ID)
		_, _ = testTiDB.DB.Exec("DELETE FROM friend_requests WHERE user_id = 1 AND friend = 2")
	}()
}

func TestGetFriendsList(t *testing.T) {
	t.Helper()

	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

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
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
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

	_, err = testTiDB.DB.Exec(friendStmt.Statement, friendStmt.Values...)

	// Call the function being tested
	response, err := GetFriendsList(context.Background(), testTiDB, testUser1)
	if err != nil {
		t.Fatalf("Failed to get friends list: %v", err)
	}

	// Check if the returned friends list is correct
	friends, ok := response["friends"].([]*models.FriendsFrontend)
	if !ok || len(friends) != 1 {
		t.Fatal("Expected to find one friend in the returned friends list, but the result was incorrect")
	}

	expectedFriend := &models.FriendsFrontend{
		ID:         "69",
		UserID:     fmt.Sprintf("%v", testUser1.ID),
		UserName:   "testuser1",
		Friend:     fmt.Sprintf("%v", testUser2.ID),
		FriendName: "testuser2",
		Date:       time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if !reflect.DeepEqual(friends[0].ID, expectedFriend.ID) {
		t.Fatalf("The returned friend does not match the expected friend\nExpected: %v\nGot: %v", expectedFriend, friends[0])
	}

	if !reflect.DeepEqual(friends[0].UserName, expectedFriend.UserName) {
		t.Fatalf("The returned friend does not match the expected friend\nExpected: %v\nGot: %v", expectedFriend, friends[0])
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", testUser1.ID)
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", testUser2.ID)
		_, _ = testTiDB.DB.Exec("DELETE FROM friend_requests WHERE user_id = 1 AND friend = 2")
	}()

	t.Log("TestGetFriendsList succeeded")
}

func TestCheckFriendRequest(t *testing.T) {
	t.Helper()

	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

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
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	// Insert a friend request
	_, err = testTiDB.DB.Exec("INSERT INTO friend_requests (user_id, friend) VALUES (?, ?)", testUser1.ID, testUser2.ID)
	if err != nil {
		t.Fatalf("Failed to insert friend request: %v", err)
	}

	// Call the function being tested
	response, err := CheckFriendRequest(context.Background(), testTiDB, testUser1, testUser2.ID)
	if err != nil {
		t.Fatalf("Failed to check friend request: %v", err)
	}

	// Check if the friend request exists
	if response["request"].(bool) != true {
		t.Fatalf("Expected to find a friend request, but none was found")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", 1)
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", 2)
		_, _ = testTiDB.DB.Exec("DELETE FROM friend_requests WHERE user_id = ? AND friend = ?", 1, 2)
	}()

	t.Log("TestCheckFriendRequest succeeded")
}
