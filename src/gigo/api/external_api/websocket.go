package external_api

import (
	"context"
	"errors"
	"fmt"
	"gigo-core/gigo/api/external_api/ws"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/go-playground/validator"
	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	// intervals for how often a user should be polled based on how
	// recently they have interacted with the web socket

	userPollIntervalHot  = 30 * time.Second
	userPollIntervalWarm = time.Minute * 3
	userPollIntervalCold = 15 * time.Minute

	// thresholds for switching polling intervals based on how
	// recently a user has interacted with the web socket

	userPollThresholdHot  = time.Minute
	userPollThresholdWarm = 15 * time.Minute
	userPollThresholdCold = 30 * time.Minute

	maxMessageSize = 1 << 18
)

// masterWebSocket
//
//	masterWebSocket is used to pass the properties of a master web socket
//	connection to the multiple goroutines that will be spawned to handle
//	the connection.
type masterWebSocket struct {
	// web socket connection
	ws *websocket.Conn

	// user that is connected to the socket
	user *atomic.Pointer[models.User]

	// time of the last websocket interaction
	lastInteraction *atomic.Pointer[time.Time]

	// channel to manually trigger a new user poll
	poll chan struct{}

	// channel to respond to a manual poll
	pollResponse chan struct{}

	// worker pool to manage the concurrent resources of this connection
	pool *pool.Pool

	// context for the connection
	ctx context.Context

	// cancel function for the connection
	cancel context.CancelCauseFunc

	// logger for the connection
	logger logging.Logger

	// map of handlers for each message type
	handlers map[ws.MessageType]WebSocketHandlerFunc

	// slice of plugins for the connection
	plugins []WebSocketPlugin
}

// WebSocketHandlerFunc
//
//	WebSocketHandlerFunc is the function signature for handlers meant to process
//	a specific ws.Message type when sent from the client to the server.
type WebSocketHandlerFunc func(socket *masterWebSocket, msg *ws.Message[any])

// WebSocketPlugin
//
//		WebSocketPlugin is the function signature for plugins meant to participate
//	 in more advanced processing of a specific ws.Message type when sent from the
//	 client to the server or to forward messages to the client from the plugin's
//	 own goroutine.
type WebSocketPlugin interface {
	// HandleMessage
	//
	//	HandleMessage is the function signature for plugins meant to participate
	//  in more advanced processing of a specific ws.Message type when sent from the
	//  client to the server.
	HandleMessage(msg *ws.Message[any])

	// OutgoingMessages
	//
	//	OutgoingMessages is the function signature for plugins meant to forward
	//  messages to the client from the plugin's own goroutine.
	OutgoingMessages() <-chan ws.Message[any]

	// Name
	//
	//	Name is the function signature for plugins meant to return the name of
	//  the plugin.
	Name() string

	// Close
	//
	//	Close is the function signature for plugins meant to perform cleanup
	//  when the master web socket connection is closed.
	Close()
}

// MasterWebSocket
//
//	The master web socket to manage all messages for the API.
func (s *HTTPServer) MasterWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "master-websocket-http")
	defer parentSpan.End()

	// retrieve calling user from context
	var callingUser *models.User
	callingUserI := r.Context().Value(CtxKeyUser)
	if callingUserI != nil {
		callingUser = callingUserI.(*models.User)
	}

	callingId := "-1"
	username := "anon"
	if callingUser != nil {
		callingId = strconv.FormatInt(callingUser.ID, 10)
		username = callingUser.UserName
	}

	// validate the origin
	if !s.validateOrigin(w, r) {
		return
	}

	// accept websocket connection with client
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.handleError(w, "failed to accept websocket connection", r.URL.Path, "ChatWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// update the max size of a single payload to 256KiB
	conn.SetReadLimit(maxMessageSize)

	// WARNING: we can no longer use the built in response handlers like
	// handleError and handleJsonResponse we have just hijacked the connection
	// and upgraded to a websocket

	// create context for the connection
	// NOTE: we create an entirely new context since the request context
	// will be terminated once this function returns but the socket will
	// live much longer than that.
	ctx, cancel := context.WithCancelCause(context.Background())

	s.logger.Infof("new web socket connection: %s", callingId)

	// wrap the user in an atomic pointer so we can
	// concurrently update it as we poll the database
	// for any user changes
	var safeUser atomic.Pointer[models.User]
	safeUser.Store(callingUser)

	// wrap the last message time in an atomic pointer so we can
	// concurrently update it as we receive messages
	var safeTime atomic.Pointer[time.Time]
	t := time.Now()
	safeTime.Store(&t)

	// assemble the master web socket
	socket := &masterWebSocket{
		ws:              conn,
		user:            &safeUser,
		lastInteraction: &safeTime,
		poll:            make(chan struct{}),
		pollResponse:    make(chan struct{}),
		// we allocate 4 workers for the socket plus one for the user poller
		pool:     pool.New().WithMaxGoroutines(5),
		ctx:      ctx,
		cancel:   cancel,
		logger:   s.logger,
		plugins:  []WebSocketPlugin{},
		handlers: map[ws.MessageType]WebSocketHandlerFunc{},
	}

	// launch the user poller via the socket's worker pool only if the user is logged in
	if callingUser != nil {
		socket.pool.Go(func() {
			s.masterWebSocketUserPoller(socket)
		})
	}

	// launch the master web socket loop via the server's waitgroup
	s.wg.Go(func() {
		s.masterWebSocketLoop(socket)
	})
}

// masterWebSocketLoop
//
//	The master web socket loop to manage all messages for the API.
func (s *HTTPServer) masterWebSocketLoop(socket *masterWebSocket) {
	// defer the cleanup function to ensure our resources are cleaned up
	defer func() {
		// cancel the context to ensure all goroutines are terminated
		socket.cancel(fmt.Errorf("masterWebSocketLoop: exiting on closure"))

		// close the web socket connection
		// NOTE: if we are closing here then something has gone wrong
		socket.ws.Close(websocket.StatusInternalError, "internal server error")

		// close all the plugins
		for _, plugin := range socket.plugins {
			plugin.Close()
		}

		// wait for the cleanup of all the goroutines associatd with this websocket
		socket.pool.Wait()
	}()

	// initialize the plugins
	chatPlugin, err := NewPluginChat(socket.ctx, s, socket)
	if err != nil {
		socket.logger.Errorf("failed to initialize chat plugin: %v", err)
		return
	}
	socket.plugins = append(socket.plugins, chatPlugin)

	workspacePlugin, err := NewPluginWorkspace(socket.ctx, s, socket)
	if err != nil {
		socket.logger.Errorf("failed to initialize workspace plugin: %v", err)
		return
	}
	socket.plugins = append(socket.plugins, workspacePlugin)

	// create a ticker to send a ping to the client every 30 seconds
	// to ensure the connection is still alive
	ticker := time.NewTicker(30 * time.Second)

	// create a channel to receive messages from the client
	// NOTE: we use a buffered channel here to prevent the goroutine
	// from blocking if the client is sending messages faster than
	// we can process them.
	messages := make(chan *ws.Message[any], 100)

	// launch a goroutine to read messages from the client
	s.wg.Go(func() {
		s.masterWebSocketRead(socket, messages)
	})

	// loop until the connection is closed handling messages and executing pings
	for {
		select {
		case <-socket.ctx.Done():
			if ctxErr := context.Cause(socket.ctx); ctxErr != nil {
				socket.logger.Debugf("masterWebSocketLoop closed: %v", ctxErr)
			}
			return
		case <-ticker.C:
			// send ping on ticker interval to keep the connection alive
			err := socket.ws.Ping(socket.ctx)
			if err != nil {
				socket.logger.Errorf("failed to send ping to client: %v", err)
				return
			}
		case msg := <-chatPlugin.OutgoingMessages():
			// forward message to client
			err := wsjson.Write(socket.ctx, socket.ws, msg)
			if err != nil {
				socket.logger.Errorf("failed to forward message to client: %v", err)
				continue
			}
		case msg := <-workspacePlugin.OutgoingMessages():
			// forward message to client
			err := wsjson.Write(socket.ctx, socket.ws, msg)
			if err != nil {
				socket.logger.Errorf("failed to forward message to client: %v", err)
				continue
			}
		case message := <-messages:
			// forward message to plugins
			for _, plugin := range socket.plugins {
				plugin := plugin
				socket.pool.Go(func() {
					s.pluginWrapper(plugin.HandleMessage, socket, message)
				})
			}

			// attempt to load the handler for the message type
			handler, ok := socket.handlers[message.Type]
			if !ok {
				continue
			}

			// submit the handler to the worker pool
			socket.pool.Go(func() {
				// execute the handler
				s.handlerWrapper(handler, socket, message)
			})
		}
	}

}

// masterWebSocketRead
//
//	The master web socket read loop to read messages from the client.
func (s *HTTPServer) masterWebSocketRead(socket *masterWebSocket, messages chan *ws.Message[any]) {
	// defer the cleanup function to ensure our resources are cleaned up
	defer func() {
		if r := recover(); r != nil {
			socket.cancel(fmt.Errorf("masterWebSocketRead: panic: %v", r))
			socket.logger.Errorf("masterWebSocketRead: panic: %v", r)
		} else {
			socket.cancel(fmt.Errorf("masterWebSocketRead: exiting on closure"))
		}
		socket.ws.Close(websocket.StatusNormalClosure, "")
	}()

	// loop until the connection is closed
	for {
		// read the next message from the client
		var message ws.Message[any]
		err := wsjson.Read(socket.ctx, socket.ws, &message)
		if err != nil {
			// check if the connection was closed
			if websocket.CloseStatus(err) != -1 || errors.Is(err, context.Canceled) {
				socket.logger.Debug("websocket closed")
			}
			return
		}

		// validate the message payload
		if !s.validateWebSocketPayload(socket.ctx, socket.ws, &message, nil) {
			continue
		}

		// update the last interaction time
		t := time.Now()
		socket.lastInteraction.Store(&t)

		// send the message to the write loop
		messages <- &message
	}
}

// validateWebSocketPayload
//
//	Loads a json request from the websocket payload and validates its schema.
func (s *HTTPServer) validateWebSocketPayload(ctx context.Context, conn *websocket.Conn, msg *ws.Message[any], inner interface{}) bool {
	// validate the schema
	var err error
	if inner != nil {
		err = s.validator.Struct(inner)
	} else {
		err = s.validator.Struct(msg)
	}

	// handle known validation errors
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		s.logger.Debugf("websocket validation error %q: %v", msg.SequenceID, validationErrors)

		// create a map and fill with the field names and validation errors
		failedValidations := make(map[string]string)
		for _, validationError := range validationErrors {
			failedValidations[validationError.Field()] = validationError.Tag()
		}

		// return the error payload to the client
		err = wsjson.Write(ctx, conn, ws.Message[ws.ValidationErrorPayload]{
			SequenceID: msg.SequenceID,
			Type:       ws.MessageTypeValidationError,
			Payload: ws.ValidationErrorPayload{
				GenericErrorPayload: ws.GenericErrorPayload{
					Code:  ws.ResponseCodeBadRequest,
					Error: "validation error",
				},
				ValidationErrors: failedValidations,
			},
		})
		if err != nil {
			s.logger.Errorf("failed to send validation error payload %q: %v", msg.SequenceID, err)
		}

		return false
	}

	// handle unexpected validation errors
	if err != nil {
		s.logger.Errorf("failed to validate payload %q: %v", msg.SequenceID, err)
		// return the error payload to the client
		err = wsjson.Write(ctx, conn, ws.Message[ws.GenericErrorPayload]{
			SequenceID: msg.SequenceID,
			Type:       ws.MessageTypeGenericError,
			Payload: ws.GenericErrorPayload{
				Code:  ws.ResponseCodeServerError,
				Error: "internal server error occurred",
			},
		})
		if err != nil {
			s.logger.Errorf("failed to send error payload %q: %v", msg.SequenceID, err)
		}
		return false
	}

	return true
}

// pluginWrapper
//
//	Wraps all plugin handler functions to provide global middleware such as panic recovery.
func (s *HTTPServer) pluginWrapper(handler func(msg *ws.Message[any]), socket *masterWebSocket, msg *ws.Message[any]) {
	// catch any panics and return them as errors
	defer func() {
		if r := recover(); r != nil {
			// format the panic and the stack trace into an error
			panicErr := fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			socket.logger.Errorf(
				"unexpected panic in websocket handler (%s): %s",
				msg.Type.String(),
				panicErr,
			)
			// return the error payload to the client
			err := wsjson.Write(socket.ctx, socket.ws, ws.PrepMessage(
				msg.SequenceID,
				ws.MessageTypeGenericError,
				ws.GenericErrorPayload{
					Error: "internal server error occurred",
					Code:  ws.ResponseCodeServerError,
				},
			))
			if err != nil {
				socket.logger.Errorf(
					"failed to send error payload: %v",
					err,
				)
			}
		}
	}()

	// handle the message
	handler(msg)
}

// handlerWrapper
//
//	Wraps all handler functions to provide global middleware such as panic recovery.
func (s *HTTPServer) handlerWrapper(handler WebSocketHandlerFunc, socket *masterWebSocket, msg *ws.Message[any]) {
	// catch any panics and return them as errors
	defer func() {
		if r := recover(); r != nil {
			// format the panic and the stack trace into an error
			panicErr := fmt.Errorf("panic: %v\n%s", r, debug.Stack())
			socket.logger.Errorf(
				"unexpected panic in websocket handler (%s): %s",
				msg.Type.String(),
				panicErr,
			)
			// return the error payload to the client
			err := wsjson.Write(socket.ctx, socket.ws, ws.PrepMessage(
				msg.SequenceID,
				ws.MessageTypeGenericError,
				ws.GenericErrorPayload{
					Error: "internal server error occurred",
					Code:  ws.ResponseCodeServerError,
				},
			))
			if err != nil {
				socket.logger.Errorf(
					"failed to send error payload: %v",
					err,
				)
			}
		}
	}()

	// handle the message
	handler(socket, msg)
}

// masterWebSocketUserPoller
//
//		The master web socket user poller to update the user in the atomic pointer
//		so the socket handlers are always working with the latest user data. The poller
//	    provides the following functionality:
//			- Updates the user in the atomic pointer with the latest user data from the database.
//			- Closes the socket if the user is deleted.
//			- Uses a hot, warm and cold poll interval to reduce the number of database queries.
func (s *HTTPServer) masterWebSocketUserPoller(socket *masterWebSocket) {
	// create a ticker to poll the database for user changes
	// defaulting to our hot poll interval
	ticker := time.NewTicker(userPollIntervalHot)
	defer ticker.Stop()

	// record the last poll so we know how long it's been since we've
	// updated the user
	lastPoll := time.Now()

	// create a variable to track the current poll interval
	interval := userPollIntervalHot

	// poll the database for user changes
	for {
		// default to a interval poll
		manualPoll := false

		// wait for the next tick
		select {
		case <-ticker.C:
		case <-socket.poll:
			// mark the poll as manual
			manualPoll = true
		case <-socket.ctx.Done():
			return
		}

		// update the interval if we've passed a polling threshold
		timeSinceLastInteraction := time.Since(*socket.lastInteraction.Load())
		if manualPoll || (timeSinceLastInteraction < userPollThresholdHot && interval != userPollIntervalHot) {
			interval = userPollIntervalHot
			ticker.Reset(interval)
		} else if timeSinceLastInteraction < userPollThresholdWarm && interval != userPollIntervalWarm {
			interval = userPollIntervalWarm
			ticker.Reset(interval)
		} else if timeSinceLastInteraction >= userPollThresholdCold && interval != userPollIntervalCold {
			interval = userPollIntervalCold
			ticker.Reset(interval)
		}

		// if we've polled within the interval then skip the poll
		if time.Since(lastPoll) < interval {
			if manualPoll {
				// inform the master loop that we've responded to the poll
				socket.pollResponse <- struct{}{}
			}
			continue
		}

		// retrieve the user from the atomic pointer
		currentUser := socket.user.Load()

		ctx, span := otel.Tracer("gigo-core").Start(socket.ctx, "master-web-socket-user-poller-refresh")
		callerName := "masterWebSocketUserPoller"
		// query for user in database
		res, err := s.tiDB.QueryContext(ctx, &span, &callerName, "select * from users where _id = ? limit 1", currentUser.ID)
		if err != nil {
			socket.logger.Errorf("failed to query for user in masterWebSocketUserPoller: %v", err)
			span.End()

			// inform the master loop that we've responded to the poll
			if manualPoll {
				socket.pollResponse <- struct{}{}
			}
			continue
		}

		// attempt to load the user into the first position of the cursor
		ok := res.Next()
		if !ok {
			// close response explicitly
			_ = res.Close()

			// log the error and close the span
			socket.logger.Errorf("failed to load user in masterWebSocketUserPoller: no user found with id %d", currentUser.ID)
			span.End()

			// inform the master loop that we've responded to the poll
			if manualPoll {
				socket.pollResponse <- struct{}{}
			}
			continue
		}

		// attempt to decode user into object
		newUser, err := models.UserFromSQLNative(s.tiDB, res)
		if err != nil {
			// close response explicitly
			_ = res.Close()

			// log the error and close the span
			socket.logger.Errorf("failed to load user in masterWebSocketUserPoller: %v", err)
			span.End()

			// inform the master loop that we've responded to the poll
			if manualPoll {
				socket.pollResponse <- struct{}{}
			}
			continue
		}

		// close response explicitly
		_ = res.Close()

		// update the user in the atomic pointer
		socket.user.Store(newUser)

		// update the last poll time
		lastPoll = time.Now()

		// inform the master loop that we've responded to the poll
		if manualPoll {
			socket.pollResponse <- struct{}{}
		}
	}
}
