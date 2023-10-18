package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/kisielk/sqlstruct"
)

func BroadcastMessage(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, callingUser *models.User, message string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "broadcast-message-core")
	defer span.End()

	callerName := "BroadcastMessage"

	// ensure calling user was passed
	if callingUser == nil {
		return nil, fmt.Errorf("BroadcastMessage core. calling user nil")
	}

	// ensure message was passed
	if message == "" {
		return nil, fmt.Errorf("BroadcastMessage core. Message cannot be empty")
	}

	// create new BroadcastEvent model
	event, err := models.CreateBroadcastEvent(sf.Generate().Int64(), callingUser.ID, callingUser.UserName, message, 0, time.Now())
	if err != nil {
		return nil, fmt.Errorf("BroadcastMessage core. Failed to create broadcast event: %v", err)
	}

	// create broadcast insertion statement
	stmt := event.ToSQLNative()

	// execute insert for broadcast event
	_, err = tidb.ExecContext(ctx, &span, &callerName, stmt.Statement, stmt.Values...)
	if err != nil {
		return nil, fmt.Errorf("BroadcastMessage core. Failed to insert new broadcast event: %v", err)
	}

	// event to frontend
	frontendEvent := event.ToFrontend()

	return map[string]interface{}{"broadcast_message": frontendEvent}, nil
}

func GetBroadcastMessages(ctx context.Context, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-broadcast-messages-core")
	defer span.End()

	callerName := "GetBroadcastMessages"

	// query broadcast_event table to collect the last 100 messages
	res, err := tidb.QueryContext(ctx, &span, &callerName, "SELECT * FROM broadcast_event WHERE broadcast_type = 0 ORDER BY time_posted DESC LIMIT 100")
	if err != nil {
		return nil, fmt.Errorf("GetBroadcastMessages core. Failed to query broadcast_event table: %v", err)
	}

	defer res.Close()

	// make a map to store the frontend events
	events := make([]models.BroadcastEventFrontend, 0)

	for res.Next() {
		var event models.BroadcastEvent

		err = sqlstruct.Scan(&event, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for results. GetBroadcastMessages core.    Error: %v", err)
		}

		// event to frontend
		frontendEvent := event.ToFrontend()

		// append to slice if not empty
		if frontendEvent != nil {
			events = append(events, *frontendEvent)
		}
	}

	return map[string]interface{}{"broadcast_messages": events}, nil
}

// AwardBroadcastCheck
//
// Designed to be called from AddXP function and calculate whether a user should be awarded a broadcast message.
func AwardBroadcastCheck(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient, callingId int64, xpAwarded uint64, renownIncrease bool, levelIncrease bool) error {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "award-broadcast-check-core")
	callerName := "AwardBroadcastCheck"

	initmsg := models2.BroadcastMessage{InitMessage: "Broadcast Initiated"}

	// Redis key for user
	redisKey := fmt.Sprintf("user:%d:last_broadcast", callingId)

	// Retrieve the last broadcast time from Redis
	lastBroadcastTime, err := rdb.Get(ctx, redisKey).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("AwardBroadcastCheck core. Failed to get last broadcast time: %v", err)
	}

	// If less than 1 hour has passed since the last broadcast, return
	if time.Now().Unix()-lastBroadcastTime < 3600 {
		return nil
	}

	if renownIncrease == true {
		_, err := tidb.ExecContext(ctx, &span, &callerName, "Update users SET has_broadcast = true Where _id = ?", callingId)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to update user: %v", err)
		}

		// format broadcast type and marshall it with gob
		buf := bytes.NewBuffer(nil)
		encoder := gob.NewEncoder(buf)
		err = encoder.Encode(initmsg)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to encode broadcast type: %v", err)
		}

		// send broadcast init message to jet stream client
		_, err = js.PublishAsync(fmt.Sprintf(streams.SubjectBroadcastMessageDynamic, callingId), buf.Bytes())
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to send broadcast init message to jetstream: %v", err)
		}

		// set user BroadCastThreshold to 0
		_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set broadcast_threshold = 0 where _id = ?", callingId)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to reset users BroadcastThreshold: %v", err)
		}

		// Store the current time as the last broadcast time
		err = rdb.Set(ctx, redisKey, time.Now().Unix(), time.Hour).Err()
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to set last broadcast time: %v", err)
		}
		return nil
	} else if levelIncrease == true {
		_, err := tidb.ExecContext(ctx, &span, &callerName, "Update users SET has_broadcast = true Where _id = ?", callingId)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to update user: %v", err)
		}

		// format broadcast type and marshall it with gob
		buf := bytes.NewBuffer(nil)
		encoder := gob.NewEncoder(buf)
		err = encoder.Encode(initmsg)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to encode broadcast type: %v", err)
		}

		// send broadcast init message to jet stream client
		_, err = js.PublishAsync(fmt.Sprintf(streams.SubjectBroadcastMessageDynamic, callingId), buf.Bytes())
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to send broadcast init message to jetstream: %v", err)
		}

		// set user BroadCastThreshold to 0
		_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set broadcast_threshold = 0 where _id = ?", callingId)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to reset users BroadcastThreshold: %v", err)
		}

		// Store the current time as the last broadcast time
		err = rdb.Set(ctx, redisKey, time.Now().Unix(), time.Hour).Err()
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to set last broadcast time: %v", err)
		}
		return nil
	} else {
		// declare variable to hold broadcast threshold
		var thresh uint64

		// query to see if a user crossed the broadcast threshold after xp award
		err := tidb.QueryRowContext(ctx, &span, &callerName, "SELECT broadcast_threshold FROM users WHERE _id = ?", callingId).Scan(&thresh)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to query users broadcast threshold: %v", err)
		}

		// steam broadcast init message if threshold is reached
		if thresh+xpAwarded >= 750 {
			_, err := tidb.ExecContext(ctx, &span, &callerName, "Update users SET has_broadcast = true Where _id = ?", callingId)
			if err != nil {
				return fmt.Errorf("AwardBroadcastCheck core. Failed to update user: %v", err)
			}

			// format broadcast type and marshall it with gob
			buf := bytes.NewBuffer(nil)
			encoder := gob.NewEncoder(buf)
			err = encoder.Encode(initmsg)
			if err != nil {
				return fmt.Errorf("AwardBroadcastCheck core. Failed to encode broadcast type: %v", err)
			}

			// send broadcast init message to jet stream client
			_, err = js.PublishAsync(fmt.Sprintf(streams.SubjectBroadcastMessageDynamic, callingId), buf.Bytes())
			if err != nil {
				return fmt.Errorf("AwardBroadcastCheck core. Failed to send broadcast init message to jetstream: %v", err)
			}

			// set user BroadCastThreshold to 0
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set broadcast_threshold = 0 where _id = ?", callingId)
			if err != nil {
				return fmt.Errorf("AwardBroadcastCheck core. Failed to reset users BroadcastThreshold: %v", err)
			}

			// Store the current time as the last broadcast time
			err = rdb.Set(ctx, redisKey, time.Now().Unix(), time.Hour).Err()
			if err != nil {
				return fmt.Errorf("AwardBroadcastCheck core. Failed to set last broadcast time: %v", err)
			}
			return nil
		}

		// calculate new user threshhold
		newThresh := thresh + xpAwarded

		// set user BroadCastThreshold to newThresh
		_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set broadcast_threshold = ? where _id = ?", newThresh, callingId)
		if err != nil {
			return fmt.Errorf("AwardBroadcastCheck core. Failed to set users BroadcastThreshold: %v", err)
		}
		return nil
	}
}

func CheckBroadcastAward(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-broadcast-award-core")
	callerName := "CheckBroadcastAward"

	// ensure calling user is not nil
	if callingUser == nil {
		return nil, fmt.Errorf("CheckBroadcastAward core. calling user nil")
	}

	// revert hasBroadcast to false
	res, err := tidb.QueryContext(ctx, &span, &callerName, "Select has_broadcast From users Where _id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("CheckBroadcastAward core. Failed to update has_broadcast: %v", err)
	}

	// hold query result
	var hasBroadcast *bool

	// attempt to scan query result
	for res.Next() {
		err = res.Scan(&hasBroadcast)
		if err != nil {
			return nil, fmt.Errorf("CheckBroadcastAward core. Failed to scan has_broadcast: %v", err)
		}
	}
	if hasBroadcast != nil && *hasBroadcast == true {
		return map[string]interface{}{"message": "Has Broadcast"}, nil
	} else {
		return map[string]interface{}{"message": "No Broadcast"}, nil
	}
}

func RevertBroadcastAward(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "revert-broadcast-award-core")
	callerName := "RevertBroadcastAward"

	if callingUser == nil {
		return nil, fmt.Errorf("RevertBroadcastAward core. calling user nil")
	}

	// revert hasBroadcast to false
	_, err := tidb.ExecContext(ctx, &span, &callerName, "Update users SET has_broadcast = false Where _id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("RevertBroadcastAward core. Failed to update has_broadcast: %v", err)
	}

	return map[string]interface{}{"message": "Revert Successful"}, nil
}
