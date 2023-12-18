package core

import (
	"context"
	"fmt"
	"gigo-core/gigo/config"
	"log"
	"strconv"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/now"
	"github.com/kisielk/sqlstruct"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/account"
	"github.com/stripe/stripe-go/v76/accountlink"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutSession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/invoiceitem"
	"github.com/stripe/stripe-go/v76/loginlink"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/product"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/transfer"
	"go.opentelemetry.io/otel"
)

type Product struct {
	ProductID string
	PriceID   string
	Price     int64
}

func CreateCustomer(ctx context.Context, callingUser *models.User, db *ti.Database) (*string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-customer-core")
	defer span.End()
	callerName := "CreateCustomer"

	name := callingUser.FirstName + " " + callingUser.LastName

	params := &stripe.CustomerParams{
		Name:  &name,
		Email: &callingUser.Email,
	}
	c, err := customer.New(params)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction failed: %s", err.Error())
	}
	defer tx.Rollback()

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update users set stripe_user = ? where _id = ?", c.ID, callingUser.UserName)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return &c.ID, nil
}

func CreateProduct(ctx context.Context, cost int64, db *ti.Database, postId int64, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-product-core")
	defer span.End()
	callerName := "CreateProduct"

	// create a stripe product using the post id as a name

	name := fmt.Sprintf("%v", postId)
	params := &stripe.ProductParams{
		Name: stripe.String(name),
	}
	newProduct, err := product.New(params)
	if err != nil {
		return nil, fmt.Errorf("Error creating product: %s", err)
	}

	// create the pricing structure for the stripe product
	priceParams := &stripe.PriceParams{
		Product:    stripe.String(newProduct.ID),
		UnitAmount: stripe.Int64(cost),
		Currency:   stripe.String(string(stripe.CurrencyUSD)),
	}
	priceFinal, err := price.New(priceParams)
	if err != nil {
		return map[string]interface{}{"product": "error creating product"}, fmt.Errorf("Error creating price: %s", err)
	}

	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("Error starting transaction failed: %s", err.Error())
	}
	defer tx.Rollback()

	priceString := fmt.Sprintf("%v", cost)

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update post set challenge_cost = ?, stripe_price_id = ? where _id = ?", priceString, priceFinal.ID, postId)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
		tx.Rollback()
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return map[string]interface{}{"message": "Product has been created."}, nil
}

func GetProjectPriceId(ctx context.Context, postId int64, db *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-project-price-id-core")
	defer span.End()
	callerName := "GetProjectPriceId"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := db.QueryContext(ctx, &span, &callerName, "select stripe_price_id from post where _id = ?", postId)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. GetCommentSideThread Core.    Error: %v", err)
	}

	var priceId *string

	defer res.Close()

	for res.Next() {
		err = sqlstruct.Scan(&priceId, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. GetCommentSideThread Core.    Error: %v", err)
		}
	}

	return map[string]interface{}{"priceId": &priceId}, nil
}

func UpdateClientPayment(ctx context.Context, callingUser *models.User, db *ti.Database, sourceToken string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-client-payment-core")
	defer span.End()

	params := &stripe.CustomerParams{Source: stripe.String(sourceToken)}
	_, err := customer.Update(*callingUser.StripeUser, params)
	if err != nil {
		return map[string]interface{}{"message": "Error updating card"}, nil
	}

	return map[string]interface{}{"message": "Card has been updated."}, nil
}

func CreatePortalSession(ctx context.Context, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-portal-session-core")
	defer span.End()

	if callingUser.StripeUser == nil {
		return map[string]interface{}{"message": "User not found."}, nil
	}

	params := &stripe.BillingPortalSessionParams{
		Customer: stripe.String(*callingUser.StripeUser),
	}

	s, err := session.New(params)
	if err != nil {
		return map[string]interface{}{"message": "Error creating session"}, nil
	}

	return map[string]interface{}{"session": s.URL}, nil
}

func CreateStripeCustomer(ctx context.Context, name string, email string) (string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-stripe-customer")
	defer span.End()

	stripeUser := &stripe.CustomerParams{
		Email: &email,
		Name:  &name,
	}

	result, err := customer.New(stripeUser)
	if err != nil {
		return "", err
	}

	return result.ID, nil
}

func DeleteStripeCustomer(ctx context.Context, id string) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "delete-stripe-customer")
	defer span.End()

	_, err := customer.Del(id, nil)
	if err != nil {
		return err
	}

	return nil
}

func CreateTrialSubscription(ctx context.Context, monthlyPriceID string, email string, db *ti.Database, tx *ti.Tx, id int64, firstName string, lastName string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-trial-subscription")
	defer span.End()
	callerName := "CreateTrialSubscription"

	fullName := firstName + "-" + lastName

	stripeUser := &stripe.CustomerParams{
		Email: &email,
		Name:  &fullName,
	}

	result, err := customer.New(stripeUser)
	if err != nil {
		return map[string]interface{}{"message": "Error creating customer"}, err
	}

	now := time.Now()

	oneMonthFromNow := now.AddDate(0, 1, 0)
	unixTimestamp := oneMonthFromNow.Unix()

	missingPayment := "cancel"
	subscriptionParams := &stripe.SubscriptionParams{
		Customer: stripe.String(result.ID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(monthlyPriceID)},
		},
		TrialEnd: stripe.Int64(unixTimestamp),
		TrialSettings: &stripe.SubscriptionTrialSettingsParams{
			EndBehavior: &stripe.SubscriptionTrialSettingsEndBehaviorParams{
				MissingPaymentMethod: &missingPayment,
			},
		},
	}

	subscriptions, err := subscription.New(subscriptionParams)
	if err != nil {
		return map[string]interface{}{"subscription": "not cancelled"}, err
	}

	// increment tag column usage_count in database
	if tx != nil {
		_, err = tx.ExecContext(ctx, &callerName, "update users set stripe_user = ?, stripe_subscription = ?, user_status = ? where _id = ?", result.ID, subscriptions.ID, 1, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update user with trial subscription: %v", err)
		}
	} else {
		_, err = db.Exec(ctx, &span, &callerName, "update users set stripe_user = ?, stripe_subscription = ?, user_status = ? where _id = ?", result.ID, subscriptions.ID, 1, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update user with trial subscription: %v", err)
		}
	}

	return map[string]interface{}{"response": "stripe trial is started"}, nil
}

func CreateTrialSubscriptionReferral(ctx context.Context, monthlyPriceID string, email string, db *ti.Database, tx *ti.Tx, id int64, firstName string, lastName string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-trial-subscription")
	defer span.End()
	callerName := "CreateTrialSubscription"

	fullName := firstName + "-" + lastName

	stripeUser := &stripe.CustomerParams{
		Email: &email,
		Name:  &fullName,
	}

	result, err := customer.New(stripeUser)
	if err != nil {
		return map[string]interface{}{"message": "Error creating customer"}, err
	}

	now := time.Now()

	oneMonthFromNow := now.AddDate(0, 2, 0)
	unixTimestamp := oneMonthFromNow.Unix()

	subscriptionParams := &stripe.SubscriptionParams{
		Customer: stripe.String(result.ID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(monthlyPriceID)},
		},
		TrialEnd: stripe.Int64(unixTimestamp),
		TrialSettings: &stripe.SubscriptionTrialSettingsParams{
			EndBehavior: &stripe.SubscriptionTrialSettingsEndBehaviorParams{
				MissingPaymentMethod: stripe.String("cancel"),
			},
		},
	}

	subscriptions, err := subscription.New(subscriptionParams)
	if err != nil {
		return map[string]interface{}{"subscription": "not cancelled"}, err
	}

	// increment tag column usage_count in database
	if tx != nil {
		_, err = tx.ExecContext(ctx, &callerName, "update users set stripe_user = ?, stripe_subscription = ?, user_status = ? where _id = ?", result.ID, subscriptions.ID, 1, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update user with trial subscription: %v", err)
		}
	} else {
		_, err = db.Exec(ctx, &span, &callerName, "update users set stripe_user = ?, stripe_subscription = ?, user_status = ? where _id = ?", result.ID, subscriptions.ID, 1, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update user with trial subscription: %v", err)
		}
	}

	return map[string]interface{}{"response": "stripe trial is started"}, nil
}

func CancelSubscription(ctx context.Context, db *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "cancel-subscription-core")
	defer span.End()
	callerName := "CancelSubscription"

	cancel := true
	subscriptionParams := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: &cancel,
	}

	_, err := subscription.Update(*callingUser.StripeSubscription, subscriptionParams)
	if err != nil {
		return map[string]interface{}{"subscription": "not cancelled"}, err
	}

	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction failed: %s", err.Error())
	}
	defer tx.Rollback()

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update users set user_status = ? where _id = ?", 0, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return map[string]interface{}{"subscription": "cancelled"}, nil
}

func CreateSubscription(ctx context.Context, stripeUser string, subscription string, trial bool, db *ti.Database, id string, rdb redis.UniversalClient, timeZone string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-subscription")
	defer span.End()
	callerName := "CreateSubscription"

	fmt.Println("made it to create subscription core: ", id)
	// open tx to execute insertions
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction failed: %s", err.Error())
	}
	defer tx.Rollback()

	userId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user id: %v", err)
	}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update users set stripe_user = ?, stripe_subscription = ?, user_status = ?, used_free_trial = greatest(used_free_trial, ?) where _id = ?", stripeUser, subscription, 1, trial, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// get the timezone for the time

	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone location: %v", err)
	}

	// get the current time in that timezone
	currentTime := time.Now().In(location)

	// get the start of the following week in that timezone
	startTime := now.BeginningOfWeek().In(location).Add(time.Hour * 168)

	// subtract the times to get the duration until the start of the following week
	finalTime := startTime.Sub(currentTime)

	keyName := "premium-streak-freeze-" + id

	_ = rdb.Set(ctx, keyName, userId, finalTime)

	return map[string]interface{}{"subscription": "subscription paid"}, nil
}

func UpdateConnectedAccount(callingUser *models.User) (map[string]interface{}, error) {

	// todo change these on production
	param := &stripe.LoginLinkParams{
		Account: stripe.String(*callingUser.StripeAccount),
	}

	result, err := loginlink.New(param)
	if err != nil {
		return map[string]interface{}{"message": "Error updating account link"}, fmt.Errorf("error updating account link stripe with prior info: %v", err)
	}

	return map[string]interface{}{"account": result.URL}, err
}

func CreateConnectedAccount(ctx context.Context, callingUser *models.User, challenge bool) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-connected-account")
	defer span.End()

	params := &stripe.AccountParams{Type: stripe.String(string(stripe.AccountTypeExpress))}
	callingUserId := fmt.Sprintf("%v", callingUser.ID)
	params.AddMetadata("user_id", callingUserId)
	account, err := account.New(params)
	if err != nil {
		return map[string]interface{}{"message": "Error creating account"}, fmt.Errorf("error creating account stripe: %v", err)
	}

	returnString := ""

	if challenge {
		returnString = "https://www.gigo.dev/create-project"
	} else {
		returnString = "https://www.gigo.dev/home"
	}

	// todo change these on production
	param := &stripe.AccountLinkParams{
		Account:    stripe.String(account.ID),
		RefreshURL: stripe.String("https://www.gigo.dev/reauth"),
		ReturnURL:  stripe.String(returnString),
		Type:       stripe.String("account_onboarding"),
	}

	result, err := accountlink.New(param)
	if err != nil {
		return map[string]interface{}{"message": "Error creating account link"}, fmt.Errorf("error creating account link stripe with prior info: %v", err)
	}

	return map[string]interface{}{"account": result.URL}, err
}

func PayOutOnContentPurchase(accountId string, amount int64) (map[string]interface{}, error) {

	finalAmount := int64(float32(amount) * .8)
	params := &stripe.TransferParams{
		Amount:      stripe.Int64(finalAmount),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Destination: stripe.String(accountId),
	}
	_, err := transfer.New(params)
	if err != nil {
		return map[string]interface{}{"message": "Error creating transfer"}, fmt.Errorf("error creating transfer stripe: %v", err)
	}

	return map[string]interface{}{"transfer": "transfer was completed"}, nil
}

func StripeCheckoutSession(ctx context.Context, priceId string, postId string, callingUser *models.User, db *ti.Database) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "stripe-checkout-session")
	defer span.End()
	callerName := "StripeCheckoutSession"

	successUrl := "https://www.gigo.dev/success"

	cancelUrl := "https://www.gigo.dev/canceled"

	// parse post repo id to integer
	postQuery, err := strconv.ParseInt(postId, 10, 64)
	if err != nil {
		return map[string]interface{}{"message": "Error parsing post id"}, fmt.Errorf("Unable to turn post id into an int64: %v", err)
	}

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := db.QueryContext(ctx, &span, &callerName, "select stripe_account from post p left join users u on p.author_id = u._id where p._id = ?", postQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
	}

	defer res.Close()

	var dataObject string

	for res.Next() {
		// attempt to load count from row
		err = res.Scan(&dataObject)
		if err != nil {
			return nil, fmt.Errorf("failed to get follower count: %v", err)
		}
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL: &successUrl,
		CancelURL:  &cancelUrl,
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			&stripe.CheckoutSessionLineItemParams{
				Price: stripe.String(priceId),
				// For metered billing, do not pass quantity
				Quantity: stripe.Int64(1),
			},
		},
	}

	params.AddMetadata("post_id", postId)
	callingUserId := fmt.Sprintf("%v", callingUser.ID)
	params.AddMetadata("user_id", callingUserId)
	params.AddMetadata("connected_account", dataObject)

	s, err := checkoutSession.New(params)
	if err != nil {
		return map[string]interface{}{"message": "Error creating session"}, fmt.Errorf("error creating stripe session: %v", err)
	}

	return map[string]interface{}{"return url": s.URL}, nil
}

func StripePremiumMembershipSession(monthlyPriceID string, yearlyPriceID string, callingUser *models.User) (map[string]interface{}, error) {
	successUrl := "https://www.gigo.dev/successMembership"
	cancelUrl := "https://www.gigo.dev/cancel"

	trialParams := stripe.CheckoutSessionSubscriptionDataParams{}
	if !callingUser.UsedFreeTrial {
		trialParams.TrialPeriodDays = stripe.Int64(30)
		trialParams.TrialSettings = &stripe.CheckoutSessionSubscriptionDataTrialSettingsParams{
			EndBehavior: &stripe.CheckoutSessionSubscriptionDataTrialSettingsEndBehaviorParams{
				MissingPaymentMethod: stripe.String("cancel"),
			},
		}
	}

	params := &stripe.CheckoutSessionParams{
		SuccessURL: &successUrl,
		CancelURL:  &cancelUrl,
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price: stripe.String(monthlyPriceID),
				// For metered billing, do not pass quantity
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &trialParams,
	}
	callingUserId := fmt.Sprintf("%v", callingUser.ID)
	params.AddMetadata("user_id", callingUserId)
	params.AddMetadata("timezone", callingUser.Timezone)

	s, err := checkoutSession.New(params)
	if err != nil {
		return map[string]interface{}{"message": "Error creating session"}, fmt.Errorf("error creating stripe session: %v", err)
	}

	paramz := &stripe.CheckoutSessionParams{
		SuccessURL: &successUrl,
		CancelURL:  &cancelUrl,
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price: stripe.String(yearlyPriceID),
				// For metered billing, do not pass quantity
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &trialParams,
	}
	paramz.AddMetadata("user_id", callingUserId)
	paramz.AddMetadata("timezone", callingUser.Timezone)

	z, err := checkoutSession.New(paramz)
	if err != nil {
		return map[string]interface{}{"message": "Error creating session"}, fmt.Errorf("error creating stripe session: %v", err)
	}

	return map[string]interface{}{"return url": s.URL, "return year": z.URL}, nil
}

func FreeMonthUpdate(callingUser *models.User, tidb *ti.Database, stripeSubConfig config.StripeSubscriptionConfig, ctx context.Context) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "free-mont-referral-core")
	defer span.End()
	callerName := "FreeMonthUpdate"

	var subscriptionID string

	if callingUser.StripeSubscription != nil {
		subscriptionID = *callingUser.StripeSubscription // Replace with the actual subscription ID
	} else {
		return map[string]interface{}{"message": "Error with free month, no current subscription"}, fmt.Errorf("error creating stripe free trial, no subscription")
	}

	subscriptions, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		log.Fatalf("Failed to retrieve subscription: %v\n", err)
	}

	if len(subscriptions.Items.Data) > 0 && subscriptions.Items.Data[0].Plan.Interval == "year" {
		// Amount in cents to credit ($15)
		amountToCredit := -1500

		// Create a negative invoice item
		params := &stripe.InvoiceItemParams{
			Customer: stripe.String(subscriptions.Customer.ID),
			Amount:   stripe.Int64(int64(amountToCredit)),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
		}
		_, err := invoiceitem.New(params)
		if err != nil {
			log.Fatalf("Failed to create invoice item: %v\n", err)
		}
	} else {
		// Check if the subscription is in trial
		inTrial := subscriptions.TrialEnd > 0 && subscriptions.TrialEnd > time.Now().Unix()
		if inTrial {
			trialEnd := subscriptions.TrialEnd + (30 * 24 * 60 * 60)

			params := &stripe.SubscriptionParams{
				TrialEnd: stripe.Int64(trialEnd),
			}
			_, err := subscription.Update(subscriptionID, params)
			if err != nil {
				return map[string]interface{}{"message": "Error creating free trial"}, fmt.Errorf("error creating stripe free trial: %v", err)
			}
		} else {
			if callingUser.UserStatus == 1 {
				// Calculate the Unix timestamp for when you want the trial to end.
				// For example, to set a 30-day trial:
				trialEnd := time.Now().AddDate(0, 1, 0).Unix() // Ends 1 month from now

				params := &stripe.SubscriptionParams{
					PauseCollection: &stripe.SubscriptionPauseCollectionParams{
						Behavior:  stripe.String(string(stripe.SubscriptionPauseCollectionBehaviorVoid)),
						ResumesAt: stripe.Int64(trialEnd),
					},
				}
				_, err := subscription.Update(subscriptionID, params)
				if err != nil {
					return map[string]interface{}{"message": "Error creating free trial"}, fmt.Errorf("error creating stripe free trial: %v", err)
				}
			} else {
				if subscriptions.Status == "canceled" {
					_, err = CreateTrialSubscription(ctx, stripeSubConfig.MonthlyPriceID, callingUser.Email, tidb, nil, callingUser.ID, callingUser.FirstName, callingUser.LastName)
					if err != nil {
						return map[string]interface{}{"message": "Error creating free trial in referral"}, fmt.Errorf("error creating stripe free trial in referral: %v", err)
					}
				} else {
					trialStart := time.Now()
					trialEnd := trialStart.Add(time.Hour * 24 * 30) // This will add 30 days to the current time

					params := &stripe.SubscriptionParams{
						TrialEnd: stripe.Int64(trialEnd.Unix()), // Convert time.Time to Unix timestamp
					}
					_, err := subscription.Update(subscriptionID, params)
					if err != nil {
						return map[string]interface{}{"message": "Error creating free trial"}, fmt.Errorf("error creating stripe free trial: %v", err)
					}

					_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set user_status = ? where _id = ?", 1, callingUser.ID)
					if err != nil {
						return map[string]interface{}{"message": "Error creating free trial, updating user"}, fmt.Errorf("error creating stripe free trial udpating user: %v", err)
					}
				}
			}
		}
	}

	return map[string]interface{}{"subscription": "free month"}, nil
}

func FreeMonthReferral(stripeSubConfig config.StripeSubscriptionConfig, subscriptionId *string, userStatus int, referralUserId int64, tidb *ti.Database, ctx context.Context, logger logging.Logger, firstName string, lastName string, email string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "free-mont-referral-core")
	defer span.End()
	callerName := "FreeMonthReferral"

	// if the subscription doesn't exist then we need to create
	if subscriptionId == nil {
		_, err := CreateTrialSubscriptionReferral(ctx, stripeSubConfig.MonthlyPriceID, email, tidb, nil, referralUserId, firstName, lastName)
		if err != nil {
			return nil, fmt.Errorf("failed to create trail subscription for referral: %v", err)
		}
		return map[string]interface{}{"subscription": "free month"}, nil
	}

	subscriptions, err := subscription.Get(*subscriptionId, nil)
	if err != nil {
		log.Fatalf("Failed to retrieve subscription: %v\n", err)
	}

	if len(subscriptions.Items.Data) > 0 && subscriptions.Items.Data[0].Plan.Interval == "year" {
		// Amount in cents to credit ($15)
		amountToCredit := -1500

		// Create a negative invoice item
		params := &stripe.InvoiceItemParams{
			Customer: stripe.String(subscriptions.Customer.ID),
			Amount:   stripe.Int64(int64(amountToCredit)),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
		}
		_, err := invoiceitem.New(params)
		if err != nil {
			log.Fatalf("Failed to create invoice item: %v\n", err)
		}
	} else {
		logger.Infof("free month referral, in updates")
		// Check if the subscription is in trial
		inTrial := subscriptions.TrialEnd > 0 && subscriptions.TrialEnd > time.Now().Unix()
		if inTrial {
			logger.Infof("free month referral, in trial")
			trialEnd := subscriptions.TrialEnd + (30 * 24 * 60 * 60)

			params := &stripe.SubscriptionParams{
				TrialEnd: stripe.Int64(trialEnd),
			}
			_, err := subscription.Update(*subscriptionId, params)
			if err != nil {
				return map[string]interface{}{"message": "Error creating free trial"}, fmt.Errorf("error creating stripe free trial: %v", err)
			}

			_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set user_status = ? where _id = ?", 1, referralUserId)
			if err != nil {
				return map[string]interface{}{"message": "Error creating free trial, updating user"}, fmt.Errorf("error creating stripe free trial udpating user: %v", err)
			}
		} else {
			logger.Infof("free month referral, out of trial")
			if userStatus == 1 {
				logger.Infof("free month referral, premium status")
				trialEnd := time.Now().AddDate(0, 1, 0).Unix() // Ends 1 month from now

				params := &stripe.SubscriptionParams{
					PauseCollection: &stripe.SubscriptionPauseCollectionParams{
						Behavior:  stripe.String(string(stripe.SubscriptionPauseCollectionBehaviorVoid)),
						ResumesAt: stripe.Int64(trialEnd),
					},
				}
				_, err := subscription.Update(*subscriptionId, params)
				if err != nil {
					return map[string]interface{}{"message": "Error creating free trial"}, fmt.Errorf("error creating stripe free trial: %v", err)
				}
			} else {
				if subscriptions.Status == "canceled" {
					logger.Infof("free month referral, cancelled")
					logger.Infof("user id: %v", referralUserId)
					_, err = CreateTrialSubscription(ctx, stripeSubConfig.MonthlyPriceID, email, tidb, nil, referralUserId, firstName, lastName)
					if err != nil {
						return map[string]interface{}{"message": "Error creating free trial in referral"}, fmt.Errorf("error creating stripe free trial in referral: %v", err)
					}
				} else {
					logger.Infof("free month referral, pleb status but not cancelled")
					logger.Infof("user id: %v", referralUserId)
					trialStart := time.Now()
					trialEnd := trialStart.Add(time.Hour * 24 * 31) // This will add 30 days to the current time

					params := &stripe.SubscriptionParams{
						TrialEnd: stripe.Int64(trialEnd.Unix()), // Convert time.Time to Unix timestamp
					}
					_, err := subscription.Update(*subscriptionId, params)
					if err != nil {
						return map[string]interface{}{"message": "Error creating free trial"}, fmt.Errorf("error creating stripe free trial: %v", err)
					}

					_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set user_status = ? where _id = ?", 1, referralUserId)
					if err != nil {
						return map[string]interface{}{"message": "Error creating free trial, updating user"}, fmt.Errorf("error creating stripe free trial udpating user: %v", err)
					}
				}
			}
		}
	}

	return map[string]interface{}{"subscription": "free month"}, nil
}

//
// func StripePortalSession(customerID string) (map[string]interface{}, error) {
//
//	// The URL to which the user is redirected when they are done managing
//	// billing in the portal.
//	returnURL := "/settings"
//
//	params := &stripe.BillingPortalSessionParams{
//		Customer:  stripe.String(customerID),
//		ReturnURL: stripe.String(returnURL),
//	}
//	ps, _ := portalsession.New(params)
//
//	return map[string]interface{}{"portal session url": ps.URL}, nil
// }
