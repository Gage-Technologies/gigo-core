package query_models

import (
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
)

type AttemptPostMerge struct {
	PostId       int64                `json:"post_id" sql:"post_id"`
	PostTitle    string               `json:"post_title" sql:"post_title"`
	AttemptTitle *string              `json:"attempt_title" sql:"attempt_title"`
	Description  string               `json:"description" sql:"description"`
	Tier         models.TierType      `json:"tier" sql:"tier"`
	Coffee       int64                `json:"coffee" sql:"coffee"`
	UpdatedAt    string               `json:"updated_at" sql:"updated_at"`
	ID           int64                `json:"_id" sql:"_id"`
	Thumbnail    string               `json:"thumbnail" sql:"thumbnail"`
	PostType     models.ChallengeType `json:"post_type" sql:"post_type"`
}

type AttemptPostMergeFrontend struct {
	PostId         string               `json:"post_id"`
	PostTitle      string               `json:"post_title"`
	AttemptTitle   *string              `json:"attempt_title" sql:"attempt_title"`
	Description    string               `json:"description"`
	Tier           models.TierType      `json:"tier"`
	Coffee         string               `json:"coffee"`
	UpdatedAt      string               `json:"updated_at"`
	ID             string               `json:"_id"`
	Thumbnail      string               `json:"thumbnail"`
	PostType       models.ChallengeType `json:"post_type"`
	PostTypeString string               `json:"post_type_string"`
}

func (merge AttemptPostMerge) ToFrontend() AttemptPostMergeFrontend {
	var title *string = nil
	if merge.AttemptTitle != nil {
		title = merge.AttemptTitle
	}
	mf := AttemptPostMergeFrontend{
		PostId:         fmt.Sprintf("%v", merge.PostId),
		PostTitle:      merge.PostTitle,
		Description:    merge.Description,
		Tier:           merge.Tier,
		Coffee:         fmt.Sprintf("%v", merge.Coffee),
		UpdatedAt:      merge.UpdatedAt,
		ID:             fmt.Sprintf("%v", merge.ID),
		Thumbnail:      merge.Thumbnail,
		PostType:       merge.PostType,
		PostTypeString: merge.PostType.String(),
		AttemptTitle:   title,
	}
	return mf
}
