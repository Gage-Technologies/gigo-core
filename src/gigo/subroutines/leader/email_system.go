package leader

import (
	"bytes"
	"context"
	"encoding/gob"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"go.opentelemetry.io/otel"
)

func StreamInactivityEmailRequests(nodeId int64, ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "stream-inactivity-email-requests-leader")
	defer span.End()
	callerName := "StreamInactivityEmailRequests"

	// Query for users who need to be notified
	query := `SELECT user_id, email, send_week, send_month FROM user_inactivity 
              WHERE (send_week = TRUE OR send_month = TRUE) AND notify_on < NOW()`
	rows, err := tidb.QueryContext(ctx, &span, &callerName, query)
	if err != nil {
		logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to query for inactive users: %v", nodeId, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		var email string
		var sendWeek, sendMonth bool

		if err = rows.Scan(&userID, &email, &sendWeek, &sendMonth); err != nil {
			logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to scan row: %v", nodeId, err)
			continue
		}

		// Publish the appropriate email request to Jetstream
		if sendMonth {
			msg := models2.NewMonthInactivityMsg{Recipient: email}
			data, err := serializeMessage(msg)
			if err != nil {
				logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to serialize month inactivity message: %v", nodeId, err)
				continue
			}
			_, err = js.Publish(streams.SubjectEmailUserInactiveMonth, data)
			if err != nil {
				logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to publish month inactivity email request: %v", nodeId, err)
				continue
			}

		} else if sendWeek {
			msg := models2.NewWeekInactivityMsg{Recipient: email}
			data, err := serializeMessage(msg)
			if err != nil {
				logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to serialize week inactivity message: %v", nodeId, err)
				continue
			}
			_, err = js.Publish(streams.SubjectEmailUserInactiveWeek, data)
			if err != nil {
				logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to publish week inactivity email request: %v", nodeId, err)
				continue
			}
		}

		// Update the user_inactivity record
		updateQuery := `UPDATE user_inactivity SET last_notified = NOW(), send_week = FALSE, send_month = FALSE WHERE user_id = ?`
		if _, err := tidb.ExecContext(ctx, &span, &callerName, updateQuery, userID); err != nil {
			logger.Errorf("(stream-inactivity-email-requests-leader: %d) failed to update user_inactivity record: %v user_id: %v", nodeId, err, userID)
		}
	}

	if err = rows.Err(); err != nil {
		logger.Errorf("(stream-inactivity-email-requests-leader: %d) error iterating over rows: %v", nodeId, err)
	}

	logger.Infof("(stream-inactivity-email-requests-leader: %d) Finished StreamInactivityEmailRequests", nodeId)
}

func serializeMessage(msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(msg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
