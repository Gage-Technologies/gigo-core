package leader

import (
	"context"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	ti "github.com/gage-technologies/gigo-lib/db"
	"go.opentelemetry.io/otel"
	"time"
)

// UserInactivityEmailCheck
//
//	Check for all users that have been inactive and schedules the appropriate emails
func UserInactivityEmailCheck(ctx context.Context, tidb *ti.Database) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-inactivity-email-check-routine")
	defer span.End()
	callerName := "UserInactivityEmailCheck"

	// Calculate the dates for one week and one month ago
	oneWeekAgo := time.Now().Add(-7 * 24 * time.Hour)
	oneMonthAgo := time.Now().Add(-30 * 24 * time.Hour)
	threeWeeksAgo := time.Now().Add(-21 * 24 * time.Hour)

	// Monthly inactivity check
	monthlyQuery := `UPDATE user_inactivity SET send_month = TRUE, notify_on = NOW() 
                     WHERE last_login < ? AND last_notified < ? AND send_month = FALSE`
	if _, err := tidb.ExecContext(ctx, &span, &callerName, monthlyQuery, oneMonthAgo, threeWeeksAgo); err != nil {
		return fmt.Errorf("failed to query for users inactive for a month: %v", err)
	}

	// Weekly inactivity check
	weeklyQuery := `UPDATE user_inactivity SET send_week = TRUE, notify_on = NOW() 
                    WHERE last_login < ? AND last_notified < ? AND send_week = FALSE AND send_month = FALSE`
	if _, err := tidb.ExecContext(ctx, &span, &callerName, weeklyQuery, oneWeekAgo, oneWeekAgo); err != nil {
		return fmt.Errorf("failed to query for users inactive for a week: %v", err)
	}

	return nil
}

func SendUserInactivityEmails(ctx context.Context, tidb *ti.Database, mailGunKey string, mailGunDomain string) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "send-user-inactivity-emails")
	defer span.End()
	callerName := "SendUserInactivityEmails"

	// Query for users who need to be notified
	query := `SELECT user_id, email, send_week, send_month FROM user_inactivity 
              WHERE (send_week = TRUE OR send_month = TRUE) AND notify_on < NOW()`
	rows, err := tidb.QueryContext(ctx, &span, &callerName, query)
	if err != nil {
		return fmt.Errorf("failed to query for inactive users: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		var email string
		var sendWeek, sendMonth bool

		if err := rows.Scan(&userID, &email, &sendWeek, &sendMonth); err != nil {
			return fmt.Errorf("failed to scan row: %v", err)
		}

		// Send the appropriate email and update the record
		if sendMonth {
			if err := core.SendMonthInactiveMessage(ctx, tidb, mailGunKey, mailGunDomain, email); err != nil {
				return fmt.Errorf("failed to send month inactive email: %v", err)
			}
		} else if sendWeek {
			if err := core.SendWeekInactiveMessage(ctx, tidb, mailGunKey, mailGunDomain, email); err != nil {
				return fmt.Errorf("failed to send week inactive email: %v", err)
			}
		}

		// Update the user_inactivity record
		updateQuery := `UPDATE user_inactivity SET last_notified = NOW(), send_week = FALSE, send_month = FALSE WHERE user_id = ?`
		if _, err := tidb.ExecContext(ctx, &span, &callerName, updateQuery, userID); err != nil {
			return fmt.Errorf("failed to update user_inactivity record: %v", err)
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating over rows: %v", err)
	}

	return nil
}
