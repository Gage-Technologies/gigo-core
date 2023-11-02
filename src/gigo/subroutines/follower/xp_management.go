package follower

import (
	"bytes"
	"context"
	"encoding/gob"
	"time"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/gigo/api/external_api/core"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
)

func asyncAddStreakXP(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient, sf *snowflake.Node, msg *nats.Msg, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "async-add-streak-xp-routine")
	defer span.End()

	// unmarshall add streak xp request message
	var addStreakMsg models2.AddStreakXPMsg
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&addStreakMsg)
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to decode add streak xp message: %v", nodeId, err)
	}

	logger.Debugf("(streak follower: %d) adding xp to user %d", nodeId, addStreakMsg.ID)

	// execute AddXP
	_, err = core.AddXP(ctx, tidb, js, rdb, sf, addStreakMsg.OwnerID, "streak", nil, nil, logger, nil)
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to add xp to user %d: %v", nodeId, addStreakMsg.ID, err)
	}

	// acknowledge that the xp has now been added to the user
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to ack add xp message: %v", nodeId, err)
	}
}

func XpManagementOperations(ctx context.Context, nodeId int64, tidb *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, rdb redis.UniversalClient, workerPool *pool.Pool, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "xp-management-operations-routine")
	defer parentSpan.End()

	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamStreakXP,
		streams.SubjectStreakAddXP,
		"gigo-core-follower-streak-add-xp",
		time.Second*30,
		"streak follower",
		logger,
		func(msg *nats.Msg) {
			asyncAddStreakXP(ctx, nodeId, tidb, js, rdb, sf, msg, logger)
		},
	)

	parentSpan.AddEvent(
		"xp-management-operations-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}

func RemoveExpiredStreakIds(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "remove-expired-streak-ids-routine")
	defer parentSpan.End()
	callerName := "RemoveExpiredStreakIds"

	_, err := js.ConsumerInfo(streams.SubjectStreakExpiration, "gigo-core-follower-streak-expiration")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamStreakXP, &nats.ConsumerConfig{
			Durable:       "gigo-core-follower-streak-expiration",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectStreakExpiration,
		})
		if err != nil {
			logger.Errorf("(streak follower: %d) failed to add streak expiration consumer: %v", nodeId, err)
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectStreakExpiration, "gigo-core-follower-streak-expiration", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to create streak expiration subscription: %v", nodeId, err)
		return
	}
	defer sub.Unsubscribe()

	msg, err := getNextJob(sub, time.Millisecond*50)
	if err != nil {
		if err == context.DeadlineExceeded {
			return
		}
		logger.Errorf("(streak follower: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}

	logger.Debugf("(streak follower: %d) removing expired streak ids", nodeId)

	res, err := tidb.ExecContext(ctx, &parentSpan, &callerName, "delete from stats_xp where expiration < ?", time.Now())
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to delete expired streak ids: %v", nodeId, err)
		return
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to check deleted count when removing expired streak ids: %v", nodeId, err)
		return
	}

	err = msg.Ack()
	if err != nil {
		logger.Errorf("(streak follower: %d) failed to ack message: %v", nodeId, err)
	}

	parentSpan.AddEvent(
		"remove-expired-streak-ids-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	logger.Debugf("(streak follower: %d) removed %d expired streak ids", nodeId, deleted)
}
