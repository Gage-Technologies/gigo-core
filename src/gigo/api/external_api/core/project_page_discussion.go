package core

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"

	"gigo-core/gigo/api/external_api/core/query_models"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/kisielk/sqlstruct"
)

func GetDiscussions(ctx context.Context, tidb *ti.Database, callingUser *models.User, postId int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-discussions-core")
	defer span.End()
	callerName := "GetDiscussions"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select d.*, r._id as reward_id, color_palette, name, render_in_front, user_status from discussion d inner join (select _id, max(revision) as revision from discussion where post_id = ? group by _id) t on d._id = t._id and d.revision = t.revision left join users u on d.author_id = u._id left join rewards r on r._id = u.avatar_reward limit ? offset ?", postId, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for discussions. GetDiscussions Core.    Error: %v", err)
	}

	// slice to hold results
	discussions := make([]*models.DiscussionBackgroundFrontend, 0)

	// slice to hold discussions that user has up voted
	voted := make([]string, 0)

	defer res.Close()

	if callingUser != nil {
		// query for discussion already liked by calling user
		upVote, err := tidb.QueryContext(ctx, &span, &callerName, "select d._id from discussion d inner join up_vote up on d._id = up.discussion_id and up.user_id = ?", callingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query for discussion up votes. GetDiscussions Core.    Error: %v", err)
		}

		for upVote.Next() {
			var tempId *int64

			err = upVote.Scan(&tempId)
			if err != nil {
				return nil, fmt.Errorf("failed to scan upvote query. GetDiscussions Core.    Error: %v", err)
			}

			if tempId != nil {
				voted = append(voted, strconv.FormatInt(*tempId, 10))
			}
		}

		// explicitly close upVote rows
		upVote.Close()

	}

	// create slice to hold comment lead ids
	var leadIds []string

	for res.Next() {
		var discussion models.DiscussionBackground

		err = sqlstruct.Scan(&discussion, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for project discussion. GetDiscussions Core.    Error: %v", err)
		}

		discussions = append(discussions, discussion.ToFrontend())

		if discussion.Leads {
			leadIds = append(leadIds, strconv.FormatInt(discussion.ID, 10))
		}
	}

	return map[string]interface{}{"discussions": discussions, "lead_ids": leadIds, "up_voted": voted}, nil
}

func GetDiscussionComments(ctx context.Context, tidb *ti.Database, callingUser *models.User, discussionId []int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-discussion-comments-core")
	defer span.End()
	callerName := "GetDiscussionComments"

	// save length of id array to variable
	idLength := len(discussionId)

	// ensure the discussion id array is not empty
	if idLength == 0 {
		return map[string]interface{}{"message": "No comments found"}, nil
	}

	// build first chunk of query variable
	query := "select c.*, r._id as reward_id, color_palette, name, render_in_front, user_status from comment c inner join (select _id, max(revision) as revision from comment where discussion_id "

	// build next query chunk depending on number of discussion ids passed
	if idLength <= 1 {
		query += "= " + strconv.FormatInt(discussionId[0], 10)
	} else {
		query += " in ("
		// iterate over discussion ids appending to query
		for i := range discussionId {
			query += strconv.FormatInt(discussionId[i], 10)
			if (i + 1) < idLength {
				query += ", "
			} else {
				query += ")"
				break
			}
		}
	}

	// append final portion of query
	query += " group by _id) t on c._id = t._id and c.revision = t.revision left join users u on c.author_id = u._id left join rewards r on r._id = u.avatar_reward limit ? offset ?"

	// query for comments with given discussion id and highest revision
	res, err := tidb.QueryContext(ctx, &span, &callerName, query, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any discussion comments. GetDiscussionComments Core.    Error: %v", err)
	}

	// slice to hold discussions that user has up voted
	voted := make([]string, 0)

	if callingUser != nil {
		// query for comment already liked by calling user
		upVote, err := tidb.QueryContext(ctx, &span, &callerName, "select c._id from comment c inner join up_vote up on c._id = up.discussion_id and up.user_id = ?", callingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query for discussion up votes. GetDiscussionComments Core.    Error: %v", err)
		}

		for upVote.Next() {
			var tempId *int64

			err = upVote.Scan(&tempId)
			if err != nil {
				return nil, fmt.Errorf("failed to scan upvote query. GetDiscussionComments Core.    Error: %v", err)
			}

			if tempId != nil {
				voted = append(voted, strconv.FormatInt(*tempId, 10))
			}
		}

		// explicitly close upVote rows
		upVote.Close()
	}

	// slice to hold results
	comments := make([]*models.CommentBackgroundFrontend, 0)

	// create slice to hold comment lead ids
	var leadIds []string

	defer res.Close()

	for res.Next() {
		var comment models.CommentBackground

		err = sqlstruct.Scan(&comment, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. GetDiscussionComments Core.    Error: %v", err)
		}

		comments = append(comments, comment.ToFrontend())

		if comment.Leads {
			leadIds = append(leadIds, strconv.FormatInt(comment.ID, 10))
		}
	}

	return map[string]interface{}{"comments": comments, "lead_ids": leadIds, "up_voted": voted}, nil

}

func GetCommentThreads(ctx context.Context, tidb *ti.Database, callingUser *models.User, commentId []int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-comment-threads-core")
	defer span.End()
	callerName := "GetCommentThreads"

	// save length of id array to variable
	idLength := len(commentId)

	// ensure the comment id array is not empty
	if idLength == 0 {
		return map[string]interface{}{"message": "No threads found"}, nil
	}

	// build first chunk of query variable
	query := "select c.*, r._id as reward_id, color_palette, name, render_in_front, user_status from thread_comment c inner join (select _id, max(revision) as revision from thread_comment where comment_id "

	// build next query chunk depending on number of comment ids passed
	if idLength <= 1 {
		query += "= " + strconv.FormatInt(commentId[0], 10)
	} else {
		query += " in ("
		// iterate over comment ids appending to query
		for i := range commentId {
			query += strconv.FormatInt(commentId[i], 10)
			if (i + 1) < idLength {
				query += ", "
			} else {
				query += ")"
				break
			}
		}
	}

	// append final portion of query
	query += " group by _id) t on c._id = t._id and c.revision = t.revision left join users u on c.author_id = u._id left join rewards r on r._id = u.avatar_reward limit ? offset ?"

	// query thread_comment with comment id and highest revision
	res, err := tidb.QueryContext(ctx, &span, &callerName, query, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for comment threads. GetCommentThreads Core.    Error: %v", err)
	}

	// slice to hold discussions that user has up voted
	voted := make([]string, 0)
	if callingUser != nil {
		// query for thread_comment already liked by calling user
		upVote, err := tidb.QueryContext(ctx, &span, &callerName, "select c._id from thread_comment c inner join up_vote up on c._id = up.discussion_id and up.user_id = ?", callingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query for discussion up votes. GetCommentThreads Core.    Error: %v", err)
		}
		for upVote.Next() {
			var tempId *int64

			err = upVote.Scan(&tempId)
			if err != nil {
				return nil, fmt.Errorf("failed to scan upvote query. GetCommentThreads Core.    Error: %v", err)
			}

			if tempId != nil {
				voted = append(voted, strconv.FormatInt(*tempId, 10))
			}
		}

		// explicitly close upVote rows
		upVote.Close()
	}

	threads := make([]*query_models.ThreadCommentBackgroundFrontend, 0)

	// create slice to hold thread lead ids
	var leadIds []string

	defer res.Close()

	for res.Next() {
		var thread query_models.ThreadCommentBackground

		err = sqlstruct.Scan(&thread, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for res. GetCommentThreads Core.    Error: %v", err)
		}

		threads = append(threads, thread.ToFrontend())

		if thread.Leads {
			leadIds = append(leadIds, strconv.FormatInt(thread.ID, 10))
		}
	}

	return map[string]interface{}{"threads": threads, "lead_ids": leadIds, "up_voted": voted}, nil
}

func GetThreadReply(ctx context.Context, tidb *ti.Database, callingUser *models.User, threadId []int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-thread-reply-core")
	defer span.End()
	callerName := "GetThreadReply"

	// save length of id array to variable
	idLength := len(threadId)

	// ensure the discussion id array is not empty
	if idLength == 0 {
		return map[string]interface{}{"message": "No threads replies found"}, nil
	}

	// build first chunk of query variable
	query := "select c.*, r._id as reward_id, color_palette, name, render_in_front, user_status from thread_reply c inner join (select _id, max(revision) as revision from thread_reply where thread_comment_id "

	// build next query chunk depending on number of discussion ids passed
	if idLength <= 1 {
		query += "= " + strconv.FormatInt(threadId[0], 10)
	} else {
		query += " in ("
		// iterate over discussion ids appending to query
		for i := range threadId {
			query += strconv.FormatInt(threadId[i], 10)
			if (i + 1) < idLength {
				query += ", "
			} else {
				query += ")"
				break
			}
		}
	}

	// append final portion of query
	query += " group by _id) t on c._id = t._id and c.revision = t.revision left join users u on c.author_id = u._id left join rewards r on r._id = u.avatar_reward limit ? offset ?"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, query, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for thread reply: %v\n   query: %s\n    params: %v",
			err, query, []interface{}{limit, skip})
	}

	// slice to hold discussions that user has up voted
	voted := make([]string, 0)

	if callingUser != nil {
		// query for thread_comment already liked by calling user
		upVote, err := tidb.QueryContext(ctx, &span, &callerName, "select c._id from thread_comment c inner join up_vote up on c._id = up.discussion_id and up.user_id = ?", callingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query for discussion up votes. GetThreadReply Core.    Error: %v", err)
		}
		for upVote.Next() {
			var tempId *int64

			err = upVote.Scan(&tempId)
			if err != nil {
				return nil, fmt.Errorf("failed to scan upvote query. GetThreadReply Core.    Error: %v", err)
			}

			if tempId != nil {
				voted = append(voted, strconv.FormatInt(*tempId, 10))
			}
		}

		// explicitly close upVote rows
		upVote.Close()
	}

	threadReplies := make([]*query_models.ThreadReplyBackgroundFrontend, 0)

	defer res.Close()

	for res.Next() {
		var threadReply query_models.ThreadReplyBackground

		err = sqlstruct.Scan(&threadReply, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. GetThreadReply Core.    Error: %v", err)
		}

		threadReplies = append(threadReplies, threadReply.ToFrontend())
	}

	return map[string]interface{}{"thread_reply": threadReplies, "up_voted": voted}, nil
}

func CreateDiscussion(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, callingUser *models.User, sf *snowflake.Node, postId int64, title string, body string, tags []*models.Tag) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-discussion-core")
	defer span.End()
	callerName := "CreateDiscussion"

	// create a new id for the discussion
	id := sf.Generate().Int64()

	// check if title is empty
	if title == "" {
		return map[string]interface{}{"message": "You must create a title for your discussion"}, fmt.Errorf("provided title was empty. CreateDiscussions Core")
	}

	// check if body is empty
	if body == "" {
		return map[string]interface{}{"message": "You must provide content for your discussion"}, fmt.Errorf("provided body was empty. CreateDiscussions Core")
	}

	// create boolean to track failure
	failed := true

	// create slice to hold tag ids
	tagIds := make([]int64, len(tags))
	newTags := make([]interface{}, 0)

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = meili.DeleteDocuments("discussion", id)
		for _, tag := range newTags {
			_ = meili.DeleteDocuments("tags", tag.(*models.TagSearch).ID)
		}
	}()

	// create transaction for discussion insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
	for _, tag := range tags {
		// conditionally create a new id and insert tag into database if it does not already exist
		if tag.ID == -1 {
			// generate new tag id
			tag.ID = sf.Generate().Int64()

			// iterate statements inserting the new tag into the database
			for _, statement := range tag.ToSQLNative() {
				_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
				if err != nil {
					return nil, fmt.Errorf("failed to perform tag insertion: %v", err)
				}
			}

			// add tag to new tags for search engine insertion
			newTags = append(newTags, tag.ToSearch())
		} else {
			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where id =?", tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}
		}

		// append tag id to tag ids slice
		tagIds = append(tagIds, tag.ID)
	}

	// create a new discussion
	discussion, err := models.CreateDiscussion(id, body, callingUser.UserName, callingUser.ID, time.Now(), time.Now(), callingUser.Tier, []int64{}, 0, postId, title, tagIds, false, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create new discussion struct: %v", err)
	}

	// format the discussion into sql insert statements
	statements := discussion.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format discussion into insert statements: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for discussion: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// attempt to insert the discussion into the search engine to make it discoverable
	err = meili.AddDocuments("discussion", discussion)
	if err != nil {
		return nil, fmt.Errorf("failed to add discussion to search engine: %v", err)
	}

	// conditionally attempt to insert the tags into the search engine to make it discoverable
	if len(newTags) > 0 {
		err = meili.AddDocuments("tags", newTags...)
		if err != nil {
			return nil, fmt.Errorf("failed to add new discussion tags to search engine: %v", err)
		}
	}

	// format discussion to frontend object
	discussionFrontend := discussion.ToFrontend()

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction for discussion: %v", err)
	}

	// set failed as false
	failed = false

	return map[string]interface{}{"message": "Discussion has been posted", "discussion": discussionFrontend}, nil
}

func CreateComment(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, callingUser *models.User, sf *snowflake.Node, discussionId int64, body string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-comment-core")
	defer span.End()
	callerName := "CreateComment"

	// create a new id for the comment
	id := sf.Generate().Int64()

	// check if body is empty
	if body == "" {
		return map[string]interface{}{"message": "You must provide content for your comment"}, fmt.Errorf("provided body was empty. CreateComment Core")
	}

	// create boolean to track failure
	failed := true

	// defer function to clean up repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}
		_ = meili.DeleteDocuments("comment", id)
	}()

	// create transaction for comment insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// create a new comment
	comment, err := models.CreateComment(id, body, callingUser.UserName, callingUser.ID, time.Now(), callingUser.Tier, []int64{}, 0, discussionId, false, 0, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to create new comment struct: %v", err)
	}

	// format the comment into sql insert statements
	statements := comment.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format comment into insert statements: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for comment: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// set leads on parent discussion as true
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update discussion set leads = true where _id = ?", discussionId)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update parent discussion: %v", err)
	}

	// attempt to insert the comment into the search engine to make it discoverable
	err = meili.AddDocuments("comment", comment)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment to search engine: %v", err)
	}

	// format discussion to frontend object
	commentFrontend := comment.ToFrontend()

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction for comment: %v", err)
	}

	// set failed as false
	failed = false

	return map[string]interface{}{"message": "Comment has been posted", "comment": commentFrontend}, nil
}

func CreateThreadComment(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, callingUser *models.User, sf *snowflake.Node, commentId int64, body string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-thread-comment-core")
	defer span.End()
	callerName := "CreateThreadComment"

	// create a new id for the comment
	id := sf.Generate().Int64()

	// check if body is empty
	if body == "" {
		return map[string]interface{}{"message": "You must provide content for your comment"}, fmt.Errorf("provided body was empty. CreateThreadComment Core")
	}

	// create boolean to track failure
	failed := true

	// defer function to clean up repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}
		_ = meili.DeleteDocuments("thread_comment", id)
	}()

	// create transaction for comment insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// create a new comment
	threadComment, err := models.CreateThreadComment(id, body, callingUser.UserName, callingUser.ID, time.Now(), callingUser.Tier, 0, commentId, false, 0, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to create new thread_comment struct: %v", err)
	}

	// format the comment into sql insert statements
	statements := threadComment.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format thread_comment into insert statements: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for thread_comment: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// set leads on parent comment as true
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update comment set leads = true where _id = ?", commentId)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update parent comment: %v", err)
	}

	// attempt to insert the comment into the search engine to make it discoverable
	err = meili.AddDocuments("thread_comment", threadComment)
	if err != nil {
		return nil, fmt.Errorf("failed to add thread_comment to search engine: %v", err)
	}

	// format discussion to frontend object
	threadFrontend := threadComment.ToFrontend()

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction for comment: %v", err)
	}

	// set failed as false
	failed = false

	return map[string]interface{}{"message": "Comment has been posted", "thread_comment": threadFrontend}, nil
}

func CreateThreadReply(ctx context.Context, tidb *ti.Database, callingUser *models.User, sf *snowflake.Node, threadId int64, body string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-thread-reply-core")
	defer span.End()
	callerName := "CreateThreadReply"

	// create a new id for the thread reply
	id := sf.Generate().Int64()

	// check if body is empty
	if body == "" {
		return map[string]interface{}{"message": "You must provide content for your comment"}, fmt.Errorf("provided body was empty. CreateThreadReply Core")
	}

	// create transaction for thread reply insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// create a new thread reply
	threadReply, err := models.CreateThreadReply(id, body, callingUser.UserName, callingUser.ID, time.Now(), callingUser.Tier, 0, threadId, 0, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to create new thread_reply struct: %v", err)
	}

	// format the thread reply into sql insert statements
	statements := threadReply.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format thread_reply into insert statements: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for thread_reply: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// format discussion to frontend object
	threadReplyFrontend := threadReply.ToFrontend()

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction for comment: %v", err)
	}

	// set leads on parent thread as true
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update thread_comment set leads = true where _id = ?", threadId)
	if err != nil {
		return nil, fmt.Errorf("failed to update parent thread: %v", err)
	}

	return map[string]interface{}{"message": "Reply has been posted", "thread_reply": threadReplyFrontend}, nil
}

func EditDiscussions(ctx context.Context, tidb *ti.Database, callingUser *models.User, meili *search.MeiliSearchEngine, sf *snowflake.Node, discussionType string, id int64, title *string, body string, tags []*models.Tag) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-discussions-core")
	defer span.End()
	callerName := "EditDiscussions"

	// ensure a body was provided
	if body == "" {
		return map[string]interface{}{"message": "You must provide content for your comment"}, fmt.Errorf("provided body was empty. EditDiscussions Core")
	}

	// ensure title is not empty if it was passed
	if title != nil && *title == "" && discussionType == "discussion" {
		return map[string]interface{}{"message": "Title cannot be empty for discussion"}, fmt.Errorf("provided title was empty. EditDiscussions Core")
	}

	// switch to handle different CommunicationTypes
	switch discussionType {
	case "discussion":
		// model to hold query results
		var oldDiscussion *models.Discussion

		// query for discussion with provided id and most recent revision
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from discussion where _id = ? order by revision desc limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query for discussion with provided id: %v", err)
		}

		// attempt to scan res into discussion model
		for res.Next() {
			oldDiscussion, err = models.DiscussionFromSQLNative(tidb, res)
			if err != nil {
				return nil, fmt.Errorf("failed to scan discussion with provided id: %v", err)
			}
		}

		defer res.Close()

		if oldDiscussion == nil {
			return nil, fmt.Errorf("could not find discussion with provided id: %v", err)
		}

		// create slice to hold tag ids
		tagIds := make([]int64, len(tags))
		newTags := make([]interface{}, 0)

		// create transaction for discussion insertion
		tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create insert tx: %v", err)
		}

		// defer closure of tx
		defer tx.Rollback()

		// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
		for _, tag := range tags {
			// conditionally create a new id and insert tag into database if it does not already exist
			if tag.ID == -1 {
				// generate new tag id
				tag.ID = sf.Generate().Int64()

				// iterate statements inserting the new tag into the database
				for _, statement := range tag.ToSQLNative() {
					_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
					if err != nil {
						return nil, fmt.Errorf("failed to perform tag insertion: %v", err)
					}
				}

				// add tag to new tags for search engine insertion
				newTags = append(newTags, tag.ToSearch())
			} else {
				// increment tag column usage_count in database
				_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where id =?", tag.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
				}
			}

			// append tag id to tag ids slice
			tagIds = append(tagIds, tag.ID)
		}

		// calculate new revision number
		newRevision := oldDiscussion.Revision + 1

		if title == nil {
			title = &oldDiscussion.Title
		}

		// create a new discussion
		discussion, err := models.CreateDiscussion(oldDiscussion.ID, body, callingUser.UserName, callingUser.ID, oldDiscussion.CreatedAt, time.Now(), callingUser.Tier, oldDiscussion.Awards, oldDiscussion.Coffee, oldDiscussion.PostId, *title, tagIds, oldDiscussion.Leads, newRevision, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to create new revision of discussion struct: %v", err)
		}

		// format the discussion into sql insert statements
		statements := discussion.ToSQLNative()
		if err != nil {
			return nil, fmt.Errorf("failed to format discussion into insert statements: %v", err)
		}

		// iterate over insert statements performing insertion into sql
		for _, statement := range statements {
			_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to perform insertion statement for discussion: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
			}
		}

		// Note: I don't defer meili clean up, because it will keep the information of the previous revision in the case of a failure
		// attempt to insert the discussion into the search engine to make it discoverable
		err = meili.AddDocuments("discussion", discussion)
		if err != nil {
			return nil, fmt.Errorf("failed to add discussion to search engine: %v", err)
		}

		// conditionally attempt to insert the tags into the search engine to make it discoverable
		if len(newTags) > 0 {
			err = meili.AddDocuments("tags", newTags...)
			if err != nil {
				return nil, fmt.Errorf("failed to add new discussion tags to search engine: %v", err)
			}
		}

		// format discussion to frontend object
		discussionFrontend := discussion.ToFrontend()

		// commit insert tx
		err = tx.Commit(&callerName)
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction for discussion: %v", err)
		}

		return map[string]interface{}{"message": "Discussion has been successfully edited", "new_discussion": discussionFrontend}, nil

	case "comment":
		// model to hold query results
		var oldComment *models.Comment

		// query for comment with provided id and latest revision
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from comment where _id = ? order by revision desc limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query for comment with provided id: %v", err)
		}

		// attempt to scan res into comment
		for res.Next() {
			oldComment, err = models.CommentFromSQLNative(tidb, res)
			if err != nil {
				return nil, fmt.Errorf("failed to decode query results: %v", err)
			}
		}

		if oldComment == nil {
			return nil, fmt.Errorf("could not find comment with provided id: %v", err)
		}

		defer res.Close()

		// create transaction for comment insertion
		tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create insert tx: %v", err)
		}

		// defer closure of tx
		defer tx.Rollback()

		// calculate new revision number
		newRevision := oldComment.Revision + 1

		// create a new comment
		comment, err := models.CreateComment(oldComment.ID, body, callingUser.UserName, callingUser.ID, time.Now(), callingUser.Tier, oldComment.Awards, oldComment.Coffee, oldComment.DiscussionId, oldComment.Leads, newRevision, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to create new comment struct: %v", err)
		}

		// format the comment into sql insert statements
		statements := comment.ToSQLNative()
		if err != nil {
			return nil, fmt.Errorf("failed to format comment into insert statements: %v", err)
		}

		// iterate over insert statements performing insertion into sql
		for _, statement := range statements {
			_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to perform insertion statement for comment: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
			}
		}

		// attempt to insert the comment into the search engine to make it discoverable
		err = meili.AddDocuments("comment", comment)
		if err != nil {
			return nil, fmt.Errorf("failed to add comment to search engine: %v", err)
		}

		// format discussion to frontend object
		commentFrontend := comment.ToFrontend()

		// commit insert tx
		err = tx.Commit(&callerName)
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction for comment: %v", err)
		}

		return map[string]interface{}{"message": "Comment has been successfully edited", "new_comment": commentFrontend}, nil
	case "thread_comment":
		// model to hold query results
		var oldThread *models.ThreadComment

		// query for thread_comment with provided id and latest revision
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from thread_comment where _id = ? order by revision desc limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query for thread comment with provided id: %v", err)
		}

		// attempt to scan res into comment
		for res.Next() {
			oldThread, err = models.ThreadCommentFromSQLNative(res)
			if err != nil {
				return nil, fmt.Errorf("failed to decode query results: %v", err)
			}
		}

		if oldThread == nil {
			return nil, fmt.Errorf("could not find thread with provided id: %v", err)
		}

		defer res.Close()

		// create transaction for thread comment insertion
		tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create insert tx: %v", err)
		}

		// defer closure of tx
		defer tx.Rollback()

		// calculate new revision number
		newRevision := oldThread.Revision + 1

		// create a new comment
		threadComment, err := models.CreateThreadComment(oldThread.ID, body, callingUser.UserName, callingUser.ID, time.Now(), callingUser.Tier, oldThread.Coffee, oldThread.CommentId, oldThread.Leads, newRevision, 2)
		if err != nil {
			return nil, fmt.Errorf("failed to create new thread_comment struct: %v", err)
		}

		// format the comment into sql insert statements
		statements := threadComment.ToSQLNative()
		if err != nil {
			return nil, fmt.Errorf("failed to format thread_comment into insert statements: %v", err)
		}

		// iterate over insert statements performing insertion into sql
		for _, statement := range statements {
			_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to perform insertion statement for thread_comment: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
			}
		}

		// attempt to insert the comment into the search engine to make it discoverable
		err = meili.AddDocuments("thread_comment", threadComment)
		if err != nil {
			return nil, fmt.Errorf("failed to add thread_comment to search engine: %v", err)
		}

		// format discussion to frontend object
		threadFrontend := threadComment.ToFrontend()

		// commit insert tx
		err = tx.Commit(&callerName)
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction for comment: %v", err)
		}

		return map[string]interface{}{"message": "Comment has been successfully edited", "new_thread_comment": threadFrontend}, nil
	case "thread_reply":
		// model to hold query results
		var oldThreadReply *models.ThreadReply

		// query for thread reply with provided id and most revision
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from thread_reply where _id = ? order by revision desc limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query for thread comment with provided id: %v", err)
		}

		// attempt to scan res into thread reply model
		for res.Next() {
			oldThreadReply, err = models.ThreadReplyFromSQLNative(res)
			if err != nil {
				return nil, fmt.Errorf("failed to decode query results: %v", err)
			}
		}

		if oldThreadReply == nil {
			return nil, fmt.Errorf("could not find thread reply with provided id: %v", err)
		}

		defer res.Close()

		// create transaction for new thread reply revision insertion
		tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create insert tx: %v", err)
		}

		// defer closure of tx
		defer tx.Rollback()

		// calculate new revision number
		newRevision := oldThreadReply.Revision + 1

		// create a new thread reply
		threadReply, err := models.CreateThreadReply(oldThreadReply.ID, body, callingUser.UserName, callingUser.ID, time.Now(), callingUser.Tier, oldThreadReply.Coffee, oldThreadReply.ThreadCommentId, newRevision, 3)
		if err != nil {
			return nil, fmt.Errorf("failed to create new thread_reply struct: %v", err)
		}

		// format the thread reply into sql insert statements
		statements := threadReply.ToSQLNative()
		if err != nil {
			return nil, fmt.Errorf("failed to format thread_reply into insert statements: %v", err)
		}

		// iterate over insert statements performing insertion into sql
		for _, statement := range statements {
			_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to perform insertion statement for thread_reply: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
			}
		}

		// format discussion to frontend object
		threadReplyFrontend := threadReply.ToFrontend()

		// commit insert tx
		err = tx.Commit(&callerName)
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction for comment: %v", err)
		}

		return map[string]interface{}{"message": "Reply has been successfully edited", "new_thread_reply": threadReplyFrontend}, nil
	default:
		return nil, fmt.Errorf("invalid CommunicationType pass. EditDiscussions Core")
	}
}

func AddDiscussionCoffee(ctx context.Context, tidb *ti.Database, callingUser *models.User, sf *snowflake.Node, id int64, discussionType string) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "add-discussion-coffee-core")
	defer span.End()
	callerName := "AddDiscussionCoffee"

	// create variable to hold query string
	query := ""
	var upVote *models.UpVote

	// create id for new upvote
	voteId := sf.Generate().Int64()

	// build queries depending on discussion type
	switch discussionType {

	case "discussion":
		// create new upvote
		upVote = models.CreateUpVote(voteId, 0, id, callingUser.ID)

		query = "Update discussion Set coffee = coffee + 1 where _id = ?"

	case "comment":
		// create new upvote
		upVote = models.CreateUpVote(voteId, 1, id, callingUser.ID)

		query = "Update comment Set coffee = coffee + 1 where _id =?"

	case "thread_comment":
		// create new upvote
		upVote = models.CreateUpVote(voteId, 2, id, callingUser.ID)

		query = "Update thread_comment Set coffee = coffee + 1 where _id =?"

	case "thread_reply":
		// create new upvote
		upVote = models.CreateUpVote(voteId, 3, id, callingUser.ID)

		query = "Update thread_reply Set coffee = coffee + 1 where _id =?"

	default:
		return map[string]interface{}{"message": "invalid CommunicationType passed"}, fmt.Errorf("invalid CommunicationType passed. EditDiscussions Core")
	}

	// generate sql insert statement
	statement := upVote.ToSQLNative()

	// execute insert
	for _, stmt := range statement {
		_, err := tidb.ExecContext(ctx, &span, &callerName, stmt.Statement, stmt.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert upvote: %v", err)
		}
	}

	// perform query
	_, err := tidb.ExecContext(ctx, &span, &callerName, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to add coffee to discussion: %v", err)
	}

	return map[string]interface{}{"message": "Coffee added to discussion"}, nil
}

func RemoveDiscussionCoffee(ctx context.Context, tidb *ti.Database, callingUser *models.User, id int64, discussionType string) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "remove-discussion-coffee-core")
	defer span.End()
	callerName := "RemoveDiscussionCoffee"

	// create variable to hold query string
	updateQuery := ""
	query := ""
	deleteQuery := "delete from up_vote where _id = (select _id from up_vote where discussion_id = ? and user_id = ? limit 1)"

	// build queries depending on discussion type
	switch discussionType {
	case "discussion":
		query = "select * from discussion where _id = ? order by revision desc limit 1"
		updateQuery = "Update discussion Set coffee = coffee - 1 where _id =?"

		// query to ensure coffee isn't going negative after update
		res, err := tidb.QueryContext(ctx, &span, &callerName, query, id)
		if err != nil {
			return nil, fmt.Errorf("failed to query discussion: %v", err)
		}

		// create variable to hold discussion
		var discussion *models.Discussion

		// iterate through results
		for res.Next() {
			discussion, err = models.DiscussionFromSQLNative(tidb, res)
			if err != nil {
				return nil, fmt.Errorf("failed to scan discussion: %v", err)
			}
		}

		if discussion.Coffee < 0 {
			return map[string]interface{}{"message": "Coffee is zero"}, nil
		}

		res.Close()
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	case "comment":
		query = "select * from comment where _id = ? order by revision desc limit 1"
		updateQuery = "Update comment Set coffee = coffee - 1 where _id =?"

		// query to ensure coffee isn't going negative after update
		res, err := tidb.QueryContext(ctx, &span, &callerName, query, id)
		if err != nil {
			return nil, fmt.Errorf("failed to query comment: %v", err)
		}

		// create variable to hold discussion
		var comment *models.Comment

		// iterate through results
		for res.Next() {
			comment, err = models.CommentFromSQLNative(tidb, res)
			if err != nil {
				return nil, fmt.Errorf("failed to scan comment: %v", err)
			}
		}

		if comment.Coffee < 1 {
			return map[string]interface{}{"message": "Coffee is zero"}, nil
		}

		res.Close()
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	case "thread_comment":
		query = "select * from thread_comment where _id = ? order by revision desc limit 1"
		updateQuery = "Update thread_comment Set coffee = coffee - 1 where _id =?"

		// query to ensure coffee isn't going negative after update
		res, err := tidb.QueryContext(ctx, &span, &callerName, query, id)
		if err != nil {
			return nil, fmt.Errorf("failed to query thread_comment: %v", err)
		}

		// create variable to hold discussion
		var thread *models.ThreadComment

		// iterate through results
		for res.Next() {
			thread, err = models.ThreadCommentFromSQLNative(res)
			if err != nil {
				return nil, fmt.Errorf("failed to scan thread_comment: %v", err)
			}
		}

		if thread.Coffee < 1 {
			return map[string]interface{}{"message": "Coffee is zero"}, nil
		}

		res.Close()
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	case "thread_reply":
		query = "select * from thread_reply where _id = ? order by revision desc limit 1"
		updateQuery = "Update thread_reply Set coffee = coffee - 1 where _id =?"

		// query to ensure coffee isn't going negative after update
		res, err := tidb.QueryContext(ctx, &span, &callerName, query, id)
		if err != nil {
			return nil, fmt.Errorf("failed to query thread_reply: %v", err)
		}

		// create variable to hold discussion
		var reply *models.ThreadReply

		// iterate through results
		for res.Next() {
			reply, err = models.ThreadReplyFromSQLNative(res)
			if err != nil {
				return nil, fmt.Errorf("failed to scan thread_reply: %v", err)
			}
		}

		if reply.Coffee < 1 {
			return map[string]interface{}{"message": "Coffee is zero"}, nil
		}

		res.Close()
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	default:
		return map[string]interface{}{"message": "invalid CommunicationType passed"}, fmt.Errorf("invalid CommunicationType passed. EditDiscussions Core")
	}

	// perform update query
	_, err := tidb.ExecContext(ctx, &span, &callerName, updateQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to remove coffee from discussion: %v", err)
	}

	// perform delete query
	_, err = tidb.ExecContext(ctx, &span, &callerName, deleteQuery, id, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete upvote: %v", err)
	}

	return map[string]interface{}{"message": "Coffee removed from discussion"}, nil
}
