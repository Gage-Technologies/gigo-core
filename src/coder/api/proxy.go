package api

import (
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gigo-core/coder/api/core"

	"github.com/gage-technologies/gigo-lib/coder/agentsdk"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gage-technologies/gigo-lib/zitimesh"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// This needs to be a super unique query parameter because we don't want to
	// conflict with query parameters that users may use.
	//nolint:gosec
	subdomainProxyAPIKeyParam = "coder_application_connect_api_key_35e783"
	// redirectURIQueryParam is the query param for the app URL to be passed
	// back to the API auth endpoint on the main access URL.
	redirectURIQueryParam = "redirect_uri"
	// appLogoutHostname is the hostname to use for the logout redirect. When
	// the dashboard logs out, it will redirect to this subdomain of the app
	// hostname, and the server will remove the cookie and redirect to the main
	// login page.
	// It is important that this URL can never match a valid app hostname.
	appLogoutHostname = "coder-logout"
)

//go:embed static
var StatidFS embed.FS

// nonCanonicalHeaders is a map from "canonical" headers to the actual header we
// should send to the app in the workspace. Some headers (such as the websocket
// upgrade headers from RFC 6455) are not canonical according to the HTTP/1
// spec. Golang has said that they will not add custom cases for these headers,
// so we need to do it ourselves.
//
// Some apps our customers use are sensitive to the case of these headers.
//
// https://github.com/golang/go/issues/18495
var nonCanonicalHeaders = map[string]string{
	"Sec-Websocket-Accept":     "Sec-WebSocket-Accept",
	"Sec-Websocket-Extensions": "Sec-WebSocket-Extensions",
	"Sec-Websocket-Key":        "Sec-WebSocket-Key",
	"Sec-Websocket-Protocol":   "Sec-WebSocket-Protocol",
	"Sec-Websocket-Version":    "Sec-WebSocket-Version",
}

// regex expression that will be used to trim the base path
// of the editor and port proxy paths so that we only use the portion
// of the that is intended to be forwarded to the proxied endpoint
var EditorPathCleaner = regexp.MustCompile("\\/editor\\/[0-9]+\\/[0-9]+-[^\\/]{1,64}")
var DesktopPathCleaner = regexp.MustCompile("\\/desktop\\/[0-9]+\\/[0-9]+-[^\\/]{1,64}")
var AgentPathCleaner = regexp.MustCompile("\\/agent\\/[0-9]+\\/[0-9]+")

// proxyWorkspacePortOptions are the required fields to proxy a workspace port.
type proxyWorkspacePortOptions struct {
	CallingUser *models.User
	AgentID     int64
	Port        uint16
	SSL         bool
}

// WorkspaceEditorProxy
//
//	Proxies a web call to the workspace editor
func (api *WorkspaceAPI) WorkspaceEditorProxy(rw http.ResponseWriter, r *http.Request) {
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-editor-proxy-http")
	defer parentSpan.End()

	// this should never happen but let's handle it just incase
	if callingUser == nil {
		api.HandleError(rw, fmt.Sprintf("calling user is nil"), r.URL.Path, "WorkspaceEditorProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "-1",
			http.StatusBadRequest, "logout", nil)
		return
	}

	// format user id to string
	callingId := fmt.Sprintf("%d", callingUser.(*models.User).ID)
	// extract username
	callingUserName := callingUser.(*models.User).UserName

	// retrieve params from url
	vars := mux.Vars(r)

	// attempt to retrieve user from params map
	userID, ok := vars["user"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no user id found in path", r.URL.Path, "WorkspaceEditorProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// ensure that calling user and user id are the same
	if callingId != userID {
		api.HandleError(rw, fmt.Sprintf("userID mismatch: %s != %s", callingId, userID), r.URL.Path, "WorkspaceEditorProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// load workspace id from url
	workspaceIDString, ok := vars["workspace"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no workspace id found in path", r.URL.Path, "WorkspaceEditorProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// parse workspaceIDString into an int as a workspace id
	workspaceID, err := strconv.ParseInt(workspaceIDString, 10, 64)
	if err != nil {
		api.HandleError(rw, fmt.Sprintf("invalid workspace id: %s", workspaceIDString), r.URL.Path, "WorkspaceEditorProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, "invalid url", nil)
		return
	}

	// ensure that if we have an empty path we redirect to root
	if EditorPathCleaner.ReplaceAllString(r.URL.Path, "") == "" {
		api.Logger.LogDebugExternalAPI(
			"function execution successful - redirecting to root",
			r.URL.Path,
			"WorkspaceEditorProxy",
			r.Method,
			r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r),
			callingUserName,
			callingId,
			http.StatusOK,
			nil,
		)
		http.Redirect(rw, r, r.URL.Path+"/", http.StatusTemporaryRedirect)
		return
	}

	// trim base path for the editor proxy to retrieve only the
	// portion of the path that should be forwarded to the editor
	internalPath := EditorPathCleaner.ReplaceAllString(r.URL.Path, "")

	// handle icon injection
	iconInjected := api.handleIconInjection(internalPath, rw, r)
	if iconInjected {
		return
	}

	// retrieve workspace and agent
	// only load working directory if this is the internal root path of the app
	// because it is a costly application and no other path is necessary
	agent, workingDir, err := core.EditorProxy(ctx, api.DB, api.VcsClient, workspaceID, callingUser.(*models.User).ID, internalPath == "/")
	if err != nil {
		if err.Error() == "agent not found" {
			api.HandleError(rw, "agent not found", r.URL.Path, "WorkspaceEditorProxy",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
				callingId, http.StatusNotFound, "not found", err)
			return
		}
		api.HandleError(rw, "failed retrieve agent", r.URL.Path, "WorkspaceEditorProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusInternalServerError, "internal server error", err)
		return
	}

	// conditionally force redirect to the working directory
	if internalPath == "/" && r.URL.Query().Get("folder") != workingDir {
		api.Logger.LogDebugExternalAPI(
			"function execution successful - redirecting to correct folder",
			r.URL.Path,
			"WorkspaceEditorProxy",
			r.Method,
			r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r),
			callingUserName,
			callingId,
			http.StatusOK,
			nil,
		)
		q := r.URL.Query()
		q.Set("folder", workingDir)
		queryString := q.Encode()
		http.Redirect(rw, r, fmt.Sprintf("%s?%s", r.URL.Path, queryString), http.StatusTemporaryRedirect)
		return
	}

	// set internal path for editor proxy target
	r.URL.Path = internalPath

	api.proxyWorkspacePort(proxyWorkspacePortOptions{
		CallingUser: callingUser.(*models.User),
		AgentID:     agent,
		// we only ever use 13337 for the editor port
		Port: 13337,
	}, rw, r)

	parentSpan.AddEvent(
		"workspace-editor-proxy",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)
}

// WorkspaceDesktopProxy
//
//	Proxies a web call to the workspace desktop
func (api *WorkspaceAPI) WorkspaceDesktopProxy(rw http.ResponseWriter, r *http.Request) {
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-desktop-proxy-http")
	defer parentSpan.End()

	// this should never happen but let's handle it just incase
	if callingUser == nil {
		api.HandleError(rw, fmt.Sprintf("calling user is nil"), r.URL.Path, "WorkspaceDesktopProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "-1",
			http.StatusBadRequest, "logout", nil)
		return
	}

	// format user id to string
	callingId := fmt.Sprintf("%d", callingUser.(*models.User).ID)
	// extract username
	callingUserName := callingUser.(*models.User).UserName

	// retrieve params from url
	vars := mux.Vars(r)

	// attempt to retrieve user from params map
	userID, ok := vars["user"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no user id found in path", r.URL.Path, "WorkspaceDesktopProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// ensure that calling user and user id are the same
	if callingId != userID {
		api.HandleError(rw, fmt.Sprintf("userID mismatch: %s != %s", callingId, userID), r.URL.Path, "WorkspaceDesktopProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// load workspace id from url
	workspaceIDString, ok := vars["workspace"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no workspace id found in path", r.URL.Path, "WorkspaceDesktopProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// parse workspaceIDString into an int as a workspace id
	workspaceID, err := strconv.ParseInt(workspaceIDString, 10, 64)
	if err != nil {
		api.HandleError(rw, fmt.Sprintf("invalid workspace id: %s", workspaceIDString), r.URL.Path, "WorkspaceDesktopProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, "invalid url", nil)
		return
	}

	// trim base path for the desltop proxy to retrieve only the
	// portion of the path that should be forwarded to the desktop server
	internalPath := DesktopPathCleaner.ReplaceAllString(r.URL.Path, "")

	// handle icon injection
	iconInjected := api.handleIconInjection(internalPath, rw, r)
	if iconInjected {
		return
	}

	// retrieve workspace and agent
	// only load working directory if this is the internal root path of the app
	// because it is a costly application and no other path is necessary
	agent, err := core.DesktopProxy(ctx, api.DB, workspaceID, callingUser.(*models.User).ID)
	if err != nil {
		if err.Error() == "agent not found" {
			api.HandleError(rw, "agent not found", r.URL.Path, "WorkspaceDesktopProxy",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
				callingId, http.StatusNotFound, "not found", err)
			return
		}
		api.HandleError(rw, "failed retrieve agent", r.URL.Path, "WorkspaceDesktopProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusInternalServerError, "internal server error", err)
		return
	}

	// set internal path for editor proxy target
	r.URL.Path = internalPath

	api.proxyWorkspacePort(proxyWorkspacePortOptions{
		CallingUser: callingUser.(*models.User),
		AgentID:     agent,
		// we only ever use 13338 for the desktop port
		Port: 13338,
	}, rw, r)

	parentSpan.AddEvent(
		"workspace-desktop-proxy",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)
}

// WorkspaceAgentProxy
//
//	Proxies a web call to the workspace editor
func (api *WorkspaceAPI) WorkspaceAgentProxy(rw http.ResponseWriter, r *http.Request) {
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-agent-proxy-http")
	defer parentSpan.End()

	// this should never happen but let's handle it just incase
	if callingUser == nil {
		api.HandleError(rw, fmt.Sprintf("calling user is nil"), r.URL.Path, "WorkspaceAgentProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "-1",
			http.StatusBadRequest, "logout", nil)
		return
	}

	// format user id to string
	callingId := fmt.Sprintf("%d", callingUser.(*models.User).ID)
	// extract username
	callingUserName := callingUser.(*models.User).UserName

	// retrieve params from url
	vars := mux.Vars(r)

	// attempt to retrieve user from params map
	userID, ok := vars["user"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no user id found in path", r.URL.Path, "WorkspaceAgentProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// ensure that calling user and user id are the same
	if callingId != userID {
		api.HandleError(rw, fmt.Sprintf("userID mismatch: %s != %s", callingId, userID), r.URL.Path, "WorkspaceAgentProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// load workspace id from url
	workspaceIDString, ok := vars["workspace"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no workspace id found in path", r.URL.Path, "WorkspaceAgentProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// parse workspaceIDString into an int as a workspace id
	workspaceID, err := strconv.ParseInt(workspaceIDString, 10, 64)
	if err != nil {
		api.HandleError(rw, fmt.Sprintf("invalid workspace id: %s", workspaceIDString), r.URL.Path, "WorkspaceAgentProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, "invalid url", nil)
		return
	}

	// ensure that if we have an empty path we redirect to ws
	if AgentPathCleaner.ReplaceAllString(r.URL.Path, "") != "/ws" {
		api.Logger.LogDebugExternalAPI(
			"function execution successful - redirecting to websocket",
			r.URL.Path,
			"WorkspaceAgentProxy",
			r.Method,
			r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r),
			callingUserName,
			callingId,
			http.StatusOK,
			nil,
		)
		http.Redirect(rw, r, r.URL.Path+"/ws", http.StatusTemporaryRedirect)
		return
	}

	// trim base path for the editor proxy to retrieve only the
	// portion of the path that should be forwarded to the editor
	internalPath := EditorPathCleaner.ReplaceAllString(r.URL.Path, "")

	// handle icon injection
	iconInjected := api.handleIconInjection(internalPath, rw, r)
	if iconInjected {
		return
	}

	// retrieve workspace and agent
	// only load working directory if this is the internal root path of the app
	// because it is a costly application and no other path is necessary
	agent, err := core.AgentProxy(ctx, api.DB, workspaceID, callingUser.(*models.User).ID)
	if err != nil {
		if err.Error() == "agent not found" {
			api.HandleError(rw, "agent not found", r.URL.Path, "WorkspaceAgentProxy",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
				callingId, http.StatusNotFound, "not found", err)
			return
		}
		api.HandleError(rw, "failed retrieve agent", r.URL.Path, "WorkspaceAgentProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusInternalServerError, "internal server error", err)
		return
	}

	// set internal path for editor proxy target
	r.URL.Path = internalPath

	api.proxyWorkspacePort(proxyWorkspacePortOptions{
		CallingUser: callingUser.(*models.User),
		AgentID:     agent,
		Port:        agentsdk.ZitiInitConnPort,
	}, rw, r)

	parentSpan.AddEvent(
		"workspace-agent-proxy",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)
}

// WorkspacePortProxy
//
//	Proxies a web call to a workspace port
func (api *WorkspaceAPI) WorkspacePortProxy(rw http.ResponseWriter, r *http.Request) {
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "workspace-port-proxy-http")
	defer parentSpan.End()

	// this should never happen but let's handle it just incase
	if callingUser == nil {
		api.HandleError(rw, fmt.Sprintf("calling user is nil"), r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "n/a", "-1",
			http.StatusBadRequest, "logout", nil)
		return
	}

	// format user id to string
	callingId := fmt.Sprintf("%d", callingUser.(*models.User).ID)
	// extract username
	callingUserName := callingUser.(*models.User).UserName

	// retrieve params from url
	vars := mux.Vars(r)

	// attempt to retrieve user from params map
	userID, ok := vars["user"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no user id found in path", r.URL.Path, "WorkspacePortProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// ensure that calling user and user id are the same
	if callingId != userID {
		api.HandleError(rw, fmt.Sprintf("userID mismatch: %s != %s", callingId, userID), r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusForbidden, "forbidden", nil)
		return
	}

	// load workspace id from url
	workspaceIDString, ok := vars["workspace"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no workspace id found in path", r.URL.Path, "WorkspacePortProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// parse workspaceIDString into an int as a workspace id
	workspaceID, err := strconv.ParseInt(workspaceIDString, 10, 64)
	if err != nil {
		api.HandleError(rw, fmt.Sprintf("invalid workspace id: %s", workspaceIDString), r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, "invalid url", nil)
		return
	}

	// load target port from url
	workspacePortString, ok := vars["port"]
	if !ok {
		// handle error internally
		api.HandleError(rw, "no port found in path", r.URL.Path, "WorkspacePortProxy", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusBadRequest,
			"invalid path", nil)
		return
	}

	// parse workspacePortString into an int32 so that we can convert it to an uint16
	portRaw, err := strconv.ParseInt(workspacePortString, 10, 32)
	if err != nil {
		api.HandleError(rw, fmt.Sprintf("invalid port: %s", workspacePortString), r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, "invalid url", nil)
		return
	}

	// ensure this is an unsigned integer
	if portRaw < 0 {
		api.HandleError(rw, fmt.Sprintf("invalid port: %s", workspacePortString), r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, "invalid url", nil)
		return
	}

	// convert the port to uint16
	port := uint16(portRaw)

	// handle icon injection
	iconInjected := api.handleIconInjection(r.URL.Path, rw, r)
	if iconInjected {
		return
	}

	// ensure that port is not blacklisted
	if _, ok := agentsdk.IgnoredListeningPorts[port]; ok {
		api.HandleError(rw, fmt.Sprintf("port %d is blacklisted", port), r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusBadRequest, fmt.Sprintf("The port %d is not allowed to be forwarded.", port),
			nil)
		return
	}

	// retrieve workspace and agent
	agent, portData, err := core.PortProxyGetWorkspaceAgentID(ctx, api.DB, workspaceID, callingUser.(*models.User).ID, port)
	if err != nil {
		if err.Error() == "port not found" {
			api.HandleError(rw, "port not found", r.URL.Path, "WorkspacePortProxy",
				r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
				callingId, http.StatusNotFound, "not found", err)
			return
		}
		api.HandleError(rw, "failed retrieve agent", r.URL.Path, "WorkspacePortProxy",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUserName,
			callingId, http.StatusInternalServerError, "internal server error", err)
		return
	}

	// TODO: maybe filter non-http ports?

	api.proxyWorkspacePort(proxyWorkspacePortOptions{
		CallingUser: callingUser.(*models.User),
		AgentID:     agent,
		Port:        port,
		SSL:         portData.SSL,
	}, rw, r)

	parentSpan.AddEvent(
		"workspace-port-proxy",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)
}

// proxyWorkspacePort
//
//	Helper function to handle the logic of forwarding a workspace port
//	to a web call via the gigo core servers.
//
//	This function is used to forward editors and normal ports
//
//	WARNING: YOU MUST AUTHENTICATE A USER FOR THE PORT THEY WILL BE PROXIED TO
//	WE DO NOT DO ANY AUTH IN THIS FUNCTION. THIS FUNCTION ASSUMES THE USER IT
//	IS PROVIDED IS PERMITTED TO ACCESS THE RESOURCE!!!!!!
func (api *WorkspaceAPI) proxyWorkspacePort(opts proxyWorkspacePortOptions, rw http.ResponseWriter, r *http.Request) {
	// format user id to string
	callingId := fmt.Sprintf("%d", opts.CallingUser.ID)

	// select the protocol based on ssl or not
	protocol := "http"
	if opts.SSL {
		protocol = "https"
	}

	// the only thing that matters in the app url is the scheme since
	// we are offloading to the zitinet which will handle the port and
	// host on the agent side of the zitinet
	appURL, err := url.Parse(fmt.Sprintf("%s://dummy-host", protocol))
	if err != nil {
		api.HandleError(rw, "failed to parse internal app url", r.URL.Path, "proxyWorkspacePort",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), opts.CallingUser.UserName,
			callingId, http.StatusInternalServerError, "internal server error", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(appURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		api.HandleError(rw, fmt.Sprintf("failed to proxy request to application: %s:%d", zitimesh.NetworkTypeTCP, int(opts.Port)), r.URL.Path, "proxyWorkspacePort",
			r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), opts.CallingUser.UserName,
			callingId, http.StatusBadGateway, "internal server error", err)
		return
	}

	// we need to override the http transport to operate on the zitinet
	proxy.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (netConn net.Conn, e error) {
			// we dial the agent here using the zitimesh server which will
			// establish a connection to the end target on the agent over
			// the ziti net mesh ovelay
			return api.ZitiServer.DialAgent(opts.AgentID, zitimesh.NetworkTypeTCP, int(opts.Port))
		},
		// insecure verify for our internal port
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// This strips the session token from a workspace app request.
	cookieHeaders := r.Header.Values("Cookie")[:]
	r.Header.Del("Cookie")
	for _, cookieHeader := range cookieHeaders {
		r.Header.Add("Cookie", StripGigoCookies(cookieHeader))
	}

	// Convert canonicalized headers to their non-canonicalized counterparts.
	// See the comment on `nonCanonicalHeaders` for more information on why this
	// is necessary.
	for k, v := range r.Header {
		if n, ok := nonCanonicalHeaders[k]; ok {
			r.Header.Del(k)
			r.Header[n] = v
		}
	}

	// log successful function execution
	api.Logger.LogDebugExternalAPI(
		fmt.Sprintf("function execution successful - proxying to %d - %s", opts.AgentID, r.URL.String()),
		r.URL.Path,
		"proxyWorkspacePort",
		r.Method,
		r.Context().Value(CtxKeyRequestID),
		network.GetRequestIP(r),
		opts.CallingUser.UserName,
		callingId,
		http.StatusOK,
		nil,
	)

	proxy.ServeHTTP(rw, r)
}

// handleIconInjection
//
// Injects the appropriate icon in place of the proxied icon.
// If an icon was injected the true is returned and the caller should exit
// since the response has already been written. If false is returned the caller
// should continue with processing the request normally.
func (api *WorkspaceAPI) handleIconInjection(path string, rw http.ResponseWriter, r *http.Request) bool {
	// detect if the path should be skipped
	if !strings.HasSuffix(path, "/favicon.ico") &&
		!strings.HasSuffix(path, "/favicon-dark-support.ico") &&
		!strings.HasSuffix(path, "/favicon-dark-support.svg") &&
		!strings.HasSuffix(path, "/apple-icon.png") &&
		!strings.HasSuffix(path, "/icon192.png") &&
		!strings.HasSuffix(path, "/icon512.png") {
		return false
	}

	// trim the path to the final element
	parts := strings.Split(path, "/")
	trimmedPath := parts[len(parts)-1]
	if strings.HasPrefix(trimmedPath, "favicon-dark-support") {
		trimmedPath = "favicon.ico"
	}

	// read the icon data from disk
	buf, err := StatidFS.ReadFile("static/" + trimmedPath)
	if err != nil {
		api.Logger.Errorf("failed to read static icon path %q: %w", path, err)
	}

	// set the correct mime type header
	mimeType := ""
	switch trimmedPath {
	case "favicon.ico":
		mimeType = "image/x-icon"
	default:
		mimeType = "image/png"
	}

	// write the icon data to the response writer
	rw.Header().Set("Content-Type", mimeType)
	rw.Header().Set("Cache-Control", "public, max-age=604800")
	rw.Header().Set("Expires", time.Now().Add(time.Hour*24*7).Format(time.RFC1123))
	rw.Header().Set("Last-Modified", time.Unix(0, 0).UTC().Format(time.RFC1123))
	rw.Header().Set("ETag", `gigo-http-icon-v1`)

	_, _ = rw.Write(buf)
	return true
}
