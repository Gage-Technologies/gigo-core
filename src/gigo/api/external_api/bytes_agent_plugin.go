package external_api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/ws"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/sourcegraph/conc"
	"go.opentelemetry.io/otel"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type AgentWebSocketMessageType int

const (
	AgentWebSocketMessageTypeInit AgentWebSocketMessageType = iota
	AgentWebSocketMessageTypeValidationError
	AgentWebSocketMessageTypeGenericError
	AgentWebSocketMessageTypeExecRequest
	AgentWebSocketMessageTypeExecResponse
	AgentWebSocketMessageTypeLintRequest
	AgentWebSocketMessageTypeLintResponse
	AgentWebSocketMessageTypeCancelExecRequest
	AgentWebSocketMessageTypeCancelExecResponse
	AgentWebSocketMessageTypeStdinExecRequest
	AgentWebSocketMessageTypeStdinExecResponse
	AgentWebSocketMessageTypeLaunchLspRequest
	AgentWebSocketMessageTypeLaunchLspResponse
)

type AgentWebSocketMessageOrigin int

const (
	AgentWebSocketMessageOriginServer AgentWebSocketMessageOrigin = iota
	AgentWebSocketMessageOriginClient
)

type AgentWsRequestMessage struct {
	ByteAttemptID string `json:"byte_attempt_id" validate:"required,number"`
	Payload       any    `json:"payload" validate:"required"`
}

type AgentWebSocketPayload struct {
	SequenceID string                      `json:"sequence_id"`
	Type       AgentWebSocketMessageType   `json:"type" validate:"required,gte=0,lte=35"`
	Origin     AgentWebSocketMessageOrigin `json:"origin" validate:"required,gte=0,lte=1"`
	CreatedAt  int64                       `json:"created_at" validate:"required,gt=0"`
	Payload    any                         `json:"payload" validate:"required"`
}

type agentWebSocketConn struct {
	*websocket.Conn
	lastMessageTime *time.Time
	byteID          int64
	workspaceID     int64
}

type ByteLivePingRequest struct {
	ByteAttemptID string `json:"byte_attempt_id" validate:"required,number"`
}

type CancelExecRequestPayload struct {
	CommandID string `json:"command_id" validate:"number"`
}

type CancelExecResponsePayload struct {
	CommandID string `json:"command_id" validate:"number"`
}

type Difficulty int

const (
	Easy Difficulty = iota
	Medium
	Hard
)

func (d Difficulty) ToString() string {
	switch d {
	case Easy:
		return "easy"
	case Medium:
		return "medium"
	case Hard:
		return "hard"
	default:
		return "medium"
	}
}

type ByteUpdateCodeRequest struct {
	ByteAttemptID     string     `json:"byte_attempt_id" validate:"required,number"`
	Content           string     `json:"content" validate:"required"`
	ContentDifficulty Difficulty `json:"content_difficulty"`
}

type WebSocketPluginBytesAgent struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         *conc.WaitGroup
	s          *HTTPServer
	socket     *masterWebSocket
	agentConns map[int64]agentWebSocketConn
	mu         *sync.Mutex
	outputChan chan ws.Message[any]
}

func (p *WebSocketPluginBytesAgent) Name() string {
	return "bytesAgent"
}

func NewPluginByteAgent(ctx context.Context, s *HTTPServer, socket *masterWebSocket) (*WebSocketPluginBytesAgent, error) {
	// create output channel to send messages to the client
	outputChan := make(chan ws.Message[any])

	// create context
	ctx, cancel := context.WithCancel(ctx)

	// create lock to prevent concurrent access to the map
	lock := &sync.Mutex{}

	byteAgent := &WebSocketPluginBytesAgent{
		ctx:        ctx,
		cancel:     cancel,
		wg:         conc.NewWaitGroup(),
		s:          s,
		socket:     socket,
		agentConns: make(map[int64]agentWebSocketConn),
		mu:         lock,
		outputChan: outputChan,
	}

	return byteAgent, nil
}

func (p *WebSocketPluginBytesAgent) HandleMessage(msg *ws.Message[any]) {

	// skip any message that is not within the purview of the bytes agent plugin
	if msg.Type != ws.MessageTypeAgentExecRequest &&
		msg.Type != ws.MessageTypeAgentLintRequest &&
		msg.Type != ws.MessageTypeByteUpdateCode &&
		msg.Type != ws.MessageTypeByteLivePing &&
		msg.Type != ws.MessageTypeCancelExecRequest &&
		msg.Type != ws.MessageTypeStdinExecRequest &&
		msg.Type != ws.MessageTypeLaunchLspRequest {
		return
	}

	p.socket.logger.Debugf("(bytes-agent-ws) received request in agent plugin: %+v", msg)

	// marshal the inner payload
	innerBuf, err := json.Marshal(msg.Payload)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to marshal inner payload: %v", err)
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

	// handle byte live ping request
	if msg.Type == ws.MessageTypeByteLivePing {
		// extend the workspace expiration by 10 minutes
		p.extendWorkspaceExpiration(msg, innerBuf)
		return
	}

	// handle byte live ping request
	if msg.Type == ws.MessageTypeByteUpdateCode {
		// update the byte code
		p.updateByteAttemptCode(msg, innerBuf)
		return
	}

	// unmarshal the inner payload
	var agentReqMsg AgentWsRequestMessage
	err = json.Unmarshal(innerBuf, &agentReqMsg)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to unmarshal inner payload: %v", err)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, agentReqMsg) {
		return
	}

	p.socket.logger.Debugf("(bytes-agent-ws) handling agent request: %+v", agentReqMsg)

	// parse byte attempt id to int64
	byteAttemptID, _ := strconv.ParseInt(agentReqMsg.ByteAttemptID, 10, 64)

	user := p.socket.user.Load()
	if user == nil {
		p.socket.logger.Debug("(bytes-agent-ws) cannot find a user in the websocket")
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "cannot find user in the websocket",
			},
		)
		return
	}

	// check if byte attempt id is valid for the user
	var userID int64
	err = p.s.tiDB.DB.QueryRow(
		"select author_id from byte_attempts where _id = ?",
		byteAttemptID,
	).Scan(&userID)
	if err != nil {
		if err != sql.ErrNoRows {
			p.socket.logger.Errorf("(bytes-agent-ws) init code-server conn for workspace %d: failed to query workspace agent: %v", byteAttemptID, err)
			return
		}
		p.socket.logger.Infof("(bytes-agent-ws) cannot find byte attempt %s", byteAttemptID)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "cannot find byte attempt",
			},
		)
		return
	}

	if userID != user.ID {
		p.socket.logger.Debugf("(bytes-agent-ws) skipping init code-server conn for workspace %d because it is not for the current user", byteAttemptID)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "this user is not authorized for this byte attempt",
			},
		)
		return
	}

	// lock to access the agentConns map
	p.mu.Lock()
	conn, ok := p.agentConns[byteAttemptID]
	p.mu.Unlock()

	// connect to the byte agent if it doesn't exist
	if !ok {
		p.socket.logger.Debugf("(bytes-agent-ws) beginning connection to agent websocket")
		// check if the byte attempt has a valid workspace and get it's agent id & secret
		var agentId int64
		var secret string
		var workspaceId int64
		var workspaceState models.WorkspaceState
		err = p.s.tiDB.DB.QueryRow(
			"select wa._id as agent_id, bin_to_uuid(wa.secret) as secret, w._id as workspace_id, w.state as workspace_state from workspaces w join workspace_agent wa on w._id = wa.workspace_id where w.code_source_id = ? order by w._id desc limit 1",
			byteAttemptID,
		).Scan(&agentId, &secret, &workspaceId, &workspaceState)
		if err != nil {
			if err != sql.ErrNoRows {
				p.socket.logger.Errorf("(bytes-agent-ws) init code-server conn for workspace %d: failed to query workspace agent: %v", byteAttemptID, err)
				return
			}
			p.socket.logger.Infof("(bytes-agent-ws) no active agents found for workspace %s", byteAttemptID)
			// handle internal server error via websocket
			p.outputChan <- ws.PrepMessage[any](
				msg.SequenceID,
				ws.MessageTypeGenericError,
				ws.GenericErrorPayload{
					Code:  ws.ResponseCodeServerError,
					Error: "cannot find workspace or workspace agent",
				},
			)
			return
		}

		// return a specific message to the caller if the workspace is not alive yet
		if workspaceState != models.WorkspaceActive {
			p.socket.logger.Debugf("(bytes-agent-ws) skipping init code-server conn for workspace %d because it is not active", byteAttemptID)
			// handle internal server error via websocket
			p.outputChan <- ws.PrepMessage[any](
				msg.SequenceID,
				ws.MessageTypeGenericError,
				ws.GenericErrorPayload{
					Code:  ws.ResponseCodeServerError,
					Error: "workspace is not active",
				},
			)
			return
		}

		// placeholder for agent id & secrete
		workspaceID := workspaceId
		agentID := agentId
		agentSecret := secret

		// create a client that will dial using the ziti mesh
		client := http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (netConn net.Conn, e error) {
					// we dial the agent here using the zitimesh server which will
					// establish a connection to the end target on the agent over
					// the ziti net mesh ovelay
					return p.s.zitiServer.DialAgent(agentID, zitimesh.NetworkTypeTCP, int(agentsdk.ZitiAgentServerPort))
				},
			},
		}

		p.socket.logger.Debugf("(bytes-agent-ws) establishing connection to agent ws")

		// establish a new connection to the byte agent
		ac, acRes, err := websocket.Dial(p.ctx, fmt.Sprintf("ws://dummy/api/v1/ws"), &websocket.DialOptions{
			HTTPClient: &client,
			HTTPHeader: http.Header{
				"Authorization": []string{
					fmt.Sprintf("Bearer %s", agentSecret),
				},
			},
		})
		if err != nil {
			p.socket.logger.Errorf("(bytes-agent-ws) failed to dial byte agent: %v", err)
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

		// handle failed connection
		if ac == nil {
			p.socket.logger.Errorf("(bytes-agent-ws) failed to connect to byte agent: %d", acRes.StatusCode)
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

		// create a new agent connection
		n := time.Now()
		agentConn := agentWebSocketConn{
			Conn:            ac,
			lastMessageTime: &n,
			byteID:          byteAttemptID,
			workspaceID:     workspaceID,
		}

		// fire off reader routine
		p.wg.Go(func() {
			p.relayConnHandler(agentConn)
		})

		// add the agent connection to the map
		p.mu.Lock()
		p.agentConns[byteAttemptID] = agentConn
		p.mu.Unlock()

		// update the outer connection variable
		conn = agentConn
	}

	p.socket.logger.Debugf("(bytes-agent-ws) relaying request to agent ws")

	// forward the message to the byte agent
	agentMsg, err := formatPayloadForAgent(msg, agentReqMsg.Payload)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to format payload for agent: %v", err)
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
	buf, _ := json.Marshal(agentMsg)
	p.socket.logger.Debugf("(bytes-agent-ws) forwarding message to byte agent %d: %s", byteAttemptID, string(buf))
	err = wsjson.Write(p.ctx, conn.Conn, agentMsg)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to write message to agent: %v", err)
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
}

func (p *WebSocketPluginBytesAgent) OutgoingMessages() <-chan ws.Message[any] {
	return p.outputChan
}

func (p *WebSocketPluginBytesAgent) Close() {
	// cancel the context
	p.cancel()

	// close all subscriptions
	for _, c := range p.agentConns {
		_ = c.Close(websocket.StatusGoingAway, "backend shutdown")
	}

	// wait for the resource routine to exit
	p.wg.Wait()
}

func (p *WebSocketPluginBytesAgent) relayConnHandler(conn agentWebSocketConn) {
	p.socket.logger.Debugf("(bytes-agent-ws) launching relayConnHandler loop")

	// loop until the connection is closed
	for {
		// read the next message from the client
		var message AgentWebSocketPayload
		err := wsjson.Read(p.ctx, conn.Conn, &message)
		if err != nil {
			// remove the connection from the map
			p.mu.Lock()
			conn.Close(websocket.StatusAbnormalClosure, "failed to read")
			delete(p.agentConns, conn.byteID)
			p.mu.Unlock()
			return
		}

		buf, _ := json.Marshal(message)
		p.socket.logger.Debugf("(bytes-agent-ws) received response from agent ws %d: %s", conn.byteID, string(buf))

		// update the last interaction time
		t := time.Now()
		conn.lastMessageTime = &t

		// format message and send the message to the write loop
		m, err := formatPayloadFromAgent(message)
		if err != nil {
			p.socket.logger.Errorf("(bytes-agent-ws) failed to format payload from agent: %v", err)
			// handle internal server error via websocket
			p.outputChan <- ws.PrepMessage[any](
				message.SequenceID,
				ws.MessageTypeGenericError,
				ws.GenericErrorPayload{
					Code:  ws.ResponseCodeServerError,
					Error: "internal server error occurred",
				},
			)
			continue
		}
		if m.SequenceID == "" {
			continue
		}
		p.outputChan <- m
	}
}

func (p *WebSocketPluginBytesAgent) extendWorkspaceExpiration(msg *ws.Message[any], innerBuf []byte) {
	ctx, span := otel.Tracer("gigo-core").Start(p.ctx, "byte-extend-workspace-expiration")
	defer span.End()
	callerName := "extendWorkspaceExpiration"

	// unmarshal the inner payload
	var pingReq ByteLivePingRequest
	err := json.Unmarshal(innerBuf, &pingReq)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to unmarshal inner payload: %v", err)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, pingReq) {
		return
	}

	// parse byte attempt id to int64
	byteAttemptID, _ := strconv.ParseInt(pingReq.ByteAttemptID, 10, 64)

	_, err = p.s.tiDB.Exec(
		ctx, &span, &callerName,
		"update workspaces set expiration = ? where code_source_id = ? and owner_id = ? and state in (0, 1)",
		time.Now().Add(time.Minute*10), byteAttemptID, p.socket.user.Load().ID,
	)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to update workspace expiration: %v", err)
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
	return
}

func (p *WebSocketPluginBytesAgent) updateByteAttemptCode(msg *ws.Message[any], innerBuf []byte) {
	ctx, span := otel.Tracer("gigo-core").Start(p.ctx, "byte-update-attempt-code")
	defer span.End()
	callerName := "updateByteAttemptCode"

	// unmarshal the inner payload
	var updateReq ByteUpdateCodeRequest
	err := json.Unmarshal(innerBuf, &updateReq)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to unmarshal inner payload: %v", err)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, updateReq) {
		return
	}

	// parse byte attempt id to int64
	byteAttemptID, _ := strconv.ParseInt(updateReq.ByteAttemptID, 10, 64)

	_, err = p.s.tiDB.Exec(
		ctx, &span, &callerName,
		fmt.Sprintf("update byte_attempts set content_%s = ?, modified = true where _id = ?", updateReq.ContentDifficulty.ToString()),
		updateReq.Content, byteAttemptID,
	)
	if err != nil {
		p.socket.logger.Errorf("(bytes-agent-ws) failed to update byte attempt code: %v", err)
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
	return
}

func formatPayloadForAgent(msg *ws.Message[any], inner any) (AgentWebSocketPayload, error) {
	t := AgentWebSocketMessageTypeGenericError
	switch msg.Type {
	case ws.MessageTypeAgentExecRequest:
		t = AgentWebSocketMessageTypeExecRequest
	case ws.MessageTypeAgentExecResponse:
		t = AgentWebSocketMessageTypeExecResponse
	case ws.MessageTypeAgentLintRequest:
		t = AgentWebSocketMessageTypeLintRequest
	case ws.MessageTypeAgentLintResponse:
		t = AgentWebSocketMessageTypeLintResponse
	case ws.MessageTypeCancelExecRequest:
		t = AgentWebSocketMessageTypeCancelExecRequest
	case ws.MessageTypeCancelExecResponse:
		t = AgentWebSocketMessageTypeCancelExecResponse
	case ws.MessageTypeStdinExecRequest:
		t = AgentWebSocketMessageTypeStdinExecRequest
	case ws.MessageTypeStdinExecResponse:
		t = AgentWebSocketMessageTypeStdinExecResponse
	case ws.MessageTypeLaunchLspRequest:
		t = AgentWebSocketMessageTypeLaunchLspRequest
	default:
		return AgentWebSocketPayload{}, fmt.Errorf("unsupported message type: %v", msg.Type)
	}

	return AgentWebSocketPayload{
		SequenceID: msg.SequenceID,
		Type:       t,
		Origin:     AgentWebSocketMessageOriginClient,
		CreatedAt:  time.Now().Unix(),
		Payload:    inner,
	}, nil
}

func formatPayloadFromAgent(msg AgentWebSocketPayload) (ws.Message[any], error) {
	t := ws.MessageTypeGenericError
	switch msg.Type {
	case AgentWebSocketMessageTypeExecResponse:
		t = ws.MessageTypeAgentExecResponse
	case AgentWebSocketMessageTypeLintResponse:
		t = ws.MessageTypeAgentLintResponse
	case AgentWebSocketMessageTypeValidationError:
		t = ws.MessageTypeValidationError
	case AgentWebSocketMessageTypeCancelExecRequest:
		t = ws.MessageTypeCancelExecRequest
	case AgentWebSocketMessageTypeCancelExecResponse:
		t = ws.MessageTypeCancelExecResponse
	case AgentWebSocketMessageTypeStdinExecRequest:
		t = ws.MessageTypeStdinExecRequest
	case AgentWebSocketMessageTypeStdinExecResponse:
		t = ws.MessageTypeStdinExecResponse
	case AgentWebSocketMessageTypeGenericError:
		t = ws.MessageTypeGenericError
	case AgentWebSocketMessageTypeLaunchLspResponse:
		t = ws.MessageTypeLaunchLspResponse
	case AgentWebSocketMessageTypeInit:
		return ws.Message[any]{}, nil
	default:
		return ws.Message[any]{}, fmt.Errorf("unsupported message type: %v", msg.Type)
	}

	return ws.Message[any]{
		SequenceID: msg.SequenceID,
		Type:       t,
		Payload:    msg.Payload,
	}, nil
}
