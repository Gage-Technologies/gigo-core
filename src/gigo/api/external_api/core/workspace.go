package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gage-technologies/gigo-lib/openvsx"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"golang.org/x/mod/semver"

	"gigo-core/gigo/utils"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/mq"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/workspace_config"
	"github.com/google/uuid"
	"github.com/jinzhu/now"
	"gopkg.in/yaml.v3"
)

// TODO: needs testing
// TODO: handle workspace ops through follower routines using jetstream to submit jobs

func CreateWorkspace(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, js *mq.JetstreamClient, snowflakeNode *snowflake.Node,
	wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User, accessUrl string, repo int64, commit string, csId int64,
	csType models.CodeSource, hostname string, tls bool) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-workspace-core")
	defer span.End()
	callerName := "CreateWorkspace"

	// attempt to retrieve any existing workspaces
	res, err := tidb.QueryContext(ctx, &span, &callerName,
		"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and code_source_id = ? and state not in (?, ?, ?) limit 1",
		repo, commit, callingUser.ID, csId, models.WorkspaceRemoving, models.WorkspaceDeleted, models.WorkspaceFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing workspace: %v\n    query: %s\n    params: %v", err,
			"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and code_source_id = ? and state not in (?, ?, ?) limit 1",
			[]interface{}{repo, commit, callingUser.ID, csId, models.WorkspaceRemoving, models.WorkspaceDeleted, models.WorkspaceFailed})
	}

	// ensure the closure of the cursor
	defer res.Close()

	// handle case that a workspace is already live
	if res.Next() {
		// attempt to load values from the cursor
		workspace, err := models.WorkspaceFromSQLNative(res)
		if err != nil {
			return nil, fmt.Errorf("failed to load workspace from cursor: %v", err)
		}

		// update expiration
		workspace.Expiration = time.Now().Add(time.Minute * 30)

		// handle non-live workspaces by performing a start transition
		if workspace.State != models.WorkspaceActive && workspace.State != models.WorkspaceStarting {
			// update workspace in tidb using a tx
			tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to start transaction for workspace start: %v", err)
			}

			// update the workspace state
			workspace.State = models.WorkspaceStarting
			workspace.InitState = -1
			workspace.LastStateUpdate = time.Now()

			_, err = tx.ExecContext(ctx, &callerName,
				"update workspaces set expiration = ?, state = ?, init_state = -1, last_state_update = ? where _id = ?",
				workspace.Expiration, workspace.State, workspace.LastStateUpdate, workspace.ID,
			)
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to update workspace in database: %v", err)
			}

			// format start workspace request and marshall it with gob
			buf := bytes.NewBuffer(nil)
			encoder := gob.NewEncoder(buf)
			err = encoder.Encode(models2.StartWorkspaceMsg{
				ID: workspace.ID,
			})
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to encode workspace: %v", err)
			}

			// send workspace start message to jetstream so a follower will
			// start the workspace
			_, err = js.PublishAsync(streams.SubjectWorkspaceStart, buf.Bytes())
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to send workspace start message to jetstream: %v", err)
			}

			// commit update tx
			err = tx.Commit(&callerName)
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to commit transaction for workspace start: %v", err)
			}
		} else {
			// update workspace in tidb
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update workspaces set expiration = ? where _id = ?", workspace.Expiration, workspace.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to update expiration for active workspace in tidb: %v", err)
			}
		}

		// get repository name from repo id
		repository, _, err := vcsClient.GiteaClient.GetRepoByID(repo)
		if err != nil {
			return nil, fmt.Errorf("failed to locate repo %d: %v", repo, err)
		}

		// retrieve the gigo workspace config for this repo and commit
		configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
			fmt.Sprintf("%d", callingUser.ID),
			repository.Name,
			commit,
			".gigo/workspace.yaml",
		)
		if err != nil {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
		}

		// parse config bytes into workspace config
		var gigoConfig workspace_config.GigoWorkspaceConfig
		err = yaml.Unmarshal(configBytes, &gigoConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse new config: %v", err)
		}

		if workspace.CodeSourceType == 1 {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update attempt set updated_at = ? where _id = ?", time.Now(), workspace.CodeSourceID)
			if err != nil {
				return nil, fmt.Errorf("failed to update row for attempt project information: %v", err)
			}
		} else {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update post set updated_at = ? where _id = ?", time.Now(), workspace.CodeSourceID)
			if err != nil {
				return nil, fmt.Errorf("failed to update row for attempt project information: %v", err)
			}
		}

		// push workspace status update to subscribers
		wsStatusUpdater.PushStatus(ctx, workspace.ID, workspace)

		return map[string]interface{}{
			"message":       "Workspace Created Successfully",
			"workspace_url": fmt.Sprintf("/editor/%d/%d-%s?folder=%s", callingUser.ID, workspace.ID, commit, url.QueryEscape(gigoConfig.WorkingDirectory)),
			"workspace":     workspace.ToFrontend(hostname, tls),
		}, nil
	}

	// close cursor explicitly
	_ = res.Close()

	// create variable to hold workspace settings bytes
	var wsSettingsBytes []byte

	// confirm that passed code source values are valid and that the calling user is the owner
	if csType == models.CodeSourcePost {
		// query posts for the passed id and user
		err := tidb.QueryRowContext(ctx, &span, &callerName,
			"select _id, workspace_settings from post where _id = ? and author_id = ? limit 1", csId, callingUser.ID,
		).Scan(&csId, &wsSettingsBytes)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{"message": "Unable to locate code source."}, fmt.Errorf("code source not found")
			}
			return nil, fmt.Errorf(
				"failed to query for existing post: %v\n    query: %s\n    params: %v",
				err, "select _id from post where _id = ? and author_id = ? limit 1", []interface{}{csId, callingUser.ID})
		}
		tidb.ExecContext(ctx, &span, &callerName, "update post set updated_at =? where _id =? and author_id = ? limit 1", time.Now(), csId, callingUser.ID)

	} else {
		// query attempts for the passed id and user
		err := tidb.QueryRowContext(ctx, &span, &callerName,
			"select _id, workspace_settings from attempt where _id = ? and author_id = ? limit 1", csId, callingUser.ID,
		).Scan(&csId, &wsSettingsBytes)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{"message": "Unable to locate code source."}, fmt.Errorf("code source not found")
			}
			return nil, fmt.Errorf(
				"failed to query for existing attempt: %v\n    query: %s\n    params: %v",
				err, "select _id from attempt where _id = ? and author_id = ? limit 1", []interface{}{csId, callingUser.ID})
		}
		tidb.ExecContext(ctx, &span, &callerName, "update attempt set updated_at =? where _id =? and author_id = ? limit 1", time.Now(), csId, callingUser.ID)
	}

	// decode workspace settings bytes or select user defaults if there are no code source specific settings
	var wsSettings models.WorkspaceSettings
	if wsSettingsBytes != nil {
		err := json.Unmarshal(wsSettingsBytes, &wsSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to decode workspace settings: %v", err)
		}
	} else {
		if callingUser.WorkspaceSettings == nil {
			return nil, fmt.Errorf("user workspace settings are not set")
		}
		wsSettings = *callingUser.WorkspaceSettings
	}

	// get repository name from repo id
	repository, _, err := vcsClient.GiteaClient.GetRepoByID(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to locate repo %d: %v", repo, err)
	}

	// retrieve the gigo workspace config from the passed branch
	configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", callingUser.ID),
		repository.Name,
		commit,
		".gigo/workspace.yaml",
	)
	if err != nil {
		if gitRes.StatusCode != http.StatusNotFound {
			return map[string]interface{}{
				"message": "Make sure that there is a valid Workspace Configuration file (.gigo/workspace.yaml) in the " +
					"repository. If you're unsure how to create a workspace configuration try our interactive " +
					"configuration editor!",
			}, fmt.Errorf("workspace config not found")
		}
		buf, _ := io.ReadAll(gitRes.Body)
		return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
	}

	// ensure that the content is present
	if len(configBytes) == 0 {
		return map[string]interface{}{
			"message": "This repository does not have a valid Workspace Configuration file (.gigo/workspace.yaml) file",
		}, fmt.Errorf("workspace config is empty")
	}

	// parse bytes into workspace config
	var wsConfig workspace_config.GigoWorkspaceConfig
	err = yaml.Unmarshal(configBytes, &wsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new config: %v", err)
	}

	// create id for new workspace
	wsId := snowflakeNode.Generate().Int64()

	///////

	var workspaceConfigId int64

	if csType == models.CodeSourcePost {
		// query attempts for the passed id and user
		err = tidb.QueryRowContext(ctx, &span, &callerName,
			"select workspace_config from post where _id = ?", csId,
		).Scan(&workspaceConfigId)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{"message": "Unable to locate ws config in post."}, fmt.Errorf("ws config in post not found")
			}
			return nil, fmt.Errorf(
				"failed to query for ws config in post: %v\n    query: %v\n    params: %v",
				err, nil, []interface{}{csId, callingUser.ID})
		}
	} else {
		// query attempts for the passed id and user
		err = tidb.QueryRowContext(ctx, &span, &callerName,
			"select p.workspace_config from post p join attempt a on p._id = a.post_id where a._id = ?", csId,
		).Scan(&workspaceConfigId)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{"message": "Unable to locate ws config in post."}, fmt.Errorf("ws config in post not found")
			}
			return nil, fmt.Errorf(
				"failed to query for ws config in post: %v\n    query: %v\n    params: %v",
				err, nil, []interface{}{csId, callingUser.ID})
		}
	}

	if workspaceConfigId > 0 {
		var comparisonContent string
		var wsConfigRevision int64

		// query attempts for the passed id and user
		err = tidb.QueryRowContext(ctx, &span, &callerName,
			"select content, revision from workspace_config where _id = ? limit 1", workspaceConfigId,
		).Scan(&comparisonContent, &wsConfigRevision)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{"message": "Unable to locate ws config content."}, fmt.Errorf("ws config content not found")
			}
			return nil, fmt.Errorf(
				"failed to query for ws config content: %v\n    query: %v\n    params: %v",
				err, nil, []interface{}{csId, callingUser.ID})
		}

		// validate that the config is in the right format
		var wsCfgComparison workspace_config.GigoWorkspaceConfig
		err = yaml.Unmarshal([]byte(comparisonContent), &wsCfgComparison)
		if err != nil {
			return map[string]interface{}{"message": "config is not found"}, err
		}

		isSame := true

		if wsCfgComparison.Version != wsConfig.Version {
			isSame = false
		}

		if wsCfgComparison.BaseContainer != wsConfig.BaseContainer {
			isSame = false
		}

		if wsCfgComparison.WorkingDirectory != wsConfig.WorkingDirectory {
			isSame = false
		}

		if wsCfgComparison.Resources.CPU != wsConfig.Resources.CPU {
			isSame = false
		}

		if wsCfgComparison.Resources.Mem != wsConfig.Resources.Mem {
			isSame = false
		}

		if wsCfgComparison.Resources.Disk != wsConfig.Resources.Disk {
			isSame = false
		}

		if isSame {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "Update workspace_config SET completions = completions + 1 Where _id = ? and revision = ?", workspaceConfigId, wsConfigRevision)
			if err != nil {
				return nil, fmt.Errorf("failed to update workspace config uses: %v", err)
			}
		}
	}

	/////

	// create expiration for workspace
	expiration := time.Now().Add(time.Minute * 15)

	var overAllocated *models.OverAllocated = nil
	overAllocatedMessage := ""

	if callingUser.UserStatus != models.UserStatusPremium {
		if wsConfig.Resources.CPU > 2 {
			wsConfig.Resources.CPU = 2
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			overAllocatedMessage = fmt.Sprintf(
				"over-allocated CPUs: %d",
				wsConfig.Resources.CPU,
			)
		}
		if wsConfig.Resources.Mem > 3 {
			wsConfig.Resources.Mem = 3
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", RAM: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated RAM: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
		if wsConfig.Resources.Disk > 15 {
			wsConfig.Resources.Disk = 15
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", DISK: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated DISK: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
	} else {
		if wsConfig.Resources.CPU > 6 {
			wsConfig.Resources.CPU = 6
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			overAllocatedMessage = fmt.Sprintf(
				"over-allocated CPUs: %d",
				wsConfig.Resources.CPU,
			)
		}
		if wsConfig.Resources.Mem > 8 {
			wsConfig.Resources.Mem = 8
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", RAM: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated RAM: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
		if wsConfig.Resources.Disk > 50 {
			wsConfig.Resources.Disk = 50
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", DISK: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated DISK: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
	}

	// create a new workspace
	workspace, err := models.CreateWorkspace(
		wsId, repo, csId, csType, time.Now(), callingUser.ID, -1, expiration, commit,
		&wsSettings, overAllocated, []models.WorkspacePort{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace model: %v", err)
	}

	// set workspace init state to -1
	workspace.InitState = -1

	// format workspace for sql insertion
	wsInsertionStatements, err := workspace.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format workspace for insertion: %v", err)
	}

	// create tx for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx for workspace insertion: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// iterate over statements executing them in sql
	for _, statement := range wsInsertionStatements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// format create workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.CreateWorkspaceMsg{
		WorkspaceID: workspace.ID,
		OwnerID:     callingUser.ID,
		OwnerEmail:  callingUser.Email,
		OwnerName:   callingUser.UserName,
		Disk:        wsConfig.Resources.Disk,
		CPU:         wsConfig.Resources.CPU,
		Memory:      wsConfig.Resources.Mem,
		Container:   wsConfig.BaseContainer,
		AccessUrl:   accessUrl,
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to encode workspace: %v", err)
	}

	// send workspace create message to jetstream so a follower will
	// create the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceCreate, buf.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to send workspace create message to jetstream: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspace.ID, workspace)

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// increment init state before frontend to represent the step that we are working on
	workspace.InitState++

	return map[string]interface{}{
		"message":                "Workspace Created Successfully",
		"workspace_url":          fmt.Sprintf("/editor/%d/%d-%s?folder=%s", callingUser.ID, workspace.ID, commit, url.QueryEscape(wsConfig.WorkingDirectory)),
		"workspace":              workspace.ToFrontend(hostname, tls),
		"over_allocated":         overAllocated,
		"over_allocated_message": overAllocatedMessage,
	}, nil
}

func CreateEWorkspace(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, js *mq.JetstreamClient, snowflakeNode *snowflake.Node,
	wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User, accessUrl string, repo int64, commit string, csId int64,
	csType models.CodeSource, hostname string, tls bool, ip int64, challengeID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-workspace-core")
	defer span.End()
	callerName := "CreateWorkspace"

	res, err := tidb.QueryContext(ctx, &span, &callerName,
		"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and is_ephemeral = ? and code_source_id = ? and state > 1 limit 1",
		repo, commit, callingUser.ID, true, csId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing workspace: %v\n    query: %s\n    params: %v", err,
			"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and is_ephemeral = ? and code_source_id = ? and state > 1 limit 1",
			[]interface{}{repo, commit, callingUser.ID, true, csId})
	}

	for res.Next() {
		return map[string]interface{}{
			"message":       "Temporary Workspace Is Expired",
			"workspace_url": "",
			"workspace":     nil,
		}, nil
	}

	// attempt to retrieve any existing workspaces
	res, err = tidb.QueryContext(ctx, &span, &callerName,
		"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and is_ephemeral = ? and code_source_id = ? and state < 2 limit 1",
		repo, commit, callingUser.ID, true, csId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing workspace: %v\n    query: %s\n    params: %v", err,
			"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and is_ephemeral = ? and code_source_id = ? and state < 2 limit 1",
			[]interface{}{repo, commit, callingUser.ID, true, csId})
	}

	// ensure the closure of the cursor
	defer res.Close()

	// handle case that a workspace is already live
	if res.Next() {
		// attempt to load values from the cursor
		workspace, err := models.WorkspaceFromSQLNative(res)
		if err != nil {
			return nil, fmt.Errorf("failed to load workspace from cursor: %v", err)
		}

		// update expiration
		workspace.Expiration = time.Now().Add(time.Minute * 30)

		// update workspace in tidb
		_, err = tidb.ExecContext(ctx, &span, &callerName, "update workspaces set expiration = ? where _id = ?", workspace.Expiration, workspace.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update expiration for active workspace in tidb: %v", err)
		}

		// get repository name from repo id
		repository, _, err := vcsClient.GiteaClient.GetRepoByID(repo)
		if err != nil {
			return nil, fmt.Errorf("failed to locate repo %d: %v", repo, err)
		}

		// retrieve the gigo workspace config for this repo and commit
		configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
			fmt.Sprintf("%d", callingUser.ID),
			repository.Name,
			commit,
			".gigo/workspace.yaml",
		)
		if err != nil {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
		}

		// parse config bytes into workspace config
		var gigoConfig workspace_config.GigoWorkspaceConfig
		err = yaml.Unmarshal(configBytes, &gigoConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse new config: %v", err)
		}

		if workspace.CodeSourceType == 1 {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update attempt set updated_at = ? where _id = ?", time.Now(), workspace.CodeSourceID)
			if err != nil {
				return nil, fmt.Errorf("failed to update row for attempt project information: %v", err)
			}
		} else {
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update post set updated_at = ? where _id = ?", time.Now(), workspace.CodeSourceID)
			if err != nil {
				return nil, fmt.Errorf("failed to update row for attempt project information: %v", err)
			}
		}

		// push workspace status update to subscribers
		wsStatusUpdater.PushStatus(ctx, workspace.ID, workspace)

		return map[string]interface{}{
			"message":       "Workspace Created Successfully",
			"workspace_url": fmt.Sprintf("/editor/%d/%d-%s?folder=%s", callingUser.ID, workspace.ID, commit, url.QueryEscape(gigoConfig.WorkingDirectory)),
			"workspace":     workspace.ToFrontend(hostname, tls),
		}, nil
	}

	// close cursor explicitly
	_ = res.Close()

	// create variable to hold workspace settings bytes
	var wsSettingsBytes []byte

	// query attempts for the passed id and user
	err = tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id, workspace_settings from attempt where _id = ? and author_id = ? limit 1", csId, callingUser.ID,
	).Scan(&csId, &wsSettingsBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{"message": "Unable to locate code source."}, fmt.Errorf("code source not found")
		}
		return nil, fmt.Errorf(
			"failed to query for existing attempt: %v\n    query: %s\n    params: %v",
			err, "select _id from attempt where _id = ? and author_id = ? limit 1", []interface{}{csId, callingUser.ID})
	}
	tidb.ExecContext(ctx, &span, &callerName, "update attempt set updated_at =? where _id =? and author_id = ? limit 1", time.Now(), csId, callingUser.ID)

	// decode workspace settings bytes or select user defaults if there are no code source specific settings
	var wsSettings models.WorkspaceSettings
	if wsSettingsBytes != nil {
		err := json.Unmarshal(wsSettingsBytes, &wsSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to decode workspace settings: %v", err)
		}
	} else {
		if callingUser.WorkspaceSettings == nil {
			return nil, fmt.Errorf("user workspace settings are not set")
		}
		wsSettings = *callingUser.WorkspaceSettings
	}

	// get repository name from repo id
	repository, _, err := vcsClient.GiteaClient.GetRepoByID(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to locate repo %d: %v", repo, err)
	}

	// retrieve the gigo workspace config from the passed branch
	configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", callingUser.ID),
		repository.Name,
		commit,
		".gigo/workspace.yaml",
	)
	if err != nil {
		if gitRes.StatusCode != http.StatusNotFound {
			return map[string]interface{}{
				"message": "Make sure that there is a valid Workspace Configuration file (.gigo/workspace.yaml) in the " +
					"repository. If you're unsure how to create a workspace configuration try our interactive " +
					"configuration editor!",
			}, fmt.Errorf("workspace config not found")
		}
		buf, _ := io.ReadAll(gitRes.Body)
		return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
	}

	// ensure that the content is present
	if len(configBytes) == 0 {
		return map[string]interface{}{
			"message": "This repository does not have a valid Workspace Configuration file (.gigo/workspace.yaml) file",
		}, fmt.Errorf("workspace config is empty")
	}

	// parse bytes into workspace config
	var wsConfig workspace_config.GigoWorkspaceConfig
	err = yaml.Unmarshal(configBytes, &wsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new config: %v", err)
	}

	// create id for new workspace
	wsId := snowflakeNode.Generate().Int64()

	// create expiration for workspace
	expiration := time.Now().Add(time.Minute * 15)

	var overAllocated *models.OverAllocated = nil
	overAllocatedMessage := ""

	if callingUser.UserStatus != models.UserStatusPremium {
		if wsConfig.Resources.CPU > 2 {
			wsConfig.Resources.CPU = 2
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			overAllocatedMessage = fmt.Sprintf(
				"over-allocated CPUs: %d",
				wsConfig.Resources.CPU,
			)
		}
		if wsConfig.Resources.Mem > 3 {
			wsConfig.Resources.Mem = 3
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", RAM: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated RAM: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
		if wsConfig.Resources.Disk > 15 {
			wsConfig.Resources.Disk = 15
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", DISK: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated DISK: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
	} else {
		if wsConfig.Resources.CPU > 6 {
			wsConfig.Resources.CPU = 6
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			overAllocatedMessage = fmt.Sprintf(
				"over-allocated CPUs: %d",
				wsConfig.Resources.CPU,
			)
		}
		if wsConfig.Resources.Mem > 8 {
			wsConfig.Resources.Mem = 8
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", RAM: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated RAM: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
		if wsConfig.Resources.Disk > 50 {
			wsConfig.Resources.Disk = 50
			overAllocated = &models.OverAllocated{CPU: wsConfig.Resources.CPU, RAM: wsConfig.Resources.Mem, DISK: wsConfig.Resources.Disk}
			if overAllocatedMessage != "" {
				overAllocatedMessage = fmt.Sprintf(
					", DISK: %d",
					wsConfig.Resources.CPU,
				)
			} else {
				overAllocatedMessage = fmt.Sprintf(
					"over-allocated DISK: %d",
					wsConfig.Resources.CPU,
				)
			}
		}
	}

	// create a new workspace
	workspace, err := models.CreateWorkspace(
		wsId, repo, csId, csType, time.Now(), callingUser.ID, -1, expiration, commit,
		&wsSettings, overAllocated, []models.WorkspacePort{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace model: %v", err)
	}

	// set workspace init state to -1
	workspace.InitState = -1

	workspace.IsEphemeral = true

	// format workspace for sql insertion
	wsInsertionStatements, err := workspace.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format workspace for insertion: %v", err)
	}

	// create tx for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx for workspace insertion: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// iterate over statements executing them in sql
	for _, statement := range wsInsertionStatements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// format create workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.CreateWorkspaceMsg{
		WorkspaceID: workspace.ID,
		OwnerID:     callingUser.ID,
		OwnerEmail:  callingUser.Email,
		OwnerName:   callingUser.UserName,
		Disk:        wsConfig.Resources.Disk,
		CPU:         wsConfig.Resources.CPU,
		Memory:      wsConfig.Resources.Mem,
		Container:   wsConfig.BaseContainer,
		AccessUrl:   accessUrl,
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to encode workspace: %v", err)
	}

	// send workspace create message to jetstream so a follower will
	// create the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceCreate, buf.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to send workspace create message to jetstream: %v", err)
	}

	_, err = tx.ExecContext(ctx, &callerName, "INSERT INTO ephemeral_shared_workspaces(workspace_id, ip, date, user_id, challenge_id) values(?, ?, ?, ?, ?)", workspace.ID, ip, time.Now(), callingUser.ID, challengeID)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to insert shared workspace: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspace.ID, workspace)

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// increment init state before frontend to represent the step that we are working on
	workspace.InitState++

	return map[string]interface{}{
		"message":                "Workspace Created Successfully",
		"workspace_url":          fmt.Sprintf("/editor/%d/%d-%s?folder=%s", callingUser.ID, workspace.ID, commit, url.QueryEscape(wsConfig.WorkingDirectory)),
		"workspace":              workspace.ToFrontend(hostname, tls),
		"over_allocated":         overAllocated,
		"over_allocated_message": overAllocatedMessage,
	}, nil
}

func StartWorkspace(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User,
	workspaceID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "start-workspace-core")
	defer span.End()
	callerName := "StartWorkspace"

	// mark workspace as starting using a tx to ensure we get the jetstream
	// message off
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx for workspace start: %v", err)
	}

	// defer rollback in case we fail
	defer tx.Rollback()

	// perform update via the tx
	res, err := tx.ExecContext(ctx, &callerName,
		"update workspaces set expiration = ?, state = ?, init_state = -1, last_state_update = ? where _id = ? and owner_id = ?",
		time.Now().Add(time.Minute*30), models.WorkspaceStarting, time.Now(), workspaceID, callingUser.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update workspace in database: %v", err)
	}

	// return not found if no rows were updated
	updated, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %v", err)
	}
	if updated == 0 {
		return nil, fmt.Errorf("not found")
	}

	result, err := tx.QueryContext(ctx, &callerName, "select code_source_id, code_source_type from workspaces where _id = ? and owner_id = ? limit 1", workspaceID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to info of project: %v", err)
	}

	var projectID int64
	var projectType int
	for result.Next() {
		err = result.Scan(&projectID, &projectType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row for project information: %v", err)
		}
	}

	if projectType == 1 {
		_, err = tx.ExecContext(ctx, &callerName, "update attempt set updated_at = ? where _id = ?", time.Now(), projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to update row for attempt project information: %v", err)
		}
	} else {
		_, err = tx.ExecContext(ctx, &callerName, "update post set updated_at = ? where _id = ?", time.Now(), projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to update row for attempt project information: %v", err)
		}
	}

	// format start workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.StartWorkspaceMsg{
		ID: workspaceID,
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to encode workspace: %v", err)
	}

	// send workspace start message to jetstream so a follower will
	// start the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceStart, buf.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to send workspace start message to jetstream: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspaceID, nil)

	return map[string]interface{}{
		"message": "Workspace is starting.",
	}, nil
}

func StopWorkspace(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User,
	workspaceID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "stop-workspace-core")
	defer span.End()
	callerName := "StopWorkspace"

	// mark workspace as starting using a tx to ensure we get the jetstream
	// message off
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx for workspace start: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// perform update via the tx
	res, err := tx.ExecContext(ctx, &callerName,
		"update workspaces set state = ?, last_state_update = ? where _id = ? and owner_id =?",
		models.WorkspaceStopping, time.Now(), workspaceID, callingUser.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update workspace table for stopping: %v", err)
	}

	// return not found if no rows were updated
	updated, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %v", err)
	}
	if updated == 0 {
		return nil, fmt.Errorf("not found")
	}

	// format stop workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.StopWorkspaceMsg{
		ID:      workspaceID,
		OwnerID: callingUser.ID,
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to encode workspace: %v", err)
	}

	// send workspace stop message to jetstream so a follower will
	// stop the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceStop, buf.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to send workspace stop message to jetstream: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspaceID, nil)

	return map[string]interface{}{
		"message": "Workspace is stopping.",
	}, nil
}

func DestroyWorkspace(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User,
	workspaceID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "destroy-workspace-core")
	defer span.End()
	callerName := "DestroyWorkspace"

	// mark workspace as starting using a tx to ensure we get the jetstream
	// message off
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx for workspace start: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// perform update via the tx
	res, err := tx.ExecContext(ctx, &callerName,
		"update workspaces set state = ?, last_state_update = ? where _id = ? and owner_id =?",
		models.WorkspaceRemoving, time.Now(), workspaceID, callingUser.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update workspace table for stopping: %v", err)
	}

	// return not found if no rows were updated
	updated, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %v", err)
	}
	if updated == 0 {
		return nil, fmt.Errorf("not found")
	}

	// format destroy workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.DestroyWorkspaceMsg{
		ID:      workspaceID,
		OwnerID: callingUser.ID,
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to encode workspace: %v", err)
	}

	// send workspace destroy message to jetstream so a follower will
	// destroy the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceDestroy, buf.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to send workspace destroy message to jetstream: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspaceID, nil)

	return map[string]interface{}{
		"message": "Workspace is destroying.",
	}, nil
}

func GetWorkspaceStatus(ctx context.Context, db *ti.Database, vcsClient *git.VCSClient, callingUser *models.User, id int64, hostname string, tls bool) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-workspace-status-core")
	defer span.End()
	callerName := "GetWorkspaceStatus"

	// query database for workspace
	res, err := db.QueryContext(ctx, &span, &callerName, "select _id, code_source_id, code_source_type, repo_id, created_at, owner_id, expiration, commit, state, init_state, init_failure, ports, is_vnc from workspaces where _id = ? and owner_id = ?", id, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workspace: %v", err)
	}

	// defer closure of rows
	defer res.Close()

	// load workspace into first position
	if !res.Next() {
		return map[string]interface{}{"message": "Unable to locate the workspace."}, fmt.Errorf("workspace not found")
	}

	// load workspace from cursor
	workspace, err := models.WorkspaceFromSQLNative(res)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace from cursor: %v", err)
	}

	// close cursor
	_ = res.Close()

	// get repository name from repo id
	repository, _, err := vcsClient.GiteaClient.GetRepoByID(workspace.RepoID)
	if err != nil {
		return nil, fmt.Errorf("failed to locate repo %d: %v", workspace.RepoID, err)
	}

	// retrieve the gigo workspace config for this repo and commit
	configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", callingUser.ID),
		repository.Name,
		workspace.Commit,
		".gigo/workspace.yaml",
	)
	if err != nil {
		buf, _ := io.ReadAll(gitRes.Body)
		return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
	}

	// parse config bytes into workspace config
	var gigoConfig workspace_config.GigoWorkspaceConfig
	err = yaml.Unmarshal(configBytes, &gigoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new config: %v", err)
	}

	// create variable to hold code source name
	var csName string

	// query for code source data
	if workspace.CodeSourceType == models.CodeSourcePost {
		err := db.QueryRowContext(ctx, &span, &callerName, "select title from post where _id = ? limit 1", workspace.CodeSourceID).Scan(&csName)
		if err != nil {
			return nil, fmt.Errorf("failed to query posts for name: %v", err)
		}
	} else {
		err := db.QueryRowContext(ctx, &span, &callerName, "select p.title from attempt a join post p on a.post_id = p._id where a._id = ? limit 1", workspace.CodeSourceID).Scan(&csName)
		if err != nil {
			return nil, fmt.Errorf("failed to query attempts for name: %v", err)
		}
	}

	// increment init state before frontend to represent the step that we are working on
	// unless we are finished then we return the true code
	if workspace.InitState != models.WorkspaceInitCompleted {
		workspace.InitState++
	}

	return map[string]interface{}{
		"workspace":     workspace.ToFrontend(hostname, tls),
		"workspace_url": fmt.Sprintf("/editor/%d/%d-%s?folder=%s", callingUser.ID, workspace.ID, workspace.Commit, url.QueryEscape(gigoConfig.WorkingDirectory)),
		"code_source": map[string]interface{}{
			"_id":         fmt.Sprintf("%d", workspace.CodeSourceID),
			"type":        workspace.CodeSourceType,
			"type_string": workspace.CodeSourceType.String(),
			"name":        csName,
		},
	}, nil
}

func StreakCheck(ctx context.Context, db *ti.Database, workspaceID int64, secret uuid.UUID) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "streak-check-core")
	defer span.End()
	callerName := "StreakCheck"

	weekDaysActive := make(map[string]bool)

	rows, err := db.QueryContext(ctx, &span, &callerName,
		"SELECT u.* from workspace w join workspace_agent a on a.workspace_id = w._id join user_daily_usage u on w.owner_id = u.user_id where w._id = ? and a.secret = uuid_to_bin(?) and u.date = ? and end_time = NULL ",
		workspaceID, secret, now.BeginningOfDay().Format("2006-January-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily user data: %v", err)
	}

	defer rows.Close()

	var id int64
	var startTime time.Time
	var endTime time.Time
	var userID int64
	var date string
	var openSession int

	for rows.Next() {

		err = rows.Scan(&id, &userID, &startTime, &endTime, &openSession, &date)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user daily usage data: %v", err)
		}

		if date != now.BeginningOfDay().Format("2006-January-02") {
			return nil, fmt.Errorf("failed to scan user daily usage data: %v", errors.New("no matching rows found"))
		}

		break
	}

	// TODO: move this to agent initialization in agent sdk once things have stabilized

	sinceStart := time.Since(startTime)

	rows, err = db.QueryContext(ctx, &span, &callerName, "SELECT streak_active, current_streak, longest_streak, days_on_platform, days_on_fire from user_stats where user_id = ? and date = ?", userID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily user stats for user: %v for today, err: %v", userID, err)

	}
	defer rows.Close()

	var streakActive bool
	var currentStreak int
	var longestStreak int
	var daysOnPlatform int
	var daysOnFire int

	for rows.Next() {
		err = rows.Scan(&streakActive, &currentStreak, &longestStreak, &daysOnPlatform, &daysOnFire)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily user stats for user: %v, err: %v", userID, err)
		}
	}

	now.WeekStartDay = time.Monday

	startOfWeek := now.BeginningOfWeek().Format("2006-January-02")
	dateObj, err := time.Parse("2006-January-02", date)
	if err != nil {
		return nil, fmt.Errorf("failed to parse date: %v, err: %v", date, err)
	}

	if startOfWeek == date {
		if sinceStart >= (30 * time.Minute) {
			weekDaysActive["Monday"] = true
		} else {
			return map[string]interface{}{"message": "User has not been active for required time",
				"streak_active": streakActive, "streak_week": weekDaysActive, "current_streak_num": currentStreak,
				"longest_streak": longestStreak, "days_on_platform": daysOnPlatform, "days_on_fire": daysOnFire}, nil
		}

	} else {
		rows, err = db.QueryContext(ctx, &span, &callerName, "SELECT streak_active, date from user_stats where user_id = ? and date >= ? and date < ?", userID, now.BeginningOfWeek().Format("2006-January-02"), date)
		if err != nil {
			return nil, fmt.Errorf("failed to query daily user stats for user: %v from: %v - to: %v, err: %v", userID, now.BeginningOfWeek().Format("2006-January-02"), date, err)
		}

		defer rows.Close()

		for rows.Next() {
			var iterDate time.Time
			var iterStreakAcrtive bool
			err = rows.Scan(&iterStreakAcrtive, &iterDate)
			if err != nil {
				return nil, fmt.Errorf("failed to scan daily user stats for user: %v from: %v to: %v, err: %v", userID, now.BeginningOfWeek().Format("2006-January-02"), date, err)
			}
			if iterStreakAcrtive {
				weekDaysActive[iterDate.Weekday().String()] = true
			} else {
				weekDaysActive[iterDate.Weekday().String()] = false
			}
		}

		if sinceStart >= (30 * time.Minute) {
			weekDaysActive[dateObj.Weekday().String()] = true
		} else {
			return map[string]interface{}{"message": "User has not been active for required time",
				"streak_active": streakActive, "streak_week": weekDaysActive, "current_streak_num": currentStreak,
				"longest_streak": longestStreak, "days_on_platform": daysOnPlatform, "days_on_fire": daysOnFire}, nil
		}
	}

	// if 30 mins have passed and the streak is not already active
	if !streakActive {
		// set streak active
		streakActive = true
		// increase current streak
		currentStreak++
		// if current streak is the longest streak set it to longestStreak
		if currentStreak > longestStreak {
			longestStreak = currentStreak
		}
		// increase days on platform and total days on fire
		daysOnPlatform++
		daysOnFire++
		res, err := db.ExecContext(ctx, &span, &callerName, "update user_stats set streak_active = ?, current_streak = ?, longest_streak = ?, "+
			"days_on_platform = ?, days_on_fire = ? where user_id = ? and date = ?",
			streakActive, currentStreak, longestStreak, daysOnPlatform, daysOnFire, userID, date)
		if err != nil {
			return nil, fmt.Errorf("failed to update daily user stats for user: %v, err: %v", userID, err)
		}

		aff, _ := res.RowsAffected()

		if aff != 1 {
			return nil, fmt.Errorf("failed to update daily user stats for user: %v, err: %v", userID, errors.New("none or multiple rows were affected"))
		}

		return map[string]interface{}{"message": "User Streak Updated Sucessfully",
			"streak_active": streakActive, "streak_week": weekDaysActive, "current_streak_num": currentStreak,
			"longest_streak": longestStreak, "days_on_platform": daysOnPlatform, "days_on_fire": daysOnFire}, nil
	}
	return map[string]interface{}{"message": "User Already Has Streak Today",
		"streak_active": streakActive, "streak_week": weekDaysActive, "current_streak_num": currentStreak,
		"longest_streak": longestStreak, "days_on_platform": daysOnPlatform, "days_on_fire": daysOnFire}, nil

}

func ExtendExpiration(ctx context.Context, db *ti.Database, wsStatusUpdater *utils.WorkspaceStatusUpdater, workspaceID int64,
	secret uuid.UUID, js *mq.JetstreamClient) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "extend-expiration-core")
	defer span.End()
	callerName := "ExtendExpiration"

	// get the current expiration date from workspace
	err := db.QueryRowContext(ctx, &span, &callerName,
		"select w._id from workspaces w join workspace_agent a on a.workspace_id = w._id where a.secret = uuid_to_binary(?) and w._id = ? and w.state in (?, ?)",
		secret, workspaceID, models.WorkspaceStarting, models.WorkspaceActive,
	).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query workspaces: %v", err)
	}

	// add 10m to expiration
	expiration := time.Now().Add(time.Minute * 10)

	// extend the expiration deadline in tidb
	_, err = db.ExecContext(ctx, &span, &callerName, "update workspaces set expiration = ? where _id = ?", expiration, &workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to extend workspace expiration time in tidb: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspaceID, nil)

	return map[string]interface{}{"message": "Workspace Expiration Updated Successfully", "expiration": expiration.Unix()}, nil
}

func WorkspaceAFK(ctx context.Context, db *ti.Database, wsStatusUpdater *utils.WorkspaceStatusUpdater, workspaceID int64,
	secret uuid.UUID, addMin int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "workspace-afk-core")
	defer span.End()
	callerName := "WorkspaceAFK"

	// make sure that no more than 60 minutes have been added
	if addMin > 60 {
		return nil, fmt.Errorf("cannot add more than 60 minutes to afk timer")
	}

	// get the current expiration date from workspace
	err := db.QueryRowContext(ctx, &span, &callerName,
		"select w._id from workspaces w join workspace_agent a on a.workspace_id = w._id where a.secret = uuid_to_binary(?) and w._id = ? and w.state in (?, ?)",
		secret, workspaceID, models.WorkspaceStarting, models.WorkspaceActive,
	).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query workspaces: %v", err)
	}

	// calculate new expiration
	expiration := time.Now().Add(time.Minute * time.Duration(addMin))

	// extend the expiration deadline in tidb
	_, err = db.ExecContext(ctx, &span, &callerName, "update workspaces set expiration = ? where _id = ?", expiration, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to extend workspace expiration time in tidb: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspaceID, nil)

	return map[string]interface{}{"message": "Workspace AFK Time Extension Added Successfully", "expiration": expiration.Unix()}, nil
}

// WorkspaceInitializationStepCompleted
//
//	Updates the workspace initialization step on completion
//
//	WARNING: ACCESS TO THIS WORKSPACE MUST BE AUTHENTICATED BEFORE THIS FUNCTION IS CALLED
//	THIS FUNCTION PERFORMS NO AUTHENTICATION OR AUTHORIZATION ON THE WORKSPACE
func WorkspaceInitializationStepCompleted(ctx context.Context, tidb *ti.Database, wsStatusUpdater *utils.WorkspaceStatusUpdater, wsId int64,
	step models.WorkspaceInitState) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "workspace-initialization-step-completed-core")
	defer span.End()
	callerName := "WorkspaceInitializationStepCompleted"

	// default workspace state to starting but set to Active if this is the VSCodeLaunch init step
	state := models.WorkspaceStarting
	if step == models.WorkspaceInitCompleted {
		state = models.WorkspaceActive
	}

	// perform update directly on the workspace row
	_, err := tidb.ExecContext(ctx, &span, &callerName,
		"update workspaces set init_state = ?, state = ?, last_state_update = ? where _id = ? and state in (?, ?)",
		step, state, time.Now(), wsId, models.WorkspaceStarting, models.WorkspaceActive,
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

// WorkspaceInitializationFailure
//
//	Updates the workspace initialization state to show failure
//	and includes a log that can be used by users or developers
//	to understand why the workspace initialization failed
//
//	WARNING: ACCESS TO THIS WORKSPACE MUST BE AUTHENTICATED BEFORE THIS FUNCTION IS CALLED
//	THIS FUNCTION PERFORMS NO AUTHENTICATION OR AUTHORIZATION ON THE WORKSPACE
func WorkspaceInitializationFailure(ctx context.Context, tidb *ti.Database, wsStatusUpdater *utils.WorkspaceStatusUpdater, wsId int64,
	step models.WorkspaceInitState, command string, status int, stdout string, stderr string, js *mq.JetstreamClient) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "workspace-initialization-failure-core")
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

	// query for the owner of the workspace
	var ownerId int64
	err = tidb.QueryRowContext(ctx, &span, &callerName, "select owner_id from workspaces where _id = ?", wsId).Scan(&ownerId)
	if err != nil {
		return fmt.Errorf("failed to query workspace owner: %v", err)
	}

	// format stop workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.StopWorkspaceMsg{
		ID:              wsId,
		OwnerID:         ownerId,
		WorkspaceFailed: true,
	})
	if err != nil {
		return fmt.Errorf("failed to encode workspace stop message: %v", err)
	}

	// send workspace stop message to jetstream so a follower will
	// stop the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceStop, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send workspace stop message to jetstream: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, wsId, nil)

	return nil
}

func GetHighestScore(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-highest-score-core")
	defer span.End()
	callerName := "GetHighestScore"

	// query attempt and projects with the user id as author id and sort by date last edited
	res := tidb.QueryRowContext(ctx, &span, &callerName, "select highest_score from users where _id = ?", callingUser.ID)
	if res.Err() != nil {
		return nil, fmt.Errorf("failed to query for highestScore    Error: %v", res.Err())
	}

	var score int64

	err := res.Scan(&score)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for highestScore    Error: %v", res.Err())
	}

	finalScore := fmt.Sprintf("%v", score)

	return map[string]interface{}{"highest_score": finalScore}, nil
}

func SetHighestScore(ctx context.Context, tidb *ti.Database, callingUser *models.User, score int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "set-highest-score-core")
	defer span.End()
	callerName := "SetHighestScore"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)

	// execute the password update
	_, err = tx.ExecContext(ctx, &callerName, "update users set highest_score = ? where _id = ?", score, callingUser.ID)
	if err != nil {
		_ = tx.Rollback()
		return map[string]interface{}{
			"message": "failed to set the udpate query for highest score",
		}, fmt.Errorf("failed to execute query for highest score.    Error: %v", err)
	}

	// commit transaction
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return map[string]interface{}{
			"message": "failed to update highest score",
		}, errors.New("tx failed to commit")
	}

	return map[string]interface{}{
		"message": "success"}, nil
}

func CodeServerPullThroughCache(ctx context.Context, storageEngine storage.Storage, version, arch, os, installType string) (io.ReadCloser, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "code-server-pull-through-cache-core")
	defer span.End()

	// create the target url for the request
	var url string
	switch installType {
	case "tar":
		url = fmt.Sprintf(
			"https://github.com/gage-technologies/gigo-code-server/releases/download/v%s/code-server-%s-%s-%s.tar.gz",
			version, version, os, arch,
		)
	case "deb":
		url = fmt.Sprintf(
			"https://github.com/gage-technologies/gigo-code-server/releases/download/v%s/code-server_%s_%s.deb",
			version, version, arch,
		)
	case "rpm":
		url = fmt.Sprintf(
			"https://github.com/gage-technologies/gigo-code-server/releases/download/v%s/code-server-%s-%s.rpm",
			version, version, arch,
		)
	default:
		return nil, fmt.Errorf("invalid install type: %s", installType)
	}

	// create the request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// set the user agent
	req.Header.Set("User-Agent", "gigo-code-server-cache; contact@gigo.dev")

	// perform the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}

	// check the status code
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	// get the content length header and convert it to an int64
	contentLength, err := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content length: %v", err)
	}

	// as we stream the body to the client we also write it to the storage engine
	// so that we can cache it for future requests

	// create a concurrent buffer to buffer the stream from the http response to the storage engine
	relayBuffer := NewChannelBuffer(64 * 1024)

	go func() {
		// TODO: reconsider how to handle this error
		_ = storageEngine.CreateFileStreamed(
			fmt.Sprintf("ext/gigo-code-server-cache/%s-%s-%s-%s", version, arch, os, installType),
			contentLength, relayBuffer)
	}()

	// create a tee reader to write the stream to the storage engine and the client
	teeReader := NewTeeReadCloser(res.Body, relayBuffer)

	// return the response body
	return teeReader, nil
}

func OpenVsxPullThroughCache(ctx context.Context, storageEngine storage.Storage, rdb redis.UniversalClient, extensionId, version, vscVersion string) (io.ReadCloser, bool, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "openvsx-pull-through-cache-core")
	defer span.End()
	// validate the vscode version if it is specified
	if vscVersion != "" {
		checkVersion := vscVersion
		if !strings.HasPrefix(checkVersion, "v") {
			checkVersion = "v" + checkVersion
		}
		if !semver.IsValid(checkVersion) {
			return nil, false, fmt.Errorf("invalid vscode version: %s", vscVersion)
		}
	}

	// split into publisher and name
	extensionIdSplit := strings.Split(extensionId, ".")
	// this was validated in the http func so this should be safe
	publisher := extensionIdSplit[0]
	name := extensionIdSplit[1]

	// create a client for the openvsx api
	client := openvsx.NewClient("", nil)

	// query metadata and select the latest version if the version is not specified
	if version == "" || version == "latest" {
		// if we have a vscode version specified then check if there is a cached version pair
		if vscVersion != "" {
			var err error
			version, err = rdb.Get(ctx, fmt.Sprintf("vsc:ext:version:%s:%s:%s", publisher, name, vscVersion)).Result()
			if err != nil && err != redis.Nil {
				return nil, false, fmt.Errorf("failed to get extension version from redis: %v", err)
			}
		}

		// continue with api check for compatible version
		if version == "" || version == "latest" {
			// loop until we find the latest version compatible with the current version of vscode (if the vscode version is specified)
			v := ""
			it := 0
			versions := make([]string, 0)
			for {
				// increment the iteration
				it++

				// fail if we have checked more than 10 versions
				if it > 10 {
					return nil, false, fmt.Errorf("failed to find compatible extension version")
				}

				// retrieve metadata for this version
				ext, err := client.GetMetadata(extensionId, v)
				if err != nil {
					return nil, false, fmt.Errorf("failed to get extension metadata: %v", err)
				}

				// check if the extension is compatible with the current version of vscode
				if vscVersion != "" {
					// load the versions from the AllVersions map and sort them in reverse order if we haven't already
					if len(versions) == 0 {
						for k := range ext.AllVersions {
							// skip "latest"
							if k == "latest" {
								continue
							}
							versions = append(versions, k)
						}
						sort.Slice(versions, func(i, j int) bool {
							return semver.Compare("v"+versions[i], "v"+versions[j]) == 1
						})
					}

					// load the vscode version compatibility as a semver
					compatVersion, ok := ext.Engines["vscode"]
					if !ok {
						// update the version and fail if we don't have a next version
						if len(versions) <= it {
							return nil, false, fmt.Errorf("failed to find compatible extension version")
						}
						v = versions[it]
						continue
					}

					// skip the version if it is not compatible
					if !IsCompatible(strings.TrimPrefix(vscVersion, "v"), compatVersion) {
						// update the version
						if len(versions) <= it {
							return nil, false, fmt.Errorf("failed to find compatible extension version")
						}
						v = versions[it]
						continue
					}
				}

				// set the version to the current version
				version = ext.Version

				// break the loop
				break
			}
		}

		// once we know the version cache it if we have a vscode version specified
		if vscVersion != "" {
			// cache the version pair in redis for 24 hours
			err := rdb.Set(ctx, fmt.Sprintf("vsc:ext:version:%s:%s:%s", publisher, name, vscVersion), version, time.Hour*24).Err()
			if err != nil {
				return nil, false, fmt.Errorf("failed to cache extension version in redis: %v", err)
			}
		}
	}

	// check if we have the extension in the cache
	buf, err := storageEngine.GetFile(fmt.Sprintf("ext/open-vsx-cache/%s/%s.%s.vsix", publisher, name, version))
	if err != nil {
		return nil, false, fmt.Errorf("failed to get extension from cache: %v", err)
	}
	if buf != nil {
		return buf, false, nil
	}

	// begin the download of the extension
	extension, contentLength, err := client.DownloadExtension(extensionId, version)
	if err != nil {
		return nil, false, fmt.Errorf("failed to download extension: %v", err)
	}

	// as we stream the body to the client we also write it to the storage engine
	// so that we can cache it for future requests

	// create a concurrent buffer to buffer the stream from the http response to the storage engine
	relayBuffer := NewChannelBuffer(64 * 1024)

	go func() {
		// TODO: reconsider how to handle this error
		_ = storageEngine.CreateFileStreamed(
			fmt.Sprintf("ext/open-vsx-cache/%s/%s.%s.vsix", publisher, name, version),
			contentLength, relayBuffer)
	}()

	// create a tee reader to write the stream to the storage engine and the client
	teeReader := NewTeeReadCloser(extension, relayBuffer)

	// return the response body
	return teeReader, true, nil
}

func DeleteEphemeral(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, rdb redis.UniversalClient, deleteIds []int64) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "delete-ephemeral")
	defer span.End()
	callerName := "deleteEphemeral"

	ownerIds := make([]int64, 0)

	for _, id := range deleteIds {
		wsQuery := fmt.Sprintf("select code_source_id, owner_id from workspaces where _id = %d and is_ephemeral = true", id)

		res, err := tidb.QueryContext(ctx, &span, &callerName, wsQuery)
		if err != nil {
			return fmt.Errorf("failed to query workspaces for ephemeral id %v: %v", id, err)
		}

		var codeSourceId, ownerId int64
		if res.Next() {
			if err := res.Scan(&codeSourceId, &ownerId); err != nil {
				res.Close()
				return fmt.Errorf("failed to scan workspace results for id %v: %v", id, err)
			}

			// remove the repo from Gitea
			err = vcsClient.DeleteRepo(fmt.Sprintf("%d", ownerId), fmt.Sprintf("%d", codeSourceId))
			if err != nil {
				res.Close()
				return fmt.Errorf("failed to delete ephemeral repo from Gitea for id %v: %v", id, err)
			}

			// delete the attempt that is associated with the ephemeral id
			deleteAttemptQuery := fmt.Sprintf("delete from attempt where _id = %d", codeSourceId)

			_, err = tidb.ExecContext(ctx, &span, &callerName, deleteAttemptQuery)
			if err != nil {
				res.Close()
				return fmt.Errorf("failed to delete ephemeral attempt for id %v: %v", id, err)
			}

			// Append the owner ID to the list
			ownerIds = append(ownerIds, ownerId)
		}
		res.Close()
	}

	// use the owner ids to delete the ephemeral users
	_, err := DeleteEphemeralUser(ctx, tidb, vcsClient, rdb, ownerIds)
	if err != nil {
		return fmt.Errorf("failed to delete ephemeral users: %v", err)
	}

	return nil
}

func CreateByteWorkspace(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, snowflakeNode *snowflake.Node,
	wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User, accessUrl string, byteId int64,
	hostname string, tls bool) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-byte-workspace-core")
	defer span.End()
	callerName := "CreateByteWorkspace"

	// attempt to retrieve any existing workspaces
	res, err := tidb.QueryContext(ctx, &span, &callerName,
		"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and code_source_id = ? and state not in (?, ?, ?) limit 1",
		-1, "", callingUser.ID, byteId, models.WorkspaceRemoving, models.WorkspaceDeleted, models.WorkspaceFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing workspace: %v\n    query: %s\n    params: %v", err,
			"select * from workspaces where repo_id = ? and commit = ? and owner_id = ? and code_source_id = ? and state not in (?, ?, ?) limit 1",
			[]interface{}{-1, "", callingUser.ID, byteId, models.WorkspaceRemoving, models.WorkspaceDeleted, models.WorkspaceFailed})
	}

	// ensure the closure of the cursor
	defer res.Close()

	// handle case that a workspace is already live
	if res.Next() {
		// attempt to load values from the cursor
		workspace, err := models.WorkspaceFromSQLNative(res)
		if err != nil {
			return nil, fmt.Errorf("failed to load workspace from cursor: %v", err)
		}

		// update expiration
		workspace.Expiration = time.Now().Add(time.Minute * 30)

		// handle non-live workspaces by performing a start transition
		if workspace.State != models.WorkspaceActive && workspace.State != models.WorkspaceStarting {
			// update workspace in tidb using a tx
			tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to start transaction for workspace start: %v", err)
			}

			// update the workspace state
			workspace.State = models.WorkspaceStarting
			workspace.InitState = -1
			workspace.LastStateUpdate = time.Now()

			_, err = tx.ExecContext(ctx, &callerName,
				"update workspaces set expiration = ?, state = ?, init_state = -1, last_state_update = ? where _id = ?",
				workspace.Expiration, workspace.State, workspace.LastStateUpdate, workspace.ID,
			)
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to update workspace in database: %v", err)
			}

			// format start workspace request and marshall it with gob
			buf := bytes.NewBuffer(nil)
			encoder := gob.NewEncoder(buf)
			err = encoder.Encode(models2.StartWorkspaceMsg{
				ID: workspace.ID,
			})
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to encode workspace: %v", err)
			}

			// send workspace start message to jetstream so a follower will
			// start the workspace
			_, err = js.PublishAsync(streams.SubjectWorkspaceStart, buf.Bytes())
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to send workspace start message to jetstream: %v", err)
			}

			// commit update tx
			err = tx.Commit(&callerName)
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to commit transaction for workspace start: %v", err)
			}
		} else {
			// update workspace in tidb
			_, err = tidb.ExecContext(ctx, &span, &callerName, "update workspaces set expiration = ? where _id = ?", workspace.Expiration, workspace.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to update expiration for active workspace in tidb: %v", err)
			}
		}

		// push workspace status update to subscribers
		wsStatusUpdater.PushStatus(ctx, workspace.ID, workspace)

		return map[string]interface{}{
			"message":   "Workspace Created Successfully",
			"agent_url": fmt.Sprintf("/agent/%d/%d/ws", callingUser.ID, workspace.ID),
			"workspace": workspace.ToFrontend(hostname, tls),
		}, nil
	}

	// close cursor explicitly
	_ = res.Close()

	// select user defaults
	var wsSettings models.WorkspaceSettings

	if callingUser.WorkspaceSettings == nil {
		return nil, fmt.Errorf("user workspace settings are not set")
	}
	wsSettings = *callingUser.WorkspaceSettings

	// set the default workspace config for bytes
	wsConfig := workspace_config.GigoWorkspaceConfig{
		Version: 0.1,
		Resources: struct {
			CPU  int `yaml:"cpu"`
			Mem  int `yaml:"mem"`
			Disk int `yaml:"disk"`
			GPU  struct {
				Count int    `yaml:"count"`
				Class string `yaml:"class"`
			} `yaml:"gpu"`
		}{
			CPU:  1,  // number of CPUs
			Mem:  1,  // in GB
			Disk: 10, // in GB
			GPU: struct {
				Count int    `yaml:"count"`
				Class string `yaml:"class"`
			}{
				Count: 1,
				Class: "p4",
			},
		},
		BaseContainer:    "gigodev/gimg:bytes-base-ubuntu",
		WorkingDirectory: "/home/gigo/codebase/",
		Environment:      nil,
		Containers:       nil,
		VSCode:           workspace_config.GigoVSCodeConfig{},
		PortForward:      nil,
		Exec:             nil,
	}

	// create id for new workspace
	wsId := snowflakeNode.Generate().Int64()

	// create expiration for workspace
	expiration := time.Now().Add(time.Minute * 15)

	// create a new workspace
	workspace, err := models.CreateWorkspace(
		wsId, -1, byteId, models.CodeSourceByte, time.Now(), callingUser.ID, -1, expiration, "",
		&wsSettings, nil, []models.WorkspacePort{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace model: %v", err)
	}

	// set workspace init state to -1
	workspace.InitState = -1

	// format workspace for sql insertion
	wsInsertionStatements, err := workspace.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format workspace for insertion: %v", err)
	}

	// create tx for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx for workspace insertion: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// iterate over statements executing them in sql
	for _, statement := range wsInsertionStatements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// format create workspace request and marshall it with gob
	buf := bytes.NewBuffer(nil)
	encoder := gob.NewEncoder(buf)
	err = encoder.Encode(models2.CreateWorkspaceMsg{
		WorkspaceID: workspace.ID,
		OwnerID:     callingUser.ID,
		OwnerEmail:  callingUser.Email,
		OwnerName:   callingUser.UserName,
		Disk:        wsConfig.Resources.Disk,
		CPU:         wsConfig.Resources.CPU,
		Memory:      wsConfig.Resources.Mem,
		Container:   wsConfig.BaseContainer,
		AccessUrl:   accessUrl,
	})
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to encode workspace: %v", err)
	}

	// send workspace create message to jetstream so a follower will
	// create the workspace
	_, err = js.PublishAsync(streams.SubjectWorkspaceCreate, buf.Bytes())
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to send workspace create message to jetstream: %v", err)
	}

	// push workspace status update to subscribers
	wsStatusUpdater.PushStatus(ctx, workspace.ID, workspace)

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// increment init state before frontend to represent the step that we are working on
	workspace.InitState++

	return map[string]interface{}{
		"message":   "Workspace Created Successfully",
		"agent_url": fmt.Sprintf("/agent/%d/%d/ws", callingUser.ID, workspace.ID),
		"workspace": workspace.ToFrontend(hostname, tls),
	}, nil
}
