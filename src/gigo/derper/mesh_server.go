package derper

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"sync"
	"time"

	"github.com/coder/retry"
	"github.com/gage-technologies/gigo-lib/cluster"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/sourcegraph/conc"
	"tailscale.com/derp"
	"tailscale.com/derp/derphttp"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

// MeshServer
//
//	Creates a DERP server that automatically meshes with
//	nodes in the GIGO core cluster
type MeshServer struct {
	*derp.Server
	ctx       context.Context
	node      cluster.Node
	lock      *sync.Mutex
	forceHttp bool
	meshNodes map[int64]*MeshNode
	wg        *conc.WaitGroup
	logger    logging.Logger
}

func NewMeshServer(ctx context.Context, node cluster.Node, forceHttp bool, meshKey string,
	port int, logger logging.Logger) (*MeshServer, error) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "derper-new-mesh-server")
	defer parentSpan.End()

	// create a new derp server
	server := derp.NewServer(key.NewNode(), logger.TailscaleDebugLogger("ts-derp"))
	server.SetMeshKey(meshKey)

	// create local derp node
	localDerpNode := tailcfg.DERPNode{
		Name:      fmt.Sprintf("%d", node.GetSelfMetadata().ID),
		RegionID:  999,
		HostName:  node.GetSelfMetadata().Address,
		DERPPort:  port,
		STUNPort:  -1,
		ForceHTTP: forceHttp,
	}

	// save local derp node to cluster storage
	buf, err := json.Marshal(localDerpNode)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal local derp node to json: %v", err)
	}

	// retry waiting for node to acquire a lease
	for retrier := retry.New(time.Millisecond*50, time.Second); retrier.Wait(ctx); {
		err = node.Put("derp-node", string(buf))
		if err != nil {
			if err == cluster.ErrNoLease {
				logger.Warn("failed to put derp node to cluster node - waiting for lease")
				continue
			}
			return nil, fmt.Errorf("failed to save local derp node to cluster storage: %v", err)
		}
		break
	}

	// marshall public key to be stored in the cluster state
	pubKey, err := server.PublicKey().MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshall public key: %v", err)
	}

	// base64 encode the public key
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKey)

	// register ourselves with the cluster
	// NOTE: we don't retry here because the above retry should have
	// confirmed that the cluster node has a lease and if it lost the
	// lease by this point something has gone seriously wrong and we
	// should be failing the function
	err = node.Put("derp-key", pubKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to register ourselves with the cluster: %v", err)
	}

	// create mesh server
	meshServer := &MeshServer{
		Server:    server,
		ctx:       ctx,
		node:      node,
		lock:      &sync.Mutex{},
		forceHttp: forceHttp,
		meshNodes: make(map[int64]*MeshNode),
		wg:        conc.NewWaitGroup(),
		logger:    logger,
	}

	// initialize the mesh server
	err = meshServer.initializeMeshClients()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mesh clients: %v", err)
	}

	// launch derp node watcher via the mesh server's wait group
	meshServer.wg.Go(meshServer.derpNodeWatcher)

	parentSpan.AddEvent(
		"derper-new-mesh-server",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	// return mesh server
	return meshServer, nil
}

// logDerpState
//
//	Logs the current state of the DERP mesh
func (s *MeshServer) logDerpState() {
	// create slice containing the node ids for all mesh nodes
	meshNodeIds := make([]int64, 0)
	for id := range s.meshNodes {
		meshNodeIds = append(meshNodeIds, id)
	}
	s.logger.Debugf("DERP mesh state: %d -> %v", s.node.GetSelfMetadata().ID, meshNodeIds)
}

// createMeshNodeClient
//
//		Creates a new mesh client for the given node and launches
//		the mesh coordinator in a goroutine via the MeshServer's
//		wait group. This function will retry with an
//	 exponential backoff until the context is canceled.
func (s *MeshServer) createMeshNodeClient(ctx context.Context, cancel context.CancelFunc, nodeMeta cluster.NodeMetadata,
	publicKey key.NodePublic) (*MeshNode, error) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "derper-create-mesh-node-client")
	defer parentSpan.End()

	// format url to derp node
	scheme := "https"
	if s.forceHttp {
		scheme = "http"
	}
	nodeUrl := fmt.Sprintf("%s://%s/derp", scheme, nodeMeta.Address)

	// create a new logger for the mesh client
	meshClientLogger := s.logger.TailscaleDebugLogger(fmt.Sprintf("ts-derp-mesh-client-%d", nodeMeta.ID))

	// run connection on exponential backoff
	for retrier := retry.New(time.Millisecond*100, time.Second*10); retrier.Wait(ctx); {
		// create DERP client to node
		client, err := derphttp.NewClient(s.PrivateKey(), nodeUrl, meshClientLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create DERP client: %v", err)
		}

		// set mesh key for derp client
		client.MeshKey = s.MeshKey()

		// create callbacks for adding and removing mesh nodes
		add := func(k key.NodePublic) { s.AddPacketForwarder(k, client) }
		remove := func(k key.NodePublic) { s.RemovePacketForwarder(k, client) }

		// create a new mesh node
		meshNode := &MeshNode{
			Client:    client,
			ID:        nodeMeta.ID,
			PublicKey: publicKey,
			logger:    meshClientLogger,
			ctx:       ctx,
			cancel:    cancel,
		}

		// launch mesh coordinator loop via mesh server's wait group
		s.wg.Go(func() {
			go meshNode.RunWatchConnectionLoop(ctx, s.PublicKey(), meshClientLogger, add, remove)
		})

		// create new mesh node and return
		return meshNode, nil
	}

	parentSpan.AddEvent(
		"derper-create-mesh-node-client",
		trace.WithAttributes(
			attribute.Bool("success", true),
		),
	)

	// return error from context if we are here
	return nil, ctx.Err()
}

// initializeMeshClients
//
//	Initializes connections to existing DERP nodes in the cluster
func (s *MeshServer) initializeMeshClients() error {

	// lock node while we initialize the mesh client
	s.lock.Lock()
	defer s.lock.Unlock()

	s.logger.Debug("initializing mesh clients")

	// retrieve public key for all known nodes in the cluster
	keys, err := s.node.GetCluster("derp-key")
	if err != nil {
		return fmt.Errorf("failed to retrieve public keys for nodes in the cluster: %v", err)
	}

	// iterate over all nodes in the cluster establishing connections to DERP nodes
	for nodeId, k := range keys {
		// skip ourselves
		if nodeId == s.node.GetSelfMetadata().ID {
			continue
		}

		// ensure that we have a value
		if len(k) == 0 {
			s.logger.Warnf("derp mesh public key not found for %d - skipping node", nodeId)
			continue
		}

		s.logger.Debugf("initialized mesh client for node: %d", nodeId)

		// decode public key bytes from base64
		pubKeyBytes, err := base64.StdEncoding.DecodeString(k[0].Value)
		if err != nil {
			return fmt.Errorf("failed to decode DERP node public key: %v", err)
		}

		// unmarshall DERP node public key
		pub := key.NodePublic{}
		err = pub.UnmarshalBinary(pubKeyBytes)
		if err != nil {
			s.logger.Warnf("failed to unmarshall DERP node public key - skipping node %d: %v", nodeId, err)
			continue
		}

		// get node metadata
		nodeMeta, err := s.node.GetNodeMetadata(nodeId)
		if err != nil {
			return fmt.Errorf("failed to get DERP node metadata: %v", err)
		}

		// handle non-existent DERP node
		if nodeMeta == nil {
			s.logger.Warnf("cluster node not found when initializing mesh client for node: %d", nodeId)
			continue
		}

		// create a new context for the mesh client
		meshCtx, meshCancel := context.WithCancel(s.ctx)

		// launch mesh node client creation in go routine via the mesh
		// server's wait group so that it can retry without blocking
		s.wg.Go(func() {
			// initialize the mesh node with nothing more than
			// the context so that we can cancel the client creation
			// if the node is deleted before we complete the client
			// creation
			s.meshNodes[nodeMeta.ID] = &MeshNode{
				ctx:    meshCtx,
				cancel: meshCancel,
			}

			// create mesh node client
			meshNode, err := s.createMeshNodeClient(meshCtx, meshCancel, *nodeMeta, pub)

			// acquire lock because whether it is an error or not we are
			// modifying the mesh
			s.lock.Lock()

			// handle error by cancelling the mesh node's context and removing
			// it from the mesh
			if err != nil {
				s.logger.Errorf("failed to create DERP mesh node client: %v", err)
				meshCancel()
				delete(s.meshNodes, nodeMeta.ID)
				s.lock.Unlock()
				return
			}

			s.logger.Debugf("added new node %d to DERP mesh on init", nodeMeta.ID)
			s.logDerpState()

			// save mesh node client to self
			s.meshNodes[nodeMeta.ID] = meshNode
			s.lock.Unlock()
		})
	}

	return nil
}

// derpNodeWatcher
//
//	Watches for the addition/removal of nodes from the cluster and automatically
//	updates the DERP mesh with the cluster state changes.
func (s *MeshServer) derpNodeWatcher() {
	s.logger.Debug("launching derp node watcher")

	// retry forever and just log because this is a
	// fundamental part of the DERP mesh cluster
	for retrier := retry.New(time.Millisecond*100, time.Millisecond*10); retrier.Wait(s.ctx); {
		// create a new context for the watch routine derived
		// from the mesh server's context
		ctx, cancel := context.WithCancel(s.ctx)

		// create watcher for the cluster nodes
		// we use `derp-key` as the key to watch for since
		// it is the last key that is set when a derp node
		// is created so it is the closest to when the node
		// will come online
		watcher, err := s.node.WatchKeyCluster(ctx, "derp-key")
		if err != nil {
			cancel()
			s.logger.Errorf("failed to create DERP node watcher: %v", err)
			continue
		}

		// iterate the watcher channel so that we can handle
		// each change event as it occurs
		for event := range watcher {
			// we don't care about modifications because the
			// only traits that can be modified in a node are
			// irrelevant to the DERP mesh
			if event.Type == cluster.EventTypeModified {
				continue
			}

			// ignore any event that has to do with us since
			// we don't want to be modifying ourselves in
			// the mesh cluster
			if event.NodeID == s.node.GetSelfMetadata().ID {
				continue
			}

			// lock node for modification
			s.lock.Lock()

			// handle deletion events by removing the node
			// from the derp mesh
			if event.Type == cluster.EventTypeDeleted {
				s.logger.Debugf("removing node %v from DERP mesh", event.NodeID)

				// retrieve node from mesh and skip if it doesn't exist
				node, ok := s.meshNodes[event.NodeID]
				if !ok {
					s.logger.Debugf("node not found in DERP mesh - skipping removal: %d", event.NodeID)
					s.lock.Unlock()
					continue
				}

				// close the node's context so that it exits gracefully
				node.cancel()

				// remove node from mesh
				delete(s.meshNodes, event.NodeID)

				s.logger.Debugf("removed node %d from DERP mesh", event.NodeID)
				s.logDerpState()
			}

			// handle addition events by adding the new node
			// to the derp mesh
			if event.Type == cluster.EventTypeAdded {
				s.logger.Debugf("adding new node %d to DERP mesh", event.NodeID)

				// sanity check to skip nodes that are already
				// present in the derp mesh
				if _, ok := s.meshNodes[event.NodeID]; ok {
					s.logger.Debugf("node already present in DERP mesh - skipping addition: %d", event.NodeID)
					s.lock.Unlock()
					continue
				}

				// decode public key bytes from base64
				pubKeyBytes, err := base64.StdEncoding.DecodeString(event.Value)
				if err != nil {
					s.lock.Unlock()
					s.logger.Errorf("failed to decode DERP node public key: %v", err)
					continue
				}

				// unmarshall DERP node public key
				pub := key.NodePublic{}
				err = pub.UnmarshalBinary(pubKeyBytes)
				if err != nil {
					s.lock.Unlock()
					s.logger.Errorf("failed to unmarshall DERP node public key: %v", err)
					continue
				}

				// get the node metadata
				nodeMeta, err := s.node.GetNodeMetadata(event.NodeID)
				if err != nil {
					s.lock.Unlock()
					s.logger.Errorf("failed to get DERP node metadata for nod %d: %v", event.NodeID, err)
					continue
				}

				// ensure that the node was found
				if nodeMeta == nil {
					s.lock.Unlock()
					s.logger.Errorf("cluster node not found while adding node %d to DERP mesh", event.NodeID)
					continue
				}

				// create a new context for the mesh client
				meshCtx, meshCancel := context.WithCancel(s.ctx)

				// launch mesh node client creation in go routine via the mesh
				// server's wait group so that it can retry without blocking
				s.wg.Go(func() {
					// initialize the mesh node with nothing more than
					// the context so that we can cancel the client creation
					// if the node is deleted before we complete the client
					// creation
					s.meshNodes[nodeMeta.ID] = &MeshNode{
						ctx:    meshCtx,
						cancel: meshCancel,
					}

					// create mesh node client
					meshNode, err := s.createMeshNodeClient(meshCtx, meshCancel, *nodeMeta, pub)

					// acquire lock because whether it is an error or not we are
					// modifying the mesh
					s.lock.Lock()

					// handle error by cancelling the mesh node's context and removing
					// it from the mesh
					if err != nil {
						s.logger.Errorf("failed to create DERP mesh node client: %v", err)
						meshCancel()
						delete(s.meshNodes, nodeMeta.ID)
						s.lock.Unlock()
						return
					}

					s.logger.Debugf("added new node %d to DERP mesh", event.NodeID)
					s.logDerpState()

					// save mesh node client to self
					s.meshNodes[nodeMeta.ID] = meshNode
					s.lock.Unlock()
				})
			}

			// unlock and begin wait for next event
			s.lock.Unlock()
		}

		// cancel context here for sanity since we
		// should never hit this point
		cancel()
	}
}
