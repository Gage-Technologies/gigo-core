package external_api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	middleware "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gigo-core/gigo/lock"
	"gigo-core/gigo/streak"

	"github.com/go-playground/validator"
	"github.com/sourcegraph/conc"

	wsApi "gigo-core/coder/api"
	"gigo-core/gigo/api/ws"
	"gigo-core/gigo/config"
	utils2 "gigo-core/gigo/utils"

	"github.com/buger/jsonparser"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redis_rate/v9"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/rs/cors"
)

type RoutePermission int

const (
	RoutePermissionPrivate RoutePermission = iota
	RoutePermissionPublic
	RoutePermissionHybrid

	CdnAccessHeader = "X-CDN-SECRET"

	CtxKeyCache      = "cache"
	CtxKeyCacheKey   = "cacheKey"
	CtxKeyIP         = "ip"
	CtxKeyUser       = "callingUser"
	CtxKeyUserID     = "callingUserID"
	CtxKeyUserName   = "callingUserName"
	CtxKeyRequestID  = "requestID"
	CtxKeyBodyBuffer = "bodyBuffer"
)

var publicRoutes = []*regexp.Regexp{
	// permit login functions
	regexp.MustCompile("^/api/auth/login([^/]+)?$"),
	regexp.MustCompile("^/api/user/forgotPasswordValidation$"),
	regexp.MustCompile("^/api/user/resetForgotPassword$"),
	regexp.MustCompile("^/api/verifyResetToken/[^/]+/[^/]+$"),
	regexp.MustCompile("^/api/auth/referralUserInfo"),
	// permit user creation
	regexp.MustCompile("^/api/user/createNewUser$"),
	regexp.MustCompile("^/api/user/createNewGithubUser$"),
	regexp.MustCompile("^/api/user/createNewGoogleUser$"),
	regexp.MustCompile("^/api/user/validateUser$"),
	regexp.MustCompile("^/api/email/verify$"),
	// permit stripe webhook
	regexp.MustCompile("^/api/stripe/webhook$"),
	regexp.MustCompile("^/api/stripe/connected/webhook$"),
	// permit all internal routes
	regexp.MustCompile("^/internal"),
	// permit all derp traffic
	regexp.MustCompile("^/derp"),
	// permit live checks
	regexp.MustCompile("^/ping$"),
	regexp.MustCompile("^/healthz$"),
	// permit access to static files
	regexp.MustCompile("^/static/ext/.*$"),
	regexp.MustCompile("^/static/ui/.*$"),
	//permit access to the sitemap
	regexp.MustCompile("^/sitemap/sitemap.xml$"),
	// permit access to unsubscribe check and modify for non-logged-in users
	regexp.MustCompile("^/api/unsubscribe/check$"),
	regexp.MustCompile("^/api/unsubscribe/modify$"),

	// TODO: REMOVE THIS!!!! SUPER INSECURE!!!!
	// regexp.MustCompile("^/debug"),
}

var hybridRoutes = []*regexp.Regexp{
	regexp.MustCompile("^/api/ws$"),
	regexp.MustCompile("^/api/home/.*$"),
	regexp.MustCompile("^/api/user/userProjects$"),
	regexp.MustCompile("^/api/notification/get$"),
	regexp.MustCompile("^/api/broadcast/get$"),
	regexp.MustCompile("^/api/search/users$"),
	regexp.MustCompile("^/api/search/posts$"),
	regexp.MustCompile("^/static/user/pfp.*$"),
	regexp.MustCompile("^/static/posts/t.*$"),
	regexp.MustCompile("^/static/attempts/t.*$"),
	regexp.MustCompile("^/api/project/attempts$"),
	regexp.MustCompile("^/api/project/get$"),
	regexp.MustCompile("^/api/project/closedAttempts$"),
	regexp.MustCompile("^/api/discussion/getDiscussions$"),
	regexp.MustCompile("^/api/user/profilePage$"),
	regexp.MustCompile("^/api/user/getId$"),
	regexp.MustCompile("^/api/project/getProjectCode$"),
	regexp.MustCompile("^/api/discussion/getComments$"),
	regexp.MustCompile("^/api/project/getProjectDirectories$"),
	regexp.MustCompile("^/api/project/getProjectFiles$"),
	regexp.MustCompile("^/api/discussion/getThreads$"),
	regexp.MustCompile("^/api/discussion/getThreadReply$"),
	regexp.MustCompile("^/api/attempt/get$"),
	regexp.MustCompile("^/api/attempt/getProject$"),
	regexp.MustCompile("^/api/search/tags$"),
	regexp.MustCompile("^/api/chat/messages$"),
	regexp.MustCompile("^/api/ephemeral/create$"),
	regexp.MustCompile("^/api/project/verifyLink$"),
	regexp.MustCompile("^/api/verifyRecaptcha$"),
	regexp.MustCompile("^/api/recordUsage$"),

	// // permit popular
	// regexp.MustCompile("^/api/popular$"),
	// // permit search endpoints
	// regexp.MustCompile("^/api/search/[^/]+$"),
	// // permit project data
	// regexp.MustCompile("^/api/project/get$"),
	// regexp.MustCompile("^/api/project/attempts$"),
	// regexp.MustCompile("^/api/project/getProjectFiles$"),
	// regexp.MustCompile("^/api/project/closedAttempts$"),
	// // permit static file endpoints
	// regexp.MustCompile("^/static/"),
}

var cacheEndpoints = []*EndpointCache{
	{
		Path:         regexp.MustCompile("^/api/project/get$"),
		Method:       "POST",
		TTL:          5 * time.Minute,
		KeyFields:    []string{"post_id"},
		UserKey:      true,
		RefreshOnHit: false,
		// Add regex to match endpoints that will trigger cache invalidation here
		InvalidateOn: regexp.MustCompile("^/api/project/editProject$"),
	},
	{
		Path:         regexp.MustCompile("^/api/project/attempts$"),
		Method:       "POST",
		TTL:          5 * time.Minute,
		KeyFields:    []string{"project_id", "skip", "limit"},
		UserKey:      true,
		RefreshOnHit: false,
		InvalidateOn: regexp.MustCompile("^/api/project/editAttempt$"),
	},
	{
		Path:         regexp.MustCompile("^/api/project/closedAttempts$"),
		Method:       "POST",
		TTL:          5 * time.Minute,
		KeyFields:    []string{"project_id", "skip", "limit"},
		UserKey:      true,
		RefreshOnHit: false,
	},
	{
		Path:         regexp.MustCompile("^/api/broadcast/get$"),
		Method:       "POST",
		TTL:          5 * time.Minute,
		KeyFields:    []string{},
		UserKey:      true,
		RefreshOnHit: false,
	},
	{
		Path:         regexp.MustCompile("^/api/attempt/getProject$"),
		Method:       "POST",
		TTL:          5 * time.Minute,
		KeyFields:    []string{"attempt_id"},
		UserKey:      true,
		RefreshOnHit: false,
	},
	{
		Path:         regexp.MustCompile("^/api/user/getId$"),
		Method:       "POST",
		TTL:          7 * 24 * time.Hour,
		KeyFields:    []string{"username"},
		UserKey:      false,
		RefreshOnHit: true,
	},
}

type EndpointCache struct {
	Path         *regexp.Regexp
	Method       string
	TTL          time.Duration
	KeyFields    []string
	UserKey      bool
	RefreshOnHit bool
	InvalidateOn *regexp.Regexp
}

type CachedResponse struct {
	Body   map[string]interface{}
	Status int
}

type HTTPServer struct {
	server                       *http.Server
	listener                     *net.Listener
	router                       *mux.Router
	tiDB                         *ti.Database
	meili                        *search.MeiliSearchEngine
	rdb                          redis.UniversalClient
	sf                           *snowflake.Node
	streakEngine                 *streak.StreakEngine
	vscClient                    *git.VCSClient
	lockManager                  *lock.RedLockManager
	storageEngine                storage.Storage
	workspaceClient              *ws.WorkspaceClient
	jetstreamClient              *mq.JetstreamClient
	wsStatusUpdater              *utils2.WorkspaceStatusUpdater
	limiter                      *redis_rate.Limiter
	wg                           *conc.WaitGroup
	passwordFilter               *utils2.PasswordFilter
	memPool                      *sync.Pool
	hostname                     string
	useTls                       bool
	accessUrl                    *url.URL
	logger                       logging.Logger
	developmentMode              bool
	publicIp                     string
	gitWebhookSecret             string
	stripeWebhookSecret          string
	stripeConnectedWebhookSecret string
	stableDiffusionHost          string
	stableDiffusionKey           string
	mailGunKey                   string
	mailGunDomain                string
	mailGunVerificationKey       string
	gigoEmail                    string
	domain                       string
	githubSecret                 string
	initialRecUrl                string
	forceCdn                     bool
	cdnKey                       string
	allowedOrigins               []string
	whitelistedIpRanges          []*net.IPNet
	validator                    *validator.Validate
	curatedSecret                string
	masterKey                    string
	captchaSecret                string
	zitiServer                   *zitimesh.Server
}

// CreateHTTPServer Creates a new HTTPServer object
// This function performs the following actions:
//   - creates router to manager incoming traffic flow
//   - link static directory
//   - open and validate TLS certificate
//   - create server with router, address, port, and certificate
//   - create listener on the passed address and port
//   - initialize logger
//   - create HTTPServer object
//   - link router to API endpoints
//
// Args:
//
//			host           - string, address that the server will run on
//			port           - string, port that the server will run on
//			protocol       - string, protocol that will be used for internet traffic
//			devEmail       - string, developer email to be used for notifications
//			db             - *tiDB.Database, database object connected to the MongoDB instance
//			alertConfig    - config.AlertConfig, configuration for the DragonflyAlert system
//			s3Client 	   - *s3.S3Client, S3 client that will be used for cache operations
//			proxyManager   - network.ProxyManager, proxy manager that will be used for all external network interactions
//			hostSite       - *string, optional domain to that the server will run on; if passed TLS certification is auto
//	                                configured for the passed domain
//			useTls         - bool, whether to use TLS encryption to secure the connection via SSL
//			keyPath        - string, path to the key directory
//
// Returns:
//
//	out            - *HTTPServer, freshly created HTTPServer object with the HTTP API initialized
func CreateHTTPServer(cfg config.HttpServerConfig, otelServiceName string, tidb *ti.Database, meili *search.MeiliSearchEngine,
	rdb redis.UniversalClient, sf *snowflake.Node, giteaClient *git.VCSClient, storageEngine storage.Storage,
	wsClient *ws.WorkspaceClient, js *mq.JetstreamClient, wsStatusUpdater *utils2.WorkspaceStatusUpdater,
	accessUrl *url.URL, passwordFilter *utils2.PasswordFilter, githubSecret string, forceCdn bool, cdnKey string, masterKey string, captchaSecret string,
	whitelistedIpRanges []*net.IPNet, zitiServer *zitimesh.Server, logger logging.Logger) (*HTTPServer, error) {

	// create MUX router to enable complex HTTP applications
	r := mux.NewRouter()

	// create empty listener to be filled
	var lis net.Listener

	// open port to listen on
	lis, err := net.Listen("tcp", cfg.Address+":"+cfg.Port)
	if err != nil {
		return nil, err
	}

	// create http server object encrypted with TLS
	s := &http.Server{
		Handler:   r,
		TLSConfig: nil,
		Addr:      cfg.Address + ":" + cfg.Port,
	}

	// retrieve public ip address
	// publicIp, err := network.GetSelfPublicIP()
	// if err != nil {
	// 	return nil, err
	// }

	// create new redis rate limiter
	limiter := redis_rate.NewLimiter(rdb)

	streakEngine := streak.NewStreakEngine(tidb, rdb, sf, logger)

	lockManager := lock.CreateRedLockManager(rdb)

	// create allowed origins
	allowedOrigins := []string{
		// production
		fmt.Sprintf("https://*.%s", cfg.Domain),
		"https://www.gigo.dev",
		"https://gigo.dev",
		// development callers
		"https://ui-dev.gigo.dev:*",
	}

	// create server object
	server := &HTTPServer{
		server:                       s,
		listener:                     &lis,
		router:                       r,
		tiDB:                         tidb,
		meili:                        meili,
		rdb:                          rdb,
		sf:                           sf,
		streakEngine:                 streakEngine,
		wg:                           conc.NewWaitGroup(),
		lockManager:                  lockManager,
		hostname:                     cfg.Hostname,
		useTls:                       cfg.UseTLS,
		accessUrl:                    accessUrl,
		logger:                       logger,
		memPool:                      &sync.Pool{},
		developmentMode:              cfg.DevelopmentMode,
		publicIp:                     "74.195.164.140",
		workspaceClient:              wsClient,
		jetstreamClient:              js,
		wsStatusUpdater:              wsStatusUpdater,
		limiter:                      limiter,
		vscClient:                    giteaClient,
		storageEngine:                storageEngine,
		gitWebhookSecret:             cfg.GitWebhookSecret,
		stripeWebhookSecret:          cfg.StripeWebhookSecret,
		stripeConnectedWebhookSecret: cfg.StripeConnectedWebhookSecret,
		stableDiffusionHost:          cfg.StableDiffusionHost,
		stableDiffusionKey:           cfg.StableDiffusionKey,
		mailGunKey:                   cfg.MailGunApiKey,
		mailGunDomain:                cfg.MailGunDomain,
		mailGunVerificationKey:       cfg.MailGunVerificationKey,
		gigoEmail:                    cfg.GigoEmail,
		domain:                       cfg.Domain,
		passwordFilter:               passwordFilter,
		githubSecret:                 githubSecret,
		initialRecUrl:                cfg.InitialRecommendationURl,
		forceCdn:                     forceCdn,
		cdnKey:                       cdnKey,
		allowedOrigins:               allowedOrigins,
		whitelistedIpRanges:          whitelistedIpRanges,
		validator:                    validator.New(),
		curatedSecret:                cfg.CuratedSecret,
		masterKey:                    masterKey,
		captchaSecret:                captchaSecret,
		zitiServer:                   zitiServer,
	}

	// TODO: refine a more conservative CORS policy
	// create cors handler
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		Debug:            false,
	})

	// link cors & authentication middleware to router
	handlers := alice.New(
		server.panicCatcher,
		corsHandler.Handler,
		server.rateLimit,
		server.blockNonCDNConnections,
		server.authenticate,
		server.initApiCall,
		server.autoCache,
	).Then

	// link api to router
	server.linkAPI()

	// link middleware to router
	r.Use(middleware.Middleware(otelServiceName))
	r.Use(handlers)

	return server, nil
}

// LinkWorkspaceAPI
//
//	Simple helper function to link the WorkspaceAPI to the main
//	mux router. We use this separate design to prevent contamination
//	of our main codebase with the AGPL-3 licensed Coder modifications.
//	This way, if we ever decide to separate from the Coder modifications
//	we have a clean break by removing the `coder/` directories and
//	repairing the broken links throughout the codebase.
func (s *HTTPServer) LinkWorkspaceAPI(api *wsApi.WorkspaceAPI) {
	// link error handler
	api.HandleError = s.handleError
	// link router and paths
	api.LinkAPI(s.router)
}

func (s *HTTPServer) Serve() error {
	// add catch all as the last step to ensure it doesn't
	// conflict with any valid routes
	s.router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	return s.server.Serve(*s.listener)
}

func (s *HTTPServer) Shutdown() error {
	return s.server.Shutdown(context.Background())
}

// GetBuffer
//
//	Retrieves a buffer from the buffer pool
func (s *HTTPServer) GetBuffer() *bytes.Buffer {
	b := s.memPool.Get()
	if b == nil {
		return &bytes.Buffer{}
	}
	return b.(*bytes.Buffer)
}

// PutBuffer
//
//	Returns a buffer to the buffer pool
func (s *HTTPServer) PutBuffer(b *bytes.Buffer) {
	b.Reset()
	s.memPool.Put(b)
}

// Handles HTTP errors by forming the error response and handles system logging
// Args:
//
//	w                 - http.ResponseWrite, response write that will be used to generate the response for the client
//	message           - string, message that will be logged as an error to the system logger
//	endpoint          - string, the endpoint that was being executed when the error occurred
//	method            - string, internal function that was being executed when the error occurred
//	methodType        - string, HTTP method type ex: [ GET, POST, PATCH, DELETE ]
//	ip                - string, IP address of the caller that executed the function that failed
//	statusCode        - int. status code that will be returned in the HTTP response
//	responseMessage   - string, message that will be returned in the HTTP response
//	err               - error, error that occurred during the function execution
func (s *HTTPServer) handleError(w http.ResponseWriter, message string, endpoint string, method string,
	methodType string, reqId interface{}, ip string, username string, userId string, statusCode int, responseMessage string, err error) {
	// log error internally
	s.logger.LogErrorExternalAPI(message, endpoint, method, methodType, reqId, ip, username, userId, statusCode, err)

	// add header content type
	w.Header().Set("Content-Type", "application/json")

	// write status code to response
	w.WriteHeader(statusCode)

	// attempt to serialize response message
	data, err := json.Marshal(map[string]interface{}{"message": responseMessage})
	if err != nil {
		// return empty response with just status code
		return
	}

	// write response message to response
	_, err = w.Write(data)
	if err != nil {
		// log response write error
		s.logger.Error(fmt.Sprintf("handleError: failed to write response message to HTTP response\n    Error: %v", err), map[string]interface{}{"function": "Web API handleError", "error": err})
	}
}

// panicCatcher
//
//	Middleware to catch panics and return a 500 error
func (s *HTTPServer) panicCatcher(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			// catch panic
			errI := recover()
			if errI == nil {
				// exit quietly if we didn't panic
				return
			}

			// attempt to load error as an error
			err, ok := errI.(error)
			if !ok {
				err = fmt.Errorf("%v", errI)
			}

			// handle error internally
			s.handleError(
				w,
				"panicCatcher: recovered from panic\nstacktrace:\n"+string(debug.Stack()),
				r.URL.Path,
				"panicCatcher",
				r.Method,
				r.Context().Value("reqId"),
				r.RemoteAddr,
				"",
				"",
				http.StatusInternalServerError,
				"Internal Server Error",
				err,
			)
		}()
		next.ServeHTTP(w, r)
	})
}

// Handles HTTP JSON response intricacies
// Formats a map into a response, generates the necessary headers, sets the status codes, and logs success
// Args:
//
//	w            - http.ResponseWriter, response write that will be used to generate the response for the client
//	res          - map[string]interface{}, map that will be formatted into a JSON for HTTP response
//	endpoint     - string, the endpoint that was being executed when the error occurred
//	method       - string, internal function that was being executed when the error occurred
//	methodType   - string, HTTP method type ex: [ GET, POST, PATCH, DELETE ]
//	ip           - string, IP address of the caller that executed the function that failed
//	statusCode   - int. status code that will be returned in the HTTP response
func (s *HTTPServer) jsonResponse(r *http.Request, w http.ResponseWriter, res map[string]interface{}, endpoint string, method string,
	methodType string, reqId interface{}, ip string, username string, userId string, statusCode int) {
	// add headers
	w.Header().Set("Content-Type", "application/json")

	// set status code
	w.WriteHeader(statusCode)

	// attempt to serialize response
	data, err := json.Marshal(res)
	if err != nil {
		// handle error internally
		s.handleError(w, "json serialization failed", endpoint, method, methodType, reqId,
			ip, username, userId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// write JSON response to HTTP response
	_, err = w.Write(data)
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", endpoint, method, methodType, reqId,
			ip, username, userId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// handle caching if it is enabled
	if cacheI := r.Context().Value(CtxKeyCache); cacheI != nil && statusCode > 199 && statusCode < 300 {
		// load the cache struct from the interface
		cache := cacheI.(*EndpointCache)

		// marshall the response so we can save it to redis
		cacheData, err := json.Marshal(CachedResponse{
			Body:   res,
			Status: statusCode,
		})
		if err != nil {
			// we can only log since we've already written to the response
			s.logger.Errorf("failed to marshall json response for caching: %v", err)
			return
		}

		// save the response to redis
		err = s.rdb.Set(r.Context(), r.Context().Value(CtxKeyCacheKey).(string), string(cacheData), cache.TTL).Err()
		if err != nil {
			// we can only log since we've already written to the response
			s.logger.Errorf("failed to save json response to redis: %v", err)
			return
		}
	}

	// handle cache invalidations
	for _, ep := range cacheEndpoints {
		// skip if the cache endpoint doesn't have an invalidation endpoint
		if ep.InvalidateOn == nil {
			continue
		}

		// check to see if the endpoint matches
		if ep.InvalidateOn.MatchString(r.URL.Path) {
			// get the cache key
			key := fmt.Sprintf("httpcache:%s", r.URL.Path)
			if ep.Method != "" {
				key = fmt.Sprintf("%s:%s", r.URL.Path, r.Method)
			}
			if ep.UserKey {
				key = fmt.Sprintf("%s:%s", key, userId)
			}

			// invalidate the cache
			err := s.rdb.Del(r.Context(), key).Err()
			if err != nil {
				// we can only log since we've already written to the response
				s.logger.Errorf("failed to invalidate cache: %v", err)
			}
		}
	}

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", endpoint, method, methodType, reqId, ip, username, userId, statusCode, nil)
}

// Loads HTTP JSON request into map for native GoLang usage
// NOTE: Returns an empty map if no JSON was passed AND the JSON was optional
// Args:
//
//		w          - http.ResponseWriter, response write that will be used to generate the response for the client
//		r          - *http.Request, incoming HTTP request object that the JSON body will be read from
//		method     - string, internal function that was being executed when this function was called
//		optional   - bool, whether the json request is an optional parameter (commonly used for functions with no
//	                    	parameters but need to pass the "test" flag)
//
// Returns:
//
//	out        - map[string]interface{}, map containing the HTTP JSON body; if nil then response has already been written
func (s *HTTPServer) jsonRequest(w http.ResponseWriter, r *http.Request, method string, optional bool, username string, userId int64) map[string]interface{} {
	// create map to load json bytes into
	var reqJson map[string]interface{}

	callingId := strconv.FormatInt(userId, 10)

	// parse JSON data from form object
	if r.Method != "GET" {
		// read body data
		body, err := io.ReadAll(r.Body)
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to read request body", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return nil
		}

		// handle empty json
		if len(body) == 0 {
			// handle optional case by returning empty map
			if optional {
				return make(map[string]interface{})
			}

			// handle required case by returning error message
			s.handleError(w, "no json body was sent", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusBadRequest, "missing json body data", nil)
			// exit
			return nil
		}

		// attempt to load byte data into map
		err = json.Unmarshal(body, &reqJson)
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to unmarshall request body\n    Body: "+string(body), r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return nil
		}
	} else {
		// assign empty map to request JSON
		reqJson = make(map[string]interface{})

		// extract query values from URL
		queryValues := r.URL.Query()

		// handle empty json
		if len(queryValues) == 0 {
			// handle optional case by returning empty map
			if optional {
				return make(map[string]interface{})
			}

			// handle required case by returning error message
			s.handleError(w, "no json data sent in query string", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusBadRequest, "missing json data in query string", nil)
			// exit
			return nil
		}

		// load query values into request JSON
		for k := range queryValues {
			reqJson[k] = queryValues.Get(k)
		}
	}

	return reqJson
}

// Loads a value from a HTTP JSON map
// Writes an error response for missing field if field is not present
// Allows for optional fields which will return nil with no error written if the are missing
// Args:
//
//	w              - http.ResponseWriter, response write that will be used to generate the response for the client
//	r              - *http.Request, incoming HTTP request object that will be used for call information
//	resJson        - map[string]interface{}, json map loaded from request body
//	method         - string, internal function that was being executed when this function was called
//	key            - string, key that will be used for load attempt
//	expectedKind   - reflect.Kind, reflect type for checking basic object types
//	sliceKind      - *reflect.Kind, optional reflect kind intended to be used for checking internal type of slices
//	optional       - bool, whether the field is optional; if true then no error message is written to response object
//
// Returns:
//
//	value          - interface{}, interface extracted from the JSON map passed;
//	                               the expected type has been validated making it safe for direct assignment
//	success        - bool, whether the operation was successful; if false an error has already been written to the response
func (s *HTTPServer) loadValue(w http.ResponseWriter, r *http.Request, reqJson map[string]interface{}, method string,
	key string, expectedKind reflect.Kind, sliceKind *reflect.Kind, optional bool, username string, userId string) (interface{}, bool) {
	// attempt to load value from body
	value, ok := reqJson[key]

	// return error if query is missing from request
	if !ok {
		// return nil with no error if key was optional
		if optional {
			return nil, true
		}

		// write error for missing key
		s.handleError(w, key+" missing from request body", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, userId, http.StatusUnprocessableEntity, fmt.Sprintf("missing %s field", key), nil)
		return nil, false
	}

	// return error if id is not of the specified kind
	if reflect.TypeOf(value).Kind() != expectedKind {
		s.handleError(w, fmt.Sprintf("failed to cast %s to %s", key, expectedKind), r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, userId, http.StatusUnprocessableEntity, "incorrect type passed for field "+key, nil)
		return nil, false
	}

	// validate further details of slice's type
	if sliceKind != nil && expectedKind == reflect.Slice && len(value.([]interface{})) > 0 {
		// return error if slice is not of the specified type
		if reflect.TypeOf(value.([]interface{})[0]).Kind() != *sliceKind {
			s.handleError(w, fmt.Sprintf("failed to cast %s to %s", key, expectedKind), r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, userId, http.StatusUnprocessableEntity, "incorrect type passed for field "+key, nil)
			return nil, false
		}
	}

	return value, true
}

// validateRequest
//
//	Loads a json request from the request body and validates it's schema.
func (s *HTTPServer) validateRequest(w http.ResponseWriter, r *http.Request, callingUser *models.User, buf io.Reader, value interface{}) bool {
	username := "anon"
	userId := "-1"
	if callingUser != nil {
		username = callingUser.UserName
		userId = fmt.Sprintf("%d", callingUser.ID)
	}

	// attempt to decode the request body
	err := json.NewDecoder(buf).Decode(value)
	if err != nil && err != io.EOF {
		s.handleError(w, "failed to decode request", r.URL.Path, "validateRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, userId,
			http.StatusInternalServerError, "internal server error", err)
		return false
	}

	// validate the schema
	err = s.validator.Struct(value)

	// handle known validation errors
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		message := ""
		for _, validationError := range validationErrors {
			if len(message) > 0 {
				message += ", "
			}
			message += fmt.Sprintf("Invalid field %q: value `%v` failed validation %s", validationError.Field(), validationError.Value(), validationError.Tag())
		}
		s.handleError(w, message, r.URL.Path, "validateRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, userId,
			http.StatusBadRequest, message, err)
		return false
	}

	// handle unexpected validation errors
	if err != nil {
		s.handleError(w, "validation failed", r.URL.Path, "validateRequest", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, userId,
			http.StatusInternalServerError, "internal server error", err)
		return false
	}

	return true
}

func (s *HTTPServer) validateOrigin(w http.ResponseWriter, r *http.Request) bool {
	// get the origin from the request
	origin := r.Header.Get("Origin")

	// check if the origin matches any of the allowed origins
	// we use regex to allow for wildcard matching
	for _, allowedOrigin := range s.allowedOrigins {
		// create regex for the allowed origin
		allowedOriginRegex := regexp.MustCompile(allowedOrigin)

		// check if the origin matches the allowed origin
		if allowedOriginRegex.MatchString(origin) {
			// we are in development mode and the origin is valid
			return true
		}
	}

	// write the error to the response
	s.handleError(w, "invalid origin", r.URL.Path, "validateOrigin", r.Method, r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "forbidden", nil)

	// we have an invalid origin
	return false
}

// Helper function used to remove the authentication token using the http response writer
func (s *HTTPServer) revokeCookie(w http.ResponseWriter, ip string) {
	// log cookie revocation
	s.logger.Warn(fmt.Sprintf("cookie revoked: %s", ip))

	// conditionally set cookie with insecure settings
	if s.developmentMode {
		// set cookie in response with an expired expiration
		http.SetCookie(w, &http.Cookie{
			Name:     "gigoAuthToken",
			Value:    "",
			Expires:  time.Unix(0, 0),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Domain:   fmt.Sprintf(".%s", s.domain),
		})
	} else {
		// set cookie in response with an expired expiration
		http.SetCookie(w, &http.Cookie{
			Name:     "gigoAuthToken",
			Value:    "",
			Expires:  time.Unix(0, 0),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   true,
			Domain:   fmt.Sprintf(".%s", s.domain),
		})
	}
}

// Middleware helper function used to block connections from IP ranges that are not whitelisted and did not access the service via a CDN
func (s *HTTPServer) blockNonCDNConnections(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// skip if we are not blocking non-cdn connections
		if !s.forceCdn {
			next.ServeHTTP(w, r)
			return
		}

		// check if we have a valid cdn key
		key := r.Header.Get(CdnAccessHeader)
		if key == s.cdnKey && s.cdnKey != "" {
			next.ServeHTTP(w, r)
			return
		}

		// if we have an invalid cdn key, log the error and reject the request
		if key != "" {
			s.handleError(w, "invalid cdn key", r.URL.Path, "blockNonCDNConnections", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "forbidden", nil)
			return
		}

		// get the ip address of the request
		ip := network.GetRequestIP(r)

		// parse the IP address
		parsedIP := net.ParseIP(ip)

		// if the IP address is nil, log the error and reject the request
		if parsedIP == nil {
			s.handleError(w, "invalid ip address", r.URL.Path, "blockNonCDNConnections", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "forbidden", nil)
			return
		}

		// check if the IP address is whitelisted
		for _, wl := range s.whitelistedIpRanges {
			if wl.Contains(parsedIP) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// if the IP address is not whitelisted, log the error and reject the request
		s.handleError(w, "ip address not whitelisted", r.URL.Path, "blockNonCDNConnections", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "n/a", "n/a", http.StatusForbidden, "forbidden", nil)
		return
	})
}

// Middleware helper function used to perform automatic caching of requests
func (s *HTTPServer) autoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check if we are caching this request
		var endpoint *EndpointCache
		for _, ep := range cacheEndpoints {
			if ep.Path.MatchString(r.URL.Path) && (ep.Method != "" || ep.Method == r.Method) {
				endpoint = ep
				break
			}
		}

		// if we are not caching handle the request and return
		if endpoint == nil {
			next.ServeHTTP(w, r)
			return
		}

		// get the user id
		callingUser := r.Context().Value(CtxKeyUser)
		username := "anon"
		userId := int64(-1)
		if callingUser != nil {
			username = callingUser.(*models.User).UserName
			userId = callingUser.(*models.User).ID
		}

		// get the cache key
		key := fmt.Sprintf("httpcache:%s", r.URL.Path)
		if endpoint.Method != "" {
			key = fmt.Sprintf("%s:%s", r.URL.Path, r.Method)
		}
		if endpoint.UserKey {
			key = fmt.Sprintf("%s:%d", key, userId)
		}
		// handle loading the cache keys from the body
		if len(endpoint.KeyFields) > 0 {
			// ensure there is a buffer
			if r.Context().Value(CtxKeyBodyBuffer) == nil {
				s.handleError(w, "missing request body buffer", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", nil)
				return
			}

			// get the body buffer
			body := r.Context().Value(CtxKeyBodyBuffer).(*bytes.Buffer).Bytes()

			// hash set to store the keys
			hashSet := ""

			// use jsonparser to pull the keys from the body
			for _, field := range endpoint.KeyFields {
				value, _, _, err := jsonparser.Get(body, field)
				if err != nil {
					s.handleError(w, "failed to parse request body", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
						network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", err)
					return
				}

				// hash the value and use the first 8 characters
				h, err := utils.HashData(value)
				if err != nil {
					s.handleError(w, "failed to hash request body", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
						network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", err)
					return
				}

				// append the hash to the hash set
				hashSet += h[:8]
			}

			// hash the hash set and use the first 8 characters
			h, err := utils.HashData([]byte(hashSet))
			if err != nil {
				s.handleError(w, "failed to hash request body", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", err)
				return
			}

			// append the hash to the key
			key = fmt.Sprintf("%s:%s", key, h[:8])
		}

		// check if the key is cached
		cached, err := s.rdb.Get(r.Context(), key).Result()
		if err != nil && err != redis.Nil {
			s.handleError(w, "failed to check cache", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", err)
			return
		}

		// if the key is cached, return the cached value
		if cached != "" {
			// load to a cached response
			var data CachedResponse
			err = json.Unmarshal([]byte(cached), &data)
			if err != nil {
				s.handleError(w, "failed to parse cached data", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", err)
				return
			}

			// ensure that the values are valid
			if data.Body == nil || data.Status == 0 {
				s.logger.Errorf("invalid cached response: %s", key)
				s.rdb.Del(r.Context(), key)
			} else {
				// write the json response
				s.jsonResponse(r, w, data.Body, r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					username, fmt.Sprintf("%d", userId), data.Status)

				// conditionally update the ttl if we update on hits
				if endpoint.RefreshOnHit {
					// update the ttl
					err = s.rdb.Expire(r.Context(), key, endpoint.TTL).Err()
					if err != nil {
						s.handleError(w, "failed to update cache ttl", r.URL.Path, "autoCache", r.Method, r.Context().Value(CtxKeyRequestID),
							network.GetRequestIP(r), username, fmt.Sprintf("%d", userId), http.StatusInternalServerError, "internal server error", err)
						return
					}
				}
				return
			}
		}

		// save the cache key and cache endpoint to the context to indicate we should cache the response
		ctx := context.WithValue(r.Context(), CtxKeyCache, endpoint)
		ctx = context.WithValue(ctx, CtxKeyCacheKey, key)
		r = r.WithContext(ctx)

		// execute the request
		next.ServeHTTP(w, r)

		return
	})
}

// Middleware helper function used to authenticate user sessions
func (s *HTTPServer) authenticateUserSession(ctx context.Context, w http.ResponseWriter, r *http.Request,
	token string, ip string) context.Context {
	// validate authentication token - we don't validate the IP for now
	valid, userID, payload, err := utils.ValidateExternalJWT(s.storageEngine, token, utils.SkipIpValidation, nil)
	if err != nil {
		// handle validation error
		s.handleError(w, "failed to validate authentication token", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", "n/a",
			http.StatusInternalServerError, "logout", err)
		return nil
	}

	callingId := strconv.FormatInt(userID, 10)

	// return if token is invalid
	if !valid {
		// handle validation error
		s.handleError(w, "authentication token invalid", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
			http.StatusForbidden, "logout", err)
		return nil
	}

	// return if userID is missing
	if userID == 0 {
		// handle validation error
		s.handleError(w, "user id missing after successful token authentication", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
			http.StatusInternalServerError, "logout", err)
		return nil
	}

	ctx, span := otel.Tracer("gigo-core").Start(r.Context(), "authenticate-user-session-http")
	defer span.End()
	callerName := "authenticate-user-session"
	// query for user in database
	res, err := s.tiDB.QueryContext(ctx, &span, &callerName, "select * from users where _id = ? limit 1", userID)
	if err != nil {
		// handle validation error
		s.handleError(w, "failed to query for user", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
			http.StatusInternalServerError, "logout", err)
		return nil
	}

	// defer closure of the cursor
	defer res.Close()

	// attempt to load the user into the first position of the cursor
	ok := res.Next()
	if !ok {
		// handle validation error
		s.handleError(w, "failed to find user in database", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
			http.StatusInternalServerError, "logout", nil)
		return nil
	}

	// attempt to decode user into object
	callingUser, err := models.UserFromSQLNative(s.tiDB, res)
	if err != nil {
		// handle validation error
		s.handleError(w, "failed to decode user object", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
			http.StatusInternalServerError, "internal server error occurred", err)
		return nil
	}

	// close response explicitly
	_ = res.Close()

	// conditionally handle user otp
	if callingUser != nil && callingUser.Otp != nil {
		// handle fully setup otp user
		if callingUser.OtpValidated != nil && *callingUser.OtpValidated && r.URL.Path != "/api/otp/validate" {
			// ensure that otp has been validated for this session
			if otpValid, ok := payload["otp_valid"]; !ok || !otpValid.(bool) {
				// handle validation error
				s.handleError(w, "otp has not been validated for this session", r.URL.Path, "authenticateUserSession",
					r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
					http.StatusForbidden, "logout", res.Err())
				return nil
			}
		} else {
			// handle a partial setup user by only permitting validate session and validate otp endpoints
			if r.URL.Path != "/api/auth/validate" && r.URL.Path != "/api/otp/validate" && r.URL.Path != "/api/otp/generateUserOtpUri" {

				// handle validation error
				s.handleError(w, "partial setup otp user attempted to access quarantined endpoint", r.URL.Path, "authenticateUserSession",
					r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
					http.StatusForbidden, "logout", res.Err())
				return nil
			}
		}
	}

	// handle github partial login
	if _, ok := payload["loginWithGithub"]; ok {
		// only permit access to the github login confirmation endpoint
		if r.URL.Path != "/api/auth/confirmLoginWithGithub" {
			// handle validation error
			s.handleError(w, "partial github login attempt to access protected endpoint", r.URL.Path,
				"authenticateUserSession", r.Method, int64(-1), network.GetRequestIP(r),
				"n/a", callingId, http.StatusForbidden, "forbidden", nil)
			return nil
		}

		// exit early with the calling user but no user session
		return context.WithValue(ctx, CtxKeyUser, callingUser)
	}

	// load session for user
	userSession, err := models.LoadUserSession(s.tiDB, s.rdb, callingUser.ID)
	if err != nil {
		if err.Error() == "no session" {
			// revoke cookie to clear the session if this is not a websocket, implicit action, or web tracking request
			// since all 3 of these api systems are managed in abnormal ways on the frontend we can't use them to trigger
			// a logout event
			if !strings.HasPrefix(r.URL.Path, "/api/ws") &&
				!strings.HasPrefix(r.URL.Path, "/api/implicit") &&
				!strings.HasPrefix(r.URL.Path, "/api/recordUsage") {
				s.revokeCookie(w, network.GetRequestIP(r))
			}

			// handle validation error
			s.handleError(w, "user not logged in", r.URL.Path, "authenticateUserSession",
				r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
				http.StatusUnauthorized, "logout", err)
			return nil
		}
		// handle validation error
		s.handleError(w, "failed to load seassion", r.URL.Path, "authenticateUserSession",
			r.Method, int64(-1), network.GetRequestIP(r), "n/a", callingId,
			http.StatusInternalServerError, "internal server error occurred", err)
		return nil
	}

	// create context to pass user object to end functions
	ctx = context.WithValue(ctx, CtxKeyUser, callingUser)
	return context.WithValue(ctx, "userSession", userSession)
}

// Middleware function to handle route security and user sessions for all http calls
func (s *HTTPServer) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get permission for route
		routePermission := DetermineRoutePermission(r.URL.Path)

		// create empty token by default
		token := ""

		// attempt to retrieve token cookie from request
		cookie, err := r.Cookie("gigoAuthToken")
		// attempt to load ephemeral user cookie
		if err != nil && err == http.ErrNoCookie {
			cookie, err = r.Cookie("gigoTempToken")
		}
		if err != nil {
			if err == http.ErrNoCookie && routePermission == RoutePermissionPrivate {
				// handle missing cookie error
				s.handleError(w, "auth cookie is not present", r.URL.Path, "authenticate", r.Method,
					int64(-1), network.GetRequestIP(r), "n/a", "n/a",
					http.StatusForbidden, "You must be logged in to access the GIGO system.", err)
				return
			} else if err != http.ErrNoCookie {
				// handle missing cookie error
				s.handleError(w, "failed to retrieve cookie", r.URL.Path, "authenticate", r.Method, int64(-1),
					network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError,
					"internal server error", err)
				return
			}
		} else if routePermission != RoutePermissionPublic {
			// retrieve auth token from cookie
			token = cookie.Value
		}

		// retrieve IP address of caller
		ip := network.GetRequestIP(r)

		// create context variable to hold user credential if
		// we are authenticating this session
		ctx := r.Context()

		// validate user session if there is a token
		if token != "" {
			ctx = s.authenticateUserSession(ctx, w, r, token, ip)
			if ctx == nil {
				return
			}
		}

		// execute end function with new context containing calling user
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *HTTPServer) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// retrieve IP address of caller
		ip := network.GetRequestIP(r)

		// check if rate limit is exceeded
		rateLimitRes, err := s.limiter.Allow(r.Context(), fmt.Sprintf("gigo-core-api-%s", ip), redis_rate.PerMinute(1000))
		if err != nil {
			// handle over use
			s.handleError(w, "failed to limit api call", r.URL.Path, "rateLimit", r.Method, int64(-1),
				network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError,
				"internal server error", err)
			return
		}

		// check if rate limit is exceeded
		if rateLimitRes.Remaining <= 0 {
			// handle over use
			s.handleError(w, fmt.Sprintf("too many requests: %v - %v - %v", rateLimitRes.RetryAfter, rateLimitRes.ResetAfter, rateLimitRes.Limit),
				r.URL.Path, "rateLimit", r.Method, int64(-1), network.GetRequestIP(r), "n/a",
				"n/a", http.StatusTooManyRequests, "too many requests", err)
			return
		}

		// execute end function with new context containing calling user
		next.ServeHTTP(w, r)
	})
}

// authenticateAgent
//
//	Middleware to authenticate a workspace agent
func (s *HTTPServer) authenticateAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// attempt to retrieve token
		token := r.Header.Get("Gigo-Agent-Token")
		if len(token) == 0 {
			s.handleError(w, "agent token missing", r.URL.Path, "authenticateAgent",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "anon",
				"-1", http.StatusUnauthorized, "agent token required", nil)
			return
		}

		// attempt to retrieve workspace id
		workspaceIDString := r.Header.Get("Gigo-Workspace-Id")
		if len(workspaceIDString) == 0 {
			s.handleError(w, "workspace id missing", r.URL.Path, "authenticateAgent",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "workspace id required", nil)
			return
		}

		// format workspace id to integer
		workspaceID, err := strconv.ParseInt(workspaceIDString, 10, 64)
		if err != nil {
			s.handleError(w, "invalid workspace id", r.URL.Path, "authenticateAgent",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "invalid workspace id", nil)
			return
		}

		// authenticate this call by using the workspace id and agent token
		// to query the database for the agent id and owner id
		var agentId int64
		var ownerId int64

		sctx, span := otel.Tracer("gigo-core").Start(r.Context(), "authenticate-agent-http")
		defer span.End()
		callerName := "authenticateAgent"
		err = s.tiDB.QueryRow(sctx, &span, &callerName,
			"select a._id, w.owner_id from workspaces w join workspace_agent a on a.workspace_id = w._id where w._id = ? and a.secret = uuid_to_bin(?) order by a.created_at desc limit 1",
			workspaceID, token,
		).Scan(&agentId, &ownerId)
		if err != nil {
			if err == sql.ErrNoRows {
				s.handleError(w, "agent not found", r.URL.Path, "authenticateAgent",
					r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					"anon", "-1", http.StatusUnauthorized, "agent not found", nil)
				return
			}
			s.handleError(w, "failed to authenticate agent", r.URL.Path,
				"authenticateAgent", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
				"anon", "-1", http.StatusUnauthorized, "failed to authenticate agent", err)
			return
		}

		// add workspace id, agent id and owner id to the context
		ctx := context.WithValue(r.Context(), "workspace", workspaceID)
		ctx = context.WithValue(ctx, "agent", agentId)
		ctx = context.WithValue(ctx, "owner", ownerId)

		// execute end function with new context containing workspace and agent ids
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Middleware function to handle validating the size of request bodies and initializing
// the api call
func (s *HTTPServer) initApiCall(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// generate a new request ID
		reqId := s.sf.Generate().Int64()

		// create context to pass request ID to end functions
		ctx := context.WithValue(r.Context(), CtxKeyRequestID, reqId)

		// derive trace span from context for telem
		span := trace.SpanFromContext(ctx)
		defer span.End()

		// link the api call with the request ID and true ip
		span.SetAttributes(attribute.Int64(CtxKeyRequestID, reqId))
		span.SetAttributes(attribute.String("ip", network.GetRequestIP(r)))

		// create default values for anonymous user
		userName := "anon"
		userId := ""

		// set user values if there is a calling user authenticated
		if ctx.Value(CtxKeyUser) != nil {
			// set username and id
			userName = ctx.Value(CtxKeyUser).(*models.User).UserName
			userId = fmt.Sprintf("%d", ctx.Value(CtxKeyUser).(*models.User).ID)

			// add user to trace
			span.SetAttributes(attribute.Bool("authenticated", true))
			span.SetAttributes(attribute.String("username", userName))
			span.SetAttributes(attribute.String("userId", userId))
		} else {
			// mark call as anonymous
			span.SetAttributes(attribute.Bool("authenticated", false))
		}

		// TODO: make sure this isn't going to cause problem with websockets

		// attempt to load body from request
		if r.Body != nil {
			// we will accept up to 10MiB in one request
			// if a request is larger we should reject it
			// to protect against giant requests intended
			// to crash the system or cause a DOS attack

			// copy the body into a buffer from the mempool with a maximum of 10MiB + 1B
			bodyBytes := s.GetBuffer()
			defer s.PutBuffer(bodyBytes)

			// read up to 10MiB + 1B from the body and if the
			// body contains more than 10MiB we'll reject the call
			_, err := io.Copy(bodyBytes, io.LimitReader(r.Body, 10*1024*1024+1))
			if err != nil {
				s.handleError(w, "failed to read body", r.URL.Path, "authenticate", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a",
					"n/a", http.StatusInternalServerError, "failed to read body", err)
				return
			}

			// check length or read to make sure we didn't
			// cross the 10MiB threshold
			if bodyBytes.Len() > (1024 * 1024 * 10) {
				s.handleError(w, "body too large", r.URL.Path, "authenticate", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a",
					"n/a", http.StatusRequestEntityTooLarge, "body too large - max 100MiB", err)
				return
			}

			// set the body to the buffer
			r.Body = io.NopCloser(bodyBytes)
			// assign the buffer to the context for direct access
			ctx = context.WithValue(r.Context(), CtxKeyBodyBuffer, bodyBytes)
		}

		// serialize headers
		headerBytes, err := json.Marshal(r.Header)
		if err != nil {
			s.handleError(w, "failed to serialize headers", r.URL.Path, "authenticate", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "n/a", "n/a", http.StatusInternalServerError, "internal server error", err)
			return
		}

		// skip logging on ping and healthcheck endpoints
		if r.Method != "GET" || (r.URL.Path != "/ping" && r.URL.Path != "/healthz") {
			s.logger.LogDebugExternalAPI(fmt.Sprintf("api call initiated: %q\n    Headers: %s",
				r.URL.Path, string(headerBytes)), r.URL.Path, "authenticate", r.Method, reqId,
				network.GetRequestIP(r), userName, userId, -1, nil)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Receives a chunked file upload and assembles the file chunks into a single temporary file
// The function returns nil if the execution requires no further action and returns the request JSON map if the
// core function logic should be executed
// Args:
//
//	w                - http.ResponseWriter, response write that will be used to generate the response for the client
//	r                - *http.Request, incoming HTTP request object that will be used for call information
//	method           - string, internal function that was being executed when this function was called
//	successMessage   - string, message that will be returned in the case of successful chunk upload
//
// Returns:
//
//	out              - map[string]interface{}, json map loaded from request body
func (s *HTTPServer) receiveUpload(w http.ResponseWriter, r *http.Request, method string, successMessage string, username string, userId int64) map[string]interface{} {
	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, method, false, username, userId)
	if reqJson == nil {
		return nil
	}

	callingId := strconv.FormatInt(userId, 10)

	// attempt to load upload id from body
	uploadId, ok := s.loadValue(w, r, reqJson, method, "upload_id", reflect.String, nil, false, username, callingId)
	if uploadId == nil || !ok {
		return nil
	}

	// attempt to load part number from body
	part, ok := s.loadValue(w, r, reqJson, method, "part", reflect.Float64, nil, false, username, callingId)
	if part == nil || !ok {
		return nil
	}

	// attempt to load total parts from body
	totalParts, ok := s.loadValue(w, r, reqJson, method, "total_parts", reflect.Float64, nil, false, username, callingId)
	if totalParts == nil || !ok {
		return nil
	}

	// attempt to load chunk from body
	encodedChunk, ok := s.loadValue(w, r, reqJson, method, "chunk", reflect.String, nil, false, username, callingId)
	if encodedChunk == nil || !ok {
		return nil
	}

	// create upload directory if uploading first part
	if part.(float64) == 1 {
		// create temporary directory for upload
		err := s.storageEngine.CreateDir("chunks/" + uploadId.(string))
		if err != nil {
			// handle error
			s.handleError(w, "failed to create temporary upload directory", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			return nil
		}

		// create a expiration time in 12 hours for when the temp directory will be removed if it still exists
		exp := fmt.Sprintf("%d", time.Now().Add(time.Hour*12).Unix())

		// insert expiration file inside directory to ensure timely removal in the case of a later failure
		err = s.storageEngine.CreateFile("chunks/"+uploadId.(string)+"/exp", []byte(exp))
		if err != nil {
			// handle error
			s.handleError(w, "failed to create expiration file for upload directory", r.URL.Path, method,
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), username, callingId,
				http.StatusInternalServerError, "internal server error occurred", err)
			return nil
		}
	}

	// base64 decode file chunk
	fileChunk, err := base64.StdEncoding.DecodeString(encodedChunk.(string))
	if err != nil {
		// handle error
		s.handleError(w, "failed to base64 decode file chunk", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return nil
	}

	// write file chunk to disk
	if part.(float64) < totalParts.(float64)+1 {
		// create part file path
		partPath := fmt.Sprintf("chunks/%s/%d", uploadId.(string), int(part.(float64)))

		// create part file
		err = s.storageEngine.CreateFile(partPath, fileChunk)
		if err != nil {
			// handle error
			s.handleError(w, "failed to write part file", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
			return nil
		}

		// write success message and exit if this is not the last part
		if part.(float64) < totalParts.(float64) {
			s.jsonResponse(r, w, map[string]interface{}{"message": successMessage}, r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), username, callingId, http.StatusOK)
			return nil
		}
	}

	// create slice to hold the paths to each file part for merge
	partPaths := make([]string, 0)

	// loop over parts assembling the paths to each part and saving them to the parts slice in order
	for i := 1; i < int(totalParts.(float64))+1; i++ {
		// assemble part file path and append to outer slice
		partPaths = append(partPaths, fmt.Sprintf("chunks/%s/%d", uploadId.(string), i))
	}

	// merge file parts into a single temporary file
	// NOTE: this uses the smallFiles param which is only safe in certain cases
	//       for developers unfamiliar with this param please review the documentation
	//       of each implementation of this function to better understand the
	//       implications of this params use
	err = s.storageEngine.MergeFiles("temp/"+uploadId.(string), partPaths, true)
	if err != nil {
		// handle error
		s.handleError(w, "failed to merge part files", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return nil
	}

	// remove upload directory
	err = s.storageEngine.DeleteDir("chunks/"+uploadId.(string), true)
	if err != nil {
		// handle error
		s.handleError(w, "failed to remove upload directory", r.URL.Path, method, r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), username, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return nil
	}

	// return json request
	return reqJson
}

// Ping function to enable the load balancer to confirm the system is alive
func (s *HTTPServer) ping(w http.ResponseWriter, r *http.Request) {
	// add headers
	w.Header().Set("Content-Type", "application/json")

	// set status code
	w.WriteHeader(200)

	// write JSON response to HTTP response
	_, err := w.Write([]byte(`{"status":"running"}`))
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "ping", r.Method, r.Context().Value(CtxKeyRequestID),
			"n/a", "n/a", network.GetRequestIP(r), http.StatusInternalServerError, "internal server error occurred", err)
		return
	}
}

// Health function to enable the load balancer to confirm the system is alive
func (s *HTTPServer) healthz(w http.ResponseWriter, r *http.Request) {
	// add headers
	w.Header().Set("Content-Type", "application/json")

	// save the status
	status := "ok"
	statusCode := 200

	// test database connection
	dbCtx, dbCancel := context.WithTimeout(context.Background(), time.Millisecond*300)
	defer dbCancel()
	err := s.tiDB.PingContext(dbCtx)
	if err != nil {
		status = "not ok"
		statusCode = 500
	}

	// set status code
	w.WriteHeader(statusCode)

	// write JSON response to HTTP response
	_, err = w.Write([]byte(fmt.Sprintf(`{"health":"%s"}`, status)))
	if err != nil {
		// handle error internally
		s.handleError(w, "write to response body failed", r.URL.Path, "healthz", r.Method, r.Context().Value(CtxKeyRequestID),
			"n/a", "n/a", network.GetRequestIP(r), http.StatusInternalServerError, "internal server error occurred", err)
		return
	}
}

// Links the HTTP Handler functions to the MUX router for execution via HTTP Requests at the designated endpoints
func (s *HTTPServer) linkAPI() {
	// /////////////////////////////////////////// CORS
	// create universal function for OPTIONS pre-flight calls
	// this will enable the server's CORS configuration to handle the header response
	s.router.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// /////////////////////////////////////////// Root
	s.router.HandleFunc("/ping", s.ping).Methods("GET")
	s.router.HandleFunc("/healthz", s.healthz).Methods("GET")

	// /////////////////////////////////////////// Debug
	// TODO: DO NOT LET USERS ACCESS THESE!!!
	// we can block with the load balancer or require admin auth
	// s.router.HandleFunc("/debug/pprof", pprof.Index).Methods("GET")
	// s.router.Handle("/debug/pprof/allocs", pprof.Handler("allocs")).Methods("GET")
	// s.router.Handle("/debug/pprof/block", pprof.Handler("block")).Methods("GET")
	// s.router.Handle("/debug/pprof/cmdline", pprof.Handler("cmdline")).Methods("GET")
	// s.router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine")).Methods("GET")
	// s.router.Handle("/debug/pprof/heap", pprof.Handler("heap")).Methods("GET")
	// s.router.Handle("/debug/pprof/mutex", pprof.Handler("mutex")).Methods("GET")
	// s.router.Handle("/debug/pprof/profile", pprof.Handler("profile")).Methods("GET")
	// s.router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate")).Methods("GET")
	// s.router.Handle("/debug/pprof/trace", pprof.Handler("trace")).Methods("GET")

	// ////////////////// Auth
	s.router.HandleFunc("/api/auth/login", s.Login).Methods("POST")
	s.router.HandleFunc("/api/auth/logout", s.Logout).Methods("POST")
	s.router.HandleFunc("/api/auth/validate", s.ValidateSession).Methods("GET")

	// ///////////////// OTP Auth
	s.router.HandleFunc("/api/otp/generateUserOtpUri", s.GenerateUserOtpUri).Methods("POST")
	s.router.HandleFunc("/api/otp/validate", s.VerifyUserOtp).Methods("POST")

	// /////////////////////////////////////////// Root
	s.router.HandleFunc("/api/ping", s.ping).Methods("GET")
	s.router.HandleFunc("/sitemap/sitemap.xml", s.GetSitemap).Methods("GET")
	s.router.HandleFunc("/api/ws", s.MasterWebSocket).Methods("GET")
	s.router.HandleFunc("/api/home/active", s.ActiveProjectsHome).Methods("POST")
	s.router.HandleFunc("/api/home/recommended", s.RecommendedProjectsHome).Methods("POST")
	s.router.HandleFunc("/api/home/following", s.RecommendedProjectsHome).Methods("POST")
	s.router.HandleFunc("/api/home/top", s.TopRecommendations).Methods("POST")
	s.router.HandleFunc("/api/following/feed", s.FeedPage).Methods("POST")
	s.router.HandleFunc("/api/active/pastWeek", s.PastWeekActive).Methods("POST")
	s.router.HandleFunc("/api/active/challenging", s.MostChallengingActive).Methods("POST")
	s.router.HandleFunc("/api/active/dontGiveUp", s.DontGiveUpActive).Methods("POST")
	s.router.HandleFunc("/api/project/get", s.ProjectInformation).Methods("POST")
	s.router.HandleFunc("/api/project/attempts", s.ProjectAttempts).Methods("POST")
	s.router.HandleFunc("/api/project/create", s.CreateProject).Methods("POST")
	s.router.HandleFunc("/api/project/delete", s.DeleteProject).Methods("POST")
	s.router.HandleFunc("/api/project/publish", s.PublishProject).Methods("POST")
	s.router.HandleFunc("/api/recommendation/top", s.TopRecommendation).Methods("POST")
	s.router.HandleFunc("/api/attempt/get", s.AttemptInformation).Methods("POST")
	s.router.HandleFunc("/api/attempt/getProject", s.ProjectAttemptInformation).Methods("POST")
	s.router.HandleFunc("/api/attempt/code", s.GetAttemptCode).Methods("POST")
	s.router.HandleFunc("/api/attempt/closeAttempt", s.CloseAttempt).Methods("POST")
	s.router.HandleFunc("/api/attempt/markSuccess", s.MarkSuccess).Methods("POST")
	s.router.HandleFunc("/api/recommendation/attempt", s.RecommendByAttempt).Methods("POST")
	s.router.HandleFunc("/api/recommendation/harder", s.HarderRecommendation).Methods("POST")
	s.router.HandleFunc("/api/discussion/getDiscussions", s.GetDiscussions).Methods("POST")
	s.router.HandleFunc("/api/discussion/getComments", s.GetDiscussionComments).Methods("POST")
	s.router.HandleFunc("/api/discussion/getThreads", s.GetCommentThreads).Methods("POST")
	s.router.HandleFunc("/api/discussion/getThreadReply", s.GetThreadReply).Methods("POST")
	s.router.HandleFunc("/api/discussion/createDiscussion", s.CreateDiscussion).Methods("POST")
	s.router.HandleFunc("/api/discussion/createComment", s.CreateComment).Methods("POST")
	s.router.HandleFunc("/api/discussion/createThreadComment", s.CreateThreadComment).Methods("POST")
	s.router.HandleFunc("/api/discussion/createThreadReply", s.CreateThreadReply).Methods("POST")
	s.router.HandleFunc("/api/discussion/editDiscussions", s.EditDiscussions).Methods("POST")
	s.router.HandleFunc("/api/discussion/addCoffee", s.AddDiscussionCoffee).Methods("POST")
	s.router.HandleFunc("/api/discussion/removeCoffee", s.RemoveDiscussionCoffee).Methods("POST")
	s.router.HandleFunc("/api/user/changeEmail", s.ChangeEmail).Methods("POST")
	s.router.HandleFunc("/api/user/changeUsername", s.ChangeUsername).Methods("POST")
	s.router.HandleFunc("/api/user/changePhone", s.ChangePhoneNumber).Methods("POST")
	s.router.HandleFunc("/api/user/userProjects", s.UserProjects).Methods("POST")
	s.router.HandleFunc("/api/user/changeUserPicture", s.ChangeUserPicture).Methods("POST")
	s.router.HandleFunc("/api/user/changePassword", s.ChangePassword).Methods("POST")
	s.router.HandleFunc("/api/user/deleteUserAccount", s.DeleteUserAccount).Methods("POST")
	s.router.HandleFunc("/api/user/subscription", s.GetSubscription).Methods("POST")
	s.router.HandleFunc("/api/user/follow", s.FollowUser).Methods("POST")
	s.router.HandleFunc("/api/user/unfollow", s.UnFollowUser).Methods("POST")
	s.router.HandleFunc("/api/project/getProjectCode", s.GetProjectCode).Methods("POST")
	s.router.HandleFunc("/api/project/getProjectFiles", s.GetProjectFile).Methods("POST")
	s.router.HandleFunc("/api/project/getProjectDirectories", s.GetProjectDirectories).Methods("POST")
	s.router.HandleFunc("/api/project/config", s.GetConfig).Methods("POST")
	s.router.HandleFunc("/api/project/editConfig", s.EditConfig).Methods("POST")
	s.router.HandleFunc("/api/project/confirmEditConfig", s.ConfirmEditConfig).Methods("POST")
	s.router.HandleFunc("/api/public_config/create", s.CreatePublicConfigTemplate).Methods("POST")
	s.router.HandleFunc("/api/public_config/edit", s.EditPublicConfigTemplate).Methods("POST")
	s.router.HandleFunc("/api/project/genImage", s.GenerateProjectImage).Methods("POST")
	s.router.HandleFunc("/api/user/createNewUser", s.CreateNewUser).Methods("POST")
	s.router.HandleFunc("/api/user/validateUser", s.ValidateUserInfo).Methods("POST")
	s.router.HandleFunc("/api/user/profilePage", s.UserProfilePage).Methods("POST")
	s.router.HandleFunc("/api/user/createNewGoogleUser", s.CreateNewGoogleUser).Methods("POST")
	s.router.HandleFunc("/api/user/streakPage", s.GetUserStreaks).Methods("POST")
	s.router.HandleFunc("/api/user/markTutorial", s.MarkTutorialAsCompleted).Methods("POST")
	s.router.HandleFunc("/api/auth/loginWithGoogle", s.LoginWithGoogle).Methods("POST")
	s.router.HandleFunc("/api/user/createNewGithubUser", s.CreateNewGithubUser).Methods("POST")
	s.router.HandleFunc("/api/auth/loginWithGithub", s.LoginWithGithub).Methods("POST")
	s.router.HandleFunc("/api/auth/referralUserInfo", s.ReferralUserInfo).Methods("POST")
	s.router.HandleFunc("/api/auth/confirmLoginWithGithub", s.ConfirmGithubLogin).Methods("POST")
	s.router.HandleFunc("/api/user/resetForgotPassword", s.ResetForgotPassword).Methods("POST")
	s.router.HandleFunc("/api/user/forgotPasswordValidation", s.ForgotPasswordValidation).Methods("POST")
	s.router.HandleFunc("/api/ephemeral/create", s.CreateEphemeral).Methods("POST")
	s.router.HandleFunc("/api/project/shareLink", s.ShareLink).Methods("POST")
	s.router.HandleFunc("/api/project/verifyLink", s.VerifyLink).Methods("POST")
	s.router.HandleFunc("/api/project/editProject", s.EditProject).Methods("POST")
	s.router.HandleFunc("/api/project/editAttempt", s.EditAttempt).Methods("POST")

	// Ephemeral
	s.router.HandleFunc("/api/ephemeral/createAccount", s.CreateAccountFromEphemeral).Methods("POST")
	s.router.HandleFunc("/api/ephemeral/createAccountGoogle", s.CreateAccountFromEphemeralGoogle).Methods("POST")
	s.router.HandleFunc("/api/ephemeral/createAccountGithub", s.CreateAccountFromEphemeralGithub).Methods("POST")
	s.router.HandleFunc("/api/verifyRecaptcha", s.VerifyCaptcha).Methods("POST")

	s.router.HandleFunc("/api/workspace/create", s.CreateWorkspace).Methods("POST")
	s.router.HandleFunc("/api/workspace/status", s.GetWorkspaceStatus).Methods("POST")
	s.router.HandleFunc("/api/search/users", s.SearchUsers).Methods("POST")
	s.router.HandleFunc("/api/search/tags", s.SearchTags).Methods("POST")
	s.router.HandleFunc("/api/search/discussions", s.SearchDiscussions).Methods("POST")
	s.router.HandleFunc("/api/search/comment", s.SearchComments).Methods("POST")
	s.router.HandleFunc("/api/search/posts", s.SearchPosts).Methods("POST")
	s.router.HandleFunc("/api/search/complete", s.CompleteSearch).Methods("POST")
	s.router.HandleFunc("/api/search/simplePost", s.SimpleSearchPosts).Methods("POST")
	s.router.HandleFunc("/api/search/workspaceConfigs", s.SearchWorkspaceConfigs).Methods("POST")
	s.router.HandleFunc("/api/search/friends", s.SearchFriends).Methods("POST")
	s.router.HandleFunc("/api/search/chatUsers", s.SearchChatUsers).Methods("POST")
	s.router.HandleFunc("/api/popular", s.PopularPageFeed).Methods("POST")
	s.router.HandleFunc("/api/workspace/config/create", s.CreateWorkspaceConfig).Methods("POST")
	s.router.HandleFunc("/api/workspace/config/update", s.UpdateWorkspaceConfig).Methods("POST")
	s.router.HandleFunc("/api/workspace/config/get", s.GetUserWorkspaceSettings).Methods("POST")
	s.router.HandleFunc("/api/workspace/config/getWsConfig", s.GetWorkspaceConfig).Methods("POST")
	s.router.HandleFunc("/api/editDescription", s.EditDescription).Methods("POST")
	s.router.HandleFunc("/api/attempt/start", s.StartAttempt).Methods("POST")
	s.router.HandleFunc("/api/project/closedAttempts", s.GetClosedAttempts).Methods("POST")
	s.router.HandleFunc("/api/user/get", s.GetUserInformation).Methods("POST")
	s.router.HandleFunc("/api/user/getId", s.GetUserID).Methods("POST")
	s.router.HandleFunc("/api/user/updateAvatar", s.UpdateAvatarSettings).Methods("POST")
	s.router.HandleFunc("/api/user/updateWorkspace", s.SetUserWorkspaceSettings).Methods("POST")
	s.router.HandleFunc("/api/user/updateExclusiveAgreement", s.UpdateUserExclusiveAgreement).Methods("POST")
	s.router.HandleFunc("/api/user/updateHolidayPreference", s.UpdateHolidayPreference).Methods("POST")
	s.router.HandleFunc("/api/nemesis/declare", s.DeclareNemesis).Methods("POST")
	s.router.HandleFunc("/api/nemesis/accept", s.AcceptNemesis).Methods("POST")
	s.router.HandleFunc("/api/nemesis/decline", s.DeclineNemesis).Methods("POST")
	s.router.HandleFunc("/api/nemesis/active", s.GetActiveNemesis).Methods("POST")
	// s.router.HandleFunc("/api/nemesis/pending", s.GetPendingNemesisRequests).Methods("POST")
	s.router.HandleFunc("/api/nemesis/battleground", s.GetNemesisBattlegrounds).Methods("POST")
	s.router.HandleFunc("/api/nemesis/recent", s.RecentNemesisBattleground).Methods("POST")
	s.router.HandleFunc("/api/nemesis/history", s.WarHistory).Methods("POST")
	s.router.HandleFunc("/api/nemesis/pending", s.PendingNemesis).Methods("POST")
	s.router.HandleFunc("/api/nemesis/victory", s.DeclareVictor).Methods("POST")
	s.router.HandleFunc("/api/nemesis/allUsers", s.GetAllUsers).Methods("POST")
	s.router.HandleFunc("/api/nemesis/dailyXP", s.GetDailyXPGain).Methods("POST")
	s.router.HandleFunc("/api/friends/request", s.SendFriendRequest).Methods("POST")
	s.router.HandleFunc("/api/friends/accept", s.AcceptFriendRequest).Methods("POST")
	s.router.HandleFunc("/api/friends/decline", s.DeclineFriendRequest).Methods("POST")
	s.router.HandleFunc("/api/friends/list", s.GetFriendsList).Methods("POST")
	s.router.HandleFunc("/api/friends/requestList", s.GetFriendRequests).Methods("POST")
	s.router.HandleFunc("/api/friends/requestCheck", s.CheckFriendRequest).Methods("POST")
	s.router.HandleFunc("/api/friends/check", s.CheckFriend).Methods("POST")
	s.router.HandleFunc("/api/implicit/recordAction", s.RecordImplicitAction).Methods("POST")
	s.router.HandleFunc("/api/reward/getUserRewardInventory", s.GetUserRewardsInventory).Methods("POST")
	s.router.HandleFunc("/api/reward/setUserReward", s.SetUserReward).Methods("POST")
	s.router.HandleFunc("/api/workspace/startWorkspace", s.StartWorkspace).Methods("POST")
	s.router.HandleFunc("/api/workspace/stopWorkspace", s.StopWorkspace).Methods("POST")
	s.router.HandleFunc("/api/workspace/getHighestScore", s.GetHighestScore).Methods("POST")
	s.router.HandleFunc("/api/workspace/setHighestScore", s.SetHighestScore).Methods("POST")
	s.router.HandleFunc("/api/xp/getXPBoost", s.GetXPBoostCount).Methods("POST")
	s.router.HandleFunc("/api/xp/getXP", s.GetXP).Methods("POST")
	s.router.HandleFunc("/api/xp/startXPBoost", s.StartXPBoost).Methods("POST")
	s.router.HandleFunc("/api/streakFreeze/get", s.GetStreakFreezeCount).Methods("POST")
	// s.router.HandleFunc("/api/workspace/webSocket", s.WorkspaceWebSocket).Methods("GET")
	s.router.HandleFunc("/api/broadcast/message", s.BroadcastMessage).Methods("POST")
	s.router.HandleFunc("/api/broadcast/get", s.GetBroadcastMessages).Methods("POST")
	s.router.HandleFunc("/api/broadcast/check", s.CheckBroadcastAward).Methods("POST")
	s.router.HandleFunc("/api/broadcast/revert", s.RevertBroadcastAward).Methods("POST")
	s.router.HandleFunc("/api/curated/add", s.AddPostToCurated).Methods("POST")
	s.router.HandleFunc("/api/curated/remove", s.RemoveCuratedPost).Methods("POST")
	s.router.HandleFunc("/api/curated/getPostsAdmin", s.GetCuratedPostsForAdmin).Methods("POST")
	s.router.HandleFunc("/api/curated/auth", s.CurationAuth).Methods("POST")
	s.router.HandleFunc("/api/email/verify", s.EmailVerification).Methods("POST")
	s.router.HandleFunc("/api/unsubscribe/check", s.CheckUnsubscribeEmail).Methods("POST")
	s.router.HandleFunc("/api/unsubscribe/modify", s.UpdateEmailPreferences).Methods("POST")
	s.router.HandleFunc("/api/notification/acknowledge", s.AcknowledgeNotification).Methods("POST")
	s.router.HandleFunc("/api/notification/acknowledgeGroup", s.AcknowledgeUserNotificationGroup).Methods("POST")
	s.router.HandleFunc("/api/notification/clearAll", s.ClearUserNotifications).Methods("POST")
	s.router.HandleFunc("/api/notification/get", s.GetUserNotifications).Methods("POST")
	s.router.HandleFunc("/static/posts/t/{id:[0-9]+}", s.SiteImages).Methods("GET")
	s.router.HandleFunc("/static/attempts/t/{id:[0-9]+}", s.SiteImages).Methods("GET")
	s.router.HandleFunc("/static/user/pfp/{id:.+}", s.SiteImages).Methods("GET")
	s.router.PathPrefix("/static/git/p/{id:[0-9]+}").HandlerFunc(s.GitImages).Methods("GET")
	s.router.PathPrefix("/static/git/a/{id:[0-9]+}").HandlerFunc(s.GitImages).Methods("GET")
	s.router.HandleFunc("/api/project/tempGenImage/{id:[0-9]+}", s.GetGeneratedImage).Methods("GET")
	s.router.PathPrefix("/static/ext").HandlerFunc(s.ExtensionFiles).Methods("GET")
	s.router.PathPrefix("/static/ui").HandlerFunc(s.UiFiles).Methods("GET")
	s.router.PathPrefix("/api/broadcast/ws/{id:[0-9]+}").HandlerFunc(s.BroadcastWebSocket).Methods("GET")
	s.router.HandleFunc("/api/verifyResetToken/{token}/{userId}", s.VerifyEmailToken).Methods("GET")
	s.router.HandleFunc("/api/reportIssue", s.CreateReportIssue).Methods("POST")
	s.router.HandleFunc("/api/recordUsage", s.RecordWebUsage).Methods("POST")
	// s.router.PathPrefix("/api/websocket/ws/{id:[0-9]+}").HandlerFunc(s.WebSocketMaster).Methods("GET")

	// ////////////////////////////////////Stripe
	s.router.HandleFunc("/api/stripe/createProduct", s.CreateProduct).Methods("POST")
	s.router.HandleFunc("/api/stripe/getPriceId", s.GetProjectPriceId).Methods("POST")
	s.router.HandleFunc("/api/stripe/cancelSubscription", s.CancelSubscription).Methods("POST")
	s.router.HandleFunc("/api/stripe/webhook", s.HandleStripeWebhook).Methods("POST")
	s.router.HandleFunc("/api/stripe/connected/webhook", s.HandleStripeConnectedWebhook).Methods("POST")
	s.router.HandleFunc("/api/stripe/updatePayment", s.UpdateClientPayment).Methods("POST")
	s.router.HandleFunc("/api/stripe/updateConnectedAccount", s.UpdateConnectedAccount).Methods("POST")
	s.router.HandleFunc("/api/stripe/portalSession", s.CreatePortalSession).Methods("POST")
	s.router.HandleFunc("/api/stripe/createConnectedAccount", s.CreateConnectedAccount).Methods("POST")
	s.router.HandleFunc("/api/stripe/stripeCheckoutSession", s.StripeCheckoutSession).Methods("POST")
	s.router.HandleFunc("/api/stripe/premiumMembershipSession", s.StripePremiumMembershipSession).Methods("POST")

	// /////////////////////////////////////////// Internal
	internalWsRouter := s.router.PathPrefix("/internal/v1/ws").Subrouter()
	internalWsRouter.Use(s.authenticateAgent)
	internalWsRouter.HandleFunc("/init-state", s.WorkspaceInitializationStepCompleted).Methods("POST")
	internalWsRouter.HandleFunc("/init-failure", s.WorkspaceInitializationFailure).Methods("POST")
	internalWsRouter.HandleFunc("/ext", s.WorkspaceGetExtension).Methods("GET")
	internalWsRouter.HandleFunc("/ext/ct", s.WorkspaceGetCtExtension).Methods("GET")
	internalWsRouter.HandleFunc("/ext/theme", s.WorkspaceGetThemeExtension).Methods("GET")
	internalWsRouter.HandleFunc("/ext/holiday-theme", s.WorkspaceGetHolidayThemeExtension).Methods("GET")
	internalWsRouter.HandleFunc("/ext/open-vsx-cache", s.OpenVsxPullThroughCache).Methods("GET")
	internalWsRouter.HandleFunc("/code-server", s.CodeServerPullThroughCache).Methods("GET")
	internalWsRouter.HandleFunc("/agent", s.WorkspaceGetAgent)

	internalExtRouter := s.router.PathPrefix("/internal/v1/ext").Subrouter()
	internalExtRouter.HandleFunc("/live-check", s.ExtendExpiration).Methods("POST")
	internalExtRouter.HandleFunc(
		"/streak-check/{id:[0-9]+}/{secret:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}",
		s.StreakHandlerExt,
	).Methods("GET")
	// internalExtRouter.HandleFunc("/streak-check", s.StreakCheck).Methods("POST")
	internalExtRouter.HandleFunc("/afk", s.WorkspaceAFK).Methods("POST")

	s.router.HandleFunc("/internal/git/push-hook", s.GiteaWebhookPush).Methods("POST")
}
