package leader

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/utils"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"strings"
	"time"
)

const updateWorkspaceExpirationForNet = `
update workspaces
set expiration = now() + interval 10 minute
where expiration < now() and 
      state = 1 and 
    _id in (
		select a.workspace_id
		from workspace_agent_stats a
		where a.timestamp >= NOW() - interval 10 minute
		group by a.workspace_id
		having sum(a.rx_bytes + a.tx_bytes) > 1024
	);
`

const queryWorkspacesStopExpired = `
select 
    * 
from workspaces 
where
    -- require that the workspace is expired
    expiration < now() and 
    (
		-- any workspace that is active or starting
		(state = 0 or state = 1) or
		-- any workspace that is stopping but has not been stopped in the last 15 minutes
		(state = 2 and last_state_update < date_sub(now(), interval 15 minute))
	)
`

const queryWorkspaceDestroySuspended = `
select 
    * 
from workspaces 
where 
    expiration < date_sub(now(), interval 24 hour) and
    (
		-- any workspace that is suspended, stopping, or failed
		(state = 3 or state = 5 or state = 2) or
		-- any workspace that is removing but has not been removed in the last 15 minutes
		(state = 4 and last_state_update < date_sub(now(), interval 15 minute))
	)
`

func stopExpired(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "stop-expired-routine")
	callerName := "stopExpired"

	// before we executed the expiration query update any workspaces with network activity
	_, err := tidb.QueryContext(ctx, &span, &callerName, updateWorkspaceExpirationForNet)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to update workspaces with network activity: %v", nodeId, err)
		return
	}

	// query workspace table for all workspaces that have an expiration less than time.Now()
	sus, err := tidb.QueryContext(ctx, &span, &callerName, queryWorkspacesStopExpired)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to query for expired active workspaces: %v\n    query: %s",
			nodeId, err, queryWorkspacesStopExpired)
		return
	}

	// defer closure of cursor
	defer sus.Close()

	// create slice for stop workspace messages
	stopMsgs := make([]models2.StopWorkspaceMsg, 0)

	// load values from cursor
	for sus.Next() {
		// load workspace from cursor
		ws, err := models.WorkspaceFromSQLNative(sus)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to scan cursor data for workspace: %v", nodeId, err)
			continue
		}

		// add to the outer messages slice to be published
		stopMsgs = append(stopMsgs, models2.StopWorkspaceMsg{
			ID:      ws.ID,
			OwnerID: ws.OwnerID,
		})
		logger.Debugf("Msg sent for stop workspace: id - %v  ownerid - %v", ws.ID, ws.OwnerID)

		// update workspace state
		ws.State = models.WorkspaceStopping

		// push workspace status update to subscribers
		wsStatusUpdater.PushStatus(ctx, ws.ID, ws)
	}

	// exit if there is nothing to do
	if len(stopMsgs) == 0 {
		return
	}

	// create slice to hold ids for workspaces that should
	// be marked as stopping and the parameter slots for the
	// sql update statement
	stopIds := make([]interface{}, 0)
	paramSlots := make([]string, 0)

	// iterate stop workspace messages publishing
	// them to the work queue
	for _, msg := range stopMsgs {
		// encode stop workspace message using gob
		buffer := bytes.NewBuffer(nil)
		encoder := gob.NewEncoder(buffer)
		err = encoder.Encode(msg)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to encode stop workspace data: %v", nodeId, err)
			continue
		}

		logger.Infof("(leader: %d) publishing stop workspace message: %d", nodeId, msg.ID)

		// publish stop workspace message so that a follower
		// can begin the process of stopping the workspace
		_, err = js.PublishAsync(streams.SubjectWorkspaceStop, buffer.Bytes())
		if err != nil {
			logger.Errorf("(workspace: %d) failed to publish stop workspace message: %v", nodeId, err)
			continue
		}

		// append id to the stop ids slice so
		// that we can mark it as stopping in the
		// next step
		stopIds = append(stopIds, msg.ID)
		paramSlots = append(paramSlots, "?")
	}

	// close cursor
	_ = sus.Close()

	// assemble the update query to mark the
	// workspaces as stopping
	query := "update workspaces set state = ?, last_state_update = ? where _id in (" + strings.Join(paramSlots, ",") + ")"

	// perform update on workspaces in database
	_, err = tidb.ExecContext(ctx, &span, &callerName, query, append([]interface{}{models.WorkspaceStopping, time.Now()}, stopIds...)...)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to update workspace state to stopping: %v\n    query: %s\n    params: %s",
			nodeId, err, query, stopIds)
	}
}

func destroySuspended(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "destroy-suspended-routine")
	callerName := "destroySuspended"

	// query workspaces that have not been re-activated for 24 hrs
	del, err := tidb.QueryContext(ctx, &span, &callerName, queryWorkspaceDestroySuspended)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to query for any attempts: %v\n    query: %s",
			nodeId, err, queryWorkspaceDestroySuspended)
		return
	}

	// defer closure of cursor
	defer del.Close()

	// create slice for destroy workspace messages
	destroyMsgs := make([]models2.DestroyWorkspaceMsg, 0)

	// load values from curser
	for del.Next() {
		// load workspace from cursor
		ws, err := models.WorkspaceFromSQLNative(del)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to scan cursor data for workspace: %v", nodeId, err)
			continue
		}

		// add to the outer messages slice to be published
		destroyMsgs = append(destroyMsgs, models2.DestroyWorkspaceMsg{
			ID:      ws.ID,
			OwnerID: ws.OwnerID,
		})

		// update workspace state
		ws.State = models.WorkspaceRemoving

		// push workspace status update to subscribers
		wsStatusUpdater.PushStatus(ctx, ws.ID, ws)
	}

	// exit if there is nothing to do
	if len(destroyMsgs) == 0 {
		return
	}

	// create slice to hold ids for workspaces that should
	// be marked as deleting and the parameter slots for the
	// sql update statement
	deleteIds := make([]interface{}, 0)
	paramSlots := make([]string, 0)

	// iterate destroy workspace messages publishing
	// them to the work queue
	for _, msg := range destroyMsgs {
		// encode destroy workspace message using gob
		buffer := bytes.NewBuffer(nil)
		encoder := gob.NewEncoder(buffer)
		err = encoder.Encode(msg)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to encode destroy workspace data: %v", nodeId, err)
			continue
		}

		logger.Infof("(leader: %d) publishing destroy workspace message: %d", nodeId, msg.ID)

		// publish stop workspace message so that a follower
		// can begin the process of stopping the workspace
		_, err = js.PublishAsync(streams.SubjectWorkspaceDestroy, buffer.Bytes())
		if err != nil {
			logger.Errorf("(workspace: %d) failed to publish destroy workspace message: %v", nodeId, err)
			continue
		}

		// append id to the delete ids slice so
		// that we can mark it as stopping in the
		// next step
		deleteIds = append(deleteIds, msg.ID)
		paramSlots = append(paramSlots, "?")
	}

	// close cursor
	_ = del.Close()

	// assemble the update query to mark the
	// workspaces as deleting
	query := "update workspaces set state = ?, last_state_update = ? where _id in (" + strings.Join(paramSlots, ",") + ")"

	// perform update on workspaces in database
	_, err = tidb.ExecContext(ctx, &span, &callerName, query, append([]interface{}{models.WorkspaceRemoving, time.Now()}, deleteIds...)...)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to update workspace state to removing: %v\n    query: %s\n    params: %s",
			nodeId, err, query, deleteIds)
	}
}

func removeDeleted(nodeId int64, js *mq.JetstreamClient, logger logging.Logger) {
	// publish message to trigger the deletion of
	// destroyed workspaces that have been destroyed
	// for more than 24 hours

	logger.Infof("(leader: %d) publishing delete workspaces message", nodeId)

	// this is handled by a follower so we don't need
	// to query we just need to instruct them to perform
	// the deletion

	_, err := js.PublishAsync(
		streams.SubjectWorkspaceDelete,
		// the message content isn't used so this is more of
		// a way to debug the pipeline. we include the leader
		// id that issued the message and the unix ts from when
		// the message was issued
		[]byte(fmt.Sprintf("%d-%d", nodeId, time.Now().Unix())),
	)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to publish clean session keys message: %v", nodeId, err)
	}
}

func WorkspaceManagementOperations(ctx context.Context, nodeId int64, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "workspace-management-operations-routine")
	defer parentSpan.End()

	stopExpired(ctx, nodeId, tidb, js, wsStatusUpdater, logger)
	destroySuspended(ctx, nodeId, tidb, js, wsStatusUpdater, logger)
	// removeDeleted(nodeId, js, logger)

	parentSpan.AddEvent(
		"workspace-management-operations-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}
