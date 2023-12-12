package core

import (
	"context"
	"gigo-core/gigo/config"
	"testing"

	config2 "github.com/gage-technologies/gigo-lib/config"
	"github.com/go-redis/redis/v8"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
)

func TestAddXP(t *testing.T) {
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
		t.Errorf("\nTestBroadcastMessage failed\n    Error: %v\n", err)
		return
	}

	// Create test users
	user, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
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

	// Insert a test reward
	_, err = testTiDB.DB.Exec(`INSERT INTO rewards (_id, name, color_palette, render_in_front) VALUES (?, ?, ?, ?)`, 1, "Test Reward", "blue", false)
	if err != nil {
		t.Fatalf("Failed to insert test reward: %v", err)
	}

	// Insert a user's reward inventory
	_, err = testTiDB.DB.Exec(`INSERT INTO user_rewards_inventory (user_id, reward_id) VALUES (?, ?)`, user.ID, 1)
	if err != nil {
		t.Fatalf("Failed to insert test user's reward inventory: %v", err)
	}

	var testTier *models.TierType
	testRenown1 := 2
	testRenown2 := 3
	// Test scenarios
	tests := []struct {
		name                 string
		source               string
		renownOfChallenge    *models.TierType
		nemesisBasesCaptured *int
	}{
		{"Test Attempt XP", "attempt", testTier, nil},
		{"Test Successful XP", "successful", testTier, nil},
		{"Test Login XP", "login", nil, nil},
		{"Test Tutorial XP", "tutorial", nil, nil},
		{"Test Streak XP", "streak", nil, nil},
		{"Test Create Challenge XP", "create", nil, nil},
		{"Test Refer XP", "refer", nil, nil},
		{"Test ERNO XP", "erno", nil, nil},
		{"Test Quest XP", "quest", nil, nil},
		{"Test Engagement XP", "engagement", nil, nil},
		{"Test Learning XP", "learning", nil, nil},
		{"Test Create Tutorial XP", "create_tutorial", nil, nil},
		{"Test Challenge Attempted XP", "challenge_is_attempted", nil, nil},
		{"Test Nemesis Victory XP", "nemesis_victory", nil, &testRenown2},
		{"Test Nemesis Defeat XP", "nemesis_defeat", nil, &testRenown1},
	}

	// create variable to hold redis client
	var rdb redis.UniversalClient

	// create local client
	rdb = redis.NewClient(&redis.Options{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AddXP(context.Background(), testTiDB, js, rdb, sf, config.StripeSubscriptionConfig{}, user.ID, tt.source, tt.renownOfChallenge, tt.nemesisBasesCaptured, logger, user)
			if err != nil {
				t.Fatalf("Failed to add XP for %s: %v", tt.name, err)
			}

			if result == nil {
				t.Fatalf("No result returned for %s", tt.name)
			}

			xpUpdate := result["xp_update"]
			if xpUpdate == nil {
				t.Fatalf("No XP update returned for %s", tt.name)
			}

			levelUpReward := result["level_up_reward"]
			if levelUpReward == nil {
				t.Logf("No level up reward returned for %s", tt.name)
			}
		})
	}

	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM user_rewards_inventory WHERE user_id = ? AND reward_id = ?`, user.ID, 1)
		if err != nil {
			t.Logf("Failed to delete test user's reward inventory: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM rewards WHERE _id = ?`, 1)
		if err != nil {
			t.Logf("Failed to delete test reward: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM users WHERE _id = ?`, user.ID)
		if err != nil {
			t.Logf("Failed to delete test user: %v", err)
		}
	}()

}

func TestLevelUpLoot(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestLevelUpLoot failed\n    Error: %v\n", err)
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
		t.Errorf("\nTestLevelUpLoot failed\n    Error: %v\n", err)
		return
	}

	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/yourpackage-test.log"))
	if err != nil {
		t.Fatal(err)
	}

	// Call the LevelUpLoot function
	result, err := LevelUpLoot(context.Background(), testTiDB, sf, config.StripeSubscriptionConfig{}, testUser.ID, logger, testUser)
	if err != nil {
		t.Errorf("LevelUpLoot() error = %v", err)
		return
	}

	if result == nil {
		t.Fatalf("LevelUpLoot() result is nil")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}

func TestGetXPBoostCount(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetXPBoostCount failed\n    Error: %v\n", err)
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

	// Call the GetXPBoostCount function
	result, err := GetXPBoostCount(context.Background(), testUser, testTiDB)
	if err != nil {
		t.Errorf("GetXPBoostCount() error = %v", err)
		return
	}

	if result == nil {
		t.Fatalf("GetXPBoostCount() result is nil")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}

func TestStartXPBoost(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestStartXPBoost failed\n    Error: %v\n", err)
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

	// Create a test XP boost
	xpBoost := models.CreateXPBoost(1, testUser.ID, nil)

	xpBoostStmt := xpBoost.ToSQLNative()

	for _, stmt := range xpBoostStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test xp boost: %v", err)
		}
	}

	// Call the StartXPBoost function
	result, err := StartXPBoost(context.Background(), testUser, testTiDB, xpBoost.ID)
	if err != nil {
		t.Errorf("StartXPBoost() error = %v", err)
		return
	}

	if result == nil {
		t.Fatalf("StartXPBoost() result is nil")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM xp_boosts WHERE _id = 1")
	}()
}
