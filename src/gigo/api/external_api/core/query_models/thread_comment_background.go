package query_models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
)

type ThreadCommentBackground struct {
	ID              int64                    `json:"_id" sql:"_id"`
	Body            string                   `json:"body" sql:"body"`
	Author          string                   `json:"author" sql:"author"`
	AuthorID        int64                    `json:"author_id" sql:"author_id"`
	CreatedAt       time.Time                `json:"created_at" sql:"created_at"`
	AuthorTier      models.TierType          `json:"author_tier" sql:"author_tier"`
	Coffee          uint64                   `json:"coffee" sql:"coffee"`
	CommentId       int64                    `json:"comment_id" sql:"comment_id"`
	Leads           bool                     `json:"leads" sql:"leads"`
	Revision        int                      `json:"revision" sql:"revision"`
	DiscussionLevel models.CommunicationType `json:"discussion_level" sql:"discussion_level"`
	RewardID        *int64                   `json:"reward_id" sql:"reward_id"`
	Name            *string                  `json:"name" sql:"name"`
	ColorPalette    *string                  `json:"color_palette" sql:"color_palette"`
	RenderInFront   *bool                    `json:"render_in_front" sql:"render_in_front"`
	UserStatus      models.UserStatus        `json:"user_status" sql:"user_status"`
}

type ThreadCommentBackgroundSQL struct {
	ID              int64                    `json:"_id" sql:"_id"`
	Body            string                   `json:"body" sql:"body"`
	Author          string                   `json:"author" sql:"author"`
	AuthorID        int64                    `json:"author_id" sql:"author_id"`
	CreatedAt       time.Time                `json:"created_at" sql:"created_at"`
	AuthorTier      models.TierType          `json:"author_tier" sql:"author_tier"`
	Coffee          uint64                   `json:"coffee" sql:"coffee"`
	CommentId       int64                    `json:"comment_id" sql:"comment_id"`
	Leads           bool                     `json:"leads" sql:"leads"`
	Revision        int                      `json:"revision" sql:"revision"`
	DiscussionLevel models.CommunicationType `json:"discussion_level" sql:"discussion_level"`
	RewardID        *int64                   `json:"reward_id" sql:"reward_id"`
	Name            *string                  `json:"name" sql:"name"`
	ColorPalette    *string                  `json:"color_palette" sql:"color_palette"`
	RenderInFront   *bool                    `json:"render_in_front" sql:"render_in_front"`
	UserStatus      models.UserStatus        `json:"user_status" sql:"user_status"`
}

type ThreadCommentBackgroundFrontend struct {
	ID              string                   `json:"_id" sql:"_id"`
	Body            string                   `json:"body" sql:"body"`
	Author          string                   `json:"author" sql:"author"`
	AuthorID        string                   `json:"author_id" sql:"author_id"`
	CreatedAt       time.Time                `json:"created_at" sql:"created_at"`
	AuthorTier      models.TierType          `json:"author_tier" sql:"author_tier"`
	Coffee          string                   `json:"coffee" sql:"coffee"`
	CommentId       string                   `json:"comment_id" sql:"comment_id"`
	Leads           bool                     `json:"leads" sql:"leads"`
	Revision        int                      `json:"revision" sql:"revision"`
	DiscussionLevel models.CommunicationType `json:"discussion_level" sql:"discussion_level"`
	Thumbnail       string                   `json:"thumbnail"`
	RewardID        *string                  `json:"reward_id" sql:"reward_id"`
	Name            *string                  `json:"name" sql:"name"`
	ColorPalette    *string                  `json:"color_palette" sql:"color_palette"`
	RenderInFront   *bool                    `json:"render_in_front" sql:"render_in_front"`
	UserStatus      models.UserStatus        `json:"user_status" sql:"user_status"`
}

func ThreadCommentBackgroundFromSQLNative(rows *sql.Rows) (*ThreadCommentBackground, error) {
	// create new discussion object to load into
	commentSQL := new(ThreadCommentBackgroundSQL)

	// scan row into comment object
	err := sqlstruct.Scan(commentSQL, rows)
	if err != nil {
		return nil, err
	}

	// create new comment for the output
	comment := &ThreadCommentBackground{
		ID:              commentSQL.ID,
		Body:            commentSQL.Body,
		Author:          commentSQL.Author,
		AuthorID:        commentSQL.AuthorID,
		CreatedAt:       commentSQL.CreatedAt,
		AuthorTier:      commentSQL.AuthorTier,
		Coffee:          commentSQL.Coffee,
		CommentId:       commentSQL.CommentId,
		Leads:           commentSQL.Leads,
		Revision:        commentSQL.Revision,
		DiscussionLevel: commentSQL.DiscussionLevel,
		RewardID:        commentSQL.RewardID,
		Name:            commentSQL.Name,
		ColorPalette:    commentSQL.ColorPalette,
		RenderInFront:   commentSQL.RenderInFront,
		UserStatus:      commentSQL.UserStatus,
	}

	return comment, nil
}

func (i *ThreadCommentBackground) ToFrontend() *ThreadCommentBackgroundFrontend {

	var rewardId *string = nil
	if i.RewardID != nil {
		reward := fmt.Sprintf("%d", *i.RewardID)
		rewardId = &reward
	}

	var colorPalette *string = nil
	if i.ColorPalette != nil {
		colorPalette = i.ColorPalette
	}

	var renderInFront *bool = nil
	if i.RenderInFront != nil {
		renderInFront = i.RenderInFront
	}

	var name *string = nil
	if i.Name != nil {
		name = i.Name
	}

	// create new comment frontend
	mf := &ThreadCommentBackgroundFrontend{
		ID:              fmt.Sprintf("%d", i.ID),
		Body:            i.Body,
		Author:          i.Author,
		AuthorID:        fmt.Sprintf("%d", i.AuthorID),
		CreatedAt:       i.CreatedAt,
		AuthorTier:      i.AuthorTier,
		Coffee:          fmt.Sprintf("%d", i.Coffee),
		CommentId:       fmt.Sprintf("%d", i.CommentId),
		Leads:           i.Leads,
		Revision:        i.Revision,
		DiscussionLevel: i.DiscussionLevel,
		Thumbnail:       fmt.Sprintf("/static/user/pfp/%v", i.AuthorID),
		RewardID:        rewardId,
		Name:            name,
		ColorPalette:    colorPalette,
		RenderInFront:   renderInFront,
		UserStatus:      i.UserStatus,
	}

	return mf
}
