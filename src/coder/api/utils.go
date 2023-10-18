package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/cluster"
	"github.com/gage-technologies/gigo-lib/coder/tailnet"
	"net/textproto"
	"nhooyr.io/websocket"
	"strings"
	"tailscale.com/tailcfg"
	"time"
)

// Heartbeat loops to ping a WebSocket to keep it alive.
// Default idle connection timeouts are typically 60 seconds.
// See: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/application-load-balancers.html#connection-idle-timeout
func Heartbeat(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := conn.Ping(ctx)
		if err != nil {
			return
		}
	}
}

// StripGigoCookies removes the session token from the cookie header provided.
func StripGigoCookies(header string) string {
	header = textproto.TrimString(header)
	cookies := []string{}

	var part string
	for len(header) > 0 { // continue since we have rest
		part, header, _ = strings.Cut(header, ";")
		part = textproto.TrimString(part)
		if part == "" {
			continue
		}
		name, _, _ := strings.Cut(part, "=")
		if name == "gigoAuthToken" {
			continue
		}
		cookies = append(cookies, part)
	}
	return strings.Join(cookies, "; ")
}

// GetClusterDerpMap
//
//	Retrieves all the DERP servers available in the cluster
//	and creates a new map of the embedded DERP region.
func GetClusterDerpMap(node cluster.Node) (*tailcfg.DERPMap, error) {
	// create base depr region with no nodes
	region := &tailcfg.DERPRegion{
		EmbeddedRelay: true,
		RegionID:      999,
		RegionCode:    "gigo",
		RegionName:    "Gigo Embedded Relay",
		Nodes:         make([]*tailcfg.DERPNode, 0),
	}

	// retrieve derp configs for each node in the cluster
	clusterDerps, err := node.GetCluster("derp-node")
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster derp nodes: %v", err)
	}

	// iterate through derp configs loading them into derp nodes
	// and appending them to the main derp region
	for _, derpConfig := range clusterDerps {
		if len(derpConfig) == 0 {
			continue
		}
		var derpNode tailcfg.DERPNode
		err := json.Unmarshal([]byte(derpConfig[0].Value), &derpNode)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal derp node: %v", err)
		}
		region.Nodes = append(region.Nodes, &derpNode)
	}

	buf, _ := json.Marshal(region)
	fmt.Println("region: ", string(buf))

	// create a new derp map
	derpMap, err := tailnet.NewDERPMap(
		context.TODO(), region, []string{"stun.l.google.com:19302"}, "", "",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new derp map: %v", err)
	}

	buf, _ = json.Marshal(derpMap)
	fmt.Println("derp map: ", string(buf))

	return derpMap, nil
}
