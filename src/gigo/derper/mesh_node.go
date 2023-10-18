// Copied and modified from https://github.com/tailscale/tailscale/blob/390db46aad3cdaa1f08151ff13a186c3c1acab72/derp/derphttp/mesh_client.go

// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package derper

import (
	"context"
	"go.opentelemetry.io/otel"
	"sync"
	"tailscale.com/derp/derphttp"
	"time"

	"tailscale.com/derp"
	"tailscale.com/types/key"
	"tailscale.com/types/logger"
)

// MeshNode
//
//	Remote DERP server in the mesh cluster.
type MeshNode struct {
	*derphttp.Client
	ID        int64
	PublicKey key.NodePublic
	logger    logger.Logf
	ctx       context.Context
	cancel    context.CancelFunc
}

// RunWatchConnectionLoop loops until ctx is done, sending WatchConnectionChanges and subscribing to
// connection changes.
//
// If the server's public key is ignoreServerKey, RunWatchConnectionLoop returns.
//
// Otherwise, the add and remove funcs are called as clients come & go.
//
// infoLogf, if non-nil, is the logger to write periodic status
// updates about how many peers are on the server. Error log output is
// set to the c's logger, regardless of infoLogf's value.
//
// To force RunWatchConnectionLoop to return quickly, its ctx needs to
// be closed, and c itself needs to be closed.
func (n *MeshNode) RunWatchConnectionLoop(ctx context.Context, ignoreServerKey key.NodePublic, infoLogf logger.Logf, add, remove func(key.NodePublic)) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "derper-run-watch-connection-loop")
	defer parentSpan.End()

	if infoLogf == nil {
		infoLogf = logger.Discard
	}
	logf := n.logger
	const retryInterval = 5 * time.Second
	const statusInterval = 10 * time.Second
	var (
		mu              sync.Mutex
		present         = map[key.NodePublic]bool{}
		loggedConnected = false
	)
	clear := func() {
		mu.Lock()
		defer mu.Unlock()
		if len(present) == 0 {
			return
		}
		logf("reconnected; clearing %d forwarding mappings", len(present))
		for k := range present {
			remove(k)
		}
		present = map[key.NodePublic]bool{}
	}
	lastConnGen := 0
	lastStatus := time.Now()
	logConnectedLocked := func() {
		if loggedConnected {
			return
		}
		infoLogf("connected; %d peers", len(present))
		loggedConnected = true
	}

	const logConnectedDelay = 200 * time.Millisecond
	timer := time.AfterFunc(2*time.Second, func() {
		mu.Lock()
		defer mu.Unlock()
		logConnectedLocked()
	})
	defer timer.Stop()

	updatePeer := func(k key.NodePublic, isPresent bool) {
		if isPresent {
			add(k)
		} else {
			remove(k)
		}

		mu.Lock()
		defer mu.Unlock()
		if isPresent {
			present[k] = true
			if !loggedConnected {
				timer.Reset(logConnectedDelay)
			}
		} else {
			// If we got a peerGone message, that means the initial connection's
			// flood of peerPresent messages is done, so we can log already:
			logConnectedLocked()
			delete(present, k)
		}
	}

	sleep := func(d time.Duration) {
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
		case <-t.C:
		}
	}

	for ctx.Err() == nil {
		err := n.WatchConnectionChanges()
		if err != nil {
			clear()
			logf("WatchConnectionChanges: %v", err)
			sleep(retryInterval)
			continue
		}

		if n.ServerPublicKey() == ignoreServerKey {
			logf("detected self-connect; ignoring host")
			return
		}
		for {
			m, connGen, err := n.RecvDetail()
			if err != nil {
				clear()
				logf("Recv: %v", err)
				sleep(retryInterval)
				break
			}
			if connGen != lastConnGen {
				lastConnGen = connGen
				clear()
			}
			infoLogf("received message from peer")
			if now := time.Now(); now.Sub(lastStatus) > statusInterval {
				lastStatus = now
				infoLogf("derp node has %d peers", len(present))
			}
			switch m := m.(type) {
			case derp.PeerPresentMessage:
				infoLogf("adding new peer")
				updatePeer(key.NodePublic(m), true)
			case derp.PeerGoneMessage:
				infoLogf("removing existing peer")
				updatePeer(key.NodePublic(m), false)
			default:
				continue
			}
		}
	}
}
