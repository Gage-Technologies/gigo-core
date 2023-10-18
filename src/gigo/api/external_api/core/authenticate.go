package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel"
	"net/http"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core/query_models"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/session"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/go-redis/redis/v8"
	"github.com/kisielk/sqlstruct"
	"google.golang.org/api/oauth2/v2"
)

// Login
// This function performs the following operations in the corresponding order:
//   - Retrieves the user from the database
//   - Confirms the correct password was passed
//   - Generates a JWT to be inserted on the user's browser as a cookie
//
// Args:
//
//	db         - *ti.Database, a database object to be used for database operations
//	username   - string, username of the user attempting to login
//	password   - string, password of the user attempting to login after being hashed with SHA3-512
//	ip         - string, IP address of the user attempting to login
//
// Returns:
//
//	out        - map[string]interface{}, JSON that will be returned to the caller
//	token      - string, JWT that will be inserted on the user's browser as a cookie for persistent authentication
func Login(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient, sf *snowflake.Node, storageEngine storage.Storage, domain string, username string,
	password string, ip string, logger logging.Logger) (map[string]interface{}, string, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "login-core")
	callerName := "Login"

	// query for user with passed credentials
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select u._id as _id, user_name, password, email, phone, user_status, encrypted_service_key, r._id as reward_id, color_palette, render_in_front, name, level, tier, user_rank, coffee, stripe_account, exclusive_agreement, tutorials from users u left join rewards r on r._id = u.avatar_reward where lower(user_name) = lower(?) limit 1", username)
	if err != nil {
		return map[string]interface{}{
			"message": "invalid username passed",
		}, "", fmt.Errorf("invalid username passed or no account with the username exists\n    Username: %s\n", username)
	}

	// defer closure of rows
	defer res.Close()

	// create variable to decode res into
	var user query_models.UserBackground

	// load row into first position
	ok := res.Next()
	// return error for missing row
	if !ok {
		return map[string]interface{}{"message": "User not found"}, "", err
	}

	// decode row results
	err = sqlstruct.Scan(&user, res)
	if err != nil {
		return map[string]interface{}{"message": "User not found"}, "", err
	}

	// validate password is correct
	valid, err := utils.CheckPassword(password, user.Password)
	if err != nil {
		return map[string]interface{}{"message": "Incorrect email of password."}, "", err
	}

	// return if password is not correct
	if !valid {
		return map[string]interface{}{
			"auth":        false,
			"user_status": "",
			"token":       "",
		}, "", nil
	}

	// generate token for user
	token := ""

	userId := user.ID

	accountValid := false

	if user.StripeAccount != nil {
		accountValid = true
	}

	// parse the tutorials from bytes to a model
	var tutorials models.UserTutorial
	if len(user.Tutorials) > 0 {
		if err := json.Unmarshal(user.Tutorials, &tutorials); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal tutorials: %v", err)
		}
	} else {
		tutorials = models.DefaultUserTutorial
	}

	token, err = utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", userId), ip, 24*30, 0, map[string]interface{}{
		"user_status":         user.UserStatus,
		"email":               user.Email,
		"phone":               user.Phone,
		"user_name":           user.UserName,
		"thumbnail":           fmt.Sprintf("/static/user/pfp/%v", user.ID),
		"color_palette":       user.ColorPalette,
		"render_in_front":     user.RenderInFront,
		"name":                user.Name,
		"exclusive_account":   accountValid,
		"exclusive_agreement": user.ExclusiveAgreement,
		"tutorials":           tutorials,
		"tier":                user.Tier,
	})
	if err != nil {
		return nil, "", err
	}

	// decrypt service password
	serviceKey, err := session.DecryptServicePassword(base64.RawStdEncoding.EncodeToString(user.EncryptedServiceKey), []byte(password))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt internal service secret: %v", err)
	}

	// create user session
	userSession, err := models.CreateUserSession(
		sf.Generate().Int64(),
		user.ID,
		serviceKey,
		time.Now().Add(time.Hour*24*30),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user session: %v", err)
	}

	// store user session
	err = userSession.Store(tidb, rdb)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store user session: %v", err)
	}

	return map[string]interface{}{
		"auth":  valid,
		"token": token,
	}, token, nil
}

func LoginWithGoogle(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient, sf *snowflake.Node, storageEngine storage.Storage, domain string,
	externalAuth string, password string, ip string, logger logging.Logger) (map[string]interface{}, string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "login-with-google-core")
	callerName := "LoginWithGoogle"

	var googleId string

	// make sure a token is provided
	if externalAuth == "" {
		return map[string]interface{}{
			"auth":        false,
			"user_status": "",
			"token":       "",
		}, "", fmt.Errorf("no token passed from google account")
	}
	var httpClient = &http.Client{}

	// start oauth 2 service for verification
	oauth2Service, err := oauth2.New(httpClient)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start oauth2 service: %v", err)
	}

	// get google token and verify
	tokenInfoCall := oauth2Service.Tokeninfo().AccessToken(externalAuth)
	tokenInfo, err := tokenInfoCall.Do()
	if err != nil {
		return nil, "", fmt.Errorf("token info call failed: %v", err)
	}

	// load unique user id from google token
	googleId = tokenInfo.UserId

	// query for user with passed credentials
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select u._id as _id, user_name, password, user_status, email, phone, user_status, encrypted_service_key, r._id as reward_id, color_palette, render_in_front, name, level, tier, user_rank, coffee, stripe_account, exclusive_agreement, tutorials from users u left join rewards r on r._id = u.avatar_reward where external_auth = ? limit 1", googleId)
	if err != nil {
		return map[string]interface{}{
			"message": "google account not linked to any users",
		}, "", fmt.Errorf("no user with this google account exists\n")
	}

	// defer closure of rows
	defer res.Close()

	// create variable to decode res into
	var user query_models.UserBackground

	// load row into first position
	ok := res.Next()
	// return error for missing row
	if !ok {
		return map[string]interface{}{"message": "User not found"}, "", err
	}

	// decode row results
	err = sqlstruct.Scan(&user, res)
	if err != nil {
		return map[string]interface{}{"message": "User not found"}, "", err
	}

	// validate password is correct
	valid, err := utils.CheckPassword(password, user.Password)
	if err != nil {
		return map[string]interface{}{"message": "Incorrect email of password."}, "", err
	}

	// return if password is not correct
	if !valid {
		return map[string]interface{}{
			"auth":        false,
			"user_status": "",
			"token":       "",
		}, "", nil
	}

	// generate token for user
	token := ""

	userId := user.ID

	accountValid := false

	if user.StripeAccount != nil {
		accountValid = true
	}

	// parse the tutorials from bytes to a model
	var tutorials models.UserTutorial
	if len(user.Tutorials) > 0 {
		if err := json.Unmarshal(user.Tutorials, &tutorials); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal tutorials: %v", err)
		}
	} else {
		tutorials = models.DefaultUserTutorial
	}

	token, err = utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", userId), ip, 24*30, 0, map[string]interface{}{
		"user_status":         user.UserStatus,
		"email":               user.Email,
		"phone":               user.Phone,
		"user_name":           user.UserName,
		"thumbnail":           fmt.Sprintf("/static/user/pfp/%v", user.ID),
		"color_palette":       user.ColorPalette,
		"render_in_front":     user.RenderInFront,
		"name":                user.Name,
		"exclusive_account":   accountValid,
		"exclusive_agreement": user.ExclusiveAgreement,
		"tutorials":           tutorials,
		"tier":                user.Tier,
	})
	if err != nil {
		return nil, "", err
	}

	// decrypt service password
	serviceKey, err := session.DecryptServicePassword(base64.RawStdEncoding.EncodeToString(user.EncryptedServiceKey), []byte(password))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt internal service secret: %v", err)
	}

	// create user session
	userSession, err := models.CreateUserSession(
		sf.Generate().Int64(),
		user.ID,
		serviceKey,
		time.Now().Add(time.Hour*24*30),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user session: %v", err)
	}

	// store user session
	err = userSession.Store(tidb, rdb)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store user session: %v", err)
	}

	// // add xp to user for logging in
	// xpRes, err := AddXP(ctx, tidb, js, sf, userId, "login", nil, nil, logger, &models.User{
	//	ID: userId,
	// })
	// if err != nil {
	//	return map[string]interface{}{
	//		"auth":  valid,
	//		"token": token,
	//	}, token, fmt.Errorf("failed to add xp to user: %v", err)
	// }

	// return response with user status and authentication success; auth token for cookie, nil error
	return map[string]interface{}{
		"auth":  valid,
		"token": token,
	}, token, nil
}

func LoginWithGithub(ctx context.Context, tidb *ti.Database, storageEngine storage.Storage, externalAuth string, ip string, githubSecret string) (map[string]interface{}, string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "login-with-github-core")
	callerName := "LoginWithGithub"

	userInfo, err := GetGithubId(externalAuth, githubSecret)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get github user info: %v", err)
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(userInfo, &m)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unmarshall user info: %v", err)
	}

	ghId := int64(m["id"].(float64))

	// query for user with passed credentials
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select u._id as _id, user_name, password, user_status, email, phone, user_status, encrypted_service_key, r._id as reward_id, color_palette, render_in_front, name, level, tier, user_rank, coffee, stripe_account, exclusive_agreement, tutorials from users u left join rewards r on r._id = u.avatar_reward where external_auth = ? limit 1", ghId)
	if err != nil {
		return map[string]interface{}{
			"message": "github account not linked to any users",
		}, "", fmt.Errorf("no user with this google account exists\n")
	}

	// defer closure of rows
	defer res.Close()

	// create variable to decode res into
	var user query_models.UserBackground

	// load row into first position
	ok := res.Next()
	// return error for missing row
	if !ok {
		return map[string]interface{}{"message": "User not found"}, "", err
	}

	// decode row results
	err = sqlstruct.Scan(&user, res)
	if err != nil {
		return map[string]interface{}{"message": "User not found"}, "", err
	}

	// generate token for user
	token := ""

	userId := user.ID

	accountValid := false

	if user.StripeAccount != nil {
		accountValid = true
	}

	// parse the tutorials from bytes to a model
	var tutorials models.UserTutorial
	if len(user.Tutorials) > 0 {
		if err := json.Unmarshal(user.Tutorials, &tutorials); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal tutorials: %v", err)
		}
	} else {
		tutorials = models.DefaultUserTutorial
	}

	token, err = utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", userId), ip, 0, 5, map[string]interface{}{
		"user_status":         user.UserStatus,
		"email":               user.Email,
		"phone":               user.Phone,
		"user_name":           user.UserName,
		"thumbnail":           fmt.Sprintf("/static/user/pfp/%v", user.ID),
		"loginWithGithub":     "true",
		"color_palette":       user.ColorPalette,
		"render_in_front":     user.RenderInFront,
		"name":                user.Name,
		"exclusive_account":   accountValid,
		"exclusive_agreement": user.ExclusiveAgreement,
		"tutorials":           tutorials,
		"tier":                user.Tier,
	})
	if err != nil {
		return nil, "", err
	}

	// return response with user status and authentication success; auth token for cookie, nil error
	return map[string]interface{}{
		"auth":  true,
		"token": token,
	}, token, nil
}

func ConfirmGithubLogin(ctx context.Context, tidb *ti.Database, rdb redis.UniversalClient, js *mq.JetstreamClient, sf *snowflake.Node, storageEngine storage.Storage,
	callingUser *models.User, password string, ip string, logger logging.Logger) (map[string]interface{}, string, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "confirm-github-login-core")
	defer span.End()

	// validate password is correct
	valid, err := utils.CheckPassword(password, callingUser.Password)
	if err != nil {
		return map[string]interface{}{"message": "Incorrect email or password."}, "", err
	}

	// return if password is not correct
	if !valid {
		return map[string]interface{}{
			"auth":        false,
			"user_status": "",
			"token":       "",
		}, "", nil
	}

	// generate token for user
	token := ""

	userId := callingUser.ID
	accountValid := false

	if callingUser.StripeAccount != nil {
		accountValid = true
	}

	token, err = utils.CreateExternalJWT(storageEngine, fmt.Sprintf("%d", userId), ip, 24*30, 0, map[string]interface{}{
		"user_status":         callingUser.UserStatus,
		"email":               callingUser.Email,
		"phone":               callingUser.Phone,
		"user_name":           callingUser.UserName,
		"thumbnail":           fmt.Sprintf("/static/user/pfp/%v", callingUser.ID),
		"exclusive_content":   accountValid,
		"exclusive_agreement": callingUser.ExclusiveAgreement,
	})
	if err != nil {
		return nil, "", err
	}

	// decrypt service password
	serviceKey, err := session.DecryptServicePassword(callingUser.EncryptedServiceKey, []byte(password))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt internal service secret: %v", err)
	}

	// create user session
	userSession, err := models.CreateUserSession(
		sf.Generate().Int64(),
		callingUser.ID,
		serviceKey,
		time.Now().Add(time.Hour*24*30),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user session: %v", err)
	}

	// store user session
	err = userSession.Store(tidb, rdb)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store user session: %v", err)
	}

	// // add xp to user for logging in
	// xpRes, err := AddXP(ctx, tidb, js, sf, userId, "login", nil, nil, logger, &models.User{ID: userId})
	// if err != nil {
	//	return map[string]interface{}{
	//		"auth":  valid,
	//		"token": token,
	//	}, token, fmt.Errorf("failed to add xp to user: %v", err)
	// }

	// return response with user status and authentication success; auth token for cookie, nil error
	return map[string]interface{}{
		"auth":  valid,
		"token": token,
	}, token, nil
}

func ReferralUserInfo(ctx context.Context, tidb *ti.Database, username string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "referral-user-info-core")
	callerName := "ReferralUserInfo"

	// query for important user information for their profile page
	response, err := tidb.QueryContext(ctx, &span, &callerName, "select tier, u._id as _id, user_name, r._id as reward_id, color_palette, render_in_front, name, user_status from users u left join rewards r on u.avatar_reward = r._id where u.user_name = ? limit 1", username)
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

	return map[string]interface{}{"user": finalUser}, nil
}
