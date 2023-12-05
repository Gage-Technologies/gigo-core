package external_api

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"gigo-core/gigo/api/external_api/core"
	"gigo-core/gigo/api/external_api/ws"

	"github.com/gage-technologies/gigo-lib/db/models"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/nats-io/nats.go"
)

type ChatKickResponse struct {
	ChatID string `json:"chat_id"`
}

type GetChatsResponse struct {
	Chats []*models.ChatFrontend `json:"chats"`
}

type GetChatMessagesResponse struct {
	Messages []*models.ChatMessageFrontend `json:"messages"`
}

type ChatSubscribeParams struct {
	ChatID   string          `json:"chat_id" validate:"required,number"`
	ChatType models.ChatType `json:"chat_type" validate:"oneof=0 1 5"`
}

type ChatSubscribeResponse struct {
	Chat *models.ChatFrontend `json:"chat"`
}

type ChatUnsubscribeParams struct {
	ChatID string `json:"chat_id" validate:"required,number"`
}

type ChatUnsubscribeResponse struct {
	ChatID string `json:"chat_id"`
}

type UpdateReadMessageResponse struct {
	MessageID string `json:"message_id"`
}

type UpdateChatMuteResponse struct {
	ChatID string `json:"chat_id"`
}

type DeleteChatResponse struct {
	ChatID string `json:"chat_id"`
}

type WebSocketPluginChat struct {
	ctx            context.Context
	s              *HTTPServer
	socket         *masterWebSocket
	mu             *sync.Mutex
	chats          map[int64]*models.Chat
	subs           map[int64]*nats.Subscription
	activeChat     *nats.Subscription
	kickChat       *nats.Subscription
	eventStream    *nats.Subscription
	outputChan     chan ws.Message[any]
	chatMsgHandler func(m *nats.Msg)
}

func NewPluginChat(ctx context.Context, s *HTTPServer, socket *masterWebSocket) (*WebSocketPluginChat, error) {
	// load the user from the socket
	callingUser := socket.user.Load()

	// only retrieve chats if the user is logged in
	var chats []*models.Chat
	var err error
	if callingUser != nil {
		// query the database for the user's chats
		chats, err = core.GetChatsInternal(ctx, s.tiDB, callingUser, core.GetChatsParams{
			Offset: 0,
			Limit:  250,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve chats: %w", err)
		}
	}

	// append the global chat to the list of chats
	// TODO: handle regional chats
	chats = append(chats, &models.Chat{
		ID:   0,
		Name: "Global",
		Type: models.ChatTypeGlobal,
	})

	// create output channel to send messages to the client
	outputChan := make(chan ws.Message[any])

	// create a function to handle stream messages from users in chats
	chatMessageHandler := func(m *nats.Msg) {
		// we always ack the message
		_ = m.Ack()

		// parse the message
		var chatMessage models2.NewMessageMsg
		decoder := gob.NewDecoder(bytes.NewBuffer(m.Data))
		err := decoder.Decode(&chatMessage)
		if err != nil {
			s.logger.Errorf("failed to decode chat message: %v", err)
			return
		}

		// skip if this message is from the calling user
		user := socket.user.Load()
		if user != nil && chatMessage.Message.AuthorID == user.ID {
			return
		}

		// format the chat message to a frontend message
		frontendMessage := chatMessage.Message.ToFrontend()

		// send the message to the client
		outputChan <- ws.PrepMessage[any](
			"",
			ws.MessageTypeNewIncomingChatMessage,
			frontendMessage,
		)
	}

	// iterate over the chats launching subs for each chat
	subs := make(map[int64]*nats.Subscription)
	chatMap := make(map[int64]*models.Chat)
	for _, chat := range chats {
		// create an async subscriber to broadcast init messages
		sub, err := s.jetstreamClient.Subscribe(
			fmt.Sprintf(streams.SubjectChatMessagesDynamic, chat.ID),
			chatMessageHandler,
			nats.AckExplicit(),
			nats.DeliverNew(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to chat stream: %w", err)
		}
		subs[chat.ID] = sub
		chatMap[chat.ID] = chat
	}

	// create lock to prevent concurrent access to the map
	lock := &sync.Mutex{}

	// only launch a new chat and chat kick handlers if the calling user is no nil
	// anon users will never have new chats or be kicked from chats
	var activeChatStream *nats.Subscription
	var chatKickStream *nats.Subscription
	var eventStream *nats.Subscription
	if callingUser != nil {
		// create function to handle stream messages for new chats
		newChatHandler := func(m *nats.Msg) {
			// we always ack the message
			_ = m.Ack()

			// parse the message
			var newChat models2.NewChatMsg
			decoder := gob.NewDecoder(bytes.NewBuffer(m.Data))
			err := decoder.Decode(&newChat)
			if err != nil {
				s.logger.Errorf("failed to decode new chat message: %v", err)
				return
			}

			// skip if we already have a subscription for this chat
			lock.Lock()
			defer lock.Unlock()
			if _, ok := subs[newChat.Chat.ID]; ok {
				return
			}

			// create an async subscriber to broadcast init messages
			sub, err := s.jetstreamClient.Subscribe(
				fmt.Sprintf(streams.SubjectChatMessagesDynamic, newChat.Chat.ID),
				chatMessageHandler,
				nats.AckExplicit(),
				nats.DeliverNew(),
			)
			if err != nil {
				s.logger.Errorf("failed to subscribe to new chat stream: %v", err)
				return
			}

			subs[newChat.Chat.ID] = sub
			chatMap[newChat.Chat.ID] = &newChat.Chat

			// send the message to the client to add the chat
			outputChan <- ws.PrepMessage[any](
				"",
				ws.MessageTypeNewChatBroadcast,
				newChat.Chat.ToFrontend(),
			)
		}

		// create an async subscriber to broadcast init messages
		activeChatStream, err = s.jetstreamClient.Subscribe(
			fmt.Sprintf(streams.SubjectChatNewChatDynamic, callingUser.ID),
			newChatHandler,
			nats.AckExplicit(),
			nats.DeliverNew(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to new chat stream: %w", err)
		}

		// create function to handle stream messages for chat kicks
		kickChatHandler := func(m *nats.Msg) {
			// we always ack the message
			_ = m.Ack()

			// parse the message
			var chatKick models2.ChatKickMsg
			decoder := gob.NewDecoder(bytes.NewBuffer(m.Data))
			err := decoder.Decode(&chatKick)
			if err != nil {
				s.logger.Errorf("failed to decode chat kick message: %v", err)
				return
			}

			socket.logger.Debugf("handling kick message %d: %d", callingUser.ID, chatKick.ChatID)

			// send the message to the client to remove the chat
			outputChan <- ws.PrepMessage[any](
				"",
				ws.MessageTypeKickChat,
				ChatKickResponse{
					ChatID: fmt.Sprintf("%d", chatKick.ChatID),
				},
			)

			// skip if we already have don't have a subscription for this chat
			lock.Lock()
			defer lock.Unlock()
			if _, ok := subs[chatKick.ChatID]; ok {
				return
			}

			// unsubscribe from the chat
			err = subs[chatKick.ChatID].Unsubscribe()
			if err != nil {
				s.logger.Errorf("failed to unsubscribe from chat: %v", err)
			}

			// delete the chat from the maps
			delete(chatMap, chatKick.ChatID)
			delete(subs, chatKick.ChatID)
		}

		// create an async subscriber to broadcast init messages
		chatKickStream, err = s.jetstreamClient.Subscribe(
			fmt.Sprintf(streams.SubjectChatKickDynamic, callingUser.ID),
			kickChatHandler,
			nats.AckExplicit(),
			nats.DeliverNew(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to chat kick stream: %w", err)
		}

		// create function to handle stream messages for chat update events
		eventStreamHandler := func(m *nats.Msg) {
			// we always ack the message
			_ = m.Ack()

			// parse the message
			var chatUpdate models2.ChatUpdatedEventMsg
			decoder := gob.NewDecoder(bytes.NewBuffer(m.Data))
			err := decoder.Decode(&chatUpdate)
			if err != nil {
				s.logger.Errorf("failed to decode chat update message: %v", err)
				return
			}

			// send the message to the client to handle the update event
			outputChan <- ws.PrepMessage[any](
				"",
				ws.MessageTypeChatUpdatedEvent,
				chatUpdate,
			)
		}

		// create an async subscriber to broadcast init messages
		eventStream, err = s.jetstreamClient.Subscribe(
			fmt.Sprintf(streams.SubjectChatUpdatedDynamic, callingUser.ID),
			eventStreamHandler,
			nats.AckExplicit(),
			nats.DeliverNew(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to chat event stream: %w", err)
		}
	}

	return &WebSocketPluginChat{
		ctx:            ctx,
		s:              s,
		socket:         socket,
		mu:             lock,
		chats:          chatMap,
		subs:           subs,
		activeChat:     activeChatStream,
		kickChat:       chatKickStream,
		eventStream:    eventStream,
		outputChan:     outputChan,
		chatMsgHandler: chatMessageHandler,
	}, nil
}

func (p *WebSocketPluginChat) Name() string {
	return "chat"
}

func (p *WebSocketPluginChat) HandleMessage(msg *ws.Message[any]) {
	p.socket.logger.Debugf("chat plugin received message: %s - %s", msg.SequenceID, msg.Type)

	// skip any message that is not within the purview of the chat plugin
	if msg.Type != ws.MessageTypeNewChat &&
		msg.Type != ws.MessageTypeUpdateChat &&
		msg.Type != ws.MessageTypeNewIncomingChatMessage &&
		msg.Type != ws.MessageTypeNewOutgoingChatMessage &&
		msg.Type != ws.MessageTypeGetChats &&
		msg.Type != ws.MessageTypeGetChatMessages &&
		msg.Type != ws.MessageTypeChatSubscribe &&
		msg.Type != ws.MessageTypeChatUnsubscribe &&
		msg.Type != ws.MessageTypeDeleteChat &&
		msg.Type != ws.MessageTypeUpdateReadMessage &&
		msg.Type != ws.MessageTypeUpdateChatMute {
		return
	}

	// load user from socket
	user := p.socket.user.Load()

	p.socket.logger.Debugf("chat plugin handling message: %s - %s", msg.SequenceID, msg.Type)

	// handle an outgoing chat message
	if user != nil && msg.Type == ws.MessageTypeNewOutgoingChatMessage {
		p.handleOutgoingChatMessage(msg)
		return
	}

	// handle a new chat message
	if user != nil && msg.Type == ws.MessageTypeNewChat {
		p.handleNewChatMessage(msg)
		return
	}

	// handle an update chat message
	if user != nil && msg.Type == ws.MessageTypeUpdateChat {
		p.handleUpdateChatMessage(msg)
		return
	}

	// handle a get chats message
	if user != nil && msg.Type == ws.MessageTypeGetChats {
		p.handleGetChatsMessage(msg)
		return
	}

	// handle chat mutes
	if user != nil && msg.Type == ws.MessageTypeUpdateChatMute {
		p.handleUpdateChatMuteMessage(msg)
		return
	}

	// handle delete chat
	if user != nil && msg.Type == ws.MessageTypeDeleteChat {
		p.handleDeleteChatMessage(msg)
		return
	}

	// handle a get chat messages message
	if msg.Type == ws.MessageTypeGetChatMessages {
		p.handleGetChatMessagesMessage(msg)
		return
	}

	// handle a chat subscribe message
	if msg.Type == ws.MessageTypeChatSubscribe {
		p.handleChatSubscribeMessage(msg)
		return
	}

	// handle a chat unsubscribe message
	if msg.Type == ws.MessageTypeChatUnsubscribe {
		p.handleChatUnsubscribeMessage(msg)
		return
	}
}

func (p *WebSocketPluginChat) OutgoingMessages() <-chan ws.Message[any] {
	return p.outputChan
}

func (p *WebSocketPluginChat) Close() {
	// close all subscriptions
	for _, sub := range p.subs {
		_ = sub.Unsubscribe()
	}
	_ = p.activeChat.Unsubscribe()
	_ = p.kickChat.Unsubscribe()
	_ = p.eventStream.Unsubscribe()
}

func (p *WebSocketPluginChat) handleOutgoingChatMessage(msg *ws.Message[any]) {
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
	var sendMessage core.SendMessageParams
	err = json.Unmarshal(innerBuf, &sendMessage)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, sendMessage) {
		return
	}

	// execute core function logic
	message, err := core.SendMessageInternal(p.ctx, p.s.tiDB, p.s.sf, p.s.logger, p.socket.user.Load(), sendMessage, p.s.mailGunKey, p.s.mailGunDomain)
	if err != nil {
		p.socket.logger.Errorf("failed to execute send message internal: %v", err)
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

	// update the message with the author tier
	message.AuthorRenown = p.socket.user.Load().Tier

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeNewOutgoingChatMessage,
		message.ToFrontend(),
	)

	// marshall message using gob
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(models2.NewMessageMsg{
		Message: *message,
	})
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// forward the message to the chat stream
	_, err = p.s.jetstreamClient.Publish(
		fmt.Sprintf(streams.SubjectChatMessagesDynamic, message.ChatID),
		buf.Bytes(),
	)
	if err != nil {
		p.socket.logger.Errorf("failed to publish message to chat stream: %v", err)
	}
}

func (p *WebSocketPluginChat) handleNewChatMessage(msg *ws.Message[any]) {
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
	var createChatParams core.CreateChatParams
	err = json.Unmarshal(innerBuf, &createChatParams)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, createChatParams) {
		p.socket.logger.Debugf("failed to validate message: %s - %s", msg.SequenceID, msg.Type)
		return
	}

	// execute core function logic
	chat, event, err := core.CreateChat(p.ctx, p.s.tiDB, p.s.sf, p.socket.user.Load(), createChatParams)
	if err != nil {
		p.socket.logger.Errorf("failed to execute create chat internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeNewChat,
		chat.ToFrontend(),
	)

	// subscribe to the new chat stream

	// create an async subscriber to broadcast init messages
	sub, err := p.s.jetstreamClient.Subscribe(
		fmt.Sprintf(streams.SubjectChatMessagesDynamic, chat.ID),
		p.chatMsgHandler,
		nats.AckExplicit(),
		nats.DeliverNew(),
	)
	if err != nil {
		p.socket.logger.Errorf("failed to subscribe to new chat stream: %v", err)
		return
	}

	// save the chat and subscription to the map
	p.mu.Lock()
	p.subs[chat.ID] = sub
	p.chats[chat.ID] = chat
	p.mu.Unlock()

	// marshall message using gob
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(models2.NewChatMsg{
		Chat: *chat,
	})
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	chatBytes := buf.Bytes()

	// forward the message to all the active users so they can subscribe to the chat
	for _, user := range chat.Users {
		// skip if the user is the calling user since me just manually subscribed
		if user == p.socket.user.Load().ID {
			continue
		}

		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatNewChatDynamic, user),
			chatBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat stream: %v", err)
		}
	}

	// if the event is nil exit
	if event == nil {
		return
	}

	// iterate over all the users including and send the event

	// marshall message using gob
	var eventBuf bytes.Buffer
	encoder = gob.NewEncoder(&eventBuf)
	err = encoder.Encode(event)
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	eventBytes := eventBuf.Bytes()

	// forward the message to all the active users so they can subscribe to the chat
	for _, user := range chat.Users {
		// skip if the user is the calling user since me just manually subscribed
		if user == p.socket.user.Load().ID {
			continue
		}

		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatUpdatedDynamic, user),
			eventBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat stream: %v", err)
		}
	}
}

func (p *WebSocketPluginChat) handleUpdateChatMessage(msg *ws.Message[any]) {
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
	var editChatParams core.EditChatParams
	err = json.Unmarshal(innerBuf, &editChatParams)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, editChatParams) {
		return
	}

	// execute core function logic
	chat, event, err := core.EditChat(p.ctx, p.s.tiDB, p.socket.user.Load(), editChatParams)
	if err != nil {
		p.socket.logger.Errorf("failed to execute edit chat internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeUpdateChat,
		chat.ToFrontend(),
	)

	// marshall message using gob
	var kickBuf bytes.Buffer
	encoder := gob.NewEncoder(&kickBuf)
	err = encoder.Encode(models2.ChatKickMsg{
		ChatID: chat.ID,
	})
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	kickBytes := kickBuf.Bytes()

	// forward the message to all the removed users so they can unsubscribe from the chat
	for _, user := range editChatParams.RemoveUsers {
		// parse the user id to an int for formatting into the stream subject
		userId, _ := strconv.ParseInt(user, 10, 64)

		// validate the user is not currently in the chat
		found := false
		for _, chatUser := range chat.Users {
			if chatUser == userId {
				found = true
				break
			}
		}

		// skip if the user is still in the chat - this means it was an invalid removal
		if found {
			continue
		}

		p.socket.logger.Debugf("sending kick message to user on user remove %d: %d", userId, chat.ID)
		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatKickDynamic, userId),
			kickBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat stream: %v", err)
		}
	}

	// marshall message using gob
	var chatBuf bytes.Buffer
	encoder = gob.NewEncoder(&chatBuf)
	err = encoder.Encode(models2.NewChatMsg{
		Chat: *chat,
	})
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	chatBytes := chatBuf.Bytes()

	// forward the message to all the active users so they can subscribe to the chat
	for _, user := range chat.Users {
		// skip if the user is the calling user since we already have a subscription
		if user == p.socket.user.Load().ID {
			continue
		}

		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatNewChatDynamic, user),
			chatBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat stream: %v", err)
		}
	}

	// iterate over all the users including the removed users and send them the event

	// marshall message using gob
	var eventBuf bytes.Buffer
	encoder = gob.NewEncoder(&eventBuf)
	err = encoder.Encode(event)
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	eventBytes := eventBuf.Bytes()

	for _, user := range chat.Users {
		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatUpdatedDynamic, user),
			eventBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat updated stream: %v", err)
		}
	}

	for _, user := range event.RemovedUsers {
		// parse the user id to an int for formatting into the stream subject
		userId, _ := strconv.ParseInt(user, 10, 64)

		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatUpdatedDynamic, userId),
			eventBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat updated stream: %v", err)
		}
	}
}

func (p *WebSocketPluginChat) handleGetChatsMessage(msg *ws.Message[any]) {
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
	var getChatsParams core.GetChatsParams
	err = json.Unmarshal(innerBuf, &getChatsParams)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, getChatsParams) {
		return
	}

	// execute core function logic
	chats, err := core.GetChats(p.ctx, p.s.tiDB, p.socket.user.Load(), getChatsParams)
	if err != nil {
		p.socket.logger.Errorf("failed to execute get chats internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeGetChats,
		GetChatsResponse{
			Chats: chats,
		},
	)
}

func (p *WebSocketPluginChat) handleGetChatMessagesMessage(msg *ws.Message[any]) {
	p.socket.logger.Debugf("handling get chat messages message: %s", msg.SequenceID)

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
	var getMsgsParams core.GetMessagesParams
	err = json.Unmarshal(innerBuf, &getMsgsParams)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, getMsgsParams) {
		return
	}

	// execute core function logic
	messages, err := core.GetMessages(p.ctx, p.s.tiDB, p.socket.user.Load(), getMsgsParams)
	if err != nil {
		p.socket.logger.Errorf("failed to execute get chat messages internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeGetChatMessages,
		GetChatMessagesResponse{
			Messages: messages,
		},
	)
}

func (p *WebSocketPluginChat) handleChatSubscribeMessage(msg *ws.Message[any]) {
	p.socket.logger.Debug("handling chat subscribe message: %s", msg.SequenceID)

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
	var subParams ChatSubscribeParams
	err = json.Unmarshal(innerBuf, &subParams)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, subParams) {
		return
	}

	// reject if this is not a challenge type - we will expand this api later
	if subParams.ChatType != models.ChatTypeChallenge {
		p.socket.logger.Errorf("failed to subscribe to chat: %v", err)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeBadRequest,
				Error: "invalid chat type",
			},
		)
		return
	}

	// return if we already have this chat
	chatId, _ := strconv.ParseInt(subParams.ChatID, 10, 64)
	p.mu.Lock()
	if _, ok := p.subs[chatId]; ok {
		// write the message to the client
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeChatSubscribe,
			ChatSubscribeResponse{
				Chat: p.chats[chatId].ToFrontend(),
			},
		)
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	// execute core function logic
	chat, err := core.ValidateChallengeChat(p.ctx, p.s.tiDB, subParams.ChatID)
	if err != nil {
		p.socket.logger.Errorf("failed to execute validate challenge chat internal: %v", err)
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

	// reject if the chat is not valid
	if chat == nil {
		p.socket.logger.Errorf("failed to subscribe to chat: %v", err)
		// handle internal server error via websocket
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeBadRequest,
				Error: "invalid chat",
			},
		)
		return
	}

	// subscribe to the chat and add it to the subscription map
	sub, err := p.s.jetstreamClient.Subscribe(
		fmt.Sprintf(streams.SubjectChatMessagesDynamic, chatId),
		p.chatMsgHandler,
		nats.AckExplicit(),
		nats.DeliverNew(),
	)
	if err != nil {
		p.socket.logger.Errorf("failed to subscribe to chat stream: %v", err)
		return
	}

	// save the chat and subscription to the map
	p.mu.Lock()
	p.subs[chatId] = sub
	p.chats[chatId] = chat
	p.mu.Unlock()

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeChatSubscribe,
		ChatSubscribeResponse{
			Chat: chat.ToFrontend(),
		},
	)
}

func (p *WebSocketPluginChat) handleChatUnsubscribeMessage(msg *ws.Message[any]) {
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
	var unParams ChatUnsubscribeParams
	err = json.Unmarshal(innerBuf, &unParams)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, unParams) {
		return
	}

	// skip if we don't have this chat
	chatId, _ := strconv.ParseInt(unParams.ChatID, 10, 64)
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.subs[chatId]; !ok {
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeBadRequest,
				Error: "chat is not subscribed",
			},
		)
		return
	}

	// skip if the chat is not a challenge type - we will expand this api later
	if p.chats[chatId].Type != models.ChatTypeChallenge {
		p.outputChan <- ws.PrepMessage[any](
			msg.SequenceID,
			ws.MessageTypeGenericError,
			ws.GenericErrorPayload{
				Code:  ws.ResponseCodeBadRequest,
				Error: "invalid chat type",
			},
		)
		return
	}

	// unsubscribe from the chat
	err = p.subs[chatId].Unsubscribe()
	if err != nil {
		p.socket.logger.Errorf("failed to unsubscribe from chat: %v", err)
	}

	// delete the chat from the maps
	delete(p.chats, chatId)
	delete(p.subs, chatId)

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeChatUnsubscribe,
		ChatUnsubscribeResponse{
			ChatID: unParams.ChatID,
		},
	)
}

func (p *WebSocketPluginChat) handleUpdateReadMessage(msg *ws.Message[any]) {
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
	var params core.UpdateReadMessageParams
	err = json.Unmarshal(innerBuf, &params)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, params) {
		return
	}

	// execute core function logic
	err = core.UpdateReadMessage(p.ctx, p.s.tiDB, p.socket.user.Load(), params)
	if err != nil {
		p.socket.logger.Errorf("failed to execute update read message internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeUpdateReadMessage,
		UpdateReadMessageResponse{
			MessageID: params.MessageId,
		},
	)
}

func (p *WebSocketPluginChat) handleUpdateChatMuteMessage(msg *ws.Message[any]) {
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
	var params core.UpdateChatMuteParams
	err = json.Unmarshal(innerBuf, &params)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, params) {
		return
	}

	// execute core function logic
	err = core.UpdateChatMute(p.ctx, p.s.tiDB, p.socket.user.Load(), params)
	if err != nil {
		p.socket.logger.Errorf("failed to execute update chat mute internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeUpdateChatMute,
		UpdateChatMuteResponse{
			ChatID: params.ChatId,
		},
	)
}

func (p *WebSocketPluginChat) handleDeleteChatMessage(msg *ws.Message[any]) {
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
	var params core.DeleteChatParams
	err = json.Unmarshal(innerBuf, &params)
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
	if !p.s.validateWebSocketPayload(p.ctx, p.socket.ws, msg, params) {
		return
	}

	// execute core function logic
	chat, event, err := core.DeleteChat(p.ctx, p.s.tiDB, p.socket.user.Load(), params)
	if err != nil {
		p.socket.logger.Errorf("failed to execute delete chat internal: %v", err)
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

	// write the message to the client
	p.outputChan <- ws.PrepMessage[any](
		msg.SequenceID,
		ws.MessageTypeDeleteChat,
		DeleteChatResponse{
			ChatID: params.ChatId,
		},
	)

	// send kick message to all of the users in the chat
	// marshall message using gob
	var kickBuf bytes.Buffer
	encoder := gob.NewEncoder(&kickBuf)
	err = encoder.Encode(models2.ChatKickMsg{
		ChatID: chat.ID,
	})
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	kickBytes := kickBuf.Bytes()

	// forward the message to all the removed users so they can unsubscribe from the chat
	for _, user := range chat.Users {
		// skip the calling user since the frontend will handle the kick for them
		if user == p.socket.user.Load().ID {
			continue
		}

		p.socket.logger.Debugf("sending kick message to user on delete %d: %d", user, chat.ID)
		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatKickDynamic, user),
			kickBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat stream: %v", err)
		}
	}

	// iterate over all the users and send them the event

	// marshall message using gob
	var eventBuf bytes.Buffer
	encoder = gob.NewEncoder(&eventBuf)
	err = encoder.Encode(event)
	if err != nil {
		p.socket.logger.Errorf("failed to encode message: %v", err)
		return
	}

	// save the buffer bytes to a variable
	eventBytes := eventBuf.Bytes()

	for _, user := range chat.Users {
		_, err = p.s.jetstreamClient.Publish(
			fmt.Sprintf(streams.SubjectChatUpdatedDynamic, user),
			eventBytes,
		)
		if err != nil {
			p.socket.logger.Errorf("failed to publish message to chat updated stream: %v", err)
		}
	}
}
