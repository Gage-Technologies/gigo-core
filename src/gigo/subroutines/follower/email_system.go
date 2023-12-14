package follower

import (
	"context"
	"gigo-core/gigo/api/external_api/core"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"go.opentelemetry.io/otel"
	"time"
)

// UserInactivityEmailCheck
//
//	Check for all users that have been inactive and schedules the appropriate emails
func UserInactivityEmailCheck(ctx context.Context, tidb *ti.Database, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-inactivity-email-check-routine")
	defer span.End()
	callerName := "UserInactivityEmailCheck"

	logger.Infof("Starting UserInactivityEmailCheck")

	// Calculate the dates for one week and one month ago
	oneWeekAgo := time.Now().Add(-7 * 24 * time.Hour)
	oneMonthAgo := time.Now().Add(-30 * 24 * time.Hour)
	threeWeeksAgo := time.Now().Add(-21 * 24 * time.Hour)

	// Monthly inactivity check
	monthlyQuery := `UPDATE user_inactivity SET send_month = TRUE, notify_on = NOW() 
                     WHERE last_login < ? AND last_notified < ? AND send_month = FALSE AND send_week = FALSE`
	if _, err := tidb.ExecContext(ctx, &span, &callerName, monthlyQuery, oneMonthAgo, threeWeeksAgo); err != nil {
		logger.Errorf("failed to query for users inactive for a month: %v", err)
	}

	// Weekly inactivity check
	weeklyQuery := `UPDATE user_inactivity SET send_week = TRUE, notify_on = NOW() 
                    WHERE last_login < ? AND last_notified < ? AND send_week = FALSE AND send_month = FALSE`
	if _, err := tidb.ExecContext(ctx, &span, &callerName, weeklyQuery, oneWeekAgo, oneWeekAgo); err != nil {
		logger.Errorf("failed to query for users inactive for a week: %v", err)
	}

	logger.Infof("Finished UserInactivityEmailCheck")
}

func SendUserInactivityEmails(ctx context.Context, tidb *ti.Database, logger logging.Logger, mailGunKey string, mailGunDomain string) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "send-user-inactivity-emails")
	defer span.End()
	callerName := "SendUserInactivityEmails"

	logger.Infof("Starting SendUserInactivityEmails")

	// Query for users who need to be notified
	query := `SELECT user_id, email, send_week, send_month FROM user_inactivity 
              WHERE (send_week = TRUE OR send_month = TRUE) AND notify_on < NOW()`
	rows, err := tidb.QueryContext(ctx, &span, &callerName, query)
	if err != nil {
		logger.Errorf("failed to query for inactive users: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		var email string
		var sendWeek, sendMonth bool

		if err := rows.Scan(&userID, &email, &sendWeek, &sendMonth); err != nil {
			logger.Errorf("failed to scan row: %v", err)
		}

		// Send the appropriate email and update the record
		if sendMonth {
			logger.Infof("Sending month inactivity email to %s", email)
			if err := core.SendMonthInactiveMessage(ctx, tidb, mailGunKey, mailGunDomain, email); err != nil {
				logger.Errorf("failed to send month inactive email: %v user_id: %v", err, userID)
			}
		} else if sendWeek {
			logger.Infof("Sending week inactivity email to %s", email)
			if err := core.SendWeekInactiveMessage(ctx, tidb, mailGunKey, mailGunDomain, email); err != nil {
				logger.Errorf("failed to send week inactive email: %v user_id: %v", err, userID)
			}
		}

		// Update the user_inactivity record
		updateQuery := `UPDATE user_inactivity SET last_notified = NOW(), send_week = FALSE, send_month = FALSE WHERE user_id = ?`
		if _, err := tidb.ExecContext(ctx, &span, &callerName, updateQuery, userID); err != nil {
			logger.Errorf("failed to update user_inactivity record: %v user_id: %v", err, userID)
		}
	}

	if err = rows.Err(); err != nil {
		logger.Errorf("error iterating over rows: %v", err)
	}

	logger.Infof("Finished SendUserInactivityEmails")
}

// UpdateLastUsage
//
// Updates the last_login field in the user_inactivity table based on the most recent activity in web_tracking table for each user.
func UpdateLastUsage(ctx context.Context, tidb *ti.Database, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-last-usage-routine")
	defer span.End()
	callerName := "UpdateLastUsage"

	logger.Infof("Starting UpdateLastUsage")

	// Query to update last_login in user_inactivity table with the latest timestamp from web_tracking for each user
	updateQuery := `
	UPDATE user_inactivity ui 
	JOIN (
		SELECT user_id, MAX(timestamp) AS latest_timestamp 
		FROM web_tracking 
		GROUP BY user_id
	) wt ON ui.user_id = wt.user_id
	SET ui.last_login = wt.latest_timestamp
	WHERE ui.last_login < wt.latest_timestamp
	`

	// Execute the update query
	if _, err := tidb.ExecContext(ctx, &span, &callerName, updateQuery); err != nil {
		logger.Errorf("failed to update last_login in user_inactivity: %v", err)
		return
	}

	logger.Infof("Finished UpdateLastUsage")
}
