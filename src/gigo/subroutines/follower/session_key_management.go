package follower

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
)

func RemoveExpiredSessionKeys(ctx context.Context, nodeId int64, db *ti.Database, js *mq.JetstreamClient, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "remove-expire-session-keys-routine")
	defer parentSpan.End()
	callerName := "RemoveExpiredSessionKeys"

	// create subscription for session key management
	_, err := js.ConsumerInfo(streams.SubjectMiscSessionCleanKeys, "gigo-core-follower-session-keys")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamMisc, &nats.ConsumerConfig{
			Durable:       "gigo-core-follower-session-keys",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectMiscSessionCleanKeys,
		})
		if err != nil {
			logger.Errorf("(session_key: %d) failed to create session key consumer: %v", nodeId, err)
			return
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectMiscSessionCleanKeys, "gigo-core-follower-session-keys", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(session_key: %d) failed to create session key subscription: %v", nodeId, err)
		return
	}
	defer sub.Unsubscribe()

	// request next message from the subscriber. this tells use
	// that we are the follower to perform the key expiration.
	// we use such a short timeout because this will re-execute in
	// ~1s so if there is nothing to do now then we should not slow
	// down the refresh rate of the follower loop
	msg, err := getNextJob(sub, time.Millisecond*50)
	if err != nil {
		// exit silently for timeout because it simply means
		// there is nothing to do
		if err == context.DeadlineExceeded {
			return
		}
		logger.Errorf("(session_key: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}

	logger.Debugf("(session_key: %d) removing expired session keys", nodeId)

	res, err := db.ExecContext(ctx, &parentSpan, &callerName, "delete from user_session_key where expiration < ?", time.Now())
	if err != nil {
		logger.Errorf("(session_key: %d) failed to remove expired session keys: %v", nodeId, err)
		return
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		logger.Errorf("(session_key: %d) failed to check deleted count when removing expired session keys: %v", nodeId, err)
		return
	}

	// ack the message so it isn't repeated
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(session_key: %d) failed to ack message: %v", nodeId, err)
		return
	}

	parentSpan.AddEvent(
		"remove-expire-session-keys-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	logger.Debugf("(session_key: %d) removed %d expired session keys", nodeId, deleted)
}
