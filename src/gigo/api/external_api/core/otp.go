package core

import (
	"context"
	"fmt"
	"log"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"

	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/gage-technologies/gotp"
	"go.opentelemetry.io/otel"
)

func GenerateUserOtpUri(ctx context.Context, callingUser *models.User, db *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "generate-user-otp-uri-core")
	defer span.End()

	callerName := "GenerateUserOtpUri"

	// ensure that the user has not already set up otp verification
	if callingUser.OtpValidated != nil && *callingUser.OtpValidated {
		return map[string]interface{}{"message": "user has already set up 2fa"}, fmt.Errorf("user with validated otp attempted to re-generate otp secret")
	}

	// generate a 64 byte (256 bit) random secret key
	secret := gotp.RandomSecret(64)

	// create an otp instance derived from the secret key
	otp := gotp.NewDefaultTOTP(secret)

	// generate a url that can be used for linking otp apps
	otpUri := otp.ProvisioningUri(callingUser.UserName, "Gigo")

	_, err := db.ExecContext(ctx, &span, &callerName, "update users set otp = ?, otp_validated = ? where _id = ?", fmt.Sprintf("%v", secret), false, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}

	// return the otp uri to the frontend
	return map[string]interface{}{"otp_uri": otpUri}, nil
}

func VerifyUserOtp(ctx context.Context, callingUser *models.User, db *ti.Database, storageEngine storage.Storage, otp string, ip string) (map[string]interface{}, string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "verify-user-otp-core")
	defer span.End()
	callerName := "VerifyUserOtp"

	// ensure that the otp has been initialized
	if callingUser.Otp == nil {
		return map[string]interface{}{"message": "user has not setup 2fa"}, "", fmt.Errorf("otp was nil in user during verify otp call")
	}

	// use the user secret to create a new otp instance and validate the otp code
	valid := gotp.NewDefaultTOTP(*callingUser.Otp).Verify(otp, time.Now().Unix())

	// create an empty string to hold the token
	token := ""

	// conditionally create a valid token for the user session
	if valid {
		accountValid := false

		inTrial := false
		hasPaymentInfo := false
		hasSubscription := false
		alreadyCancelled := false

		if callingUser.StripeAccount != nil {
			accountValid = true
			sub, err := subscription.Get(*callingUser.StripeSubscription, nil)
			if err != nil {
				inTrial = false
				hasPaymentInfo = false
				hasSubscription = false
				alreadyCancelled = false
			}

			// Check if the subscription is in trial
			inTrial = sub.TrialEnd > 0 && sub.TrialEnd > time.Now().Unix()
			hasSubscription = true
			alreadyCancelled = sub.CancelAtPeriodEnd

			customerId := sub.Customer.ID

			pmParams := &stripe.CustomerListPaymentMethodsParams{
				Customer: &customerId,
			}

			i := customer.ListPaymentMethods(pmParams)

			for i.Next() {
				hasPaymentInfo = true
				break
			}

			if i.Err() != nil {
				log.Println("Error retrieving payment methods:", i.Err())
			}
		}

		// create a token for the user session
		t, err := utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", callingUser.ID), ip, 24*30, 0, map[string]interface{}{
			"user_status":         callingUser.UserStatus,
			"email":               callingUser.Email,
			"phone":               callingUser.Phone,
			"user_name":           callingUser.UserName,
			"thumbnail":           fmt.Sprintf("/static/user/pfp/%v", callingUser.ID),
			"exclusive_content":   accountValid,
			"exclusive_agreement": callingUser.ExclusiveAgreement,
			"in_trial":            inTrial,
			"has_payment_info":    hasPaymentInfo,
			"has_subscription":    hasSubscription,
			"already_cancelled":   alreadyCancelled,
			"used_free_trial":     callingUser.UsedFreeTrial,
		})
		if err != nil {
			return nil, "", err
		}

		// assign the token to the outer scope variable
		token = t

		// conditionally update the user if this is the first time they are verifying their otp login
		if callingUser.OtpValidated != nil && !*callingUser.OtpValidated {
			// update user marking their otp login as validated
			_, err := db.ExecContext(ctx, &span, &callerName, "update users set otp_validated = ? where _id = ?", true, callingUser.ID)
			if err != nil {
				return nil, "", fmt.Errorf("failed to update otp for user: Error: %v", err)
			}
		}
	}

	// return the authentication and token to the frontend
	return map[string]interface{}{
		"auth":  valid,
		"token": token,
	}, token, nil
}
