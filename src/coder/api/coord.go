package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/coder/api/core"

	"github.com/gage-technologies/gigo-lib/network"
	"nhooyr.io/websocket"
)

func (api *WorkspaceAPI) WorkspaceAgentCoordinate(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-agent-coordinate-http")
	defer parentSpan.End()

	api.WebsocketWaitMutex.Lock()
	api.WebsocketWaitGroup.Add(1)
	api.WebsocketWaitMutex.Unlock()
	defer api.WebsocketWaitGroup.Done()

	// attempt to retrieve agent id from context
	agentId := ctx.Value("agent")
	if agentId == nil {
		api.HandleError(rw, "agent missing in context", r.URL.Path, "WorkspaceAgentCoordinate",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := ctx.Value("workspace")
	if workspaceId == nil {
		api.HandleError(rw, "workspace missing in context", r.URL.Path, "WorkspaceAgentCoordinate",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to retrieve owner id from context
	ownerId := ctx.Value("owner")
	if ownerId == nil {
		api.HandleError(rw, "owner missing in context", r.URL.Path, "WorkspaceAgentCoordinate",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve agent from database
	agent, err := core.GetWorkspaceAgentByID(ctx, api.DB, agentId.(int64))
	if err != nil {
		if err.Error() == "agent not found" {
			api.HandleError(rw, "agent was not found in database", r.URL.Path, "WorkspaceAgentCoordinate",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
				http.StatusNotFound, "not found", nil)
			return
		}
		api.HandleError(rw, "failed to retrieve agent from database", r.URL.Path, "WorkspaceAgentCoordinate",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusInternalServerError, "internal server error", nil)
		return
	}

	conn, err := websocket.Accept(rw, r, nil)
	if err != nil {
		api.HandleError(rw, "failed to accept websocket connection", r.URL.Path, "WorkspaceAgentCoordinate",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"agent", fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
			http.StatusBadRequest, "Failed to accept websocket.", err)
		return
	}
	go Heartbeat(ctx, conn)

	ctx, wsConn := websocketNetConn(ctx, conn, websocket.MessageBinary)
	defer wsConn.Close()

	// log success here since the web socket will continue indefinitely
	api.Logger.LogDebugExternalAPI(
		"function execution successful",
		r.URL.Path,
		"WorkspaceAgentCoordinate",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		"agent",
		fmt.Sprintf("%d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64)),
		http.StatusOK,
		nil,
	)

	parentSpan.AddEvent(
		"workspace-agent-coordinate",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	now := time.Now()

	firstConnect := now
	if agent.FirstConnect != nil {
		firstConnect = *agent.FirstConnect
	}
	lastConnect := &now
	lastDisconnect := &now

	updateConnectionTimes := core.UpdateConnectionTimes(ctx, core.UpdateConnectionTimesOptions{
		DB:                api.DB,
		AgentId:           agentId.(int64),
		FirstConnect:      firstConnect,
		LastConnect:       lastConnect,
		LastDisconnect:    lastDisconnect,
		LastConnectedNode: api.ID,
	})

	defer func() {
		n := time.Now()
		lastDisconnect = &n
		_ = updateConnectionTimes()
		_ = api.RDB.Publish(context.TODO(), watchWorkspaceChannel(workspaceId.(int64)), []byte{})
	}()

	err = updateConnectionTimes()
	if err != nil {
		_ = conn.Close(websocket.StatusGoingAway, err.Error())
		return
	}
	api.publishWorkspaceUpdate(workspaceId.(int64))

	api.Logger.Infof("accepting agent: %d-%d-%d", agentId.(int64), workspaceId.(int64), ownerId.(int64))

	defer conn.Close(websocket.StatusNormalClosure, "")

	closeChan := make(chan struct{})
	go func() {
		defer close(closeChan)
		err := (*api.TailnetCoordinator.Load()).ServeAgent(wsConn, agentId.(int64))
		if err != nil {
			api.Logger.Warnf("tailnet coordinator agent error: %v", err)
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}
	}()
	ticker := time.NewTicker(api.AgentConnectionUpdateFrequency)
	defer ticker.Stop()
	for {
		select {
		case <-closeChan:
			return
		case <-ticker.C:
		}
		n := time.Now()
		lastConnect = &n
		err = updateConnectionTimes()
		if err != nil {
			_ = conn.Close(websocket.StatusGoingAway, err.Error())
			return
		}
	}
}

// wsNetConn wraps net.Conn created by websocket.NetConn(). Cancel func
// is called if a read or write error is encountered.
type wsNetConn struct {
	cancel context.CancelFunc
	net.Conn
}

func (c *wsNetConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err != nil {
		c.cancel()
	}
	return n, err
}

func (c *wsNetConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if err != nil {
		c.cancel()
	}
	return n, err
}

func (c *wsNetConn) Close() error {
	defer c.cancel()
	return c.Conn.Close()
}

// websocketNetConn wraps websocket.NetConn and returns a context that
// is tied to the parent context and the lifetime of the conn. Any error
// during read or write will cancel the context, but not close the
// conn. Close should be called to release context resources.
func websocketNetConn(ctx context.Context, conn *websocket.Conn, msgType websocket.MessageType) (context.Context, net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	nc := websocket.NetConn(ctx, conn, msgType)
	return ctx, &wsNetConn{
		cancel: cancel,
		Conn:   nc,
	}
}

func watchWorkspaceChannel(id int64) string {
	return fmt.Sprintf("workspace:%d", id)
}

func (api *WorkspaceAPI) publishWorkspaceUpdate(workspaceID int64) {
	cmd := api.RDB.Publish(context.TODO(), watchWorkspaceChannel(workspaceID), []byte{})
	if cmd.Err() != nil {
		api.Logger.Warnf("failed to publish workspace update: %d", cmd.Err())
	}
}
