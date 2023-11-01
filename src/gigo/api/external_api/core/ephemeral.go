/*
 *
 *  *  *********************************************************************************
 *  *   GAGE TECHNOLOGIES CONFIDENTIAL
 *  *   __________________
 *  *
 *  *    Gage Technologies
 *  *    Copyright (c) 2021
 *  *    All Rights Reserved.
 *  *
 *  *   NOTICE:  All information contained herein is, and remains
 *  *   the property of Gage Technologies and its suppliers,
 *  *   if any.  The intellectual and technical concepts contained
 *  *   herein are proprietary to Gage Technologies
 *  *   and its suppliers and may be covered by U.S. and Foreign Patents,
 *  *   patents in process, and are protected by trade secret or copyright law.
 *  *   Dissemination of this information or reproduction of this material
 *  *   is strictly forbidden unless prior written permission is obtained
 *  *   from Gage Technologies.
 *
 *
 */

package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gigo-core/gigo/streak"

	"github.com/bwmarrin/snowflake"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/oauth2/v2"

	utils2 "gigo-core/gigo/utils"
	utils3 "gigo-core/gigo/utils"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/session"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/now"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
)

func CreateEphemeral(ctx context.Context, tidb *ti.Database, storageEngine storage.Storage, meili *search.MeiliSearchEngine, sf *snowflake.Node,
	domain string, vscClient *git.VCSClient, masterKey string, jetstreamClient *mq.JetstreamClient,
	wsStatusUpdater *utils2.WorkspaceStatusUpdater, rdb redis.UniversalClient, challengeID int64, ip int64, workspacePath string, accessUrl string,
	hostname string, useTLS bool, ipString string, logger logging.Logger) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-ephemeral-core")
	callerName := "CreateEphemeral"

	var exists int
	err := tidb.QueryRowContext(ctx, &span, &callerName, "SELECT EXISTS (SELECT 1 FROM ephemeral_shared_workspaces WHERE ip = ? AND challenge_id = ?)", ip, challengeID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		logger.Errorf("failed to check if ephemeral system had been used on this ip before, ip: %v, err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	if exists == 1 {
		return map[string]interface{}{"message": "ephemeral system has been used on this network before"}, nil
	}

	callingUser, err := CreateNewEUser(ctx, tidb, meili, sf, domain, vscClient, masterKey)
	if err != nil {
		logger.Errorf("failed to create new ephemeral user, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	// decrypt service password
	serviceKey, err := session.DecryptServicePassword(callingUser.EncryptedServiceKey, []byte(masterKey))
	if err != nil {
		logger.Errorf("failed to decrypt internal service secret, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, fmt.Errorf("failed to decrypt internal service secret: %v", err)
	}

	userSession, err := models.CreateUserSession(sf.Generate().Int64(), callingUser.ID, serviceKey, time.Now().Add(24*time.Hour))
	if err != nil {
		logger.Errorf("failed to decrypt internal service secret, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	// store user session
	err = userSession.Store(tidb, rdb)
	if err != nil {
		logger.Errorf("failed to store user session, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, fmt.Errorf("failed to store user session: %v", err)
	}

	res, err := StartEAttempt(ctx, tidb, vscClient, callingUser, userSession, sf, challengeID, nil, logger)
	if err != nil {
		return nil, err
	}

	attempt := res["attempt"].(*models.AttemptFrontend)

	repoID, err := strconv.ParseInt(attempt.RepoID, 10, 64)
	if err != nil {
		logger.Errorf("failed to parse repo id, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	attemptID, err := strconv.ParseInt(attempt.ID, 10, 64)
	if err != nil {
		logger.Errorf("failed to parse attempt id, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	// execute core function logic
	res, err = CreateEWorkspace(ctx, tidb, vscClient, jetstreamClient, sf, wsStatusUpdater, callingUser,
		accessUrl, repoID, workspacePath, attemptID, models.CodeSource(float64(1)),
		hostname, useTLS, ip, challengeID)
	if err != nil {
		logger.Errorf("failed to create ephemeral attempt, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	token, err := utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", callingUser.ID), ipString, 24, 0, map[string]interface{}{
		"workspace_id":  res["workspace"].(*models.WorkspaceFrontend).ID,
		"workspace_url": res["workspace_url"],
		"attempt_id":    attemptID,
		"user_id":       callingUser.ID,
	})
	if err != nil {
		logger.Errorf("failed to create token for ephemeral user, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	if token == "" {
		logger.Errorf("failed to create token for ephemeral user, ip: %v err: %v", fmt.Sprintf("%v", ip), errors.New("empty token"))
		return nil, errors.New("empty token")
	}

	frontendUser, err := callingUser.ToFrontend()
	if err != nil {
		logger.Errorf("failed to convert user to frontend, ip: %v err: %v", fmt.Sprintf("%v", ip), err)
		return nil, err
	}

	return map[string]interface{}{
		"message":           "Successfully created ephemeral",
		"ephemeral_user":    frontendUser,
		"ephemeral_attempt": attempt,
		"token":             token,
		"workspace_id":      res["workspace"].(*models.WorkspaceFrontend).ID,
		"workspace_url":     res["workspace_url"],
		"attempt_id":        attemptID,
	}, nil
}

func CreateAccountFromEphemeral(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine,
	streakEngine *streak.StreakEngine, domain string, userName string, password string, email string, phone string, bio string,
	firstName string, lastName string, vcsClient *git.VCSClient, starterUserInfo models.UserStart, timezone string, thumbnailPath string,
	storageEngine storage.Storage, avatarSettings models.AvatarSettings, filter *utils3.PasswordFilter, forcePass bool, initialRecUrl string,
	logger logging.Logger, mgKey string, mgDomain string, referralUser *string, eUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-user-core")
	callerName := "CreateNewUser"

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

	externalAuth := "None"
	userStatus := models.UserStatusBasic
	broadcastThresh := uint64(0)

	newUser, statement, err := eUser.EditUser(&userName, &password, &email, &phone,
		&userStatus, &bio, []int64{}, nil, &firstName, &lastName, &eUser.GiteaID, &externalAuth,
		&starterUserInfo, &timezone, &avatarSettings, &broadcastThresh)
	if err != nil {
		return nil, fmt.Errorf("failed to edit user: %v", err)
	}

	_, err = tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to update database from edit user: %v", err)
	}

	// create a new user object
	// newUser, err := models.CreateUser(snowflakeNode.Generate().Int64(), userName, password, email, phone,
	//	models.UserStatusBasic, bio, []int64{}, nil, firstName, lastName, -1, "None",
	//	starterUserInfo, timezone, avatarSettings, 0)
	// if err != nil {
	//	return nil, err
	// }

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	err = vcsClient.EditUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), firstName+" "+lastName, fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to edit git user: %v", err)
	}
	// // create a new git user
	// gitUser, err := vcsClient.CreateUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), firstName+" "+lastName, fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	// if err != nil {
	//	return map[string]interface{}{"message": "unable to create gitea user"}, err
	// }

	// // update new user with git user
	// newUser.GiteaID = gitUser.ID

	// create boolean to track failure
	// failed := true

	// defer function to cleanup coder and gitea user in the case of a failure
	// defer func() {
	//	// skip cleanup if we succeeded
	//	if !failed {
	//		return
	//	}
	//
	//	// cleaned git user
	//	_ = vcsClient.DeleteUser(gitUser.UserName)
	//	// clean user from search
	//	_ = meili.DeleteDocuments("users", newUser.ID)
	// }()

	// // retrieve the insert command for the life cycle
	// insertStatement, err := newUser.ToSQLNative()
	// if err != nil {
	//	return nil, fmt.Errorf("failed to load insert statement for new user creation: %v", err)
	// }

	// // open transaction for insertion
	// tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	// if err != nil {
	//	return nil, fmt.Errorf("failed to open insertion transaction while creating new user: %v", err)
	// }
	//
	// // executed insert for source image group
	// for _, statement := range insertStatement {
	//	_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
	//	if err != nil {
	//		// roll transaction back
	//		_ = tx.Rollback()
	//		return nil, fmt.Errorf("failed to insert new user: %v", err)
	//	}
	// }

	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open insertion transaction while creating new user from ephemeral: %v", err)
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
	idHash, err := utils.HashData([]byte(fmt.Sprintf("%d", newUser.ID)))
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

		// give teh created user the extra free month, 2 in total
		_, err = CreateTrialSubscriptionReferral(ctx, email, tidb, tx, newUser.ID, newUser.FirstName, newUser.LastName)
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
		}

		if userQuery.StripeSubscription != nil {
			// give the referral user the free month
			_, err = FreeMonthReferral(*userQuery.StripeSubscription, int(userQuery.UserStatus), userQuery.ID, tidb, ctx, logger, userQuery.FirstName, userQuery.LastName, userQuery.Email)
			if err != nil {
				return nil, fmt.Errorf("failed to create trial subscription for referral user: %v, err: %v", user.ID, err)
			}
		}
	} else {
		_, err = CreateTrialSubscription(ctx, email, tidb, tx, newUser.ID, newUser.FirstName, newUser.LastName)
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
		}
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	// // mark failed as false to block cleanup operation
	// failed = false

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
		// send user sign up message after creation
		err = SendSignUpMessage(ctx, mgKey, mgDomain, user.Email, user.UserName)
		if err != nil {
			logger.Errorf("failed to send sign up message to user: %v, err: %v", user.ID, err)
		}
	}

	return map[string]interface{}{"message": "User Created.", "user": user}, nil

}

func CreateAccountFromEphemeralGoogle(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, snowflakeNode *snowflake.Node, streakEngine *streak.StreakEngine,
	domain string, externalAuth string, password string, vcsClient *git.VCSClient, starterUserInfo models.UserStart, timezone string, avatarSettings models.AvatarSettings, thumbnailPath string,
	storageEngine storage.Storage, mgKey string, mgDomain string, initialRecUrl string, referralUser *string, eUser *models.User, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-google-user-core")
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

	// query users to ensure username does not already exist
	response, err := tidb.QueryContext(ctx, &span, &callerName, nameQuery, userInfo.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		userInfo.Name = userInfo.Name + snowflakeNode.Generate().String()
		// return map[string]interface{}{
		//	"message": "that username already exists",
		// }, errors.New("duplicate username in user creation")
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
	// newUser, err := models.CreateUser(snowflakeNode.Generate().Int64(), userInfo.Name, password, tokenInfo.Email,
	//	"N/A", models.UserStatusBasic, "", nil, nil, userInfo.GivenName, userInfo.FamilyName,
	//	-1, googleId, starterUserInfo, timezone, avatarSettings, 0)
	// if err != nil {
	//	return nil, err
	// }

	phone := "N/A"
	userSatus := models.UserStatusBasic
	bio := ""
	broadcastThresh := uint64(0)

	newUser, statement, err := eUser.EditUser(&userInfo.Name, &password, &tokenInfo.Email,
		&phone, &userSatus, &bio, nil, nil, &userInfo.GivenName, &userInfo.FamilyName,
		&eUser.GiteaID, &googleId, &starterUserInfo, &timezone, &avatarSettings, &broadcastThresh)
	if err != nil {
		return nil, fmt.Errorf("failed to edit user: %v", err)
	}

	_, err = tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to update database from edit user: %v", err)
	}

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// create a new git user
	err = vcsClient.EditUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), userInfo.GivenName+" "+userInfo.FamilyName, fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return map[string]interface{}{"message": "unable to create gitea user"}, err
	}

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
	idHash, err := utils.HashData([]byte(fmt.Sprintf("%d", newUser.ID)))
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

		_, err = CreateTrialSubscriptionReferral(ctx, tokenInfo.Email, tidb, tx, newUser.ID, newUser.FirstName, newUser.LastName)
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
		}

		if userQuery.StripeSubscription != nil {
			// give the referral user the free month
			_, err = FreeMonthReferral(*userQuery.StripeSubscription, int(userQuery.UserStatus), userQuery.ID, tidb, ctx, logger, userQuery.FirstName, userQuery.LastName, userQuery.Email)
			if err != nil {
				return nil, fmt.Errorf("failed to create trial subscription for referral user: %v, err: %v", user.ID, err)
			}
		}
	} else {
		_, err = CreateTrialSubscription(ctx, tokenInfo.Email, tidb, tx, newUser.ID, newUser.FirstName, newUser.LastName)
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
		}
	}

	// _, err = CreateTrialSubscription(ctx, tokenInfo.Email, tidb, tx, newUser.ID, newUser.FirstName, newUser.LastName)
	// if err != nil {
	//	return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
	// }

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
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
		// send user sign up message after creation
		err = SendSignUpMessage(ctx, mgKey, mgDomain, user.Email, user.UserName)
		if err != nil {
			return nil, fmt.Errorf("failed to send welcome message to user: %v", err)
		}
	}

	return map[string]interface{}{"message": "Google User Added.", "user": user}, nil
}

func CreateAccountFromEphemeralGithub(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, snowflakeNode *snowflake.Node, streakEngine *streak.StreakEngine,
	domain string, externalAuth string, password string, vcsClient *git.VCSClient, starterUserInfo models.UserStart,
	timezone string, avatarSetting models.AvatarSettings, githubSecret string, thumbnailPath string,
	storageEngine storage.Storage, mgKey string, mgDomain string, initialRecUrl string, referralUser *string,
	eUser *models.User, logger logging.Logger) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-new-github-user-core")
	callerName := "CreateNewGithubUser"

	userInfo, gitMail, err := GetGithubId(externalAuth, githubSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get github user info: %v", err)
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(userInfo, &m)
	if err != nil {
		fmt.Println("error:", err)
	}

	if m["message"] == "Bad credentials" {
		return nil, fmt.Errorf("bad credentials: user's github token has expired or isn't valid")
	}

	userId := int64(m["id"].(float64))

	// build query to check if user already exists
	nameQuery := "select external_auth from users where external_auth = ?"

	// query users to ensure user does not exist with this github id
	response, err := tidb.QueryContext(ctx, &span, &callerName, nameQuery, strconv.FormatInt(userId, 10))
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return map[string]interface{}{
			"message": "that user already linked their github",
		}, errors.New("duplicate github user in user creation")
	}

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

	// // build query to check if username already exists
	// nameQuery = "select user_name from users where user_name = ?"
	//
	// // query users to ensure username does not already exist
	// response, err = tidb.QueryContext(ctx, &span, &callerName, nameQuery, m["login"].(string))
	// if err != nil {
	//	return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	// }
	//
	// // ensure the closure of the rows
	// defer response.Close()
	//
	// if response.Next() {
	//	return map[string]interface{}{
	//		"message": "that username already exists",
	//	}, errors.New("duplicate username in user creation")
	// }

	// build query to check if username already exists
	nameQuery = "select user_name from users where user_name = ?"

	// query users to ensure username does not already exist
	response, err = tidb.QueryContext(ctx, &span, &callerName, nameQuery, m["login"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	loginName := m["login"].(string)

	if response.Next() {
		loginName = m["login"].(string) + snowflakeNode.Generate().String()
		// return map[string]interface{}{
		//	"message": "that username already exists",
		// }, errors.New("duplicate username in user creation")
	}

	// // build query to check if username already exists
	// emailQuery := "select user_name from users where email = ?"
	//
	// // query users to ensure username does not already exist
	// responseEmail, err := tidb.QueryContext(ctx, &span, &callerName, emailQuery, tokenInfo.Email)
	// if err != nil {
	//	return nil, fmt.Errorf("failed to query for duplicate username: %v", err)
	// }
	//
	// // ensure the closure of the rows
	// defer responseEmail.Close()
	//
	// if responseEmail.Next() {
	//	return map[string]interface{}{
	//		"message": "that email is already in use",
	//	}, errors.New("duplicate username in user creation")
	// }

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

	// TODO: download avatar and store locally
	// create a new user object with Google id added
	// newUser, err := models.CreateUser(genID, loginName,
	//	password, email, "N/A", models.UserStatusBasic, "", nil,
	//	nil, name, "", -1, strconv.FormatInt(userId, 10), starterUserInfo, timezone, avatarSetting, 0)
	// if err != nil {
	//	return nil, err
	// }

	phone := "N/A"
	userSatus := models.UserStatusBasic
	bio := ""
	broadcastThresh := uint64(0)
	githubId := strconv.FormatInt(userId, 10)
	lastName := ""

	newUser, statement, err := eUser.EditUser(&loginName, &password, &email,
		&phone, &userSatus, &bio, nil, nil, &name, &lastName,
		&eUser.GiteaID, &githubId, &starterUserInfo, &timezone, &avatarSetting, &broadcastThresh)
	if err != nil {
		return nil, fmt.Errorf("failed to edit user: %v", err)
	}

	_, err = tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to update database from edit user: %v", err)
	}

	// decrypt user service password
	servicePassword, err := session.DecryptServicePassword(newUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt user password: %v", err)
	}

	// create a new git user
	err = vcsClient.EditUser(fmt.Sprintf("%d", newUser.ID), fmt.Sprintf("%d", newUser.ID), m["login"].(string), fmt.Sprintf("%d@git.%s", newUser.ID, domain), servicePassword)
	if err != nil {
		return map[string]interface{}{"message": "unable to create gitea user"}, err
	}

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

	// calculate the beginning of the day in the user's timezone
	startOfDay := now.New(time.Now().In(userTz)).BeginningOfDay()

	// initialize the user stats
	err = streakEngine.InitializeFirstUserStats(ctx, tx, newUser.ID, startOfDay)
	if err != nil {
		// roll transaction back
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to initialize user stats: %v", err)
	}

	// format user to frontend
	user, err := newUser.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format new user: %v", err)
	}

	// write thumbnail to final location
	idHash, err := utils.HashData([]byte(fmt.Sprintf("%d", newUser.ID)))
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

		_, err = CreateTrialSubscriptionReferral(ctx, email, tidb, tx, newUser.ID, name, " ")
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
		}

		if userQuery.StripeSubscription != nil {
			// give the referral user the free month
			_, err = FreeMonthReferral(*userQuery.StripeSubscription, int(userQuery.UserStatus), userQuery.ID, tidb, ctx, logger, userQuery.FirstName, userQuery.LastName, userQuery.Email)
			if err != nil {
				return nil, fmt.Errorf("failed to create trial subscription for referral user: %v, err: %v", user.ID, err)
			}
		}
	} else {
		_, err = CreateTrialSubscription(ctx, email, tidb, tx, newUser.ID, name, " ")
		if err != nil {
			return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
		}
	}

	// _, err = CreateTrialSubscription(ctx, email, tidb, tx, newUser.ID, name, " ")
	// if err != nil {
	//	return nil, fmt.Errorf("failed to create trial subscription for user: %v, err: %v", user.ID, err)
	// }

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
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
		// send user sign up message after creation
		err = SendSignUpMessage(ctx, mgKey, mgDomain, user.Email, user.UserName)
		if err != nil {
			return nil, fmt.Errorf("failed to send welcome message to user: %v", err)
		}
	}

	return map[string]interface{}{"message": "Github User Added.", "user": user}, nil
}
