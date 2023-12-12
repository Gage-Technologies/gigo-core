package external_api

import (
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"io"
	"net/http"
	"strconv"

	"github.com/gage-technologies/gigo-lib/network"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "handle-stripe-webhook-http")
	defer parentSpan.End()

	// read up to 1MiB of the http body
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	// read the clipped body from the http request
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to read body"), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "stripe", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// create strip event to unmarshall into
	event := stripe.Event{}

	// attempt to unmarshall http payload into strip event
	if err := json.Unmarshal(payload, &event); err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to unmarshall stripe event: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "stripe", "", http.StatusBadRequest, "internal server error occurred", err)
		return
	}

	// get signature from header
	signatureHeader := r.Header.Get("Stripe-Signature")

	// format strip event through sdk to validate the signature using our secret
	event, err = webhook.ConstructEvent(payload, signatureHeader, s.stripeWebhookSecret)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate event: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
		return
	}

	if !event.Livemode {
		buf, _ := json.MarshalIndent(event, "", "  ")
		s.logger.Debugf("stripe webhook debug\n:%s", string(buf))
	}

	// unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "invoice.payment_failed":
		err = core.StripeInvoicePaymentFailed(ctx, s.tiDB, event, s.logger)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to handle strip invoice payment failure: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

	// revoke the users subscription status
	case "customer.subscription.deleted":
		err = core.StripeCustomerSubscriptionDeleted(ctx, s.tiDB, event)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to handle strip customer subscription deletion: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}
	case "customer.subscription.paused":
		err = core.StripeCustomerSubscriptionPaused(ctx, s.tiDB, event)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to handle strip customer subscription pause: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}
	case "invoice.payment_action_required":
		err = core.StripeInvoicePaymentActionRequired(ctx, s.tiDB, event, s.logger)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to handle strip invoice payment action required: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}
	case "customer.subscription.resumed":
		err = core.StripeCustomerSubscriptionResumed(ctx, s.tiDB, event)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to handle strip customer subscription resume: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}
	case "checkout.session.completed":
		err := core.StripeCheckoutSessionCompleted(ctx, s.tiDB, s.rdb, event, s.logger)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to handle strip checkout session completed: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}
	default:
	}

	parentSpan.AddEvent(
		"handle-stripe-webhook",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, map[string]interface{}{"message": "webhook succeeded"}, r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "stripe", event.ID, http.StatusOK)
}

func (s *HTTPServer) HandleStripeConnectedWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "handle-stripe-connected-webhook-http")
	defer parentSpan.End()
	callerName := "HandleStripeConnectedWebhook"

	// read up to 64kb of the http body
	r.Body = http.MaxBytesReader(w, r.Body, 65536)

	// read the clipped body from the http request
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to read body"), r.URL.Path, "HandleStripeConnectedWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "stripe", "", http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// create strip event to unmarshall into
	event := stripe.Event{}

	// attempt to unmarshall http payload into strip event
	if err := json.Unmarshal(payload, &event); err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to unmarshall stripe event: %v", err), r.URL.Path, "HandleStripeConnectedWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "stripe", "", http.StatusBadRequest, "internal server error occurred", err)
		return
	}

	// get signature from header
	signatureHeader := r.Header.Get("Stripe-Signature")

	// format strip event through sdk to validate the signature using our secret
	event, err = webhook.ConstructEvent(payload, signatureHeader, s.stripeConnectedWebhookSecret)
	if err != nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("failed to validate event: %v", err), r.URL.Path, "HandleStripeConnectedWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
		return
	}

	// unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "account.updated":
		// create strip invoice to unmarshall into
		var account stripe.Account

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &account)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		if account.ChargesEnabled == true && len(account.Metadata) != 0 {
			// open tx to execute insertions
			tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			// parse post id to integer
			userId, err := strconv.ParseInt(account.Metadata["user_id"], 10, 64)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to convert id to int: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
				tx.Rollback()
			}

			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "update users set stripe_account = ? where _id = ?", account.ID, userId)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to open tx for user subcription resumed: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
				tx.Rollback()
			}
			err = tx.Commit(&callerName)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to finish transation for stripe subscripton resumed: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}
		}

		// revoke the users subscription status
	default:
	}

	parentSpan.AddEvent(
		"handle-stripe-connected-webhook",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, map[string]interface{}{"message": "webhook succeeded"}, r.URL.Path, "HandleStripeConnectedWebhook", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "stripe", event.ID, http.StatusOK)
}

// func HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
//	if r.Method != "POST" {
//		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
//		return
//	}
//	b, err := ioutil.ReadAll(r.Body)
//	if err != nil {
//		http.Error(w, err.Error(), http.StatusBadRequest)
//		log.Printf("ioutil.ReadAll: %v", err)
//		return
//	}
//
//	event, err := webhook.ConstructEvent(b, r.Header.Get("Stripe-Signature"), "we_1MMcSOKRClXv1ERHRlMeHX2a")
//	if err != nil {
//		http.Error(w, err.Error(), http.StatusBadRequest)
//		log.Printf("webhook.ConstructEvent: %v", err)
//		return
//	}
//
//	switch event.Type {
//	case "checkout.session.completed":
//		// Payment is successful and the subscription is created.
//		// You should provision the subscription and save the customer ID to your database.
//	case "invoice.paid":
//		// Continue to provision the subscription as payments continue to be made.
//		// Store the status in your database and check when a user accesses your service.
//		// This approach helps you avoid hitting rate limits.
//	case "invoice.payment_failed":
//		// The payment failed or the customer does not have a valid payment method.
//		// The subscription becomes past_due. Notify your customer and send them to the
//		// customer portal to update their payment information.
//	default:
//		// unhandled event type
//	}
// }
