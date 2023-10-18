package follower

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/gigo/api/external_api/core"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/kisielk/sqlstruct"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
)

const (
	notificationMsg = "Notification Initiated"
)

func LaunchNemesisListener(ctx context.Context, db *ti.Database, sf *snowflake.Node, rdb redis.UniversalClient, workerPool *pool.Pool, js *mq.JetstreamClient, nodeId int64, logger logging.Logger) {
	// logger.Errorf("(nemesis_stat_change: %d) starting LaunchUserStatsManagementRoutine", nodeId)
	// create subscription for session key management

	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "launch-nemesis-listener-routine")
	defer parentSpan.End()

	_, err := js.ConsumerInfo(streams.SubjectNemesisStatChange, "gigo-core-nemesis-stat-change")
	if err != nil {
		_, err = js.AddConsumer(streams.StreamNemesis, &nats.ConsumerConfig{
			Durable:       "gigo-core-nemesis-stat-change",
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       time.Second * 30,
			FilterSubject: streams.SubjectNemesisStatChange,
		})
		if err != nil {
			logger.Errorf("(nemesis_stat_change: %d) failed to create session key consumer: %v", nodeId, err)
			return
		}
	}
	sub, err := js.PullSubscribe(streams.SubjectNemesisStatChange, "gigo-core-nemesis-stat-change", nats.AckExplicit())
	if err != nil {
		logger.Errorf("(nemesis_stat_change: %d) failed to create session key subscription: %v", nodeId, err)
		return
	}
	defer sub.Unsubscribe()

	// logger.Errorf("(nemesis_stat_change: %d) acquiring message in LaunchUserStatsManagementRoutine", nodeId)
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
		logger.Errorf("(nemesis_stat_change: %d) failed to retrieve message from jetstream: %v", nodeId, err)
		return
	}

	// logger.Errorf("(nemesis_stat_change: %d) executing worker pool on handleInactiveUserDayRollover", nodeId)
	workerPool.Go(func() {
		err := handleNemesisChanges(ctx, db, rdb, sf, js, logger)
		if err != nil {
			logger.Errorf("failed to handleNemesisChange: %v", err)
		}

		parentSpan.AddEvent(
			"handled-nemesis-change",
			trace.WithAttributes(
				attribute.Bool("success", true),
			),
		)

	})

	// ack the message so it isn't repeated
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(nemesis_stat_change: %d) failed to ack message: %v", nodeId, err)
		return
	}
}

func handleNemesisChanges(ctx context.Context, db *ti.Database, rdb redis.UniversalClient, sf *snowflake.Node, js *mq.JetstreamClient, logger logging.Logger) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "handle-nemesis-changes")
	callerName := "handle-nemesis-changes"

	logger.Debugf("starting to handleNemesisChanges")
	// create array to hold timezones that are past midnight in their local time

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// create query to retrieve all users that are active and just had a day change in their timezone
	query := "select * from nemesis where victor is NULL and is_accepted = true"

	// execute end of day query
	res, err := tx.QueryContext(ctx, &callerName, query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}

	defer res.Close()

	nemeses := make([]models.Nemesis, 0)

	for res.Next() {
		var nemesis models.Nemesis
		// scan row into user stats object
		err := sqlstruct.Scan(&nemesis, res)
		if err != nil {
			return fmt.Errorf("Error scanning user stats in first scan: %v", err)
		}

		nemeses = append(nemeses, nemesis)
	}

	if len(nemeses) < 1 {
		return nil
	}

	for _, nemesis := range nemeses {
		res, err = tx.QueryContext(ctx, &callerName, "select sum(xp) from xp_reasons where date >= ? and user_id = ?", nemesis.TimeOfVillainy, nemesis.AntagonistID)
		if err != nil {
			return fmt.Errorf("failed to execute query for total antagonist xp, err: %v", err)
		}
		var antagonistXP sql.NullInt64
		for res.Next() {
			err = res.Scan(&antagonistXP)
			if err != nil {
				return fmt.Errorf("failed to scan results for total protagonist xp, err: %v", err)
			}
		}

		// If antagonistXP is NULL, set it to 0.
		if !antagonistXP.Valid {
			antagonistXP.Int64 = 0
		}

		res, err = tx.QueryContext(ctx, &callerName, "select sum(xp) from xp_reasons where date >= ? and user_id = ?", nemesis.TimeOfVillainy, nemesis.ProtagonistID)
		if err != nil {
			return fmt.Errorf("failed to execute query for total protagonist xp, err: %v", err)
		}
		var protagonistXP sql.NullInt64
		for res.Next() {
			err = res.Scan(&protagonistXP)
			if err != nil {
				return fmt.Errorf("failed to scan results for total protagonist xp, err: %v", err)
			}
		}

		// If protagonistXP is NULL, set it to 0.
		if !protagonistXP.Valid {
			protagonistXP.Int64 = 0
		}

		rows, err := tx.QueryContext(ctx, &callerName,
			"select * from nemesis_history where match_id = ? ORDER BY created_at DESC LIMIT 1 ",
			nemesis.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to execute query: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			return sql.ErrNoRows
		}

		var history models.NemesisHistory
		err = sqlstruct.Scan(&history, rows)
		if err != nil {
			return fmt.Errorf("failed to scan results for nemesis history, err: %v", err)
		}

		_ = rows.Close()

		logger.Debugf("handleNemesisChanges: nemesis history: %v", history)
		diff := antagonistXP.Int64 - protagonistXP.Int64
		antagTowers := 0
		protagTowers := 0
		changedTowers := ""

		var prevAntagTowers = 0
		var prevProtagTowers = 0
		var antagCapturedTowers = 0
		var protagCapturedTowers = 0

		var victor int64 = 0

		if diff >= 1000 {
			antagTowers = 5
			protagTowers = 0
			changedTowers = fmt.Sprintf("%v captured %v's base", nemesis.AntagonistName, nemesis.ProtagonistName)
			victor = nemesis.AntagonistID
		} else if diff >= 500 {
			antagTowers = 4
			protagTowers = 1
			changedTowers = fmt.Sprintf("%v captured left tower", nemesis.AntagonistName)
		} else if diff >= 1 {
			antagTowers = 3
			protagTowers = 2
			changedTowers = fmt.Sprintf("%v captured middle tower", nemesis.AntagonistName)
		} else if diff == 0 {
			antagTowers = 2
			protagTowers = 2
			changedTowers = fmt.Sprintf("middle tower is uncontested")
		} else if diff <= -1000 {
			protagTowers = 5
			antagTowers = 0
			changedTowers = fmt.Sprintf("%v captured %v's base", nemesis.ProtagonistName, nemesis.AntagonistName)
			victor = nemesis.ProtagonistID
		} else if diff <= -500 {
			protagTowers = 4
			antagTowers = 1
			changedTowers = fmt.Sprintf("%v captured right tower", nemesis.ProtagonistName)
		} else if diff <= -1 {
			protagTowers = 3
			antagTowers = 2
			changedTowers = fmt.Sprintf("%v captured middle tower", nemesis.ProtagonistName)
		}

		// Calculate the difference from the previous state
		antagCaptureDiff := antagTowers - prevAntagTowers
		protagCaptureDiff := protagTowers - prevProtagTowers

		// Update the total capture counts
		antagCapturedTowers += antagCaptureDiff
		protagCapturedTowers += protagCaptureDiff

		// Store the new state for the next iteration
		prevAntagTowers = antagTowers
		prevProtagTowers = protagTowers

		changedStats := false
		changedXP := false

		if int64(antagTowers) != history.AntagonistTowersHeld || int64(protagTowers) != history.ProtagonistTowersHeld {
			history.ProtagonistTowersHeld = int64(protagTowers)
			history.AntagonistTowersHeld = int64(antagTowers)
			changedStats = true
		}

		if history.ProtagonistTotalXP != protagonistXP.Int64 || history.AntagonistTotalXP != antagonistXP.Int64 {
			history.ProtagonistTotalXP = protagonistXP.Int64
			history.AntagonistTotalXP = antagonistXP.Int64
			changedXP = true
		}

		if changedStats || changedXP {
			history.ID = sf.Generate().Int64()
			history.CreatedAt = time.Now()
			statements := history.ToSQLNative()
			for _, statement := range statements {
				_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
				if err != nil {
					return fmt.Errorf("handleNemesisChanges: failed to execute insert for new nemesis history, err: %v", err)
				}
			}

			_, err = tx.ExecContext(ctx, &callerName, "update nemesis set antagonist_towers_captured = ?, protagonist_towers_captured = ? where _id = ?", antagCapturedTowers, protagCapturedTowers, nemesis.ID)
			if err != nil {
				return fmt.Errorf("handleNemesisChanges: failed update towers captured in nemesis table, err: %v", err)
			}

			if changedStats {
				fmt.Println(changedTowers)
				// send notification to antagonist
				_, err := core.CreateNotification(ctx, db, js, sf, nemesis.AntagonistID, changedTowers, models.NemesisAlert, &nemesis.ProtagonistID)
				if err != nil {
					return fmt.Errorf("handleNemesisChanges. Failed to create notification: %v", err)
				}

			}
		}

		if nemesis.EndTime.Before(time.Now()) {
			if antagonistXP.Int64 > protagonistXP.Int64 {
				victor = nemesis.AntagonistID
			} else {
				victor = nemesis.ProtagonistID
			}
		}

		if victor != 0 {
			_, err = tx.ExecContext(ctx, &callerName, "update nemesis set victor = ?, end_time = ? where _id = ?", victor, time.Now(), nemesis.ID)
			if err != nil {
				return fmt.Errorf("handleNemesisChanges: failed update towers captured in nemesis table, err: %v", err)
			}

			if victor == nemesis.AntagonistID {
				// AddXP 'nemesis_victory' for nemesis.AntagonistID
				_, err = core.AddXP(ctx, db, js, rdb, sf, nemesis.AntagonistID, "nemesis_victory", nil, &antagCapturedTowers, logger, nil)
				if err != nil {
					return fmt.Errorf("handleNemesisChanges. Failed to add XP: %v", err)
				}

				// AddXP 'nemesis_defeat' for nemesis.ProtagonistID
				_, err = core.AddXP(ctx, db, js, rdb, sf, nemesis.ProtagonistID, "nemesis_defeat", nil, &protagCapturedTowers, logger, nil)
				if err != nil {
					return fmt.Errorf("handleNemesisChanges. Failed to add XP: %v", err)
				}
			} else {
				// AddXP 'nemesis_victory' for nemesis.ProtagonistID
				_, err = core.AddXP(ctx, db, js, rdb, sf, nemesis.ProtagonistID, "nemesis_victory", nil, &protagCapturedTowers, logger, nil)
				if err != nil {
					return fmt.Errorf("handleNemesisChanges. Failed to add XP: %v", err)
				}

				// AddXP 'nemesis_defeat' for nemesis.AntagonistID
				_, err = core.AddXP(ctx, db, js, rdb, sf, nemesis.AntagonistID, "nemesis_defeat", nil, &antagCapturedTowers, logger, nil)
				if err != nil {
					return fmt.Errorf("handleNemesisChanges. Failed to add XP: %v", err)
				}
			}
		}

	}

	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	logger.Debugf("(handleNemesisChanges) successfully inserted new rows")
	return nil
}
