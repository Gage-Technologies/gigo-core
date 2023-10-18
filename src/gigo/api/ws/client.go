package ws

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	proto "gigo-core/protos/ws"

	"github.com/gage-technologies/drpc-lib/muxconn"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/google/uuid"
)

var (
	ErrMalformedRequest         = fmt.Errorf("malformed request")
	ErrWorkspaceNotFound        = fmt.Errorf("workspace not found")
	ErrAlternativeRequestActive = fmt.Errorf("alternative request active")
)

// CreateWorkspaceOptions
//
//	Options used to create a new workspace.
type CreateWorkspaceOptions struct {
	WorkspaceID int64
	OwnerID     int64
	OwnerEmail  string
	OwnerName   string
	Disk        int
	CPU         int
	Memory      int
	Container   string
	AccessUrl   string
}

// NewAgent
//
//	Return values for a new agent created by CreateWorkspace or StartWorkspace.
type NewAgent struct {
	ID    int64
	Token uuid.UUID
}

// WorkspaceClientConnectionOptions
//
//	Options for a new workspace client connection.
type WorkspaceClientConnectionOptions struct {
	Host string
	Port int
}

// WorkspaceClientConnection
//
//	Connection to a remote provisioner
type WorkspaceClientConnection struct {
	WorkspaceClientConnectionOptions
	conn   *muxconn.Conn
	client proto.DRPCGigoWSClient
}

// WorkspaceClientOptions
//
//	Options for a new workspace client
type WorkspaceClientOptions struct {
	Servers []WorkspaceClientConnectionOptions
	Logger  logging.Logger
}

// WorkspaceClient
//
//	Workspace client wrapper to manage connections to remote provisioners.
//	The struct will manage auto-reconnect and load balancing.
type WorkspaceClient struct {
	clients []*WorkspaceClientConnection
	idx     int
	lock    *sync.Mutex
	logger  logging.Logger
}

// dial
//
//	Dials a remote provisioner server and returns an
//	active drpc connection.
func dial(opts WorkspaceClientConnectionOptions) (proto.DRPCGigoWSClient, *muxconn.Conn, error) {
	// dial server
	rawconn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial remote server: %v", err)
	}

	// create a drpc connection
	conn, err := muxconn.New(rawconn)
	if err != nil {
		rawconn.Close()
		return nil, nil, fmt.Errorf("failed to create drpc connection: %v", err)
	}

	// create new client
	client := proto.NewDRPCGigoWSClient(conn)

	return client, conn, nil
}

func NewWorkspaceClient(opts WorkspaceClientOptions) (*WorkspaceClient, error) {
	// ensure there is at least one client
	if len(opts.Servers) == 0 {
		return nil, fmt.Errorf("at least one server is required")
	}

	// create slice to hold client connections
	clients := make([]*WorkspaceClientConnection, 0)

	// keep tally of active clients
	active := 0

	// connect to each server
	for _, server := range opts.Servers {
		opts.Logger.Debugf("connecting to remote provisioner: %s:%d", server.Host, server.Port)
		c, conn, err := dial(server)

		// we only want to log this because the connection may become available
		// in the future and we want to preserve the ability to reconnect
		if err != nil {
			opts.Logger.Warnf("failed connection to remote provisioner: %s:%d", server.Host, server.Port)
		} else {
			opts.Logger.Debugf("established connection to remote provisioner: %s:%d", server.Host, server.Port)
		}

		clients = append(clients, &WorkspaceClientConnection{
			WorkspaceClientConnectionOptions: server,
			client:                           c,
			conn:                             conn,
		})
		active++
	}

	// ensure we got at least one client
	if active == 0 {
		return nil, fmt.Errorf("failed to connnection to remote provisioner")
	}

	return &WorkspaceClient{
		clients: clients,
		idx:     0,
		lock:    new(sync.Mutex),
		logger:  opts.Logger,
	}, nil
}

// getClient
//
//	Returns an active client connection. If the current
//	client connection is not valid, it will create a new one.
func (c *WorkspaceClient) getClient() (proto.DRPCGigoWSClient, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// reset index if we have exceeded the client count
	if c.idx >= len(c.clients) {
		c.idx = 0
	}

	// create integer to track how many clients we've
	// tried because after we have failed on all we need
	// to fail the connection
	attempts := 0

	// loop until we fail or succeed
	for {
		// reset the index if we have exceeded the client count
		if c.idx >= len(c.clients) {
			c.idx = 0
		}

		// retrieve client from set
		client := c.clients[c.idx]

		// save our current index
		idx := c.idx

		// increment the index and attempt
		c.idx++
		attempts++

		// exit if we have attempted all clients up to 3 times
		if attempts > len(c.clients)*3 {
			c.logger.Warn("failed to establish connection with all clients")
			return nil, fmt.Errorf("failed to connect to any remote provisioners after 3 attempts")
		}

		// sleep for 500ms after we have attempted all clients
		if attempts%len(c.clients) == 0 {
			time.Sleep(500 * time.Millisecond)
		}

		// check if the connection is still active but only if the client and connection are not nil
		active := false
		if client == nil && client.conn == nil {
			select {
			case <-client.conn.Closed():
				active = false
			default:
				// we think the connection is alive but we ping the server to be sure
				echoCtx, echoCancel := context.WithTimeout(context.Background(), time.Second)
				_, err := client.client.Echo(echoCtx, &proto.EchoRequest{})
				echoCancel()
				if err != nil {
					// close out the connection for any cleanup that is necessary
					active = false
					_ = client.conn.Close()
				} else {
					active = true
				}
			}
		}

		// attempt to reconnect if the connection is not active
		if !active {
			c.logger.Debug(fmt.Sprintf(
				"attempting connection to remote provisioner: %s:%d  attempts #: %v",
				client.WorkspaceClientConnectionOptions.Host, client.WorkspaceClientConnectionOptions.Port, attempts,
			))
			newClient, newConn, err := dial(client.WorkspaceClientConnectionOptions)
			if err != nil {
				c.logger.Warn(fmt.Sprintf(
					"failed connection to remote provisioner: %s:%d",
					client.WorkspaceClientConnectionOptions.Host, client.WorkspaceClientConnectionOptions.Port,
				))
				// attempt the next client
				continue
			}

			c.logger.Debug(fmt.Sprintf(
				"established connection to remote provisioner: %s:%d",
				client.WorkspaceClientConnectionOptions.Host, client.WorkspaceClientConnectionOptions.Port,
			))

			// double check the connection is alive with a ping
			echoCtx, echoCancel := context.WithTimeout(context.Background(), time.Second)
			_, err = newClient.Echo(echoCtx, &proto.EchoRequest{})
			echoCancel()
			if err != nil {
				c.logger.Warn(fmt.Sprintf(
					"failed connection to remote provisioner: %s:%d",
					client.WorkspaceClientConnectionOptions.Host, client.WorkspaceClientConnectionOptions.Port,
				))
				// attempt the next client
				continue
			}

			// store the new connection in the old connections place
			c.clients[idx] = &WorkspaceClientConnection{
				WorkspaceClientConnectionOptions: client.WorkspaceClientConnectionOptions,
				client:                           newClient,
				conn:                             newConn,
			}
		}

		return c.clients[idx].client, nil
	}
}

// CreateWorkspace
//
//	Creates a new workspace via the remote provisoner
//	and returns a new agent id and secret
func (c *WorkspaceClient) CreateWorkspace(ctx context.Context, opts CreateWorkspaceOptions) (*NewAgent, error) {
	// create proto for request
	req := &proto.CreateWorkspaceRequest{
		WorkspaceId: opts.WorkspaceID,
		OwnerId:     opts.OwnerID,
		OwnerEmail:  opts.OwnerEmail,
		OwnerName:   opts.OwnerName,
		Disk:        int32(opts.Disk),
		Cpu:         int32(opts.CPU),
		Memory:      int32(opts.Memory),
		Container:   opts.Container,
		AccessUrl:   opts.AccessUrl,
	}

	// retrieve a client
	client, err := c.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve a client: %v", err)
	}

	// execute remote provision call
	res, err := client.CreateWorkspace(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %v", err)
	}

	// check status code
	if res.GetStatus() != proto.ResponseCode_SUCCESS {
		// handle go error
		if res.GetError() != nil && res.GetError().GetGoError() != "" {
			return nil, fmt.Errorf("remote server error creating workspace: %v", res.GetError().GetGoError())
		}

		// handle command error
		if res.GetError() != nil && res.GetError().GetCmdError() != nil {
			cmdErr := res.GetError().GetCmdError()
			return nil, fmt.Errorf(
				"remote command error creating workspace\n    status: %d\n    out: %s\n    err: %s",
				cmdErr.GetExitCode(), cmdErr.GetStdout(), cmdErr.GetStderr(),
			)
		}

		// handle unknown error
		return nil, selectError(res.GetStatus(), fmt.Errorf("failed to create workspace: %v", res.GetStatus().String()))
	}

	// ensure that agent id and token are present
	if res.GetAgentId() == 0 || res.GetAgentToken() == "" {
		return nil, fmt.Errorf("failed to create workspace: new agent data missing")
	}

	// format token to uuid
	tokenUuid, err := uuid.Parse(res.GetAgentToken())
	if err != nil {
		return nil, fmt.Errorf("failed to parse uuid: %v", err)
	}

	return &NewAgent{
		ID:    res.GetAgentId(),
		Token: tokenUuid,
	}, nil
}

// StartWorkspace
//
//	Starts a stopped workspace via the remote
//	provisioner and returns an existing agent id
//	and new agent token
func (c *WorkspaceClient) StartWorkspace(ctx context.Context, workspaceId int64) (*NewAgent, error) {
	// retrieve a client
	client, err := c.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve a client: %v", err)
	}

	// execute remote provision call
	res, err := client.StartWorkspace(ctx, &proto.StartWorkspaceRequest{
		WorkspaceId: workspaceId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start workspace: %v", err)
	}

	// check status code
	if res.GetStatus() != proto.ResponseCode_SUCCESS {
		// handle go error
		if res.GetError() != nil && res.GetError().GetGoError() != "" {
			return nil, fmt.Errorf("remote server error start workspace: %v", res.GetError().GetGoError())
		}

		// handle command error
		if res.GetError() != nil && res.GetError().GetCmdError() != nil {
			cmdErr := res.GetError().GetCmdError()
			return nil, fmt.Errorf(
				"remote command error start workspace\n    status: %d\n    out: %s\n    err: %s",
				cmdErr.GetExitCode(), cmdErr.GetStdout(), cmdErr.GetStderr(),
			)
		}

		// handle unknown error
		return nil, selectError(res.GetStatus(), fmt.Errorf("failed to start workspace: %v", res.GetStatus().String()))
	}

	// ensure that agent id and token are present
	if res.GetAgentId() == 0 || res.GetAgentToken() == "" {
		return nil, fmt.Errorf("failed to start workspace: new agent data missing")
	}

	// format token to uuid
	tokenUuid, err := uuid.Parse(res.GetAgentToken())
	if err != nil {
		return nil, fmt.Errorf("failed to parse uuid: %v", err)
	}

	return &NewAgent{
		ID:    res.GetAgentId(),
		Token: tokenUuid,
	}, nil
}

// StopWorkspace
//
//	Stops an active workspace via the remote provisioner
func (c *WorkspaceClient) StopWorkspace(ctx context.Context, workspaceId int64) error {
	// retrieve a client
	client, err := c.getClient()
	if err != nil {
		return fmt.Errorf("failed to retrieve a client: %v", err)
	}

	// execute remote provision call
	res, err := client.StopWorkspace(ctx, &proto.StopWorkspaceRequest{
		WorkspaceId: workspaceId,
	})
	if err != nil {
		return fmt.Errorf("failed to stop workspace: %v", err)
	}

	// check status code
	if res.GetStatus() != proto.ResponseCode_SUCCESS {
		// handle go error
		if res.GetError() != nil && res.GetError().GetGoError() != "" {
			return fmt.Errorf("remote server error stop workspace: %v", res.GetError().GetGoError())
		}

		// handle command error
		if res.GetError() != nil && res.GetError().GetCmdError() != nil {
			cmdErr := res.GetError().GetCmdError()
			return fmt.Errorf(
				"remote command error stop workspace\n    status: %d\n    out: %s\n    err: %s",
				cmdErr.GetExitCode(), cmdErr.GetStdout(), cmdErr.GetStderr(),
			)
		}

		// handle unknown error
		return selectError(res.GetStatus(), fmt.Errorf("failed to start workspace: %v", res.GetStatus().String()))
	}

	return nil
}

// DestroyWorkspace
//
//	Destroys an existing workspace via the remote provisioner
//	for a stopped or active workspace.
func (c *WorkspaceClient) DestroyWorkspace(ctx context.Context, workspaceId int64) error {
	// retrieve a client
	client, err := c.getClient()
	if err != nil {
		return fmt.Errorf("failed to retrieve a client: %v", err)
	}

	// execute remote provision call
	res, err := client.DestroyWorkspace(ctx, &proto.DestroyWorkspaceRequest{
		WorkspaceId: workspaceId,
	})
	if err != nil {
		return fmt.Errorf("failed to destroy workspace: %v", err)
	}

	// check status code
	if res.GetStatus() != proto.ResponseCode_SUCCESS {
		// handle go error
		if res.GetError() != nil && res.GetError().GetGoError() != "" {
			return fmt.Errorf("remote server error destroy workspace: %v", res.GetError().GetGoError())
		}

		// handle command error
		if res.GetError() != nil && res.GetError().GetCmdError() != nil {
			cmdErr := res.GetError().GetCmdError()
			return fmt.Errorf(
				"remote command error destroy workspace\n    status: %d\n    out: %s\n    err: %s",
				cmdErr.GetExitCode(), cmdErr.GetStdout(), cmdErr.GetStderr(),
			)
		}

		// handle unknown error
		return selectError(res.GetStatus(), fmt.Errorf("failed to destroy workspace: %v", res.GetStatus().String()))
	}

	return nil
}

func selectError(code proto.ResponseCode, fallbackErr error) error {
	switch code {
	case proto.ResponseCode_MALFORMED_REQUEST:
		return ErrMalformedRequest
	case proto.ResponseCode_NOT_FOUND:
		return ErrWorkspaceNotFound
	case proto.ResponseCode_ALTERNATIVE_REQUEST_ACTIVE:
		return ErrAlternativeRequestActive
	default:
		return fallbackErr
	}
}
