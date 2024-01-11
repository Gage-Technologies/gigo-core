package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/config"
	"gigo-core/gigo/constants"
	"gigo-core/gigo/utils"
	"io"
	"net/url"
	"strings"
	"time"

	"gigo-core/gigo/streak"

	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/workspace_config"
	"github.com/gage-technologies/gitea-go/gitea"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
	"tailscale.com/tailcfg"
)

type InitializeAgentOptions struct {
	DB             *ti.Database
	StreakEngine   *streak.StreakEngine
	VcsClient      *git.VCSClient
	AgentID        int64
	WorkspaceId    int64
	OwnerId        int64
	AccessUrl      *url.URL
	AppHostname    string
	DERPMap        *tailcfg.DERPMap
	GitUseTLS      bool
	RegistryCaches []config.RegistryCacheConfig
	IsVNC          bool
}

type UpdateAgentStatsOptions struct {
	DB            *ti.Database
	SnowflakeNode *snowflake.Node
	AgentID       int64
	WorkspaceID   int64
	Stats         agentsdk.AgentStats
}

// handleRegistryCaches
//
//	Checks if the container is from any of the source registries
//	and if so, replaces the host with container registry cache in the container
//	name. If there is a cache configured for docker.io and the container
//	contains no host then the container name is assumed to be from docker.io
func handleRegistryCaches(containerName string, caches []config.RegistryCacheConfig) string {
	// create a variable to hold the docker.io cache if it exists
	var dockerCache config.RegistryCacheConfig

	// iterate over the registry caches
	for _, cache := range caches {
		// if the container name contains the registry host
		if strings.HasPrefix(containerName, cache.Source) {
			// replace the registry host with the cache host
			return strings.Replace(containerName, cache.Source, cache.Cache, 1)
		}

		// save the docker cache if it exists in case the container has no host prefix
		if cache.Source == "docker.io" {
			// set the docker cache
			dockerCache = cache
		}
	}

	// if the container name has no host prefix and the docker cache exists
	// then we assume the container is from docker.io and prepend the cache
	if dockerCache.Source == "docker.io" && strings.Count(containerName, "/") <= 1 {
		// if the container name does not contain an organization then we prepend library/
		// if !strings.Contains(containerName, "/") {
		// 	return fmt.Sprintf("%s/library/%s", dockerCache.Cache, containerName)
		// }
		return fmt.Sprintf("%s/%s", dockerCache.Cache, containerName)
	}

	// return the container name if no cache was found
	return containerName
}

// InitializeAgent
//
//	Initializes an agent session by retrieving metadata needed to bootstrap
//	a GIGO workspace and initializing credentials for the agent/workspace
func InitializeAgent(ctx context.Context, opts InitializeAgentOptions) (*agentsdk.WorkspaceAgentMetadata, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "initialize-agent-core")
	defer span.End()
	callerName := "InitializeAgent"

	// create empty variables to hold data
	var repo int64
	var commit string
	var expiration time.Time
	var wsSettingsBytes []byte
	var initState models.WorkspaceInitState
	var state models.WorkspaceState
	var userStatus models.UserStatus
	var enableHolidayThemes bool
	var challengeType models.ChallengeType
	var projectId int64
	var projectType models.CodeSource
	var ephemeralUser bool
	var createdAt time.Time
	var startTime *time.Duration
	var zitiId string
	var zitiToken string

	// retrieve the user status for the owner
	err := opts.DB.QueryRowContext(ctx, &span, &callerName,
		"select user_status, holiday_themes, is_ephemeral from users where _id = ? limit 1",
		&opts.OwnerId,
	).Scan(&userStatus, &enableHolidayThemes, &ephemeralUser)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to query users: %v", err)
	}

	// use owner ID to get data from a new workspace
	err = opts.DB.QueryRowContext(ctx, &span, &callerName,
		"select repo_id, commit, expiration, workspace_settings, init_state, state, code_source_id, code_source_type, created_at, start_time from workspaces where _id = ? and owner_id = ? limit 1",
		&opts.WorkspaceId, &opts.OwnerId,
	).Scan(&repo, &commit, &expiration, &wsSettingsBytes, &initState, &state, &projectId, &projectType, &createdAt, &startTime)
	if err != nil {
		if err == sql.ErrNoRows {
			///// ASK SAM
			return &agentsdk.WorkspaceAgentMetadata{
				Unassigned: true,
			}, nil
		}
		////
		return nil, fmt.Errorf("failed to query workspaces: %v", err)

	}

	// use agent id to get the ziti id and token
	err = opts.DB.QueryRowContext(ctx, &span, &callerName,
		"select ziti_id, ziti_token from workspace_agent where _id = ? limit 1",
		opts.AgentID,
	).Scan(&zitiId, &zitiToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace agent not found")
		}
		return nil, fmt.Errorf("failed to query workspace agent: %v", err)
	}

	if projectType == models.CodeSourcePost {
		err = opts.DB.QueryRowContext(ctx, &span, &callerName,
			"SELECT post_type FROM post WHERE _id = ? LIMIT 1",
			&projectId,
		).Scan(&challengeType)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("workspace project not found")
			}
			return nil, fmt.Errorf("failed to query project workspaces: %v", err)
		}
	} else if projectType == models.CodeSourceByte {
		challengeType = models.BytesChallenge
	} else {
		err = opts.DB.QueryRowContext(ctx, &span, &callerName,
			"SELECT post_type FROM attempt WHERE _id = ? LIMIT 1",
			&projectId,
		).Scan(&challengeType)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("workspace project not found")
			}
			return nil, fmt.Errorf("failed to query project workspaces: %v", err)
		}
	}

	// unmarshall workspace settings
	var workspaceSettings models.WorkspaceSettings
	err = json.Unmarshal(wsSettingsBytes, &workspaceSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall workspace settings: %v", err)
	}

	// use the repo id to retrieve the repository URL
	repository, _, err := opts.VcsClient.GiteaClient.GetRepoByID(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve repository URL: %v", err)
	}

	var gigoConfig workspace_config.GigoWorkspaceConfig

	if projectType != models.CodeSourceByte {
		// retrieve the gigo workspace config for this repo and commit
		configBytes, gitRes, err := opts.VcsClient.GiteaClient.GetFile(
			fmt.Sprintf("%d", opts.OwnerId),
			repository.Name,
			commit,
			".gigo/workspace.yaml",
		)
		if err != nil {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
		}

		// parse config bytes into workspace config
		err = yaml.Unmarshal(configBytes, &gigoConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse new config: %v", err)
		}
	} else {
		gigoConfig = constants.BytesWorkspaceConfig
	}

	// attempt to route any of the container images in the containers spec
	// through the registry caches if they exist
	if services, ok := gigoConfig.Containers["services"]; ok {
		// only proceed if we can assert services as a map[string]interface{}
		if servicesMap, ok := services.(map[string]interface{}); ok {
			// iterate over the container services
			for _, service := range servicesMap {
				// try to assert the service as a map[string]interface{}
				if serviceMap, ok := service.(map[string]interface{}); ok {
					// if the service has an image key
					if image, ok := serviceMap["image"]; ok {
						// try to assert the image as a string
						if imageName, ok := image.(string); ok {
							// handle the registry caches
							serviceMap["image"] = handleRegistryCaches(imageName, opts.RegistryCaches)
						}
					}
				}
			}
		}
	}

	var giteaToken string

	if projectType == models.CodeSourceByte {
		giteaToken = ""
	} else {
		// create a git token for the workspace
		token, _, err := opts.VcsClient.GiteaClient.CreateAccessTokenAdmin(fmt.Sprintf("%d", opts.OwnerId), gitea.CreateAccessTokenOption{
			Name: fmt.Sprintf("%d", opts.WorkspaceId),
			// since gitea v1.19.0, the scopes are now required so we
			// grant just private repo access for the workspace
			Scopes: []gitea.AccessTokenScope{
				gitea.AccessTokenScopeRepo,
			},
		})
		if err != nil {
			// we return for wny unexpected error
			if !strings.Contains(err.Error(), "access token name has been used already") {
				return nil, fmt.Errorf("failed to create a token for the workspace: %v", err)
			}
			// if the token already exists then we return "exists"
			// so that the agent knows to skip git configuration
			token = &gitea.AccessToken{
				Token: "exists",
			}
		}

		giteaToken = token.Token
	}

	// set the workspace as initialized
	_, err = opts.DB.ExecContext(ctx, &span, &callerName, "update workspaces set last_state_update = ?, is_vnc = ? where _id = ?", time.Now(), opts.IsVNC, opts.WorkspaceId)
	if err != nil {
		return nil, fmt.Errorf("failed to set workspace as 'initialized': %v", err)
	}

	// format the base path for port forwarding using {{port}}
	// indicate the variable of the dynamic port that is allocated
	accessPort := ""
	if opts.AccessUrl.Port() != "" {
		accessPort += fmt.Sprintf(":%s", opts.AccessUrl.Port())
	}

	// format the proxy url structure so that vscode knows how to forward our ports
	vscodeProxyURI := fmt.Sprintf("%s://%d-%d-{{port}}.%s%s",
		opts.AccessUrl.Scheme,
		opts.OwnerId,
		opts.WorkspaceId,
		opts.AppHostname,
		accessPort,
	)

	// handle first start logic
	if initState != models.WorkspaceInitCompleted {
		// update the user stats for a new workspace but only if the user is not ephemeral
		if !ephemeralUser {
			err = opts.StreakEngine.UserStartWorkspace(ctx, opts.OwnerId)
			if err != nil {
				return nil, fmt.Errorf("failed to start streak workspace for user %v: %v", opts.OwnerId, err)
			}
		}

		// record the time to start if we have never recorded it
		if startTime == nil {
			// calculate the time since the workspace was created
			duration := time.Since(createdAt)
			startTime = &duration

			// open a tx
			tx, err := opts.DB.BeginTx(ctx, &span, &callerName, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to begin transaction: %v", err)
			}
			defer tx.Rollback()

			// update the workspace table with the start time
			_, err = tx.ExecContext(
				ctx, &callerName,
				"update workspaces set start_time = ? where _id = ?",
				startTime, opts.WorkspaceId,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to update workspace start time: %v", err)
			}

			// update the post or attempt (dependent on the project type)
			if projectType == models.CodeSourcePost {
				_, err = tx.ExecContext(
					ctx, &callerName,
					"update post set start_time = ? where _id = ?",
					startTime, projectId,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to update post start time: %v", err)
				}
			} else {
				_, err = tx.ExecContext(
					ctx, &callerName,
					"update attempt set start_time = ? where _id = ?",
					startTime, projectId,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to update attempt start time: %v", err)
				}
			}

			// commit the tx
			err = tx.Commit(&callerName)
			if err != nil {
				return nil, fmt.Errorf("failed to commit transaction: %v", err)
			}
		}
	}

	// parse the clone url to retrieve only the path to the repository
	parsedCloneUrl, err := url.Parse(repository.CloneURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse clone url: %v", err)
	}
	cloneURL := opts.VcsClient.HostUrl + parsedCloneUrl.Path
	if opts.GitUseTLS {
		cloneURL = strings.ReplaceAll(cloneURL, "http://", "https://")
	}

	return &agentsdk.WorkspaceAgentMetadata{
		WorkspaceID:        opts.WorkspaceId,
		WorkspaceIDString:  fmt.Sprintf("%d", opts.WorkspaceId),
		Repo:               cloneURL,
		Commit:             commit,
		GitToken:           giteaToken,
		GitEmail:           fmt.Sprintf("%d@git.%s", opts.OwnerId, opts.AppHostname),
		GitName:            fmt.Sprintf("%d", opts.OwnerId),
		Expiration:         expiration.Unix(),
		OwnerID:            opts.OwnerId,
		OwnerIDString:      fmt.Sprintf("%d", opts.OwnerId),
		WorkspaceSettings:  &workspaceSettings,
		VSCodePortProxyURI: vscodeProxyURI,
		DERPMap:            opts.DERPMap,
		GigoConfig:         gigoConfig.ToAgent(),
		LastInitState:      initState,
		WorkspaceState:     state,
		UserStatus:         userStatus,
		HolidaySeason:      agentsdk.Holiday(utils.DetermineHoliday()),
		ChallengeType:      challengeType,
		UserHolidayTheme:   enableHolidayThemes,
		ZitiID:             zitiId,
		ZitiToken:          zitiToken,
	}, nil
}

func GetWorkspaceAgentByID(ctx context.Context, db *ti.Database, id int64) (*models.WorkspaceAgent, error) {
	ctx, span := otel.Tracer("gigo-core").Start(context.Background(), "get-workspace-agent-by-id-core")
	defer span.End()
	callerName := "GetWorkspaceAgentByID"

	// query for workspace agent in database
	res, err := db.QueryContext(ctx, &span, &callerName, "select * from workspace_agent where _id = ? limit 1", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query for workspace agent: %v", err)
	}

	// attempt to load agent into the first position of the cursor
	if !res.Next() {
		return nil, fmt.Errorf("agent not found")
	}

	// load agent from cursor
	agent, err := models.WorkspaceAgentFromSQLNative(res)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent from cursor: %v", err)
	}

	return agent, nil
}

func UpdateWorkspaceAgentState(ctx context.Context, db *ti.Database, agent int64, state models.WorkspaceAgentState) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-workspace-agent-state")
	defer span.End()
	callerName := "UpdateWorkspaceAgentState"

	if state.String() == "Invalid" {
		return fmt.Errorf("invalid agent state")
	}
	_, err := db.ExecContext(ctx, &span, &callerName, "update workspace_agent set state = ? where _id =?", state, agent)
	if err != nil {
		return fmt.Errorf("failed to update workspace agent state: %v", err)
	}
	return nil
}

func UpdateWorkspaceAgentVersion(ctx context.Context, db *ti.Database, agent int64, version string) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-workspace-agent-version")
	defer span.End()
	callerName := "UpdateWorkspaceAgentVersion"

	// if !semver.IsValid(version) {
	// 	return fmt.Errorf("invalid version")
	// }
	_, err := db.ExecContext(ctx, &span, &callerName, "update workspace_agent set version = ? where _id =?", version, agent)
	if err != nil {
		return fmt.Errorf("failed to update workspace agent version: %v", err)
	}
	return nil
}

func UpdateWorkspaceAgentPorts(ctx context.Context, db *ti.Database, wsStatusUpdater *utils.WorkspaceStatusUpdater, workspaceID int64, newPorts []agentsdk.ListeningPort) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-workspace-agent-ports")
	defer span.End()
	callerName := "UpdateWorkspaceAgentPorts"

	// open tx for update
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback() // nolint: errcheck

	// retrieve current ports
	var portsBuf []byte
	err = db.QueryRowContext(ctx, &span, &callerName, "select ports from workspaces where _id =?", workspaceID).Scan(&portsBuf)
	if err != nil {
		return fmt.Errorf("failed to retrieve current ports: %v", err)
	}

	// unmarshall ports into workspace ports
	var currentPorts []models.WorkspacePort
	err = json.Unmarshal(portsBuf, &currentPorts)
	if err != nil {
		return fmt.Errorf("failed to unmarshall ports: %v", err)
	}

	// we have to merge the current ports with the new ports
	// preserving configured ports from the current ports
	// but saving only new ports that are nor configured
	// we also need to mark any configured ports as active
	// or inactive depending on whether they are present in
	// the new ports

	// extract configured ports from the current ports
	// and store them in a map correlated to workspace
	// port model then store them in another map
	// correlated to a boolean that will represent whether
	// the configure port has been added to the new state
	configuredPorts := make(map[uint16]models.WorkspacePort)
	configuredPortsAdded := make(map[uint16]bool)
	for _, p := range currentPorts {
		if !p.Configured {
			continue
		}
		configuredPorts[p.Port] = p
		configuredPortsAdded[p.Port] = false
	}

	// format listening ports to native models
	ports := make([]models.WorkspacePort, len(newPorts))
	for i, p := range newPorts {
		// create a new workspace port
		port := models.WorkspacePort{
			Name:   p.ProcessName,
			Port:   p.Port,
			Active: true,
			HTTP:   p.HTTP,
			SSL:    p.SSL,
		}

		// handle a configured port
		if cPort, ok := configuredPorts[p.Port]; ok {
			port.Name = cPort.Name
			port.Configured = true
			configuredPortsAdded[p.Port] = true
		}

		// set port in new state slice
		ports[i] = port
	}

	// iterate configured ports adding any inactive ports
	for p, added := range configuredPortsAdded {
		// skip added ports
		if added {
			continue
		}

		// add configured port as inactive
		inactivePort := configuredPorts[p]
		inactivePort.Active = false
		ports = append(ports, inactivePort)
	}

	// serialize ports for insertion
	buf, err := json.Marshal(ports)
	if err != nil {
		return fmt.Errorf("failed to serialize ports: %v", err)
	}

	_, err = db.ExecContext(ctx, &span, &callerName, "update workspaces set ports = ? where _id =?", buf, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to update workspace ports: %v", err)
	}

	// forward the update to the user
	wsStatusUpdater.PushStatus(ctx, workspaceID, nil)

	return nil
}

func UpdateWorkspaceAgentStats(ctx context.Context, opts UpdateAgentStatsOptions) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-workspace-agent-stats-agent")
	defer span.End()
	callerName := "UpdateWorkspaceAgentStats"

	// create new agent stats model
	agentStats := models.CreateWorkspaceAgentStats(
		opts.SnowflakeNode.Generate().Int64(),
		opts.AgentID,
		opts.WorkspaceID,
		time.Now(),
		opts.Stats.ConnsByProto,
		opts.Stats.NumConns,
		opts.Stats.RxPackets,
		opts.Stats.RxBytes,
		opts.Stats.TxPackets,
		opts.Stats.TxBytes,
	)

	// create tx for insert operation
	tx, err := opts.DB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to start database transaction for agent stats update: %v", err)
	}

	// defer tx rollback
	defer tx.Rollback()

	// format agent stats for insertion
	statements, err := agentStats.ToSQLNative()
	if err != nil {
		return fmt.Errorf("failed to format agent stats for insertion: %v", err)
	}

	// iterate statements performing insertion
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return fmt.Errorf("failed to insert agent stats: %v", err)
		}
	}

	// close tx
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("failed to commit database transaction for agent stats update: %v", err)
	}

	// we intentionally keep the following expiration extension
	// outside of the tx since we are technically okay if the
	// expiration extension fails but not if we miss the stats

	// retrieve current expiration for workspace
	// require that the workspace is either starting or active
	// and has at least 10s left since we don't want to be
	// extending the expiration on a workspace that has
	// already timed out

	// var expiration time.Time
	// err = opts.DB.DB.QueryRow(
	// 	"select expiration from workspaces where _id = ? and expiration > ? and state in (?, ?)",
	// 	opts.WorkspaceID, time.Now().Add(-10*time.Second), models.WorkspaceStarting, models.WorkspaceActive,
	// ).Scan(&expiration)
	// if err != nil {
	// 	return fmt.Errorf("failed to retrieve current expiration for workspace: %v", err)
	// }
	//
	// // update the expiration
	// _, err = opts.DB.DB.Exec("update workspaces set expiration = ? where _id =?", time.Now().Add(10*time.Minute), opts.WorkspaceID)
	// if err != nil {
	// 	return fmt.Errorf("failed to update workspace expiration: %v", err)
	// }

	return nil
}
