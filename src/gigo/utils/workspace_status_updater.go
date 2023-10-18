package utils

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"go.opentelemetry.io/otel"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
)

type WorkspaceStatusUpdaterOptions struct {
	Js       *mq.JetstreamClient
	DB       *ti.Database
	Hostname string
	Tls      bool
}

// WorkspaceStatusUpdater
//
//	Exposes a set of simple functions to push workspace
//	status updates to the workspace status jetstream.
//
//	NOTE: This struct was created to standardize the update
//	logic for the workspace status jetstream since the updates
//	can come from all across the system. This stuct should
//	be used for all workspace status updates to prevent
//	improper messages from being pushed to the status stream.
type WorkspaceStatusUpdater struct {
	WorkspaceStatusUpdaterOptions
}

// NewWorkspaceStatusUpdater
//
//	Creates a new instance of the WorkspaceStatusUpdater struct.
func NewWorkspaceStatusUpdater(options WorkspaceStatusUpdaterOptions) *WorkspaceStatusUpdater {
	return &WorkspaceStatusUpdater{
		WorkspaceStatusUpdaterOptions: options,
	}
}

// getWorkspace
//
//	Helper function to retrieve a workspace from the database
func (u *WorkspaceStatusUpdater) getWorkspace(ctx context.Context, id int64) (*models.Workspace, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-workspace-routine")
	callerName := "getWorkspace"

	// query database for the workspace
	res, err := u.DB.QueryContext(ctx, &span, &callerName, "select * from workspaces where _id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query for workspace %d: %v", id, err)
	}
	defer res.Close()

	// attempt to load workspace into first position of cursor
	if !res.Next() {
		return nil, fmt.Errorf("workspace %d not found", id)
	}

	// attempt to load the workspace from the cursor
	ws, err := models.WorkspaceFromSQLNative(res)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace %d: %v", id, err)
	}
	return ws, nil
}

// PushStatus
//
//	Pushes an update to the workspace status jetstream.
func (u *WorkspaceStatusUpdater) PushStatus(ctx context.Context, id int64, ws *models.Workspace) error {
	// retrieve the workspace if we did not receive one
	if ws == nil {
		var err error
		ws, err = u.getWorkspace(ctx, id)
		if err != nil {
			return err
		}
	}

	// increment init state before writing the update to represent the step that we
	// are working on unless we are finished then we return the true code
	if ws.InitState != models.WorkspaceInitCompleted {
		ws.InitState++
	}

	// gob encode the status update
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(models2.WorkspaceStatusUpdateMsg{
		Workspace: ws.ToFrontend(u.Hostname, u.Tls),
	})
	if err != nil {
		return fmt.Errorf("failed to serialize workspace status update: %v", err)
	}

	// send workspace status update message to jetstream so any listening websocket
	// connections can properly update the client for the current state of the workspace
	_, err = u.Js.PublishAsync(fmt.Sprintf(streams.SubjectWorkspaceStatusUpdateDynamic, ws.ID), buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send workspace status update message to jetstream: %v", err)
	}

	return nil
}
