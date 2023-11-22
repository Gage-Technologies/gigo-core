package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/coder/api/core"

	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"github.com/gage-technologies/gigo-lib/network"
)

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

	var req agentsdk.InitializeWorkspaceAgentRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		api.HandleError(rw, "failed to unmarshall request body", r.URL.Path,
			"PostWorkspaceAgentState", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "agent",
			fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusBadRequest, "invalid request body", err)
		return
	}

	// call core function to initialize agent and retrieve agent metadata
	meta, err := core.InitializeAgent(ctx, core.InitializeAgentOptions{
		DB:             api.DB,
		StreakEngine:   api.StreakEngine,
		VcsClient:      api.VcsClient,
		AgentID:        agentId.(int64),
		WorkspaceId:    workspaceId.(int64),
		OwnerId:        ownerId.(int64),
		AccessUrl:      api.AccessURL,
		AppHostname:    api.AppHostname,
		GitUseTLS:      api.GitUseTLS,
		RegistryCaches: api.RegistryCaches,
		IsVNC:          req.IsVNC,
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
