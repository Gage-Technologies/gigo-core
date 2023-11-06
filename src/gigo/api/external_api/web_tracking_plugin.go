package external_api

import (
	"context"
	"encoding/json"
	"gigo-core/gigo/api/external_api/core"
	"gigo-core/gigo/api/external_api/ws"
	"time"
)

type WebSocketPluginWebTracking struct {
	ctx    context.Context
	s      *HTTPServer
	socket *masterWebSocket
}

func NewPluginWebTracking(ctx context.Context, s *HTTPServer, socket *masterWebSocket) *WebSocketPluginWebTracking {
	return &WebSocketPluginWebTracking{
		ctx:    ctx,
		s:      s,
		socket: socket,
	}
}

func (p *WebSocketPluginWebTracking) OutgoingMessages() <-chan ws.Message[any] {
	return make(<-chan ws.Message[any], 1)
}

func (p *WebSocketPluginWebTracking) Name() string {
	return "WebTracking"
}

func (p *WebSocketPluginWebTracking) Close() {}

func (p *WebSocketPluginWebTracking) HandleMessage(msg *ws.Message[any]) {
	// skip any message that is not within the purview of the web tracking plugin
	if msg.Type != ws.MessageTypeRecordWebUsage {
		return
	}

	// load the user from the socket
	user := p.socket.user.Load()

	// handle the subscription request

	// marshal the inner payload
	innerBuf, err := json.Marshal(msg.Payload)
	if err != nil {
		p.socket.logger.Errorf("failed to marshal inner payload: %v", err)
		return
	}

	// unmarshal the inner payload
	var subReq core.RecordWebUsageParams
	err = json.Unmarshal(innerBuf, &subReq)
	if err != nil {
		p.socket.logger.Errorf("failed to unmarshal inner payload: %v", err)
		return
	}

	// validate the new message payload
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, subReq) {
		p.socket.logger.Errorf("failed to validate payload: %s", string(innerBuf))
		return
	}

	// add the IP, user id and timestamp
	subReq.IP = p.socket.ipAddr
	subReq.Timestamp = time.Now()
	if user != nil {
		subReq.UserID = &user.ID
	}

	// send the usage record to the database
	core.RecordWebUsage(p.ctx, p.s.tiDB, p.s.sf, &subReq)
}
