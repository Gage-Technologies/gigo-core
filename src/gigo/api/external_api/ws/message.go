package ws

type MessageType int

type ResponseCode int

const (
	ResponseCodeBadRequest ResponseCode = iota
	ResponseCodeServerError
)

type Message[T any] struct {
	SequenceID string      `json:"sequence_id" validate:"required"`
	Type       MessageType `json:"type" validate:"required,lte=19"`
	Payload    T           `json:"payload" validate:"required"`
}

type GenericErrorPayload struct {
	Code  ResponseCode `json:"code" validate:"required"`
	Error string       `json:"error" validate:"required"`
}

type ValidationErrorPayload struct {
	GenericErrorPayload
	ValidationErrors map[string]string `json:"validation_errors" validate:"required"`
}

const (
	MessageTypeNewIncomingChatMessage MessageType = iota
	MessageTypeNewChat
	MessageTypeNewOutgoingChatMessage
	MessageTypeValidationError
	MessageTypeGenericError
	MessageTypeSubscribeWorkspace
	MessageTypeWorkspaceStatusUpdate
	MessageTypeUnsubscribeWorkspace
	MessageTypeUpdateChat
	MessageTypeKickChat
	MessageTypeGetChats
	MessageTypeGetChatMessages
	MessageTypeNewChatBroadcast
	MessageTypeChatSubscribe
	MessageTypeChatUnsubscribe
	MessageTypeDeleteChat
	MessageTypeUpdateReadMessage
	MessageTypeUpdateChatMute
	MessageTypeChatUpdatedEvent
	MessageTypeRecordWebUsage
)

func (t MessageType) String() string {
	return [...]string{
		"MessageTypeNewIncomingChatMessage",
		"MessageTypeNewChat",
		"MessageTypeNewOutgoingChatMessage",
		"MessageTypeValidationError",
		"MessageTypeGenericError",
		"MessageTypeSubscribeWorkspace",
		"MessageTypeWorkspaceStatusUpdate",
		"MessageTypeUnsubscribeWorkspace",
		"MessageTypeUpdateChat",
		"MessageTypeKickChat",
		"MessageTypeGetChats",
		"MessageTypeGetChatMessages",
		"MessageTypeNewChatBroadcast",
		"MessageTypeChatSubscribe",
		"MessageTypeChatUnsubscribe",
		"MessageTypeDeleteChat",
		"MessageTypeUpdateReadMessage",
		"MessageTypeUpdateChatMute",
		"MessageTypeChatUpdatedEvent",
		"MessageTypeRecordWebUsage",
	}[t]
}
