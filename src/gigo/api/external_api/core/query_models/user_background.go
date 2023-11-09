package query_models

import (
	"encoding/json"
	"fmt"

	"github.com/gage-technologies/gigo-lib/db/models"
)

type UserBackground struct {
	ID                  int64                     `json:"_id" sql:"_id"`
	UserName            string                    `json:"user_name" sql:"user_name"`
	Password            string                    `json:"password"`
	Email               string                    `json:"email" sql:"email"`
	Phone               string                    `json:"phone" sql:"phone"`
	UserStatus          models.UserStatus         `json:"user_status" sql:"user_status"`
	EncryptedServiceKey []byte                    `json:"encrypted_service_key" sql:"encrypted_service_key"`
	RewardId            *int64                    `json:"reward_id" sql:"reward_id"`
	ColorPalette        *string                   `json:"color_palette" sql:"color_palette"`
	RenderInFront       *bool                     `json:"render_in_front" sql:"render_in_front"`
	Name                *string                   `json:"name" sql:"name"`
	FollowerCount       *uint64                   `json:"follower_count" sql:"follower_count"`
	Level               models.LevelType          `json:"level" sql:"level"`
	Tier                models.TierType           `json:"tier" sql:"tier"`
	Rank                models.RankType           `json:"user_rank" sql:"user_rank"`
	Coffee              uint64                    `json:"coffee" sql:"coffee"`
	StripeAccount       *string                   `json:"stripe_account" sql:"stripe_account"`
	ExclusiveAgreement  bool                      `json:"exclusive_agreement" sql:"exclusive_agreement"`
	Tutorials           []byte                    `json:"tutorials" sql:"tutorials"`
	AuthRole            models.AuthenticationRole `json:"auth_role" sql:"auth_role"`
	StripeSubscription  *string                   `json:"stripe_subscription" sql:"stripe_subscription"`
}

type UserBackgroundFrontend struct {
	ID                 string                    `json:"_id" sql:"_id"`
	PFPPath            string                    `json:"pfp_path" sql:"pfp_path"`
	UserName           string                    `json:"user_name" sql:"user_name"`
	Email              string                    `json:"email" sql:"email"`
	Phone              string                    `json:"phone" sql:"phone"`
	UserStatus         models.UserStatus         `json:"user_status" sql:"user_status"`
	UserStatusString   string                    `json:"user_status_string" sql:"user_status_string"`
	RewardId           *string                   `json:"reward_id" sql:"reward_id"`
	ColorPalette       *string                   `json:"color_palette" sql:"color_palette"`
	RenderInFront      *bool                     `json:"render_in_front" sql:"render_in_front"`
	Name               *string                   `json:"name" sql:"name"`
	FollowerCount      *uint64                   `json:"follower_count" sql:"follower_count"`
	Level              models.LevelType          `json:"level" sql:"level"`
	Tier               models.TierType           `json:"tier" sql:"tier"`
	Rank               models.RankType           `json:"user_rank" sql:"user_rank"`
	Coffee             uint64                    `json:"coffee" sql:"coffee"`
	StripeAccount      bool                      `json:"stripe_account" sql:"stripe_account"`
	ExclusiveAgreement bool                      `json:"exclusive_agreement" sql:"exclusive_agreement"`
	Tutorials          models.UserTutorial       `json:"tutorials" sql:"tutorials"`
	AuthRole           models.AuthenticationRole `json:"auth_role" sql:"auth_role"`
	StripeSubscription *string                   `json:"stripe_subscription" sql:"stripe_subscription"`
}

func (i *UserBackground) ToFrontend() (*UserBackgroundFrontend, error) {
	var rewardId *string = nil
	if i.RewardId != nil {
		reward := fmt.Sprintf("%d", *i.RewardId)
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

	var follower *uint64 = nil
	if i.FollowerCount != nil {
		follower = i.FollowerCount
	}

	var stripeAccount bool = false
	if i.StripeAccount != nil {
		stripeAccount = true
	}

	var stripeSubscription *string = nil
	if i.StripeSubscription != nil {
		stripeSubscription = i.StripeSubscription
	}

	// parse the tutorials from bytes to a model
	var tutorials models.UserTutorial
	if len(i.Tutorials) > 0 {
		if err := json.Unmarshal(i.Tutorials, &tutorials); err == nil {
			tutorials = models.DefaultUserTutorial
		} else {
			tutorials = models.DefaultUserTutorial
		}
	} else {
		tutorials = models.DefaultUserTutorial
	}

	// create new user frontend
	mf := &UserBackgroundFrontend{
		ID:                 fmt.Sprintf("%d", i.ID),
		PFPPath:            fmt.Sprintf("/static/user/pfp/%v", i.ID),
		UserName:           i.UserName,
		Email:              i.Email,
		Phone:              i.Phone,
		UserStatus:         i.UserStatus,
		UserStatusString:   i.UserStatus.String(),
		RewardId:           rewardId,
		ColorPalette:       colorPalette,
		RenderInFront:      renderInFront,
		Name:               name,
		FollowerCount:      follower,
		Level:              i.Level,
		Tier:               i.Tier,
		Rank:               i.Rank,
		Coffee:             i.Coffee,
		StripeAccount:      stripeAccount,
		ExclusiveAgreement: i.ExclusiveAgreement,
		Tutorials:          tutorials,
		StripeSubscription: stripeSubscription,
		AuthRole:           i.AuthRole,
	}

	return mf, nil
}
