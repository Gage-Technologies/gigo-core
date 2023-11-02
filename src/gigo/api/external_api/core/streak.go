package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/jinzhu/now"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
	"time"
)

// CheckElapsedStreakTime checks the elapsed streak time of the given user
// returns
//   - bool, is user streak active today
//   - int, current streak for user
//   - map[string]bool, map for each day in string form to a bool for whether the streak was active that day
//   - *time.Duration, elapsed time since the start of workspace today
//   - error, if any
func CheckElapsedStreakTime(ctx context.Context, db *ti.Database, userID int64, userTimeZone string, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-elapsed-streak-time")
	defer span.End()
	callerName := "CheckElapsedStreakTime"

	logger.Debugf("CheckElapsedStreakTime: called for user %d", userID)
	locale, err := time.LoadLocation(userTimeZone)
	if err != nil {
		logger.Errorf("CheckElapsedStreakTime: failed to load time zone %s for user: %v, err: %v", userTimeZone, userID, err)
		return nil, fmt.Errorf("failed to load time zone for user: %v, err: %v", userID, err)
	}

	// check if the user is ephemeral
	var ephemeral bool
	err = db.QueryRowContext(ctx, &span, &callerName, "SELECT is_ephemeral FROM users WHERE _id = ?", userID).Scan(&ephemeral)
	if err != nil {
		logger.Errorf("CheckElapsedStreakTime: failed to query user for user: %v, err: %v", userID, err)
		return nil, fmt.Errorf("failed to find user for user: %v, err: %v", userID, err)
	}

	date := now.New(time.Now().In(locale)).BeginningOfDay()

	// return blank if user is ephemeral
	if ephemeral {
		return map[string]interface{}{
			"streak_active":       false,
			"streak_freeze_used":  false,
			"current_streak":      0,
			"longest_streak":      0,
			"current_day_of_week": date.Weekday().String(),
			"week_in_review": map[string]bool{
				"Monday":    false,
				"Tuesday":   false,
				"Wednesday": false,
				"Thursday":  false,
				"Friday":    false,
				"Saturday":  false,
				"Sunday":    false,
			},
			"elapsed_time": 0,
		}, nil
	}

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		logger.Errorf("CheckElapsedStreakTime: failed to create transaction for user: %v, err: %v", userID, err)
		return nil, fmt.Errorf("failed to start transaction for user: %v, err: %v", userID, err)
	}

	defer tx.Commit(&callerName)

	logger.Debugf("CheckElapsedStreakTime: loaded date for  %d", userID)

	res, err := tx.QueryContext(ctx, &callerName, "SELECT * FROM user_stats WHERE user_id =? order by date desc limit 1", userID)
	if err != nil {
		logger.Errorf("CheckElapsedStreakTime: failed to query user stats for user: %v, err: %v", userID, err)
		return nil, fmt.Errorf("failed to find user stats for user: %v, err: %v", userID, err)
	}

	if !res.Next() {
		return nil, fmt.Errorf("failed to find user stats for user: %v", userID)
	}

	userStatsModel, err := models.UserStatsFromSQLNative(db, res)
	if err != nil {
		logger.Errorf("CheckElapsedStreakTime: failed to convert user stats for user: %v, err: %v", userID, err)
		return nil, fmt.Errorf("failed to parse user stats for user: %v, err: %v", userID, err)
	}

	if userStatsModel == nil {
		logger.Errorf("CheckElapsedStreakTime: user stats not found for user: %v, err: incorrect data returned", userID)
		return nil, fmt.Errorf("failed to find user stats for user: %v, err: incorrect data returned", userID)
	}

	res.Close()

	logger.Debugf("CheckElapsedStreakTime: successfully retrieved user stats for user %d, date: %v", userID, date)

	var elapsedTime time.Duration = 0

	weekInReview := map[string]bool{
		"Monday":    false,
		"Tuesday":   false,
		"Wednesday": false,
		"Thursday":  false,
		"Friday":    false,
		"Saturday":  false,
		"Sunday":    false,
	}
	for _, usage := range userStatsModel.DailyIntervals {
		if usage.EndTime != nil {
			temp := usage.EndTime.In(locale).Sub(usage.StartTime.In(locale))
			logger.Debugf("complete usage in locale: %v userID: %v ", temp, userID)
			elapsedTime += temp
		}

		elapsedTime += time.Since(usage.StartTime.In(locale))
		logger.Debugf("usage in locale: %v userID: %v ", time.Since(usage.StartTime.In(locale)), userID)
	}

	logger.Debugf("CheckElapsedStreakTime: calculated elapsed daily time: %v for  %d, date: %v", elapsedTime, userID, date)

	todayOfWeek := date.Weekday()

	if userStatsModel.StreakActive {
		weekInReview[todayOfWeek.String()] = true
	}

	var currentDayOfWeek time.Weekday
	if todayOfWeek != time.Monday {
		previousDay := date.Add(-24 * time.Hour)
		for {
			streakIsActive := false
			currentDayOfWeek = previousDay.Weekday()
			logger.Debugf("CheckElapsedStreakTime: calculating streak week for %d current day: %v", userID, currentDayOfWeek)
			if currentDayOfWeek == time.Sunday {
				break
			}

			formattedPreviousDay := previousDay.Format("2006-01-02 15:04:05")

			res, err = tx.QueryContext(ctx, &callerName, "SELECT streak_active from user_stats where date = ? and user_id = ?", formattedPreviousDay, userID)
			if err != nil {
				logger.Errorf("CheckElapsedStreakTime: failed to query user stats for user: %v and date: %v, err: %v", userID, date, err)
				return nil, fmt.Errorf("failed to find user stats for user: %v for %v, err: %v", userID, currentDayOfWeek, err)
			}

			for res.Next() {
				err = res.Scan(&streakIsActive)
				if err != nil {
					logger.Errorf("CheckElapsedStreakTime: failed to scan user stats for user: %v, err: %v", userID, err)
					return nil, fmt.Errorf("failed to scan streak for user: %v for %v, err: %v", userID, currentDayOfWeek, err)
				}
			}

			res.Close()

			logger.Debugf("CheckElapsedStreakTime: query SELECT streak_active from user_stats where date = %v and user_id = %v", date, userID)
			logger.Debugf("CheckElapsedStreakTime: streak active for %d is: %v on %v", userID, streakIsActive, currentDayOfWeek)

			weekInReview[currentDayOfWeek.String()] = streakIsActive
			previousDay = previousDay.Add(time.Hour * -24)
		}

		logger.Debugf("CheckElapsedStreakTime: finished for %d, date: %v on non monday, week in review: %v", userID, date, weekInReview)
		return map[string]interface{}{"streak_active": userStatsModel.StreakActive, "streak_freeze_used": userStatsModel.StreakFreezeUsed, "current_streak": userStatsModel.CurrentStreak, "longest_streak": userStatsModel.LongestStreak, "current_day_of_week": todayOfWeek.String(), "week_in_review": weekInReview, "elapsed_time": elapsedTime}, nil
		// return userStatsModel.StreakActive, userStatsModel.CurrentStreak, weekInReview, elapsedTime, nil
	}

	weekInReview[todayOfWeek.String()] = userStatsModel.StreakActive
	logger.Debugf("CheckElapsedStreakTime: finished for %d, date: %v", userID, date)
	return map[string]interface{}{
		"streak_active":       userStatsModel.StreakActive,
		"streak_freeze_used":  userStatsModel.StreakFreezeUsed,
		"current_streak":      userStatsModel.CurrentStreak,
		"longest_streak":      userStatsModel.LongestStreak,
		"current_day_of_week": todayOfWeek.String(),
		"week_in_review":      weekInReview,
		"elapsed_time":        elapsedTime,
	}, nil
}

// UpdateStreak updates today's streak active variable to true
func UpdateStreak(ctx context.Context, db *ti.Database, userID int64, streakNum int, longestStreak int, userTimeZone string) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-streak-core")
	defer span.End()
	callerName := "UpdateStreak"

	locale, err := time.LoadLocation(userTimeZone)
	if err != nil {
		return fmt.Errorf("failed to load time zone for user: %v, err: %v", userID, err)
	}

	date := now.New(time.Now().In(locale)).BeginningOfDay()

	streakNum++

	if streakNum > longestStreak {
		longestStreak = streakNum
	}

	_, err = db.ExecContext(ctx, &span, &callerName, "UPDATE user_stats SET streak_active =?, current_streak = ?, longest_streak = ?  WHERE user_id =? AND date = ?", true, streakNum, longestStreak, userID, date)
	if err != nil {
		return fmt.Errorf("failed to update streak for user: %v, err: %v", userID, err)
	}

	return nil
}

func GetUserStreaks(ctx context.Context, db *ti.Database, callingUser *models.User, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-streaks-core")
	defer span.End()
	callerName := "GetUserStreaks"

	userID := callingUser.ID
	locale, err := time.LoadLocation(callingUser.Timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to set today's date for query: %v, err: %v", userID, err)
	}

	today := now.New(time.Now().In(locale)).BeginningOfDay()

	// query data from the user_stats table
	res, err := db.QueryContext(ctx, &span, &callerName, "SELECT * FROM user_stats WHERE user_id = ? AND date = ?", userID, today)
	if err != nil {
		logger.Errorf("GetUserStreaks: %v", err)
		return nil, fmt.Errorf("failed to query user stats for user: %v, err: %v", userID, err)
	}

	// create variable to store the results
	stats := make([]*models.UserStatsFrontend, 0)

	defer res.Close()

	// load into stats variable
	for res.Next() {
		var stat models.UserStats

		err = sqlstruct.Scan(&stat, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Streak core.    Error: %v", err)
			logger.Errorf("GetUserStreaks: %v", err)
		}

		stats = append(stats, stat.ToFrontend())
	}

	// query the user_daily_usage table to get all dates that had an active streak
	str, err := db.QueryContext(ctx, &span, &callerName, "SELECT date FROM user_stats WHERE streak_active = ? AND user_id = ?", true, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user streak dates for user: %v, err: %v", userID, err)
	}

	var streaks []string

	defer str.Close()

	for str.Next() {
		var streak string
		err = str.Scan(&streak)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Streak core.    Error: %v", err)
		}
		streaks = append(streaks, streak)
	}

	// query the user_daily_usage table to get all dates that had an active streak
	frez, err := db.QueryContext(ctx, &span, &callerName, "SELECT date FROM user_stats WHERE streak_active = ? AND streak_freeze_used = ? AND user_id = ?", false, true, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user streak dates for user: %v, err: %v", userID, err)
	}

	var freezes []string

	defer frez.Close()

	for frez.Next() {
		var freeze string
		err = frez.Scan(&freeze)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Streak core.    Error: %v", err)
		}
		freezes = append(freezes, freeze)
	}

	return map[string]interface{}{"stats": stats, "streaks": streaks, "freezes": freezes}, nil
}

func GetStreakFreezeCount(ctx context.Context, db *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-streak-freeze-count-core")
	defer span.End()
	callerName := "GetStreakFreezeCount"

	// execute end of day query
	res, err := db.QueryContext(ctx, &span, &callerName, "select streak_freezes from user_stats where user_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("Error query failed: %s", err.Error())
	}

	defer res.Close()

	var streakFreezes int

	for res.Next() {
		// scan row into user stats object
		err := res.Scan(&streakFreezes)
		if err != nil {
			return nil, fmt.Errorf("Error scan failed: %s", err.Error())
		}
	}

	return map[string]interface{}{"streak_freezes": streakFreezes}, nil
}
