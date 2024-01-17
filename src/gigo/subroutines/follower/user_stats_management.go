package follower

import (
	"context"
	"fmt"
	"gigo-core/gigo/streak"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/now"
	"github.com/kisielk/sqlstruct"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const queryExpiredUserStats = `
select 
    us.*, sum(ifnull(udu.open_session, 0)) as open_session, u.timezone as timezone
from user_stats us
    join users u on u._id = us.user_id
	left join user_daily_usage udu on udu.end_time is null and udu.user_id = us.user_id
where 
    us.closed = false and 
    us.expiration <= NOW()
group by us._id
`

func LaunchUserStatsManagementRoutine(ctx context.Context, db *ti.Database, streakEngine *streak.StreakEngine, snowflakeN *snowflake.Node, workerPool *pool.Pool, js *mq.JetstreamClient, nodeId int64, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "launch-user-stats-management-routine")
	defer parentSpan.End()

	// logger.Errorf("(user_stats_management: %d) starting LaunchUserStatsManagementRoutine", nodeId)
	// create subscription for session key management
	_, err := js.ConsumerInfo(streams.StreamStreakXP, "gigo-core-follower-streak-day-rollover")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamStreakXP, &nats.ConsumerConfig{
			Durable:       "gigo-core-follower-streak-day-rollover",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectDayRollover,
		})
		if err != nil {
			logger.Errorf("(user_stats_management: %d) failed to create user stats rollover consumer: %v", nodeId, err)
			return
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectDayRollover, "gigo-core-follower-streak-day-rollover", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(user_stats_management: %d) failed to create user stats rollover consumer in user state management: %v", nodeId, err)
		return
	}
	defer sub.Unsubscribe()

	// logger.Errorf("(user_stats_management: %d) acquiring message in LaunchUserStatsManagementRoutine", nodeId)
	// request next message from the subscriber. this tells use
	// that we are the follower to perform the key expiration.
	// we use such a short timeout because this will re-execute in
	// ~1s so if there is nothing to do now then we should not slow
	// down the refresh rate of the follower loop
	msg, err := getNextJob(sub, time.Millisecond*50)
	if err != nil {
		// exit silently for timeout because it simply means
		// there is nothing to do
		if err == context.DeadlineExceeded {
			return
		}
		logger.Errorf("(user_stats_management: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}

	workerPool.Go(func() {
		err := handleUserDayRollover(db, snowflakeN, logger)
		if err != nil {
			logger.Errorf("failed to handleInactiveUserDayRollover: %v", err)
		}
	})

	// ack the message so it isn't repeated
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(user_stats_management: %d) failed to ack message: %v", nodeId, err)
		return
	}

	parentSpan.AddEvent(
		"launch-user-stats-management-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}

func LaunchPremiumWeeklyFreeze(ctx context.Context, db *ti.Database, workerPool *pool.Pool, js *mq.JetstreamClient, nodeId int64, rdb redis.UniversalClient, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "launch-premium-weekly-freeze-routine")
	defer parentSpan.End()

	logger.Info("premium weekly freeze start")

	// logger.Errorf("(user_stats_management: %d) starting LaunchUserStatsManagementRoutine", nodeId)
	// create subscription for session key management
	_, err := js.ConsumerInfo(streams.SubjectPremiumFreeze, "gigo-core-streak-freeze")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamStreakXP, &nats.ConsumerConfig{
			Durable:       "gigo-core-streak-freeze",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectPremiumFreeze,
		})
		logger.Info("premium weekly freeze consumer created")
		if err != nil {
			logger.Errorf("(user_stats_management: %d) failed to create streak freeze consumer: %v", nodeId, err)
			return
		}
	}
	logger.Info("premium weekly freeze about to pull subscribe")
	sub, err := js.PullSubscribe(streams.SubjectPremiumFreeze, "gigo-core-streak-freeze", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(user_stats_management: %d) failed to create session key subscription in premium weekly freeze: %v", nodeId, err)
		return
	}
	defer sub.Unsubscribe()

	// logger.Errorf("(user_stats_management: %d) acquiring message in LaunchUserStatsManagementRoutine", nodeId)
	// request next message from the subscriber. this tells use
	// that we are the follower to perform the key expiration.
	// we use such a short timeout because this will re-execute in
	// ~1s so if there is nothing to do now then we should not slow
	// down the refresh rate of the follower loop
	msg, err := getNextJob(sub, time.Millisecond*50)
	if err != nil {
		// exit silently for timeout because it simply means
		// there is nothing to do
		if err == context.DeadlineExceeded {
			return
		}
		logger.Errorf("(user_stats_management: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}

	// logger.Errorf("(user_stats_management: %d) executing worker pool on handleInactiveUserDayRollover", nodeId)
	workerPool.Go(func() {
		err := premiumWeeklyFreeze(ctx, db, logger, rdb)
		if err != nil {
			logger.Errorf("failed to premiumWeeklyFreeze: %v", err)
		}
	})

	// ack the message so it isn't repeated
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(user_stats_management: %d) failed to ack message: %v", nodeId, err)
		return
	}

	parentSpan.AddEvent(
		"launch-premium-weekly-freeze-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}

func handleUserDayRollover(db *ti.Database, sf *snowflake.Node, logger logging.Logger) error {
	ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "handle-inactive-user-day-rollover-routine")
	defer span.End()
	callerName := "handleInactiveUserDayRollover"

	// open tx for all queries
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	// query for user stat rows that have expired
	res, err := tx.QueryContext(ctx, &callerName, queryExpiredUserStats)
	if err != nil {
		return fmt.Errorf("failed to execute expired rows query: %v", err)
	}
	defer res.Close()

	closedStats := make([]interface{}, 0)
	closedStatsParamSlots := make([]string, 0)

	newStats := make([]interface{}, 0)
	newStatsParamSlots := make([]string, 0)

	newDailyUsage := make([]interface{}, 0)
	newDailyUsageParamSlots := make([]string, 0)

	execs := make([]string, 0)
	params := make([][]interface{}, 0)

	for res.Next() {
		userStatsSQL := new(models.UserStatsSQL)
		// scan row into user stats object
		err := sqlstruct.Scan(userStatsSQL, res)
		if err != nil {
			return fmt.Errorf("error scanning user stats in first scan: %v", err)
		}

		// handle day where the streak could close
		if userStatsSQL.CurrentStreak > 0 && !userStatsSQL.StreakActive {
			if userStatsSQL.StreakFreezes > 0 {
				// if user is on a streak, has streak freeze and is inactive, decrement streak freezes and set streak_freeze_used to true
				userStatsSQL.StreakFreezes--
				userStatsSQL.CurrentStreak++

				// we only give fire if the user has been on fire prior to this - you can't get fire from a streak freeze
				if userStatsSQL.DaysOnFire > 0 {
					userStatsSQL.DaysOnFire++
				}

				// update the current user stats row to indicate that the streak freeze was used
				execs = append(execs, "update user_stats set streak_freeze_used = true, streak_freezes = streak_freezes - 1, current_streak = current_streak + 1, days_on_fire = ? where _id = ?")
				params = append(params, []interface{}{userStatsSQL.DaysOnFire, userStatsSQL.ID})
			} else {
				// if user is on a streak, does not have a streak freeze and is inactive, reset the streak to 0
				userStatsSQL.CurrentStreak = 0
				userStatsSQL.DaysOnFire = 0
			}
		}

		// calculate new date and expiration times
		oldExpiration := userStatsSQL.Expiration
		userLocation, err := time.LoadLocation(userStatsSQL.Timezone)
		if err != nil {
			return fmt.Errorf("error loading time zone %s", userStatsSQL.Timezone)
		}
		// calculate the date by taking the expiration time in the user's timezone and calculate the beginning of the day
		// we add 5 minutes to the expiration to ensure that the expiration is in the correct day
		date := now.New(oldExpiration.In(userLocation).Add(time.Minute * 5)).BeginningOfDay()
		nextExpiration := now.New(date).EndOfDay()

		// conditionally perform rollover for active workspace sessions
		if userStatsSQL.OpenSession > 0 {
			execs = append(execs, "update user_daily_usage set end_time = ? where user_id = ? and end_time is null")
			params = append(params, []interface{}{oldExpiration, userStatsSQL.UserID})

			newDailyUsage = append(
				newDailyUsage,
				userStatsSQL.UserID,
				// everything goes to UTC in the database so there is no need to mess with timezones
				time.Now(),
				nil,
				userStatsSQL.OpenSession,
				date,
			)
			newDailyUsageParamSlots = append(newDailyUsageParamSlots, "(?, ?, ?, ?, ?)")
		}

		// Append data for new row
		newStats = append(newStats,
			sf.Generate().Int64(),
			userStatsSQL.UserID,
			userStatsSQL.ChallengesCompleted,
			false,
			userStatsSQL.CurrentStreak,
			userStatsSQL.LongestStreak,
			userStatsSQL.TotalTimeSpent,
			userStatsSQL.AvgTime,
			userStatsSQL.DaysOnPlatform,
			userStatsSQL.DaysOnFire,
			userStatsSQL.StreakFreezes,
			false,
			0,
			date,
			nextExpiration,
		)
		newStatsParamSlots = append(newStatsParamSlots, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Append data for closed row
		closedStats = append(closedStats, userStatsSQL.ID)
		closedStatsParamSlots = append(closedStatsParamSlots, "?")
	}

	// execute execs
	for i, exec := range execs {
		_, err := tx.ExecContext(ctx, &callerName, exec, params[i]...)
		if err != nil {
			return fmt.Errorf("failed to execute exec statement: %v params: %v", err, params[i])
		}
	}

	// Insert new rows
	if len(newStats) > 0 {
		insertStmt := "INSERT IGNORE INTO user_stats(_id, user_id, challenges_completed, streak_active, current_streak, longest_streak, total_time_spent, avg_time, days_on_platform, days_on_fire, streak_freezes, streak_freeze_used, xp_gained, date, expiration) VALUES " +
			strings.Join(newStatsParamSlots, ",")

		_, err = tx.ExecContext(ctx, &callerName, insertStmt, newStats...)
		if err != nil {
			return fmt.Errorf("failed to execute insert of update user stats rows: %v statement: %v params: %v", err, insertStmt, newStats)
		}
	}
	if len(newDailyUsage) > 0 {
		insertStmt := "INSERT IGNORE INTO user_daily_usage(user_id, start_time, end_time, open_session, date) VALUES " +
			strings.Join(newDailyUsageParamSlots, ",")

		_, err = tx.ExecContext(ctx, &callerName, insertStmt, newDailyUsage...)
		if err != nil {
			return fmt.Errorf("failed to execute insert of update user daily usage rows: %v statement: %v params: %v", err, insertStmt, newDailyUsage)
		}
	}

	// update user stats to mark the rows as closed
	if len(closedStats) > 0 {
		_, err = tx.ExecContext(ctx, &callerName, "update user_stats set closed = true where _id in ("+strings.Join(closedStatsParamSlots, ",")+")", closedStats...)
		if err != nil {
			return fmt.Errorf("failed to execute update of user stats rows: %v", err)
		}
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("failed to commit tx: %v", err)
	}

	return nil
}

func premiumWeeklyFreeze(ctx context.Context, db *ti.Database, logger logging.Logger, rdb redis.UniversalClient) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "premium-weekly-freeze-routine")
	defer span.End()
	callerName := "premiumWeeklyFreeze"

	logger.Debugf("starting premiumWeeklyFreeze")
	// create array to hold timezones that are past midnight in their local time

	//beginningOFDayZone := make([]interface{}, 0)
	//bParamSlots := make([]string, 0)
	//
	//// iterate over all timezones available
	//for _, v := range timezones {
	//	for _, z := range v {
	//		// load user timezone
	//		timeLocation, err := time.LoadLocation(z)
	//		if err != nil {
	//			return fmt.Errorf("error loading time zone %s", z)
	//		}
	//
	//		// calculate the beginning of the day in the timezone
	//		// beginningOfDayTZ := now.BeginningOfDay().In(timeLocation)
	//		beginningOfDayTZ := now.New(time.Now().In(timeLocation)).BeginningOfDay()
	//
	//		// calculate the time since the beginning of the day in the timezone
	//
	//		// TODO change this calculation to be based on the current time in time zone
	//		// timeSinceDayStart := time.Since(beginningOfDayTZ)
	//		timeSinceDayStart := now.New(time.Now().In(timeLocation)).Sub(beginningOfDayTZ)
	//
	//		// logger.Debugf("(premiumWeeklyFreeze) handling time zone %s time since day start: %v day start: %v current time in zone: %v", timeLocation.String(), timeSinceDayStart, beginningOfDayTZ, time.Now().In(timeLocation))
	//
	//		// if we are within 10 seconds of the start of the day append the time zone to the array
	//		if timeSinceDayStart > 0 && timeSinceDayStart < 5*time.Minute && beginningOfDayTZ.Weekday() == time.Monday {
	//
	//			if len(beginningOFDayZone) > 0 {
	//				postToList := true
	//				for _, tz := range beginningOFDayZone {
	//					if tz.(string) == now.New(time.Now().In(timeLocation)).BeginningOfDay().Add(-24*time.Hour).String() {
	//						postToList = false
	//					}
	//				}
	//
	//				if postToList {
	//					logger.Debugf("(premiumWeeklyFreeze) appending time zone %v with time: %v", timeLocation.String(), now.New(time.Now().In(timeLocation)).BeginningOfDay().Add(-24*time.Hour).String())
	//					beginningOFDayZone = append(beginningOFDayZone, now.New(time.Now().In(timeLocation)).BeginningOfDay().Add(-24*time.Hour).String())
	//					bParamSlots = append(bParamSlots, "?")
	//				}
	//
	//			} else {
	//				logger.Debugf("(premiumWeeklyFreeze) appending time zone %v with time: %v", timeLocation.String(), now.New(time.Now().In(timeLocation)).BeginningOfDay().Add(-24*time.Hour).String())
	//				beginningOFDayZone = append(beginningOFDayZone, now.New(time.Now().In(timeLocation)).BeginningOfDay().Add(-24*time.Hour).String())
	//				bParamSlots = append(bParamSlots, "?")
	//			}
	//
	//		}
	//	}
	//}
	//
	//// exit if there is no work to do
	//if len(beginningOFDayZone) == 0 {
	//	return nil
	//}

	//logger.Debugf("(premiumWeeklyFreeze) found %d timezones", len(beginningOFDayZone))

	query := "select _id, timezone from users where user_status = 1"

	res, err := db.QueryContext(ctx, &span, &callerName, query)
	if err != nil {
		logger.Error(fmt.Sprintf("(premiumWeeklyFreeze) failed to query premium users: %v\n    query: %s\n",
			err, query))
		return fmt.Errorf("failed to execute query: %v", err)
	}

	// defer closure of cursor
	defer res.Close()

	// iterate through workspaces loading ids
	for res.Next() {
		// create variable to hold workspace id
		var user models.User

		err = sqlstruct.Scan(&user, res)
		if err != nil {
			logger.Error(fmt.Sprintf("(premiumWeeklyFreeze) failed to scan user query results: %v\n",
				err))
			return fmt.Errorf("failed to scan query results: %v", err)
		}

		logger.Debugf("(premiumWeeklyFreeze) attempting to perform streak freeze update for user: %v", user.ID)

		keyName := "premium-streak-freeze-" + fmt.Sprintf("%d", user.ID)

		rediRes, err := rdb.Get(ctx, keyName).Result()
		if err == redis.Nil {
			location, err := time.LoadLocation(user.Timezone)
			if err != nil {
				logger.Error(fmt.Sprintf("(premiumWeeklyFreeze) failed to load timezone location: %v\n",
					err))
				return fmt.Errorf("failed to load timezone location: %v", err)
			}

			updateQuery := "update user_stats set streak_freezes = streak_freezes + 2 where user_id = ? order by date desc limit 1"

			_, err = db.ExecContext(ctx, &span, &callerName, updateQuery, user.ID)
			if err != nil {
				logger.Error(fmt.Sprintf("(premiumWeeklyFreeze) failed to execute query: %v\n", err))
				return fmt.Errorf("failed to execute query: %v", err)
			}

			//get the current time in that timezone
			currentTime := time.Now().In(location)

			//get the start of the following week in that timezone
			startTime := now.BeginningOfWeek().In(location).Add(time.Hour * 168)

			//subtract the times to get the duration until the start of the following week
			finalTime := startTime.Sub(currentTime)

			logger.Debugf("(premiumWeeklyFreeze) duration until key is updated: %v", finalTime)

			_ = rdb.Set(ctx, keyName, user.ID, finalTime)

			logger.Debugf("(premiumWeeklyFreeze) updated user_stats with new streak freezes for user: %v", user.ID)
		} else if err != nil {
			logger.Error(fmt.Sprintf("(premiumWeeklyFreeze) failed to get users redi key: %v\n",
				err))
			//return fmt.Errorf("failed to get users redi key: %v", err)
		} else {
			logger.Debugf("(premiumWeeklyFreeze) user had redi key: %v with key signature: %v", user.ID, rediRes)
			//return nil
		}
	}

	//// create query to retrieve all users that are active and just had a day change in their timezone
	//query := "UPDATE user_stats JOIN users ON user_stats.user_id = users._id SET user_stats.streak_freezes = user_stats.streak_freezes + 2 WHERE users.user_status = 1 and user_stats.date in (" + strings.Join(bParamSlots, ",") + ")"
	//
	//// query := "select us._id, us.user_id, us.challenge_complete us.streak_active, us.current_streak, us.longest_streak, us.longest_streak, us.total_time_spent, us.avg_time, us.days_on_platform, us.days_on_fire, us.date, us.streak_freezes, us.streak_freeze_used, us.xp_gained from user_stats us join users u on us.user_id = u._id where us.streak_active = false and u.timezone in (" + strings.Join(bParamSlots, ",") + ")"

	//logger.Debugf("(premiumWeeklyFreeze) query: %v params: %v", query, beginningOFDayZone)
	//// execute end of day query
	//_, err := db.ExecContext(ctx, &span, &callerName, query, beginningOFDayZone...)
	//if err != nil {
	//	return fmt.Errorf("failed to execute query: %v", err)
	//}

	logger.Debugf("(premiumWeeklyFreeze) successfully inserted new rows")
	return nil
}
