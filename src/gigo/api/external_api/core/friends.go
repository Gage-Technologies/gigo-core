package core

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/mq"
)

func SendFriendRequest(ctx context.Context, db *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, callingUser *models.User, friendID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "send-friend-request-core")
	defer span.End()
	callerName := "SendFriendRequest"
	// create transaction for friend request insertion
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// check if two users are already friends or if either user has sent a request already
	row := tx.QueryRowContext(ctx, `
				SELECT CASE WHEN EXISTS (SELECT * FROM friends WHERE user_id = ? AND friend = ?) THEN 'already friends'
				WHEN EXISTS (SELECT * FROM friend_requests WHERE user_id = ? AND friend = ? AND response IS NULL) THEN 'pending request'
				WHEN EXISTS (SELECT * FROM friend_requests WHERE user_id = ? AND friend = ? AND response IS NULL) THEN 'mutual request'
				ELSE 'not friends'
				END`, callingUser.ID, friendID, callingUser.ID, friendID, friendID, callingUser.ID,
	)

	var message string
	if err := row.Scan(&message); err != nil {
		return nil, fmt.Errorf("failed to scan friend request: %v", err)
	}

	if message == "already friends" {
		return map[string]interface{}{"message": message}, nil
	} else if message == "pending request" {
		return map[string]interface{}{"message": message}, nil
	} else if message == "mutual request" {
		return map[string]interface{}{"message": message}, nil
	}

	var username string
	var friendname string

	err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id = ?", callingUser.ID).Scan(&username)
	if err != nil {
		return nil, fmt.Errorf("failed to select username: %v", err)
	}

	err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id = ?", friendID).Scan(&friendname)
	if err != nil {
		return nil, fmt.Errorf("failed to select username: %v", err)
	}

	notif, err := CreateNotification(ctx, db, js, sf, friendID, "New Friend Request", models.FriendRequest, &callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to send notification: %v", err)
	}

	_, err = tx.ExecContext(ctx, &callerName, "insert ignore into friend_requests(_id, user_id, user_name, friend, friend_name, date, notification_id) values (?, ?, ?, ?, ?, ?, ?);", sf.Generate().Int64(), callingUser.ID, username, friendID, friendname, time.Now(), notif.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert friend_requests: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit friend_requests: %v", err)
	}

	return map[string]interface{}{"message": "friend request sent"}, nil
}

func AcceptFriendRequest(ctx context.Context, db *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, callingUser *models.User, requesterID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "accept-friend-request-core")
	defer span.End()
	callerName := "AcceptFriendRequest"

	// create transaction for friend request insertion
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	var username string
	var friendname string
	var notificationIds []int64

	err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id = ?", callingUser.ID).Scan(&username)
	if err != nil {
		return nil, fmt.Errorf("failed to select username: %v", err)
	}

	err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id = ?", requesterID).Scan(&friendname)
	if err != nil {
		return nil, fmt.Errorf("failed to select username: %v", err)
	}

	req, err := db.QueryContext(ctx, &span, &callerName, "select notification_id from friend_requests where user_id = ? and friend = ?", requesterID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to select username: %v", err)
	}

	// defer closure of tx
	defer req.Close()

	// Scan the notification IDs into the notificationIds slice
	for req.Next() {
		var notificationId int64
		if err := req.Scan(&notificationId); err != nil {
			return nil, fmt.Errorf("failed to scan notification ID: %v", err)
		}
		notificationIds = append(notificationIds, notificationId)
	}

	// create a row for the acceptor
	_, err = tx.ExecContext(ctx, &callerName, "insert ignore into friends(_id, user_id, user_name, friend, friend_name, date) values (?, ?, ?, ?, ?, ?);", sf.Generate().Int64(), callingUser.ID, username, requesterID, friendname, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to insert friend_requests: %v", err)
	}

	// create a row for the requester
	_, err = tx.ExecContext(ctx, &callerName, "insert ignore into friends(_id, user_id, user_name, friend, friend_name, date) values (?, ?, ?, ?, ?, ?);", sf.Generate().Int64(), requesterID, friendname, callingUser.ID, username, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to insert friend_requests: %v", err)
	}

	// set response to true on request table
	_, err = tx.ExecContext(ctx, &callerName, "update friend_requests set response = true where user_id = ? and friend = ?", requesterID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert friend_requests: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// After the transaction is committed, delete the notifications
	for _, notificationId := range notificationIds {
		_, err = db.ExecContext(ctx, &span, &callerName, "delete from notification where _id = ?", notificationId)
		if err != nil {
			return nil, fmt.Errorf("failed to delete notification: %v", err)
		}
	}

	_, err = CreateNotification(ctx, db, js, sf, requesterID, "Accepted Friend Request", models.FriendRequest, &callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to send notification: %v", err)
	}

	return map[string]interface{}{"message": "friend request accepted"}, nil
}

func DeclineFriendRequest(ctx context.Context, db *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, callingUser *models.User, requesterID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "decline-friend-request-core")
	defer span.End()
	callerName := "DeclineFriendRequest"

	// create transaction for friend request insertion
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	var notificationIds []int64

	// Fetching notifications before marking them as declined
	req, err := db.QueryContext(ctx, &span, &callerName, "select notification_id from friend_requests where user_id = ? and friend = ?", requesterID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to select notification_ids: %v", err)
	}
	defer req.Close()

	// Scan the notification IDs into the notificationIds slice
	for req.Next() {
		var notificationId int64
		if err := req.Scan(&notificationId); err != nil {
			return nil, fmt.Errorf("failed to scan notification ID: %v", err)
		}
		notificationIds = append(notificationIds, notificationId)
	}

	// set response to false in request table
	_, err = tx.ExecContext(ctx, &callerName, "update friend_requests set response = false where user_id = ? and friend = ?", requesterID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update friend_requests: %v", err)
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// After the transaction is committed, delete the notifications
	for _, notificationId := range notificationIds {
		_, err = db.ExecContext(ctx, &span, &callerName, "delete from notification where _id = ?", notificationId)
		if err != nil {
			return nil, fmt.Errorf("failed to delete notification: %v", err)
		}
	}

	_, err = CreateNotification(ctx, db, js, sf, requesterID, "Declined Friend Request", models.FriendRequest, &callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to send notification: %v", err)
	}

	return map[string]interface{}{"message": "friend request declined"}, nil
}

func GetFriendsList(ctx context.Context, db *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-friends-list-core")
	defer span.End()
	callerName := "GetFriendsList"

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, &callerName, "select * from friends where user_id =?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get friends: %v", err)
	}

	defer rows.Close()

	friends := make([]*models.FriendsFrontend, 0)
	for rows.Next() {
		friend := &models.FriendsFrontend{}
		err = rows.Scan(&friend.ID, &friend.UserID, &friend.UserName, &friend.Friend, &friend.FriendName, &friend.Date)
		if err != nil {
			return nil, fmt.Errorf("failed to scan friend: %v", err)
		}

		friends = append(friends, friend)
	}

	return map[string]interface{}{"friends": friends}, nil
}

func GetFriendRequests(ctx context.Context, db *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-friends--core")
	defer span.End()
	callerName := "CheckFriends"

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, &callerName, "select * from friend_requests where (user_id = ? or friend = ?) and response is null", callingUser.ID, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get friend requests: %v", err)
	}

	defer rows.Close()

	requests := make([]*models.FriendRequestsFrontend, 0)
	for rows.Next() {
		request := &models.FriendRequestsFrontend{}
		err = rows.Scan(&request.ID, &request.UserID, &request.UserName, &request.Friend, &request.FriendName, &request.Response, &request.Date, &request.NotificationID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan friend requests: %v", err)
		}

		requests = append(requests, request)
	}

	return map[string]interface{}{"requests": requests}, nil
}

func CheckFriend(ctx context.Context, db *ti.Database, callingUser *models.User, friendId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-friends--core")
	defer span.End()
	callerName := "CheckFriends"

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, &callerName, "select * from friends where user_id = ? and friend = ?", callingUser.ID, friendId)
	if err != nil {
		return nil, fmt.Errorf("failed to get friend requests: %v", err)
	}

	var friend bool

	if rows.Next() {
		friend = true
	} else {
		friend = false
	}

	defer rows.Close()

	return map[string]interface{}{"friend": friend}, nil
}

func CheckFriendRequest(ctx context.Context, db *ti.Database, callingUser *models.User, otherUserId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-friend-request-core")
	defer span.End()
	callerName := "CheckFriendRequest"

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, &callerName, "select * from friend_requests where user_id = ? and friend = ?", callingUser.ID, otherUserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get friend request: %v", err)
	}

	defer rows.Close()

	// flag to track whether an active request was found
	hasRequest := false

	if rows.Next() {
		hasRequest = true
	}

	return map[string]interface{}{"request": hasRequest}, nil
}
