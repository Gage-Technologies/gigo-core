package query_models

import (
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"time"
)

type PostUserStatus struct {
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
	Deleted                 bool                         `json:"deleted" sql:"deleted"`
	ExclusiveDescription    *string                      `json:"exclusive_description,omitempty" sql:"exclusive_description"`
	UserStatus              models.UserStatus            `json:"user_status" sql:"user_status"`
	BackgroundName          *string                      `json:"background_name" sql:"background_name"`
	BackgroundPalette       *string                      `json:"background_palette" sql:"background_palette"`
	BackgroundRender        *bool                        `json:"background_render" sql:"background_render"`
}

type PostUserStatusFrontend struct {
	ID                      string                       `json:"_id"`
	Title                   string                       `json:"title"`
	Description             string                       `json:"description"`
	Author                  string                       `json:"author"`
	AuthorID                string                       `json:"author_id"`
	CreatedAt               time.Time                    `json:"created_at"`
	UpdatedAt               time.Time                    `json:"updated_at"`
	RepoID                  string                       `json:"repo_id"`
	Tier                    models.TierType              `json:"tier"`
	TierString              string                       `json:"tier_string"`
	Awards                  []string                     `json:"awards"`
	TopReply                *string                      `json:"top_reply"`
	Coffee                  uint64                       `json:"coffee"`
	PostType                models.ChallengeType         `json:"post_type"`
	PostTypeString          string                       `json:"post_type_string"`
	Views                   int64                        `json:"views"`
	Completions             int64                        `json:"completions"`
	Attempts                int64                        `json:"attempts"`
	Languages               []models.ProgrammingLanguage `json:"languages"`
	LanguageStrings         []string                     `json:"languages_strings"`
	Published               bool                         `json:"published"`
	Visibility              models.PostVisibility        `json:"visibility"`
	VisibilityString        string                       `json:"visibility_string"`
	Tags                    []string                     `json:"tags"`
	Thumbnail               string                       `json:"thumbnail"`
	ChallengeCost           *string                      `json:"challenge_cost"`
	WorkspaceConfig         string                       `json:"workspace_config"`
	WorkspaceConfigRevision int                          `json:"workspace_config_revision"`
	Leads                   bool                         `json:"leads" sql:"leads"`
	Deleted                 bool                         `json:"deleted" sql:"deleted"`
	ExclusiveDescription    *string                      `json:"exclusive_description"`
	UserStatus              models.UserStatus            `json:"user_status" sql:"user_status"`
	BackgroundName          *string                      `json:"background_name" sql:"background_name"`
	BackgroundPalette       *string                      `json:"background_palette" sql:"background_palette"`
	BackgroundRender        *bool                        `json:"background_render" sql:"background_render"`
}

func (i *PostUserStatus) ToFrontend() (*PostUserStatusFrontend, error) {

	// create slice to hold award ids in string form
	awards := make([]string, 0)

	// iterate award ids formatting them to string format and saving them to the above slice
	for b := range i.Awards {
		awards = append(awards, fmt.Sprintf("%d", b))
	}

	// create slice to hold tag ids in string form
	tags := make([]string, 0)

	// iterate tag ids formatting them to string format and saving them to the above slice
	for b := range i.Tags {
		tags = append(tags, fmt.Sprintf("%d", b))
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

	if i.Deleted {
		return &PostUserStatusFrontend{
			ID:                      fmt.Sprintf("%d", i.ID),
			Title:                   "[Removed]",
			Description:             "This post had been removed by the original author.",
			Author:                  i.Author,
			AuthorID:                fmt.Sprintf("%d", i.AuthorID),
			CreatedAt:               i.CreatedAt,
			UpdatedAt:               i.UpdatedAt,
			RepoID:                  fmt.Sprintf("%d", i.RepoID),
			Tier:                    i.Tier,
			TierString:              i.Tier.String(),
			Awards:                  awards,
			TopReply:                &topReply,
			Coffee:                  i.Coffee,
			PostType:                i.PostType,
			PostTypeString:          i.PostType.String(),
			Views:                   i.Views,
			Attempts:                i.Attempts,
			Completions:             i.Completions,
			Languages:               i.Languages,
			LanguageStrings:         langStrings,
			Published:               i.Published,
			Visibility:              i.Visibility,
			VisibilityString:        i.Visibility.String(),
			Tags:                    tags,
			ChallengeCost:           i.ChallengeCost,
			WorkspaceConfig:         fmt.Sprintf("%d", i.WorkspaceConfig),
			WorkspaceConfigRevision: i.WorkspaceConfigRevision,
			Leads:                   i.Leads,
			Thumbnail:               fmt.Sprintf("/static/posts/t/%v", i.ID),
			Deleted:                 true,
			ExclusiveDescription:    i.ExclusiveDescription,
			UserStatus:              i.UserStatus,
			BackgroundName:          i.BackgroundName,
			BackgroundPalette:       i.BackgroundPalette,
			BackgroundRender:        i.BackgroundRender,
		}, nil
	}

	// // hash id for thumbnail path
	// idHash, err := utils.HashData([]byte(fmt.Sprintf("%d", i.ID)))
	// if err != nil {
	//	return nil, fmt.Errorf("failed to hash post id: %v", err)
	// }

	// create new post frontend
	mf := &PostUserStatusFrontend{
		ID:                      fmt.Sprintf("%d", i.ID),
		Title:                   i.Title,
		Description:             i.Description,
		Author:                  i.Author,
		AuthorID:                fmt.Sprintf("%d", i.AuthorID),
		CreatedAt:               i.CreatedAt,
		UpdatedAt:               i.UpdatedAt,
		RepoID:                  fmt.Sprintf("%d", i.RepoID),
		Tier:                    i.Tier,
		TierString:              i.Tier.String(),
		Awards:                  awards,
		TopReply:                &topReply,
		Coffee:                  i.Coffee,
		PostType:                i.PostType,
		PostTypeString:          i.PostType.String(),
		Views:                   i.Views,
		Attempts:                i.Attempts,
		Completions:             i.Completions,
		Languages:               i.Languages,
		LanguageStrings:         langStrings,
		Published:               i.Published,
		Visibility:              i.Visibility,
		VisibilityString:        i.Visibility.String(),
		Tags:                    tags,
		ChallengeCost:           i.ChallengeCost,
		WorkspaceConfig:         fmt.Sprintf("%d", i.WorkspaceConfig),
		WorkspaceConfigRevision: i.WorkspaceConfigRevision,
		Leads:                   i.Leads,
		Thumbnail:               fmt.Sprintf("/static/posts/t/%v", i.ID),
		ExclusiveDescription:    i.ExclusiveDescription,
		UserStatus:              i.UserStatus,
		BackgroundName:          i.BackgroundName,
		BackgroundPalette:       i.BackgroundPalette,
		BackgroundRender:        i.BackgroundRender,
	}

	return mf, nil
}
