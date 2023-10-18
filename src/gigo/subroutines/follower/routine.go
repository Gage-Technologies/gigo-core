package follower

import (
	"context"
	"github.com/go-redis/redis/v8"

	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/GIGO/src/gigo/api/ws"
	"github.com/gage-technologies/GIGO/src/gigo/config"
	"github.com/gage-technologies/GIGO/src/gigo/streak"
	"github.com/gage-technologies/GIGO/src/gigo/utils"
	"github.com/gage-technologies/gigo-lib/cluster"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/sourcegraph/conc/pool"
)

func Routine(nodeId int64, cfg *config.Config, tiDB *ti.Database, wsClient *ws.WorkspaceClient, vcsClient *git.VCSClient,
	js *mq.JetstreamClient, workerPool *pool.Pool, streakEngine *streak.StreakEngine, sf *snowflake.Node,
	wsStatusUpdater *utils.WorkspaceStatusUpdater, rdb redis.UniversalClient, logger logging.Logger) cluster.FollowerRoutine {
	// we log fatal for all setup operation in this function
	// because the system cannot launch if these do not complete
	// therefore killing the process for a failure is the simplest
	// way to make sure that the error is addressed

	// create integer to track execution count
	execCount := 0

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

		// execute once every ~2s
		if execCount%2 == 0 {
			RemoveExpiredSessionKeys(ctx, nodeId, tiDB, js, logger)
		}

		// execute workspace management operations every second
		WorkspaceManagementOperations(ctx, nodeId, tiDB, wsClient, vcsClient, js, workerPool, streakEngine,
			wsStatusUpdater, rdb, logger)

		// todo possibly implement later for streak milestones

		// execute xp management operations every second
		// XpManagementOperations(ctx, nodeId, tiDB, sf, js, rdb, workerPool, logger)
		// RemoveExpiredStreakIds(ctx, nodeId, tiDB, js, logger)

		LaunchUserStatsManagementRoutine(ctx, tiDB, streakEngine, sf, workerPool, js, nodeId, logger)
		LaunchPremiumWeeklyFreeze(ctx, tiDB, workerPool, js, nodeId, rdb, logger)

		clearFreeUserWeeks(ctx, tiDB, logger, js, nodeId)

		// LaunchNemesisListener(ctx, tiDB, sf, rdb, workerPool, js, nodeId, logger)

		return nil
	}
}
