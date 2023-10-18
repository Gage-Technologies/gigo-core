package core

import (
	"context"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/google/uuid"
	"testing"
)

func TestRecordImplicitAction(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestRecordImplicitAction failed\n    Error: %v\n", err)
		return
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

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestRecordImplicitAction failed\n    Error: %v\n", err)
		return
	}

	postID := int64(1)
	sessionID := uuid.New()

	// Test all ImplicitAction types
	for _, actionType := range []models.ImplicitAction{
		models.ImplicitTypeAttemptStart,
		models.ImplicitTypeAttemptEnd,
		models.ImplicitTypeChallengeStart,
		models.ImplicitTypeChallengeEnd,
		models.ImplicitTypeInteractiveStart,
		models.ImplicitTypeClicked,
		models.ImplicitTypeClickedOff,
		models.ImplicitTypeClickedOwnedProject,
		models.ImplicitTypeClickedOffOwnedProject,
		models.ImplicitTypeOwnedProjectStart,
		models.ImplicitTypeOwnedProjectEnd,
		models.ImplicitTypeInteractiveEnd,
	} {
		// Call the RecordImplicitAction function
		err = RecordImplicitAction(context.Background(), testTiDB, sf, testUser, postID, sessionID, actionType)
		if err != nil {
			t.Errorf("RecordImplicitAction() error for action type %v = %v", actionType, err)
		}
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM implicit_rec WHERE user_id = 1")
	}()
}
