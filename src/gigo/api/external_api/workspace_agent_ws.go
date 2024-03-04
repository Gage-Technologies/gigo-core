package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/ws"
	"nhooyr.io/websocket"
	"time"
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
	// deprecated
	ByteAttemptID string `json:"byte_attempt_id"`

	CodeSourceID string `json:"code_source_id"`
	Payload      any    `json:"payload" validate:"required"`
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
