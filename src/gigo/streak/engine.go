package streak

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/lock"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/go-redis/redis/v8"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// StreakEngine
//
//	Management system for handling streak updates throughout
//	the Gigo system.
type StreakEngine struct {
	db        *ti.Database
	rdb       redis.UniversalClient
	snowFlake *snowflake.Node
	logger    logging.Logger
	lockMngr  *lock.RedLockManager
}

// NewStreakEngine
//
//	Creates a new streak engine
func NewStreakEngine(db *ti.Database, rdb redis.UniversalClient, snowflakeNode *snowflake.Node, logger logging.Logger) *StreakEngine {
	return &StreakEngine{
		db:        db,
		rdb:       rdb,
		snowFlake: snowflakeNode,
		logger:    logger,
		lockMngr:  lock.CreateRedLockManager(rdb),
	}
}

// InitializeFirstUserStats creates the first user's stats'
// args
//
//	userID - int64, the user's ID'
//	beginningDayUser - time.Time, the beginning of the day in the user's location
//
// returns
//
//	err - error, nil on success
func (s *StreakEngine) InitializeFirstUserStats(ctx context.Context, tx *ti.Tx, userID int64, beginningDayUser time.Time) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "initialize-first-user-stats-core")
	defer span.End()
	callerName := "InitializeFirstUserStats"

	s.logger.Debugf("starting initialize user: %v stats in streak engine", userID)

	// create empty user stats model
	userStatsModel, err := models.CreateUserStats(s.snowFlake.Generate().Int64(), userID, 0,
		false, 0, 0, time.Minute*0, time.Minute*0, 0,
		0, 0, beginningDayUser, beginningDayUser.Add(time.Hour*24), nil)
	if err != nil {
		return fmt.Errorf("failed to initialize first user stats: %w", err)
	}

	if userStatsModel == nil {
		return fmt.Errorf("failed to initialize first user stats: user stats model is nil")
	}

	// insert empty model
	statements := userStatsModel.ToSQLNative()
	for _, statement := range statements {
		if tx == nil {
			_, err = s.db.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
		} else {
			_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		}
		if err != nil {
			return fmt.Errorf("failed to insert user stats: %w", err)
		}
	}

	span.AddEvent(
		"initialize-first-user-stats",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("username", fmt.Sprintf("%v", userID)),
		),
	)

	s.logger.Debugf("successfully inserted first user stats in streak engine, for user: %v", userID)
	return nil
}

// GetUsersLastStatsDay gets yesterday's stats for a given user
// args
//
//		userID - int64, the user's ID'
//	    beginningDayUser - time.Time, the beginning of the day in the user's location'
//
// returns
//
//	err - error, nil on success
func (s *StreakEngine) GetUsersLastStatsDay(ctx context.Context, userID int64) (*models.UserStats, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-users-last-stats-day-core")
	defer span.End()
	callerName := "GetUsersLastStatsDay"

	s.logger.Debugf("starting get yesterday user: %v stats in streak engine", userID)
	// calculate the start of yesterday in the users location
	// beginningYesterday := beginningDayUser.AddDate(0, 0, -1)

	// query for yesterday's stats'
	res, err := s.db.QueryContext(ctx, &span, &callerName, "SELECT * FROM user_stats WHERE user_id = ? ORDER BY date DESC LIMIT 1", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get users yesterday stats from query: %w", err)
	}
	defer res.Close()

	fmt.Println("Query result: ", res.Err())

	// create new user stats object to load into
	userStatsSQL := new(models.UserStatsSQL)

	for res.Next() {
		// scan row into user stats object
		err = sqlstruct.Scan(userStatsSQL, res)
		if err != nil {
			return nil, fmt.Errorf("error scanning user stats in first scan: %w", err)
		}

		break
	}

	_ = res.Close()

	res, err = s.db.QueryContext(ctx, &span, &callerName, "SELECT start_time, end_time, open_session from user_daily_usage where user_id = ? and date = ?", userStatsSQL.UserID, userStatsSQL.Date)
	if err != nil {
		return nil, fmt.Errorf("failed to query for daily usage, err: %v  query: %v", err, fmt.Sprintf("SELECT start_time, end_time, open_session from user_daily_usage where user_id = %v and date = %s;", userStatsSQL.UserID, userStatsSQL.Date))
	}
	defer res.Close()

	if res.Err() != nil {
		return nil, fmt.Errorf("failed to query for daily usage, err: %v", res.Err())
	}

	dailyUses := make([]*models.DailyUsage, 0)

	for res.Next() {
		dailuse := new(models.DailyUsage)
		err := sqlstruct.Scan(dailuse, res)
		if err != nil {
			return nil, err
		}
		dailyUses = append(dailyUses, dailuse)
	}

	if len(dailyUses) == 0 {
		s.logger.Debugf("no daily usage found for user: %v in get last user day", userID)
		dailyUses = nil
	}

	// create new user stats
	userStats := &models.UserStats{
		ID:                  userStatsSQL.ID,
		UserID:              userStatsSQL.UserID,
		ChallengesCompleted: userStatsSQL.ChallengesCompleted,
		StreakActive:        userStatsSQL.StreakActive,
		CurrentStreak:       userStatsSQL.CurrentStreak,
		LongestStreak:       userStatsSQL.LongestStreak,
		TotalTimeSpent:      userStatsSQL.TotalTimeSpent,
		AvgTime:             userStatsSQL.AvgTime,
		DailyIntervals:      dailyUses,
		DaysOnPlatform:      userStatsSQL.DaysOnPlatform,
		DaysOnFire:          userStatsSQL.DaysOnFire,
		StreakFreezeUsed:    userStatsSQL.StreakFreezeUsed,
		StreakFreezes:       userStatsSQL.StreakFreezes,
		Date:                userStatsSQL.Date,
	}

	span.AddEvent(
		"get-users-last-stats-day",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	return userStats, nil
}

// UserStartWorkspace called when user starts a workspace
// args
//
//		userID - int64, the user's ID'
//	    beginningDayUser - time.Time, the beginning of the day in the user's location'
//
// returns
//
//	err - error, nil on success
func (s *StreakEngine) UserStartWorkspace(ctx context.Context, userID int64) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-start-workspace-core")
	defer span.End()
	callerName := "UserStartWorkspace"

	s.logger.Debugf("starting user: %v start workspace in streak engine", userID)

	// open tx for the operation
	tx, err := s.db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return fmt.Errorf("error opening tx: %w", err)
	}
	defer tx.Rollback()

	// get the user's timezone
	var userTz string
	err = tx.QueryRowContext(ctx, "SELECT timezone FROM users WHERE _id = ?", userID).Scan(&userTz)
	if err != nil {
		return fmt.Errorf("error getting user timezone: %v", err)
	}

	// load user timezone
	timeLocation, err := time.LoadLocation(userTz)
	if err != nil {
		return fmt.Errorf("error loading time zone: %v", err)
	}

	// query for today's stats'
	res, err := tx.QueryContext(ctx, &callerName, "SELECT * FROM user_stats WHERE user_id = ? ORDER BY date DESC LIMIT 1", userID)
	if err != nil {
		return fmt.Errorf("failed to get users today stats from query: %w", err)
	}
	defer res.Close()

	if !res.Next() {
		return fmt.Errorf("failed to find user stats for user: %v", userID)
	}

	// populate the user stats
	userStatsModel, err := models.UserStatsFromSQLNative(s.db, res)
	if err != nil {
		return fmt.Errorf("failed to get users today stats from sql native: %w", err)
	}

	if userStatsModel == nil {
		return fmt.Errorf("failed to get users today stats from sql native: %w", err)
	}

	b, _ := json.Marshal(userStatsModel)
	s.logger.Debugf("successfully retrieved daily usage rows for user %v: %s", userID, string(b))

	// if there are no intervals today or they have all been closed then create a new one
	createNewInterval := len(userStatsModel.DailyIntervals) == 0
	if !createNewInterval && userStatsModel.DailyIntervals != nil {
		createNewInterval = true
		for _, interval := range userStatsModel.DailyIntervals {
			if interval.EndTime == nil {
				createNewInterval = false
				break
			}
		}
	}

	res.Close()
	if createNewInterval {
		s.logger.Debugf("no open daily intervals found for user: %v for today", userID)
		// insert the new interval into the db
		_, err = tx.ExecContext(ctx, &callerName,
			"insert into user_daily_usage(user_id, start_time, end_time, open_session, date) values (?, ?, ?, ?, ?);",
			userID, time.Now().In(timeLocation), nil, 1, userStatsModel.Date,
		)
		if err != nil {
			return fmt.Errorf("failed to insert user daily interval: %w", err)
		}
	} else {
		// update daily usage interval for current interval
		_, err = tx.ExecContext(ctx, &callerName, "UPDATE user_daily_usage SET open_session = open_session + 1 WHERE user_id = ? and end_time is null",
			userID)
		if err != nil {
			return fmt.Errorf("failed to update user daily interval with new session: %w", err)
		}
	}

	// commit the tx
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("error committing tx: %w", err)
	}

	return nil

}

// UserStopWorkspace called when a user stops a workspace
// args
//
//		userID - int64, the user's ID'
//	    beginningDayUser - time.Time, the beginning of the day in the user's location'
//
// returns
//
//	err - error, nil on success
func (s *StreakEngine) UserStopWorkspace(ctx context.Context, userID int64) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-stop-workspace-core")
	defer span.End()
	callerName := "UserStopWorkspace"

	s.logger.Debugf("starting user: %v stop workspace in streak engine", userID)

	// open tx for the operation
	tx, err := s.db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return fmt.Errorf("error opening tx: %w", err)
	}
	defer tx.Rollback()

	// get the user's timezone
	var userTz string
	err = tx.QueryRowContext(ctx, "SELECT timezone FROM users WHERE _id = ?", userID).Scan(&userTz)
	if err != nil {
		return fmt.Errorf("error getting user timezone: %v", err)
	}

	rows, err := tx.QueryContext(ctx, &callerName, "SELECT * FROM user_stats WHERE user_id = ? ORDER BY date DESC LIMIT 1", userID)
	if err != nil {
		return fmt.Errorf("failed to get users today stats from query: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return fmt.Errorf("failed to find user stats for user: %v", userID)
	}

	userStatsModel, err := models.UserStatsFromSQLNative(s.db, rows)
	if err != nil {
		return fmt.Errorf("failed to get users today stats from sql native: %w", err)
	}

	if userStatsModel == nil {
		return fmt.Errorf("failed to get users today stats from sql native: %w", err)
	}

	rows.Close()

	err = s.UpdateTimeStats(ctx, tx, userID, userStatsModel.Date)
	if err != nil {
		return fmt.Errorf("failed to update user daily usage stats: %w", err)
	}

	_, err = tx.ExecContext(ctx, &callerName, "UPDATE user_daily_usage SET open_session = open_session - 1, end_time = if(open_session = 1, now(), null) WHERE user_id = ? and end_time is null",
		userID)
	if err != nil {
		return fmt.Errorf("failed to update user daily interval with new session: %w", err)
	}

	// commit the tx
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("error committing tx: %w", err)
	}

	s.logger.Debugf("successfully close interval for user: %v stop workspace in streak engine", userID)
	return nil

}

func (s *StreakEngine) UpdateTimeStats(ctx context.Context, tx *ti.Tx, userID int64, date time.Time) (err error) {
	callerName := "UpdateTimeStats"

	// create boolean to track if we failed incase
	// we need our own tx
	failed := true

	// create our own tx if one is not provided
	if tx == nil {
		ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-time-stats-core")
		defer span.End()
		tx, err = s.db.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			return fmt.Errorf("error opening tx: %w", err)
		}
		defer func() {
			if failed {
				_ = tx.Rollback()
			} else {
				err = tx.Commit(&callerName)
				if err != nil {
					_ = tx.Rollback()
					// update the error before we return
					err = fmt.Errorf("error committing tx: %w", err)
				}
			}
		}()
	}

	var totalMinutes *int64

	// Calculate total duration from today's sessions
	err = tx.QueryRowContext(ctx, "SELECT SUM(TIMESTAMPDIFF(MINUTE, start_time, now())) FROM user_daily_usage WHERE user_id = 1663938304510263296 AND end_time IS NULL AND open_session = 1").Scan(&totalMinutes)
	if err != nil {
		// we exit quietly if there are no rows
		if err == sql.ErrNoRows {
			return nil
		}
		err = fmt.Errorf("failed to calculate total duration: %w", err)
		return
	}

	// we exit quietly if there are no rows
	if totalMinutes == nil {
		failed = false
		return
	}

	// Get days_on_platform and update stats
	res, err := tx.ExecContext(ctx, &callerName, "UPDATE user_stats SET total_time_spent = total_time_spent + ?, avg_time = (total_time_spent + ?) / days_on_platform WHERE user_id = ? AND date = ?", *totalMinutes, *totalMinutes, userID, date)
	if err != nil {
		err = fmt.Errorf("error updating user stats: %v", err)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err == nil && rowsAffected == 0 {
		err = fmt.Errorf("failed to find user stats for user: %v", userID)
		return
	}

	s.logger.Debugf("time stats updated for user: %v, added time spent today: %v", userID, *totalMinutes)

	// mark operation as successful
	failed = false

	return
}
