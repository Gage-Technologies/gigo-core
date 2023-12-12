package core

import (
	"context"
	"fmt"
	"gigo-core/gigo/config"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
	"go.opentelemetry.io/otel"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
)

type XPUpdate struct {
	OldXP         uint64           `json:"old_xp"`
	NewXP         uint64           `json:"new_xp"`
	OldRenown     *models.TierType `json:"old_renown"`
	NewRenown     models.TierType  `json:"new_renown"`
	CurrentRenown models.TierType  `json:"current_renown"`
	OldLevel      models.LevelType `json:"old_level"`
	NewLevel      models.LevelType `json:"new_level"`
	NextLevel     models.LevelType `json:"next_level"`
	MaxXpForLvl   uint64           `json:"max_xp_for_lvl"`
}

func AddXP(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient, sf *snowflake.Node, stripeSubConfig config.StripeSubscriptionConfig, userID int64, source string, renownOfChallenge *models.TierType, nemesisBasesCaptured *int, logger logging.Logger, callingUser *models.User) (map[string]interface{}, error) {
	// create variables to load query data into
	var exp uint64
	var oldRenown models.TierType
	var oldLevel models.LevelType

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "add-xp-core")
	defer span.End()

	callerName := "AddXP"

	// query for user's current xp, tier (renown), and level
	err := tidb.QueryRowContext(ctx, &span, &callerName, "select xp, tier, level from users where _id = ?", userID).Scan(&exp, &oldRenown, &oldLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid xp source type. AddXP Core: %v", err)
	}

	// create variable to hold the amount of xp gain
	var expGain uint64

	xpReason := source

	switch source {
	// xp gain for an attempt
	case "attempt":

		if renownOfChallenge == nil {
			return nil, fmt.Errorf("no renown found type. AddXP Core")
		}

		logger.Infof("AddXP attempt with user renown: %v", oldRenown)

		challengeStr := strings.Replace(renownOfChallenge.String(), "Tier", "", -1)
		userStr := strings.Replace(oldRenown.String(), "Tier", "", -1)

		challengeRenown, err := strconv.ParseInt(challengeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse renown of challenge, err: %v", err)
		}
		// decrement the parsed renown to be index zero compliant
		challengeRenown--

		userRenown, err := strconv.ParseInt(userStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse renown of user, err: %v", err)
		}
		// decrement the parsed renown to be index zero compliant
		userRenown--

		diff := challengeRenown - userRenown

		switch diff {
		case -3:
			expGain = 50
			break
		case -2:
			expGain = 75
			break
		case -1:
			expGain = 100
			break
		case 0:
			expGain = 125
			break
		case 1:
			expGain = 150
			break
		case 2:
			expGain = 175
			break
		case 3:
			expGain = 200
			break
		default:
			if diff < -3 {
				expGain = 50
				break
			}
			if diff > 3 {
				expGain = 200
				break
			}
		}
		break
	// xp gain for a successful attempt TODO: success based on challenge level
	case "successful":
		if renownOfChallenge == nil {
			return nil, fmt.Errorf("no renown found type. AddXP Core")
		}

		challengeStr := strings.Replace(renownOfChallenge.String(), "Tier", "", -1)
		userStr := strings.Replace(oldRenown.String(), "Tier", "", -1)

		challengeRenown, err := strconv.ParseInt(challengeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse renown of challenge, err: %v", err)
		}
		// decrement the parsed renown to be index zero compliant
		challengeRenown--

		userRenown, err := strconv.ParseInt(userStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse renown of user, err: %v", err)
		}
		// decrement the parsed renown to be index zero compliant
		userRenown--

		diff := challengeRenown - userRenown

		switch diff {
		case -3:
			expGain = 100
			break
		case -2:
			expGain = 150
			break
		case -1:
			expGain = 200
			break
		case 0:
			expGain = 250
			break
		case 1:
			expGain = 500
			break
		case 2:
			expGain = 1000
			break
		case 3:
			expGain = 2000
			break
		case 4:
			expGain = 4000
			break
		case 5:
			expGain = 8000
			break
		case 6:
			expGain = 16000
			break
		case 7:
			expGain = 32000
			break
		case 8:
			expGain = 64000
			break
		case 9:
			expGain = 64000
			break
		default:
			expGain = 100
			break
		}
		break
	// xp gain for login (limit to once a day)
	//case "login":
	//	expGain = 100
	//	break
	// xp gain upon tutorial completion
	case "tutorial":
		expGain = 100
		break
	// xp gain for streak (every 10 streak days)
	case "streak":
		expGain = 100
		break
	// xp gain for creating a challenge
	case "create":
		expGain = 200
		break
	// xp gain for referring a friend
	case "refer":
		expGain = 450
		break
	// xp gain for an Early Riser or Night Owl
	case "erno":
		expGain = 50
		break
	// xp gain for mini-challenges, or quests, that users complete
	case "quest":
		expGain = 50
		break
	// xp granted from interacting with other users (e.g. commenting or liking posts)
	case "engagement":
		expGain = 25
		break
	// xp granted for completing a tutorial
	case "learning":
		expGain = 50
		break
	case "create_tutorial":
		expGain = 250
		break
	// xp granted when another user attempts your challenge
	case "challenge_is_attempted":
		expGain = 25
		break
	case "nemesis_victory":
		expGain = 100
		expGain += uint64(10 * *nemesisBasesCaptured)
		break
	case "nemesis_defeat":
		expGain = 50
		expGain += uint64(10 * *nemesisBasesCaptured)
		break
	default:
		return nil, fmt.Errorf("invalid xp source type. AddXP Core")
	}

	// create variable to hold the total amount of xp after gain
	var expTot uint64

	// calculate users total xp after adding the xp gain
	expTot = exp + expGain

	// if the user renwonw is 10 and level is 10 then this sends 0 and breaks

	// determine if the user's renown and/or level has increased
	newRenown, newLevel, _, max := models.DetermineUserRenownLevel(expTot)
	if newRenown == 69 && newLevel == 69 {
		return nil, fmt.Errorf("DetermineUserRenownLevel is broken")
	}

	// load the update to user's xp
	update := &XPUpdate{
		OldXP:       exp,
		NewXP:       expTot,
		MaxXpForLvl: max,
		NextLevel:   newLevel + 1,
	}

	// booleans to track whether renown or level increased
	//renownIncreased := false
	//levelIncreased := false

	// if the user's renown increased, update the struct
	if oldRenown != newRenown {
		update.OldRenown = &oldRenown
		update.NewRenown = newRenown
		//renownIncreased = true
		update.CurrentRenown = newRenown
	} else {
		update.CurrentRenown = oldRenown
	}

	var levelUpReward map[string]interface{}

	// if the user's level increased, update the struct
	if oldLevel != newLevel {
		update.OldLevel = oldLevel
		update.NewLevel = newLevel

		levelUpReward, err = LevelUpLoot(ctx, tidb, sf, stripeSubConfig, userID, logger, callingUser)
		if err != nil {
			return nil, fmt.Errorf("failed to get loot from level up, err: %v", err)
		}
		//levelIncreased = true
	}

	//// call AwardBroadcastCheck to see if broadcast should be awarded after xp gain
	//err = AwardBroadcastCheck(ctx, tidb, js, rdb, userID, expGain, renownIncreased, levelIncreased)
	//if err != nil {
	//	return nil, fmt.Errorf("AwardBroadcastCheck failed in AddXP Core: %v", err)
	//}

	res, err := tidb.QueryContext(ctx, &span, &callerName, "select end_date from xp_boosts where user_id = ? and end_date is not NULL and end_date >= ? ", userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get xp boosts, err: %v", err)
	}

	var boostEnd time.Time = time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
	isBoosted := false
	for res.Next() {
		err = res.Scan(&boostEnd)
		if err != nil {
			return nil, fmt.Errorf("failed to scan xp boosts, err: %v", err)
		}
		isBoosted = true
	}

	if isBoosted || boostEnd.After(time.Now()) {
		expGain = expGain * 2
		expTot = exp + expGain
		update.NewXP = expTot
	}

	// run the query to set user's xp, tier (renown), and level
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set xp = ?, tier = ?, level = ? where _id = ?", expTot, newRenown, newLevel, userID)
	if err != nil {
		return nil, fmt.Errorf("query failed in AddXP Core: %v", err)
	}

	// run the query to set user's xp, tier (renown), and level
	_, err = tidb.ExecContext(ctx, &span, &callerName, "UPDATE user_stats SET xp_gained = xp_gained + ? WHERE date = (SELECT MAX(date) FROM user_stats) AND user_id = ?;", expGain, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update xp_gained for user_stats in AddXP Core: %v", err)
	}

	currentTime := time.Now()

	reasonModel := models.CreateXPReason(sf.Generate().Int64(), userID, &currentTime, xpReason, int64(expGain))
	statement := reasonModel.ToSQLNative()
	_, err = tidb.ExecContext(ctx, &span, &callerName, statement[0].Statement, statement[0].Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to update xp_reasons for user: %v in AddXP Core: %v", userID, err)
	}

	return map[string]interface{}{"xp_update": update, "level_up_reward": levelUpReward}, nil
}

func GetXP(ctx context.Context, tidb *ti.Database, userID int64) (map[string]interface{}, error) {
	// create variables to load query data into
	var exp uint64
	var oldRenown models.TierType
	var oldLevel models.LevelType

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "add-xp-core")
	defer span.End()

	callerName := "GetXP"

	// query for user's current xp, tier (renown), and level
	err := tidb.QueryRowContext(ctx, &span, &callerName, "select xp, tier, level from users where _id = ?", userID).Scan(&exp, &oldRenown, &oldLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid xp source type. Get Core: %v", err)
	}

	// determine if the user's renown and/or level has increased
	newRenown, newLevel, min, max := models.DetermineUserRenownLevel(exp)
	if newRenown == 69 && newLevel == 69 {
		return nil, fmt.Errorf("DetermineUserRenownLevel is broken")
	}

	return map[string]interface{}{"current_xp": exp, "max_xp": max, "min_xp": min}, nil
}

func LevelUpLoot(ctx context.Context, db *ti.Database, sf *snowflake.Node, stripeSubConfig config.StripeSubscriptionConfig, userID int64, logger logging.Logger, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "level-up-loot-core")
	defer span.End()
	callerName := "LevelUpLoot"

	num := rand.Intn(1000)
	if num <= 1000 {
		// try up to 10 times to give a random avatar
		var reward *models.RewardsFrontend
		var err error
		reward, err = GiveRandomAward(ctx, db, userID, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to give avatar, err: %v", err)
		}
		// handle the case that there is no more loot to give
		if reward == nil {
			return nil, nil
		}
		return map[string]interface{}{"reward_type": "avatar_background", "reward": reward}, nil
	}
	if num > 500 && num <= 800 {
		_, err := db.ExecContext(ctx, &span, &callerName, "update user_stats set streak_freezes = streak_freezes + 1 where user_id = ? ORDER BY date DESC LIMIT 1", userID)
		if err != nil {
			return nil, fmt.Errorf("increase streak freeze failed in LevelUpLoot Core: %v", err)
		}
		return map[string]interface{}{"reward_type": "streak_freeze", "reward": "streak_freeze"}, nil
	}
	if num > 800 && num <= 950 {
		xpBoost := models.CreateXPBoost(sf.Generate().Int64(), userID, nil)
		statements := xpBoost.ToSQLNative()
		for _, statement := range statements {
			_, err := db.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to insert new xp boost in LevelUpLoot Core: %v", err)
			}
		}
		return map[string]interface{}{"reward_type": "xp_boost", "reward": "xp_boost"}, nil
	}
	if num > 950 && num <= 1000 {

		if callingUser.StripeSubscription != nil {

			sub, err := subscription.Get(*callingUser.StripeSubscription, nil)
			if err != nil {
				return map[string]interface{}{"message": "unable to grab subscription"}, err
			}

			inTrial := sub.TrialEnd > 0 && sub.TrialEnd > time.Now().Unix()

			paused := false
			if sub.PauseCollection != nil {
				paused = sub.PauseCollection.Behavior != ""
			}

			endSoon := sub.CancelAt

			// Retrieve the customer
			cust, err := customer.Get(sub.Customer.ID, nil)
			if err != nil {
				log.Fatalf("Failed to retrieve customer: %v\n", err)
			}

			//todo check this

			if inTrial || paused || endSoon != 0 || cust.Balance >= 0 {
				_, err := db.ExecContext(ctx, &span, &callerName, "update user_stats set streak_freezes = streak_freezes + 1 where user_id = ? ORDER BY date DESC LIMIT 1", userID)
				if err != nil {
					return nil, fmt.Errorf("increase streak freeze failed in LevelUpLoot Core: %v", err)
				}
				return map[string]interface{}{"reward_type": "streak_freeze", "reward": "streak_freeze"}, nil
			} else {
				_, err := FreeMonthUpdate(callingUser, db, stripeSubConfig, ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to update free month in LevelUpLoot Core: %v", err)
				}
			}

		} else {
			_, err := CreateTrialSubscription(ctx, stripeSubConfig.MonthlyPriceID, callingUser.Email, db, nil, callingUser.ID, callingUser.FirstName, callingUser.LastName)
			if err != nil {
				return nil, fmt.Errorf("failed to create trial subscription in LevelUpLoot Core: %v", err)
			}
		}

		return map[string]interface{}{"reward_type": "free_week", "reward": "free_week"}, nil
	}

	return nil, fmt.Errorf("invalid number found")
}

type XP struct {
	Count   int
	EndDate *time.Time
	Id      *int64
}

type XPFrontend struct {
	Count   int
	EndDate *time.Time
	Id      *string
}

func (i *XP) ToFrontend() *XPFrontend {
	var endDate *time.Time

	if i.EndDate != nil {
		endDate = i.EndDate
	}

	var ID *string

	if i.Id != nil {
		i := fmt.Sprintf("%v", *i.Id)
		ID = &i
	}
	return &XPFrontend{
		Id:      ID,
		Count:   i.Count,
		EndDate: endDate,
	}
}

func GetXPBoostCount(ctx context.Context, callingUser *models.User, db *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-xp-boost-count-core")
	defer span.End()
	callerName := "GetXPBoostCount"

	// execute end of day query
	res, err := db.QueryContext(ctx, &span, &callerName, "select count(*) as count, end_date, _id from xp_boosts where user_id = ? and end_date is null union select count(*) as active_count, end_date, _id from xp_boosts where user_id = ? and end_date is not null and end_date > curdate()", callingUser.ID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("Error query failed: %s", err.Error())
	}

	defer res.Close()

	nemeses := make([]XPFrontend, 0)

	for res.Next() {
		var nemesis XP
		// scan row into user stats object
		err := res.Scan(&nemesis.Count, &nemesis.EndDate, &nemesis.Id)
		if err != nil {
			return nil, fmt.Errorf("Error scan failed: %s", err.Error())
		}

		nemeses = append(nemeses, *nemesis.ToFrontend())
	}

	return map[string]interface{}{"xp data": nemeses}, nil
}

func StartXPBoost(ctx context.Context, callingUser *models.User, db *ti.Database, xpId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "start-xp-boost-core")
	defer span.End()
	callerName := "StartXPBoost"

	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("Error starting transaction failed: %s", err.Error())
	}
	defer tx.Rollback()

	currentTime := time.Now().Add(time.Hour * 24)

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update xp_boosts set end_date = ? where _id = ? and user_id = ?", currentTime, xpId, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to start xp boost: %v", err)
		tx.Rollback()
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return map[string]interface{}{"message": "xp boost has started"}, nil
}
