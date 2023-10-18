package api

import (
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net"
	"net/http"
	"net/netip"

	"github.com/gage-technologies/GIGO/src/coder/api/core"
	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"github.com/gage-technologies/gigo-lib/coder/tailnet"
	"github.com/gage-technologies/gigo-lib/network"
	"golang.org/x/xerrors"
	"tailscale.com/tailcfg"
)

func (api *WorkspaceAPI) dialWorkspaceAgentTailnet(r *http.Request, agentID int64) (*agentsdk.AgentConn, error) {
	api.Logger.Debugf("(dialer: %d) dialing agent", agentID)
	clientConn, serverConn := net.Pipe()

	// retrieve latest version of the cluster derp map
	derpMap, err := GetClusterDerpMap(api.ClusterNode)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster derp map: %v", err)
	}

	for _, region := range derpMap.Regions {
		if !region.EmbeddedRelay {
			continue
		}
		var node *tailcfg.DERPNode
		for _, n := range region.Nodes {
			if n.STUNOnly {
				continue
			}
			node = n
			break
		}
		if node == nil {
			continue
		}
		// TODO: This should dial directly to execute the
		// DERP server instead of contacting localhost.
		//
		// This requires modification of Tailscale internals
		// to pipe through a proxy function per-region, so
		// this is an easy and mostly reliable hack for now.
		cloned := node.Clone()
		// Add p for proxy.
		// This first node supports TLS.
		cloned.Name += "p"
		cloned.IPv4 = "127.0.0.1"
		cloned.InsecureForTests = true
		region.Nodes = append(region.Nodes, cloned.Clone())
		// This second node forces HTTP.
		cloned.Name += "-http"
		cloned.ForceHTTP = true
		region.Nodes = append(region.Nodes, cloned)
	}

	connID := api.SnowflakeNode.Generate().Int64()
	api.Logger.Debugf("(dialer: %d) creating new connection with server id %d: %+v", agentID, connID, derpMap)
	/// TODO check if we should convert this back to node id.
	conn, err := tailnet.NewConn(tailnet.ConnTypeServer, &tailnet.Options{
		NodeID:    connID,
		Addresses: []netip.Prefix{netip.PrefixFrom(tailnet.IP(), 128)},
		DERPMap:   derpMap,
	}, api.Logger)
	if err != nil {
		return nil, xerrors.Errorf("create tailnet conn: %w", err)
	}

	api.Logger.Debugf("(dialer: %d) connecting to coordinator", agentID)
	_ = conn.ConnectToCoordinator(clientConn)
	go func() {
		err := (*api.TailnetCoordinator.Load()).ServeClient(serverConn, api.ID, agentID)
		if err != nil {
			api.Logger.Warnf("tailnet coordinator client error: %v", err)
			_ = conn.Close()
		}
	}()
	return &agentsdk.AgentConn{
		Conn: conn,
		CloseFunc: func() {
			_ = clientConn.Close()
			_ = serverConn.Close()
		},
	}, nil
}

func (api *WorkspaceAPI) InitializeAgent(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "initialize-agent-http")
	defer parentSpan.End()

	// attempt to retrieve agent id from context
	agentId := ctx.Value("agent")
	if agentId == nil {
		api.HandleError(rw, "agent missing in context", r.URL.Path,
			"InitializeAgent", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := ctx.Value("workspace")
	if workspaceId == nil {
		api.HandleError(rw, "workspace missing in context", r.URL.Path,
			"InitializeAgent", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve owner id from context
	ownerId := ctx.Value("owner")
	if ownerId == nil {
		api.HandleError(rw, "owner missing in context", r.URL.Path,
			"InitializeAgent", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve latest version of the cluster derp map
	derpMap, err := GetClusterDerpMap(api.ClusterNode)
	if err != nil {
		api.HandleError(rw, "failed to get cluster derp map", r.URL.Path,
			"InitializeAgent", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", err)
		return
	}

	buf, _ := json.Marshal(derpMap)
	fmt.Println("derp map for agent:", string(buf))

	// call core function to initialize agent and retrieve agent metadata
	meta, err := core.InitializeAgent(ctx, core.InitializeAgentOptions{
		DB:             api.DB,
		StreakEngine:   api.StreakEngine,
		VcsClient:      api.VcsClient,
		WorkspaceId:    workspaceId.(int64),
		OwnerId:        ownerId.(int64),
		AccessUrl:      api.AccessURL,
		AppHostname:    api.AppHostname,
		DERPMap:        derpMap,
		GitUseTLS:      api.GitUseTLS,
		RegistryCaches: api.RegistryCaches,
	})
	if err != nil {
		if err.Error() == "workspace not found" {
			api.HandleError(rw, "InitializeAgent core failed", r.URL.Path, "InitializeAgent", r.Method,
				r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
				http.StatusNotFound, "workspace not found", err)
			return
		}
		api.HandleError(rw, "InitializeAgent core failed", r.URL.Path, "InitializeAgent", r.Method,
			r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusInternalServerError, "internal server error", err)
		return
	}

	// log successful function execution
	api.Logger.LogDebugExternalAPI(
		"function execution successful",
		r.URL.Path,
		"InitializeAgent",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		"agent",
		fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
		http.StatusOK,
		nil,
	)

	parentSpan.AddEvent(
		"initialize-agent",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	agentsdk.Write(ctx, rw, http.StatusOK, meta)
}

func (api *WorkspaceAPI) PostWorkspaceAgentState(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "post-workspace-agent-state-http")
	defer parentSpan.End()

	// attempt to retrieve agent id from context
	agentId := ctx.Value("agent")
	if agentId == nil {
		api.HandleError(rw, "agent missing in context", r.URL.Path,
			"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := ctx.Value("workspace")
	if workspaceId == nil {
		api.HandleError(rw, "workspace missing in context", r.URL.Path,
			"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve owner id from context
	ownerId := ctx.Value("owner")
	if ownerId == nil {
		api.HandleError(rw, "owner missing in context", r.URL.Path,
			"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	var req agentsdk.PostWorkspaceAgentState
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.HandleError(rw, "failed to unmarshall request body", r.URL.Path,
			"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusBadRequest, "invalid request body", err)
		return
	}

	api.Logger.Debug(fmt.Sprintf(
		"workspace state report: %d-%d-%d",
		agentId.(int64), workspaceId.(int64), ownerId.(int64),
	))

	err = core.UpdateWorkspaceAgentState(ctx, api.DB, agentId.(int64), req.State)
	if err != nil {
		if err.Error() == "invalid agent state" {
			api.HandleError(rw, "invalid agent state", r.URL.Path,
				"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
				http.StatusBadRequest, "invalid request body", err)
			return
		}
		api.HandleError(rw, "failed to update workspace agent state", r.URL.Path,
			"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusInternalServerError, "internal server error", err)
		return
	}

	api.publishWorkspaceUpdate(workspaceId.(int64))

	// log successful function execution
	api.Logger.LogDebugExternalAPI(
		"function execution successful",
		r.URL.Path,
		"PostWorkspaceAgentState",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		"agent",
		fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
		http.StatusOK,
		nil,
	)

	parentSpan.AddEvent(
		"post-workspace-agent-state",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	agentsdk.Write(ctx, rw, http.StatusNoContent, nil)
}

func (api *WorkspaceAPI) PostWorkspaceAgentVersion(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "post-workspace-agent-version-http")
	defer parentSpan.End()

	// attempt to retrieve agent id from context
	agentId := ctx.Value("agent")
	if agentId == nil {
		api.HandleError(rw, "agent missing in context", r.URL.Path,
			"PostWorkspaceAgentVersion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := ctx.Value("workspace")
	if workspaceId == nil {
		api.HandleError(rw, "workspace missing in context", r.URL.Path,
			"PostWorkspaceAgentVersion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve owner id from context
	ownerId := ctx.Value("owner")
	if ownerId == nil {
		api.HandleError(rw, "owner missing in context", r.URL.Path,
			"PostWorkspaceAgentVersion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	var req agentsdk.PostWorkspaceAgentVersionRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.HandleError(rw, "failed to unmarshall request body", r.URL.Path,
			"PostWorkspaceAgentVersion", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusBadRequest, "invalid request body", err)
		return
	}

	api.Logger.Debug(fmt.Sprintf(
		"post workspace version: %q %d-%d-%d",
		req.Version, agentId.(int64), workspaceId.(int64), ownerId.(int64),
	))

	err = core.UpdateWorkspaceAgentVersion(ctx, api.DB, agentId.(int64), req.Version)
	if err != nil {
		if err.Error() == "invalid version" {
			api.HandleError(rw, fmt.Sprintf("invalid semver: %s", req.Version), r.URL.Path,
				"PostWorkspaceAgentVersion", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "agent",
				fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
				http.StatusBadRequest, "Invalid workspace agent version provided.", err)
			return
		}
		api.HandleError(rw, "failed to update workspace agent version", r.URL.Path,
			"PostWorkspaceAgentVersion", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusInternalServerError, "internal server error", err)
		return
	}

	// log successful function execution
	api.Logger.LogDebugExternalAPI(
		"function execution successful",
		r.URL.Path,
		"PostWorkspaceAgentVersion",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		"agent",
		fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
		http.StatusOK,
		nil,
	)

	parentSpan.AddEvent(
		"post-workspace-agent-version",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	agentsdk.Write(ctx, rw, http.StatusOK, nil)
}

func (api *WorkspaceAPI) PostWorkspaceAgentPort(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "post-workspace-agent-port-http")
	defer parentSpan.End()

	// attempt to retrieve agent id from context
	agentId := ctx.Value("agent")
	if agentId == nil {
		api.HandleError(rw, "agent missing in context", r.URL.Path,
			"PostWorkspaceAgentPort", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := ctx.Value("workspace")
	if workspaceId == nil {
		api.HandleError(rw, "workspace missing in context", r.URL.Path,
			"PostWorkspaceAgentPort", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve owner id from context
	ownerId := ctx.Value("owner")
	if ownerId == nil {
		api.HandleError(rw, "owner missing in context", r.URL.Path,
			"PostWorkspaceAgentPort", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	var req agentsdk.AgentPorts
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.HandleError(rw, "failed to unmarshall request body", r.URL.Path,
			"PostWorkspaceAgentPort", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusBadRequest, "invalid request body", err)
		return
	}

	api.Logger.Debug(fmt.Sprintf(
		"post workspace ports: %d %d-%d-%d",
		len(req.Ports), agentId.(int64), workspaceId.(int64), ownerId.(int64),
	))

	err = core.UpdateWorkspaceAgentPorts(ctx, api.DB, workspaceId.(int64), req.Ports)
	if err != nil {
		api.HandleError(rw, "failed to update workspace agent ports", r.URL.Path,
			"PostWorkspaceAgentPort", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusInternalServerError, "internal server error", err)
		return
	}

	// log successful function execution
	api.Logger.LogDebugExternalAPI(
		"function execution successful",
		r.URL.Path,
		"PostWorkspaceAgentPort",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		"agent",
		fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
		http.StatusOK,
		nil,
	)

	parentSpan.AddEvent(
		"post-workspace-agent-port",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	agentsdk.Write(ctx, rw, http.StatusNoContent, nil)
}

func (api *WorkspaceAPI) WorkspaceAgentReportStats(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "post-workspace-agent-report-stats-http")
	defer parentSpan.End()

	// attempt to retrieve agent id from context
	agentId := ctx.Value("agent")
	if agentId == nil {
		api.HandleError(rw, "agent missing in context", r.URL.Path,
			"WorkspaceAgentReportStats", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := ctx.Value("workspace")
	if workspaceId == nil {
		api.HandleError(rw, "workspace missing in context", r.URL.Path,
			"WorkspaceAgentReportStats", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve owner id from context
	ownerId := ctx.Value("owner")
	if ownerId == nil {
		api.HandleError(rw, "owner missing in context", r.URL.Path,
			"WorkspaceAgentReportStats", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	var req agentsdk.AgentStats
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.HandleError(rw, "failed to unmarshall request body", r.URL.Path,
			"WorkspaceAgentReportStats", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.RxBytes == 0 && req.TxBytes == 0 {
		// log successful function execution
		api.Logger.LogDebugExternalAPI(
			"function execution successful",
			r.URL.Path,
			"WorkspaceAgentReportStats",
			r.Method,
			r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r),
			"agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusOK,
			nil,
		)
		agentsdk.Write(ctx, rw, http.StatusOK, agentsdk.AgentStatsResponse{
			ReportInterval: api.AgentStatsRefreshInterval,
		})
		return
	}

	api.Logger.Debugf("read stats report: %d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64))

	err = core.UpdateWorkspaceAgentStats(ctx, core.UpdateAgentStatsOptions{
		DB:            api.DB,
		SnowflakeNode: api.SnowflakeNode,
		AgentID:       agentId.(int64),
		WorkspaceID:   workspaceId.(int64),
		Stats:         req,
	})
	if err != nil {
		api.HandleError(rw, "failed to update stats", r.URL.Path,
			"WorkspaceAgentReportStats", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusInternalServerError, "failed to update stats", err)
		return
	}

	// log successful function execution
	api.Logger.LogDebugExternalAPI(
		"function execution successful",
		r.URL.Path,
		"WorkspaceAgentReportStats",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		"agent",
		fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
		http.StatusOK,
		nil,
	)

	parentSpan.AddEvent(
		"post-workspace-agent-report-stats",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	agentsdk.Write(ctx, rw, http.StatusOK, agentsdk.AgentStatsResponse{
		ReportInterval: api.AgentStatsRefreshInterval,
	})
}
