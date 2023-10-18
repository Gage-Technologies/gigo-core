package external_api

import (
	"encoding/json"
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gage-technologies/gigo-lib/network"
	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/webhook"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "handle-stripe-webhook-http")
	defer parentSpan.End()
	callerName := "HandleStripeWebhook"

	// read up to 64kb of the http body
	r.Body = http.MaxBytesReader(w, r.Body, 65536)

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

	// unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	// case "invoice.paid":
	//	// create strip invoice to unmarshall into
	//	var invoice stripe.Invoice
	//
	//	// unmarshall raw message into stripe invoice
	//	err := json.Unmarshal(event.Data.Raw, &invoice)
	//	if err != nil {
	//		// handle error internally
	//		s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//			network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		return
	//	}
	//
	//	// route invoice to the correct core function
	//	if invoice.Subscription != nil {
	//		// create a new subscription to premium for the user
	//		res, err := core.CreateSubscription(ctx, invoice.Customer.ID, invoice.Subscription.ID, s.tiDB, invoice.Metadata["user_id"])
	//		if err != nil {
	//			// handle error internally
	//			s.handleError(w, fmt.Sprintf("failed to create premium subscription for user: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//			return
	//		}
	//
	//		if res["subscription"] != "subscription paid" {
	//			// handle error internally
	//			s.handleError(w, fmt.Sprintf("failed to create premium subscription for user on the core: %v", fmt.Errorf("Core function failed")), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//			return
	//		}
	//	} else {
	//		//postId, err := strconv.ParseInt(invoice.Lines.Data[0].Price.Nickname, 10, 64)
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to adjust post id to int when paying for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//res, err := s.tiDB.DB.Query("select * from post where _id = ?", postId)
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to get post id when paying for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//// ensure closure of cursor
	//		//defer res.Close()
	//		//
	//		//// attempt to load post
	//		//post, err := models.PostFromSQLNative(s.tiDB, res)
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to fully decode post when paying for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//rez, err := s.tiDB.DB.Query("select * from user where email = ?", invoice.Customer.Email)
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to get post id when paying for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//// ensure closure of cursor
	//		//defer rez.Close()
	//		//
	//		//// attempt to load post
	//		//user, err := models.UserFromSQLNative(s.tiDB, rez)
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to fully decode post when paying for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//// create a new attempt
	//		//attempt, err := models.CreateAttempt(s.sf.Generate().Int64(), post.Title, post.Description, user.UserName,
	//		//	user.ID, time.Now(), time.Now(), -1, user.Tier, nil, 0, postId, 0, nil, 0)
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to pay for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//// format attempt for insertion
	//		//insertStatements, err := attempt.ToSQLNative()
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to pay for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//// open tx for attempt insertion
	//		//tx, err := s.tiDB.DB.Begin()
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to pay for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//		//
	//		//defer tx.Rollback()
	//		//
	//		//// iterate over insert statements executing them in sql
	//		//for _, statement := range insertStatements {
	//		//	_, err = tx.Exec(statement.Statement, statement.Values...)
	//		//	if err != nil {
	//		//		s.handleError(w, fmt.Sprintf("failed to pay for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//			network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//		return
	//		//	}
	//		//}
	//		//
	//		//// commit tx
	//		//err = tx.Commit()
	//		//if err != nil {
	//		//	s.handleError(w, fmt.Sprintf("failed to pay for attempt: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//		//		network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		//	return
	//		//}
	//
	//	}
	case "invoice.payment_failed":
		// create strip invoice to unmarshall into
		var invoice stripe.Invoice

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		if invoice.Subscription != nil {
			// open tx to execute insertions
			tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}
			defer tx.Rollback()

			// remove the subscription and user status in the database
			_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &invoice.Customer.ID)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to revoke user subscription: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			// commit tx to revoke user subscription
			err = tx.Commit(&callerName)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to commit tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}
		} else {
			s.logger.Infof("Payment for %s failed", invoice.Customer.Email)
		}

		// revoke the users subscription status
	case "customer.subscription.deleted":
		// create strip invoice to unmarshall into
		var subscription stripe.Subscription

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		// revoke the users subscription status

		// open tx to execute insertions
		tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
		defer tx.Rollback()

		// remove the subscription and user status in the database
		_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &subscription.Customer.ID)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to revoke user subscription: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		// commit tx to revoke user subscription
		err = tx.Commit(&callerName)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to commit tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
	case "customer.subscription.paused":
		// create strip invoice to unmarshall into
		var subscription stripe.Subscription

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		// revoke the users subscription status

		// open tx to execute insertions
		tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
		defer tx.Rollback()

		// remove the subscription and user status in the database
		_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &subscription.Customer.ID)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to revoke user subscription: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		// commit tx to revoke user subscription
		err = tx.Commit(&callerName)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to commit tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}
	case "invoice.payment_action_required":
		// create strip invoice to unmarshall into
		var invoice stripe.Invoice

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		if invoice.Subscription != nil {
			// open tx to execute insertions
			tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}
			defer tx.Rollback()

			// remove the subscription and user status in the database
			_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &invoice.Customer.ID)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to revoke user subscription: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			// commit tx to revoke user subscription
			err = tx.Commit(&callerName)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to commit tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}
		} else {
			s.logger.Infof("Payment for %s failed", invoice.Customer.Email)
		}
	//case "customer.subscription.updated":
	//	// create strip invoice to unmarshall into
	//	var subscription stripe.Subscription
	//
	//	// unmarshall raw message into stripe invoice
	//	err := json.Unmarshal(event.Data.Raw, &subscription)
	//	if err != nil {
	//		// handle error internally
	//		s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//			network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
	//		return
	//	}
	//
	//	if subscription.PauseCollection != nil {
	//		// open tx to execute insertions
	//		tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
	//		if err != nil {
	//			// handle error internally
	//			s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
	//			return
	//		}
	//		defer tx.Rollback()
	//
	//		// remove the subscription and user status in the database
	//		_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &subscription.Customer.ID)
	//		if err != nil {
	//			// handle error internally
	//			s.handleError(w, fmt.Sprintf("failed to revoke user subscription: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
	//			return
	//		}
	//
	//		// commit tx to revoke user subscription
	//		err = tx.Commit(&callerName)
	//		if err != nil {
	//			// handle error internally
	//			s.handleError(w, fmt.Sprintf("failed to commit tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
	//				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
	//			return
	//		}
	//	}
	case "customer.subscription.resumed":
		// create strip invoice to unmarshall into
		var subscription stripe.Subscription

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		// open tx to execute insertions
		tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
			return
		}

		// increment tag column usage_count in database
		_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 1, subscription.Customer.ID)
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
	case "checkout.session.completed":
		// create strip invoice to unmarshall into
		var session stripe.CheckoutSession

		// unmarshall raw message into stripe invoice
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to unmarshall stripe invoice: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
			return
		}

		if session.Subscription != nil {
			// create a new subscription to premium for the user
			res, err := core.CreateSubscription(ctx, session.Customer.ID, session.Subscription.ID, s.tiDB, session.Metadata["user_id"], s.rdb, session.Metadata["timezone"])
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to create premium subscription for user: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
				return
			}

			if res["subscription"] != "subscription paid" {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to create premium subscription for user on the core: %v", fmt.Errorf("Core function failed")), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusBadRequest, "internal server error occurred", err)
				return
			}
		} else {
			// open tx to execute insertions
			tx, err := s.tiDB.BeginTx(ctx, &parentSpan, &callerName, nil)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to open tx for user subcription revoke: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			// parse post id to integer
			userId, err := strconv.ParseInt(session.Metadata["user_id"], 10, 64)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to convert id to int: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
				tx.Rollback()
			}

			// parse post id to integer
			postId, err := strconv.ParseInt(session.Metadata["post_id"], 10, 64)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to convert id to int: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
				tx.Rollback()
			}

			_, err = tx.ExecContext(ctx, &callerName, "insert into exclusive_content_purchases (user_id, post, date) values (?, ?, ?)", userId, postId, time.Now())
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to execute update for stripe purchaes: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			err = tx.Commit(&callerName)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to finish transation for stripe db payment: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			transfer, err := core.PayOutOnContentPurchase(session.Metadata["connected_account"], session.AmountSubtotal)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to finish transation for stripe purchase payout: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}

			if transfer["transfer"] != "transfer was completed" {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to actually confirm payout success: %v", err), r.URL.Path, "HandleStripeWebhook", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), "stripe", event.ID, http.StatusInternalServerError, "internal server error occurred", err)
				return
			}
		}

		// revoke the users subscription status
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
