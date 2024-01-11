package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/workspace_config"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
)

func EditorProxy(ctx context.Context, db *ti.Database, vcsClient *git.VCSClient, workspaceId int64, ownerId int64, workingDir bool) (int64, string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "editor-proxy-http")
	defer span.End()
	callerName := "EditorProxy"

	// create int64 to load agent id into
	agentId := int64(-1)
	codeSourceId := int64(-1)
	commit := ""

	// query database for the current workspace agent
	err := db.QueryRowContext(ctx, &span, &callerName,
		"select a._id, w.code_source_id, w.commit from workspaces w join workspace_agent a on a.workspace_id = w._id where w._id = ? and w.owner_id = ? and a.state = ? and w.state = ? order by a.created_at desc limit 1",
		workspaceId, ownerId, models.WorkspaceAgentStateRunning, models.WorkspaceActive,
	).Scan(&agentId, &codeSourceId, &commit)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, "", fmt.Errorf("agent not found")
		}
		return -1, "", fmt.Errorf("failed to query database for workspace agent: %v", err)
	}

	// create working directory as empty string by default
	workDir := ""

	// conditionally load working directory
	if workingDir {
		// retrieve config from git repo
		configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
			fmt.Sprintf("%d", ownerId),
			fmt.Sprintf("%d", codeSourceId),
			commit,
			".gigo/workspace.yaml",
		)
		defer gitRes.Body.Close()
		if err != nil {
			buf, _ := io.ReadAll(gitRes.Body)
			return -1, "", fmt.Errorf("failed to retrieve config from git repo: %v\n    res: %s", err, string(buf))
		}

		// unmarshall the config
		var config workspace_config.GigoWorkspaceConfig
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			return -1, "", fmt.Errorf("failed to unmarshall workspace config: %v", err)
		}

		workDir = config.WorkingDirectory
	}

	return agentId, workDir, nil
}

func DesktopProxy(ctx context.Context, db *ti.Database, workspaceId int64, ownerId int64) (int64, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "dekstop-proxy")
	defer span.End()
	callerName := "DesktopProxy"

	// create variables to load agent id and serialized ports into
	agentId := int64(-1)
	var portsBuf []byte

	// query database for the current workspace agent
	err := db.QueryRowContext(ctx, &span, &callerName,
		"select a._id, w.ports from workspaces w join workspace_agent a on a.workspace_id = w._id where w._id = ? and w.owner_id = ? and a.state = ? and w.state = ? order by a.created_at desc limit 1",
		workspaceId, ownerId, models.WorkspaceAgentStateRunning, models.WorkspaceActive,
	).Scan(&agentId, &portsBuf)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, fmt.Errorf("agent not found")
		}
		return -1, fmt.Errorf("failed to query database for workspace agent: %v", err)
	}

	return agentId, nil
}

func AgentProxy(ctx context.Context, db *ti.Database, workspaceId int64, ownerId int64) (int64, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "agent-proxy")
	defer span.End()
	callerName := "AgentProxy"

	// create variables to load agent id and serialized ports into
	agentId := int64(-1)
	var portsBuf []byte

	// query database for the current workspace agent
	err := db.QueryRowContext(ctx, &span, &callerName,
		"select a._id, w.ports from workspaces w join workspace_agent a on a.workspace_id = w._id where w._id = ? and w.owner_id = ? and a.state = ? and w.state = ? order by a.created_at desc limit 1",
		workspaceId, ownerId, models.WorkspaceAgentStateRunning, models.WorkspaceActive,
	).Scan(&agentId, &portsBuf)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, fmt.Errorf("agent not found")
		}
		return -1, fmt.Errorf("failed to query database for workspace agent: %v", err)
	}

	return agentId, nil
}

func PortProxyGetWorkspaceAgentID(ctx context.Context, db *ti.Database, workspaceId int64, ownerId int64, targetPort uint16) (int64, *models.WorkspacePort, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "port-proxy-get-workspace-agent-id")
	defer span.End()
	callerName := "PortProxyGetWorkspaceAgentID"

	// create variables to load agent id and serialized ports into
	agentId := int64(-1)
	var portsBuf []byte

	// query database for the current workspace agent
	err := db.QueryRowContext(ctx, &span, &callerName,
		"select a._id, w.ports from workspaces w join workspace_agent a on a.workspace_id = w._id where w._id = ? and w.owner_id = ? and a.state = ? and w.state = ? order by a.created_at desc limit 1",
		workspaceId, ownerId, models.WorkspaceAgentStateRunning, models.WorkspaceActive,
	).Scan(&agentId, &portsBuf)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil, fmt.Errorf("agent not found")
		}
		return -1, nil, fmt.Errorf("failed to query database for workspace agent: %v", err)
	}

	// exit early if no ports are authorized
	if len(portsBuf) == 0 {
		return -1, nil, fmt.Errorf("port not found")
	}

	// create ports slice to unmarshall ports json into
	var ports []models.WorkspacePort
	err = json.Unmarshal(portsBuf, &ports)
	if err != nil {
		return -1, nil, fmt.Errorf("failed to unmarshall ports: %v", err)
	}

	// locate the port
	for _, p := range ports {
		if p.Port == targetPort {
			return agentId, &p, nil
		}
	}

	// return not found for unauthorized port access
	return -1, nil, fmt.Errorf("port not found")
}
