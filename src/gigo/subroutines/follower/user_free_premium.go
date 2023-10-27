package follower

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
)

func clearFreeUserWeeks(ctx context.Context, db *ti.Database, logger logging.Logger, js *mq.JetstreamClient, nodeId int64) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "claim-free-user-weeks-routine")
	defer parentSpan.End()
	callerName := "clearFreeUserWeeks"

	// create subscription for session key management
	_, err := js.ConsumerInfo(streams.SubjectMiscUserFreePremium, "gigo-core-follower-user-free-premium")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamMisc, &nats.ConsumerConfig{
			Durable:       "gigo-core-follower-user-free-premium",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectMiscUserFreePremium,
		})
		if err != nil {
			logger.Errorf("(session_key: %d) failed to create user free premium consumer: %v", nodeId, err)
			return
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectMiscUserFreePremium, "gigo-core-follower-user-free-premium", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(session_key: %d) failed to create user free premium subscription: %v", nodeId, err)
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

	res, err := db.QueryContext(ctx, &parentSpan, &callerName, "select _id, user_id from user_free_premium where end_date <= CURDATE()")
	if err != nil {
		logger.Errorf("failed to query for users: %v", err)
		return
	}

	defer res.Close()

	// var userPremiums []models.UserFreePremium
	for res.Next() {
		var userPremium models.UserFreePremium
		err = res.Scan(&userPremium.Id, &userPremium.UserId)
		if err != nil {
			logger.Errorf("failed to decode user model in subroutine: %v", err)
		}

		// // attempt to decode res into post model
		// userPremium, err := models.UserFreePremiumFromSQLNative(res)
		// if err != nil {
		//	logger.Errorf("failed to decode user model in subroutine: %v", err)
		//	return
		// }

		// increment tag column usage_count in database
		_, err = db.ExecContext(ctx, &parentSpan, &callerName, "update users set user_status = ? where _id = ?", 0, userPremium.UserId)
		if err != nil {
			logger.Errorf("failed to update users for basic again: %v", err)
			return
		}

		// increment tag column usage_count in database
		_, err = db.ExecContext(ctx, &parentSpan, &callerName, "delete from user_free_premium where _id = ?", userPremium.Id)
		if err != nil {
			logger.Errorf("failed to update users for basic again: %v", err)
			return
		}
	}

	// ack the message so it isn't repeated
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(session_key: %d) failed to ack message: %v", nodeId, err)
		return
	}

	parentSpan.AddEvent(
		"claim-free-user-weeks-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	logger.Debugf("updated the users to be basic")
}
