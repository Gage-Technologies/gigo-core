package query_models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
)

type PostUserBackground struct {
	ID                      int64                        `json:"_id" sql:"_id"`
	Title                   string                       `json:"title" sql:"title"`
	Description             string                       `json:"description" sql:"description"`
	Author                  string                       `json:"author" sql:"author"`
	AuthorID                int64                        `json:"author_id" sql:"author_id"`
	CreatedAt               time.Time                    `json:"created_at" sql:"created_at"`
	UpdatedAt               time.Time                    `json:"updated_at" sql:"updated_at"`
	RepoID                  int64                        `json:"repo_id" sql:"repo_id"`
	Tier                    models.TierType              `json:"tier" sql:"tier"`
	Awards                  []int64                      `json:"awards" sql:"awards"`
	TopReply                *int64                       `json:"top_reply,omitempty" sql:"top_reply"`
	Coffee                  uint64                       `json:"coffee" sql:"coffee"`
	Tags                    []int64                      `json:"tags" sql:"tags"`
	TagStrings              []string                     `json:"tag_strings" sql:"tag_strings"`
	PostType                models.ChallengeType         `json:"post_type" sql:"post_type"`
	Views                   int64                        `json:"views" sql:"views"`
	Completions             int64                        `json:"completions" sql:"completions"`
	Attempts                int64                        `json:"attempts" sql:"attempts"`
	Languages               []models.ProgrammingLanguage `json:"languages" sql:"languages"`
	Published               bool                         `json:"published" sql:"published"`
	Visibility              models.PostVisibility        `json:"visibility" sql:"visibility"`
	StripePriceId           *string                      `json:"stripe_price_id" sql:"stripe_price_id"`
	ChallengeCost           *string                      `json:"challenge_cost" sql:"challenge_cost"`
	WorkspaceConfig         int64                        `json:"workspace_config" sql:"workspace_config"`
	WorkspaceConfigRevision int                          `json:"workspace_config_revision" sql:"workspace_config_revision"`
	WorkspaceSettings       *models.WorkspaceSettings    `json:"workspace_settings" sql:"workspace_settings"`
	Leads                   bool                         `json:"leads" sql:"leads"`
	Embedded                bool                         `json:"embedded" sql:"embedded"`
	RewardID                *int64                       `json:"reward_id" sql:"reward_id"`
	Name                    *string                      `json:"name" sql:"name"`
	ColorPalette            *string                      `json:"color_palette" sql:"color_palette"`
	RenderInFront           *bool                        `json:"render_in_front" sql:"render_in_front"`
	Deleted                 bool                         `json:"deleted" sql:"deleted"`
	HasAccess               *bool                        `json:"has_access" sql:"has_access"`
	ExclusiveDescription    *string                      `json:"exclusive_description,omitempty" sql:"exclusive_description"`
	EstimatedTutorialTime   *time.Duration               `json:"estimated_tutorial_time,omitempty" sql:"estimated_tutorial_time"`
}

type PostUserBackgroundSQL struct {
	ID                      int64                 `json:"_id" sql:"_id"`
	Title                   string                `json:"title" sql:"title"`
	Description             string                `json:"description" sql:"description"`
	Author                  string                `json:"author" sql:"author"`
	AuthorID                int64                 `json:"author_id" sql:"author_id"`
	CreatedAt               time.Time             `json:"created_at" sql:"created_at"`
	UpdatedAt               time.Time             `json:"updated_at" sql:"updated_at"`
	RepoID                  int64                 `json:"repo_id" sql:"repo_id"`
	Tier                    models.TierType       `json:"tier" sql:"tier"`
	TopReply                *int64                `json:"top_reply,omitempty" sql:"top_reply"`
	Coffee                  uint64                `json:"coffee" sql:"coffee"`
	PostType                models.ChallengeType  `json:"post_type" sql:"post_type"`
	Views                   int64                 `json:"views" sql:"views"`
	Completions             int64                 `json:"completions" sql:"completions"`
	Attempts                int64                 `json:"attempts" sql:"attempts"`
	Published               bool                  `json:"published" sql:"published"`
	Visibility              models.PostVisibility `json:"visibility" sql:"visibility"`
	StripePriceId           *string               `json:"stripe_price_id" sql:"stripe_price_id"`
	ChallengeCost           *string               `json:"challenge_cost" sql:"challenge_cost"`
	WorkspaceConfig         int64                 `json:"workspace_config" sql:"workspace_config"`
	WorkspaceConfigRevision int                   `json:"workspace_config_revision" sql:"workspace_config_revision"`
	WorkspaceSettings       []byte                `json:"workspace_settings" sql:"workspace_settings"`
	Leads                   bool                  `json:"leads" sql:"leads"`
	Embedding               bool                  `json:"embedded" sql:"embedded"`
	RewardID                *int64                `json:"reward_id" sql:"reward_id"`
	Name                    *string               `json:"name" sql:"name"`
	ColorPalette            *string               `json:"color_palette" sql:"color_palette"`
	RenderInFront           *bool                 `json:"render_in_front" sql:"render_in_front"`
	Deleted                 bool                  `json:"deleted" sql:"deleted"`
	HasAccess               *bool                 `json:"has_access" sql:"has_access"`
	ExclusiveDescription    *string               `json:"exclusive_description,omitempty" sql:"exclusive_description"`
	EstimatedTutorialTime   *time.Duration        `json:"estimated_tutorial_time,omitempty" sql:"estimated_tutorial_time"`
}

type PostUserBackgroundFrontend struct {
	ID                          string                       `json:"_id"`
	Title                       string                       `json:"title"`
	Description                 string                       `json:"description"`
	Author                      string                       `json:"author"`
	AuthorID                    string                       `json:"author_id"`
	CreatedAt                   time.Time                    `json:"created_at"`
	UpdatedAt                   time.Time                    `json:"updated_at"`
	RepoID                      string                       `json:"repo_id"`
	Tier                        models.TierType              `json:"tier"`
	TierString                  string                       `json:"tier_string"`
	Awards                      []string                     `json:"awards"`
	TopReply                    *string                      `json:"top_reply"`
	Coffee                      uint64                       `json:"coffee"`
	PostType                    models.ChallengeType         `json:"post_type"`
	PostTypeString              string                       `json:"post_type_string"`
	Views                       int64                        `json:"views"`
	Completions                 int64                        `json:"completions"`
	Attempts                    int64                        `json:"attempts"`
	Languages                   []models.ProgrammingLanguage `json:"languages"`
	LanguageStrings             []string                     `json:"languages_strings"`
	Published                   bool                         `json:"published"`
	Visibility                  models.PostVisibility        `json:"visibility"`
	VisibilityString            string                       `json:"visibility_string"`
	Tags                        []string                     `json:"tags"`
	TagStrings                  []string                     `json:"tag_strings"`
	Thumbnail                   string                       `json:"thumbnail"`
	ChallengeCost               *string                      `json:"challenge_cost"`
	WorkspaceConfig             string                       `json:"workspace_config"`
	WorkspaceConfigRevision     int                          `json:"workspace_config_revision"`
	Leads                       bool                         `json:"leads" sql:"leads"`
	RewardID                    *string                      `json:"reward_id" sql:"reward_id"`
	Name                        *string                      `json:"name" sql:"name"`
	ColorPalette                *string                      `json:"color_palette" sql:"color_palette"`
	RenderInFront               *bool                        `json:"render_in_front" sql:"render_in_front"`
	Deleted                     bool                         `json:"deleted" sql:"deleted"`
	HasAccess                   *bool                        `json:"has_access" sql:"has_access"`
	StripePriceId               *string                      `json:"stripe_price_id" sql:"stripe_price_id"`
	ExclusiveDescription        *string                      `json:"exclusive_description,omitempty" sql:"exclusive_description"`
	EstimatedTutorialTimeMillis *int64                       `json:"estimated_tutorial_time_millis,omitempty" sql:"estimated_tutorial_time_millis"`
}

func PostUserBackgroundFromSQLNative(ctx context.Context, db *ti.Database, rows *sql.Rows) (*PostUserBackground, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "post-user-background-from-sql-native-core")
	defer span.End()
	callerName := "PostUserBackgroundFromSQLNative"

	// create new post object to load into
	postSQL := new(PostUserBackgroundSQL)

	// scan row into post object
	err := sqlstruct.Scan(postSQL, rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan post into sql object: %v", err)
	}

	// query link table to get award ids
	awardRows, err := db.QueryContext(ctx, &span, &callerName, "select award_id from post_awards where post_id = ?", postSQL.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query post awards link table: %v", err)
	}

	// defer closure of cursor
	defer awardRows.Close()

	// create slice to hold award ids loaded from cursor
	awards := make([]int64, 0)

	// iterate cursor loading award ids and saving to id slice created abov
	for awardRows.Next() {
		var award int64
		err = awardRows.Scan(&award)
		if err != nil {
			return nil, fmt.Errorf("failed to scan award id from link table cursor: %v", err)
		}
		awards = append(awards, award)
	}

	// query tag link table to get tab ids
	tagRows, err := db.QueryContext(ctx, &span, &callerName, "select pt.tag_id, t.value from post_tags pt join tag t on pt.tag_id = t._id where pt.post_id = ?", postSQL.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tag link table for tag ids: %v", err)
	}

	// defer closure of tag rows
	defer tagRows.Close()

	// create slice to hold tag ids
	tags := make([]int64, 0)
	tagValues := make([]string, 0)

	// iterate cursor scanning tag ids and saving the to the slice created above
	for tagRows.Next() {
		var tag int64
		var value string
		err = tagRows.Scan(&tag, &value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag id from link tbale cursor: %v", err)
		}
		tags = append(tags, tag)
		tagValues = append(tagValues, value)
	}

	// query lang link table to get lang ids
	langRows, err := db.QueryContext(ctx, &span, &callerName, "select lang_id from post_langs where post_id =?", postSQL.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query lang link table for lang ids: %v", err)
	}

	// defer closure of lang rows
	defer langRows.Close()

	// create slice to hold lang ids
	languages := make([]models.ProgrammingLanguage, 0)

	// iterate cursor scanning lang ids and saving the to the slice created above
	for langRows.Next() {
		var lang models.ProgrammingLanguage
		err = langRows.Scan(&lang)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lang id from link table cursor: %v", err)
		}
		languages = append(languages, lang)
	}

	// create workspace settings to unmarshall into
	var workspaceSettings *models.WorkspaceSettings
	if postSQL.WorkspaceSettings != nil {
		var ws models.WorkspaceSettings
		err = json.Unmarshal(postSQL.WorkspaceSettings, &ws)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall workspace settings: %v", err)
		}
		workspaceSettings = &ws
	}

	// create new post for the output
	post := &PostUserBackground{
		ID:                      postSQL.ID,
		Title:                   postSQL.Title,
		Description:             postSQL.Description,
		Author:                  postSQL.Author,
		AuthorID:                postSQL.AuthorID,
		CreatedAt:               postSQL.CreatedAt,
		UpdatedAt:               postSQL.UpdatedAt,
		RepoID:                  postSQL.RepoID,
		TopReply:                postSQL.TopReply,
		Tier:                    postSQL.Tier,
		Awards:                  awards,
		Coffee:                  postSQL.Coffee,
		PostType:                postSQL.PostType,
		Views:                   postSQL.Views,
		Completions:             postSQL.Completions,
		Attempts:                postSQL.Attempts,
		Languages:               languages,
		Published:               postSQL.Published,
		Visibility:              postSQL.Visibility,
		Tags:                    tags,
		TagStrings:              tagValues,
		ChallengeCost:           postSQL.ChallengeCost,
		StripePriceId:           postSQL.StripePriceId,
		WorkspaceConfig:         postSQL.WorkspaceConfig,
		WorkspaceConfigRevision: postSQL.WorkspaceConfigRevision,
		WorkspaceSettings:       workspaceSettings,
		Leads:                   postSQL.Leads,
		Embedded:                postSQL.Embedding,
		RewardID:                postSQL.RewardID,
		Name:                    postSQL.Name,
		ColorPalette:            postSQL.ColorPalette,
		RenderInFront:           postSQL.RenderInFront,
		Deleted:                 postSQL.Deleted,
		ExclusiveDescription:    postSQL.ExclusiveDescription,
		EstimatedTutorialTime:   postSQL.EstimatedTutorialTime,
	}

	return post, nil
}

func (i *PostUserBackground) ToFrontend() (*PostUserBackgroundFrontend, error) {
	// create slice to hold award ids in string form
	awards := make([]string, 0)

	// iterate award ids formatting them to string format and saving them to the above slice
	for b := range i.Awards {
		awards = append(awards, fmt.Sprintf("%d", b))
	}

	// create slice to hold tag ids in string form
	tags := make([]string, 0)
	tagValues := make([]string, 0)

	// iterate tag ids formatting them to string format and saving them to the above slice
	for _, b := range i.Tags {
		tags = append(tags, fmt.Sprintf("%d", b))
	}
	for _, b := range i.TagStrings {
		tagValues = append(tagValues, b)
	}

	// conditionally format top reply id into string
	var topReply string
	if i.TopReply != nil {
		topReply = fmt.Sprintf("%d", *i.TopReply)
	}

	// create slice to hold language strings
	langStrings := make([]string, 0)

	// iterate language ids formatting them to string format and saving them to the above slice
	for _, l := range i.Languages {
		langStrings = append(langStrings, l.String())
	}

	// // hash id for thumbnail path
	// idHash, err := utils.HashData([]byte(fmt.Sprintf("%d", i.ID)))
	// if err != nil {
	//	return nil, fmt.Errorf("failed to hash post id: %v", err)
	// }

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

	var priceId *string = nil
	if i.StripePriceId != nil {
		priceId = i.StripePriceId
	}

	var exclusiveDescription *string = nil
	if i.ExclusiveDescription != nil {
		exclusiveDescription = i.ExclusiveDescription
	}

	var tutorialMillis *int64
	if i.EstimatedTutorialTime != nil {
		millis := i.EstimatedTutorialTime.Milliseconds()
		tutorialMillis = &millis
	}

	// create new post frontend
	mf := &PostUserBackgroundFrontend{
		ID:                          fmt.Sprintf("%d", i.ID),
		Title:                       i.Title,
		Description:                 i.Description,
		Author:                      i.Author,
		AuthorID:                    fmt.Sprintf("%d", i.AuthorID),
		CreatedAt:                   i.CreatedAt,
		UpdatedAt:                   i.UpdatedAt,
		RepoID:                      fmt.Sprintf("%d", i.RepoID),
		Tier:                        i.Tier,
		TierString:                  i.Tier.String(),
		Awards:                      awards,
		TopReply:                    &topReply,
		Coffee:                      i.Coffee,
		PostType:                    i.PostType,
		PostTypeString:              i.PostType.String(),
		Views:                       i.Views,
		Attempts:                    i.Attempts,
		Completions:                 i.Completions,
		Languages:                   i.Languages,
		LanguageStrings:             langStrings,
		Published:                   i.Published,
		Visibility:                  i.Visibility,
		VisibilityString:            i.Visibility.String(),
		Tags:                        tags,
		TagStrings:                  tagValues,
		ChallengeCost:               i.ChallengeCost,
		WorkspaceConfig:             fmt.Sprintf("%d", i.WorkspaceConfig),
		WorkspaceConfigRevision:     i.WorkspaceConfigRevision,
		Leads:                       i.Leads,
		Thumbnail:                   fmt.Sprintf("/static/posts/t/%v", i.ID),
		RewardID:                    rewardId,
		Name:                        name,
		ColorPalette:                colorPalette,
		RenderInFront:               renderInFront,
		Deleted:                     i.Deleted,
		StripePriceId:               priceId,
		ExclusiveDescription:        exclusiveDescription,
		EstimatedTutorialTimeMillis: tutorialMillis,
	}

	return mf, nil
}
