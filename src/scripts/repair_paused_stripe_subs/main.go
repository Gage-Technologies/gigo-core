package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/subscription"
	"gopkg.in/yaml.v2"
)

type Config struct {
	TitaniumConfig config.TitaniumConfig `yaml:"tidb"`
	StripeKey      string                `yaml:"stripe_key"`
}

func repairUser(db *ti.Database, user *models.User) {
	// check if the user has a subscription
	if user.StripeSubscription == nil {
		return
	}

	// get subscription info from stripe
	s, err := subscription.Get(*user.StripeSubscription, nil)
	if err != nil {
		log.Fatalf("failed to get subscription: %v", err)
	}

	trialActive := s.TrialStart > 0 && time.Now().Unix() < s.TrialEnd

	if trialActive {
		params := stripe.SubscriptionParams{
			TrialSettings: &stripe.SubscriptionTrialSettingsParams{
				EndBehavior: &stripe.SubscriptionTrialSettingsEndBehaviorParams{
					MissingPaymentMethod: stripe.String("cancel"),
				},
			},
		}

		_, err = subscription.Update(*user.StripeSubscription, &params)
		if err != nil {
			log.Fatalf("failed to update subscription: %v", err)
		}

		fmt.Printf("Repaired Active Trial: %d\n", user.ID)
		return
	}

	// Retrieve the latest invoice
	inv, err := invoice.Get(s.LatestInvoice.ID, nil)
	if err != nil {
		log.Fatal("failed to get latest invoice: %v", err)
	}

	paused := inv.PaymentIntent == nil

	if !paused {
		// Check the payment intent status
		pi, err := paymentintent.Get(inv.PaymentIntent.ID, nil)
		if err != nil {
			log.Fatal("failed to get latest payment intent: %v", err)
		}

		// Determine if the payment failed or requires action
		paused = pi.Status == stripe.PaymentIntentStatusRequiresPaymentMethod || pi.Status == stripe.PaymentIntentStatusRequiresAction
	}

	if paused {
		_, err = subscription.Cancel(*user.StripeSubscription, nil)
		if err != nil {
			fmt.Printf("ERROR: failed to cancel subscription: %v\n", err)
		}

		_, err = db.DB.Exec("UPDATE users SET stripe_subscription = NULL WHERE _id = ?", user.ID)
		if err != nil {
			log.Fatalf("failed to update user subscription: %v", err)
		}
	}

	fmt.Printf("Subscription Paused: %d - %v\n", user.ID, paused)
}

func main() {
	configPath := flag.String("c", "config.yml", "Path to the configuration file")
	flag.Parse()

	cfgBuf, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}

	var cfg Config
	err = yaml.Unmarshal(cfgBuf, &cfg)
	if err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

	stripe.Key = cfg.StripeKey

	db, err := ti.CreateDatabase(cfg.TitaniumConfig.TitaniumHost, cfg.TitaniumConfig.TitaniumPort, "mysql", cfg.TitaniumConfig.TitaniumUser,
		cfg.TitaniumConfig.TitaniumPassword, cfg.TitaniumConfig.TitaniumName)
	if err != nil {
		log.Fatal("failed to create titanium database: ", err)
	}

	res, err := db.DB.Query("select * from users where stripe_subscription is not null")
	if err != nil {
		log.Fatal("failed to get users: ", err)
	}

	for res.Next() {
		user, err := models.UserFromSQLNative(db, res)
		if err != nil {
			log.Println("ERROR: failed to load user: ", err)
			continue
		}

		repairUser(db, user)
	}
}
