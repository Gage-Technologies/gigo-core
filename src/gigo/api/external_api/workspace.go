package external_api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"tailscale.com/net/speedtest"

	"gigo-core/gigo/api/external_api/core"

	"github.com/gage-technologies/gigo-lib/db/models"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"github.com/gage-technologies/gigo-lib/mq/streams"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nats-io/nats.go"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func (s *HTTPServer) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "create-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "CreateWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load workspacePath from body
	workspacePath, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "commit", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if workspacePath == nil || !ok {
		return
	}

	// attempt to load repo id from body
	repoIdI, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "repo", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if repoIdI == nil || !ok {
		return
	}

	// parse post repo id to integer
	repoId, err := strconv.ParseInt(repoIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse repo id string to integer: %s", repoIdI.(string)), r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source id from body
	csIdI, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "code_source_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if csIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	csId, err := strconv.ParseInt(csIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse code source id string to integer: %s", repoIdI.(string)), r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// attempt to load code source type from body
	csType, ok := s.loadValue(w, r, reqJson, "CreateWorkspace", "code_source_type", reflect.Float64, nil, false, callingUser.(*models.User).UserName, callingId)
	if csType == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.CreateWorkspace(ctx, s.tiDB, s.vscClient, s.jetstreamClient, s.sf, s.wsStatusUpdater, callingUser.(*models.User),
		s.accessUrl.String(), repoId, workspacePath.(string), csId, models.CodeSource(csType.(float64)),
		s.hostname, s.useTls)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "CreateWorkspace core failed", r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"create-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "CreateWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-workspace-status-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load ownerCount from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "GetWorkspace", "id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post ownerCount to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", wsIdI.(string)), r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetWorkspaceStatus(ctx, s.tiDB, s.vscClient, callingUser.(*models.User), wsId, s.hostname, s.useTls)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetWorkspace core failed", r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-workspace-status",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

// WorkspaceWebSocket Will open and handle a write only websocket connection
func (s *HTTPServer) WorkspaceWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-websocket-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "WorkspaceWebSocket: calling user missing from context", r.URL.Path, "WorkspaceWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	vars := mux.Vars(r)
	wsIdI, ok := vars["id"]
	if wsIdI == "" || !ok {
		s.handleError(w, "WorkspaceWebSocket: workspace id missing from url", r.URL.Path, "WorkspaceWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusUnavailableForLegalReasons, "internal server error occurred", nil)
		return
	}

	// parse post ownerCount to integer
	wsId, err := strconv.ParseInt(wsIdI, 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("WorkspaceWebSocket: failed to parse workspace id string to integer: %s", wsIdI), r.URL.Path, "WorkspaceWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// /////////// Upgrade to websocket

	// validate the origin
	if !s.validateOrigin(w, r) {
		return
	}

	// accept websocket connection with client
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.handleError(w, fmt.Sprintf("WorkspaceWebSocket: failed to accept websocket connection"), r.URL.Path, "WorkspaceWebSocket", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	s.logger.Debugf("WorkspaceWebSocket (%d): accepted", wsId)

	// launch reader to handle reading the pong messages
	parentCtx, cancel := context.WithCancel(context.Background())
	ctx = ws.CloseRead(parentCtx)

	// write initialization message to socket so the client knows we're ready
	initMessage := "Socket connected successfully"
	err = ws.Write(ctx, websocket.MessageText, []byte(initMessage))
	if err != nil {
		cancel()
		s.logger.Errorf("WorkspaceWebSocket (%d): failed to write open message: %s", initMessage)
		return
	}

	// todo: check to see what SubscribeSync options we want here
	// create a synchronous subscriber to workspace status events
	sub, err := s.jetstreamClient.SubscribeSync(
		fmt.Sprintf(streams.SubjectWorkspaceStatusUpdateDynamic, wsId),
		nats.AckExplicit(),
	)
	if err != nil {
		cancel()
		s.logger.Errorf("WorkspaceWebSocket (%d): failed to subscribe to workspace %q events: %v", wsId, streams.SubjectWorkspaceStatusUpdateDynamic, err)
		return
	}

	// perform initial status call for complete status data
	status, err := core.GetWorkspaceStatus(ctx, s.tiDB, s.vscClient, callingUser.(*models.User), wsId, s.hostname, s.useTls)
	if err != nil {
		cancel()
		// handle error internally
		s.logger.Errorf("WorkspaceWebSocket (%d): workspace core failed: %v", wsId, err)
		// exit
		return
	}

	// write initial status data as the first message
	err = wsjson.Write(ctx, ws, status)
	if err != nil {
		cancel()
		s.logger.Errorf("WorkspaceWebSocket (%d): socket write failed: %v", wsId, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-websocket",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// create a channel to pipe workspace status updates from the
	// stream reader to the websocket handler
	statusUpdateChan := make(chan *models.WorkspaceFrontend)

	// launch goroutine to read next status message until we exit
	s.wg.Go(func() {
		lastWorkspaceState := status["workspace"].(*models.WorkspaceFrontend)
		for {
			select {
			// check if context is done and exit
			case <-ctx.Done():
				return
			// attempt to read another message
			default:
				// get next message from jetstream sub
				msg, err := sub.NextMsg(time.Millisecond * 100)
				if err != nil {
					// loop for timeout so we can check if our context has expired
					if errors.Is(err, nats.ErrTimeout) {
						continue
					}
					if !strings.Contains(err.Error(), "invalid subscription") {
						s.logger.Errorf("WorkspaceWebSocket (%d): failed to read next message from jetstream : %v", wsId, err)
					}
					return
				}

				// continue if the message is nil
				if msg == nil {
					continue
				}

				// ack message now - even if we fail later we don't want to repeat the message
				_ = msg.Ack()

				// create new frontend workspace model to decode msg into
				var statusMsg models2.WorkspaceStatusUpdateMsg
				decoder := gob.NewDecoder(bytes.NewBuffer(msg.Data))
				err = decoder.Decode(&statusMsg)
				if err != nil {
					s.logger.Errorf("WorkspaceWebSocket (%d): failed to decode workspace status message: %v", wsId, err)
					continue
				}

				// if this is a workspace start completed message then we should stall
				// the notification while we initialize a connection to the workspace
				if statusMsg.Workspace.InitState == models.WorkspaceInitCompleted &&
					lastWorkspaceState.InitState != models.WorkspaceInitCompleted {
					var agentId int64
					callerName := "WorkspaceWebSocket"
					err = s.tiDB.QueryRow(ctx, &parentSpan, &callerName, "SELECT _id FROM workspace_agent WHERE workspace_id = ? limit 1", wsId).Scan(&agentId)
					if err != nil {
						s.logger.Errorf("WorkspaceWebSocket (%d): failed to query workspace agent: %v", wsId, err)
						continue
					}

					s.logger.Debugf("WorkspaceWebSocket (%d): workspace start completed, acquiring connection to workspace agent", wsId)

					// acquire a connection to the workspace agent
					conn, release, err := s.WorkspaceAgentCache.Acquire(r, agentId)
					if err != nil {
						s.logger.Errorf("WorkspaceWebSocket (%d): failed to acquire connection to workspace agent: %v", wsId, err)
						continue
					}

					s.logger.Debugf("WorkspaceWebSocket (%d): workspace agent connection acquired; awaiting network reachability", wsId)

					// wait up to 10s for the workspace agent to become reachable
					reachableCtx, cancelReachableCtx := context.WithTimeout(context.TODO(), time.Second*10)
					reachable := conn.AwaitReachable(reachableCtx)
					cancelReachableCtx()
					if !reachable {
						s.logger.Errorf("WorkspaceWebSocket (%d): workspace agent connection failed to become reachable; dropping connection and re-establishing", wsId)
						// release the connection from the cache and close the connection
						// we need to create a new connection to the workspace agent
						release()
						s.WorkspaceAgentCache.ForgetAndClose(agentId)

						// create a new connection to the workspace agent
						conn, release, err = s.WorkspaceAgentCache.Acquire(r, agentId)
						if err != nil {
							s.logger.Errorf("WorkspaceWebSocket (%d): failed to acquire connection to workspace agent: %v", wsId, err)
							release()
							continue
						}

						// make another attempt to wait for the workspace agent to become reachable
						// but this time wait up to 30s
						reachableCtx, cancelReachableCtx := context.WithTimeout(context.TODO(), time.Second*30)
						reachable := conn.AwaitReachable(reachableCtx)
						cancelReachableCtx()
						if !reachable {
							release()
							// fail here since we can't connect to the workspace agent
							s.logger.Errorf("WorkspaceWebSocket (%d): workspace agent is not reachable", wsId)
							err = core.WorkspaceInitializationFailure(ctx, s.tiDB, s.wsStatusUpdater, wsId,
								models.WorkspaceInitVSCodeLaunch, "connecting to workspace", -1,
								"", "failed to establish connection to workspace", s.jetstreamClient)
							if err != nil {
								s.logger.Errorf("WorkspaceWebSocket (%d): failed to mark workspace as failed: %v", wsId, err)
								continue
							}
						}
					}

					s.logger.Debugf("WorkspaceWebSocket (%d): workspace agent is reachable; running speedtest", wsId)

					// make a direct http request to the workspace agent to initialize the connection
					_, err = conn.Speedtest(ctx, speedtest.Download, time.Second)
					if err != nil {
						s.logger.Errorf("WorkspaceWebSocket (%d): failed to initialize connection to workspace agent: %v", wsId, err)
						release()
						continue
					}

					s.logger.Debugf("WorkspaceWebSocket (%d): workspace agent connection initialized; waiting agent ready", wsId)

					// wait for the workspace agent to become ready
					timeout := time.After(time.Second * 30)
					for {
						exit := false
						select {
						case <-ctx.Done():
							return
						case <-timeout:
							s.logger.Errorf("WorkspaceWebSocket (%d): failed to wait agent ready", wsId)
							exit = true
							break
						default:
							// check if the agent is ready
							err = s.tiDB.QueryRowContext(ctx, &parentSpan, &callerName,
								"select _id from workspace_agent a where workspace_id = ? and a.state = ? order by a.created_at desc limit 1",
								wsId, models.WorkspaceAgentStateRunning,
							).Scan(&agentId)
							if err != nil {
								if err != sql.ErrNoRows {
									s.logger.Errorf("WorkspaceWebSocket (%d): failed to query workspace agent: %v", wsId, err)
								}
								s.logger.Debugf("WorkspaceWebSocket (%d): workspace agent not ready yet", wsId)
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

					s.logger.Debugf("WorkspaceWebSocket (%d): workspace agent is ready", wsId)

					// release the connection
					release()
					cancelReachableCtx()
				}

				// update the last workspace state
				lastWorkspaceState = statusMsg.Workspace

				// write decoded status message to status update channel to
				// be written to the websocket by the handler routine
				statusUpdateChan <- statusMsg.Workspace
			}
		}
	})

	// launch goroutine to handle the websocket from now on
	s.wg.Go(func() {
		// defer closure of websocket connection
		defer func() {
			cancel()
			_ = ws.Close(websocket.StatusGoingAway, "closing websocket")
			err = sub.Unsubscribe()
			if err != nil {
				s.logger.Errorf("WorkspaceWebSocket (%d): failed to unsubscribe to workspace %q events: %v", wsId, streams.SubjectWorkspaceStatusUpdateDynamic, err)
			}
		}()

		// create new ticker for pings
		pingTicker := time.NewTicker(time.Second)
		defer pingTicker.Stop()

		// loop until the socket closes
		for {
			select {
			case <-ctx.Done():
				s.logger.Debugf("WorkspaceWebSocket (%d): context canceled", wsId)
				return
			case status := <-statusUpdateChan:
				err = wsjson.Write(ctx, ws, status)
				if err != nil {
					s.logger.Errorf("WorkspaceWebSocket (%d): socket write failed: %v", wsId, err)
					// exit
					return
				}
			case <-pingTicker.C:
				err = ws.Ping(ctx)
				if err != nil {
					s.logger.Errorf("WorkspaceWebSocket (%d): ping failed: %v", wsId, err)
					return
				}
			}
		}
	})
}

func (s *HTTPServer) StreakCheck(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "streak-check-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StreakCheck", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to load coder workspace id from body
	workspaceId, ok := s.loadValue(w, r, reqJson, "StreakCheck", "workspace_id", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	wsId, err := strconv.ParseInt(workspaceId.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load coder workspace id from body
	secret, ok := s.loadValue(w, r, reqJson, "StreakCheck", "secret", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	decodedSecret, err := uuid.Parse(secret.(string))
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StreakCheck(ctx, s.tiDB, wsId, decodedSecret)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StreakCheck core failed", r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"streak-check",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, res, r.URL.Path, "StreakCheck", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) ExtendExpiration(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "extend-expiration-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "ExtendExpiration", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to load coder workspace id from body
	workspaceId, ok := s.loadValue(w, r, reqJson, "ExtendExpiration", "workspace_id", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	wsId, err := strconv.ParseInt(workspaceId.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load coder workspace id from body
	secret, ok := s.loadValue(w, r, reqJson, "ExtendExpiration", "secret", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	decodedSecret, err := uuid.Parse(secret.(string))
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.ExtendExpiration(ctx, s.tiDB, s.wsStatusUpdater, wsId, decodedSecret, s.jetstreamClient)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "ExtendExpiration core failed", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"extend-expiration",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, res, r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceAFK(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-afk-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceAFK", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to load coder workspace id from body
	workspaceId, ok := s.loadValue(w, r, reqJson, "WorkspaceAFK", "workspace_id", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	wsId, err := strconv.ParseInt(workspaceId.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "ExtendExpiration", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load minutes added from body
	rawMinutes, ok := s.loadValue(w, r, reqJson, "WorkspaceAFK", "add_min", reflect.String, nil, true, "workspace", "")
	if !ok {
		return
	}

	// load minutes added if it was passed
	addMin, err := strconv.ParseInt(rawMinutes.(string), 10, 64)
	if err != nil {
		return
	}

	// attempt to load coder workspace id from body
	secret, ok := s.loadValue(w, r, reqJson, "WorkspaceAFK", "secret", reflect.String, nil, false, "workspace", "")
	if workspaceId == nil || !ok {
		return
	}

	// decode the secret
	decodedSecret, err := uuid.Parse(secret.(string))
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.WorkspaceAFK(ctx, s.tiDB, s.wsStatusUpdater, wsId, decodedSecret, addMin)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "WorkspaceAFK core failed", r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-afk",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, res, r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceInitializationStepCompleted(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-initialization-step-completed-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceInitializationStepCompleted", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceInitializationStepCompleted",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to load step int from the json body
	state, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationStepCompleted", "state", reflect.Float64, nil, false, "workspace", "")
	if state == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WorkspaceInitializationStepCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	err := core.WorkspaceInitializationStepCompleted(ctx, s.tiDB, s.wsStatusUpdater, workspaceId.(int64), models.WorkspaceInitState(state.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "WorkspaceInitializationStepCompleted core failed", r.URL.Path,
			"WorkspaceInitializationStepCompleted", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", fmt.Sprintf("%d", workspaceId),
			http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-initialization-step-completed",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "WorkspaceInitializationStepCompleted", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceInitializationFailure(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-initialization-failure-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceInitializationFailure", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to retrieve workspace id from context
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceInitializationFailure",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to load step int from the json body
	state, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "state", reflect.Float64, nil, false, "workspace", "")
	if state == nil || !ok {
		return
	}

	// attempt to load command string from the json body
	command, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "command", reflect.String, nil, false, "workspace", "")
	if command == nil || !ok {
		return
	}

	// attempt to load status int from the json body
	status, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "status", reflect.Float64, nil, false, "workspace", "")
	if status == nil || !ok {
		return
	}

	// attempt to load stdout string from the json body
	stdout, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "stdout", reflect.String, nil, false, "workspace", "")
	if stdout == nil || !ok {
		return
	}

	// attempt to load stderr string from the json body
	stderr, ok := s.loadValue(w, r, reqJson, "WorkspaceInitializationFailure", "stderr", reflect.String, nil, false, "workspace", "")
	if stderr == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
		return
	}

	// execute core function logic
	err := core.WorkspaceInitializationFailure(ctx, s.tiDB, s.wsStatusUpdater, workspaceId.(int64), models.WorkspaceInitState(state.(float64)),
		command.(string), int(status.(float64)), stdout.(string), stderr.(string), s.jetstreamClient)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "WorkspaceInitializationFailure core failed", r.URL.Path, "WorkspaceInitializationFailure", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"workspace-initialization-failure",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "WorkspaceInitializationFailure", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)

}

func (s *HTTPServer) WorkspaceGetExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-extension-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("ext/gigo-developer.vsix")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-extension",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetCtExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-ct-extension-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetCtExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("ext/code-teacher.vsix")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetCtExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetCtExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-ct-extension",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetCtExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetThemeExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-theme-extension-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetThemeExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("ext/gigo-theme.vsix")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-theme-extension-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetThemeExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetHolidayThemeExtension(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-holiday-theme-extension-http")
	defer parentSpan.End()

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "WorkspaceGetHolidayThemeExtension", false, "workspace", -1)
	if reqJson == nil {
		return
	}

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "WorkspaceGetHolidayThemeExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// attempt to load step int from the json body
	holidayS, ok := s.loadValue(w, r, reqJson, "WorkspaceGetHolidayThemeExtension", "holiday", reflect.String, nil, false, "workspace", "")
	if holidayS == nil || !ok {
		return
	}

	// parse post id to integer
	holiday, err := strconv.ParseInt(holidayS.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse holiday string to integer: %s", holidayS.(string)), r.URL.Path, "WorkspaceGetHolidayThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}
	if holiday < 1 {
		s.handleError(w, "invalid holiday passed in", r.URL.Path, "WorkspaceGetHolidayThemeExtension",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile(fmt.Sprintf("ext/%v.vsix", agentsdk.Holiday(holiday).String()))
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "WorkspaceGetHolidayThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetThemeExtension", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get--holiday-theme-extension-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetHolidayThemeExtension",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) WorkspaceGetAgent(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-get-agent-http")
	defer parentSpan.End()

	// retrieve extension file from storage
	extension, err := s.storageEngine.GetFile("bin/agent")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve agent binary", r.URL.Path, "WorkspaceGetAgent", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer extension.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, extension)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "WorkspaceGetAgent", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-agent",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "WorkspaceGetAgent",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
}

func (s *HTTPServer) StartWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "start-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StartWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "StartWorkspace", "workspace_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", wsIdI.(string)), r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StartWorkspace(ctx, s.tiDB, s.jetstreamClient, s.wsStatusUpdater, callingUser.(*models.User), wsId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StartWorkspace core failed", r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"start-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "StartWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) StopWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "stop-workspace-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "StopWorkspace", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "StopWorkspace", "workspace_id", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse workspace id string to integer: %s", wsIdI.(string)), r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.StopWorkspace(ctx, s.tiDB, s.jetstreamClient, s.wsStatusUpdater, callingUser.(*models.User), wsId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "StopWorkspace core failed", r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"stop-workspace",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "StopWorkspace", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) GetHighestScore(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-highest-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetHighestScore", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.GetHighestScore(ctx, s.tiDB, callingUser.(*models.User))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetHighestScore core failed", r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-highest-score",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "GetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) SetHighestScore(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "set-highest-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SetHighestScore", false, callingUser.(*models.User).UserName, callingUser.(*models.User).ID)
	if reqJson == nil {
		return
	}

	// attempt to load code source id from body
	wsIdI, ok := s.loadValue(w, r, reqJson, "SetHighestScore", "highest_score", reflect.String, nil, false, callingUser.(*models.User).UserName, callingId)
	if wsIdI == nil || !ok {
		return
	}

	// parse post code source id to integer
	wsId, err := strconv.ParseInt(wsIdI.(string), 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to parse highest score string into int: %s", wsIdI.(string)), r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SetHighestScore(ctx, s.tiDB, callingUser.(*models.User), wsId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SetHighestScore core failed", r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"set-highest-score",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SetHighestScore", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)
}

func (s *HTTPServer) CodeServerPullThroughCache(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "code-server-pull-through-cache-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "CodeServerPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve the version, arch and install type from the query params
	version := r.URL.Query().Get("version")
	arch := r.URL.Query().Get("arch")
	os := r.URL.Query().Get("os")
	installType := r.URL.Query().Get("type")

	// ensure both values are provided
	if version == "" || arch == "" || os == "" || installType == "" {
		s.handleError(w, "version, arch, os and type are required", r.URL.Path, "CodeServerPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "version, arch, os and type are required", nil)
		return
	}

	// validate the install type
	if installType != "tar" && installType != "deb" && installType != "rpm" {
		s.handleError(w, "invalid install type provided", r.URL.Path, "CodeServerPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "invalid install type provided", nil)
		return
	}

	// attempt to retrieve file from storage
	buf, err := s.storageEngine.GetFile(fmt.Sprintf("ext/code-server-cache/%s-%s-%s-%s", version, arch, os, installType))
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "CodeServerPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// if we miss the cache then download the file from the source
	download := false
	if buf == nil {
		// if we have a cache miss then download the file from the source
		buf, err = core.CodeServerPullThroughCache(r.Context(), s.storageEngine, version, arch, os, installType)
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to retrieve extension file", r.URL.Path, "CodeServerPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		download = true
	}

	// ensure closure of file
	defer buf.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, buf)
	if err != nil {
		// if we are downloading then we need to invalidate the cache
		if download {
			// delete the file from storage
			_ = s.storageEngine.DeleteFile(fmt.Sprintf("ext/code-server-cache/%s-%s-%s-%s", version, arch, os, installType))
		}

		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "CodeServerPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-code-server",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "CodeServerPullThroughCache",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
	return
}

func (s *HTTPServer) OpenVsxPullThroughCache(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "openvsx-pull-through-cache-http")
	defer parentSpan.End()

	// attempt to retrieve workspace id from context
	// this is all the validation we need since this requires
	// that the workspace id and agent secret were provided
	workspaceId := r.Context().Value("workspace")
	if workspaceId == nil {
		s.handleError(w, "workspace missing in context", r.URL.Path, "OpenVsxPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusInternalServerError, "internal server error", nil)
		return
	}

	// retrieve the extension id and version from the query params
	extensionId := r.URL.Query().Get("ext")
	version := r.URL.Query().Get("version")
	vscVersion := r.URL.Query().Get("vscVersion")

	// ensure the extension id is provided
	if extensionId == "" {
		s.handleError(w, "extension id required", r.URL.Path, "OpenVsxPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "extension id required", nil)
		return
	}

	// split the extension id into the publisher and name
	extensionIdSplit := strings.Split(extensionId, ".")
	if len(extensionIdSplit) != 2 {
		s.handleError(w, "invalid extension id provided", r.URL.Path, "OpenVsxPullThroughCache",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
			"anon", "-1", http.StatusBadRequest, "invalid extension id provided", nil)
		return
	}

	// retrieve the extension
	buf, download, err := core.OpenVsxPullThroughCache(r.Context(), s.storageEngine, s.rdb, extensionId, version, vscVersion)
	if err != nil {
		if strings.Contains(err.Error(), "extension not found") {
			s.handleError(w, fmt.Sprintf("extension not found: %s - %s", extensionId, version), r.URL.Path, "OpenVsxPullThroughCache",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusNotFound, "extension not found", nil)
			return
		}
		// handle error internally
		s.handleError(w, "failed to retrieve extension file", r.URL.Path, "OpenVsxPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		// exit
		return
	}

	// ensure closure of file
	defer buf.Close()

	// add headers
	w.Header().Set("Content-Type", "application/octet-stream")

	// set status code
	w.WriteHeader(http.StatusOK)

	// copy file bytes to response
	_, err = io.Copy(w, buf)
	if err != nil {
		// if we are downloading then we need to invalidate the cache
		if download {
			// delete the file from storage
			_ = s.storageEngine.DeleteFile(fmt.Sprintf("ext/open-vsx-cache/%s/%s.%s.vsix", extensionIdSplit[0], extensionIdSplit[1], version))
		}

		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "OpenVsxPullThroughCache", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	parentSpan.AddEvent(
		"workspace-get-extension-cache",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("extension", extensionId),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "OpenVsxPullThroughCache",
		r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "",
		http.StatusOK, nil)
	return
}
