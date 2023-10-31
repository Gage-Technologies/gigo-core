package external_api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"io"
	"net/http"

	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gage-technologies/gigo-lib/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TODO: needs testing

func (s *HTTPServer) GiteaWebhookPush(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "gitea-webhook-push-http")
	defer parentSpan.End()

	// retrieve delivery id
	deliveryId := r.Header.Get("X-Gitea-Delivery")

	// read the request body
	sigRequestBody, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		s.handleError(w, "failed to read body of signature request clone", r.URL.Path, "GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "git", deliveryId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// create a new read closed and assign the body to it to preserve the default logic of the jsonRequest function
	r.Body = io.NopCloser(bytes.NewBuffer(sigRequestBody))

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "GiteaWebhookPush", false, "git", -1)
	if reqJson == nil {
		return
	}

	// martial request json and attempt to unmarshall into types.GiteaWebhookPush
	var push types.GiteaWebhookPush
	buf, err := json.Marshal(reqJson)
	if err != nil {
		s.handleError(w, "failed to marshall request", r.URL.Path, "GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "git", deliveryId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	err = json.Unmarshal(buf, &push)
	if err != nil {
		s.handleError(w, "failed to unmarshall request", r.URL.Path, "GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "git", deliveryId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	// calculate signature
	sig256 := hmac.New(sha256.New, []byte(s.gitWebhookSecret))
	_, err = sig256.Write(sigRequestBody)
	if err != nil {
		s.handleError(w, "failed to write payload to signature hashed", r.URL.Path,
			"GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "git", deliveryId, http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}
	sig := hex.EncodeToString(sig256.Sum(nil))

	// validate git webhook signature with local signature
	secret := r.Header.Get("X-Gitea-Signature")
	if secret != sig {
		s.handleError(w, fmt.Sprintf("signature check failed %s != %s", secret, sig), r.URL.Path, "GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "git", "", http.StatusUnauthorized, "signature check failed", nil)
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "git", deliveryId, http.StatusOK)
		return
	}

	// execute core function logic
	err = core.GiteaWebhookPush(ctx, s.tiDB, s.vscClient, s.sf, s.wg, s.logger, &push)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", nil)
		// handle error internally
		s.handleError(w, "GiteaWebhookPush core failed", r.URL.Path, "WorkspaceAFK", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "git", deliveryId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"gitea-webhook-push",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	s.jsonResponse(r, w, map[string]interface{}{"message": "success"}, r.URL.Path, "GiteaWebhookPush", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "git", deliveryId, http.StatusOK)

}
