package external_api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/ws"
	"github.com/gage-technologies/gigo-lib/db/models"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"tailscale.com/net/speedtest"
)

type WorkspaceSubscribeParams struct {
	WorkspaceID string `json:"workspace_id" validate:"required,number"`
}

type WebSocketPluginWorkspace struct {
	ctx        context.Context
	s          *HTTPServer
	socket     *masterWebSocket
	wsSubs     map[int64]*nats.Subscription
	lastStates map[string]*models.WorkspaceFrontend
	mu         *sync.Mutex
	outputChan chan ws.Message[any]
}

func NewPluginWorkspace(ctx context.Context, s *HTTPServer, socket *masterWebSocket) (*WebSocketPluginWorkspace, error) {
	// create output channel to send messages to the client
	outputChan := make(chan ws.Message[any])

	return &WebSocketPluginWorkspace{
		ctx:        ctx,
		s:          s,
		socket:     socket,
		mu:         &sync.Mutex{},
		wsSubs:     make(map[int64]*nats.Subscription),
		lastStates: make(map[string]*models.WorkspaceFrontend),
		outputChan: outputChan,
	}, nil
}

func (p *WebSocketPluginWorkspace) Name() string {
	return "workspace"
}

func (p *WebSocketPluginWorkspace) HandleMessage(msg *ws.Message[any]) {
	// skip any message that is not within the purview of the workspace plugin
	if msg.Type != ws.MessageTypeSubscribeWorkspace &&
		msg.Type != ws.MessageTypeUnsubscribeWorkspace {
		return
	}

	// load the user from the socket
	user := p.socket.user.Load()

	// discard any message if the user is not logged in
	if user == nil {
		return
	}

	// handle the subscription request

	// marshal the inner payload
	innerBuf, err := json.Marshal(msg.Payload)
	if err != nil {
		p.socket.logger.Errorf("failed to marshal inner payload: %v", err)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "internal server error occurred",
			},
		)
		return
	}

	// unmarshal the inner payload
	var subReq WorkspaceSubscribeParams
	err = json.Unmarshal(innerBuf, &subReq)
	if err != nil {
		p.socket.logger.Errorf("failed to unmarshal inner payload: %v", err)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "internal server error occurred",
			},
		)
		return
	}

	// validate the new message payload
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, subReq) {
		p.socket.logger.Errorf("failed to validate payload: %s", string(innerBuf))
		return
	}

	// parse workspace id
	workspaceId, _ := strconv.ParseInt(subReq.WorkspaceID, 10, 64)

	// if this is an unsubscribe request then unsubscribe from the workspace and exit
	if msg.Type == ws.MessageTypeUnsubscribeWorkspace {
		p.mu.Lock()
		defer p.mu.Unlock()
		if sub, ok := p.wsSubs[workspaceId]; ok {
			_ = sub.Unsubscribe()
			delete(p.wsSubs, workspaceId)
		}
		return
	}

	// perform initial status call for complete status data
	status, err := core.GetWorkspaceStatus(p.ctx, p.s.tiDB, p.s.vscClient, user, workspaceId, p.s.hostname, p.s.useTls)
	if err != nil {
		p.socket.logger.Errorf("failed to get workspace status: %v", err)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "internal server error occurred",
			},
		)
		return
	}

	// format the message and send it to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeWorkspaceStatusUpdate,
		status,
	)

	// exit if we already have a subscription for this workspace
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.wsSubs[workspaceId]; ok {
		return
	}

	// create a subscriber to workspace status events
	sub, err := p.s.jetstreamClient.Subscribe(
		fmt.Sprintf(streams.SubjectWorkspaceStatusUpdateDynamic, workspaceId),
		p.workspaceStatusCallback,
	)
	if err != nil {
		p.socket.logger.Errorf("failed to create workspace status subscriber: %v", err)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "internal server error occurred",
			},
		)
		return
	}

	// save the subscription
	p.wsSubs[workspaceId] = sub
}

func (p *WebSocketPluginWorkspace) OutgoingMessages() <-chan ws.Message[any] {
	return p.outputChan
}

func (p *WebSocketPluginWorkspace) Close() {
	// close all subscriptions
	for _, sub := range p.wsSubs {
		_ = sub.Unsubscribe()
	}
}

func (p *WebSocketPluginWorkspace) workspaceStatusCallback(msg *nats.Msg) {
	// defer the ack of the message
	defer msg.Ack()

	// create new frontend workspace model to decode msg into
	var statusMsg models2.WorkspaceStatusUpdateMsg
	decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	err := decoder.Decode(&statusMsg)
	if err != nil {
		p.s.logger.Errorf("failed to decode workspace status message: %v", err)
		return
	}

	// get last workspace state
	p.mu.Lock()
	lastWorkspaceState, ok := p.lastStates[statusMsg.Workspace.ID]
	p.mu.Unlock()

	// if this is a workspace start completed message then we should stall
	// the notification while we initialize a connection to the workspace
	if statusMsg.Workspace.InitState == models.WorkspaceInitCompleted &&
		(ok && lastWorkspaceState.InitState != models.WorkspaceInitCompleted) {
		ctx, parentSpan := otel.Tracer("gigo-core").Start(p.ctx, "workspace-test-agent-conn")
		callerName := "WorkspaceWebSocketPlugin"
		defer parentSpan.End()

		var agentId int64
		err = p.s.tiDB.QueryRow(ctx, &parentSpan, &callerName, "SELECT _id FROM workspace_agent WHERE workspace_id = ? limit 1", statusMsg.Workspace.ID).Scan(&agentId)
		if err != nil {
			p.s.logger.Errorf("failed to query workspace agent: %v", err)
			return
		}

		p.s.logger.Debugf("WorkspaceWebSocket (%s): workspace start completed, acquiring connection to workspace agent", statusMsg.Workspace.ID)

		// create a new http request to the workspace agent
		r, err := http.NewRequestWithContext(p.ctx, http.MethodGet, fmt.Sprintf("http://localhost:13337/healthz"), nil)
		if err != nil {
			p.s.logger.Errorf("failed to create http request: %v", err)
			return
		}

		// acquire a connection to the workspace agent
		conn, release, err := p.s.WorkspaceAgentCache.Acquire(r, agentId)
		if err != nil {
			p.s.logger.Errorf("WorkspaceWebSocket (%s): failed to acquire connection to workspace agent: %v", statusMsg.Workspace.ID, err)
			return
		}

		p.s.logger.Debugf("WorkspaceWebSocket (%s): workspace agent connection acquired; awaiting network reachability", statusMsg.Workspace.ID)

		// parse id to int
		id, _ := strconv.ParseInt(statusMsg.Workspace.ID, 10, 64)

		// wait up to 10s for the workspace agent to become reachable
		reachableCtx, cancelReachableCtx := context.WithTimeout(context.TODO(), time.Second*10)
		reachable := conn.AwaitReachable(reachableCtx)
		cancelReachableCtx()
		if !reachable {
			p.s.logger.Errorf("WorkspaceWebSocket (%s): workspace agent connection failed to become reachable; dropping connection and re-establishing", statusMsg.Workspace.ID)
			// release the connection from the cache and close the connection
			// we need to create a new connection to the workspace agent
			release()
			p.s.WorkspaceAgentCache.ForgetAndClose(agentId)

			// create a new connection to the workspace agent
			conn, release, err = p.s.WorkspaceAgentCache.Acquire(r, agentId)
			if err != nil {
				p.s.logger.Errorf("WorkspaceWebSocket (%s): failed to acquire connection to workspace agent: %v", statusMsg.Workspace.ID, err)
				release()
				return
			}

			// make another attempt to wait for the workspace agent to become reachable
			// but this time wait up to 30s
			reachableCtx, cancelReachableCtx := context.WithTimeout(context.TODO(), time.Second*30)
			reachable := conn.AwaitReachable(reachableCtx)
			cancelReachableCtx()
			if !reachable {
				release()
				// fail here since we can't connect to the workspace agent
				p.s.logger.Errorf("WorkspaceWebSocket (%s): workspace agent is not reachable", statusMsg.Workspace.ID)
				// mark the workspace as failed
				err = core.WorkspaceInitializationFailure(p.ctx, p.s.tiDB, p.s.wsStatusUpdater, id,
					models.WorkspaceInitVSCodeLaunch, "connecting to workspace", -1,
					"", "failed to establish connection to workspace", p.s.jetstreamClient)
				if err != nil {
					p.s.logger.Errorf("WorkspaceWebSocket (%s): failed to mark workspace as failed: %v", statusMsg.Workspace.ID, err)
					return
				}
			}
		}

		p.s.logger.Debugf("WorkspaceWebSocket (%s): workspace agent is reachable; running speedtest", statusMsg.Workspace.ID)

		// make a direct http request to the workspace agent to initialize the connection
		_, err = conn.Speedtest(p.ctx, speedtest.Download, time.Second)
		if err != nil {
			p.s.logger.Errorf("WorkspaceWebSocket (%s): failed to initialize connection to workspace agent: %v", statusMsg.Workspace.ID, err)
			release()
			return
		}

		p.s.logger.Debugf("WorkspaceWebSocket (%s): workspace agent connection initialized; waiting agent ready", statusMsg.Workspace.ID)

		// wait for the workspace agent to become ready
		timeout := time.After(time.Second * 30)
		for {
			exit := false
			select {
			case <-p.ctx.Done():
				return
			case <-timeout:
				p.s.logger.Errorf("WorkspaceWebSocket (%d): failed to wait agent ready", statusMsg.Workspace.ID)
				exit = true
				break
			default:
				// check if the agent is ready
				err = p.s.tiDB.QueryRowContext(ctx, &parentSpan, &callerName,
					"select _id from workspace_agent a where workspace_id = ? and a.state = ? order by a.created_at desc limit 1",
					id, models.WorkspaceAgentStateRunning,
				).Scan(&agentId)
				if err != nil {
					if err != sql.ErrNoRows {
						p.s.logger.Errorf("WorkspaceWebSocket (%s): failed to query workspace agent: %v", statusMsg.Workspace.ID, err)
					}
					p.s.logger.Debugf("WorkspaceWebSocket (%s): workspace agent not ready yet", statusMsg.Workspace.ID)
					time.Sleep(time.Second)
					continue
				}
				exit = true
				break
			}

			if exit {
				break
			}
		}

		p.s.logger.Debugf("WorkspaceWebSocket (%s): workspace agent is ready", statusMsg.Workspace.ID)

		// release the connection
		release()
		cancelReachableCtx()
	}

	// write status to the websocket
	p.outputChan <- ws.PrepMessage[any](
		"",
		ws.MessageTypeWorkspaceStatusUpdate,
		statusMsg,
	)

	// update the last workspace state
	p.mu.Lock()
	p.lastStates[statusMsg.Workspace.ID] = statusMsg.Workspace
	p.mu.Unlock()
}
