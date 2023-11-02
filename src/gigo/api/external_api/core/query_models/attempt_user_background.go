package query_models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
)

type AttemptUserBackground struct {
	ID                int64                     `json:"_id" sql:"_id"`
	PostTitle         string                    `json:"post_title" sql:"post_title"`
	Description       string                    `json:"description" sql:"description"`
	Author            string                    `json:"author" sql:"author"`
	AuthorID          int64                     `json:"author_id" sql:"author_id"`
	CreatedAt         time.Time                 `json:"created_at" sql:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at" sql:"updated_at"`
	RepoID            int64                     `json:"repo_id" sql:"repo_id"`
	AuthorTier        models.TierType           `json:"author_tier" sql:"author_tier"`
	Awards            []int64                   `json:"awards" sql:"awards"`
	Coffee            uint64                    `json:"coffee" sql:"coffee"`
	PostID            int64                     `json:"post_id" sql:"post_id"`
	Closed            bool                      `json:"closed" sql:"closed"`
	Success           bool                      `json:"success" sql:"success"`
	ClosedDate        *time.Time                `json:"closed_date" sql:"closed_date"`
	Tier              models.TierType           `json:"tier" sql:"tier"`
	ParentAttempt     *int64                    `json:"parent_attempt" sql:"parent_attempt"`
	WorkspaceSettings *models.WorkspaceSettings `json:"workspace_settings" sql:"workspace_settings"`
	RewardID          *int64                    `json:"reward_id" sql:"reward_id"`
	Name              *string                   `json:"name" sql:"name"`
	ColorPalette      *string                   `json:"color_palette" sql:"color_palette"`
	RenderInFront     *bool                     `json:"render_in_front" sql:"render_in_front"`
	PostType          models.ChallengeType      `json:"post_type" sql:"post_type"`
}

type AttemptUserBackgroundSQL struct {
	ID                int64                `json:"_id" sql:"_id"`
	PostTitle         string               `json:"post_title" sql:"post_title"`
	Description       string               `json:"description" sql:"description"`
	Author            string               `json:"author" sql:"author"`
	AuthorID          int64                `json:"author_id" sql:"author_id"`
	CreatedAt         time.Time            `json:"created_at" sql:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at" sql:"updated_at"`
	RepoID            int64                `json:"repo_id" sql:"repo_id"`
	AuthorTier        models.TierType      `json:"author_tier" sql:"author_tier"`
	Awards            []byte               `json:"awards" sql:"awards"`
	Coffee            uint64               `json:"coffee" sql:"coffee"`
	PostID            int64                `json:"post_id" sql:"post_id"`
	Closed            bool                 `json:"closed" sql:"closed"`
	Success           bool                 `json:"success" sql:"success"`
	ClosedDate        *time.Time           `json:"closed_date" sql:"closed_date"`
	Tier              models.TierType      `json:"tier" sql:"tier"`
	ParentAttempt     *int64               `json:"parent_attempt" sql:"parent_attempt"`
	WorkspaceSettings []byte               `json:"workspace_settings" sql:"workspace_settings"`
	RewardID          *int64               `json:"reward_id" sql:"reward_id"`
	Name              *string              `json:"name" sql:"name"`
	ColorPalette      *string              `json:"color_palette" sql:"color_palette"`
	RenderInFront     *bool                `json:"render_in_front" sql:"render_in_front"`
	PostType          models.ChallengeType `json:"post_type" sql:"post_type"`
}

type AttemptUserBackgroundFrontend struct {
	ID             string               `json:"_id" sql:"_id"`
	PostTitle      string               `json:"post_title" sql:"post_title"`
	Description    string               `json:"description" sql:"description"`
	Author         string               `json:"author" sql:"author"`
	AuthorID       string               `json:"author_id" sql:"author_id"`
	CreatedAt      time.Time            `json:"created_at" sql:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at" sql:"updated_at"`
	RepoID         string               `json:"repo_id" sql:"repo_id"`
	AuthorTier     models.TierType      `json:"author_tier" sql:"author_tier"`
	Awards         []string             `json:"awards" sql:"awards"`
	Coffee         string               `json:"coffee" sql:"coffee"`
	PostID         string               `json:"post_id" sql:"post_id"`
	Closed         bool                 `json:"closed" sql:"closed"`
	Success        bool                 `json:"success" sql:"success"`
	ClosedDate     *time.Time           `json:"closed_date" sql:"closed_date"`
	Tier           models.TierType      `json:"tier" sql:"tier"`
	ParentAttempt  *string              `json:"parent_attempt" sql:"parent_attempt"`
	Thumbnail      string               `json:"thumbnail"`
	RewardID       *string              `json:"reward_id" sql:"reward_id"`
	Name           *string              `json:"name" sql:"name"`
	ColorPalette   *string              `json:"color_palette" sql:"color_palette"`
	RenderInFront  *bool                `json:"render_in_front" sql:"render_in_front"`
	PostType       models.ChallengeType `json:"post_type" sql:"post_type"`
	PostTypeString string               `json:"post_type_string" sql:"post_type_string"`
}

func AttemptUserBackgroundFromSQLNative(ctx context.Context, db *ti.Database, rows *sql.Rows) (*AttemptUserBackground, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "attempt-user-background-from-sql-native-core")
	defer span.End()
	callerName := "AttemptUserBackgroundFromSQLNative"

	// create new attempt object to load into
	attemptSQL := new(AttemptUserBackgroundSQL)

	// scan row into attempt object
	err := sqlstruct.Scan(attemptSQL, rows)
	if err != nil {
		return nil, err
	}

	awardRows, err := db.QueryContext(ctx, &span, &callerName, "select award_id from attempt_awards where attempt_id = ?", attemptSQL.ID)
	if err != nil {
		return nil, err
	}

	defer awardRows.Close()

	awards := make([]int64, 0)

	for awardRows.Next() {
		var award int64
		err = awardRows.Scan(&award)
		if err != nil {
			return nil, err
		}
		awards = append(awards, award)
	}

	// create workspace settings to unmarshall into
	var workspaceSettings *models.WorkspaceSettings
	if attemptSQL.WorkspaceSettings != nil {
		var ws models.WorkspaceSettings
		err = json.Unmarshal(attemptSQL.WorkspaceSettings, &ws)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall workspace settings: %v", err)
		}
		workspaceSettings = &ws
	}

	// create new attempt for the output
	attempt := &AttemptUserBackground{
		ID:                attemptSQL.ID,
		PostTitle:         attemptSQL.PostTitle,
		Description:       attemptSQL.Description,
		Author:            attemptSQL.Author,
		AuthorID:          attemptSQL.AuthorID,
		CreatedAt:         attemptSQL.CreatedAt,
		UpdatedAt:         attemptSQL.UpdatedAt,
		RepoID:            attemptSQL.RepoID,
		AuthorTier:        attemptSQL.AuthorTier,
		Coffee:            attemptSQL.Coffee,
		Awards:            awards,
		PostID:            attemptSQL.PostID,
		Closed:            attemptSQL.Closed,
		Success:           attemptSQL.Success,
		ClosedDate:        attemptSQL.ClosedDate,
		Tier:              attemptSQL.Tier,
		ParentAttempt:     attemptSQL.ParentAttempt,
		WorkspaceSettings: workspaceSettings,
		RewardID:          attemptSQL.RewardID,
		Name:              attemptSQL.Name,
		ColorPalette:      attemptSQL.ColorPalette,
		RenderInFront:     attemptSQL.RenderInFront,
		PostType:          attemptSQL.PostType,
	}

	return attempt, nil
}

func (i *AttemptUserBackground) ToFrontend() *AttemptUserBackgroundFrontend {
	awards := make([]string, 0)

	for b := range i.Awards {
		awards = append(awards, fmt.Sprintf("%d", b))
	}

	// conditionally set parent attempt
	var parentAttempt *string
	if i.ParentAttempt != nil {
		pa := fmt.Sprintf("%d", *i.ParentAttempt)
		parentAttempt = &pa
	}

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

	// create new attempt frontend
	mf := &AttemptUserBackgroundFrontend{
		ID:             fmt.Sprintf("%d", i.ID),
		PostTitle:      i.PostTitle,
		Description:    i.Description,
		Author:         i.Author,
		AuthorID:       fmt.Sprintf("%d", i.AuthorID),
		CreatedAt:      i.CreatedAt,
		UpdatedAt:      i.UpdatedAt,
		RepoID:         fmt.Sprintf("%d", i.RepoID),
		AuthorTier:     i.AuthorTier,
		Awards:         awards,
		PostID:         fmt.Sprintf("%d", i.PostID),
		Coffee:         fmt.Sprintf("%d", i.Coffee),
		Closed:         i.Closed,
		Success:        i.Success,
		ClosedDate:     i.ClosedDate,
		Tier:           i.Tier,
		ParentAttempt:  parentAttempt,
		Thumbnail:      fmt.Sprintf("/static/posts/t/%v", i.PostID),
		RewardID:       rewardId,
		Name:           name,
		ColorPalette:   colorPalette,
		RenderInFront:  renderInFront,
		PostType:       i.PostType,
		PostTypeString: i.PostType.String(),
	}

	return mf
}
