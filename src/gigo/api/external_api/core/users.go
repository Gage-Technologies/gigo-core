package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"

	"gigo-core/gigo/config"
	"gigo-core/gigo/streak"

	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/now"
	"go.opentelemetry.io/otel"

	"io"
	"io/ioutil"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"gigo-core/gigo/api/external_api/core/query_models"
	utils3 "gigo-core/gigo/utils"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/session"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	utils2 "github.com/gage-technologies/gigo-lib/utils"
	"github.com/gage-technologies/gitea-go/gitea"
	"github.com/kisielk/sqlstruct"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/oauth2/v2"
)

type DateCount struct {
	Count    int64
	PostDate string
}

func CreateNewUser(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, stripeSubConfig config.StripeSubscriptionConfig, streakEngine *streak.StreakEngine,
	snowflakeNode *snowflake.Node, domain string, userName string, password string, email string, phone string, bio string,
	firstName string, lastName string, vcsClient *git.VCSClient, starterUserInfo models.UserStart, timezone string, thumbnailPath string,
	storageEngine storage.Storage, avatarSettings models.AvatarSettings, filter *utils3.PasswordFilter, forcePass bool, initialRecUrl string,
	logger logging.Logger, mgKey string, mgDomain string, referralUser *string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-user-core")
	defer span.End()
	callerName := "CreateNewUser"

	// require that username be present for all users
	if userName == "" {
		return map[string]interface{}{
			"message": "a username is required for user creation",
		}, errors.New("username missing from user creation")
	}

	username := strings.ReplaceAll(userName, " ", "_")

	// build query to check if username already exists
	nameQuery := "select user_name from users where user_name = ?"

	// query users to ensure username does not already exist
	response, err := tidb.QueryContext(ctx, &span, &callerName, nameQuery, username)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return map[string]interface{}{
			"message": "that username already exists",
		}, errors.New("duplicate username in user creation")
	}

	// build query to check if email is already in use
	emailQuery := "select user_name from users where email = ?"

	// query users to ensure email is not already in use
	response, err = tidb.QueryContext(ctx, &span, &callerName, emailQuery, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate email: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return map[string]interface{}{
			"message": "that email is already in use",
		}, errors.New("duplicate email in user creation")
	}

	// todo add something to check email

	// require that password be present for all users
	if password == "" || len(password) < 5 {
		return map[string]interface{}{
			"message": "password is too short for user creation",
		}, errors.New("password missing or too short for user creation")
	}

	// password is checked on creation unless a user decides to force their password
	if !forcePass {
		// check the user's password against the db of leaked passwords
		unsafe, err := filter.CheckPasswordFilter(password)
		if err != nil {
			return map[string]interface{}{
				"message": "cannot check password",
			}, err
		}

		// let the user know they have an unsafe password
		if unsafe == true {
			return map[string]interface{}{
				"message": "unsafe password",
			}, nil
		}
	}

	// require that email be present for all users
	if email == "" {
		return map[string]interface{}{
			"message": "email is required for user creation",
		}, errors.New("email missing from user creation")
	}

	// validate the email
	_, err = mail.ParseAddress(email)
	if err != nil {
		return map[string]interface{}{
			"message": "not a valid email address",
		}, errors.New("not a valid email address")
	}

	// require that email be present for all users
	if phone == "" {
		return map[string]interface{}{
			"message": "phone number is required for user creation",
		}, errors.New("phone number missing from user creation")
	}

	// load the timezone to ensure it is valid
	userTz, err := time.LoadLocation(timezone)
	if err != nil {
		return map[string]interface{}{
			"message": "invalid timezone",
		}, fmt.Errorf("failed to load user timezone: %v", err)
	}

	// create a new user object
	newUser, err := models.CreateUser(snowflakeNode.Generate().Int64(), username, password, email, phone,
		models.UserStatusBasic, bio, []int64{}, nil, firstName, lastName, -1, "None",
		starterUserInfo, timezone, avatarSettings, 0, nil)
	if err != nil {
		return nil, err
	}

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// create a new strip customer for the user
	stripeID, err := CreateStripeCustomer(ctx, newUser.UserName, newUser.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to create stripe customer: %v", err)
	}
	newUser.StripeUser = &stripeID

	// create a new git user
	gitUser, err := vcsClient.CreateUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), firstName+" "+lastName, fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return map[string]interface{}{"message": "unable to create gitea user"}, err
	}

	// update new user with git user
	newUser.GiteaID = gitUser.ID

	// create boolean to track failure
	failed := true

	// defer function to cleanup coder and gitea user in the case of a failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		// cleaned git user
		_ = vcsClient.DeleteUser(gitUser.UserName)
		// clean user from search
		_ = meili.DeleteDocuments("users", newUser.ID)
		// clean the stripe user
		if newUser.StripeUser != nil {
			_ = DeleteStripeCustomer(ctx, *newUser.StripeUser)
		}
	}()

	// open transaction for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open insertion transaction while creating new user: %v", err)
	}

	// calculate the beginning of the day in the user's timezone
	startOfDay := now.New(time.Now().In(userTz)).BeginningOfDay()

	// initialize the user stats
	err = streakEngine.InitializeFirstUserStats(ctx, tx, newUser.ID, startOfDay)
	if err != nil {
		// roll transaction back
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to initialize user stats: %v", err)
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", newUser.ID)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash post id: %v", err)
	}
	err = storageEngine.MoveFile(
		thumbnailPath,
		fmt.Sprintf("user/%s/%s/%s/profile-pic.svg", idHash[:3], idHash[3:6], idHash),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	// attempt to insert user into search engine
	err = meili.AddDocuments("users", newUser.ToSearch())
	if err != nil {
		return nil, fmt.Errorf("failed to insert new user into search engine: %v", err)
	}

	// format user to frontend
	user, err := newUser.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format new user: %v", err)
	}

	if referralUser != nil {
		// query to see if referred user is an actual user
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select stripe_subscription, user_status, _id, first_name, last_name, email from users where user_name = ? limit 1", referralUser)
		if err != nil {
			return nil, fmt.Errorf("failed to query referral user: %v", err)
		}

		// defer closure of rows
		defer res.Close()

		// create variable to decode res into
		var userQuery models.User

		// load row into first position
		ok := res.Next()
		// return error for missing row
		if !ok {
			return nil, fmt.Errorf("failed to find referral user: %v", err)
		}

		// decode row results
		err = sqlstruct.Scan(&userQuery, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode refferal user: %v", err)
		}

		// update the user with the referral user
		newUser.ReferredBy = &userQuery.ID

		// give the referral user the free month
		_, err = FreeMonthReferral(stripeSubConfig, userQuery.StripeSubscription, int(userQuery.UserStatus), userQuery.ID, tidb, ctx, logger, userQuery.FirstName, userQuery.LastName, userQuery.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for referral user: %v, err: %v", user.ID, err)
		}

		err = SendReferredFriendMessage(ctx, tidb, mgKey, mgDomain, userQuery.Email, newUser.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}

		err = SendWasReferredMessage(ctx, tidb, mgKey, mgDomain, newUser.Email, userQuery.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}
	}

	// retrieve the insert command for the user
	insertStatement, err := newUser.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to load insert statement for new user creation: %v", err)
	}

	// executed insert for the user
	for _, statement := range insertStatement {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			// roll transaction back
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to insert new user: %v", err)
		}
	}

	resp, err := http.Get(fmt.Sprintf("%v/%v", initialRecUrl, user.ID))
	if err != nil {
		logger.Errorf("failed to get initial recommendations for user: %v, err: %v", user.ID, err)
	} else {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			logger.Errorf("failed to get initial recommendations for user: %v, err: non status code 200 returned: %v", user.ID, errors.New(resp.Status))
		}
	}

	// only send email if email is present and not test
	if user.Email != "" && user.Email != "test" {
		// create a new email subscription record for the new user
		emailSub, err := models.CreateEmailSubscription(newUser.ID, newUser.Email, true, true, true, true, true, true, true, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create email subscription: %v", err)
		}

		// email subscription to sql insertion
		subStatement := emailSub.ToSQLNative()
		// attempt to insert new email subscription
		for _, subStmt := range subStatement {
			_, err = tx.ExecContext(ctx, &callerName, subStmt.Statement, subStmt.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to insert email subscription: %v", err)
			}
		}

		// send user sign up message after creation
		err = SendSignUpMessage(ctx, mgKey, mgDomain, user.Email, user.UserName)
		if err != nil {
			logger.Errorf("failed to send sign up message to user: %v, err: %v", user.ID, err)
		}
	}

	inactivity, err := models.CreateUserInactivity(newUser.ID, time.Now(), time.Now(), false, false, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to create user inactivity model, err: %v", err.Error())
	}

	stmt := inactivity.ToSQLNative()

	for _, statement := range stmt {
		_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute statement for user inactivity insertion, err: %v", err.Error())
		}
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	// mark failed as false to block cleanup operation
	failed = false

	return map[string]interface{}{"message": "User Created.", "user": user}, nil
}

// creates a new ephemeral user and inserts it into the database
func CreateNewEUser(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine,
	snowflakeNode *snowflake.Node, domain string, vcsClient *git.VCSClient, masterKey string) (*models.User, error) {

	id := snowflakeNode.Generate().Int64()

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-user-core")
	defer span.End()
	callerName := "CreateNewUser"

	var userName string
	for {
		var err error
		userName, err = utils3.GenerateRandomUsername(16)
		if err != nil {
			return nil, errors.New("username missing from user creation")
		}

		// require that username be present for all users
		if userName == "" {
			return nil, errors.New("username missing from user creation")
		}

		userName = "E-" + userName

		// build query to check if username already exists
		nameQuery := "select user_name from users where user_name = ?"

		// query users to ensure username does not already exist
		response, err := tidb.QueryContext(ctx, &span, &callerName, nameQuery, userName)
		if err != nil {
			return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
		}

		// ensure the closure of the rows
		defer response.Close()

		if response.Next() {
			continue
		}

		break
	}

	// build query to check if email is already in use
	emailQuery := "select user_name from users where email = ?"

	email := fmt.Sprintf("%vephem@gigo.dev", id)

	// query users to ensure email is not already in use
	response, err := tidb.QueryContext(ctx, &span, &callerName, emailQuery, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate email: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return nil, errors.New("duplicate email in user creation")
	}

	// todo add something to check email

	password := masterKey

	// require that email be present for all users
	if email == "" {
		return nil, errors.New("email missing from user creation")
	}

	// validate the email
	_, err = mail.ParseAddress(email)
	if err != nil {
		return nil, errors.New("not a valid email address")
	}

	// create a new user object
	newUser, err := models.CreateUser(id, userName, password, email, "123456789",
		models.UserStatusBasic, "ephemeral", []int64{}, nil, "ephemeral", "ephemeral", -1, "None",
		models.UserStart{
			Usage:             "",
			Proficiency:       "",
			Tags:              "",
			PreferredLanguage: "",
		}, "America/Chicago", models.AvatarSettings{
			TopType:         "",
			AccessoriesType: "",
			HairColor:       "",
			FacialHairType:  "",
			ClotheType:      "",
			ClotheColor:     "",
			EyeType:         "",
			EyebrowType:     "",
			MouthType:       "",
			AvatarStyle:     "",
			SkinColor:       "",
		}, 0, nil)
	if err != nil {
		return nil, err
	}

	// SETTING USER TO EPHEMERAL
	newUser.IsEphemeral = true

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// create a new git user

	// TODO maybe need tweaking for epehemeral git users
	gitUser, err := vcsClient.CreateUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), "ephemeral ephemeral", fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return nil, err
	}

	// update new user with git user
	newUser.GiteaID = gitUser.ID

	// create boolean to track failure
	failed := true

	// defer function to cleanup coder and gitea user in the case of a failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		// cleaned git user
		_ = vcsClient.DeleteUser(gitUser.UserName)
		// clean user from search
		_ = meili.DeleteDocuments("users", newUser.ID)
	}()

	// retrieve the insert command for the life cycle
	insertStatement, err := newUser.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to load insert statement for new user creation: %v", err)
	}

	// open transaction for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open insertion transaction while creating new user: %v", err)
	}

	// executed insert for source image group
	for _, statement := range insertStatement {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			// roll transaction back
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to insert new user: %v", err)
		}
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	failed = false

	return newUser, nil
}

func ValidateUserInfo(ctx context.Context, tidb *ti.Database, userName string, password string, email string, phone string, timezone string,
	filter *utils3.PasswordFilter, forcePass bool) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "validate-user-info-core")
	defer span.End()
	callerName := "ValidateUserInfo"

	// require that username be present for all users
	if userName == "" {
		return map[string]interface{}{
			"message": "a username is required for user creation",
		}, errors.New("username missing from user creation")
	}

	// build query to check if username already exists
	nameQuery := "select user_name from users where user_name = ?"

	// query users to ensure username does not already exist
	response, err := tidb.QueryContext(ctx, &span, &callerName, nameQuery, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return map[string]interface{}{
			"message": "that username already exists",
		}, errors.New("duplicate username in user creation")
	}

	// build query to check if username already exists
	emailQuery := "select user_name from users where email = ?"

	// query users to ensure username does not already exist
	responseEmail, err := tidb.QueryContext(ctx, &span, &callerName, emailQuery, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer responseEmail.Close()

	if responseEmail.Next() {
		return map[string]interface{}{
			"message": "that email already exists",
		}, errors.New("duplicate username in user creation")
	}

	// require that password be present for all users
	if password == "" || len(password) < 5 {
		return map[string]interface{}{
			"message": "password is too short for user creation",
		}, errors.New("password missing or too short for user creation")
	}

	// password is checked on creation unless a user decides to force their password
	if !forcePass {
		// check the user's password against the db of leaked passwords
		unsafe, err := filter.CheckPasswordFilter(password)
		if err != nil {
			return map[string]interface{}{
				"message": "cannot check password",
			}, err
		}

		// let the user know they have an unsafe password
		if unsafe == true {
			return map[string]interface{}{
				"message": "unsafe password",
			}, nil
		}
	}

	// require that email be present for all users
	if email == "" {
		return map[string]interface{}{
			"message": "email is required for user creation",
		}, errors.New("email missing from user creation")
	}

	// validate the email
	_, err = mail.ParseAddress(email)
	if err != nil {
		return map[string]interface{}{
			"message": "not a valid email address",
		}, errors.New("not a valid email address")
	}

	// require that email be present for all users
	if phone == "" {
		return map[string]interface{}{
			"message": "phone number is required for user creation",
		}, errors.New("phone number missing from user creation")
	}

	return map[string]interface{}{"message": "User Cleared."}, nil
}

func ForgotPasswordValidation(ctx context.Context, tiDB *ti.Database, apiKey string, domain string, email string, url string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "forgot-password-validation-core")
	defer span.End()
	callerName := "ForgotPassword"

	// ensure email is not nil
	if email == "" {
		return map[string]interface{}{
			"message": "must provide email for password recovery",
		}, fmt.Errorf("must provide email")
	}

	// query to find user with email and username
	passQuery := "select _id, user_name from users where email = ?"

	// store retrieved user info
	userId := ""
	userName := ""

	// execute query
	response, err := tiDB.QueryContext(ctx, &span, &callerName, passQuery, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query for account: %v", err)
	}

	// scan results
	if response.Next() {
		err = response.Scan(&userId, &userName)
		if err != nil {
			return nil, fmt.Errorf("failed to scan response: %v", err)
		}
	} else {
		return map[string]interface{}{
			"message": "account not found",
		}, fmt.Errorf("account not found")
	}

	defer response.Close()

	// check if account exists
	if response == nil || userId == "" {
		return map[string]interface{}{
			"message": "account not found",
		}, fmt.Errorf("account not found")
	}

	// generate password reset token
	token, err := utils.GenerateEmailToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate reset token: %v", err)
	}

	// store the token in the user model
	_, err = tiDB.ExecContext(ctx, &span, &callerName, "update users set reset_token = ? where _id = ?", token, userId)
	if err != nil {
		return map[string]interface{}{
			"message": "failed to store reset token",
		}, fmt.Errorf("failed to execute query to store reset token. Error: %v", err)
	}

	// generate password reset URL
	resetURL := fmt.Sprintf("https://%s/resetPassword?token=%s&id=%s", url, token, userId)

	err = SendPasswordVerificationEmail(ctx, apiKey, domain, email, resetURL, userName)
	if err != nil {
		return map[string]interface{}{
			"message": "failed to send password reset email",
		}, fmt.Errorf("failed to send password reset email. SendPasswordVerificationEmail Core Error: %v", err)
	}

	return map[string]interface{}{"message": "Password reset email sent", "_id": userId}, nil
}

const UpdateUserPasswordsQuery = `
        update users 
        set 
            password = ?, 
            encrypted_service_key = ? 
        where 
            _id = ?
    `

func ResetForgotPassword(ctx context.Context, tiDB *ti.Database, vcsClient *git.VCSClient, userId string, newPassword string, retypedPassword string, filter *utils3.PasswordFilter, forcePass bool, validToken bool) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "reset-forgot-password-core")
	defer span.End()
	callerName := "ResetForgotPassword"

	// ensure token was valid
	if validToken == false {
		return map[string]interface{}{
			"message": "invalid token",
		}, fmt.Errorf("invalid token")
	}

	// ensure both passwords matched
	if newPassword != retypedPassword {
		return map[string]interface{}{
			"message": "passwords do not match",
		}, fmt.Errorf("passwords do not match")
	}

	// retrieve the user from the database
	res, err := tiDB.QueryContext(ctx, &span, &callerName, "select * from users where _id = ? limit 1", userId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user from database: %v", err)
	}

	// close the query results after processing
	defer res.Close()

	// check if there's a row to be loaded into the user variable
	if !res.Next() {
		return nil, fmt.Errorf("failed to retrieve user from database: no user found")
	}

	// decode the query result into the user variable
	user, err := models.UserFromSQLNative(tiDB, res)
	if err != nil {
		return nil, fmt.Errorf("UserFromSQLNative failed. ResetForgotPassword Core : %v", err)
	}

	// check if the newPassword is valid
	if len(newPassword) < 5 || newPassword == "" {
		return nil, fmt.Errorf("new password is too short or empty")
	}

	// check if the password is too long
	if len(newPassword) > 20 {
		return nil, fmt.Errorf("new password is too long")
	}

	// check if the new password contains spaces
	if strings.Contains(newPassword, " ") {
		return nil, fmt.Errorf("new password contains spaces")
	}

	// password is checked on reset unless a user decides to force their password
	if !forcePass {
		// check the user's password against the db of leaked passwords
		unsafe, err := filter.CheckPasswordFilter(newPassword)
		if err != nil {
			return map[string]interface{}{
				"message": "cannot check password",
			}, fmt.Errorf("password check failed: %v", err)
		}

		// let the user know they have an unsafe password
		if unsafe == true {
			return map[string]interface{}{
				"message": "unsafe password",
			}, nil
		}
	}

	// start a transaction for updating the password
	tx, err := tiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction for password update: %v", err)
	}

	// hash the new password
	hashedPass, err := utils.HashPassword(newPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new password: %v", err)
	}

	// generate new service password
	serviceKey, err := session.GenerateServicePassword()
	if err != nil {
		return nil, fmt.Errorf("failed to generate internal service password: %v", err)
	}

	// re-encrypt the service password using the new user password
	encryptedServiceKey, err := session.EncryptServicePassword(serviceKey, []byte(newPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to re-encrypt service password: %v", err)
	}

	// perform a sanity check of the encryption process
	skSanityCheck, err := session.DecryptServicePassword(encryptedServiceKey, []byte(newPassword))
	if err != nil {
		return nil, fmt.Errorf("failed encryption validation: %v", err)
	}
	if serviceKey != skSanityCheck {
		return nil, fmt.Errorf("failed encryption validation: service passwords do not match")
	}

	// base 64 encode the service password
	finalKey, err := base64.RawStdEncoding.DecodeString(encryptedServiceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode service password: %v", err)
	}

	// options struct for AdminEditUser operation
	vcsEditOpts := gitea.EditUserOption{LoginName: fmt.Sprintf("%v", user.ID), Password: serviceKey}

	// attempt to edit the gitea user password
	vcsResponse, err := vcsClient.GiteaClient.AdminEditUser(fmt.Sprintf("%v", user.ID), vcsEditOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to edit Gitea user Err: %v", err)
	}

	// ensure response was 200 ok
	if vcsResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update vcs service password. ResetForgotPassword Core err : %v", err)
	}

	// execute the update password operation, including the newly encrypted service password
	_, err = tx.ExecContext(ctx, &callerName, UpdateUserPasswordsQuery, hashedPass, finalKey, userId)
	if err != nil {
		// rollback transaction if error occurs
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to update password in database: %v", err)
	}

	// commit the transaction
	err = tx.Commit(&callerName)
	if err != nil {
		// rollback transaction if error occurs
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// if successful, remove the reset token from the user model
	_, err = tiDB.ExecContext(ctx, &span, &callerName, "update users set reset_token = '' where _id =?", userId)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query to remove reset token. Error: %v", err)
	}

	// return a success message
	return map[string]interface{}{"message": "Password reset successfully"}, nil
}

const userProjectsQuery = `
SELECT p.*
FROM post p
JOIN (
  SELECT code_source_id, MAX(created_at) as max_created
  FROM workspaces
  GROUP BY code_source_id
) w ON p._id = w.code_source_id
WHERE p.author_id = ?
ORDER BY w.max_created DESC
LIMIT ? 
OFFSET ?;
`

func UserProjects(ctx context.Context, callingUser *models.User, tidb *ti.Database, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-projects-core")
	defer span.End()
	callerName := "UserProjects"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, userProjectsQuery, callingUser.ID, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
	}

	projects := make([]*models.PostFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project models.Post

		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post from cursor: %v", err)
		}

		// create post frontend
		frontendProject, err := project.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("faield to format post to frontend object: %v", err)
		}

		// // calculate thumbnail url
		// idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", project.ID)))
		// if err != nil {
		//	return nil, fmt.Errorf("failed to hash post id: %v", err)
		// }

		// // add thumbnail url to frontend project
		// frontendProject.Thumbnail = idHash

		// add frontendProject to projects slice
		projects = append(projects, frontendProject)
	}

	return map[string]interface{}{"projects": projects}, nil
}

func FollowUser(ctx context.Context, callingUser *models.User, tidb *ti.Database, following int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "follow-user-core")
	defer span.End()
	callerName := "FollowUser"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, &callerName, "insert ignore into follower(follower, following) values (?, ?);", callingUser.ID, following)
	if err != nil {
		return nil, fmt.Errorf("failed to insert following: %v", err)
	}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update users set follower_count = follower_count + 1 where _id =?", following)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit following: %v", err)
	}

	return map[string]interface{}{"message": "successful"}, nil
}

func UnFollowUser(ctx context.Context, callingUser *models.User, tidb *ti.Database, following int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "un-follow-user-core")
	defer span.End()
	callerName := "UnFollowUser"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// perform deletion via tx
	_, err = tx.ExecContext(ctx, &callerName, "delete from follower where follower = ? and following = ?", callingUser.ID, following)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update users set follower_count = follower_count - 1 where _id =?", following)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit following: %v", err)
	}

	return map[string]interface{}{"message": "successful"}, nil
}

type UserUsage struct {
	DateDifference *string `json:"date_difference"`
	Date           string  `json:"date"`
}

func UserProfilePage(ctx context.Context, callingUser *models.User, tidb *ti.Database, userId *int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "user-profile-page-core")
	defer span.End()
	callerName := "UserProfilePage"

	following := false
	if userId == nil {
		if callingUser == nil {
			return map[string]interface{}{"response": "no login"}, nil
		} else {
			userId = &callingUser.ID
		}
	} else if callingUser != nil {

		// query attempt and projects with the user id as author id and sort by date last edited
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select count(*) as count from follower where follower = ? and following = ? ", callingUser.ID, userId)
		if err != nil {
			return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
		}

		defer res.Close()

		var dataObject int64

		for res.Next() {
			// attempt to load count from row
			err = res.Scan(&dataObject)
			if err != nil {
				return nil, fmt.Errorf("failed to get follower count: %v", err)
			}
		}

		if dataObject == 1 {
			following = true
		}
	}

	currentMonth := time.Now().Month()

	var finalMonth string

	if currentMonth < 10 {
		finalMonth = "0" + fmt.Sprintf("%v", int(currentMonth))
	} else {
		finalMonth = fmt.Sprintf("%v", int(currentMonth))
	}

	currentYear := time.Now().Year()

	monthStart := fmt.Sprintf("%v", currentYear) + "-" + finalMonth + "-01"

	// query attempt and projects with the user id as author id and sort by date last edited
	// res, err := tidb.DB.Query("select date_format(updated_at, '%Y-%m-%d') as date from attempt where author_id = ? and updated_at > ? union select date_format(updated_at, '%Y-%m-%d') as date from post where author_id = ? and updated_at > ? order by date asc", userId, monthStart, userId, monthStart)
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select SUM(TIMESTAMPDIFF(SECOND, start_time, end_time)) as date_difference, date from user_daily_usage where user_id = ? and date > ? group by date order by date asc", userId, monthStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
	}

	data := make([]UserUsage, 0)

	defer res.Close()

	for res.Next() {
		var dataObject UserUsage

		err = res.Scan(&dataObject.DateDifference, &dataObject.Date)
		if err != nil {
			return nil, fmt.Errorf("failed to scan date count from cursor: %v", err)
		}

		data = append(data, dataObject)
	}

	// query for important user information for their profile page
	response, err := tidb.QueryContext(ctx, &span, &callerName, "select tier, level, bio, user_rank, coffee, u._id as _id, follower_count, user_name, r._id as reward_id, color_palette, render_in_front, name, user_status from users u left join rewards r on u.avatar_reward = r._id where u._id = ? limit 1", userId)
	if err != nil {
		return nil, fmt.Errorf("failed to query for user info: %v", err)
	}

	// defer closure of rows
	defer response.Close()

	// create variable to decode res into
	var user query_models.UserBackground

	// load row into first position
	ok := response.Next()
	// return error for missing row
	if !ok {
		return nil, ErrNotFound
	}

	// decode row results
	err = sqlstruct.Scan(&user, response)
	if err != nil {
		return nil, fmt.Errorf("failed to scan row to struct: %v", err)
	}

	finalUser, err := user.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format struct for frontend: %v", err)
	}

	return map[string]interface{}{"activity": data, "user": finalUser, "following": following}, nil
}

func ChangeEmail(ctx context.Context, callingUser *models.User, tidb *ti.Database, newEmail string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "change-email-core")
	defer span.End()
	callerName := "ChangeEmail"

	// todo: needs some sort of email validation in the future
	// ensure input email is valid
	if len(newEmail) < 5 || newEmail == "" {
		return nil, fmt.Errorf("new email was not valid. ChangeEmail core")
	}

	// open tx to perform email update
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// query if email is already in use
	check, err := tx.QueryContext(ctx, &callerName, "select * from users where email = ?", newEmail)
	if err != nil {
		return nil, fmt.Errorf("change email core failed.  Error: %v", err)
	}

	// ensure query found no matches with given email
	if check.Next() {
		return map[string]interface{}{"message": "email is already in use"}, nil
	}

	// close rows
	_ = check.Close()

	// update user email in user model
	res, err := tx.QueryContext(ctx, &callerName, "update users set email = ? where _id = ?", newEmail, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user email. ChangeEmail core.  Error: %v", err)
	}

	// close cursor
	_ = res.Close()

	nextRes, err := tx.QueryContext(ctx, &callerName, "select stripe_user from users where _id = ?", callingUser.ID)
	if err != nil {
		return map[string]interface{}{"message": "could not find user by id"}, nil
	}

	defer nextRes.Close()

	var stripeId *string

	defer res.Close()

	for res.Next() {

		err = sqlstruct.Scan(&stripeId, res)
		if err != nil {
			return nil, fmt.Errorf("failed to scan strip id from cursor: %v", err)
		}
	}

	params := &stripe.CustomerParams{Email: stripe.String(newEmail)}
	// params.AddMetadata("email", newEmail)
	_, err = customer.Update(*stripeId, params)
	if err != nil {
		return map[string]interface{}{"message": "unable to update the stripe email"}, nil
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to perform tx commit on email update: %v", err)
	}

	return map[string]interface{}{"message": "Email updated successfully"}, nil
}

func ChangePhoneNumber(ctx context.Context, callingUser *models.User, tidb *ti.Database, newPhone string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "change-phonenumber-core")
	defer span.End()
	callerName := "ChangePhoneNumber"

	// todo: may need a way to validate phone number

	// ensure new phone number is valid
	if len(newPhone) > 15 || len(newPhone) < 5 {
		return nil, fmt.Errorf("new phone number was not valid. ChangePhoneNumber core")
	}

	// query if phone number is already in use
	check, err := tidb.QueryContext(ctx, &span, &callerName, "select * from users where phone = ?", newPhone)
	if err != nil {
		return nil, fmt.Errorf("change phone number core failed.  Error: %v", err)
	}

	// ensure query found no matches with given phone number
	if check.Next() {
		return nil, fmt.Errorf("account with that phone number already exists")
	}

	// close rows
	_ = check.Close()

	// open tx to perform phone update
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx: %v", err)
	}

	// defer rollback incase we fail
	defer tx.Rollback()

	// update user phone in user model
	res, err := tx.QueryContext(ctx, &callerName, "update users set phone =? where _id =?", newPhone, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user phone number. ChangePhoneNumber core.")
	}

	// close cursor
	_ = res.Close()

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to perform tx commit on phone number update: %v", err)
	}

	return map[string]interface{}{"message": "Phone number updated successfully"}, nil
}

func ChangeUsername(ctx context.Context, callingUser *models.User, tidb *ti.Database, newUsername string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "change-username-core")
	defer span.End()
	callerName := "ChangeUsername"

	// ensure input username is valid
	if len(newUsername) < 2 || newUsername == "" {
		return nil, fmt.Errorf("failed to update username, input too short. ChangeUsername core")
	}

	// query if username is already in use
	check, err := tidb.QueryContext(ctx, &span, &callerName, "select * from users where user_name = ?", newUsername)
	if err != nil {
		return nil, fmt.Errorf("change username core failed.  Error: %v", err)
	}

	// ensure query found no matches with given username
	if check.Next() {
		return nil, fmt.Errorf("account with that username already exists")
	}

	// close rows
	check.Close()

	// update username in user model
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update users set user_name = ? where _id = ?", newUsername, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update username. ChangeUsername core.  Error: %v", err)
	}

	return map[string]interface{}{"message": "Username updated successfully"}, nil
}

func ChangePassword(ctx context.Context, callingUser *models.User, tidb *ti.Database, oldPassword string, newPassword string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "change-password-core")
	defer span.End()
	callerName := "ChangePassword"

	res, err := tidb.QueryContext(ctx, &span, &callerName, "select user_name, password, encrypted_service_key from users where _id = ? limit 1", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update username, input too short. ChangePassword core")
	}

	// defer closure of rows
	defer res.Close()

	// create variable to decode res into
	var user models.UserSQL

	// load row into first position
	ok := res.Next()
	// return error for missing row
	if !ok {
		return nil, fmt.Errorf("failed to update username, input too short. ChangePassword core")
	}

	// decode row results
	err = sqlstruct.Scan(&user, res)
	if err != nil {
		return nil, fmt.Errorf("failed to update username, input too short. ChangePassword core")
	}
	// ensure input username is valid
	if len(newPassword) < 5 || newPassword == "" {
		return nil, fmt.Errorf("failed to update username, input too short. ChangePassword core")
	}

	// ensure oldPassword is not empty
	if len(oldPassword) == 0 {
		return nil, fmt.Errorf("failed to update password, user did not input current password. ChangePassword core")
	}

	// insure oldPassword matches callingUser password
	_, err = utils.CheckPassword(oldPassword, user.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to update password, users current password was incorrect. ChangePassword core")
	}

	// ensure password is not too long
	if len(newPassword) > 20 {
		return nil, fmt.Errorf("failed to update username, input too long. ChangePassword core")
	}

	// check if new password contains spaces
	if strings.Contains(newPassword, " ") {
		return nil, fmt.Errorf("failed to update username, input contains spaces. ChangePassword core")
	}

	// open tx to perform password update
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to change password: %v", err)
	}

	// hash password
	hashedPass, err := utils.HashPassword(newPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash user password: %v", err)
	}

	// decrypt service password
	serviceKey, err := session.DecryptServicePassword(base64.RawStdEncoding.EncodeToString(user.EncryptedServiceKey), []byte(oldPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// encrypt internal service password using new plain-text user password
	encryptedServiceKey, err := session.EncryptServicePassword(serviceKey, []byte(newPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt internal service password: %v", err)
	}

	// sanity check our decryption because if this doesn't work it is
	// a huge headache to repair the database
	skSanityCheck, err := session.DecryptServicePassword(encryptedServiceKey, []byte(newPassword))
	if err != nil {
		return nil, fmt.Errorf("failed encryption validation: %v", err)
	}
	if serviceKey != skSanityCheck {
		return nil, fmt.Errorf("failed encryption validation: internal service password does not match")
	}

	// base64 decode encrypted service password
	rawEncryptedServiceKey, err := base64.RawStdEncoding.DecodeString(encryptedServiceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted service password: %v", err)
	}

	// format statement and execute
	_, err = tx.ExecContext(ctx, &callerName, "update users set password = ?, encrypted_service_key = ? where _id = ?", hashedPass, rawEncryptedServiceKey, callingUser.ID)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to change password : %v", err)
	}

	// commit transaction
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to change password : %v", err)
	}

	return map[string]interface{}{"message": "Password updated successfully"}, nil
}

func ChangeUserPicture(ctx context.Context, callingUser *models.User, tidb *ti.Database, newImagePath string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "change-user-picture-core")
	defer span.End()
	callerName := "ChangePassword"

	// ensure input username is valid
	if len(newImagePath) < 1 || newImagePath == "" {
		return nil, fmt.Errorf("failed to update user picture, path too short. ChangeUserPicture core")
	}

	// update thumb in user model
	res, err := tidb.QueryContext(ctx, &span, &callerName, "update users set user_name = ? where _id = ?", newImagePath, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile picture. ChangeUserPicture core.  Error: %v", err)
	}

	defer res.Close()

	return map[string]interface{}{"message": "Profile picture updated successfully"}, nil
}

func DeleteUserAccount(ctx context.Context, db *ti.Database, meili *search.MeiliSearchEngine, vcsClient *git.VCSClient,
	callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "delete-user-account-core")
	defer span.End()
	callerName := "DeleteUserAccount"

	if callingUser.UserStatus == models.UserStatusPremium && callingUser.StripeSubscription != nil {
		subscriptions, err := subscription.Get(*callingUser.StripeSubscription, nil)
		if err != nil {
			log.Fatalf("Failed to retrieve subscription: %v\n", err)
		}

		if subscriptions.Status != stripe.SubscriptionStatusCanceled {
			res, err := CancelSubscription(ctx, db, callingUser)
			if err != nil {
				return nil, fmt.Errorf("failed to cancel subscription: %v", err)
			}

			if res["subscription"] != "cancelled" {
				return nil, fmt.Errorf("failed to cancel subscription, function did not return cancelled")
			}
		}
	}

	// open tx to delete user
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx: %v", err)
	}

	// perform deletion via tx
	_, err = tx.ExecContext(ctx, &callerName, "delete from attempt where author_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}

	res, err := db.QueryContext(ctx, &span, &callerName, "select * from post where author_id =?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("error getting user rewards inventory: %v", err)
	}

	defer res.Close()

	trueBool := true

	for res.Next() {
		var post models.Post

		err = sqlstruct.Scan(&post, res)
		if err != nil {
			return nil, fmt.Errorf("error scanning user rewards inventory: %v", err)
		}

		_, gitRes, err := vcsClient.GiteaClient.EditRepo(
			fmt.Sprintf("%d", post.AuthorID),
			fmt.Sprintf("%d", post.ID),
			gitea.EditRepoOption{Archived: &trueBool},
		)
		if err != nil {
			if gitRes.StatusCode != 404 {
				buf, _ := io.ReadAll(gitRes.Body)
				return nil, fmt.Errorf("failed to retrieve attempt repo: %v\n     response: %d - %q", err, gitRes.StatusCode, string(buf))
			}
		}

		err = meili.DeleteDocuments("posts", post.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete user from meili: Error: %v", err)
		}
	}

	_, err = tx.ExecContext(ctx, &callerName, "update post set deleted = ?, published = ?, author = ?, author_id = ? where author_id = ?", true, false, "Deleted User", 69, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "update discussion set author = ?, author_id = ? where author_id = ?", "Deleted User", 69, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "update comment set author = ?, author_id = ?, author_tier = 0 where author_id = ?", "Deleted User", 69, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "update thread_comment set author = ?, author_id = ?, author_tier = 0 where author_id = ?", "Deleted User", 69, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "update thread_reply set author = ?, author_id = ?, author_tier = 0 where author_id = ?", "Deleted User", 69, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from friends where user_id = ? or friend = ?", callingUser.ID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from friend_requests where user_id = ? or friend = ?", callingUser.ID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "update users set follower_count = follower_count - 1 where _id = (select following from follower where follower = ?)", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from follower where follower = ? or following = ?", callingUser.ID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from nemesis where antagonist_id = ? or protagonist_id = ?", callingUser.ID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}

	_, err = tx.ExecContext(ctx, &callerName, "delete from user_stats where user_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from user_daily_usage where user_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to edit post description: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from email_subscription where user_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete the email_subscription record: %v", err)
	}
	_, err = tx.ExecContext(ctx, &callerName, "delete from user_inactivity where user_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete the user_inactivity record: %v", err)
	}

	// perform deletion via tx
	_, err = tx.ExecContext(ctx, &callerName, "delete from users where _id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}

	// delete the user's subscription
	if callingUser.StripeSubscription != nil {
		_, err = subscription.Cancel(*callingUser.StripeSubscription, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel Stripe subscription: %v", err)
		}
	}

	// delete the user's stripe account
	if callingUser.StripeAccount != nil {
		_, err = customer.Del(*callingUser.StripeAccount, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to delete Stripe account: %v", err)
		}
	}

	// todo don't delete user but archive all projects
	// err = vcsClient.DeleteUser(fmt.Sprintf("%d", callingUser.ID))
	// if err != nil {
	//	return nil, fmt.Errorf("failed to delete user from vcs: Error: %v", err)
	// }
	err = meili.DeleteDocuments("users", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user from meili: Error: %v", err)
	}

	// commit transaction
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit delete user tx: %v", err)
	}

	return map[string]interface{}{"message": "Account has been deleted."}, nil
}

func CreateNewGoogleUser(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, snowflakeNode *snowflake.Node, stripeSubConfig config.StripeSubscriptionConfig, streakEngine *streak.StreakEngine,
	domain string, externalAuth string, password string, vcsClient *git.VCSClient, starterUserInfo models.UserStart, timezone string, avatarSettings models.AvatarSettings, thumbnailPath string,
	storageEngine storage.Storage, mgKey string, mgDomain string, initialRecUrl string, referralUser *string, logger logging.Logger) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-google-user-core")
	defer span.End()
	callerName := "CreateNewGoogleUser"

	var httpClient = &http.Client{}

	// start oauth 2 service for verification
	oauth2Service, err := oauth2.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to start oauth2 service: %v", err)
	}

	// get google token and verify
	tokenInfoCall := oauth2Service.Tokeninfo().AccessToken(externalAuth)
	tokenInfo, err := tokenInfoCall.Do()
	if err != nil {
		return nil, fmt.Errorf("token info call failed: %v", err)
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do(googleapi.QueryParameter("access_token", externalAuth))
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}

	// load unique user id from google token
	googleId := tokenInfo.UserId

	// build query to check if username already exists
	nameQuery := "select user_name from users where user_name = ?"

	username := strings.ReplaceAll(userInfo.Name, " ", "_")

	// check if username exists, if it does then append a number until it is unique
	for {
		var existingUsername string
		// execute the query with the current username
		err := tidb.QueryRowContext(ctx, &span, &callerName, nameQuery, username).Scan(&existingUsername)

		// if there is no result, the username is unique
		if err == sql.ErrNoRows {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
		}

		// if we get a result, append a random number to the username and try again
		username = fmt.Sprintf("%s%d", username, rand.Intn(1000))
	}

	// build query to check if username already exists
	emailQuery := "select user_name from users where email = ?"

	// query users to ensure username does not already exist
	responseEmail, err := tidb.QueryContext(ctx, &span, &callerName, emailQuery, tokenInfo.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer responseEmail.Close()

	if responseEmail.Next() {
		return map[string]interface{}{
			"message": "that email is already in use",
		}, errors.New("duplicate username in user creation")
	}

	// require that password be present for all users
	if password == "" || len(password) < 5 {
		return map[string]interface{}{
			"message": "password is too short for user creation",
		}, errors.New("password missing or too short for user creation")
	}

	// load the timezone to ensure it is valid
	userTz, err := time.LoadLocation(timezone)
	if err != nil {
		return map[string]interface{}{
			"message": "invalid timezone",
		}, fmt.Errorf("failed to load user timezone: %v", err)
	}

	// create a new user object with google id added
	newUser, err := models.CreateUser(snowflakeNode.Generate().Int64(), username, password, tokenInfo.Email,
		"N/A", models.UserStatusBasic, "", nil, nil, userInfo.GivenName, userInfo.FamilyName,
		-1, googleId, starterUserInfo, timezone, avatarSettings, 0, nil)
	if err != nil {
		return nil, err
	}

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// create a new strip customer for the user
	stripeID, err := CreateStripeCustomer(ctx, newUser.UserName, newUser.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to create stripe customer: %v", err)
	}
	newUser.StripeUser = &stripeID

	// create a new git user
	gitUser, err := vcsClient.CreateUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), userInfo.GivenName+" "+userInfo.FamilyName, fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return map[string]interface{}{"message": "unable to create gitea user"}, err
	}

	// update new user with git user
	newUser.GiteaID = gitUser.ID

	// create boolean to track failure
	failed := true

	// defer function to cleanup coder and gitea user in the case of a failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		// cleaned git user
		_ = vcsClient.DeleteUser(gitUser.UserName)
		// clean user from search
		_ = meili.DeleteDocuments("users", newUser.ID)
		// clean the stripe user
		if newUser.StripeUser != nil {
			_ = DeleteStripeCustomer(ctx, *newUser.StripeUser)
		}
	}()

	// open transaction for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open insertion transaction while creating new user: %v", err)
	}

	// calculate the beginning of the day in the user's timezone
	startOfDay := now.New(time.Now().In(userTz)).BeginningOfDay()

	// initialize the user stats
	err = streakEngine.InitializeFirstUserStats(ctx, tx, newUser.ID, startOfDay)
	if err != nil {
		// roll transaction back
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to initialize user stats: %v", err)
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", newUser.ID)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash post id: %v", err)
	}
	err = storageEngine.MoveFile(
		thumbnailPath,
		fmt.Sprintf("user/%s/%s/%s/profile-pic.svg", idHash[:3], idHash[3:6], idHash),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	// attempt to insert user into search engine
	err = meili.AddDocuments("users", newUser.ToSearch())
	if err != nil {
		return nil, fmt.Errorf("failed to insert new user into search engine: %v", err)
	}

	// format user to frontend
	user, err := newUser.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format new user: %v", err)
	}

	if referralUser != nil {
		// query to see if referred user is an actual user
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select stripe_subscription, user_status, _id, first_name, last_name, email from users where user_name = ? limit 1", referralUser)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("referral user was not in the database: %v", err)
			}
			return nil, fmt.Errorf("failed to query referral user: %v", err)
		}

		// defer closure of rows
		defer res.Close()

		// create variable to decode res into
		var userQuery models.User

		// load row into first position
		ok := res.Next()
		// return error for missing row
		if !ok {
			return nil, fmt.Errorf("failed to find referral user: %v", err)
		}

		// decode row results
		err = sqlstruct.Scan(&userQuery, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode refferal user: %v", err)
		}

		// update the user with the referral user
		newUser.ReferredBy = &userQuery.ID

		// give the referral user the free month
		_, err = FreeMonthReferral(stripeSubConfig, userQuery.StripeSubscription, int(userQuery.UserStatus), userQuery.ID, tidb, ctx, logger, userQuery.FirstName, userQuery.LastName, userQuery.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for referral user: %v, err: %v", user.ID, err)
		}

		err = SendReferredFriendMessage(ctx, tidb, mgKey, mgDomain, userQuery.Email, newUser.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}

		err = SendWasReferredMessage(ctx, tidb, mgKey, mgDomain, newUser.Email, userQuery.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}
	}

	// retrieve the insert command for the new user
	insertStatement, err := newUser.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to load insert statement for new user creation: %v", err)
	}

	// executed insert for new user
	for _, statement := range insertStatement {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			// roll transaction back
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to insert new user: %v", err)
		}
	}

	resp, err := http.Get(fmt.Sprintf("%v/%v", initialRecUrl, user.ID))
	if err != nil {
		logger.Errorf("failed to get initial recommendations for user: %v, err: %v", user.ID, err)
	} else {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			logger.Errorf("failed to get initial recommendations for user: %v, err: non status code 200 returned: %v", user.ID, errors.New(resp.Status))
		}
	}

	// only send email if email is present and not test
	if user.Email != "" && user.Email != "test" {
		// create a new email subscription record for the new user
		emailSub, err := models.CreateEmailSubscription(newUser.ID, newUser.Email, true, true, true, true, true, true, true, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create email subscription: %v", err)
		}

		// email subscription to sql insertion
		subStatement := emailSub.ToSQLNative()
		// attempt to insert new email subscription
		for _, subStmt := range subStatement {
			_, err = tx.ExecContext(ctx, &callerName, subStmt.Statement, subStmt.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to insert email subscription: %v", err)
			}
		}

		// send user sign up message after creation
		err = SendSignUpMessage(ctx, mgKey, mgDomain, user.Email, user.UserName)
		if err != nil {
			logger.Errorf("failed to send sign up message to user: %v, err: %v", user.ID, err)
		}
	}

	inactivity, err := models.CreateUserInactivity(newUser.ID, time.Now(), time.Now(), false, false, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to create user inactivity model, err: %v", err.Error())
	}

	stmt := inactivity.ToSQLNative()

	for _, statement := range stmt {
		_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute statement for user inactivity insertion, err: %v", err.Error())
		}
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	// mark failed as false to block cleanup operation
	failed = false

	return map[string]interface{}{"message": "Google User Added.", "user": user}, nil
}

func GetGithubId(githubCode string, githubSecret string) ([]byte, string, error) {

	clientId := "9ac1616be22aebfdeb3e"

	// set up the request body as JSON
	requestBodyMap := map[string]string{
		"client_id":     clientId,
		"client_secret": githubSecret,
		"code":          githubCode,
	}

	// create the request
	requestJSON, err := json.Marshal(requestBodyMap)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request JSON: %v", err)
	}

	// create the request to send to GitHub's api
	req, err := http.NewRequest(
		"POST",
		"https://github.com/login/oauth/access_token",
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}

	// make sure request has correct format
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// execute the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %v", err)
	}

	// read the response body from GitHub
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %v", err)
	}

	// hold the user's access token
	type githubAccessTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}

	// unmarshal the response holding the access token
	var githubRes githubAccessTokenResponse
	err = json.Unmarshal(resBody, &githubRes)
	if err != nil {
		return nil, "", err
	}

	// create the new request to send the access token for user information
	userReq, err := http.NewRequest(
		"GET",
		"https://api.github.com/user",
		nil,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user request: %v", err)
	}

	authorizationHeaderValue := fmt.Sprintf("token %s", githubRes.AccessToken)
	userReq.Header.Set("Authorization", authorizationHeaderValue)

	// execute the request for user info
	userRes, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute user request: %v", err)
	}

	// Create a new request to fetch email addresses
	emailReq, err := http.NewRequest(
		"GET",
		"https://api.github.com/user/emails",
		nil,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create email request: %v", err)
	}

	emailReq.Header.Set("Authorization", authorizationHeaderValue)

	emailRes, err := http.DefaultClient.Do(emailReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute email request: %v", err)
	}

	// Read the email response body
	emailResBody, err := ioutil.ReadAll(emailRes.Body)
	if err != nil {
		return nil, "", err
	}

	// For debugging: print the email response body
	fmt.Println("Email Response:", string(emailResBody))

	// Assuming it's an array based on your struct; if it's not, this will need to change
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.Unmarshal(emailResBody, &emails); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal email response: %v", err)
	}

	primaryEmail := ""
	for _, email := range emails {
		if email.Primary {
			primaryEmail = email.Email
			break
		}
	}

	// read the response body from GitHub and store as []byte
	userResBody, err := ioutil.ReadAll(userRes.Body)

	return userResBody, primaryEmail, nil
}

func CreateNewGithubUser(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, snowflakeNode *snowflake.Node, stripeSubConfig config.StripeSubscriptionConfig, streakEngine *streak.StreakEngine,
	domain string, externalAuth string, password string, vcsClient *git.VCSClient, starterUserInfo models.UserStart,
	timezone string, avatarSetting models.AvatarSettings, githubSecret string, thumbnailPath string,
	storageEngine storage.Storage, mgKey string, mgDomain string, initialRecUrl string, referralUser *string, ip string, logger logging.Logger) (map[string]interface{}, string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-github-user-core")
	defer span.End()
	callerName := "CreateNewGithubUser"

	userInfo, gitMail, err := GetGithubId(externalAuth, githubSecret)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get github user info: %v", err)
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(userInfo, &m)
	if err != nil {
		fmt.Println("error:", err)
	}

	if m["message"] == "Bad credentials" {
		return nil, "", fmt.Errorf("bad credentials: user's github token has expired or isn't valid")
	}

	userId := int64(m["id"].(float64))

	// build query to check if user already exists
	nameQuery := "select external_auth from users where external_auth = ?"

	// query users to ensure user does not exist with this github id
	response, err := tidb.QueryContext(ctx, &span, &callerName, nameQuery, strconv.FormatInt(userId, 10))
	if err != nil {
		return nil, "", fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return map[string]interface{}{
			"message": "that user already linked their github",
		}, "", errors.New("duplicate github user in user creation")
	}

	// generate the user's GIGO id
	genID := snowflakeNode.Generate().Int64()

	// handle if user does not have an email on their GitHub profile
	var email string
	if m["email"] == nil {
		email = gitMail
	} else {
		email = m["email"].(string)
	}

	// handle if user does not have name on their profile
	var name string
	if m["name"] == nil {
		name = ""
	} else {
		name = m["name"].(string)
	}

	// build query to check if username already exists
	nameQuery = "select user_name from users where user_name = ?"

	username := strings.ReplaceAll(m["login"].(string), " ", "_")

	// check if username exists, if it does then append a number until it is unique
	for {
		var existingUsername string
		// execute the query with the current username
		err := tidb.QueryRowContext(ctx, &span, &callerName, nameQuery, username).Scan(&existingUsername)

		// if there is no result, the username is unique
		if err == sql.ErrNoRows {
			break
		}

		if err != nil {
			return nil, "", fmt.Errorf("failed to query for duplicate username: %v", err)
		}

		// if we get a result, append a random number to the username and try again
		username = fmt.Sprintf("%s%d", username, rand.Intn(1000))
	}

	// require that password be present for all users
	if password == "" || len(password) < 5 {
		return map[string]interface{}{
			"message": "password is too short for user creation",
		}, "", errors.New("password missing or too short for user creation")
	}

	// load the timezone to ensure it is valid
	userTz, err := time.LoadLocation(timezone)
	if err != nil {
		return map[string]interface{}{
			"message": "invalid timezone",
		}, "", fmt.Errorf("failed to load user timezone: %v", err)
	}

	// TODO: download avatar and store locally
	// create a new user object with Google id added
	newUser, err := models.CreateUser(genID, username,
		password, email, "N/A", models.UserStatusBasic, "", nil,
		nil, name, "", -1, strconv.FormatInt(userId, 10), starterUserInfo, timezone, avatarSetting, 0, nil)
	if err != nil {
		return nil, "", err
	}

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// create a new strip customer for the user
	stripeID, err := CreateStripeCustomer(ctx, newUser.UserName, newUser.Email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stripe customer: %v", err)
	}
	newUser.StripeUser = &stripeID

	// create a new git user
	gitUser, err := vcsClient.CreateUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), m["login"].(string), fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return map[string]interface{}{"message": "unable to create gitea user"}, "", err
	}

	// update new user with git user
	newUser.GiteaID = gitUser.ID

	// create boolean to track failure
	failed := true

	// defer function to cleanup coder and gitea user in the case of a failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		// cleaned git user
		_ = vcsClient.DeleteUser(gitUser.UserName)
		// clean user from search
		_ = meili.DeleteDocuments("users", newUser.ID)
		// clean the stripe user
		if newUser.StripeUser != nil {
			_ = DeleteStripeCustomer(ctx, *newUser.StripeUser)
		}
	}()

	// open transaction for insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open insertion transaction while creating new user: %v", err)
	}

	// calculate the beginning of the day in the user's timezone
	startOfDay := now.New(time.Now().In(userTz)).BeginningOfDay()

	// initialize the user stats
	err = streakEngine.InitializeFirstUserStats(ctx, tx, newUser.ID, startOfDay)
	if err != nil {
		// roll transaction back
		_ = tx.Rollback()
		return nil, "", fmt.Errorf("failed to initialize user stats: %v", err)
	}

	// format user to frontend
	user, err := newUser.ToFrontend()
	if err != nil {
		return nil, "", fmt.Errorf("failed to format new user: %v", err)
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", newUser.ID)))
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash post id: %v", err)
	}
	err = storageEngine.MoveFile(
		thumbnailPath,
		fmt.Sprintf("user/%s/%s/%s/profile-pic.svg", idHash[:3], idHash[3:6], idHash),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	// attempt to insert user into search engine
	err = meili.AddDocuments("users", newUser.ToSearch())
	if err != nil {
		return nil, "", fmt.Errorf("failed to insert new user into search engine: %v", err)
	}

	if referralUser != nil {
		// query to see if referred user is an actual user
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select stripe_subscription, user_status, _id, first_name, last_name, email from users where user_name = ? limit 1", referralUser)
		if err != nil {
			return nil, "", fmt.Errorf("failed to query referral user: %v", err)
		}

		// defer closure of rows
		defer res.Close()

		// create variable to decode res into
		var userQuery models.User

		// load row into first position
		ok := res.Next()
		// return error for missing row
		if !ok {
			return nil, "", fmt.Errorf("failed to find referral user: %v", err)
		}

		// decode row results
		err = sqlstruct.Scan(&userQuery, res)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode refferal user: %v", err)
		}

		// update the user for the referral status
		newUser.ReferredBy = &userQuery.ID

		// give the referral user the free month
		_, err = FreeMonthReferral(stripeSubConfig, userQuery.StripeSubscription, int(userQuery.UserStatus), userQuery.ID, tidb, ctx, logger, userQuery.FirstName, userQuery.LastName, userQuery.Email)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create trial subscription for referral user: %v, err: %v", user.ID, err)
		}

		err = SendReferredFriendMessage(ctx, tidb, mgKey, mgDomain, userQuery.Email, newUser.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}

		err = SendWasReferredMessage(ctx, tidb, mgKey, mgDomain, newUser.Email, userQuery.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}
	}

	// retrieve the insert command for the user
	insertStatement, err := newUser.ToSQLNative()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load insert statement for new user creation: %v", err)
	}

	// executed insert for the user
	for _, statement := range insertStatement {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			// roll transaction back
			_ = tx.Rollback()
			return nil, "", fmt.Errorf("failed to insert new user: %v", err)
		}
	}

	resp, err := http.Get(fmt.Sprintf("%v/%v", initialRecUrl, user.ID))
	if err != nil {
		logger.Errorf("failed to get initial recommendations for user: %v, err: %v", user.ID, err)
	} else {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			logger.Errorf("failed to get initial recommendations for user: %v, err: non status code 200 returned: %v", user.ID, errors.New(resp.Status))
		}
	}

	// only send email if email is present and not test
	if user.Email != "" && user.Email != "test" {
		// create a new email subscription record for the new user
		emailSub, err := models.CreateEmailSubscription(newUser.ID, newUser.Email, true, true, true, true, true, true, true, true)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create email subscription: %v", err)
		}

		// email subscription to sql insertion
		subStatement := emailSub.ToSQLNative()
		// attempt to insert new email subscription
		for _, subStmt := range subStatement {
			_, err = tx.ExecContext(ctx, &callerName, subStmt.Statement, subStmt.Values...)
			if err != nil {
				return nil, "", fmt.Errorf("failed to insert email subscription: %v", err)
			}
		}

		// send user sign up message after creation
		err = SendSignUpMessage(ctx, mgKey, mgDomain, user.Email, user.UserName)
		logger.Errorf("failed to send sign up message to user: %v, err: %v", user.ID, err)
	}

	inactivity, err := models.CreateUserInactivity(newUser.ID, time.Now(), time.Now(), false, false, user.Email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user inactivity, err: %v", err.Error())
	}

	stmt := inactivity.ToSQLNative()

	for _, statement := range stmt {
		_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, "", fmt.Errorf("failed to execute statement for user inactivity insertion, err: %v", err.Error())
		}
	}

	token, err := utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", newUser.ID), ip, 0, 5, map[string]interface{}{
		"loginWithGithub": "true",
	})
	if err != nil {
		return nil, "", err
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, "", fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	// mark failed as false to block cleanup operation
	failed = false

	return map[string]interface{}{"message": "Github User Added.", "user": user}, token, nil
}

func GetSubscription(ctx context.Context, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-subcription-core")
	defer span.End()

	userStatus := callingUser.UserStatus
	payment := "0.00"
	membershipStart := int64(0)
	lastPayment := int64(0)
	upcomingPayment := int64(0)
	inTrial := false
	hasPaymentInfo := false
	hasSubscription := false
	alreadyCancelled := false

	if callingUser.StripeSubscription == nil && callingUser.AuthRole != 1 {
		return map[string]interface{}{
			"subscription":        0,
			"subscription_string": "basic",
			"payment":             "0.00",
			"membershipStart":     "N/A",
			"lastPayment":         "N/A",
			"upcomingPayment":     "N/A",
			"inTrial":             false,
			"hasPaymentInfo":      false,
			"hasSubscription":     false,
			"alreadyCancelled":    false,
		}, nil
	}

	if callingUser.StripeSubscription == nil && callingUser.AuthRole == 1 {
		return map[string]interface{}{
			"subscription":        1,
			"subscription_string": "premium",
			"payment":             "0.00",
			"membershipStart":     "N/A",
			"lastPayment":         "N/A",
			"upcomingPayment":     "N/A",
			"inTrial":             false,
			"hasPaymentInfo":      false,
			"hasSubscription":     false,
			"alreadyCancelled":    false,
		}, nil
	}

	if callingUser.StripeSubscription != nil {
		hasSubscription = true
	}

	sub, err := subscription.Get(*callingUser.StripeSubscription, nil)
	if err != nil {
		return map[string]interface{}{"message": "unable to grab subscription"}, err
	}

	if callingUser.UserStatus == models.UserStatusPremium && callingUser.AuthRole == 0 && callingUser.StripeSubscription != nil {

		payment = fmt.Sprintf("%.2f", float64(sub.Items.Data[0].Price.UnitAmount)/100)
		membershipStart = sub.Created
		lastPayment = sub.CurrentPeriodStart
		upcomingPayment = sub.CurrentPeriodEnd

		// // Retrieve the customer to check payment information
		// customerParams := &stripe.CustomerParams{}
		// cus, err := customer.Get(sub.Customer.ID, customerParams)
		// if err != nil {
		//	return map[string]interface{}{"message": fmt.Sprintf("unable to grab customer  %v", cus.DefaultSource)}, err
		// }
	}

	alreadyCancelled = sub.CancelAtPeriodEnd

	// Check if the subscription is in trial
	inTrial = sub.TrialEnd > 0 && sub.TrialEnd > time.Now().Unix()

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
		return map[string]interface{}{"message": "unable to retrieve payment methods"}, i.Err()
	}

	if callingUser.AuthRole == 1 {
		userStatus = models.UserStatusPremium
		payment = "0.00"
	}

	return map[string]interface{}{
		"subscription":        userStatus,
		"subscription_string": userStatus.String(),
		"payment":             payment,
		"membershipStart":     membershipStart,
		"lastPayment":         lastPayment,
		"upcomingPayment":     upcomingPayment,
		"inTrial":             inTrial,
		"hasPaymentInfo":      hasPaymentInfo,
		"hasSubscription":     hasSubscription,
		"alreadyCancelled":    alreadyCancelled,
	}, nil
}

func GetUserInformation(ctx context.Context, callingUser *models.User, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-information-core")
	defer span.End()
	callerName := "GetUserInformation"

	// query for user information
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select user_name, email, phone, workspace_settings, start_user_info, avatar_settings, stripe_account, holiday_themes from users where _id = ? limit 1", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for user information: %v", err)
	}

	// check if post was found with given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no post found with given id: %v", err)
	}

	// attempt to decode res into post model
	user, err := models.UserFromSQLNative(tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for post. ProjectInformation core. Error: %v", err)
	}

	// close explicitly
	_ = res.Close()

	return map[string]interface{}{
		"user": user}, nil
}

func UpdateAvatarSettings(ctx context.Context, callingUser *models.User, tidb *ti.Database, avatarSettings models.AvatarSettings, thumbnailPath string, storageEngine storage.Storage) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-avatar-settings-core")
	defer span.End()
	callerName := "UpdateAvatarSettings"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	setting, err := json.Marshal(avatarSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal avatar settings: %v", err)
	}

	_, err = tx.ExecContext(ctx, &callerName, "update users set avatar_settings = ? where _id = ?", setting, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update avatar for user: Error: %v", err)
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", callingUser.ID)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash user id: %v", err)
	}
	err = storageEngine.MoveFile(
		thumbnailPath,
		fmt.Sprintf("user/%s/%s/%s/profile-pic.svg", idHash[:3], idHash[3:6], idHash),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit user info: %v", err)
	}

	return map[string]interface{}{"message": "avatar settings edited successfully"}, nil
}

func SetUserWorkspaceSettings(ctx context.Context, callingUser *models.User, tidb *ti.Database, workspaceSettings *models.WorkspaceSettings) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "set-user-workspace-settings-core")
	defer span.End()
	callerName := "setUserWorkspaceSettings"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	fmt.Println("workspace settings: \n", workspaceSettings)

	setting, err := json.Marshal(workspaceSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal avatar settings: %v", err)
	}

	fmt.Println("setting are: \n", setting)

	_, err = tx.ExecContext(ctx, &callerName, "update users set workspace_settings = ? where _id = ?", setting, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	return map[string]interface{}{"message": "workspace settings edited successfully"}, nil
}

func GetUserWorkspaceSettings(ctx context.Context, callingUser *models.User, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-workspace-settings-core")
	defer span.End()
	callerName := "getUserWorkspaceSettings"

	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select workspace_settings from users where _id = ? limit 1", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query post: %v", err)
	}

	// check if post was found with given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no post found with given id: %v", err)
	}

	// attempt to decode res into post model
	user, err := models.UserFromSQLNative(tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for post. ProjectInformation core. Error: %v", err)
	}

	// close explicitly
	_ = res.Close()

	return map[string]interface{}{
		"workspace": user}, nil
}

func UpdateUserExclusiveAgreement(ctx context.Context, callingUser *models.User, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "set-user-exclusive-agreement-core")
	defer span.End()
	callerName := "setUserExclusiveAgreement"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, &callerName, "update users set exclusive_agreement = ? where _id = ?", true, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update otp for user: Error: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	return map[string]interface{}{"message": "user agreement updated"}, nil
}

func UpdateHolidayPreference(ctx context.Context, callingUser *models.User, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "set-user-holiday-preference-core")
	defer span.End()
	callerName := "setUserHolidayPreference"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, &callerName, "update users set holiday_themes = not holiday_themes where _id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update holiday preference for user: Error: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	return map[string]interface{}{"message": "user holiday preference updated"}, nil
}

func MarkTutorialAsCompleted(ctx context.Context, callingUser *models.User, tidb *ti.Database, tutorialKey string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "mark-tutorial-as-completed-core")
	defer span.End()
	callerName := "markTutorialAsCompleted"

	// validate that the tutorial key is valid
	validTutorials := map[string]struct{}{
		"all":            {},
		"home":           {},
		"challenge":      {},
		"workspace":      {},
		"nemesis":        {},
		"stats":          {},
		"create_project": {},
		"launchpad":      {},
		"vscode":         {},
	}
	if _, ok := validTutorials[tutorialKey]; !ok {
		return map[string]interface{}{"message": "invalid tutorial key"}, fmt.Errorf("invalid tutorial key: %s", tutorialKey)
	}

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// update the tutorials json with true for the ley value
	// NOTE: this is only safe from SQL injection because of the hard validation of the tutorial key above DO NOT DO THIS ELSEWHERE!!!!
	query := fmt.Sprintf("UPDATE users SET tutorials = JSON_SET(IFNULL(tutorials, '{}'), '$.%s', true) WHERE _id = ?", tutorialKey)
	_, err = tx.ExecContext(ctx, &callerName, query, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update holiday preference for user: Error: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	return map[string]interface{}{"message": "tutorial marked as completed"}, nil
}

func GetUserID(ctx context.Context, tidb *ti.Database, username string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-user-id-core")
	defer span.End()
	callerName := "getUserID"

	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select _id from users where lower(user_name) = ? limit 1", strings.ToLower(username))
	if err != nil {
		return nil, fmt.Errorf("failed to query post: %v", err)
	}

	// check if post was found with given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no post found with given id: %v", err)
	}

	// attempt to decode res into post model
	var id int64
	err = res.Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for post. ProjectInformation core. Error: %v", err)
	}

	// close explicitly
	_ = res.Close()

	return map[string]interface{}{
		"id": fmt.Sprintf("%d", id),
	}, nil
}

func DeleteEphemeralUser(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, rdb redis.UniversalClient, ownerIds []int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "delete-ephemeral-user")
	defer span.End()
	callerName := "deleteEphemeralUser"

	// delete the user ids from from DB and Gitea and their user session
	for i := range ownerIds {

		_, err := tidb.ExecContext(ctx, &span, &callerName, "delete from users where _id =?", ownerIds[i])
		if err != nil {
			return nil, fmt.Errorf("failed to delete ephemeral users: %v", err)
		}

		err = vcsClient.DeleteUser(fmt.Sprintf("%d", ownerIds[i]))
		if err != nil {
			return nil, fmt.Errorf("failed to delete ephemeral user from Gitea: %v", err)
		}

		// retrieve session from redis
		sessionBytes, err := rdb.Get(context.Background(), fmt.Sprintf("gigo-user-sess-%d", ownerIds[i])).Bytes()
		if err != nil {
			if err == redis.Nil {
				return nil, fmt.Errorf("no session")
			}
			return nil, fmt.Errorf("failed to retrieve session from redis: %v", err)
		}

		// unmarshal session
		var session models.UserSession
		err = json.Unmarshal(sessionBytes, &session)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal session from redis: %v", err)
		}

		// delete the session from db
		_, err = tidb.ExecContext(ctx, &span, &callerName, "delete from user_session_key where _id = ?", session.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete ephemeral user session key from db: %v", err)
		}

		// delete the session from redis
		_, err = rdb.Del(context.Background(), fmt.Sprintf("gigo-user-sess-%d", ownerIds[i])).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to delete ephemeral user session from redis: %v", err)
		}
	}

	return map[string]interface{}{"message": "successfully deleted ephemeral users"}, nil
}
