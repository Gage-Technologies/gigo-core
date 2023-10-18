package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	models2 "github.com/gage-technologies/gigo-lib/mq/models"
	"strconv"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"go.opentelemetry.io/otel"
)

type CreateChatParams struct {
	Name     string          `json:"name" validate:"required,gte=1,lte=50"`
	ChatType models.ChatType `json:"chat_type" validate:"gte=2,lte=5"`
	Users    []string        `json:"users" validate:"required,dive,number"`
}

func CreateChat(ctx context.Context, db *ti.Database, sf *snowflake.Node, callingUser *models.User, params CreateChatParams) (*models.Chat, *models2.ChatUpdatedEventMsg, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-chat-core")
	defer span.End()
	callerName := "CreateChat"

	// convert the users to ints
	addedUsers := make(map[int64]bool)
	users := make([]int64, 0, len(params.Users))
	for _, user := range params.Users {
		// we can skip the error since it is validated as a number already
		id, _ := strconv.ParseInt(user, 10, 64)

		// ensure the user is not added twice
		if addedUsers[id] {
			continue
		}

		users = append(users, id)
		addedUsers[id] = true
	}

	// add the calling user to the chat if they are not already in it
	if !addedUsers[callingUser.ID] {
		users = append(users, callingUser.ID)
		addedUsers[callingUser.ID] = true
	}

	// ensure that there are at least two users
	if len(users) < 2 {
		return nil, nil, fmt.Errorf("chat must have at least two users")
	}

	// ensure that the type is a DM if we are only adding two users and a group chat if we are adding more than two
	if len(users) == 2 && params.ChatType != models.ChatTypeDirectMessage {
		return nil, nil, fmt.Errorf("chat type must be direct message for two users")
	} else if len(users) > 2 && params.ChatType != models.ChatTypePrivateGroup {
		return nil, nil, fmt.Errorf("chat type must be group for more than two users")
	}

	// if the chat is a DM, ensure that the users are not in any other DMs together
	if params.ChatType == models.ChatTypeDirectMessage {
		// query for the existing chat
		res, err := db.QueryContext(
			ctx, &span, &callerName,
			"select c.* from chat c join chat_users cu on c._id = cu.chat_id where c.type = ? and cu.user_id in (?, ?) group by c._id having count(c._id) = 2",
			models.ChatTypeDirectMessage, users[0], users[1],
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to query for existing chat: %v", err)
		}

		defer res.Close()

		// if we can load the chat into the first position of the cursor then we return it instead of creating a new chat
		if res.Next() {
			chat, err := models.ChatFromSQLNative(callingUser.ID, db, res)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to scan chat: %v", err)
			}
			return chat, nil, nil
		}

		_ = res.Close()
	}

	// create a new chat
	chat := models.CreateChat(sf.Generate().Int64(), params.Name, params.ChatType, users)

	// insert the chat into the database
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	defer tx.Rollback()
	for _, stmt := range chat.ToSQLNative() {
		_, err := tx.ExecContext(ctx, &callerName, stmt.Statement, stmt.Values...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to insert chat: %v", err)
		}
	}
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// retrieve the full chat from the database
	res, err := db.QueryContext(
		ctx, &span, &callerName,
		"select * from chat where _id = ?",
		chat.ID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	defer res.Close()

	// load the chat into the first position of the cursor
	if !res.Next() {
		return nil, nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	// scan the chat
	chat, err = models.ChatFromSQLNative(callingUser.ID, db, res)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan chat: %v", err)
	}

	_ = res.Close()

	// retrieve the calling user's icon info
	var background sql.NullString
	var backgroundPalette sql.NullString
	var backgroundRenderInFront sql.NullBool
	var pro sql.NullBool

	err = db.QueryRow(
		ctx, &span, &callerName,
		"select r.name as background, r.color_palette as background_palette, r.render_in_front as render_in_front, u.user_status = 1 as pro from users u left join rewards r on r._id = u.avatar_reward  where u._id = ?",
		callingUser.ID,
	).Scan(
		&background,
		&backgroundPalette,
		&backgroundRenderInFront,
		&pro,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query for user %d: %w", callingUser.ID, err)
	}

	// create update event
	updateEvents := models2.ChatUpdatedEventMsg{
		Chat:                           chat.ToFrontend(),
		UpdateEvents:                   []models2.ChatUpdateEvent{models2.ChatUpdateEventUserAdd},
		AddedUsers:                     make([]string, 0, len(users)),
		Updater:                        callingUser.UserName,
		UpdaterIcon:                    fmt.Sprintf("/static/user/pfp/%v", callingUser.ID),
		UpdaterBackground:              background.String,
		UpdaterBackgroundPalette:       backgroundPalette.String,
		UpdaterBackgroundRenderInFront: backgroundRenderInFront.Bool,
		UpdaterPro:                     pro.Bool,
	}

	// add the users to the chat
	for _, user := range chat.Users {
		// add the user to the event
		updateEvents.AddedUsers = append(updateEvents.AddedUsers, fmt.Sprintf("%d", user))
	}

	// return the chat
	return chat, &updateEvents, nil
}

type EditChatParams struct {
	ChatId      string   `json:"chat_id" validate:"required,number"`
	Name        string   `json:"name" validate:"lte=20"`
	AddUsers    []string `json:"add_users" validate:"dive,number"`
	RemoveUsers []string `json:"remove_users" validate:"dive,number"`
}

func EditChat(ctx context.Context, db *ti.Database, callingUser *models.User, params EditChatParams) (*models.Chat, *models2.ChatUpdatedEventMsg, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-chat-core")
	defer span.End()
	callerName := "EditChat"

	// parse chat id - we can skip the error since it is validated as a number already
	chatId, _ := strconv.ParseInt(params.ChatId, 10, 64)

	// check if the user is in the chat
	var userInChat bool
	err := db.QueryRow(
		ctx, &span, &callerName,
		"select exists(select 1 from chat_users where chat_id = ? and user_id = ?)",
		chatId,
		callingUser.ID,
	).Scan(&userInChat)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query for user in chat: %v", err)
	}

	// throw an error if the user is not in the chat
	if !userInChat {
		return nil, nil,
			fmt.Errorf("user attempted to edit a chat they are not in: %d - %d", callingUser.ID, chatId)
	}

	// retrieve the chat
	res, err := db.QueryContext(
		ctx, &span, &callerName,
		"select * from chat where _id = ? for update",
		chatId,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	defer res.Close()

	// load chat into the first position of the cursor
	if !res.Next() {
		return nil, nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	// scan the chat
	chat, err := models.ChatFromSQLNative(callingUser.ID, db, res)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan chat: %v", err)
	}

	_ = res.Close()

	// open a transaction
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	defer tx.Rollback()

	// create update event
	updateEvents := models2.ChatUpdatedEventMsg{}

	// parse and validate the add users
	addedUsers := make(map[int64]bool)
	addUsers := make([]int64, 0, len(params.AddUsers))
	for _, user := range params.AddUsers {
		// we can skip the error since it is validated as a number already
		id, _ := strconv.ParseInt(user, 10, 64)

		// ensure the user is not added twice
		if addedUsers[id] {
			continue
		}

		addUsers = append(addUsers, id)
		addedUsers[id] = true
	}

	// remove any users that are already in the chat
	for _, user := range chat.Users {
		if addedUsers[user] {
			delete(addedUsers, user)
			continue
		}

		// update the event with the added user
		updateEvents.AddedUsers = append(updateEvents.AddedUsers, fmt.Sprintf("%d", user))
	}

	// update the event if we added any users
	if len(addUsers) > 0 {
		updateEvents.UpdateEvents = append(updateEvents.UpdateEvents, models2.ChatUpdateEventUserAdd)
	}

	// parse and validate the remove users
	removedUsers := make(map[int64]bool)
	removeUsers := make([]int64, 0, len(params.RemoveUsers))
	for _, user := range params.RemoveUsers {
		// we can skip the error since it is validated as a number already
		id, _ := strconv.ParseInt(user, 10, 64)

		// ensure the user is not added twice and that the user is not the calling user
		if removedUsers[id] || id == callingUser.ID {
			continue
		}

		removeUsers = append(removeUsers, id)
		removedUsers[id] = true
	}

	// remove any users that are not in the chat
	for _, user := range chat.Users {
		if !removedUsers[user] {
			delete(removedUsers, user)
			continue
		}

		// update the event with the removed user
		updateEvents.RemovedUsers = append(updateEvents.RemovedUsers, fmt.Sprintf("%d", user))
	}

	// update the event if we removed any users
	if len(removeUsers) > 0 {
		updateEvents.UpdateEvents = append(updateEvents.UpdateEvents, models2.ChatUpdateEventUserRemove)
	}

	// add the users to the chat
	for _, user := range addUsers {
		_, err = tx.Exec(&callerName, "insert into chat_users (chat_id, user_id) values (?, ?)", chatId, user)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add user to chat: %v", err)
		}

		// add the user to the chat
		chat.Users = append(chat.Users, user)
	}

	// remove the users from the chat
	for _, user := range removeUsers {
		_, err = tx.Exec(&callerName, "delete from chat_users where chat_id = ? and user_id = ?", chatId, user)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to remove user from chat: %v", err)
		}

		// remove the user from the chat
		for i, u := range chat.Users {
			if u == user {
				chat.Users = append(chat.Users[:i], chat.Users[i+1:]...)
				break
			}
		}
	}

	// update the chat name
	if params.Name != "" {
		_, err = tx.Exec(&callerName, "update chat set name = ? where _id = ?", params.Name, chatId)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update chat name: %v", err)
		}

		// save the old name to the update event
		updateEvents.OldName = chat.Name

		// update the chat name
		chat.Name = params.Name

		// add the name change event
		updateEvents.UpdateEvents = append(updateEvents.UpdateEvents, models2.ChatUpdateEventNameChange)
	}

	// retrieve the calling user's icon info
	var background sql.NullString
	var backgroundPalette sql.NullString
	var backgroundRenderInFront sql.NullBool
	var pro sql.NullBool

	err = tx.QueryRow(
		&callerName,
		"select r.name as background, r.color_palette as background_palette, r.render_in_front as render_in_front, u.user_status = 1 as pro from users u left join rewards r on r._id = u.avatar_reward  where u._id = ?",
		callingUser.ID,
	).Scan(
		&background,
		&backgroundPalette,
		&backgroundRenderInFront,
		&pro,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query for user %d: %w", callingUser.ID, err)
	}

	// set the updater info
	updateEvents.Updater = callingUser.UserName
	updateEvents.UpdaterIcon = fmt.Sprintf("/static/user/pfp/%v", callingUser.ID)
	updateEvents.UpdaterBackground = background.String
	updateEvents.UpdaterBackgroundPalette = backgroundPalette.String
	updateEvents.UpdaterBackgroundRenderInFront = backgroundRenderInFront.Bool
	updateEvents.UpdaterPro = pro.Bool

	// set the chat
	updateEvents.Chat = chat.ToFrontend()

	// commit the transaction
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// return the chat
	return chat, &updateEvents, nil
}

type GetChatsParams struct {
	Offset int `json:"offset" validate:"gte=0"`
	Limit  int `json:"limit" validate:"gt=0"`
}

func GetChatsInternal(ctx context.Context, db *ti.Database, callingUser *models.User, params GetChatsParams) ([]*models.Chat, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-chats-core")
	defer span.End()
	callerName := "GetChats"

	// query for the chats
	rows, err := db.QueryContext(
		ctx, &span, &callerName,
		"select c.* from chat c join chat_users cu on c._id = cu.chat_id where cu.user_id = ? order by c.last_message_time desc limit ? offset ?",
		callingUser.ID, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for chats: %v", err)
	}

	// create a slice to hold the chats
	chats := make([]*models.Chat, 0)

	// scan the rows into chats
	for rows.Next() {
		chat, err := models.ChatFromSQLNative(callingUser.ID, db, rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat: %v", err)
		}
		chats = append(chats, chat)
	}

	return chats, nil
}

func GetChats(ctx context.Context, db *ti.Database, callingUser *models.User, params GetChatsParams) ([]*models.ChatFrontend, error) {
	// retrieve the chats
	chats, err := GetChatsInternal(ctx, db, callingUser, params)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chats from internal: %v", err)
	}

	// format to frontend
	frontendChats := make([]*models.ChatFrontend, 0, len(chats))
	for _, chat := range chats {
		frontendChats = append(frontendChats, chat.ToFrontend())
	}

	// return the chats
	return frontendChats, nil
}

type DeleteChatParams struct {
	ChatId string `json:"chat_id" validate:"required"`
}

func DeleteChat(ctx context.Context, db *ti.Database, callingUser *models.User, params DeleteChatParams) (*models.Chat, *models2.ChatUpdatedEventMsg, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "delete-chat-core")
	defer span.End()
	callerName := "DeleteChat"

	// reject if calling user is nil
	if callingUser == nil {
		return nil, nil, errors.New("user is nil")
	}

	// parse chat id - we can skip the error since it is validated as a number already
	chatId, _ := strconv.ParseInt(params.ChatId, 10, 64)

	// check if the user is in the chat
	var userInChat bool
	err := db.QueryRow(
		ctx, &span, &callerName,
		"select exists(select 1 from chat_users where chat_id = ? and user_id = ?)",
		chatId,
		callingUser.ID,
	).Scan(&userInChat)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query for user in chat: %v", err)
	}

	// throw an error if the user is not in the chat
	if !userInChat {
		return nil, nil, fmt.Errorf("user attempted to delete a chat they are not in: %d - %d", callingUser.ID, chatId)
	}

	// retrieve the full chat
	res, err := db.QueryContext(
		ctx, &span, &callerName,
		"select * from chat where _id = ?",
		chatId,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}
	defer res.Close()

	// load the chat into the first position of the cursor
	if !res.Next() {
		return nil, nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	// scan the chat
	chat, err := models.ChatFromSQLNative(0, db, res)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan chat: %v", err)
	}

	// open a transaction
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	defer tx.Rollback()

	// delete the chat
	_, err = tx.Exec(&callerName, "delete from chat where _id = ?", chatId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete chat: %v", err)
	}

	// delete the messages
	_, err = tx.Exec(&callerName, "delete from chat_messages where chat_id = ?", chatId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete chat messages: %v", err)
	}

	// delete the chat users
	_, err = tx.Exec(&callerName, "delete from chat_users where chat_id = ?", chatId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete chat users: %v", err)
	}

	// retrieve the calling user's icon info
	var background sql.NullString
	var backgroundPalette sql.NullString
	var backgroundRenderInFront sql.NullBool
	var pro sql.NullBool

	err = tx.QueryRow(
		&callerName,
		"select r.name as background, r.color_palette as background_palette, r.render_in_front as render_in_front, u.user_status = 1 as pro from users u left join rewards r on r._id = u.avatar_reward  where u._id = ?",
		callingUser.ID,
	).Scan(
		&background,
		&backgroundPalette,
		&backgroundRenderInFront,
		&pro,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query for user %d: %w", callingUser.ID, err)
	}

	// create update event
	updateEvents := models2.ChatUpdatedEventMsg{
		Chat:                           chat.ToFrontend(),
		UpdateEvents:                   []models2.ChatUpdateEvent{models2.ChatUpdateEventDeleted},
		Updater:                        callingUser.UserName,
		UpdaterIcon:                    fmt.Sprintf("/static/user/pfp/%v", callingUser.ID),
		UpdaterBackground:              background.String,
		UpdaterBackgroundPalette:       backgroundPalette.String,
		UpdaterBackgroundRenderInFront: backgroundRenderInFront.Bool,
		UpdaterPro:                     pro.Bool,
	}

	// commit the transaction
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return chat, &updateEvents, nil
}

type SendMessageParams struct {
	ChatId      string                 `json:"chat_id" validate:"number"`
	Content     string                 `json:"content" validate:"required"`
	MessageType models.ChatMessageType `json:"message_type" validate:"gte=0,lte=1"`
}

func SendMessageInternal(ctx context.Context, db *ti.Database, sf *snowflake.Node, callingUser *models.User, params SendMessageParams) (*models.ChatMessage, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "send-message-core")
	defer span.End()
	callerName := "SendMessage"

	// parse chat id - we can skip the error since it is validated as a number already
	chatId, _ := strconv.ParseInt(params.ChatId, 10, 64)

	// open a transaction
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	defer tx.Rollback()

	// get the chat type
	var chatType models.ChatType
	err = tx.QueryRow(
		&callerName,
		"select type from chat where _id = ?",
		params.ChatId,
	).Scan(&chatType)
	if err != nil {
		return nil, fmt.Errorf("failed to query for chat type: %v", err)
	}

	// if this isn't a global, regional, or challenge chat, check if the user is in the chat
	// Note: we reserve the first 1000 chat ids for global and regional chats
	if chatId > 1000 &&
		(chatType != models.ChatTypeGlobal && chatType != models.ChatTypeRegional && chatType != models.ChatTypeChallenge) {
		var userInChat bool
		err = tx.QueryRow(
			&callerName,
			"select exists(select 1 from chat_users where chat_id = ? and user_id = ?)",
			params.ChatId,
			callingUser.ID,
		).Scan(&userInChat)
		if err != nil {
			return nil, fmt.Errorf("failed to query for user in chat: %v", err)
		}

		// throw an error if the user is not in the chat
		if !userInChat {
			return nil,
				fmt.Errorf("user attempted to send a message to a chat they are not in: %d - %d", callingUser.ID, chatId)
		}
	}

	// create a new chat message
	message := models.CreateChatMessage(
		sf.Generate().Int64(),
		chatId,
		callingUser.ID,
		callingUser.UserName,
		params.Content,
		time.Now(),
		0,
		params.MessageType,
	)

	// insert the message
	stmt := message.ToSQLNative()
	_, err = tx.Exec(&callerName, stmt.Statement, stmt.Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %v", err)
	}

	// update the chat last message time
	_, err = tx.Exec(
		&callerName,
		"update chat set last_message_time = current_timestamp, last_message = ? where _id = ?",
		message.ID, params.ChatId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update chat last message time: %v", err)
	}

	// if this is not a global, regional, or challenge chat, update the last read message for the sender
	if chatId > 1000 &&
		(chatType != models.ChatTypeGlobal && chatType != models.ChatTypeRegional && chatType != models.ChatTypeChallenge) {
		_, err = tx.Exec(
			&callerName,
			"update chat_users set last_read_message = ? where chat_id = ? and user_id = ?",
			message.ID, params.ChatId, callingUser.ID,
		)
	}

	// commit the transaction
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// return nil
	return message, nil
}

type GetMessagesParams struct {
	ChatId     string    `json:"chat_id" validate:"required,number"`
	Timestamp  time.Time `json:"timestamp" validate:"required"`
	Descending bool      `json:"descending" validate:"required"`
	Limit      int       `json:"limit" validate:"gt=0"`
}

const getMessagesQuery = `
select 
    cm._id as _id, 
    cm.chat_id as chat_id,
    cm.author_id as author_id,
    u.user_name as author,
    cm.message as message,
    cm.created_at as created_at,
    cm.revision as revision,
    cm.type as type,
    u.tier as author_renown
from chat_messages cm 
    join users u on cm.author_id = u._id
where 
    cm.chat_id = ? 
  	and cm.created_at < ? 
order by cm.created_at %s 
limit ?
`

func GetMessages(ctx context.Context, db *ti.Database, callingUser *models.User, params GetMessagesParams) ([]*models.ChatMessageFrontend, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-messages-core")
	defer span.End()
	callerName := "GetMessages"

	// parse chat id - we can skip the error since it is validated as a number already
	chatId, _ := strconv.ParseInt(params.ChatId, 10, 64)

	// create tx
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %v", err)
	}
	defer tx.Rollback()

	// load the chat type from the database
	var chatType models.ChatType
	err = tx.QueryRowContext(
		ctx,
		"select type from chat where _id = ?",
		chatId,
	).Scan(&chatType)
	if err != nil {
		return nil, fmt.Errorf("failed to query for chat type: %v", err)
	}

	// if this isn't a global, regional, or challenge chat, check if the user is in the chat
	// we use this as an opportunity to retrieve the last_read_message for the user so we can know whether to
	// update at the end of the function
	// Note: we reserve the first 1000 chat ids for global and regional chats
	var lastReadMessage *int64
	if chatId > 1000 &&
		(chatType != models.ChatTypeGlobal && chatType != models.ChatTypeRegional && chatType != models.ChatTypeChallenge) {
		// reject any user that is not logged in
		if callingUser == nil {
			return nil, fmt.Errorf("user attempted to retrieve messages from a chat they are not in: %d - %d", callingUser.ID, chatId)
		}

		// query for the last read message
		err = tx.QueryRowContext(
			ctx,
			"select last_read_message from chat_users where chat_id = ? and user_id = ?",
			chatId, callingUser.ID,
		).Scan(&lastReadMessage)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("user attempted to retrieve messages from a chat they are not in: %d - %d", callingUser.ID, chatId)
			}
			return nil, fmt.Errorf("failed to query for last read message: %v", err)
		}
	}

	// query for the messages
	var rows *sql.Rows

	order := "asc"
	if params.Descending {
		order = "desc"
	}
	rows, err = tx.QueryContext(
		ctx, &callerName,
		fmt.Sprintf(getMessagesQuery, order),
		chatId, params.Timestamp, params.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for messages: %v", err)
	}

	// create a slice to hold the messages
	messages := make([]*models.ChatMessageFrontend, 0)

	// save the highest message id to mark the last read message
	var highestMessageId int64

	// scan the rows into messages
	for rows.Next() {
		message, err := models.ChatMessageFromSQLNative(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %v", err)
		}

		// update the highest message id
		if message.ID > highestMessageId {
			highestMessageId = message.ID
		}

		messages = append(messages, message.ToFrontend())
	}

	// if this is not a global, regional, or challenge chat and the highest message id is greater that the
	// current last read message, update the last read message for the reader
	if highestMessageId > 0 && (lastReadMessage == nil || highestMessageId > *lastReadMessage) &&
		chatId > 1000 &&
		(chatType != models.ChatTypeGlobal && chatType != models.ChatTypeRegional && chatType != models.ChatTypeChallenge) {
		_, err = tx.ExecContext(
			ctx, &callerName,
			"update chat_users set last_read_message = ? where chat_id = ? and user_id = ?",
			highestMessageId, chatId, callingUser.ID,
		)
	}

	// commit the transaction
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// return the messages
	return messages, nil
}

func ValidateChallengeChat(ctx context.Context, db *ti.Database, challengeID string) (*models.Chat, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "validate-challenge-chat-core")
	defer span.End()
	callerName := "ValidateChallengeChat"

	// parse challenge id - we can skip the error since it is validated as a number already
	id, _ := strconv.ParseInt(challengeID, 10, 64)

	// check if the chat already exists
	var chatID int64
	err := db.QueryRowContext(
		ctx, &span, &callerName,
		"select _id from chat where type = ? and _id = ?",
		models.ChatTypeChallenge, id,
	).Scan(&chatID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query for chat: %v", err)
	}

	// create the chat if it doesn't exist
	if err == sql.ErrNoRows {
		// validate that the challenge exists
		var challengeName string
		err = db.QueryRowContext(
			ctx, &span, &callerName,
			"select title from post where _id = ?",
			id,
		).Scan(&challengeName)
		if err != nil {
			return nil, fmt.Errorf("failed to query for challenge: %v", err)
		}

		// create challenge and insert into the database
		chat := models.CreateChat(id, challengeName, models.ChatTypeChallenge, []int64{})
		tx, err := db.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create transaction: %v", err)
		}
		defer tx.Rollback()
		for _, stmt := range chat.ToSQLNative() {
			_, err := tx.ExecContext(ctx, &callerName, stmt.Statement, stmt.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to insert chat: %v", err)
			}
		}
		err = tx.Commit(&callerName)
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %v", err)
		}

		return chat, nil
	}

	// retrieve the chat
	res, err := db.QueryContext(
		ctx, &span, &callerName,
		"select * from chat where _id = ?",
		chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	defer res.Close()

	// load the chat into the first position of the cursor
	if !res.Next() {
		return nil, fmt.Errorf("failed to retrieve chat: %v", err)
	}

	// scan the chat
	chat, err := models.ChatFromSQLNative(0, db, res)
	if err != nil {
		return nil, fmt.Errorf("failed to scan chat: %v", err)
	}

	_ = res.Close()

	// return the chat
	return chat, nil
}

type UpdateReadMessageParams struct {
	ChatId    string `json:"chat_id" validate:"required,number"`
	MessageId string `json:"message_id" validate:"required,number"`
}

func UpdateReadMessage(ctx context.Context, db *ti.Database, callingUser *models.User, params UpdateReadMessageParams) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-read-message-core")
	defer span.End()
	callerName := "UpdateReadMessage"

	// return error if the user is nil
	if callingUser == nil {
		return fmt.Errorf("user attempted to update read message without being logged in")
	}

	// parse chat id and message id- we can skip the error since it is validated as a number already
	chatId, _ := strconv.ParseInt(params.ChatId, 10, 64)
	messageId, _ := strconv.ParseInt(params.MessageId, 10, 64)

	// execute update query
	_, err := db.ExecContext(
		ctx, &span, &callerName,
		"update chat_users set last_read_message = ? where chat_id = ? and user_id = ?",
		messageId, chatId, callingUser.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update read message: %v", err)
	}

	return nil
}

type UpdateChatMuteParams struct {
	ChatId string `json:"chat_id" validate:"required,number"`
	Mute   bool   `json:"mute"`
}

func UpdateChatMute(ctx context.Context, db *ti.Database, callingUser *models.User, params UpdateChatMuteParams) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-chat-mute-core")
	defer span.End()
	callerName := "UpdateChatMute"

	// return error if the user is nil
	if callingUser == nil {
		return fmt.Errorf("user attempted to update chat mute without being logged in")
	}

	// parse chat id - we can skip the error since it is validated as a number already
	chatId, _ := strconv.ParseInt(params.ChatId, 10, 64)

	// execute update query
	_, err := db.ExecContext(
		ctx, &span, &callerName,
		"update chat_users set muted = ? where chat_id = ? and user_id = ?",
		params.Mute, chatId, callingUser.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update chat mute: %v", err)
	}

	return nil
}
