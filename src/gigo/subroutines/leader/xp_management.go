package leader

import (
	"bytes"
	"context"
	"encoding/gob"
	"go.opentelemetry.io/otel"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
)

func addStreakXP(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "add-streak-xp-routine")
	defer span.End()
	callerName := "addStreakXP"
	// query for user's streaks that have reached the XP threshold of every 10 days
	stk, err := tidb.QueryContext(ctx, &span, &callerName, "select us._id, us.user_id from user_stats us left join stats_xp sx on us._id = sx.stats_id where us.current_streak % 10 = 0 and sx.expiration is null")
	if err != nil {
		logger.Errorf("(streak xp leader: %d) failed to query for streaks: %v", nodeId, err)
		return
	}

	// defer closure of cursor
	defer stk.Close()

	// create slice for streak messages
	streakMsgs := make([]models2.AddStreakXPMsg, 0)

	// load values from cursor
	for stk.Next() {
		// create empty variable to hold ids
		var stkId int64
		var ownerId int64

		err := stk.Scan(&stkId, &ownerId)
		if err != nil {
			logger.Errorf("(streak xp leader: %d) failed to scan cursor data for streaks: %v", nodeId, err)
			return
		}

		// add to outer messages slice to be published
		streakMsgs = append(streakMsgs, models2.AddStreakXPMsg{
			ID:      stkId,
			OwnerID: ownerId,
		})
	}

	// exit if there is nothing to do
	for len(streakMsgs) == 0 {
		return
	}

	// create base insert statement for user stat xp insertions
	insertStatement := "insert ignore into stats_xp(stats_id, expiration) values "
	params := make([]interface{}, 0)

	// create a slice to hold ids for streaks that have reached the xp threshold
	for _, msg := range streakMsgs {
		// encode streak message using gob
		buffer := bytes.NewBuffer(nil)
		encoder := gob.NewEncoder(buffer)
		err = encoder.Encode(msg)
		if err != nil {
			logger.Errorf("(streak xp leader: %d) failed to encode streak message: %v", nodeId, err)
			return
		}

		logger.Infof("(leader: %d) publishing add streak xp message: %s", nodeId, msg.ID)

		// publish add streak xp message so that a follower can begin process of adding xp
		_, err = js.PublishAsync(streams.SubjectStreakAddXP, buffer.Bytes())
		if err != nil {
			logger.Errorf("(streak xp leader: %d) failed to publish add streak xp message: %v", nodeId, err)
			return
		}

		// add values to insert statement and params slice
		if len(params) > 0 {
			insertStatement += ", "
		}
		insertStatement += "(?, ?)"
		// set the expiration 24 hours ahead since that is the earliest that we can perform another
		// streak based xp grant
		params = append(params, msg.ID, time.Now().Add(time.Hour*24))
	}

	// perform insertion of user stat xp models
	_, err = tidb.ExecContext(ctx, &span, &callerName, insertStatement, params...)
	if err != nil {
		logger.Errorf(
			"(streak xp leader: %d) failed to insert user stat xp: %v\n    query: %s\n    params: %v",
			nodeId, err, insertStatement, params,
		)
		return
	}

	// close cursor
	_ = stk.Close()
}

func XpManagementOperations(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, logger logging.Logger) {
	addStreakXP(ctx, nodeId, tidb, js, logger)
}
