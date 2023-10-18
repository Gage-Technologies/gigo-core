// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Copied and modified from https://github.com/tailscale/tailscale/blob/fdc2018d67e65956a2424349c27d742b466d8b60/derp/derphttp/derphttp_server.go

package derper

import (
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"log"
	"net/http"
	"strings"

	"tailscale.com/derp"
)

// fastStartHeader is the header (with value "1") that signals to the HTTP
// server that the DERP HTTP client does not want the HTTP 101 response
// headers and it will begin writing & reading the DERP protocol immediately
// following its HTTP request.
const fastStartHeader = "Derp-Fast-Start"

func Handler(s *MeshServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "derper-handler-http")
		defer parentSpan.End()

		up := strings.ToLower(r.Header.Get("Upgrade"))
		if up != "websocket" && up != "derp" {
			if up != "" {
				log.Printf("Weird upgrade: %q", up)
			}
			http.Error(w, "DERP requires connection upgrade", http.StatusUpgradeRequired)
			return
		}

		fastStart := r.Header.Get(fastStartHeader) == "1"

		h, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "HTTP does not support general TCP support", 500)
			return
		}

		netConn, conn, err := h.Hijack()
		if err != nil {
			log.Printf("Hijack failed: %v", err)
			http.Error(w, "HTTP does not support general TCP support", 500)
			return
		}

		if !fastStart {
			pubKey := s.PublicKey()
			fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\n"+
				"Upgrade: DERP\r\n"+
				"Connection: Upgrade\r\n"+
				"Derp-Version: %v\r\n"+
				"Derp-Public-Key: %s\r\n\r\n",
				derp.ProtocolVersion,
				pubKey.UntypedHexString())
		}

		parentSpan.AddEvent(
			"derper-handler",
			trace.WithAttributes(
				attribute.Bool("success", true),
			),
		)

		s.Accept(r.Context(), netConn, conn, netConn.RemoteAddr().String())
	})
}
