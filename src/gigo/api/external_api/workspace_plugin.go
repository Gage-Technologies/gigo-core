package external_api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gigo-core/gigo/api/external_api/core"
	"gigo-core/gigo/api/external_api/ws"

	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"github.com/gage-technologies/gigo-lib/db/models"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/nats-io/nats.go"
	"github.com/sourcegraph/conc"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/singleflight"
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

	// launch the init conn ping routine
	plugin.wg.Go(plugin.wsPingRoutine)

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

		// get the workspace agent id
		var agentId int64
		err := p.s.tiDB.DB.QueryRow(
			"select _id from workspace_agent where workspace_id = ? order by created_at desc limit 1",
			statusMsg.Workspace.ID,
		).Scan(&agentId)
		if err != nil {
			if err != sql.ErrNoRows {
				p.socket.logger.Errorf("init code-server conn for workspace %d: failed to query workspace agent: %v", statusMsg.Workspace.ID, err)
				return
			}
			p.socket.logger.Infof("no active agents found for workspace %s", statusMsg.Workspace.ID)
		}

		p.s.logger.Debugf("initializing agent connection: %s", statusMsg.Workspace.ID)

		// we record the start time of our ping loop so we can exit if it takes longer than 5s
		startTime := time.Now()
		for {
			// exit if it's been more than five seconds since we started waiting
			if time.Since(startTime) > 5*time.Second {
				p.socket.logger.Warnf("timed out initializing agent connection: %s", statusMsg.Workspace.ID)
				break
			}

			// create a client that will dial using the ziti mesh
			client := http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (netConn net.Conn, e error) {
						// we dial the agent here using the zitimesh server which will
						// establish a connection to the end target on the agent over
						// the ziti net mesh ovelay
						return p.s.zitiServer.DialAgent(agentId, zitimesh.NetworkTypeTCP, agentsdk.ZitiAgentServerPort)
					},
				},
			}

			// execute a get request to the /healthz endpoint - this warms up the connection to the agent via ziti
			resp, err := client.Get("http://dummy/healthz")
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if err != nil {
				p.socket.logger.Debugf("failed to warmup agent connection for workspace %s: %v", statusMsg.Workspace.ID, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			if resp.StatusCode != 200 {
				p.socket.logger.Debugf("agent healthcheck returned non-successful response for workspace %s: %s", statusMsg.Workspace.ID, resp.StatusCode)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			break
		}

		p.s.logger.Debugf("waiting agent ready: %s", statusMsg.Workspace.ID)

		// wait for the workspace agent to become ready
		timeout := time.After(time.Second * 30)
		for {
			exit := false
			select {
			case <-p.ctx.Done():
				return
			case <-timeout:
				p.s.logger.Errorf("failed to wait agent ready: %s", statusMsg.Workspace.ID)
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
						p.s.logger.Errorf("failed to query workspace agent %s: %v", statusMsg.Workspace.ID, err)
					}
					p.s.logger.Debugf("workspace agent not ready yet: %s", statusMsg.Workspace.ID)
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

		p.s.logger.Debugf("workspace agent is ready: %s", statusMsg.Workspace.ID)
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

				// skip if the state is not running
				if wsState.State != models.WorkspaceActive {
					continue
				}

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

func (p *WebSocketPluginWorkspace) wsPingRoutine() {
	// loop forever pinging workspaces in start up
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// create a singleflight group so only one goroutine can call the ping function at once
	var g singleflight.Group

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			// iterate all the workspaces pinging the ones in a starting state
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
					p.socket.logger.Warnf("skipping init conn ping for workspace %d: missing state", id)
					continue
				}
				if wsState == nil {
					p.socket.logger.Debugf("nil state in init conn  ping routine %d", id)
					continue
				}

				p.socket.logger.Debugf("attempting to init conn ping for workspace %d", id)

				// skip if the state is not running
				if wsState.State != models.WorkspaceStarting {
					p.socket.logger.Debugf("skipping init conn ping for workspace %d because its state is not 'starting'", id)
					continue
				}

				p.socket.logger.Debugf("attempting to init conn ping for workspace %d", id)

				// get the workspace agent id
				var agentId int64
				err := p.s.tiDB.DB.QueryRow(
					"select _id from workspace_agent where workspace_id = ? order by created_at desc limit 1",
					id,
				).Scan(&agentId)
				if err != nil {
					if err != sql.ErrNoRows {
						p.socket.logger.Errorf("init conn ping for workspace %d: failed to query workspace agent: %v", id, err)
						continue
					}
					p.socket.logger.Infof("no active agents found for workspace %d", id)
					continue
				}

				// attempt to ping the init connection server on the agent
				pingFunc := func() (interface{}, error) {
					p.socket.logger.Debugf("attempting to ping workspace %d", id)

					// create a client that will dial using the ziti mesh
					client := http.Client{
						Transport: &http.Transport{
							DialContext: func(ctx context.Context, network, addr string) (netConn net.Conn, e error) {
								// we dial the agent here using the zitimesh server which will
								// establish a connection to the end target on the agent over
								// the ziti net mesh ovelay
								return p.s.zitiServer.DialAgent(agentId, zitimesh.NetworkTypeTCP, agentsdk.ZitiAgentServerPort)
							},
						},
					}

					// execute a get request to the /ping endpoint
					resp, err := client.Get("http://dummy/ping")
					if resp != nil && resp.Body != nil {
						defer resp.Body.Close()
					}

					if resp != nil && resp.StatusCode == 200 {
						p.socket.logger.Debugf("successfully pinged init conn workspace %d", id)
					} else {
						var code int
						var buf []byte
						if resp != nil {
							code = resp.StatusCode
							if resp.Body != nil {
								buf, _ = io.ReadAll(resp.Body)
							}
						}
						p.socket.logger.Debugf("failed to ping init conn for workspace %d: %d - %v - %s", id, code, err, string(buf))
					}

					// NOTE: we really don't have to handle anything here - just making the connection is good enough
					return nil, nil
				}

				// launch the ping function via the singleflight
				p.wg.Go(func() {
					_, _, _ = g.Do(strconv.FormatInt(id, 10), pingFunc)
				})
			}
		}
	}
}
