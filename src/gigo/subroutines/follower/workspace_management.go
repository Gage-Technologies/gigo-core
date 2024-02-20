package follower

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/gigo/streak"
	"gigo-core/gigo/utils"

	"github.com/go-redis/redis/v8"

	"gigo-core/gigo/api/external_api/core"
	"gigo-core/gigo/api/ws"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc/pool"
)

// TODO: needs test

// ///////// General helper functions

// workspaceInitializationFailure
//
//	Updates the workspace initialization state to show failure
//	and includes a log that can be used by users or developers
//	to understand why the workspace initialization failed
func workspaceInitializationFailure(ctx context.Context, tidb *ti.Database, wsStatusUpdater *utils.WorkspaceStatusUpdater, wsId int64,
	step models.WorkspaceInitState, command string, status int, stdout string, stderr string) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "workspace-initialization-failure-routine")
	defer span.End()
	callerName := "WorkspaceInitializationFailure"

	// create workspace init failure with the passed initFailureBytes
	initFailure := &models.WorkspaceInitFailure{
		Command: command,
		Status:  status,
		Stdout:  stdout,
		Stderr:  stderr,
	}

	// serialize init failure data
	initFailureBytes, err := json.Marshal(initFailure)
	if err != nil {
		return fmt.Errorf("failed to serialize init failure initFailureBytes: %v", err)
	}

	// perform update directly on the workspace row
	_, err = tidb.ExecContext(ctx, &span, &callerName,
		"update workspaces set init_state = ?, init_failure = ?, state = ?, last_state_update = ? where _id = ?",
		step, initFailureBytes, models.WorkspaceFailed, time.Now(), wsId,
	)
	if err != nil {
		// handle not found
		if err == sql.ErrNoRows {
			return fmt.Errorf("workspace not found")
		}
		// handle error
		return fmt.Errorf("failed to update workspace: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, wsId, nil)

	return nil
}

// boundWorkspaceAllocations
//
//	Checks the user's account status and bounds the maximum values for
//	the workspace resources to the maximum amount permitted for the user's
//	account status.
func boundWorkspaceAllocations(ctx context.Context, tidb *ti.Database, msg models2.CreateWorkspaceMsg) (models2.CreateWorkspaceMsg, error) {
	ctx, span := otel.Tracer("gigo-core").Start(context.Background(), "bound-workspace-allocations-routine")
	defer span.End()
	callerName := "BoundWorkspaceAllocations"

	// query user status for the owner of the workspace
	var userStatus models.UserStatus
	err := tidb.QueryRowContext(ctx, &span, &callerName, "select user_status from users where _id = ?", msg.OwnerID).Scan(&userStatus)
	if err != nil {
		return models2.CreateWorkspaceMsg{}, fmt.Errorf("failed to query user status during allocation bouding: %v", err)
	}

	// bound the values to the maximum permitted for the user's account status
	if userStatus != models.UserStatusPremium {
		if msg.CPU > 2 {
			msg.CPU = 2
		}
		if msg.Memory > 3 {
			msg.Memory = 3
		}
		if msg.Disk > 15 {
			msg.Disk = 15
		}
	} else {
		if msg.CPU > 6 {
			msg.CPU = 6
		}
		if msg.Memory > 8 {
			msg.Memory = 8
		}
		if msg.Disk > 50 {
			msg.Disk = 50
		}
	}

	return msg, nil
}

// ///////// Helper functions to handle the provisioner call and state updates for individual workspaces
// Note: these functions use local contexts outside the scope of the follower routine so that they can
// complete their operations even after the follower routine has been cancelled

// asyncCreateWorkspace
//
//	Handles the creation of a new workspace via the remote provisioner
//	system (gigo-ws) and updates the database directly on completion or failure
func asyncCreateWorkspace(nodeId int64, tidb *ti.Database, wsClient *ws.WorkspaceClient, wsStatusUpdater *utils.WorkspaceStatusUpdater,
	js *mq.JetstreamClient, msg *nats.Msg, zitiManager *zitimesh.Manager, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "async-create-workspace-routine")
	defer span.End()
	callerName := "asyncCreateWorkspace"

	// unmarshall create workspace request message
	var createWsMsg models2.CreateWorkspaceMsg
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&createWsMsg)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to decode create workspace message: %v", nodeId, err)
		return
	}

	// log that we started provisioning
	logger.Infof("(workspace: %d) creating workspace %d", nodeId, createWsMsg.WorkspaceID)

	// create boolean to track failure
	failed := true

	// create variable to hold agent id so we can cleanup if we fail
	agentId := int64(-1)

	// create context for workspace create
	// we set the timeout to 15 minutes which is pretty high
	// given that this API call will only need to instruct kubernetes
	// to launch a new pod for the workspace, so it should complete within
	// 40s (if this isn't true the image is too big or the system broke)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)

	// cleanup workspace on failure
	defer func() {
		// recover from panic if one occurred
		if r := recover(); r != nil {
			logger.Errorf(
				"(workspace: %d) panic while creating workspace %d: %v\n--- Stack---\n%s",
				nodeId, createWsMsg.WorkspaceID, r, string(debug.Stack()),
			)
		}

		// we always ack because each workspace is either successfully created or failed on a on-shot basis
		// we may someday change this to perform a retry on failures but for now we just want to get rid of the
		// workspace and allow the user to create a new one
		err = msg.Ack()
		if err != nil {
			logger.Errorf("(workspace: %d) failed to ack create workspace message: %v", nodeId, err)
		}

		// defer cancel
		defer cancel()

		// skip if we succeeded
		if !failed {
			return
		}

		// I'm not sure that we need consistency here since this is inherently an optimistic
		// cleanup operation. If we failed this entire cleanup it wouldn't matter much since
		// the workspace manager will cleanup within the next 30 minutes. This is mostly for
		// a good user experience - being able to see the failure instantly.

		// mark workspace as failed
		err := workspaceInitializationFailure(ctx, tidb, wsStatusUpdater, createWsMsg.WorkspaceID, models.WorkspaceInitProvisioning, "provision",
			-1, "", "provisioning failed")
		if err != nil {
			logger.Warn(fmt.Errorf(
				"(workspace: %d) failed to mark failed workspace %d with failure: %v",
				nodeId, createWsMsg.WorkspaceID, err,
			))
		}

		// delete the workspace agent if it was created
		if agentId != -1 {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "delete from workspace_agent where _id = ?", agentId)
			if err != nil {
				logger.Warn(fmt.Errorf(
					"(workspace: %d) failed delete agent %d for for workspace %d: %v",
					nodeId, agentId, createWsMsg.WorkspaceID, err,
				))
			}
		}

		// create context for the deletion operation
		// for reference on why we set timeout to 5m look in the comments
		// below for context creation when creating a workspace
		destroyCtx, destroyCancel := context.WithTimeout(context.Background(), time.Minute*5)

		// defer cancel to ensure we don't leak resources
		defer destroyCancel()

		// destroy the workspace via the remote provisioner
		err = wsClient.DestroyWorkspace(destroyCtx, createWsMsg.WorkspaceID)
		if err != nil {
			logger.Warn(fmt.Errorf(
				"(workspace: %d) failed destroy workspace %d on creation failure: %v",
				nodeId, createWsMsg.WorkspaceID, err,
			))
		}
	}()

	// bound the workspace allocations to the maximum permitted for the user's account status
	createWsMsg, err = boundWorkspaceAllocations(ctx, tidb, createWsMsg)
	if err != nil {
		logger.Error(fmt.Errorf(
			"(workspace: %d) failed to bound workspace allocations for workspace %d: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
		return
	}

	// create workspace via remote provisioner
	newAgentData, err := wsClient.CreateWorkspace(ctx, ws.CreateWorkspaceOptions{
		WorkspaceID: createWsMsg.WorkspaceID,
		OwnerID:     createWsMsg.OwnerID,
		OwnerName:   createWsMsg.OwnerName,
		OwnerEmail:  createWsMsg.OwnerEmail,
		Disk:        createWsMsg.Disk,
		Memory:      createWsMsg.Memory,
		CPU:         createWsMsg.CPU,
		Container:   createWsMsg.Container,
		AccessUrl:   createWsMsg.AccessUrl,
	})
	if err != nil {
		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to create workspace %d via remote prvisioner: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
		return
	}

	// create a new ziti mesh agent for the new agent
	zitiAgentID, zitiAgentToken, err := zitiManager.CreateAgent(newAgentData.ID)
	if err != nil {
		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to create ziti agent for workspace %d: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
		return
	}

	// enroll the identity into a configuration
	zitiConfig, err := zitimesh.EnrollIdentity(zitiAgentToken)
	if err != nil {
		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to enroll ziti agent for workspace %d: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
		return
	}
	zitiConfigBuf, err := json.Marshal(&zitiConfig)
	if err != nil {
		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to marshal ziti config for workspace %d: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
	}

	// create a new agent
	newAgent := models.CreateWorkspaceAgent(
		newAgentData.ID,
		createWsMsg.WorkspaceID,
		"",
		createWsMsg.OwnerID,
		newAgentData.Token,
		zitiAgentID,
		string(zitiConfigBuf),
	)

	// update agent id for cleanup function
	agentId = newAgent.ID

	// format agent to sql
	agentInsertionStatements := newAgent.ToSQLNative()

	// create tx for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to open tx for agent insertion and ws update %d: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
	}

	// defer rollback in case we fail
	defer tx.Rollback()

	// iterate insertion statements executing insertions via tx
	for _, statement := range agentInsertionStatements {
		_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			logger.Error(fmt.Sprintf(
				"(workspace: %d) failed to insert agent for newly create workspace %d: %v",
				nodeId, createWsMsg.WorkspaceID, err,
			))
		}
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to commit tx for agent insertion and ws update %d: %v",
			nodeId, createWsMsg.WorkspaceID, err,
		))
	}

	// mark failed as false so we don't trigger the cleanup function
	failed = false
}

// asyncStartWorkspace
//
//	Handles the start of an existing workspace via the remote provisioner
//	system (gigo-ws) and updates the database directly on completion or failure
func asyncStartWorkspace(nodeId int64, tidb *ti.Database, wsClient *ws.WorkspaceClient, wsStatusUpdater *utils.WorkspaceStatusUpdater,
	js *mq.JetstreamClient, msg *nats.Msg, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "async-start-workspace-routine")
	defer span.End()
	callerName := "asyncStartWorkspace"

	// unmarshall start workspace request message
	var startWsMsg models2.StartWorkspaceMsg
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&startWsMsg)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to decode create workspace message: %v", nodeId, err)
		return
	}

	// log that we are starting the workspace
	logger.Infof("(workspace: %d) starting workspace %d", nodeId, startWsMsg.ID)

	// create boolean to track failure
	failed := true

	// create variable to hold agent id so we can cleanup if we fail
	agentId := int64(-1)

	// cleanup workspace on failure
	defer func() {
		// recover from panic if one occurred
		if r := recover(); r != nil {
			logger.Errorf(
				"(workspace: %d) panic while starting workspace %d: %v\n--- Stack---\n%s",
				nodeId, startWsMsg.ID, r, string(debug.Stack()),
			)
		}

		// skip if we succeeded
		if !failed {
			return
		}

		// mark workspace as stopping
		_, err := tidb.ExecContext(ctx, &span, &callerName,
			"update workspaces set state = ?, last_state_update = ? where _id = ?",
			models.WorkspaceStopping, time.Now(), startWsMsg.ID,
		)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to update workspace table for stopping: %v", nodeId, err)
		}

		// delete the workspace agent if it was created
		if agentId != -1 {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "delete from workspace_agent where _id = ?", agentId)
			if err != nil {
				logger.Warn(fmt.Errorf(
					"(workspace: %d) failed delete agent %d for workspace %d: %v",
					nodeId, agentId, startWsMsg.ID, err,
				))
			}
		}

		// create context for the stop operation
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Minute*5)

		// defer cancel to ensure we don't leak resources
		defer stopCancel()

		// set the state that we will fallback to
		fallbackState := models.WorkspaceSuspended

		// stop the workspace via the remote provisioner
		err = wsClient.StopWorkspace(stopCtx, startWsMsg.ID)
		if err != nil {
			if errors.Is(err, ws.ErrWorkspaceNotFound) {
				fallbackState = models.WorkspaceFailed
			}
			logger.Warn(fmt.Errorf(
				"(workspace: %d) failed stop workspace %d on start failure: %v",
				nodeId, startWsMsg.ID, err,
			))
		}

		// mark workspace as stopped and update the expiration to now
		// so that the workspace volume won't be destroyed for 24 hours
		_, err = tidb.ExecContext(stopCtx, &span, &callerName,
			"update workspaces set state = ?, last_state_update = ? where _id = ?",
			fallbackState, time.Now(), startWsMsg.ID,
		)
		if err != nil {
			logger.Error(fmt.Sprintf(
				"(workspace: %d) failed to update workspace table for suspended: %v",
				nodeId, err,
			))
		}
	}()

	// create context for workspace create
	// we set the timeout to 15 minutes which is pretty high
	// given that this API call will only need to instruct kubernetes
	// to launch a new pod for the workspace, so it should complete within
	// 40s (if this isn't true the image is too big or the system broke)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)
	defer cancel()

	// start workspace via the remote provisioner
	newAgentData, err := wsClient.StartWorkspace(ctx, startWsMsg.ID)
	if err != nil {
		if errors.Is(err, ws.ErrAlternativeRequestActive) {
			// another node is handling the start request so skip
			return
		} else if errors.Is(err, ws.ErrWorkspaceNotFound) {
			logger.Warnf("(workspace: %d) failed start workspace %d - not found", nodeId, startWsMsg.ID)

			// marshall an init state object to mark the failure
			initState, err := json.Marshal(models.WorkspaceInitFailure{Stderr: "workspace not found"})
			if err != nil {
				// fallback to a hard set json
				// NOTE: it is better to marshall the object above so that we can track any changes
				//       to the object down the line - setting the json like this allows unexpected errors
				initState = []byte(`{"stderr":"workspace not found"}`)
			}

			// if the workspace is not found t
			_, err = tidb.ExecContext(ctx, &span, &callerName,
				"update workspaces set state = ?, last_state_update = ?, init_failure = ? where _id = ?",
				models.WorkspaceFailed, time.Now(), startWsMsg.ID, initState,
			)
			if err != nil {
				logger.Errorf("(workspace: %d) failed to update workspace table for failed on start: %v", nodeId, err)
				return
			}
			// send workspace status update
			wsStatusUpdater.PushStatus(ctx, startWsMsg.ID, nil)
			return
		}

		logger.Error(fmt.Sprintf(
			"(workspace: %d) failed to perform start transition on existing workspace: %v",
			nodeId, err,
		))
		return
	}

	// update the secret for the agent
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update workspace_agent set secret = uuid_to_bin(?) where _id = ?", newAgentData.Token, newAgentData.ID)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to insert new workspace agent: %v", nodeId, err)
		return
	}

	// mark failed as false
	failed = false

	// acknowledge that the workspace is now started
	err = msg.Ack()
	if err != nil {
		logger.Errorf("(workspace: %d) failed to ack start workspace message: %v", nodeId, err)
	}
}

// asyncStopWorkspace
//
//	Handles the stopping of an existing workspace via the remote provisioner
//	system (gigo-ws) and updates the database directly on completion or failure
func asyncStopWorkspace(nodeId int64, tidb *ti.Database, wsClient *ws.WorkspaceClient, js *mq.JetstreamClient,
	streakEngine *streak.StreakEngine, wsStatusUpdater *utils.WorkspaceStatusUpdater, msg *nats.Msg, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "async-stop-workspace-routine")
	defer span.End()
	callerName := "asyncStopWorkspace"

	// unmarshall stop workspace message
	var stopWsMsg models2.StopWorkspaceMsg
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&stopWsMsg)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to decode stop workspace message: %v", nodeId, err)
		return
	}

	// log that we are stopping the workspace
	logger.Infof("(workspace: %d) stopping workspace %d", nodeId, stopWsMsg.ID)

	// always ack the message since the leader will retry if we fail
	defer func() {
		// acknowledge that the workspace is now stopped
		err := msg.Ack()
		if err != nil {
			logger.Errorf("(workspace: %d) failed to ack stop workspace message: %v", stopWsMsg.ID, err)
		}
	}()

	logger.Debugf("(workspace: %d) stopping workspace %d", nodeId, stopWsMsg.ID)

	// create context for the stop operation
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)

	// defer cancel to ensure we don't leak resources
	defer cancel()

	// set the state that will be recorded in the database
	state := models.WorkspaceSuspended

	// stop the workspace via the remote provisioner
	err = wsClient.StopWorkspace(ctx, stopWsMsg.ID)
	if err != nil {
		if errors.Is(err, ws.ErrAlternativeRequestActive) {
			// another node is handling the stop request so skip
			return
		} else if errors.Is(err, ws.ErrWorkspaceNotFound) {
			state = models.WorkspaceFailed
			logger.Warnf("(workspace: %d) failed stop workspace %d - not found", nodeId, stopWsMsg.ID)
		} else {
			logger.Warnf("(workspace: %d) failed stop workspace %d: %v", nodeId, stopWsMsg.ID, err)
		}
		return
	}
	if stopWsMsg.WorkspaceFailed {
		state = models.WorkspaceFailed
	}

	// mark workspace as stopped and update the expiration to now
	// so that the workspace volume won't be destroyed for 24 hours
	_, err = tidb.ExecContext(ctx, &span, &callerName,
		"update workspaces set state = ?, last_state_update = ? where _id = ?", state, time.Now(), stopWsMsg.ID,
	)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to update workspace table for suspended: %v", nodeId, err)
		return
	}

	// send workspace status update
	wsStatusUpdater.PushStatus(ctx, stopWsMsg.ID, nil)

	// query for the workspace owner's ephemeral state
	var ephemeralUser bool
	err = tidb.QueryRowContext(ctx, &span, &callerName, "select is_ephemeral from users where _id = ?", stopWsMsg.OwnerID).Scan(&ephemeralUser)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to query user ephemeral state: %v", stopWsMsg.ID, err)
		return
	}

	// conditionally stop the workspace in the streak system if this is a real user
	if !ephemeralUser {
		logger.Debugf("(workspace: %d) executing stop streak workspace with user: %v", stopWsMsg.ID, stopWsMsg.OwnerID, err)
		err = streakEngine.UserStopWorkspace(ctx, stopWsMsg.OwnerID)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to stop workspace streak engine for user: %v", stopWsMsg.ID, stopWsMsg.OwnerID, err)
			return
		}
	}

	logger.Debugf("(workspace: %d) finished executing stop streak workspace with user: %v", stopWsMsg.ID, stopWsMsg.OwnerID, err)
}

// asyncDestroyWorkspace
//
//	Handles the destruction of an existing workspace via the remote provisioner
//	system (gigo-ws) and updates the database directly on completion or failure
func asyncDestroyWorkspace(nodeId int64, tidb *ti.Database, wsClient *ws.WorkspaceClient, vcsClient *git.VCSClient,
	wsStatusUpdater *utils.WorkspaceStatusUpdater, js *mq.JetstreamClient, rdb redis.UniversalClient, msg *nats.Msg,
	zitiManager *zitimesh.Manager, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "async-destroy-workspace-routine")
	defer span.End()
	callerName := "asyncDestroyWorkspace"

	// unmarshall destroy workspace message
	var destroyWsMsg models2.DestroyWorkspaceMsg
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&destroyWsMsg)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to decode destroy workspace message: %v", nodeId, err)
		return
	}

	// log that we are destroying the workspace
	logger.Infof("(workspace: %d) destroying workspace %d", nodeId, destroyWsMsg.ID)

	// always ack the message since the leader will retry if we fail
	defer func() {
		// acknowledge that the workspace is now destroyed
		err = msg.Ack()
		if err != nil {
			logger.Errorf("(workspace: %d) failed to ack destroy workspace message: %v", nodeId, err)
		}
	}()

	logger.Debugf("destroying workspace %d", destroyWsMsg.ID)

	// query for the workspace CodeSource type
	var projectType models.CodeSource
	err = tidb.QueryRowContext(ctx, &span, &callerName, "select code_source_type from workspaces where _id = ?", destroyWsMsg.ID).Scan(&projectType)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to query workspace CodeSource type: %v", destroyWsMsg.ID, err)
		return
	}

	if projectType == models.CodeSourcePost || projectType == models.CodeSourceAttempt {
		// delete access token to vcs for the user
		gitRes, err := vcsClient.GiteaClient.DeleteAccessTokenAdmin(fmt.Sprintf("%d", destroyWsMsg.OwnerID), fmt.Sprintf("%d", destroyWsMsg.ID))
		if err != nil && gitRes.StatusCode != http.StatusNotFound {
			logger.Warnf("(workspace: %d) failed to delete workspace vcs access token while deleting workspace: %v", nodeId, err)
			return
		}
	}

	// create context for the stop operation
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)

	// defer cancel to ensure we don't leak resources
	defer cancel()

	// destroy the workspace via the remote provisioner
	err = wsClient.DestroyWorkspace(ctx, destroyWsMsg.ID)
	if err != nil {
		// if the workspace is not found then we have no problem and if there's an alternative request then
		// another node is handling the destroy request so we can skip
		if !errors.Is(err, ws.ErrAlternativeRequestActive) && errors.Is(err, ws.ErrWorkspaceNotFound) {
			logger.Errorf("(workspace: %d) failed destroy workspace %d: %v", nodeId, destroyWsMsg.ID, err)
			return
		}
	}

	// mark workspace as deleted
	_, err = tidb.ExecContext(ctx, &span, &callerName,
		"update workspaces set state = ?, last_state_update = ? where _id = ?",
		models.WorkspaceDeleted, time.Now(), destroyWsMsg.ID,
	)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to update workspace table for deleted: %v", nodeId, err)
		return
	}

	// send workspace status update
	wsStatusUpdater.PushStatus(ctx, destroyWsMsg.ID, nil)

	// retrieve all agents for this workspace
	res, err := tidb.QueryContext(ctx, &span, &callerName,
		"select _id from workspace_agent where workspace_id = ?",
		destroyWsMsg.ID,
	)
	if err != nil {
		logger.Errorf("(workspace: %d) failed to query for workspace agents: %v", nodeId, err)
		return
	}

	// iterate the agents (even though there should only be 1)
	// and delete the agents on the ziti mesh
	for res.Next() {
		// load the agent id from the cursor
		var id int64
		if err := res.Scan(&id); err != nil {
			logger.Errorf("(workspace: %d) failed to scan workspace agent id: %v", nodeId, err)
			continue
		}

		// remove the ziti agent from the mesh
		err = zitiManager.DeleteAgent(id)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to delete ziti agent: %v", nodeId, err)
		}
	}

	// remove any ephemeral users associated with the workspace
	err = core.DeleteEphemeral(ctx, tidb, vcsClient, rdb, []int64{destroyWsMsg.ID})
	if err != nil {
		logger.Error(fmt.Errorf("(workspace: %d) failed to delete ephemeral workspaces: %v", nodeId, err))
	}
}

// asyncRemoveDestroyed
//
//		Handles the deletion of destroyed workspaces from the database that have been
//	 destroyed for more than 24 hours. Unlike all the other helper functions for
//	 workspaces this function performs a bulk operations on all workspaces that
//	 have been destroyed for more than 24 hours.
func asyncRemoveDestroyed(nodeId int64, tidb *ti.Database, vcsClient *git.VCSClient, msg *nats.Msg, logger logging.Logger) {
	ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "async-remove-destroyed-routine")
	defer span.End()
	callerName := "asyncRemoveDestroyed"

	// always ack the message since the leader will retry if we fail
	defer func() {
		// acknowledge that the workspaces are now deleted
		err := msg.Ack()
		if err != nil {
			logger.Errorf("(workspace: %d) failed to ack delete workspaces message: %v", nodeId, err)
			return
		}
	}()

	// query for workspaces that have been removed for more than a day
	query := "select _id from workspaces where state = ? and last_state_update < ?"
	res, err := tidb.QueryContext(ctx, &span, &callerName, query, models.WorkspaceDeleted, time.Now().Add(-time.Hour*24))
	if err != nil {
		logger.Error(fmt.Sprintf("(workspace: %d) failed to query deleted workspaces: %v\n    query: %s\n    params: %v",
			nodeId, err, query, []interface{}{models.WorkspaceDeleted, time.Now().Add(-time.Hour * 24)}))
	}

	// defer closure of cursor
	defer res.Close()

	// create slice to hold id for workspaces needing removal
	wsIds := make([]int64, 0)

	// iterate through workspaces loading ids
	for res.Next() {
		// create variable to hold workspace id
		var wsId int64

		err = res.Scan(&wsId)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to scan cursor data for workspace id: %v", nodeId, err)
			continue
		}

		// append id to outer slice
		wsIds = append(wsIds, wsId)
	}

	// iterate over workspace ids performing deletions
	for _, workspaceId := range wsIds {
		// open tx for deletion
		tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			logger.Errorf("(workspace: %d) failed to start transaction for destroying workspace %d: %v", nodeId, workspaceId, err)
			continue
		}

		// delete the workspace and all of its agents from the database
		_, err = tidb.ExecContext(ctx, &span, &callerName, "delete from workspaces where _id = ?", workspaceId)
		if err != nil {
			_ = tx.Rollback()
			logger.Errorf("(workspace: %d) failed to delete workspace %d from table: %v", nodeId, workspaceId, err)
			continue
		}

		_, err = tidb.ExecContext(ctx, &span, &callerName, "delete from workspace_agent where workspace_id = ?", workspaceId)
		if err != nil {
			_ = tx.Rollback()
			logger.Errorf("(workspace: %d) failed to delete agents for workspace %d from table: %v", nodeId, workspaceId, err)
			continue
		}

		_, err = tidb.ExecContext(ctx, &span, &callerName, "delete from workspace_agent_stats where workspace_id = ?", workspaceId)
		if err != nil {
			_ = tx.Rollback()
			logger.Errorf("(workspace: %d) failed to delete agents for workspace %d from table: %v", nodeId, workspaceId, err)
			continue
		}

		// commit tx
		err = tx.Commit(&callerName)
		if err != nil {
			_ = tx.Rollback()
			logger.Errorf("(workspace: %d) failed to commit transaction for destroying workspace %d: %v", nodeId, workspaceId, err)
			continue
		}
	}

	return
}

func WorkspaceManagementOperations(ctx context.Context, nodeId int64, tidb *ti.Database, wsClient *ws.WorkspaceClient, vcsClient *git.VCSClient,
	js *mq.JetstreamClient, workerPool *pool.Pool, streakEngine *streak.StreakEngine, wsStatusUpdater *utils.WorkspaceStatusUpdater, rdb redis.UniversalClient,
	zitiManager *zitimesh.Manager, logger logging.Logger) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "workspace-management-operations-routine")
	defer parentSpan.End()

	// all errors related to jetstream operations should be logged
	// and them trigger an exit since it most likely is a network
	// issue that will persist and should be addressed by devops

	// process workspace create stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamWorkspace,
		streams.SubjectWorkspaceCreate,
		"gigo-core-follower-ws-create",
		time.Minute*10,
		"workspace",
		logger,
		func(msg *nats.Msg) {
			asyncCreateWorkspace(nodeId, tidb, wsClient, wsStatusUpdater, js, msg, zitiManager, logger)
		},
	)

	// process workspace start stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamWorkspace,
		streams.SubjectWorkspaceStart,
		"gigo-core-follower-ws-start",
		time.Minute*10,
		"workspace",
		logger,
		func(msg *nats.Msg) {
			asyncStartWorkspace(nodeId, tidb, wsClient, wsStatusUpdater, js, msg, logger)
		},
	)

	// process workspace stop stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamWorkspace,
		streams.SubjectWorkspaceStop,
		"gigo-core-follower-ws-stop",
		time.Minute*10,
		"workspace",
		logger,
		func(msg *nats.Msg) {
			asyncStopWorkspace(nodeId, tidb, wsClient, js, streakEngine, wsStatusUpdater, msg, logger)
		},
	)

	// process workspace destroy stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamWorkspace,
		streams.SubjectWorkspaceDestroy,
		"gigo-core-follower-ws-destroy",
		time.Minute*10,
		"workspace",
		logger,
		func(msg *nats.Msg) {
			asyncDestroyWorkspace(nodeId, tidb, wsClient, vcsClient, wsStatusUpdater, js, rdb, msg, zitiManager, logger)
		},
	)

	// process workspace delete stream
	processStream(
		nodeId,
		js,
		workerPool,
		streams.StreamWorkspace,
		streams.SubjectWorkspaceDelete,
		"gigo-core-follower-ws-delete",
		time.Minute*10,
		"workspace",
		logger,
		func(msg *nats.Msg) {
			asyncRemoveDestroyed(nodeId, tidb, vcsClient, msg, logger)
		},
	)

	parentSpan.AddEvent(
		"workspace-management-operations-routine",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)
}
