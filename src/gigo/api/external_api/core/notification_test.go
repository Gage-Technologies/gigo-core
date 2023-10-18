package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/bwmarrin/snowflake"
	config2 "github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/kisielk/sqlstruct"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCreateNotification(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a Snowflake Node
	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Fatalf("Failed to create Snowflake node: %v", err)
	}

	// Test data
	userId := int64(1)
	message := "Test notification message"
	notificationType := models.NotificationType(1)
	interactingUserId := int64(2)

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

	// Call the function being tested
	frontendNotification, err := CreateNotification(context.Background(), testTiDB, js, testSnowflake, userId, message, notificationType, &interactingUserId)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	// Assert the expected response
	assert.Equal(t, fmt.Sprintf("%v", userId), frontendNotification.UserID, "Expected user ID to match")
	assert.Equal(t, message, frontendNotification.Message, "Expected message to match")
	assert.Equal(t, notificationType, frontendNotification.NotificationType, "Expected notification type to match")
	assert.Equal(t, fmt.Sprintf("%v", interactingUserId), *frontendNotification.InteractingUserID, "Expected interacting user ID to match")
	assert.False(t, frontendNotification.Acknowledged, "Expected seen status to be false")

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM notification`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	// Query the created notification from the database
	res, err := testTiDB.DB.Query("SELECT * FROM notification WHERE _id = ?", frontendNotification.ID)
	if err != nil {
		t.Fatalf("Failed to query created notification: %v", err)
	}
	defer res.Close()

	// Check if the created notification exists in the database
	ok := res.Next()
	if !ok {
		t.Fatal("Expected to find the created notification in the database, but it was not found")
	}

	// Decode the row results into a Notification model
	dbNotification := models.Notification{}
	err = sqlstruct.Scan(&dbNotification, res)
	if err != nil {
		t.Fatalf("Failed to scan notification row: %v", err)
	}

	// Compare the created notification with the one in the database
	assert.Equal(t, frontendNotification.ID, fmt.Sprintf("%v", dbNotification.ID), "Expected database notification ID to match")
	assert.Equal(t, userId, dbNotification.UserID, "Expected database user ID to match")
	assert.Equal(t, message, dbNotification.Message, "Expected database message to match")
	assert.Equal(t, notificationType, dbNotification.NotificationType, "Expected database notification type to match")
	assert.Equal(t, interactingUserId, *dbNotification.InteractingUserID, "Expected database interacting user ID to match")
	assert.False(t, dbNotification.Acknowledged, "Expected database seen status to be false")

	t.Log("TestCreateNotification succeeded")
}

func TestAcknowledgeNotification(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
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
	_, err = testTiDB.DB.Exec(notificationStmt.Statement, notificationStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test notification: %v", err)
	}

	// Call the function being tested
	response, err := AcknowledgeNotification(context.Background(), testTiDB, notificationID)
	if err != nil {
		t.Fatalf("Failed to acknowledge notification: %v", err)
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM notification`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	// Assert the expected response
	assert.Equal(t, "Notification acknowledged", response["message"], "Expected message to match")

	// Query the acknowledged notification from the database
	res, err := testTiDB.DB.Query("SELECT * FROM notification WHERE _id = ?", notificationID)
	if err != nil {
		t.Fatalf("Failed to query acknowledged notification: %v", err)
	}
	defer res.Close()

	// Check if the acknowledged notification has been deleted from the database
	ok := res.Next()
	if ok {
		t.Fatal("Expected the acknowledged notification to be deleted from the database, but it was found")
	}

	t.Log("TestAcknowledgeNotification succeeded")
}

func TestAcknowledgeUserNotificationGroup(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Insert test notifications into the notifications table
	notificationID1 := int64(1)
	notificationID2 := int64(2)
	userID := int64(1)
	message := "Test notification message"
	notificationType := models.NotificationType(1)
	interactingUserID := int64(2)

	testNotifications := []models.Notification{
		{
			ID:                notificationID1,
			UserID:            userID,
			Message:           message,
			NotificationType:  notificationType,
			CreatedAt:         time.Now(),
			Acknowledged:      false,
			InteractingUserID: &interactingUserID,
		},
		{
			ID:                notificationID2,
			UserID:            userID,
			Message:           message,
			NotificationType:  notificationType,
			CreatedAt:         time.Now(),
			Acknowledged:      false,
			InteractingUserID: &interactingUserID,
		},
	}

	for _, testNotification := range testNotifications {
		notificationStmt := testNotification.ToSQLNative()
		_, err = testTiDB.DB.Exec(notificationStmt.Statement, notificationStmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test notification: %v", err)
		}
	}

	// Define a user
	callingUser := &models.User{ID: userID}

	// Call the function being tested
	response, err := AcknowledgeUserNotificationGroup(context.Background(), testTiDB, callingUser, int(notificationType))
	if err != nil {
		t.Fatalf("Failed to acknowledge notifications: %v", err)
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM notification`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	// Assert the expected response
	assert.Equal(t, "Notification acknowledged", response["message"], "Expected message to match")

	// Query the acknowledged notifications from the database
	res, err := testTiDB.DB.Query("SELECT * FROM notification WHERE user_id = ? AND notification_type = ?", userID, notificationType)
	if err != nil {
		t.Fatalf("Failed to query acknowledged notifications: %v", err)
	}
	defer res.Close()

	// Check if the acknowledged notifications have been deleted from the database
	ok := res.Next()
	if ok {
		t.Fatal("Expected the acknowledged notifications to be deleted from the database, but they were found")
	}

	t.Log("TestAcknowledgeUserNotificationGroup succeeded")
}

func TestClearUserNotifications(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Insert test notifications into the notifications table
	notificationID1 := int64(1)
	notificationID2 := int64(2)
	userID := int64(1)
	message := "Test notification message"
	notificationType1 := models.NotificationType(1)
	notificationType2 := models.NotificationType(2)
	interactingUserID := int64(2)

	testNotifications := []models.Notification{
		{
			ID:                notificationID1,
			UserID:            userID,
			Message:           message,
			NotificationType:  notificationType1,
			CreatedAt:         time.Now(),
			Acknowledged:      false,
			InteractingUserID: &interactingUserID,
		},
		{
			ID:                notificationID2,
			UserID:            userID,
			Message:           message,
			NotificationType:  notificationType2,
			CreatedAt:         time.Now(),
			Acknowledged:      false,
			InteractingUserID: &interactingUserID,
		},
	}

	for _, testNotification := range testNotifications {
		notificationStmt := testNotification.ToSQLNative()
		_, err = testTiDB.DB.Exec(notificationStmt.Statement, notificationStmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test notification: %v", err)
		}
	}

	// Define a user
	callingUser := &models.User{ID: userID}

	// Call the function being tested
	response, err := ClearUserNotifications(context.Background(), testTiDB, callingUser)
	if err != nil {
		t.Fatalf("Failed to clear notifications: %v", err)
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM notification`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	// Assert the expected response
	assert.Equal(t, "Notifications cleared", response["message"], "Expected message to match")

	// Query the cleared notifications from the database
	res, err := testTiDB.DB.Query("SELECT * FROM notification WHERE user_id = ?", userID)
	if err != nil {
		t.Fatalf("Failed to query cleared notifications: %v", err)
	}
	defer res.Close()

	// Check if the cleared notifications have been deleted from the database
	ok := res.Next()
	if ok {
		t.Fatal("Expected the cleared notifications to be deleted from the database, but they were found")
	}

	t.Log("TestClearUserNotifications succeeded")
}

func TestGetUserNotifications(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a Snowflake Node
	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Fatalf("Failed to create Snowflake node: %v", err)
	}

	// Test data
	callingUser := &models.User{
		ID: 1,
	}

	// Create a few test notifications
	notificationMessages := []string{"Test notification 1", "Test notification 2", "Test notification 3"}
	notificationType := models.NotificationType(1)
	interactingUserId := int64(2)

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

	for _, message := range notificationMessages {
		_, err := CreateNotification(context.Background(), testTiDB, js, testSnowflake, callingUser.ID, message, notificationType, &interactingUserId)
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}
	}

	// Call the function being tested
	response, err := GetUserNotifications(context.Background(), testTiDB, callingUser)
	if err != nil {
		t.Fatalf("Failed to get user notifications: %v", err)
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM notification`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	// Check the returned notifications
	notifications, ok := response["notifications"].([]models.NotificationFrontend)
	if !ok {
		t.Fatal("Expected notifications to be of type []models.NotificationFrontend")
	}

	// Check if the number of notifications is correct
	if len(notifications) != len(notificationMessages) {
		t.Fatalf("Expected to receive %d notifications, but received %d", len(notificationMessages), len(notifications))
	}

	// Check if the notifications have the correct messages
	for i, notification := range notifications {
		assert.Equal(t, notificationMessages[i], notification.Message, "Expected notification message to match")
	}

	t.Log("TestGetUserNotifications succeeded")
}

func TestEncodeDecode(t *testing.T) {
	// This will act as our "network", simulating the message being sent and received.
	msgChan := make(chan []byte, 1)

	// Prepare a buffer to encode into
	buf := bytes.NewBuffer(nil)

	// Create a BroadcastNotification
	notification := models2.BroadcastNotification{Notification: "Test notification"}

	// Create an encoder and encode the BroadcastNotification
	enc := gob.NewEncoder(buf)
	err := enc.Encode(&notification)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	fmt.Println(notification)

	// "Send" the encoded message
	msgChan <- buf.Bytes()

	// Simulate a listener on another goroutine
	go func() {
		// "Receive" the message
		encodedMsg := <-msgChan

		// Now decode it
		dec := gob.NewDecoder(bytes.NewBuffer(encodedMsg))
		var bn models2.BroadcastNotification
		err = dec.Decode(&bn)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}

		fmt.Println(bn)

		// Check the decoded value
		if bn.Notification != "Test notification" {
			t.Errorf("decoded value not as expected: got %v, want %v", bn.Notification, "Test notification")
		}
	}()

	// Wait a bit to allow the goroutine to execute
	time.Sleep(time.Second)
}
