package api

import (
	"context"
	"database/sql"
	"fmt"
	"gigo-core/gigo/config"
	"gigo-core/gigo/streak"
	"gigo-core/gigo/utils"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/gigo-lib/cluster"
	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
)

const (
	CtxKeyUser      = "callingUser"
	CtxKeyRequestID = "requestID"
)

type WorkspaceAPIOptions struct {
	ID          int64
	ClusterNode cluster.Node
	Address     string
	Ctx         context.Context

	AgentConnectionUpdateFrequency time.Duration
	AgentInactiveDisconnectTimeout time.Duration
	AgentStatsRefreshInterval      time.Duration

	Logger logging.Logger

	StreakEngine *streak.StreakEngine

	DB            *ti.Database
	RDB           redis.UniversalClient
	VcsClient     *git.VCSClient
	SnowflakeNode *snowflake.Node

	AccessURL   *url.URL
	AppHostname string
	GitUseTLS   bool
	Js          *mq.JetstreamClient

	RegistryCaches []config.RegistryCacheConfig

	ZitiServer      *zitimesh.Server
	WsStatusUpdater *utils.WorkspaceStatusUpdater
}

// WorkspaceAPI
//
//	An http server for interacting with workspaces and their agent.
//	This is a forked and heavily modified version of Coder's API (https://github.com/coder/coder/blob/b20cb993bd3fd39b84591d6ca300cad1171db727/coderd/coderd.go)
type WorkspaceAPI struct {
	*WorkspaceAPIOptions
	WorkspaceClientCoordinateOverride atomic.Pointer[func(rw http.ResponseWriter) bool]

	// APIHandler serves "/api/v2" and "/bin"
	APIHandler *mux.Router

	WebsocketWaitMutex sync.Mutex
	WebsocketWaitGroup sync.WaitGroup

	// these are functions that exist in the main project's http api server that we will back link to
	// the workspace api. that way we can preserve our framework and system and have all auth and
	// logging centralized without contaminating the main codebase with the coder modifications
	HandleError func(w http.ResponseWriter, message string, endpoint string, method string, methodType string,
		reqId interface{}, ip string, username string, userId string, statusCode int, responseMessage string, err error)
}

// prepWorkspaceAPIOptions
//
//	Helper function to prep options for NewWorkspaceAPI
func prepWorkspaceAPIOptions(options *WorkspaceAPIOptions) *WorkspaceAPIOptions {
	// panic if the snowflake node isn't passed - the underlying node id
	// for snowflake is critical to uniqueness and selecting a random
	// id is just asking for trouble
	if options.SnowflakeNode == nil {
		panic("snowflake node must be configured outside of the workspace api to ensure a unique id")
	}

	if options.AgentConnectionUpdateFrequency == 0 {
		options.AgentConnectionUpdateFrequency = 3 * time.Second
	}
	if options.AgentInactiveDisconnectTimeout == 0 {
		// Multiply the update by two to allow for some lag-time.
		options.AgentInactiveDisconnectTimeout = options.AgentConnectionUpdateFrequency * 2
	}
	if options.AgentStatsRefreshInterval == 0 {
		options.AgentStatsRefreshInterval = 5 * time.Minute
	}

	return options
}

// NewWorkspaceAPI
//
//	Creates a new NewWorkspaceAPI and initializes the http routes
func NewWorkspaceAPI(opts *WorkspaceAPIOptions) (*WorkspaceAPI, error) {
	// prep options to ensure everything is initialized
	opts = prepWorkspaceAPIOptions(opts)

	// create a new workspace api
	wsApi := &WorkspaceAPI{
		WorkspaceAPIOptions: opts,
	}

	return wsApi, nil
}

// LinkAPI
//
//	Helper function to link the API to an existing mux router
func (api *WorkspaceAPI) LinkAPI(r *mux.Router) {
	// router to api
	api.APIHandler = r

	// create workspace endpoints as prefixes so that we can accept path
	// extensions in a proxy-like way for relaying the path to the end destination
	r.PathPrefix("/editor/{user:[0-9]+}/{workspace:[0-9]+}-{commit:.{1,64}}").HandlerFunc(api.WorkspaceEditorProxy)
	r.PathPrefix("/desktop/{user:[0-9]+}/{workspace:[0-9]+}-{commit:.{1,64}}").HandlerFunc(api.WorkspaceDesktopProxy)
	// r.PathPrefix("/port/{user:[0-9]+}/{workspace:[0-9]+}/{port:[0-9]+}").HandlerFunc(api.WorkspacePortProxy)
	r.Host(fmt.Sprintf("{user:[0-9]+}-{workspace:[0-9]+}-{port:[0-9]+}.%s", api.AppHostname)).HandlerFunc(api.WorkspacePortProxy)
	r.Host(fmt.Sprintf("{byte:[0-9]+}-lsp.%s", api.AppHostname)).HandlerFunc(api.ByteLspProxy)

	// This is used when UDP is blocked, and latency must be checked via HTTP(s).
	r.HandleFunc("/derp/latency-check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// routes for agent->core communication
	internalWsRouter := r.PathPrefix("/internal/v1/ws").Subrouter()
	internalWsRouter.Use(api.authenticateAgent)
	internalWsRouter.HandleFunc("/initialize", api.InitializeAgent).Methods("POST")
	internalWsRouter.HandleFunc("/state", api.PostWorkspaceAgentState).Methods("POST")
	internalWsRouter.HandleFunc("/version", api.PostWorkspaceAgentVersion).Methods("POST")
	internalWsRouter.HandleFunc("/ports", api.PostWorkspaceAgentPort).Methods("POST")
	internalWsRouter.HandleFunc("/stats", api.WorkspaceAgentReportStats).Methods("POST")
}

// authenticateAgent
//
//	Middleware to authenticate a workspace agent
func (api *WorkspaceAPI) authenticateAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "authenticate-workspace-agent-http")
		defer parentSpan.End()
		callerName := "AuthenticateWorkspaceAgent"

		// attempt to retrieve token
		token := r.Header.Get("Gigo-Agent-Token")
		if len(token) == 0 {
			c, _ := r.Cookie("Gigo-Agent-Token")
			if c != nil {
				token = c.Value
			}
		}
		if len(token) == 0 {
			api.HandleError(w, "agent token missing", r.URL.Path, "authenticateAgent",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "anon",
				"-1", http.StatusUnauthorized, "agent token required", nil)
			return
		}

		// attempt to retrieve workspace id
		workspaceIDString := r.Header.Get("Gigo-Workspace-Id")
		if len(workspaceIDString) == 0 {
			c, _ := r.Cookie("Gigo-Workspace-Id")
			if c != nil {
				workspaceIDString = c.Value
			}
		}
		if len(workspaceIDString) == 0 {
			api.HandleError(w, "workspace id missing", r.URL.Path, "authenticateAgent",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "workspace id required", nil)
			return
		}

		// format workspace id to integer
		workspaceID, err := strconv.ParseInt(workspaceIDString, 10, 64)
		if err != nil {
			api.HandleError(w, "invalid workspace id", r.URL.Path, "authenticateAgent",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid workspace id", nil)
			return
		}

		// authenticate this call by using the workspace id and agent token
		// to query the database for the agent id and owner id
		var agentId int64
		var ownerId int64
		err = api.DB.QueryRowContext(ctx, &parentSpan, &callerName,
			"select a._id, w.owner_id, w._id from workspaces w join workspace_agent a on a.workspace_id = w._id left join workspace_pool wsp on w._id = wsp.workspace_table_id where (w._id = ? or wsp._id = ?) and a.secret = uuid_to_bin(?) order by a.created_at desc limit 1",
			workspaceID, workspaceID, token,
		).Scan(&agentId, &ownerId, &workspaceID)
		if err != nil {
			if err == sql.ErrNoRows {
				agentsdk.Write(ctx, w, http.StatusNotFound, nil)
				return
			}
			api.HandleError(w, "failed to authenticate agent", r.URL.Path,
				"authenticateAgent", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "failed to authenticate agent", err)
			return
		}

		// add workspace id, agent id and owner id to the context
		ctx = context.WithValue(r.Context(), "workspace", workspaceID)
		ctx = context.WithValue(ctx, "agent", agentId)
		ctx = context.WithValue(ctx, "owner", ownerId)

		// execute end function with new context containing workspace and agent ids
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
