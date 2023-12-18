package leader

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"gigo-core/gigo/config"
	"gigo-core/gigo/utils"

	"github.com/gage-technologies/gigo-lib/cluster"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/mq/streams"
)

func publishSessionKeyCleanup(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	_, err := js.PublishAsync(
		streams.SubjectMiscSessionCleanKeys,
		// the message content isn't used so this is more of
		// a way to debug the pipeline. we include the leader
		// id that issued the message and the unix ts from when
		// the message was issued
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish clean session keys message: %v", nodeId, err)
	}
}

func publishSitemapGeneration(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	_, err := js.PublishAsync(
		streams.SubjectMiscSitemapGenerate,
		// the message content isn't used so this is more of
		// a way to debug the pipeline. we include the leader
		// id that issued the message and the unix ts from when
		// the message was issued
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish sitemap generation message: %v", nodeId, err)
	}
}

func publishStreakExpirationCleanup(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	_, err := js.PublishAsync(
		streams.SubjectStreakExpiration,
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish streak expiration message: %v", nodeId, err)
	}
}

func publishHandleDayRollover(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	_, err := js.PublishAsync(
		streams.SubjectDayRollover,
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish streak expiration message: %v", nodeId, err)
	}
}

func publishNemesisListener(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	_, err := js.PublishAsync(
		streams.SubjectNemesisStatChange,
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish nemesis listener message: %v", nodeId, err)
	}
}

func publishUserFreePremium(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	_, err := js.PublishAsync(
		streams.SubjectMiscUserFreePremium,
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish user free premium message: %v", nodeId, err)
	}
}

func publishHandlePremiumWeeklyFreeze(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	logger.Infof("starting the asyncs task for premium weekly freeze: %d", nodeId)
	_, err := js.PublishAsync(
		streams.SubjectPremiumFreeze,
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	logger.Info("starting the asyncs task for premium weekly freeze after")
	if err != nil {
		logger.Errorf("(leader: %d) failed to publish premium weekly freeze message: %v", nodeId, err)
	}
}

func Routine(nodeId int64, cfg *config.Config, tiDB *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient,
	wsStatusUpdater *utils.WorkspaceStatusUpdater, logger logging.Logger) cluster.LeaderRoutine {
	// create integer to track execution count
	execCount := 0

	// create time variable to track the last execution of the user stats routine
	lastUserStatsExec := time.Unix(0, 0)
	lastSiteMapGenExec := time.Unix(0, 0)
	// lastNemesisExec := time.Unix(0, 0)

	// this function will be executed approximately once every second.
	// when defining routine logic that will execute on interval
	// use the execCount variable to offset the execution from the
	// refresh rate. for example, if we want to execute a function
	// once every 5 seconds we should only perform the execution if
	// execCount % 5 == 0 so that the logic is only executed once
	// every 5 refreshes or approximately every 5 seconds.
	return func(ctx context.Context) error {
		// defer function to handle logic that should be
		// executed on each refresh interval
		defer func() {
			execCount++
		}()

		logger.Infof("(leader: %d) executing leader routine tick", nodeId)

		// send job for session key cleanup once every 3s
		if execCount%3 == 0 {
			logger.Infof("(leader: %d) publishing session key cleanup", nodeId)
			publishSessionKeyCleanup(nodeId, js, logger)
		}

		logger.Infof("(leader: %d) executing workspace management operations", nodeId)

		// perform workspace management operations every second
		WorkspaceManagementOperations(ctx, nodeId, tiDB, js, wsStatusUpdater, logger)

		// todo implement for streak milestones later

		// perform xp management operations every second
		// XpManagementOperations(ctx, nodeId, tiDB, js, logger)
		// publishStreakExpirationCleanup(nodeId, js, logger)

		// handle user day rollover every second
		publishHandleDayRollover(nodeId, js, logger)

		// send job for email jobs every 3s
		if execCount%3 == 0 {
			logger.Infof("(leader: %d) sending email jobs", nodeId)
			StreamInactivityEmailRequests(nodeId, ctx, tiDB, js, logger)
		}

		logger.Info("right before the weekly freeze handle function")

		// publishHandlePremiumWeeklyFreeze(nodeId, js, logger)
		// publishUserFreePremium(nodeId, js, logger)

		// execute on every 30m interval
		timeNow := time.Now()
		timeNowTrimmedSec := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), timeNow.Hour(), timeNow.Minute(), 0, 0, timeNow.Location())
		if timeNow.Minute()%30 == 0 && timeNow.Second() < 10 &&
			timeNowTrimmedSec.Unix() != lastUserStatsExec.Unix() {
			// update last execution time
			lastUserStatsExec = timeNowTrimmedSec
			logger.Infof("(leader: %d) executing user state management operations", nodeId)

			publishHandlePremiumWeeklyFreeze(nodeId, js, logger)
			publishUserFreePremium(nodeId, js, logger)
		}

		// execute at midnight
		if timeNow.Hour() == 0 && timeNow.Minute() == 0 && timeNow.Second() < 10 &&
			timeNowTrimmedSec.Unix() != lastSiteMapGenExec.Unix() {
			// update last execution time
			lastSiteMapGenExec = timeNowTrimmedSec
			logger.Infof("(leader: %d) executing site map generation operations", nodeId)

			publishSitemapGeneration(nodeId, js, logger)
		}

		// timeNow = time.Now()
		// timeNowTrimmedSec = time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), timeNow.Hour(), timeNow.Minute(), 0, 0, timeNow.Location())
		// if timeNow.Minute()%10 == 0 && timeNow.Second() < 10 &&
		// 	timeNowTrimmedSec.Unix() != lastNemesisExec.Unix() {
		// 	// update last execution time
		// 	lastNemesisExec = timeNowTrimmedSec
		// 	logger.Infof("(leader: %d) executing nemesis management operations", nodeId)
		//
		// 	publishNemesisListener(nodeId, js, logger)
		// }

		return nil
	}
}
