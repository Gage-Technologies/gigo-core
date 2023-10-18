package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
)

func GiveRandomAward(ctx context.Context, db *ti.Database, userId int64, logger logging.Logger) (*models.RewardsFrontend, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "give-random-award-core")
	callerName := "GiveRandomAward"

	var reward models.RewardsSQL

	res, err := db.QueryContext(ctx, &span, &callerName,
		"select * from rewards where _id not in (select reward_id from user_rewards_inventory where user_id = ?) order by rand() limit 1",
		userId,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting rewards with query: %v: %v",
			"select * from rewards where _id not in (select reward_id from user_rewards_inventory where user_id = ?) order by rand() limit 1",
			err,
		)
	}

	// we need to handle the case that someone has received all rewards
	if !res.Next() {
		return nil, nil
	}

	err = sqlstruct.Scan(&reward, res)
	if err != nil {
		return nil, fmt.Errorf("error scanning rewards: %v", err)
	}

	_, err = db.ExecContext(ctx, &span, &callerName, "insert into user_rewards_inventory (reward_id, user_id) values (?, ?)", reward.ID, userId)
	if err != nil {
		return nil, fmt.Errorf("error inserting user rewards inventory: %v", err)
	}

	logger.Debugf("user %d has awarded %d rewards", userId, reward)

	return &models.RewardsFrontend{
		ID:            fmt.Sprintf("%d", reward.ID),
		UserID:        fmt.Sprintf("%d", userId),
		Name:          reward.Name,
		RenderInFront: reward.RenderInFront,
		ColorPalette:  reward.ColorPalette,
	}, nil
}

func GetUserRewardsInventory(ctx context.Context, db *ti.Database, userId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-rewards-inventory-core")
	callerName := "GetUserRewardsInventory"

	res, err := db.QueryContext(ctx, &span, &callerName, "select r._id, r.color_palette, r.render_in_front, r.name from user_rewards_inventory uri join rewards r on uri.reward_id = r._id where uri.user_id = ?", userId)
	if err != nil {
		return nil, fmt.Errorf("error getting user rewards inventory: %v", err)
	}

	defer res.Close()

	rewards := make([]*models.RewardsFrontend, 0)

	for res.Next() {
		var reward models.Rewards
		err = sqlstruct.Scan(&reward, res)
		if err != nil {
			return nil, fmt.Errorf("error scanning user rewards inventory: %v", err)
		}

		rewards = append(rewards, reward.ToFrontend())
	}

	return map[string]interface{}{"rewards": rewards}, nil
}

func SetUserReward(ctx context.Context, db *ti.Database, userId int64, rewardId *int64) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "set-user-reward-core")
	callerName := "SetUserReward"

	if rewardId == nil {
		_, err := db.ExecContext(ctx, &span, &callerName, "update users set avatar_reward = null where _id =?", userId)
		if err != nil {
			return fmt.Errorf("failed to update user avatar reward: %v", err)
		}
	} else {
		res, err := db.QueryContext(ctx, &span, &callerName, "select * from user_rewards_inventory where user_id = ? and reward_id = ?", userId, rewardId)
		if err != nil {
			return fmt.Errorf("could not query for if reward is available to user: %v", err)
		}
		defer res.Close()
		if !res.Next() {
			return fmt.Errorf("user does not have access to reward: %v", rewardId)
		}

		_, err = db.ExecContext(ctx, &span, &callerName, "update users set avatar_reward = ? where _id =?", rewardId, userId)
		if err != nil {
			return fmt.Errorf("failed to update user avatar reward: %v", err)
		}
	}
	return nil
}
