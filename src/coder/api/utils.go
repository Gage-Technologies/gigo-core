package api

import (
	"context"
	"net/textproto"
	"strings"
	"time"

	"nhooyr.io/websocket"
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
