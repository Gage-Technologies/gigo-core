package follower

import (
	"context"
	"time"

	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
)

// getNextJob
//
//	Helper function to fetch the next message from a
//	NATS Jetstream pull subscription.
func getNextJob(sub *nats.Subscription, timeout time.Duration) (*nats.Msg, error) {
	// create context with timeout for fetch operation
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// wait a max of 50ms before we timeout
	msg, err := sub.Fetch(1, nats.Context(ctx))
	if err != nil {
		return nil, err
	}

	return msg[0], nil
}

// processStream
//
//		Helper function to abstract the logic of creating a new jetstream consumer, reading
//		messages from the subject until completion and relaying each message to an async
//	    execution pool.
func processStream(nodeId int64, js *mq.JetstreamClient, workerPool *pool.Pool, stream string, subject string,
	consumerName string, ackWait time.Duration, routine string, logger logging.Logger, messageHandler func(msg *nats.Msg)) {
	// create subscriber to workspace stop events
	_, err := js.ConsumerInfo(stream, consumerName)
	if err != nil {
		_, err = js.AddConsumer(stream, &nats.ConsumerConfig{
			Durable:       consumerName,
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       ackWait,
			FilterSubject: subject,
		})
		if err != nil {
			logger.Errorf("(%s: %d) failed to create %q consumer: %v", routine, nodeId, subject, err)
			return
		}
	}
	sub, err := js.PullSubscribe(subject, consumerName, nats.AckExplicit())
	if err != nil {
		logger.Errorf("(%s: %d) failed to subscribe to workspace %q events: %v", routine, nodeId, subject, err)
		return
	}

	// defer unsubscribe to ensure we don't wait on any messages
	// that cannot be processed
	defer sub.Unsubscribe()

	// iterate through active workspace stop events launching stop tasks
	// via the worker pool until we hit a timeout
	for {
		// wait a max of 50ms before we timeout
		msg, err := getNextJob(sub, time.Millisecond*50)
		if err != nil {
			// exit loop silently for timeout because it simply means
			// there is nothing to do
			if err == context.DeadlineExceeded {
				break
			}
			logger.Errorf("(%s: %d) failed to get next %q event: %v", routine, nodeId, subject, err)
			return
		}

		// launch async stopper via worker pool
		workerPool.Go(func() {
			messageHandler(msg)
		})
	}
}
