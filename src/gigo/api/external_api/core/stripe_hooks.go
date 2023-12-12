package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/go-redis/redis/v8"
	"github.com/stripe/stripe-go/v76"
	"go.opentelemetry.io/otel"
)

func StripeInvoicePaymentFailed(ctx context.Context, db *ti.Database, event stripe.Event, logger logging.Logger) error {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "strip-invoice-payment-failed")
	defer parentSpan.End()
	callerName := "StripeInvoicePaymentFailed"

	// create strip invoice to unmarshall into
	var invoice stripe.Invoice

	// unmarshall raw message into stripe invoice
	err := json.Unmarshal(event.Data.Raw, &invoice)
	if err != nil {
		return fmt.Errorf("failed to unmarshall stripe invoice: %v", err)
	}

	if invoice.Subscription != nil {
		// open tx to execute insertions
		tx, err := db.BeginTx(ctx, &parentSpan, &callerName, nil)
		if err != nil {
			return fmt.Errorf("failed to open tx for user subcription revoke: %v", err)
		}
		defer tx.Rollback()

		// remove the subscription and user status in the database
		_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &invoice.Customer.ID)
		if err != nil {
			return fmt.Errorf("failed to update user status: %v", err)
		}

		// commit tx to revoke user subscription
		err = tx.Commit(&callerName)
		if err != nil {
			return fmt.Errorf("failed to commit tx for user subscription revoke: %v", err)
		}
	} else {
		logger.Infof("Payment for %s failed", invoice.Customer.Email)
	}

	return nil
}

func StripeCustomerSubscriptionDeleted(ctx context.Context, db *ti.Database, event stripe.Event) error {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "strip-customer-subscription-deleted")
	defer parentSpan.End()
	callerName := "StripeCustomerSubscriptionDeleted"

	// create strip invoice to unmarshall into
	var subscription stripe.Subscription

	// unmarshall raw message into stripe invoice
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return fmt.Errorf("failed to unmarshall stripe invoice: %v", err)
	}

	// revoke the users subscription status

	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &parentSpan, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to open tx for user subcription revoke: %v", err)
	}
	defer tx.Rollback()

	// remove the subscription and user status in the database
	_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ?, stripe_subscription = null where stripe_user = ?", 0, &subscription.Customer.ID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}

	// commit tx to revoke user subscription
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("failed to commit tx for user subscription revoke: %v", err)
	}

	return nil
}

func StripeCustomerSubscriptionPaused(ctx context.Context, db *ti.Database, event stripe.Event) error {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "strip-customer-subscription-paused")
	defer parentSpan.End()
	callerName := "StripeCustomerSubscriptionPaused"

	// create strip invoice to unmarshall into
	var subscription stripe.Subscription

	// unmarshall raw message into stripe invoice
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return fmt.Errorf("failed to unmarshall stripe invoice: %v", err)
	}

	// revoke the users subscription status

	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &parentSpan, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to open tx for user subcription revoke: %v", err)
	}
	defer tx.Rollback()

	// remove the subscription and user status in the database
	_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &subscription.Customer.ID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}

	// commit tx to revoke user subscription
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("failed to commit tx for user subscription revoke: %v", err)
	}

	return nil
}

func StripeInvoicePaymentActionRequired(ctx context.Context, db *ti.Database, event stripe.Event, logger logging.Logger) error {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "strip-invoice-payment-action-required")
	defer parentSpan.End()
	callerName := "StripeInvoicePaymentActionRequired"

	// create strip invoice to unmarshall into
	var invoice stripe.Invoice

	// unmarshall raw message into stripe invoice
	err := json.Unmarshal(event.Data.Raw, &invoice)
	if err != nil {
		return fmt.Errorf("failed to unmarshall stripe invoice: %v", err)
	}

	// revoke the users subscription status
	if invoice.Subscription != nil {
		// open tx to execute insertions
		tx, err := db.BeginTx(ctx, &parentSpan, &callerName, nil)
		if err != nil {
			return fmt.Errorf("failed to open tx for user subcription revoke: %v", err)
		}
		defer tx.Rollback()

		// remove the subscription and user status in the database
		_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 0, &invoice.Customer.ID)
		if err != nil {
			return fmt.Errorf("failed to update user status: %v", err)
		}

		// commit tx to revoke user subscription
		err = tx.Commit(&callerName)
		if err != nil {
			return fmt.Errorf("failed to commit tx for user subscription revoke: %v", err)
		}
	} else {
		logger.Infof("Payment for %s failed", invoice.Customer.Email)
	}

	return nil
}

func StripeCustomerSubscriptionResumed(ctx context.Context, db *ti.Database, event stripe.Event) error {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "strip-customer-subscription-resumed")
	defer parentSpan.End()
	callerName := "StripeCustomerSubscriptionResumed"

	// create strip invoice to unmarshall into
	var subscription stripe.Subscription

	// unmarshall raw message into stripe invoice
	err := json.Unmarshal(event.Data.Raw, &subscription)
	if err != nil {
		return fmt.Errorf("failed to unmarshall stripe invoice: %v", err)
	}

	// reinstate the users subscription status

	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &parentSpan, &callerName, nil)
	if err != nil {
		return fmt.Errorf("failed to open tx for user subcription revoke: %v", err)
	}
	defer tx.Rollback()

	// remove the subscription and user status in the database
	_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where stripe_user = ?", 1, &subscription.Customer.ID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}

	// commit tx to revoke user subscription
	err = tx.Commit(&callerName)
	if err != nil {
		return fmt.Errorf("failed to commit tx for user subscription revoke: %v", err)
	}

	return nil
}

func StripeCheckoutSessionCompleted(ctx context.Context, db *ti.Database, rdb redis.UniversalClient, event stripe.Event, logger logging.Logger) error {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(ctx, "strip-checkout-session-completed")
	defer parentSpan.End()
	callerName := "StripeCheckoutSessionCompleted"

	// create strip invoice to unmarshall into
	var session stripe.CheckoutSession

	// unmarshall raw message into stripe invoice
	err := json.Unmarshal(event.Data.Raw, &session)
	if err != nil {
		return fmt.Errorf("failed to unmarshall stripe invoice: %v", err)
	}

	if session.Subscription != nil {
		// create a new subscription to premium for the user
		res, err := CreateSubscription(ctx, session.Customer.ID, session.Subscription.ID, db, session.Metadata["user_id"], rdb, session.Metadata["timezone"])
		if err != nil {
			return fmt.Errorf("failed to create premium subscription for user on the core: %v", err)
		}

		if res["subscription"] != "subscription paid" {
			return fmt.Errorf("failed to create premium subscription for user on the core: %+v", res)
		}
	} else {
		// open tx to execute insertions
		tx, err := db.BeginTx(ctx, &parentSpan, &callerName, nil)
		if err != nil {
			return fmt.Errorf("failed to open tx for user subcription revoke: %v", err)
		}
		defer tx.Rollback()

		// parse post id to integer
		userId, err := strconv.ParseInt(session.Metadata["user_id"], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert id to int: %v", err)
		}

		// parse post id to integer
		postId, err := strconv.ParseInt(session.Metadata["post_id"], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert id to int: %v", err)
		}

		_, err = tx.ExecContext(ctx, &callerName, "insert into exclusive_content_purchases (user_id, post, date) values (?, ?, ?)", userId, postId, time.Now())
		if err != nil {
			return fmt.Errorf("failed to insert into exclusive content purchases: %v", err)
		}

		err = tx.Commit(&callerName)
		if err != nil {
			return fmt.Errorf("failed to commit tx for user subscription revoke: %v", err)
		}

		transfer, err := PayOutOnContentPurchase(session.Metadata["connected_account"], session.AmountSubtotal)
		if err != nil {
			return fmt.Errorf("failed to payout on content purchase: %v", err)
		}

		if transfer["transfer"] != "transfer was completed" {
			return fmt.Errorf("failed to payout on content purchase: %+v", transfer)
		}
	}

	return nil
}
