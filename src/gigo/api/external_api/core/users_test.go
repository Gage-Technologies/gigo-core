package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/utils"
	"reflect"
	"testing"
	"time"
)

// func TestCreateNewUser(t *testing.T) {
//	//todo: create vcs and coder clients
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	testSnowflake, err := snowflake.NewNode(0)
//	if err != nil {
//		t.Error("\nTestCreateNewUser failed\n    Error: ", err)
//		return
//	}
//
//	user, err := CreateNewUser(testTiDB, testSnowflake, "test", "testPassword", "email", "phone", "status", "pfp", "bio", []int64{int64(32)}, models.TierType(int64(2)), uint64(2), uint64(67))
//	if err != nil {
//		t.Error("\nTestCreateNewUser failed\n    Error: ", err)
//		return
//	}
//
//	if user["message"] != "User Created." {
//		t.Error("\nTestCreateNewUser failed\n    Error: Incorrect message")
//		return
//	}
//
//	t.Log("\nTestCreateNewUser succeeded")
//
// }
//

func TestChangePhoneNumber(t *testing.T) {
	testDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestChangePhoneNumber failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	tests := []struct {
		name        string
		callingUser *models.User
		tidb        *ti.Database
		newPhone    string
		wantErr     bool
		wantMessage string
	}{
		{
			name:        "Success",
			callingUser: user,
			tidb:        testDB,
			newPhone:    "555-555-1234",
			wantErr:     false,
			wantMessage: "Phone number updated successfully",
		},
		{
			name:        "Phone number too short",
			callingUser: user,
			tidb:        testDB,
			newPhone:    "123",
			wantErr:     true,
		},
		{
			name:        "Phone number too long",
			callingUser: user,
			tidb:        testDB,
			newPhone:    "555-555-5555-555-5555",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ChangePhoneNumber(context.Background(), tt.callingUser, tt.tidb, tt.newPhone)

			if (err != nil) != tt.wantErr {
				t.Errorf("ChangePhoneNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if gotResult["message"] != tt.wantMessage {
					t.Errorf("ChangePhoneNumber() message = %v, wantMessage %v", gotResult["message"], tt.wantMessage)
				}

				// Check if the phone number is updated in the database
				var updatedPhone string
				err = tt.tidb.DB.QueryRow("SELECT phone FROM users WHERE _id = ?", tt.callingUser.ID).Scan(&updatedPhone)
				if err != nil {
					t.Fatal("Failed to query updated phone number:", err)
				}

				if updatedPhone != tt.newPhone {
					t.Errorf("Expected phone number to be updated to %s, but got %s", tt.newPhone, updatedPhone)
				}
			}
		})
	}
}

func TestChangeUsername(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create two test users
	testUser1, err := models.CreateUser(100, "testUser1", "testUser1@email.com", "", "hashedPassword1", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Fatal("Create user 1 failed:", err)
	}

	testUser2, err := models.CreateUser(101, "testUser2", "testUser2@email.com", "", "hashedPassword2", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Fatal("Create user 2 failed:", err)
	}

	// Insert the test users into the database
	for _, testUser := range []*models.User{testUser1, testUser2} {
		userStmt, err := testUser.ToSQLNative()
		if err != nil {
			t.Fatal("Convert user to SQL failed:", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatal("Insert test user failed:", err)
			}
		}
	}

	// Test cases
	tests := []struct {
		name        string
		callingUser *models.User
		tidb        *ti.Database
		newUsername string
		wantErr     bool
		wantMessage string
	}{
		{
			name:        "Test ChangeUsername successful",
			callingUser: testUser1,
			tidb:        testTiDB,
			newUsername: "newUsername",
			wantErr:     false,
			wantMessage: "Username updated successfully",
		},
		{
			name:        "Test ChangeUsername with existing username",
			callingUser: testUser1,
			tidb:        testTiDB,
			newUsername: "testUser2",
			wantErr:     true,
		},
		{
			name:        "Test ChangeUsername with short username",
			callingUser: testUser1,
			tidb:        testTiDB,
			newUsername: "a",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ChangeUsername(context.Background(), tt.callingUser, tt.tidb, tt.newUsername)
			if (err != nil) != tt.wantErr {
				t.Errorf("ChangeUsername() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if gotResult["message"] != tt.wantMessage {
					t.Errorf("ChangeUsername() message = %v, wantMessage %v", gotResult["message"], tt.wantMessage)
				}

				// Check if the username is updated in the database
				var updatedUsername string
				err = tt.tidb.DB.QueryRow("SELECT user_name FROM users WHERE _id = ?", tt.callingUser.ID).Scan(&updatedUsername)
				if err != nil {
					t.Fatal("Failed to query updated username:", err)
				}

				if updatedUsername != tt.newUsername {
					t.Errorf("Expected username to be updated to %s, but got %s", tt.newUsername, updatedUsername)
				}
			}
		})
	}
}

func TestForgotPasswordValidation(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Hash the test user's password
	hashedPassword, err := utils.HashPassword("test")
	if err != nil {
		t.Fatal("Failed to hash test user's password:", err)
	}

	// Create a test user with the hashed password
	testUser, err := models.CreateUser(69, "test", "test@email.com", "", hashedPassword, models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Fatal("Create user failed:", err)
	}

	// Insert the test user into the database
	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatal("Convert user to SQL failed:", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatal("Insert test user failed:", err)
		}
	}

	// Test cases
	tests := []struct {
		name        string
		email       string
		username    string
		tidb        *ti.Database
		url         string
		wantErr     bool
		wantMessage string
	}{
		{
			name:        "Test ForgotPasswordValidation successful",
			email:       testUser.Email,
			username:    testUser.UserName,
			tidb:        testTiDB,
			url:         "example.com",
			wantErr:     false,
			wantMessage: "Password reset email sent",
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ForgotPasswordValidation(context.Background(), tt.tidb, "", "", tt.email, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ForgotPasswordValidation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if gotResult["message"] != tt.wantMessage {
					t.Errorf("ForgotPasswordValidation() gotResult message = %v, want %v", gotResult["message"], tt.wantMessage)
				}
			}
		})
	}
}

//func TestResetForgotPassword(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
//	if err != nil {
//		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
//	}
//
//	// Create a MeiliSearch client
//	cfg := config.MeiliConfig{
//		Host:  "http://gigo-dev-meili:7700",
//		Token: "gigo-dev",
//		Indices: map[string]config.MeiliIndexConfig{
//			"posts": {
//				Name:                 "posts",
//				PrimaryKey:           "_id",
//				SearchableAttributes: []string{"title", "description", "languages", "tags"},
//				FilterableAttributes: []string{"languages", "tags"},
//				SortableAttributes:   []string{},
//			},
//		},
//	}
//
//	meili, err := search.CreateMeiliSearchEngine(cfg)
//	if err != nil {
//		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
//	}
//
//	ctx := context.Background()
//	streakEngine := new(streak.StreakEngine)
//	snowflakeNode := new(snowflake.Node)
//	domain := "test.com"
//	userName := "testuser"
//	password := "testpass"
//	email := "testuser@test.com"
//	phone := "1234567890"
//	bio := "This is a test user"
//	firstName := "Test"
//	lastName := "User"
//	starterUserInfo := models.UserStart{}
//	timezone := "America/New_York"
//	thumbnailPath := "path/to/thumbnail"
//	storageEngine := new(storage.Storage)
//	avatarSettings := models.AvatarSettings{}
//	filter := new(utils3.PasswordFilter)
//	forcePass := false
//
//	// Validate user info before creating
//	_, err = ValidateUserInfo(ctx, tidb, userName, password, email, phone, timezone, filter, forcePass)
//	assert.NoError(t, err)
//
//	userInfo, err := CreateNewUser(ctx, tidb, meili, streakEngine, snowflakeNode, domain, userName, password, email, phone, bio, firstName, lastName, vcsClient, starterUserInfo, timezone, thumbnailPath, storageEngine, avatarSettings, filter, forcePass)
//
//	assert.NoError(t, err)
//
//	assert.Equal(t, "User Created.", userInfo["message"])
//
//	// Check that the user field is correctly populated
//	user, ok := userInfo["user"]
//	assert.True(t, ok)
//
//	// Test cases
//	tests := []struct {
//		name            string
//		userId          string
//		newPassword     string
//		retypedPassword string
//		filter          *utils3.PasswordFilter
//		forcePass       bool
//		validToken      bool
//		wantErr         bool
//		wantMessage     string
//	}{
//		{
//			name:            "Test ResetForgotPassword successful",
//			userId:          fmt.Sprintf("%v", testUser.ID),
//			newPassword:     "newpassword",
//			retypedPassword: "newpassword",
//			filter:          nil,
//			forcePass:       false,
//			validToken:      true,
//			wantErr:         false,
//			wantMessage:     "Password reset successfully",
//		},
//		// Add more test cases if needed
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			gotResult, err := ResetForgotPassword(context.Background(), testTiDB, vcsClient, tt.userId, tt.newPassword, tt.retypedPassword, tt.filter, tt.forcePass, tt.validToken)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("ResetForgotPassword() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//
//			if !tt.wantErr {
//				if gotResult["message"] != tt.wantMessage {
//					t.Errorf("ResetForgotPassword() gotResult message = %v, want %v", gotResult["message"], tt.wantMessage)
//				}
//			}
//		})
//	}
//}

func TestDeleteUserAccount(t *testing.T) {
	// Initialize test database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"users": {
				Name:                 "users",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"name", "email"},
				FilterableAttributes: []string{"user_status"},
				SortableAttributes:   []string{"created_at"},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	vcsClient, err := git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		t.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	// Create a user
	user, err := models.CreateUser(420, "test", "", "", "", models.UserStatusPremium, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestMostChallengingActive failed\n    Error: %v\n", err)
		return
	}

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

	// Begin a transaction
	tx, err := testTiDB.DB.Begin()
	if err != nil {
		t.Fatalf("Failed to open transaction: %v", err)
	}

	attempt, err := models.CreateAttempt(1, "title", "description", user.UserName, user.ID, time.Now(), time.Now(), 1, user.Tier, nil, 0, 420, 1, nil, 0)
	if err != nil {
		t.Errorf("\nTestProjectAttempts failed\n    Error: %v\n", err)
		return
	}
	atpmtstmt, err := attempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert attempt to SQL: %v", err)
	}
	for _, stmt := range atpmtstmt {
		_, err = tx.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test attempt: %v", err)
		}
	}

	post, err := models.CreatePost(
		69420, "test", "content", user.UserName, user.ID, time.Now(),
		time.Now(), 69, 1, []int64{}, nil, 6969, 20, 40, 24, 27,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil,
		64752, 3, &models.DefaultWorkspaceSettings, false, false, nil,
	)
	if err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	postStmt, err := post.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		if _, err = tx.Exec(stmt.Statement, stmt.Values...); err != nil {
			t.Fatalf("Failed to insert test post: %v", err)
		}
	}

	discussion, err := models.CreateDiscussion(69, "test", user.UserName, user.ID,
		time.Now(), time.Now(), 1, []int64{}, 6, post.ID, "test", nil, false, 0, 0)
	if err != nil {
		t.Errorf("\nTestGetDiscussions failed\n    Error: %v\n", err)
		return
	}

	statement := discussion.ToSQLNative()

	for _, stmt := range statement {
		_, err = tx.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Errorf("\nTestGetDiscussions failed\n    Error: %v", err)
			return
		}
	}

	comment, err := models.CreateComment(69, "test", user.UserName, user.ID, time.Now(), 1, []int64{}, 0, 4, false, 2, 1)
	if err != nil {
		t.Fatalf("Failed to create test comment: %v", err)
	}
	commentStmt := comment.ToSQLNative()

	for _, stmt := range commentStmt {
		if _, err = tx.Exec(stmt.Statement, stmt.Values...); err != nil {
			t.Fatalf("Failed to insert test comment: %v", err)
		}
	}

	threadComment, err := models.CreateThreadComment(69, "test", user.UserName, user.ID, time.Now(), 1, 0, 4, false, 2, 1)
	if err != nil {
		t.Fatalf("Failed to create test thread comment: %v", err)
	}
	threadCommentStmt := threadComment.ToSQLNative()

	for _, stmt := range threadCommentStmt {
		if _, err = tx.Exec(stmt.Statement, stmt.Values...); err != nil {
			t.Fatalf("Failed to insert test thread comment: %v", err)
		}
	}

	threadReply, err := models.CreateThreadReply(69, "test", user.UserName, user.ID, time.Now(), 1, 0, 4, 2, 1)
	if err != nil {
		t.Fatalf("Failed to create test thread reply: %v", err)
	}
	threadReplyStmt := threadReply.ToSQLNative()

	for _, stmt := range threadReplyStmt {
		if _, err = tx.Exec(stmt.Statement, stmt.Values...); err != nil {
			t.Fatalf("Failed to insert test thread reply: %v", err)
		}
	}

	friend, err := models.CreateUser(2, "testuser2", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
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

	friendReq, err := models.CreateFriendRequests(69, user.ID, user.UserName, friend.ID, friend.UserName, time.Now(), 69)
	if err != nil {
		t.Errorf("\nTestSendFriendRequest failed\n    Error: %v\n", err)
		return
	}

	reqStmt := friendReq.ToSQLNative()

	_, err = testTiDB.DB.Exec(reqStmt.Statement, reqStmt.Values...)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// follower, err := models.CreateFollower(user.ID, friend.ID)
	// if err != nil {
	//	t.Fatalf("Failed to create test thread reply: %v", err)
	// }
	// followerStatemnt := follower.ToSQLNative()
	//
	// if _, err = tx.Exec(followerStatemnt.Statement, followerStatemnt.Values); err != nil {
	//	t.Fatalf("Failed to insert test thread reply: %v", err)
	// }

	currentTime := time.Now()

	nemesis := models.CreateNemesis(696969, user.ID, user.UserName, friend.ID, friend.UserName, time.Now(), nil, true, &currentTime, uint64(1), uint64(1))
	if nemesis == nil {
		t.Errorf("\nTestSendFriendRequest failed\n    Error: %v\n", err)
		return
	}

	statements := nemesis.ToSQLNative()
	for _, statement := range statements {
		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\nTestSendFriendRequest failed\n    Error: %v\n", err)
			return
		}
	}

	userStats, err := models.CreateUserStats(6969420, user.ID, 4, false, 0, 2, time.Duration(4), time.Duration(4), 2, 2, 2, time.Now(), time.Now(), nil)
	if err != nil {
		t.Errorf("\nTestSendFriendRequest failed\n    Error: %v\n", err)
		return
	}
	statStmnt := userStats.ToSQLNative()

	for _, stmt := range statStmnt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert friend: %v", err)
		}
	}

	defer func() {
		testTiDB.DB.Exec("delete from users where user_name = ? and _id = ?;", "test", 420)
		testTiDB.DB.Exec("delete from users where user_name = ? and _id = ?;", "testuser2", 2)
		testTiDB.DB.Exec("delete from attempt where post_title = ? and _id = ?;", "title", 1)
		testTiDB.DB.Exec("delete from post where title = ? and _id = ?;", "test", 69420)
		testTiDB.DB.Exec("delete from discussion where _id = ?;", 69)
		testTiDB.DB.Exec("delete from comment where _id = ?;", 69)
		testTiDB.DB.Exec("delete from thread_comment where _id = ?;", 69)
		testTiDB.DB.Exec("delete from thread_reply where _id = ?;", 69)
		testTiDB.DB.Exec("delete from friend_requests where _id = ?;", 69)
		testTiDB.DB.Exec("delete from nemesis where _id = ?;", 696969)
		testTiDB.DB.Exec("delete from user_stats where _id = ?;", 6969420)
	}()

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Call the DeleteUserAccount function
	response, err := DeleteUserAccount(context.Background(), testTiDB, meili, vcsClient, user)
	if err != nil {
		t.Errorf("DeleteUserAccount() error = %v", err)
		return
	}

	// Check that the function returned the expected response
	expectedResponse := map[string]interface{}{"message": "Account has been deleted."}
	if !reflect.DeepEqual(response, expectedResponse) {
		t.Errorf("DeleteUserAccount() = %v, want %v", response, expectedResponse)
	}

	// Check if the user was deleted
	var count int
	err = testTiDB.DB.QueryRow("SELECT COUNT(*) FROM users WHERE _id = ?", user.ID).Scan(&count)
	if err != nil {
		t.Errorf("Failed to fetch user count: %v", err)
		return
	}

	if count != 0 {
		t.Errorf("User was not deleted, got count: %v, want: 0", count)
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM users WHERE _id = ?", user.ID)
	}()
}

// func TestChangePhoneNumber(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	var badges []int64
//
//	user, err := models.CreateUser(69, "", "test", "123456789", "testing@test.com", "2845765803", "", "", badges, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago")
//	if err != nil {
//		t.Errorf("\nTestChangePhoneNumber failed\n    Error: %v\n", err)
//		return
//	}
//
//	statements := user.ToSQLNative()
//
//	for _, statement := range statements {
//		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nTestChangePhoneNumber failed\n    Error: %v\n", err)
//			return
//		}
//	}
//
//	res, err := ChangePhoneNumber(*user, testTiDB, "4696694204")
//	if err != nil {
//		t.Errorf("\nTestChangePhoneNumber failed\n    Error: %v\n", err)
//		return
//	}
//
//	if res["message"] != "Phone number updated successfully" {
//		t.Error("\nTestChangePhoneNumber failed\n    Error: Incorrect message")
//	}
//
//	t.Log("\nTestChangePhoneNumber succeeded")
// }

// func TestForgotPassword(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}

//	var badges []int64
//
//	user, err := models.CreateUser(69, "", "user", "123456789", "test@test.com", "2845765803", "", "", badges, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago")
//	if err != nil {
//		t.Errorf("\nTestForgotPassword failed\n    Error: %v\n", err)
//		return
//	}
//	statements := user.ToSQLNative()
//
//	for _, statement := range statements {
//		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nTestForgotPassword failed\n    Error: %v\n", err)
//			return
//		}
//	}
//
//	defer func() {
//		testTiDB.DB.Exec("drop table users")
//	}()
//
//	var email *string
//
//	e := "test@test.com"
//
//	email = &e
//
//	res, err := ForgotPassword(testTiDB, email, nil, "test42069")
//	if err != nil {
//		t.Errorf("\nTestForgotPassword failed\n    Error: %v\n", err)
//		return
//	}
//
//	if res["message"] != "Password Successfully reset" {
//		t.Error("\nTestForgotPassword failed\n    Error: Incorrect message")
//	}
//
//	t.Log("\nTestForgotPassword succeeded")
// }
//
// //func TestChangeUserPicture(t *testing.T) {
// //	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
// //	var badges []int64
// //
// //	user, err := models.CreateUser(42069, "path", "username", "password", "email", "phone", "status", "bio", badges, 3, 1, 2, badges, nil)
// //	if err != nil {
// //		t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
// //		return
// //	}
// //
// //	stmt := user.ToSQLNative()
// //
// //	for _, statement := range stmt {
// //		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
// //		if err != nil {
// //			t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
// //			return
// //		}
// //	}
// //
// //	defer func() {
// //		testTiDB.DB.Exec("drop table users")
// //	}()
// //
// //	testImage := "updated_path"
// //
// //	res, err := ChangeUserPicture(*user, testTiDB, testImage)
// //	if err != nil {
// //		t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
// //		return
// //	}
// //
// //	if res["message"] != "Profile picture updated successfully" {
// //		t.Error("\nTestChangeUserPicture failed\n    Error: Incorrect message")
// //		return
// //	}
// //
// //	t.Log("\nTestChangeUserPicture succeeded")
// //
// //	testTiDB.DB.Exec("drop table users")
// //}
// //
// func TestDeleteUserAccount(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	var badges []int64
//
//	user, err := models.CreateUser(69, "path", "username", "password", "email", "phone", "status", "bio", badges, nil, "first", "last", 420 ,"none", models.UserStart{}, "America/Chicago")
//	if err != nil {
//		t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
//		return
//	}
//
//	stmt := user.ToSQLNative()
//
//	for _, statement := range stmt {
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
//			return
//		}
//	}
//
//	results, err := DeleteUserAccount(testTiDB, user)
//	if err != nil {
//		t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
//		return
//	}
//
//	if results["message"] != "Account has been deleted." {
//		t.Errorf("\nTestDeleteUserAccount failed\n      Error: Message was incorrect\n")
//	}
//
//	// query for all active projects for specified user
//	res, err := testTiDB.DB.Query("select * from users where _id = ?", user.ID)
//	if err != nil {
//		t.Errorf("\nTestChangeUserPicture failed\n    Error: %v\n", err)
//		return
//	}
//
//	// ensure the closure of the rows
//	defer res.Close()
//
//
//	// check if any active projects were found
//	if res == nil {
//		t.Log("\nTestDeleteUserAccount succeeded")
//	} else {
//		t.Errorf("\nTestDeleteUserAccount failed\n      Error: User was found\n")
//		return
//	}
//
// }
//
// func TestChangeUsername(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//	var badges []int64
//
//	user, err := models.CreateUser(69, "", "test", "123456789", "test@test.com", "2845765803", "", "", badges, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago")
//	if err != nil {
//		t.Errorf("\nTestForgotPassword failed\n    Error: %v\n", err)
//		return
//	}
//	statements := user.ToSQLNative()
//
//	for _, statement := range statements {
//		_, err := testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nTestForgotPassword failed\n    Error: %v\n", err)
//			return
//		}
//	}
//
//	res, err := ChangeUsername(*user, testTiDB, "test-succeeded")
//	if err != nil {
//		t.Errorf("\nTestChangeUsername failed\n    Error: %v\n", err)
//		return
//	}
//
//	if res["message"] != "Username updated successfully" {
//		t.Errorf("\nTestChangeUsername failed\n      Error: Message was incorrect\n")
//	}
//
//	check, err := testTiDB.DB.Query("select user_name from users where _id = ? limit 1", user.ID)
//	if err != nil {
//		t.Errorf("\nTestChangeUsername failed\n    Error: %v\n", err)
//		return
//	}
//
//	var nameCheck string
//
//	for check.Next() {
//		var name string
//		err = check.Scan(&name)
//		if err != nil {
//			t.Errorf("\nTestChangeUsername failed\n    Error: %v", err)
//		}
//		nameCheck = name
//		break
//	}
//
//	// ensure the closure of the rows
//	defer check.Close()
//
//	if nameCheck != "test-succeeded" {
//		t.Errorf("\nTestChangeUsername failed\n      Error: Name was incorrect\n")
//	}
//
//	fmt.Println(fmt.Sprintf("%v", nameCheck))
//
//	t.Log("\nTestChangeUsername succeeded")
// }

func TestGetGithubId(t *testing.T) {
	user, _, err := GetGithubId("", "")
	if err != nil {

	}

	m := make(map[string]interface{})
	err = json.Unmarshal(user, &m)
	if err != nil {
		fmt.Println("error:", err)
	}

	// id := int64(m["id"].(float64))

	fmt.Printf("\n User ID: %v \n", m)

}

// func TestCreateNewGithubUser(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	testSnowflakeNode, err := snowflake.NewNode(1)
//	if err != nil {
//		log.Panicf("Error: Init() : %v", err)
//	}
//
//	_, err = CreateNewGithubUser(testTiDB, testSnowflakeNode, "01f3e86bbb5d6a01cda7")
//	if err != nil {
//		t.Errorf("\nTestCreateNewGithubUser failed\n    Error: %v\n", err)
//		return
//	}
// }

func TestUserProjects(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	awards := make([]int64, 0)

	var topReply *int64

	posts := []models.Post{
		{
			ID:          69,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
		{
			ID:          420,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
		{
			ID:          6942069,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
	}

	for i := 0; i < len(posts); i++ {
		p, err := models.CreatePost(posts[i].ID, posts[i].Title, posts[i].Description, posts[i].Author,
			posts[i].AuthorID, posts[i].CreatedAt, posts[i].UpdatedAt, posts[i].RepoID,
			posts[i].Tier, posts[i].Awards, posts[i].TopReply, posts[i].Coffee, posts[i].PostType, 69,
			posts[i].Completions, posts[i].Attempts, posts[i].Languages, posts[i].Visibility, posts[i].Tags,
			nil, nil, 2758, 24, nil, false, false, nil)
		if err != nil {
			t.Errorf("\nTestUserProjects failed\n    Error: %v", err)
			return
		}

		statements, err := p.ToSQLNative()

		for _, statement := range statements {
			_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
			if err != nil {
				t.Errorf("\nTestUserProjects failed\n    Error: %v", err)
				return
			}
		}
	}

	res, err := UserProjects(context.Background(), &models.User{ID: 420}, testTiDB, 0, 3)
	if err != nil {
		t.Errorf("\nTestUserProjects failed\n    Error: %v", err)
		return
	}

	if res == nil || len(res) == 0 {
		t.Errorf("\nTestUserProjects failed\n    Error: %v", err)
		return
	}

	if res["projects"].([]*models.PostFrontend)[0].AuthorID != "420" {
		t.Errorf("\nTestUserProjects failed\n    Error: %v", err)
		return
	}

	for _, i := range res["projects"].([]*models.PostFrontend) {
		fmt.Println(i)
	}

}
