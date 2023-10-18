package query_models

import (
	"fmt"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
)

type RecommendedPostMerge struct {
	ID          int64           `json:"_id" sql:"_id"`
	Title       string          `json:"title" sql:"title"`
	Author      string          `json:"author" sql:"author"`
	AuthorID    int64           `json:"author_id" sql:"author_id"`
	Description string          `json:"description" sql:"description"`
	CreatedAt   time.Time       `json:"created_at" sql:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" sql:"updated_at"`
	RepoID      int64           `json:"repo_id" sql:"repo_id"`
	Tier        models.TierType `json:"tier" sql:"tier"`
	Awards      []int64         `json:"awards" sql:"awards"`
	TopReply    *int64          `json:"top_reply,omitempty" sql:"top_reply"`
	Coffee      uint64          `json:"coffee" sql:"coffee"`
	Tags        []string        `json:"tags" sql:"tags"`
	// TODO eg interactive, playground, community, verified...
	PostType          models.ChallengeType         `json:"post_type" sql:"post_type"`
	Views             int64                        `json:"views" sql:"views"`
	Completions       int64                        `json:"completions" sql:"completions"`
	Attempts          int64                        `json:"attempts" sql:"attempts"`
	Thumbnail         string                       `json:"thumbnail" sql:"thumbnail"`
	Languages         []models.ProgrammingLanguage `json:"languages" sql:"languages"`
	Similarity        float32                      `json:"similarity" sql:"similarity"`
	RecommendedID     int64                        `json:"recommended_id" sql:"recommended_id"`
	UserTier          models.TierType              `json:"user_tier" sql:"user_tier"`
	BackgroundName    *string                      `json:"background_name" sql:"background_name"`
	BackgroundPalette *string                      `json:"background_palette" sql:"background_palette"`
	BackgroundRender  *bool                        `json:"background_render" sql:"background_render"`
	ChallengeCost     *string                      `json:"challenge_cost" sql:"challenge_cost"`
	UserStatus        models.UserStatus            `json:"user_status" sql:"user_status"`
}

type RecommendedPostMergeFrontend struct {
	ID                string                       `json:"_id" sql:"_id"`
	Title             string                       `json:"title" sql:"title"`
	Author            string                       `json:"author" sql:"author"`
	Description       string                       `json:"description" sql:"description"`
	AuthorID          string                       `json:"author_id" sql:"author_id"`
	CreatedAt         time.Time                    `json:"created_at" sql:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at" sql:"updated_at"`
	RepoID            string                       `json:"repo_id" sql:"repo_id"`
	Tier              models.TierType              `json:"tier" sql:"tier"`
	Awards            []string                     `json:"awards" sql:"awards"`
	TopReply          *string                      `json:"top_reply,omitempty" sql:"top_reply"`
	Coffee            string                       `json:"coffee" sql:"coffee"`
	PostType          string                       `json:"post_type" sql:"post_type"`
	Views             string                       `json:"views" sql:"views"`
	Completions       string                       `json:"completions" sql:"completions"`
	Attempts          string                       `json:"attempts" sql:"attempts"`
	Thumbnail         string                       `json:"thumbnail" sql:"thumbnail"`
	Languages         []models.ProgrammingLanguage `json:"languages"`
	LanguageStrings   []string                     `json:"languages_strings"`
	Similarity        string                       `json:"similarity" sql:"similarity"`
	RecommendedID     string                       `json:"recommended_id" sql:"recommended_id"`
	UserTier          models.TierType              `json:"user_tier" sql:"user_tier"`
	BackgroundName    *string                      `json:"background_name" sql:"background_name"`
	BackgroundPalette *string                      `json:"background_palette" sql:"background_palette"`
	BackgroundRender  *bool                        `json:"background_render" sql:"background_render"`
	ChallengeCost     *string                      `json:"challenge_cost" sql:"challenge_cost"`
	UserStatus        models.UserStatus            `json:"user_status" sql:"user_status"`
}

func (i *RecommendedPostMerge) ToFrontend() *RecommendedPostMergeFrontend {
	awards := make([]string, 0)

	for b := range i.Awards {
		awards = append(awards, fmt.Sprintf("%d", b))
	}

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

	// create new post frontend
	mf := &RecommendedPostMergeFrontend{
		ID:                fmt.Sprintf("%d", i.ID),
		Title:             i.Title,
		Author:            i.Author,
		AuthorID:          fmt.Sprintf("%d", i.AuthorID),
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
		RepoID:            fmt.Sprintf("%d", i.RepoID),
		Tier:              i.Tier,
		Awards:            awards,
		TopReply:          &topReply,
		Coffee:            fmt.Sprintf("%d", i.Coffee),
		PostType:          i.PostType.String(),
		Views:             fmt.Sprintf("%d", i.Views),
		Attempts:          fmt.Sprintf("%d", i.Attempts),
		Completions:       fmt.Sprintf("%d", i.Completions),
		Thumbnail:         fmt.Sprintf("/static/posts/t/%v", i.ID),
		Languages:         i.Languages,
		LanguageStrings:   langStrings,
		Similarity:        fmt.Sprintf("%v", i.Similarity),
		RecommendedID:     fmt.Sprintf("%d", i.RecommendedID),
		Description:       i.Description,
		UserTier:          i.UserTier,
		BackgroundName:    i.BackgroundName,
		BackgroundPalette: i.BackgroundPalette,
		BackgroundRender:  i.BackgroundRender,
		ChallengeCost:     i.ChallengeCost,
		UserStatus:        i.UserStatus,
	}

	return mf
}
