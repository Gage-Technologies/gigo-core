package external_api

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/nats-io/nats.go"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// BroadcastWebSocket Will open a websocket connection that handles the broadcasting of system-wide events.
func (s *HTTPServer) BroadcastWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "broadcast-websocket-http")
	defer parentSpan.End()
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "BroadcastWebSocket: calling user missing from context", r.URL.Path, "BroadcastWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// validate the origin
	if !s.validateOrigin(w, r) {
		return
	}

	// accept websocket connection with client
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.handleError(w, fmt.Sprintf("BroadcastWebSocket: failed to accept websocket connection"), r.URL.Path, "BroadcastWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	s.logger.Debugf("BroadcastWebSocket connection (%d): accepted", callingId)

	// launch reader to handle reading the pong messages
	parentCtx, cancel := context.WithCancel(context.Background())
	ctx = ws.CloseRead(parentCtx)

	// create a synchronous subscriber to broadcast init messages
	broadcastInitSub, err := s.jetstreamClient.SubscribeSync(
		fmt.Sprintf(streams.SubjectBroadcastMessageDynamic, callingUser.(*models.User).ID),
		nats.AckExplicit(),
	)
	if err != nil {
		cancel()
		s.logger.Errorf("BroadcastWebSocket (%d): failed to subscribe to broadcast %q Init: %v", callingUser.(*models.User).ID, streams.SubjectBroadcastMessageDynamic, err)
		return
	}

	// create a synchronous subscriber to broadcast init messages
	notificationSub, err := s.jetstreamClient.SubscribeSync(
		fmt.Sprintf(streams.SubjectBroadcastNotificationDynamic, callingUser.(*models.User).ID),
		nats.AckExplicit(),
	)
	if err != nil {
		cancel()
		s.logger.Errorf("BroadcastWebSocket (%d): failed to subscribe to broadcast %q Notification: %v", callingUser.(*models.User).ID, streams.SubjectBroadcastNotificationDynamic, err)
		return
	}

	// create channels to pipe broadcast init messages and broadcast notifications to socket handler functions
	broadcastMessageChan := make(chan string)
	notificationChan := make(chan string)

	// launch a goroutine to read the next broadcast init message
	s.wg.Go(func() {
		for {
			select {
			// check if context is done and exit
			case <-ctx.Done():
				return
			// attempt to read another message
			default:
				// get next message from jetstream sub
				msg, err := broadcastInitSub.NextMsg(time.Millisecond * 100)
				if err != nil {
					// loop for timeout so we can check if our context has expired
					if errors.Is(err, nats.ErrTimeout) {
						continue
					}
					if !strings.Contains(err.Error(), "invalid subscription") {
						s.logger.Errorf("BroadcastWebSocket (%d): failed to read next message from jetstream : %v", callingUser.(*models.User).ID, err)
					}
					return
				}

				// if there is a new msg, decode it into initMessage
				if msg != nil {
					// create new NotificationFrontend model to decode msg into
					var initMsg models2.BroadcastMessage
					decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
					err := decoder.Decode(&initMsg)
					if err != nil {
						s.logger.Errorf("BroadcastWebSocket (%d): failed to decode broadcast init message: %v", callingUser.(*models.User).ID, err)
						return
					}

					// write decoded init message to broadcast init channel to
					// be written to the websocket by the handler routine
					broadcastMessageChan <- initMsg.InitMessage

					// ack message
					_ = msg.Ack()
				}
			}
		}
	})

	// launch a goroutine to read the next notification message
	s.wg.Go(func() {
		for {
			select {
			// check if context is done and exit
			case <-ctx.Done():
				return
			// attempt to read another message
			default:
				// get next message from jetstream sub
				msg, err := notificationSub.NextMsg(time.Millisecond * 100)
				if err != nil {
					// loop for timeout so we can check if our context has expired
					if errors.Is(err, nats.ErrTimeout) {
						continue
					}
					s.logger.Errorf("BroadcastWebSocket (%d): failed to read next notification message from jetstream : %v", callingUser.(*models.User).ID, err)
					return
				}

				// if there is a new msg, decode it into initMessage
				if msg != nil {
					// create new BroadcastNotification model to decode msg into
					var notification models2.BroadcastNotification
					decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
					err := decoder.Decode(&notification)
					if err != nil {
						err = s.jetstreamClient.PurgeStream(fmt.Sprintf(streams.SubjectBroadcastNotificationDynamic, callingUser.(*models.User).ID))
						if err != nil {
							s.logger.Errorf("BroadcastWebSocket (%d): failed to purge the notification stream: %v", callingId, err)
						}
						s.logger.Errorf("BroadcastWebSocket (%d): failed to decode broadcast notification : %v \n notification : %v", callingUser.(*models.User).ID, err, msg.Data)
						return
					}

					// write decoded init message to broadcast init channel to
					// be written to the websocket by the handler routine
					notificationChan <- notification.Notification

					// ack message
					_ = msg.Ack()
				}
			}
		}
	})

	// launch goroutine to handle the websocket from now on
	s.wg.Go(func() {
		// defer closure of websocket connection
		defer func() {
			cancel()
			_ = ws.Close(websocket.StatusGoingAway, "closing websocket")
		}()

		// create new ticker for pings
		pingTicker := time.NewTicker(time.Second)
		defer pingTicker.Stop()

		// loop until the socket closes
		for {
			select {
			case <-ctx.Done():
				s.logger.Debugf("BroadcastWebSocket (%d): context canceled %v", callingId, ctx.Err())
				return
			case init := <-broadcastMessageChan:
				err = wsjson.Write(ctx, ws, init)
				if err != nil {
					s.logger.Errorf("BroadcastWebSocket (%d): socket write failed for broadcast message init: %v", callingUser.(*models.User).ID, err)
					// exit
					return
				}
				s.logger.Debugf("BroadcastWebSocket (%d): init wrote", callingId)
			case notification := <-notificationChan:
				err = wsjson.Write(ctx, ws, notification)
				if err != nil {
					s.logger.Errorf("BroadcastWebSocket (%d): socket write failed for notification: %v", callingUser.(*models.User).ID, err)
					// exit
					return
				}
				s.logger.Debugf("BroadcastWebSocket (%d): notif wrote", callingId)
			case <-pingTicker.C:
				err = ws.Ping(ctx)
				if err != nil {
					s.logger.Errorf("BroadcastWebSocket (%d): ping failed: %v", callingId, err)
					return
				}
			}
		}
	})
}

func (s *HTTPServer) BroadcastMessage(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "past-week-active-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "BroadcastMessage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "BroadcastMessage", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load repo id from body
	message, ok := s.loadValue(w, r, reqJson, "BroadcastMessage", "message", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if message == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.BroadcastMessage(ctx, s.tiDB, s.sf, callingUser.(*models.User), message.(string))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "BroadcastMessage core failed", r.URL.Path, "BroadcastMessage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"broadcast-message",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "BroadcastMessage", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetBroadcastMessages(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-broadcast-messages-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := network.GetRequestIP(r)
	userName := network.GetRequestIP(r)
	callingIdInt := int64(0)
	if callingUser != nil {
		callingId = strconv.FormatInt(callingUser.(*models.User).ID, 10)
		userName = callingUser.(*models.User).UserName
		callingIdInt = callingUser.(*models.User).ID
	}

	// // return if calling user was not retrieved in authentication
	// if callingUser == nil {
	//	s.handleError(w, "calling user missing from context", r.URL.Path, "GetBroadcastMessages", r.Method, r.Context().Value(CtxKeyRequestID),
	//		network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
	//	return
	// }

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetBroadcastMessages", false, userName, callingIdInt)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetBroadcastMessages(ctx, s.tiDB)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetBroadcastMessages core failed", r.URL.Path, "GetBroadcastMessages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-broadcast-messages",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetBroadcastMessages", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) CheckBroadcastAward(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "check-broadcast-award-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	userName := ""

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CheckBroadcastAward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	} else {
		userName = callingUser.(*models.User).UserName
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CheckBroadcastAward", false, userName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CheckBroadcastAward(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CheckBroadcastAward core failed", r.URL.Path, "CheckBroadcastAward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"check-broadcast-award",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CheckBroadcastAward", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}

func (s *HTTPServer) RevertBroadcastAward(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "revert-broadcast-award-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	userName := ""

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "RevertBroadcastAward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "", callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	} else {
		userName = callingUser.(*models.User).UserName
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "RevertBroadcastAward", false, userName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GenerateUserOtpUri", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.RevertBroadcastAward(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "RevertBroadcastAward core failed", r.URL.Path, "RevertBroadcastAward", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"revert-broadcast-award",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingId),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "RevertBroadcastAward", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, callingId, http.StatusOK)
}
