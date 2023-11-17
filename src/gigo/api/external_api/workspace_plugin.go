package external_api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"gigo-core/gigo/api/external_api/core"
	"gigo-core/gigo/api/external_api/ws"

	"github.com/gage-technologies/gigo-lib/db/models"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc"
	"go.opentelemetry.io/otel"
)

type WorkspaceSubscribeParams struct {
	WorkspaceID string `json:"workspace_id" validate:"required,number"`
}

type WebSocketPluginWorkspace struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         *conc.WaitGroup
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

	// create context
	ctx, cancel := context.WithCancel(ctx)

	plugin := &WebSocketPluginWorkspace{
		ctx:        ctx,
		cancel:     cancel,
		wg:         conc.NewWaitGroup(),
		s:          s,
		socket:     socket,
		mu:         &sync.Mutex{},
		wsSubs:     make(map[int64]*nats.Subscription),
		lastStates: make(map[string]*models.WorkspaceFrontend),
		outputChan: outputChan,
	}

	// launch the resource routine
	plugin.wg.Go(plugin.resourceUtilRoutine)

	// return the new plugin instance
	return plugin, nil
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

	// save the latest state
	p.mu.Lock()
	p.lastStates[fmt.Sprintf("%d", workspaceId)] = status["workspace"].(*models.WorkspaceFrontend)
	p.mu.Unlock()

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
	// cancel the context
	p.cancel()

	// close all subscriptions
	for _, sub := range p.wsSubs {
		_ = sub.Unsubscribe()
	}

	// wait for the resource routine to exit
	p.wg.Wait()
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

		p.s.logger.Debugf("waiting agent ready: %d", statusMsg.Workspace.ID)

		// wait for the workspace agent to become ready
		timeout := time.After(time.Second * 30)
		for {
			exit := false
			select {
			case <-p.ctx.Done():
				return
			case <-timeout:
				p.s.logger.Errorf("failed to wait agent ready: %d", statusMsg.Workspace.ID)
				exit = true
				break
			default:
				// check if the agent is ready
				var agentId int64
				err = p.s.tiDB.QueryRowContext(ctx, &parentSpan, &callerName,
					"select _id from workspace_agent a where workspace_id = ? and a.state = ? order by a.created_at desc limit 1",
					statusMsg.Workspace.ID, models.WorkspaceAgentStateRunning,
				).Scan(&agentId)
				if err != nil {
					if err != sql.ErrNoRows {
						p.s.logger.Errorf("failed to query workspace agent %d: %v", statusMsg.Workspace.ID, err)
					}
					p.s.logger.Debugf("workspace agent not ready yet: %d", statusMsg.Workspace.ID)
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

		p.s.logger.Debugf("workspace agent is ready: %d", statusMsg.Workspace.ID)
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

func (p *WebSocketPluginWorkspace) resourceUtilRoutine() {
	// loop forever loading the resource utilization data
	// for each workspace every 5s
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	user := p.socket.user.Load()
	if user == nil {
		p.socket.logger.Debug("resource util routine exited without an initial user")
		return
	}
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			// load the resource utilization data for all workspaces
			for id := range p.wsSubs {
				// check if we are exiting
				select {
				case <-p.ctx.Done():
					return
				default:
				}

				// retrieve the latest state for the workspace
				p.mu.Lock()
				wsState, ok := p.lastStates[fmt.Sprintf("%d", id)]
				p.mu.Unlock()
				if !ok {
					p.socket.logger.Warnf("skipping resource utilization data retrieval for workspace %d-%d: missing state", user.ID, id)
					continue
				}
				if wsState == nil {
					p.socket.logger.Debugf("nil state in resource utilization routine %d-%d", user.ID, id)
					continue
				}

				p.socket.logger.Debugf("loading resource utilization data for workspace %d-%d", user.ID, id)

				// skip if the state is not running
				if wsState.State != models.WorkspaceActive {
					p.socket.logger.Debugf("workspace resource utilization skipped for invalid state %d-%d: %d", user.ID, id, wsState.State)
					continue
				}

				p.socket.logger.Debugf("attempting to retrieve resource utilization data for workspace %d-%d", user.ID, id)

				// attempt to retrieve the resource utilization data for the workspace
				ctx, cancel := context.WithTimeout(context.TODO(), time.Second*10)
				util, err := p.s.workspaceClient.GetResourceUtil(ctx, id, user.ID)
				cancel()
				if err != nil {
					p.socket.logger.Errorf("failed to retrieve resource utilization data for workspace %d-%d: %v", user.ID, id, err)
					continue
				}

				// retrieve the latest state for the workspace in case it changed
				p.mu.Lock()
				wsState = p.lastStates[fmt.Sprintf("%d", id)]
				p.mu.Unlock()

				// write status to the websocket
				p.outputChan <- ws.PrepMessage[any](
					"",
					ws.MessageTypeWorkspaceStatusUpdate,
					models2.WorkspaceStatusUpdateMsg{
						Workspace: wsState,
						Resources: &models2.WorkspaceResourceUtil{
							CPU:         util.CPU,
							Memory:      util.Mem,
							CPULimit:    util.CPULimit,
							MemoryLimit: util.MemLimit,
							CPUUsage:    util.CPUUsage,
							MemoryUsage: util.MemUsage,
						},
					},
				)
			}
		}
	}
}
