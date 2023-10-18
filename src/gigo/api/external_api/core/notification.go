package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
	"time"
)

func CreateNotification(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, sf *snowflake.Node, userId int64, message string, notificationType models.NotificationType, interactingUserId *int64) (*models.NotificationFrontend, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-notification-core")
	callerName := "CreateNotification"

	// ensure message was passed
	if message == "" {
		return nil, fmt.Errorf("CreateNotification core. Message cannot be empty")
	}

	// create new notification model
	notification, err := models.CreateNotification(sf.Generate().Int64(), userId, message, notificationType, time.Now(), false, interactingUserId)
	if err != nil {
		return nil, fmt.Errorf("CreateNotification core. Failed to create broadcast : %v", err)
	}

	// create notification insertion statement
	stmt := notification.ToSQLNative()

	// create notification js message
	notificationMessage := models2.BroadcastNotification{Notification: "Notification Initiated"}

	// format broadcast type and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(notificationMessage)
	if err != nil {
		return nil, fmt.Errorf("CreateNotification core. Failed to encode notificationMessage: %v", err)
	}

	// send broadcast init message to jet stream client
	_, err = js.PublishAsync(fmt.Sprintf(streams.SubjectBroadcastNotificationDynamic, userId), buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to send notification CreateNotification Core: %v", err)
	}

	// execute insert for notification
	_, err = tidb.ExecContext(ctx, &span, &callerName, stmt.Statement, stmt.Values...)
	if err != nil {
		return nil, fmt.Errorf("CreateNotification core. Failed to insert new notification : %v", err)
	}

	// event to frontend
	frontendNotification := notification.ToFrontend()

	return frontendNotification, nil
}

func AcknowledgeNotification(ctx context.Context, tidb *ti.Database, notificationId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "acknowledge-notification-core")
	callerName := "AcknowledgeNotification"

	// execute delete for notification
	_, err := tidb.ExecContext(ctx, &span, &callerName, "DELETE FROM notification WHERE _id = ?", notificationId)
	if err != nil {
		return nil, fmt.Errorf("AcknowledgeNotification core. Failed to delete notification : %v", err)
	}

	return map[string]interface{}{"message": "Notification acknowledged"}, nil
}

func AcknowledgeUserNotificationGroup(ctx context.Context, tidb *ti.Database, callingUser *models.User, notificationType int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "acknowledge-user-notification-group-core")
	callerName := "AcknowledgeUserNotificationGroup"

	// ensure calling user is not nil
	if callingUser == nil {
		return nil, fmt.Errorf("AcknowledgeUserNotificationGroup core. Calling user was nil")
	}

	// execute delete for all user notifications of the specified type
	_, err := tidb.ExecContext(ctx, &span, &callerName, "DELETE FROM notification WHERE user_id = ? and notification_type = ?", callingUser.ID, notificationType)
	if err != nil {
		return nil, fmt.Errorf("AcknowledgeNotification core. Failed to delete notification : %v", err)
	}

	return map[string]interface{}{"message": "Notification acknowledged"}, nil
}

func ClearUserNotifications(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "clear-user-notification-core")
	callerName := "ClearUserNotifications"

	// ensure calling user is not nil
	if callingUser == nil {
		return nil, fmt.Errorf("ClearUserNotifications core. Calling user was nil")
	}

	// execute delete for all user notifications of the specified type
	_, err := tidb.ExecContext(ctx, &span, &callerName, "DELETE FROM notification WHERE user_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("AcknowledgeNotification core. Failed to delete notification : %v", err)
	}

	return map[string]interface{}{"message": "Notifications cleared"}, nil
}

func GetUserNotifications(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-notification-core")
	callerName := "GetUserNotification"

	// query notification table to collect unacknowledged notifications
	res, err := tidb.QueryContext(ctx, &span, &callerName, "SELECT * FROM notification WHERE user_id = ? and acknowledged = FALSE ORDER BY created_at DESC", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("GetUserNotifications core. Failed to query notification table: %v", err)
	}

	defer res.Close()

	// make a map to store the frontend notifications
	notifications := make([]models.NotificationFrontend, 0)

	for res.Next() {
		var notification models.Notification

		err = sqlstruct.Scan(&notification, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for results. GetUserNotifications core.    Error: %v", err)
		}

		// notification to frontend
		frontendNotification := notification.ToFrontend()

		// append to slice if not empty
		if frontendNotification != nil {
			notifications = append(notifications, *frontendNotification)
		}
	}

	return map[string]interface{}{"notifications": notifications}, nil
}
