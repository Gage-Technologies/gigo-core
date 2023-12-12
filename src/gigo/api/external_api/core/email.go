package core

import (
	"context"
	"database/sql"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/go-redis/redis/v8"
	"github.com/mailgun/mailgun-go/v4"
	"go.opentelemetry.io/otel"
	"net/mail"
	"regexp"
	"time"
)

func SendPasswordVerificationEmail(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string, resetURL string, username string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure verification email content
	message := mg.NewMessage("", "Reset Your Gigo Password", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("passwordReset")

	// add template variables
	err = message.AddTemplateVariable("username", username)
	if err != nil {
		return fmt.Errorf("failed to add template username variable: %v", err)
	}

	err = message.AddTemplateVariable("reseturl", resetURL)
	if err != nil {
		return fmt.Errorf("failed to add template reset Url variable: %v", err)
	}

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send verification email: %v", err)
	}

	return nil
}

// SendSignUpMessage sends a welcoming message to a new user
func SendSignUpMessage(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string, username string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure verification email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("welcomemessage")

	// add template variables
	err = message.AddTemplateVariable("username", username)
	if err != nil {
		return fmt.Errorf("failed to add template username variable: %v", err)
	}

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %v", err)
	}

	return nil
}

// SendStreakExpirationMessage sends a message to a user informing them that their streak is about to expire
func SendStreakExpirationMessage(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string, username string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure verification email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("streakending")

	// add template variables
	err = message.AddTemplateVariable("username", username)
	if err != nil {
		return fmt.Errorf("failed to add template username variable: %v", err)
	}

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %v", err)
	}

	return nil
}

// SendWeekInactiveMessage sends a message to a user that has not been active for one week
func SendWeekInactiveMessage(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure verification email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("inactivehtml")

	// set the email template version
	message.SetTemplateVersion("inactiveoneweek")

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %v", err)
	}

	return nil
}

// SendMonthInactiveMessage sends a message to a user that has not been active for one month
func SendMonthInactiveMessage(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure verification email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("monthinactivehtml")

	// set the email template version
	message.SetTemplateVersion("monthinactive")

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %v", err)
	}

	return nil
}

// SendMessageReceivedEmail sends a message to a user that received a message on gigo. Limited to not send a user more than one message-received email per hour.
func SendMessageReceivedEmail(ctx context.Context, rdb redis.UniversalClient, mailGunKey string, mailGunDomain string, recipient string, username string) error {
	// Define a more specific Redis key
	redisKey := fmt.Sprintf("email:message:received:%v", username)

	// Check if the key exists in Redis
	exists, err := rdb.Exists(ctx, redisKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check Redis for key existence: %v", err)
	}

	// If the key exists, return without sending the email
	if exists > 0 {
		return nil
	}

	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err = mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure verification email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("messagereceived")

	// add template variables
	err = message.AddTemplateVariable("username", username)
	if err != nil {
		return fmt.Errorf("failed to add template username variable: %v", err)
	}

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send welcome email: %v", err)
	}

	// After successfully sending the email, set the key in Redis with a 1-hour expiration
	err = rdb.Set(ctx, redisKey, "sent", time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to set key in Redis: %v", err)
	}

	return nil
}

// SendReferredFriendMessage sends a message after a user successfully refers another account
func SendReferredFriendMessage(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string, referredUser string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure  email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("referredfriend")

	// add template variables
	err = message.AddTemplateVariable("username", referredUser)
	if err != nil {
		return fmt.Errorf("failed to add template username variable: %v", err)
	}

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send referral email: %v", err)
	}

	return nil
}

// SendWasReferredMessage sends a message after a user successfully uses another users referral link
func SendWasReferredMessage(ctx context.Context, mailGunKey string, mailGunDomain string, recipient string, referringUser string) error {
	// create new Mailgun client
	mg := mailgun.NewMailgun(mailGunDomain, mailGunKey)

	// validate email addresses
	_, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient email: %v", err)
	}

	// configure  email content
	message := mg.NewMessage("", "", "", recipient)

	// set the preconfigured email template
	message.SetTemplate("wasreferred")

	// add template variables
	err = message.AddTemplateVariable("username", referringUser)
	if err != nil {
		return fmt.Errorf("failed to add template username variable: %v", err)
	}

	// send the message
	_, _, err = mg.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send referral email: %v", err)
	}

	return nil
}

// ListActiveTemplates iterates over all templates on a given domain. Useful for finding template info programmatically
func ListActiveTemplates(mg *mailgun.MailgunImpl) (*[]mailgun.Template, error) {

	// List all active templates
	it := mg.ListTemplates(&mailgun.ListTemplateOptions{Active: true})

	// context with cancel
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// hold templates
	var page, result []mailgun.Template

	// iterate over templates
	if it != nil {
		for it.Next(ctx, &page) {
			//append result
			result = append(result, page...)
		}
	} else {
		return nil, fmt.Errorf("ListActiveTemplates returned nill itterator : %v", it.Err())
	}

	if it.Err() != nil {
		return nil, fmt.Errorf("failed to iterate over templates : %v", it.Err())
	}

	return &result, nil
}

func VerifyEmailToken(ctx context.Context, tiDB *ti.Database, userId string, token string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "verify-email-token-core")
	defer span.End()
	callerName := "VerifyToken"

	// ensure token and email were provided
	if token == "" || userId == "" {
		return nil, fmt.Errorf("no token or user id provided")
	}

	// Query to find user with the given token and user id
	tokenQuery := "SELECT * FROM users WHERE _id = ? AND reset_token = ? Limit 1"

	// Execute query
	response, err := tiDB.QueryContext(ctx, &span, &callerName, tokenQuery, userId, token)
	if err != nil {
		return nil, fmt.Errorf("failed to query for token: %v", err)
	}

	// Check results
	if !response.Next() {
		return map[string]interface{}{"message": "Token not valid"}, fmt.Errorf("token not found or invalid")
	}

	response.Close()

	return map[string]interface{}{"message": "Token Validated"}, nil
}

func EmailVerification(ctx context.Context, mailGunVerificationKey string, address string) (map[string]interface{}, error) {
	// perform simple email check before advanced validation
	if address == "" || len(address) > 511 {
		return map[string]interface{}{"valid": false}, fmt.Errorf("email cannot be empty")
	}

	// Basic email validation using regex
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !re.MatchString(address) {
		return map[string]interface{}{"valid": false}, fmt.Errorf("email was invalid")
	}

	// create new Mailgun validator client
	validator := mailgun.NewEmailValidator(mailGunVerificationKey)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// validate email address via Mailgun
	email, err := validator.ValidateEmail(ctx, address, true)
	if err != nil {
		return map[string]interface{}{"valid": false}, fmt.Errorf("error while attempting to validate email with mailgun validator client : %v", err)
	}

	// check if the email is valid and not disposable
	if email.IsValid != true {
		return map[string]interface{}{"valid": false}, nil
	} else if email.IsDisposableAddress {
		return map[string]interface{}{"valid": false}, nil
	}

	// flag to hold MailboxVerification
	flag := false

	if email.MailboxVerification == "unknown" || email.MailboxVerification == "false" {
		flag = false
	} else if email.MailboxVerification == "true" {
		flag = true
	} else {
		return map[string]interface{}{"valid": false}, fmt.Errorf("MailboxVerification not true, false, or unknown")
	}

	return map[string]interface{}{"valid": flag}, nil
}

// CheckUnsubscribeEmail
//
// Checks if an email exists in the users table and returns the user's ID if found.
func CheckUnsubscribeEmail(ctx context.Context, tidb *ti.Database, email string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-unsubscribe-email-core")
	defer span.End()
	callerName := "CheckUnsubscribeEmail"

	// Require that email be present
	if email == "" {
		return nil, fmt.Errorf("email must be provided")
	}

	// Build query to check if email already exists and get user ID
	emailQuery := "SELECT _id FROM users WHERE email = ?"

	// Query users to check if email already exists
	response, err := tidb.QueryContext(ctx, &span, &callerName, emailQuery, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing email: %v", err)
	}

	// Ensure the closure of the rows
	defer response.Close()

	// Check if the email exists and retrieve the user ID
	var userID int64
	if response.Next() {
		if err := response.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to retrieve user ID: %v", err)
		}
		return map[string]interface{}{
			"userFound": true,
			"userID":    fmt.Sprintf("%d", userID), // Convert int64 to string
		}, nil
	}

	return map[string]interface{}{"userFound": false}, nil
}

// UpdateEmailPreferences
//
// Updates the email preferences for a given user.
func UpdateEmailPreferences(ctx context.Context, tidb *ti.Database, userID int64, allEmails bool, streak bool, pro bool, newsletter bool, inactivity bool, messages bool, referrals bool, promotional bool) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-email-preferences-core")
	defer span.End()
	callerName := "UpdateEmailPreferences"

	// If all_emails is set to false, set all other preferences to false
	if !allEmails {
		streak = false
		pro = false
		newsletter = false
		inactivity = false
		messages = false
		referrals = false
		promotional = false
	}

	// Build the update query
	updateQuery := `
	UPDATE email_subscription
	SET 
		all_emails = ?,
		streak = ?,
		pro = ?,
		newsletter = ?,
		inactivity = ?,
		messages = ?,
		referrals = ?,
		promotional = ?
	WHERE user_id = ?
	`

	// Execute the update query
	if _, err := tidb.ExecContext(ctx, &span, &callerName, updateQuery, allEmails, streak, pro, newsletter, inactivity, messages, referrals, promotional, userID); err != nil {
		return fmt.Errorf("failed to update email preferences: %v", err)
	}

	return nil
}

// GetUserEmailPreferences
//
// Retrieves the email preferences for a given user.
func GetUserEmailPreferences(ctx context.Context, tidb *ti.Database, userID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-email-preferences-core")
	defer span.End()
	callerName := "GetUserEmailPreferences"

	// Build the query to retrieve email preferences
	query := `
	SELECT all_emails, streak, pro, newsletter, inactivity, messages, referrals, promotional
	FROM email_subscription
	WHERE user_id = ?
	`

	// Execute the query
	var preferences struct {
		AllEmails   bool
		Streak      bool
		Pro         bool
		Newsletter  bool
		Inactivity  bool
		Messages    bool
		Referrals   bool
		Promotional bool
	}
	err := tidb.QueryRowContext(ctx, &span, &callerName, query, userID).Scan(&preferences.AllEmails, &preferences.Streak, &preferences.Pro, &preferences.Newsletter, &preferences.Inactivity, &preferences.Messages, &preferences.Referrals, &preferences.Promotional)
	if err != nil {
		if err == sql.ErrNoRows {
			// No preferences found for the user
			return nil, fmt.Errorf("no email preferences found for user ID: %d", userID)
		}
		return nil, fmt.Errorf("failed to retrieve email preferences: %v", err)
	}

	// Return the preferences as a map
	return map[string]interface{}{
		"allEmails":   preferences.AllEmails,
		"streak":      preferences.Streak,
		"pro":         preferences.Pro,
		"newsletter":  preferences.Newsletter,
		"inactivity":  preferences.Inactivity,
		"messages":    preferences.Messages,
		"referrals":   preferences.Referrals,
		"promotional": preferences.Promotional,
	}, nil
}
