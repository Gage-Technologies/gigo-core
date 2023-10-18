package external_api

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strconv"
	"time"

	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func (s *HTTPServer) StreakHandlerExt(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "streak-handler-ext-http")
	defer parentSpan.End()

	// retrieve url parameters
	urlParams := mux.Vars(r)

	// extract workspace id and secret from url parameters
	wsIdString, ok := urlParams["id"]
	if !ok {
		s.handleError(w, "workspace id missing from url", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusBadRequest, "missing workspace id in url", nil)
		return
	}
	wsSecret, ok := urlParams["secret"]
	if !ok {
		s.handleError(w, "workspace secret missing from url", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusBadRequest, "missing workspace secret in url", nil)
		return
	}

	wsId, err := strconv.ParseInt(wsIdString, 10, 64)
	if err != nil {
		s.handleError(w, "failed to decode id", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	decodedSecret, err := uuid.Parse(wsSecret)
	if err != nil {
		s.handleError(w, "failed to decode secret", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	callerName := "StreakHandlerExt"

	res, err := s.tiDB.QueryContext(ctx, &parentSpan, &callerName, "SELECT a.owner_id, u.timezone, u.user_name FROM workspace_agent a join users u on a.owner_id = u._id WHERE a.workspace_id =? and a.secret = uuid_to_bin(?)", wsId, decodedSecret)
	if err != nil {
		s.logger.Errorf("StreakHandler: failed to query database for user id and timezone: %v", err)
		s.handleError(w, "failed query for user details", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	defer res.Close()

	var ownerID int64
	var timezone string
	var userName string

	for res.Next() {
		err = res.Scan(&ownerID, &timezone, &userName)
		if err != nil {
			s.logger.Errorf("StreakHandler: failed to scan results from query for user id and timezone: %v", err)
			s.handleError(w, "failed to decode secret", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", nil)
			return
		}
	}

	if timezone == "" || userName == "" || ownerID == 0 {
		s.logger.Errorf("StreakHandler: failed to query database for user id and timezone: results returned empty from query: \n SELECT a.owner_id, u.timezone, u.user_name FROM workspace_agent a join users u on a.owner_id = u._id WHERE a.workspace_id =%v and a.secret = uuid_to_bin(%v)", wsId, decodedSecret)
		s.handleError(w, "failed to decode timezone, username, ownerID", r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "workspace", "", http.StatusInternalServerError, "internal server error occurred", errors.New("failed to decode timezone, username, ownerID"))
		return
	}
	//
	// // check if this is a test
	// if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
	//	// return success for test
	//	s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "StreakHandlerExt", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "workspace", "", http.StatusOK)
	//	return
	// }

	s.logger.Debugf("StreakHandler: starting")

	s.logger.Debugf("StreakHandler: accepting connection")

	// accept websocket connection with client
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to accept websocket connection"), r.URL.Path, "StreakHandler", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, strconv.FormatInt(ownerID, 10), http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	s.logger.Debugf("StreakHandler: accepted")

	// launch reader to handle reading the pong messages
	ctx = ws.CloseRead(context.Background())

	s.logger.Debugf("StreakHandler: connected")

	// write initialization message to socket so the client knows we're ready
	initMessage := "Socket connected successfully"
	err = ws.Write(ctx, websocket.MessageText, []byte(initMessage))
	if err != nil {
		s.logger.Errorf("StreakHandler: failed to write open message: %s", initMessage)
		return
	}

	parentSpan.AddEvent(
		"streak-handler-ext",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.logger.Debugf("StreakHandler: wrote open message")

	// set default leader value to be a follower
	isLeader := false

	// create user lock
	uLock := s.lockManager.GetLock(fmt.Sprintf("user_streak_update:%v", ownerID))

	// create function to handle the locking of a user status
	// and the assignment of leader status to this session
	acquireLockFunc := func() {
		// create a context of 500ms to acquire the lock
		// if the lock cannot be acquired within this timeframe
		// we will continue as a follower
		lockAcquireCtx, lockAcquireCancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		defer lockAcquireCancel()
		err := uLock.LockContext(lockAcquireCtx)
		if err != nil {
			// mark as follower wince no matter what our prior state is
			// we have now become a follower
			isLeader = false

			// only log the error if this was an abnormal error
			if !errors.Is(err, redsync.ErrFailed) {
				s.logger.Errorf("StreakHandler: failed to acquire lock: %s", err)
			}

			return
		}

		// we have successfully acquired the lock and
		// therefore are the leader
		isLeader = true
	}

	// create function to handle pushing the workspace status to the frontend
	statusPushFunc := func() {
		defer func() {
			// recover from panic if one occured. Set err to nil otherwise.
			if recover() != nil {
				s.logger.Debugf("StreakHandler: panic recovered: %v", recover())
			}
		}()

		s.logger.Debugf("StreakHandler: executing status push")

		// attempt to acquire leader status if we are the follower
		if !isLeader {
			acquireLockFunc()
		}

		s.logger.Debugf("StreakHandler: leader election complete - leader status: %v", isLeader)

		// call check elapsed streak time, and get resmap as response
		resMap, err := core.CheckElapsedStreakTime(ctx, s.tiDB, ownerID, timezone, s.logger)
		if err != nil {
			// handle error internally
			s.logger.Errorf("StreakHandler: failed to check elapsed streak time: %s", err)
			// exit
			return
		}

		// sanity check that we have all the required fields
		if resMap == nil {
			s.logger.Errorf("StreakHandler: failed to check elapsed streak time, err: result map returned nil")
			return
		}

		s.logger.Debugf("StreakHandler: return map: %v", resMap)

		// ensure that elapsed time is not nil after return
		if resMap["elapsed_time"] == nil {
			s.logger.Errorf("StreakHandler: failed to check elapsed streak time, err: result map returned nil")
			return
		}

		// if socket is leader determine if update is needed
		if isLeader {
			// get the amount of time elapsed
			elapsedStreakTime := resMap["elapsed_time"].(time.Duration)

			// if the elapsed time is greater than the update interval and the users streak is not active today
			if elapsedStreakTime >= time.Minute*30 && !resMap["streak_active"].(bool) {
				s.logger.Debugf("StreakHandler: updating streak with elapsed time: %v for user: %v", elapsedStreakTime, ownerID)

				// update the streak values for the user
				err = core.UpdateStreak(ctx, s.tiDB, ownerID, resMap["current_streak"].(int), resMap["longest_streak"].(int), timezone)
				if err != nil {
					s.logger.Errorf("StreakHandler: failed to update streak: %v", err)
					return
				}

				// retrieve updated map for user after streak update
				resMap, err = core.CheckElapsedStreakTime(ctx, s.tiDB, ownerID, timezone, s.logger)
				if err != nil {
					// handle error internally
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update: %s", err)
					// exit
					return
				}

				s.logger.Debugf("StreakHandler: return map before check: %v", resMap)

				// sanity check that we have all the required fields
				if resMap == nil {
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update, err: result map returned nil")
					return
				}
				if resMap["elapsed_time"] == nil {
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update, err: result map returned nil")
					return
				}

				s.logger.Debugf("StreakHandler: return map: %v", resMap)

				if !resMap["streak_active"].(bool) {
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update, err: streak active variable was not updated")
					return
				}

				s.logger.Debugf("StreakHandler: updated streak")
			}

		}

		// write results to json
		err = wsjson.Write(ctx, ws, resMap)
		if err != nil {
			s.logger.Errorf("StreakHandler: socket write failed: %v", err)
			// exit
			return
		}
	}

	// launch goroutine to handle the websocket from now on
	s.wg.Go(func() {
		// defer closure of websocket connection
		defer func() {
			// release the lock if we are the leader
			if isLeader {
				_, err := uLock.Unlock()
				if err != nil {
					s.logger.Errorf("StreakHandler: failed to release lock: %v", err)
				}
			}

			// kill the lock so that we close its goroutine
			uLock.Kill()

			_ = ws.Close(websocket.StatusGoingAway, "closing websocket")
		}()

		// create new ticker for status push
		statusTicker := time.NewTicker(time.Second * 3)
		defer statusTicker.Stop()

		// create new ticker for pings
		pingTicker := time.NewTicker(time.Second)
		defer pingTicker.Stop()

		// execute the first status push
		statusPushFunc()

		// loop until the socket closes
		for {
			select {
			case <-ctx.Done():
				s.logger.Debugf("StreakHandler: context canceled")
				return
			case <-statusTicker.C:
				statusPushFunc()
			case <-pingTicker.C:
				s.logger.Debugf("StreakHandler: ping")
				err = ws.Ping(ctx)
				if err != nil {
					s.logger.Errorf("StreakHandler: ping failed: %v", err)
					return
				}
			}
		}
	})
}

func (s *HTTPServer) StreakHandlerFrontend(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "streak-handler-frontend-http")
	defer parentSpan.End()

	s.logger.Debugf("StreakHandler: starting")

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	s.logger.Debugf("StreakHandler: callingId: %s", callingId)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "StreakHandler", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	s.logger.Debugf("StreakHandler: accepting connection")

	// validate the origin
	if !s.validateOrigin(w, r) {
		return
	}

	// accept websocket connection with client
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.handleError(w, fmt.Sprintf("failed to accept websocket connection"), r.URL.Path, "StreakHandler", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	s.logger.Debugf("StreakHandler: accepted")

	// launch reader to handle reading the pong messages
	ctx = ws.CloseRead(context.Background())

	s.logger.Debugf("StreakHandler: connected")

	// write initialization message to socket so the client knows we're ready
	initMessage := "Socket connected successfully"
	err = ws.Write(ctx, websocket.MessageText, []byte(initMessage))
	if err != nil {
		s.logger.Errorf("StreakHandler: failed to write open message: %s", initMessage)
		return
	}

	parentSpan.AddEvent(
		"streak-handler-frontend",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	s.logger.Debugf("StreakHandler: wrote open message")

	// set default leader value to be a follower
	isLeader := false

	// create user lock
	uLock := s.lockManager.GetLock(fmt.Sprintf("user_streak_update:%v", callingUser.(*models.User).ID))

	// create function to handle the locking of a user status
	// and the assignment of leader status to this session
	acquireLockFunc := func() {
		// create a context of 500ms to acquire the lock
		// if the lock cannot be acquired within this timeframe
		// we will continue as a follower
		lockAcquireCtx, lockAcquireCancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		defer lockAcquireCancel()
		err := uLock.LockContext(lockAcquireCtx)
		if err != nil {
			// mark as follower wince no matter what our prior state is
			// we have now become a follower
			isLeader = false

			// only log the error if this was an abnormal error
			if !errors.Is(err, redsync.ErrFailed) {
				s.logger.Errorf("StreakHandler: failed to acquire lock: %s", err)
			}

			return
		}

		// we have successfully acquired the lock and
		// therefore are the leader
		isLeader = true
	}

	// create function to handle pushing the workspace status to the frontend
	statusPushFunc := func() {
		s.logger.Debugf("StreakHandler: executing status push")

		// attempt to acquire leader status if we are the follower
		if !isLeader {
			acquireLockFunc()
		}

		s.logger.Debugf("StreakHandler: after acquire lock")

		// call check elapsed streak time, and get resmap as response
		resMap, err := core.CheckElapsedStreakTime(ctx, s.tiDB, callingUser.(*models.User).ID, callingUser.(*models.User).Timezone, s.logger)
		if err != nil {
			// handle error internally
			s.logger.Errorf("StreakHandler: failed to check elapsed streak time: %s", err)
			// exit
			return
		}

		s.logger.Debugf("StreakHandler: after check elapsed streak time")

		// sanity check that we have all the required fields
		if resMap == nil {
			s.logger.Errorf("StreakHandler: failed to check elapsed streak time, err: result map returned nil")
			return
		}

		// ensure that elapsed time is not nil after return
		if resMap["elapsed_time"] == nil {
			s.logger.Errorf("StreakHandler: failed to check elapsed streak time, err: result map returned nil")
			return
		}

		s.logger.Debugf("StreakHandler: return map: %v", resMap)

		// if socket is leader determine if update is needed
		if isLeader {
			// get the amount of time elapsed
			elapsedStreakTime := resMap["elapsed_time"].(time.Duration)

			// if the elapsed time is greater than the update interval and the users streak is not active today
			if elapsedStreakTime >= time.Minute*30 && !resMap["streak_active"].(bool) {
				s.logger.Debugf("StreakHandler: updating streak")

				// update the streak values for the user
				err = core.UpdateStreak(ctx, s.tiDB, callingUser.(*models.User).ID, resMap["current_streak"].(int), resMap["longest_streak"].(int), callingUser.(*models.User).Timezone)
				if err != nil {
					s.logger.Errorf("StreakHandler: failed to update streak: %v", err)
					return
				}

				// retrieve updated map for user after streak update
				resMap, err = core.CheckElapsedStreakTime(ctx, s.tiDB, callingUser.(*models.User).ID, callingUser.(*models.User).Timezone, s.logger)
				if err != nil {
					// handle error internally
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update: %s", err)
					// exit
					return
				}

				// sanity check that we have all the required fields
				if resMap == nil {
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update, err: result map returned nil")
					return
				}
				if resMap["elapsed_time"] == nil {
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update, err: result map returned nil")
					return
				}
				if !resMap["streak_active"].(bool) {
					s.logger.Errorf("StreakHandler: failed to check elapsed streak time after update, err: streak active variable was not updated")
					return
				}

				s.logger.Debugf("StreakHandler: updated streak")
			}

		}

		// write results to json
		err = wsjson.Write(ctx, ws, resMap)
		if err != nil {
			s.logger.Errorf("StreakHandler: socket write failed: %v", err)
			// exit
			return
		}
	}

	// launch goroutine to handle the websocket from now on
	s.wg.Go(func() {
		// defer closure of websocket connection
		defer func() {
			// release the lock if we are the leader
			if isLeader {
				_, err := uLock.Unlock()
				if err != nil {
					s.logger.Errorf("StreakHandler: failed to release lock: %v", err)
				}
			}

			// kill the lock so that we close its goroutine
			uLock.Kill()

			_ = ws.Close(websocket.StatusGoingAway, "closing websocket")
		}()

		// create new ticker for status push
		statusTicker := time.NewTicker(time.Second * 3)
		defer statusTicker.Stop()

		// create new ticker for pings
		pingTicker := time.NewTicker(time.Second)
		defer pingTicker.Stop()

		statusPushFunc()
		// execute the first status push

		// loop until the socket closes
		for {
			select {
			case <-ctx.Done():
				s.logger.Debugf("StreakHandler: context canceled")
				return
			case <-statusTicker.C:
				statusPushFunc()
			case <-pingTicker.C:
				s.logger.Debugf("StreakHandler: ping")
				err = ws.Ping(ctx)
				if err != nil {
					s.logger.Errorf("StreakHandler: ping failed: %v", err)
					return
				}
			}
		}
	})
}

func (s *HTTPServer) GetUserStreaks(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-user-streaks-http")
	defer parentSpan.End()

	s.logger.Debugf("GetUserStreaks: start of call")
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetUserStreaks", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetUserStreaks", false, "", -1)
	if reqJson == nil {
		return
	}

	// execute core function logic
	res, err := core.GetUserStreaks(ctx, s.tiDB, callingUser.(*models.User), s.logger)
	s.logger.Debugf("GetUserStreaks: calling core function now")
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetUserStreaks core failed", r.URL.Path, "GetUserStreaks", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-user-streaks",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	s.logger.Debugf("GetUserStreaks: called core function")

	// return JSON response
	s.jsonResponse(r, w, res, r.URL.Path, "GetUserStreaks", r.Method, r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)

}

func (s *HTTPServer) GetStreakFreezeCount(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "get-streak-freeze-count-http")
	defer parentSpan.End()

	s.logger.Debugf("GetStreakFreezeCount: start of call")
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)
	callingId := strconv.FormatInt(callingUser.(*models.User).ID, 10)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GetStreakFreezeCount", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, "-1", http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GetStreakFreezeCount", false, "", -1)
	if reqJson == nil {
		return
	}

	// execute core function logic
	res, err := core.GetStreakFreezeCount(ctx, s.tiDB, callingUser.(*models.User))
	s.logger.Debugf("GetStreakFreezeCount: calling core function now")
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "GetStreakFreezeCount core failed", r.URL.Path, "GetStreakFreezeCount", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"get-streak-freeze-count",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	s.logger.Debugf("GetStreakFreezeCount: called core function")

	// return JSON response
	s.jsonResponse(r, w, res, r.URL.Path, "GetStreakFreezeCount", r.Method, r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK)

}
