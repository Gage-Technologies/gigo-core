package core

import (
	"context"
	"reflect"
	"strings"
	"testing"

	config2 "github.com/gage-technologies/gigo-lib/config"

	_ "gigo-core/gigo/config"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/go-redis/redis/v8"
)

func TestLogin(t *testing.T) {
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
	username := "testuser"
	password := "testpassword"
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	testUser := models.User{
		ID:       userID,
		UserName: username,
		Password: hashedPassword,
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
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

	// create variable to hold redis client
	var rdb redis.UniversalClient

	// create local client
	rdb = redis.NewClient(&redis.Options{})

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestBroadcastMessage failed\n    Error: %v\n", err)
		return
	}

	// create variable for storage engine
	var storageEngine storage.Storage

	domain := "example.com"
	ip := "127.0.0.1"

	// make test logger
	var testLogger logging.Logger

	// Call the function being tested
	response, token, err := Login(context.Background(), testTiDB, js, rdb, testSnowflake, storageEngine, domain, strings.ToLower(username), password, ip, testLogger)
	if err != nil {
		t.Fatalf("Failed to log in: %v", err)
	}

	// Assert the expected response
	expectedKeys := []string{"auth", "token", "xp"}
	for _, key := range expectedKeys {
		_, ok := response[key]
		if !ok {
			t.Fatalf("Expected key '%s' in response, but not found", key)
		}
	}

	// Assert the authentication is successful
	auth, ok := response["auth"].(bool)
	if !ok {
		t.Fatalf("Expected value of type bool, but got %T", response["auth"])
	}
	if !auth {
		t.Fatal("Expected authentication to be successful, but it was not")
	}

	// Assert the token is not empty
	if token == "" {
		t.Fatal("Expected token to be non-empty, but it was empty")
	}

	t.Log("TestLogin succeeded")
}

//func TestLoginWithGoogle(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
//		Host:        "mq://gigo-dev-nats:4222",
//		Username:    "gigo-dev",
//		Password:    "gigo-dev",
//		MaxPubQueue: 256,
//	}, logger)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	defer js.Close()
//
//	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
//		Addrs: []string{"localhost:6379"},
//	})
//
//	storageEngine := storage.NewLocalStorage("/tmp/gigo-storage-test")
//
//	sf, err := snowflake.NewNode(0)
//	if err != nil {
//		t.Errorf("\nLoginWithGoogle failed\n    Error: %v\n", err)
//		return
//	}
//
//	domain := "yourdomain.com"
//	externalAuth := "your_external_auth"
//	password := "your_password"
//	ip := "your_ip"
//
//	// Create a user and insert it into the database
//	// Replace the following dummy values with appropriate values for your function
//	user, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
//	if err != nil {
//		t.Errorf("\nTestLoginWithGoogle failed\n    Error: %v\n", err)
//		return
//	}
//
//	userStmt, err := user.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert user to SQL: %v", err)
//	}
//
//	for _, stmt := range userStmt {
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert user: %v", err)
//		}
//	}
//
//	// Set up mocks for the OAuth2 service
//	httpClient := &http.Client{}
//
//	oauth2Service, err := oauth2.New(httpClient)
//	if err != nil {
//		t.Fatalf("Failed to start oauth2 service: %v", err)
//	}
//
//	// Replace this with the appropriate mocking code for the OAuth2 service
//	// ...
//
//	result, token, err := LoginWithGoogle(testTiDB, js, rdb, sf, storageEngine, domain, externalAuth, password, ip, logger)
//	if err != nil {
//		t.Errorf("LoginWithGoogle() error = %v", err)
//		return
//	}
//
//	assert.NotEmpty(t, token)
//	// Add more assertions to check the returned result
//
//	// Deferred removal of inserted data
//	defer func() {
//		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
//		// Clean up any other created data if necessary
//	}()
//
//	// Check if the user session was inserted into the database
//	var userSession models.UserSession
//	err = testTiDB.DB.QueryRow("SELECT _id, user_id, service_key, expires_at FROM user_session WHERE user_id = ?", user.ID).Scan(&userSession.ID, &userSession.UserID, &userSession.ServiceKey, &userSession.ExpiresAt)
//	if err != nil {
//		t.Errorf("Failed to retrieve user session from database: %v", err)
//	}
//
//	if userSession.UserID != user.ID {
//		t.Errorf("User session not inserted correctly: expected user_id = %d; got user_id = %d", user.ID, userSession.UserID)
//	}
//
//}

//func TestLoginWithGithub(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	// Create a storage.Storage for testing purposes
//	storageEngine := storage.NewMemoryStorage()
//
//	// Create a Github user
//	user, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
//	if err != nil {
//		t.Errorf("\nTestLoginWithGithub failed\n    Error: %v\n", err)
//		return
//	}
//
//	user.ExternalAuth = 12345 // Sample Github ID
//
//	userStmt, err := user.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert user to SQL: %v", err)
//	}
//
//	for _, stmt := range userStmt {
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert user: %v", err)
//		}
//	}
//
//	// Mock GetGithubId function
//	originalGetGithubId := GetGithubId
//	defer func() { GetGithubId = originalGetGithubId }()
//	GetGithubId = func(externalAuth string) ([]byte, error) {
//		userInfo := map[string]interface{}{
//			"id": float64(user.ExternalAuth),
//		}
//		return json.Marshal(userInfo)
//	}
//
//	ip := "127.0.0.1"
//
//	result, token, err := LoginWithGithub(testTiDB, storageEngine, "sample_external_auth", ip)
//	if err != nil {
//		t.Errorf("LoginWithGithub() error = %v", err)
//		return
//	}
//	expectedResult := map[string]interface{}{
//		"auth":  true,
//		"token": token,
//	}
//	if !reflect.DeepEqual(result, expectedResult) {
//		t.Errorf("LoginWithGithub() = %v, want %v", result, expectedResult)
//	}
//
//	// Add more checks and assertions based on the expected behavior of the function
//	// For example, check if the token is stored in the storage.Storage
//	// ...
//
//	// You can also add test cases for different scenarios, such as invalid externalAuth or user not found
//}

func TestConfirmGithubLogin(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a Redis client for testing purposes
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{"localhost:6379"},
	})

	// Create a Jetstream client for testing purposes
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

	// Create a Snowflake node for testing purposes
	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Fatalf("Failed to create Snowflake node: %v", err)
	}

	// Create a storage.Storage for testing purposes
	storageEngine := new(storage.FileSystemStorage)

	// Create a Github user
	user, err := models.CreateUser(1, "testuser1", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0, nil)
	if err != nil {
		t.Errorf("\nTestConfirmGithubLogin failed\n    Error: %v\n", err)
		return
	}

	// Set user's password for testing purposes
	user.Password = "hashed_password" // Replace with a hashed password

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}
	}

	//// Replace the AddXP function with a mocked version
	//originalAddXP := AddXP
	//defer func() { AddXP = originalAddXP }()
	//AddXP = func(tidb *ti.Database, js *mq.JetstreamClient, sf *snowflake.Node, userId int64, xpType string, user *models.User, authUser *models.User, logger logging.Logger) (map[string]interface{}, error) {
	//	return map[string]interface{}{"xp": 10}, nil
	//}

	ip := "127.0.0.1"
	password := "test_password" // Replace with the original password (not hashed)

	result, token, err := ConfirmGithubLogin(context.Background(), testTiDB, rdb, js, sf, storageEngine, user, password, ip, logger)
	if err != nil {
		t.Errorf("ConfirmGithubLogin() error = %v, wantErr = nil", err)
		return
	}

	wantResult := map[string]interface{}{
		"auth":  true,
		"token": token,
		"xp":    10,
	}

	if !reflect.DeepEqual(result, wantResult) {
		t.Errorf("ConfirmGithubLogin() result = %v, wantResult = %v", result, wantResult)
	}

	// Test invalid password
	invalidPassword := "wrong_password"
	_, _, err = ConfirmGithubLogin(context.Background(), testTiDB, rdb, js, sf, storageEngine, user, invalidPassword, ip, logger)
	if err == nil {
		t.Error("ConfirmGithubLogin() should return an error for an invalid password")
	}
}
