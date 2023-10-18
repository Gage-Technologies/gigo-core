package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/kisielk/sqlstruct"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func TestGiveRandomAward(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	userId := int64(1)

	defer os.Remove("test.log")
	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-core-test.log"))
	if err != nil {
		log.Panicf("Error: TestGiveRandomAward test logger : %v", err)
	}

	// Call the function being tested
	reward, err := GiveRandomAward(context.Background(), testTiDB, userId, testLogger)
	if err != nil {
		t.Fatalf("Failed to give random award: %v", err)
	}

	// Assert the returned reward
	assert.NotNil(t, reward, "Expected a reward to be returned")

	// Query the user_rewards_inventory table to check if the reward was added
	res, err := testTiDB.DB.Query("SELECT * FROM user_rewards_inventory WHERE user_id = ? AND reward_id = ?", userId, reward.ID)
	if err != nil {
		t.Fatalf("Failed to query user_rewards_inventory table: %v", err)
	}
	defer res.Close()

	// Check if the reward was added for the user
	ok := res.Next()
	if !ok {
		t.Fatal("Expected to find the given reward in the user_rewards_inventory table, but it was not found")
	}

	t.Log("TestGiveRandomAward succeeded")
}

func TestGetUserRewardsInventory(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Test data
	userId := int64(1)

	// Call the function being tested
	result, err := GetUserRewardsInventory(context.Background(), testTiDB, userId)
	if err != nil {
		t.Fatalf("Failed to get user rewards inventory: %v", err)
	}

	// Assert the expected response
	rewards := result["rewards"].([]*models.RewardsFrontend)

	// Query the user_rewards_inventory table
	res, err := testTiDB.DB.Query("SELECT r._id, r.color_palette, r.render_in_front, r.name FROM user_rewards_inventory uri JOIN rewards r ON uri.reward_id = r._id WHERE uri.user_id = ?", userId)
	if err != nil {
		t.Fatalf("Failed to query user_rewards_inventory table: %v", err)
	}
	defer res.Close()

	// Check if the rewards in the database match the rewards returned by the function
	for res.Next() {
		var reward models.Rewards
		err = sqlstruct.Scan(&reward, res)
		if err != nil {
			t.Fatalf("Failed to scan user rewards inventory row: %v", err)
		}

		found := false
		for _, r := range rewards {
			if r.ID == fmt.Sprintf("%d", reward.ID) {
				found = true
				assert.Equal(t, reward.Name, r.Name, "Expected reward name to match")
				assert.Equal(t, reward.ColorPalette, r.ColorPalette, "Expected reward color palette to match")
				assert.Equal(t, reward.RenderInFront, r.RenderInFront, "Expected reward render in front to match")
				break
			}
		}

		if !found {
			t.Fatalf("Expected to find reward with ID %d in the result, but it was not found", reward.ID)
		}
	}

	t.Log("TestGetUserRewardsInventory succeeded")
}

func TestSetUserReward(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
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

	// Test data
	userId := user.ID
	rewardId := int64(1)

	// Add the reward to the user_rewards_inventory table
	_, err = testTiDB.DB.Exec("INSERT INTO user_rewards_inventory (reward_id, user_id) VALUES (?, ?)", rewardId, userId)
	if err != nil {
		t.Fatalf("Failed to add the reward to the user_rewards_inventory table: %v", err)
	}

	// Call the function being tested
	err = SetUserReward(context.Background(), testTiDB, userId, &rewardId)
	if err != nil {
		t.Fatalf("Failed to set user reward: %v", err)
	}

	// Query the user table to check if the reward was set
	res, err := testTiDB.DB.Query("SELECT avatar_reward FROM users WHERE _id = ?", userId)
	if err != nil {
		t.Fatalf("Failed to query user table: %v", err)
	}
	defer res.Close()

	// Check if the reward was set for the user
	ok := res.Next()
	if !ok {
		t.Fatal("Expected to find the user in the users table, but it was not found")
	}

	var avatarReward int64
	err = res.Scan(&avatarReward)
	if err != nil {
		t.Fatalf("Failed to scan user row: %v", err)
	}

	assert.Equal(t, rewardId, avatarReward, "Expected the user's avatar reward to be set")

	t.Log("TestSetUserReward succeeded")
}
