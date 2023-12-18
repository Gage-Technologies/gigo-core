package follower

import (
	"bytes"
	"context"
	"encoding/gob"
	"gigo-core/gigo/api/external_api/core"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"
)

func asyncSendWeekInactivityEmail(nodeId int64, ctx context.Context, db *ti.Database, logger logging.Logger, mgKey string, mgDomain string,
	msg *nats.Msg) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "async-send-week-inactivity-email")
	defer parentSpan.End()

	// Unmarshall the month inactivity request message
	var inactivityRequest models2.NewWeekInactivityMsg

	// Decode the message
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&inactivityRequest)
	if err != nil {
		logger.Errorf("(async-send-week-inactivity-email: %d) failed to decode week inactivity message: %v", nodeId, err)
		return
	}

	// send the inactivity message to the Recipient passed through the stream
	err = core.SendWeekInactiveMessage(ctx, db, mgKey, mgDomain, inactivityRequest.Recipient)
	if err != nil {
		logger.Errorf("(async-send-week-inactivity-email: %d) failed to send weekly inactivity message: %v", nodeId, err)
	}

	parentSpan.AddEvent(
		"async-send-week-inactivity-email",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}

func asyncSendMonthInactivityEmail(nodeId int64, ctx context.Context, db *ti.Database, logger logging.Logger, mgKey string, mgDomain string,
	msg *nats.Msg) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "async-send-month-inactivity-email")
	defer parentSpan.End()

	// Unmarshall the month inactivity request message
	var inactivityRequest models2.NewMonthInactivityMsg

	// Decode the message
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&inactivityRequest)
	if err != nil {
		logger.Errorf("(async-send-month-inactivity-email: %d) failed to decode month inactivity message: %v", nodeId, err)
		return
	}

	// send the inactivity message to the Recipient passed through the stream
	err = core.SendMonthInactiveMessage(ctx, db, mgKey, mgDomain, inactivityRequest.Recipient)
	if err != nil {
		logger.Errorf("(async-send-month-inactivity-email: %d) failed to send month inactivity message: %v", nodeId, err)
	}

	parentSpan.AddEvent(
		"async-send-month-inactivity-email",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}

// UserInactivityEmailCheck
//
//	Check for all users that have been inactive and schedules the appropriate emails
func UserInactivityEmailCheck(ctx context.Context, nodeId int64, tidb *ti.Database, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-inactivity-email-check-follower")
	defer span.End()
	callerName := "UserInactivityEmailCheckFollower"

	// Calculate the dates for one week and one month ago
	oneWeekAgo := time.Now().Add(-7 * 24 * time.Hour)
	oneMonthAgo := time.Now().Add(-30 * 24 * time.Hour)
	threeWeeksAgo := time.Now().Add(-21 * 24 * time.Hour)

	// Monthly inactivity check
	monthlyQuery := `UPDATE user_inactivity SET send_month = TRUE, notify_on = NOW() 
                     WHERE last_login < ? AND last_notified < ? AND send_month = FALSE AND send_week = FALSE`
	if _, err := tidb.ExecContext(ctx, &span, &callerName, monthlyQuery, oneMonthAgo, threeWeeksAgo); err != nil {
		logger.Errorf("(stream-inactivity-email-requests-follower: %d) failed to query for users inactive for a month: %v", nodeId, err)
	}

	// Weekly inactivity check
	weeklyQuery := `UPDATE user_inactivity SET send_week = TRUE, notify_on = NOW() 
                    WHERE last_login < ? AND last_notified < ? AND send_week = FALSE AND send_month = FALSE`
	if _, err := tidb.ExecContext(ctx, &span, &callerName, weeklyQuery, oneWeekAgo, oneWeekAgo); err != nil {
		logger.Errorf("(stream-inactivity-email-requests-follower: %d) failed to query for users inactive for a week: %v", nodeId, err)
	}
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

func EmailSystemManagement(ctx context.Context, nodeId int64, tidb *ti.Database,
	js *mq.JetstreamClient, workerPool *pool.Pool, logger logging.Logger, mgKey string, mgDomain string) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "workspace-management-operations-routine")
	defer parentSpan.End()

	// all errors related to jetstream operations should be logged
	// and them trigger an exit since it most likely is a network
	// issue that will persist and should be addressed by devops

	// process week inactivity email stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamEmail,
		streams.SubjectEmailUserInactiveWeek,
		"gigo-core-email-inactivity-week",
		time.Minute*10,
		"email",
		logger,
		func(msg *nats.Msg) {
			asyncSendWeekInactivityEmail(nodeId, ctx, tidb, logger, mgKey, mgDomain, msg)
		},
	)

	// process month inactivity email stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamEmail,
		streams.SubjectEmailUserInactiveMonth,
		"gigo-core-email-inactivity-week",
		time.Minute*10,
		"email",
		logger,
		func(msg *nats.Msg) {
			asyncSendMonthInactivityEmail(nodeId, ctx, tidb, logger, mgKey, mgDomain, msg)
		},
	)

	parentSpan.AddEvent(
		"workspace-management-operations-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}
