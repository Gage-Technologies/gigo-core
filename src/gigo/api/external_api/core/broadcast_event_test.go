package core

import (
	"context"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func TestBroadcastMessage(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestBroadcastMessage failed\n    Error: %v\n", err)
		return
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestBroadcastMessage failed\n    Error: %v\n", err)
		return
	}

	// Call the function being tested
	response, err := BroadcastMessage(context.Background(), testTiDB, testSnowflake, user, "Test message")
	if err != nil {
		t.Errorf("\nTestBroadcastMessage failed\n    Error: %v\n", err)
		return
	}

	// Assert the expected response
	expectedKey := "broadcast_message"
	if _, ok := response[expectedKey]; !ok {
		t.Errorf("\nTestBroadcastMessage failed\n    Expected key '%s' in response, but not found\n", expectedKey)
		return
	}

	id, err := strconv.ParseInt(response[expectedKey].(*models.BroadcastEventFrontend).ID, 10, 64)
	if err != nil {
		t.Errorf("\nTestBroadcastMessage failed\n    Error: %v\n", err)
		return
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = 69")
		_, _ = testTiDB.DB.Exec("DELETE FROM broadcast_event WHERE _id = ?", id)
	}()

	t.Log("\nTestBroadcastMessage succeeded")
}

func TestGetBroadcastMessages(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	_, _ = testTiDB.DB.Exec("DELETE FROM broadcast_event")

	// Insert some test data into the broadcast_event table
	events := []models.BroadcastEvent{
		{ID: 1, UserID: 1, UserName: "user1", Message: "Test message 1", BroadcastType: 0, TimePosted: time.Now()},
		{ID: 2, UserID: 2, UserName: "user2", Message: "Test message 2", BroadcastType: 0, TimePosted: time.Now()},
		{ID: 3, UserID: 3, UserName: "user3", Message: "Test message 3", BroadcastType: 0, TimePosted: time.Now()},
	}

	for _, event := range events {
		event, err := models.CreateBroadcastEvent(event.ID, event.UserID, event.UserName, event.Message, event.BroadcastType, event.TimePosted)
		if err != nil {
			t.Errorf("\nTestGetBroadcastMessages failed\n    Error: %v\n", err)
			return
		}

		statement := event.ToSQLNative()

		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\nTestGetBroadcastMessages failed\n    Error: %v", err)
			return
		}

	}

	// Call the function being tested
	response, err := GetBroadcastMessages(context.Background(), testTiDB)
	if err != nil {
		t.Fatalf("Failed to get broadcast messages: %v", err)
	}

	// Assert the expected response
	expectedKey := "broadcast_messages"
	val, ok := response[expectedKey]
	if !ok {
		t.Fatalf("Expected key '%s' in response, but not found", expectedKey)
	}

	// Assert the type of the value is a slice of BroadcastEventFrontend
	eventsFrontend, ok := val.([]models.BroadcastEventFrontend)
	if !ok {
		t.Fatalf("Expected value of type []models.BroadcastEventFrontend, but got %T", val)
	}

	// Assert the length of the events slice is equal to the number of test data inserted
	expectedLength := len(events)
	if expectedLength != len(eventsFrontend) {
		t.Fatalf("Expected %d broadcast messages, but got %d", expectedLength, len(eventsFrontend))
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM broadcast_event")
	}()

	t.Log("TestGetBroadcastMessages succeeded")
}

func TestCheckBroadcastAward(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Insert a test user into the users table
	userID := int64(1)
	userName := "Test User"
	hasBroadcast := true

	testUser := models.User{
		ID:           userID,
		UserName:     userName,
		HasBroadcast: hasBroadcast,
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to create user statement: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Call the function being tested
	response, err := CheckBroadcastAward(context.Background(), testTiDB, &testUser)
	if err != nil {
		t.Fatalf("Failed to check broadcast award: %v", err)
	}

	// Assert the expected response
	assert.Equal(t, "Has Broadcast", response["message"], "Expected message to match")

	// Now, update the hasBroadcast flag to false and check again
	testUser.HasBroadcast = false

	// Call the function being tested
	response, err = CheckBroadcastAward(context.Background(), testTiDB, &testUser)
	if err != nil {
		t.Fatalf("Failed to check broadcast award: %v", err)
	}

	// Assert the expected response
	assert.Equal(t, "No Broadcast", response["message"], "Expected message to match")

	t.Log("TestCheckBroadcastAward succeeded")
}

func TestRevertBroadcastAward(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Insert a test user into the users table
	userID := int64(1)
	userName := "Test User"
	hasBroadcast := true

	testUser := models.User{
		ID:           userID,
		UserName:     userName,
		HasBroadcast: hasBroadcast,
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to create user statement: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Call the function being tested
	response, err := RevertBroadcastAward(context.Background(), testTiDB, &testUser)
	if err != nil {
		t.Fatalf("Failed to revert broadcast award: %v", err)
	}

	// Assert the expected response
	assert.Equal(t, "Revert Successful", response["message"], "Expected message to match")

	// Query the updated user from the database
	res, err := testTiDB.DB.Query("SELECT has_broadcast FROM users WHERE _id = ?", userID)
	if err != nil {
		t.Fatalf("Failed to query updated user: %v", err)
	}
	defer res.Close()

	// Check if the broadcast award has been reverted for the user
	if !res.Next() {
		t.Fatal("Expected user not found in the database")
	}

	var hasBroadcastAfterRevert bool
	err = res.Scan(&hasBroadcastAfterRevert)
	if err != nil {
		t.Fatalf("Failed to scan result: %v", err)
	}

	if hasBroadcastAfterRevert {
		t.Fatal("Expected the user's has_broadcast field to be false, but it was true")
	}

	t.Log("TestRevertBroadcastAward succeeded")
}
