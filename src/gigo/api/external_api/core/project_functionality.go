package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"gigo-core/gigo/config"
	"gigo-core/gigo/utils"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/storage"
	utils2 "github.com/gage-technologies/gigo-lib/utils"
	"github.com/gage-technologies/gigo-lib/workspace_config"
	"github.com/gage-technologies/gitea-go/gitea"
	"github.com/go-git/go-git/v5/utils/ioutil"
	"github.com/jinzhu/now"
	"gopkg.in/yaml.v3"
)

// TODO: needs testing

func CreateProject(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, stripeSubConfig config.StripeSubscriptionConfig, vcsClient *git.VCSClient,
	storageEngine storage.Storage, rdb redis.UniversalClient, js *mq.JetstreamClient, callingUser *models.User, name string, description string, sf *snowflake.Node,
	languages []models.ProgrammingLanguage, challengeType models.ChallengeType, tier models.TierType, tags []*models.Tag,
	thumbnailPath string, workspaceConfigId int64, workspaceConfigRevision int, workspaceConfigContent string,
	workspaceConfigTitle string, workspaceConfigDescription string, workspaceConfigTags []*models.Tag,
	workspaceConfigLangs []models.ProgrammingLanguage, visibility models.PostVisibility, createWorkspaceConfig bool,
	workspaceSettings *models.WorkspaceSettings, evaluation *string, projectCost *int64, logger logging.Logger, exclusiveDescription *string) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-project-core")
	defer span.End()
	callerName := "CreateProject"

	if projectCost != nil && callingUser.StripeAccount == nil {
		response, err := CreateConnectedAccount(ctx, callingUser, true)
		if err != nil {
			return map[string]interface{}{"message": "You must have a stripe connected account and the link messed up"}, fmt.Errorf("CreateConnectedAccount in create project: %v", err)
		}
		return map[string]interface{}{"message": response["account"]}, nil
	}

	// create a new id for the post
	id := sf.Generate().Int64()

	// get temp thumbnail file from storage
	thumbnailTempFile, err := storageEngine.GetFile(thumbnailPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	}
	defer thumbnailTempFile.Close()

	// sanitize thumbnail image
	thumbnailBuffer := bytes.NewBuffer([]byte{})
	_, err = utils.PrepImageFile(thumbnailTempFile, ioutil.WriteNopCloser(thumbnailBuffer), false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to prep thumbnail file: %v", err)
	}

	if len(workspaceConfigContent) > 0 {
		// validate that the config is in the right format
		var wsCfg workspace_config.GigoWorkspaceConfig
		err := yaml.Unmarshal([]byte(workspaceConfigContent), &wsCfg)
		if err != nil {
			return map[string]interface{}{"message": "config is not the right format"}, err
		}

		if wsCfg.Version != 0.1 {
			return map[string]interface{}{"message": "version must be 0.1"}, nil
		}

		if wsCfg.BaseContainer == "" {
			return map[string]interface{}{"message": "must have a base container"}, nil
		}

		if wsCfg.WorkingDirectory == "" {
			return map[string]interface{}{"message": "must have a working directory"}, nil
		}

		// make sure cpu cores are set
		if wsCfg.Resources.CPU <= 0 {
			return map[string]interface{}{"message": "must provide cpu cores"}, nil
		}

		// make sure memory is set
		if wsCfg.Resources.Mem <= 0 {
			return map[string]interface{}{"message": "must provide memory"}, nil
		}

		// make sure disk is set
		if wsCfg.Resources.Disk <= 0 {
			return map[string]interface{}{"message": "must provide disk"}, nil
		}

		// make sure no more than 6 cores are used
		if wsCfg.Resources.CPU > 6 {
			return map[string]interface{}{"message": "cannot use more than 6 CPU cores"}, nil
		}

		// make sure no more than 8gb of memory is used
		if wsCfg.Resources.Mem > 8 {
			return map[string]interface{}{"message": "cannot use more than 8 GB of RAM"}, nil
		}

		// make sure no more than 100gb of storage is used
		if wsCfg.Resources.Disk > 100 {
			return map[string]interface{}{"message": "cannot use more than 100 GB of disk space"}, nil
		}
	}

	// query for workspace config if we're using a template
	if workspaceConfigId > 0 {
		// query for workspace config
		err = tidb.QueryRowContext(ctx, &span, &callerName,
			"select content from workspace_config where _id = ? and revision = ? limit 1",
			workspaceConfigId, workspaceConfigRevision,
		).Scan(&workspaceConfigContent)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{
					"message": "We couldn't find that Workspace Config! We're sorry, try again or try another Workspace Config.",
				}, fmt.Errorf("workspace config %d not found", workspaceConfigId)
			}
			return nil, fmt.Errorf("failed to query workspace_config: %v", err)
		}

		_, err = tidb.ExecContext(ctx, &span, &callerName, "Update workspace_config SET uses = uses + 1 Where _id = ? and revision = ?", workspaceConfigId, workspaceConfigRevision)
		if err != nil {
			return nil, fmt.Errorf("failed to update workspace config uses: %v", err)
		}
	}

	// create repo for the project
	repo, err := vcsClient.CreateRepo(
		fmt.Sprintf("%d", callingUser.ID),
		fmt.Sprintf("%d", id),
		"",
		true,
		"",
		"",
		"",
		"main",
	)
	if err != nil {
		return map[string]interface{}{"message": "Unable to create repo"}, err
	}

	// create boolean to track failure
	failed := true

	// create slice to hold tag ids
	tagIds := make([]int64, len(tags))
	newTags := make([]interface{}, 0)

	// create variable to track the id of a config template that is created
	var configTemplateId *int64

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = vcsClient.DeleteRepo(fmt.Sprintf("%d", callingUser.ID), fmt.Sprintf("%d", id))
		_ = meili.DeleteDocuments("posts", id)
		for _, tag := range newTags {
			_ = meili.DeleteDocuments("tags", tag.(*models.TagSearch).ID)
		}
		if configTemplateId != nil {
			_ = meili.DeleteDocuments("workspace_configs", *configTemplateId)
		}
	}()

	// encode workspace config content to base64
	workspaceConfigContentBase64 := base64.StdEncoding.EncodeToString([]byte(workspaceConfigContent))

	// add workspace config to repository
	_, gitRes, err := vcsClient.GiteaClient.CreateFile(
		fmt.Sprintf("%d", callingUser.ID),
		fmt.Sprintf("%d", id),
		".gigo/workspace.yaml",
		gitea.CreateFileOptions{
			Content: workspaceConfigContentBase64,
			FileOptions: gitea.FileOptions{
				Message:    "[GIGO-INIT] workspace config",
				BranchName: "main",
				Author: gitea.Identity{
					Name:  "Gigo",
					Email: "gigo@gigo.dev",
				},
				Committer: gitea.Identity{
					Name:  "Gigo",
					Email: "gigo@gigo.dev",
				},
			},
		},
	)
	if err != nil {
		var buf []byte
		if gitRes != nil {
			buf, _ = io.ReadAll(gitRes.Body)
		}
		return nil, fmt.Errorf("failed to create workspace config in repo: %v\n    res: %v", err, string(buf))
	}

	if evaluation != nil {
		// encode evaluation content to base64
		evaluationBase64 := base64.StdEncoding.EncodeToString([]byte(*evaluation))
		// add evaluation file to repository
		_, gitRes, err := vcsClient.GiteaClient.CreateFile(
			fmt.Sprintf("%d", callingUser.ID),
			fmt.Sprintf("%d", id),
			"EVALUATION.md",
			gitea.CreateFileOptions{
				Content: evaluationBase64,
				FileOptions: gitea.FileOptions{
					Message:    "[GIGO-INIT] evaluation",
					BranchName: "main",
					Author: gitea.Identity{
						Name:  "Gigo",
						Email: "gigo@gigo.dev",
					},
					Committer: gitea.Identity{
						Name:  "Gigo",
						Email: "gigo@gigo.dev",
					},
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create evaluation in repo: %v\n    res: %v", err, gitRes)
		}
	}

	// create transaction for image insertion
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
			_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where _id =?", tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}
		}

		// append tag id to tag ids slice
		tagIds = append(tagIds, tag.ID)
	}

	// handle creation of a new workspace config template
	if createWorkspaceConfig && workspaceConfigId == -1 {
		// create slice to hold tag ids for workspace config tags
		wsConfigTagIds := make([]int64, len(workspaceConfigTags))

		// iterate over the workspace config tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
		for _, tag := range workspaceConfigTags {
			// if this tag is a new tag ensure that it has not already been created in the workspace tags creation above
			if tag.ID == -1 {
				// iterate new tags looking for a matching value
				for _, t := range newTags {
					// check if the values match and assign new tag id to the tag if they do
					if strings.ToLower(t.(*models.TagSearch).Value) == strings.ToLower(tag.Value) {
						tag.ID = t.(*models.TagSearch).ID
						break
					}
				}
			}

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
				_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where _id =?", tag.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
				}
			}

			// append tag id to tag ids slice
			wsConfigTagIds = append(wsConfigTagIds, tag.ID)
		}

		// create a new workspace config template
		wsCfg := models.CreateWorkspaceConfig(
			sf.Generate().Int64(),
			workspaceConfigTitle,
			workspaceConfigDescription,
			workspaceConfigContent,
			callingUser.ID,
			0,
			wsConfigTagIds,
			workspaceConfigLangs,
			1,
		)

		// format to sql insertion statements
		statements, err := wsCfg.ToSQLNative()
		if err != nil {
			return nil, fmt.Errorf("failed to format workspace config to sql: %v", err)
		}

		// perform sql insertion
		for _, statement := range statements {
			_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to perform workspace config insertion: %v", err)
			}
		}

		// perform search engine insertion
		err = meili.AddDocuments("workspace_configs", wsCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to insert workspace config to search engine: %v", err)
		}

		// update workspace config id
		workspaceConfigId = wsCfg.ID

		// update workspace config template id so that we can cleanup if we fail
		configTemplateId = &wsCfg.ID
	}

	if visibility == models.PrivateVisibility && callingUser.UserStatus != models.UserStatusPremium {
		visibility = models.PublicVisibility
	}

	// create a new post
	post, err := models.CreatePost(
		id,
		name,
		description,
		callingUser.UserName,
		callingUser.ID,
		time.Now(),
		time.Now(),
		repo.ID,
		tier,
		[]int64{},
		nil,
		uint64(0),
		challengeType,
		0,
		0,
		0,
		languages,
		visibility,
		tagIds,
		nil,
		nil,
		workspaceConfigId,
		workspaceConfigRevision,
		workspaceSettings,
		false,
		false,
		exclusiveDescription,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new post struct: %v", err)
	}

	// format the post into sql insert statements
	statements, err := post.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format post into insert statements: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for post: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash post id: %v", err)
	}
	err = storageEngine.CreateFile(
		fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash),
		thumbnailBuffer.Bytes(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	// attempt to insert the prost into the search engine to make it discoverable
	err = meili.AddDocuments("posts", post)
	if err != nil {
		return nil, fmt.Errorf("failed to add post to search engine: %v", err)
	}

	// conditionally attempt to insert the tags into the search engine to make it discoverable
	if len(newTags) > 0 {
		err = meili.AddDocuments("tags", newTags...)
		if err != nil {
			return nil, fmt.Errorf("failed to add new tags to search engine: %v", err)
		}
	}

	// format post to frontend object
	fp, err := post.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format post to frontend object: %v", err)
	}

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit ")
	}

	if projectCost != nil {
		fullProjectCost := *projectCost * 100
		cost, err := CreateProduct(ctx, fullProjectCost, tidb, post.ID, callingUser)
		if err != nil {
			return map[string]interface{}{"message": "Project has been created. But there was an issue creating the pricing for it", "project": fp}, fmt.Errorf("failed to create stripe price: %v", err)
		}

		if cost["message"] != "Product has been created." {
			return map[string]interface{}{"message": "Project has been created. But there was an issue creating the pricing for it on function", "project": fp}, fmt.Errorf("failed to create stripe price in function: %v", cost["message"])
		}
	}

	// set failed as false
	failed = false

	// delete the redis image generation count key since successful project creation
	_ = rdb.Del(ctx, fmt.Sprintf("user:%v:image:gen:count", callingUser.ID)).Err()

	if challengeType.String() == "Interactive" {
		xpRes, err := AddXP(ctx, tidb, js, rdb, sf, stripeSubConfig, callingUser.ID, "create_tutorial", nil, nil, logger, callingUser)
		if err != nil {
			return map[string]interface{}{"message": "Project has been created.", "project": fp}, fmt.Errorf("failed to add xp to user: %v", err)
		}
		return map[string]interface{}{"message": "Project has been created.", "project": fp, "xp": xpRes}, nil
	} else {
		// add xp to user for creating a project
		xpRes, err := AddXP(ctx, tidb, js, rdb, sf, stripeSubConfig, callingUser.ID, "create", nil, nil, logger, callingUser)
		if err != nil {
			return map[string]interface{}{"message": "Project has been created.", "project": fp}, fmt.Errorf("failed to add xp to user: %v", err)
		}
		return map[string]interface{}{"message": "Project has been created.", "project": fp, "xp": xpRes}, nil
	}

}

func EditProject(ctx context.Context, tidb *ti.Database, id int64, storageEngine storage.Storage, thumbnailPath *string, title *string, challengeType *models.ChallengeType, tier *models.TierType, meili *search.MeiliSearchEngine, addedTags []*models.Tag, removedTags []*models.Tag, sf *snowflake.Node) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-project")
	defer span.End()
	callerName := "EditProject"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	if thumbnailPath != nil {
		// get temp thumbnail file from storage
		thumbnailTempFile, err := storageEngine.GetFile(*thumbnailPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
		}
		defer thumbnailTempFile.Close()

		// sanitize thumbnail image
		thumbnailBuffer := bytes.NewBuffer([]byte{})
		_, err = utils.PrepImageFile(thumbnailTempFile, ioutil.WriteNopCloser(thumbnailBuffer), false, false)
		if err != nil {
			return nil, fmt.Errorf("failed to prep thumbnail file: %v", err)
		}

		// write thumbnail to final location
		idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
		if err != nil {
			return nil, fmt.Errorf("failed to hash post id: %v", err)
		}
		err = storageEngine.CreateFile(
			fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash),
			thumbnailBuffer.Bytes(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
		}
	}

	if title != nil {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update post set title = ?, embedded = ? where _id = ?", title, false, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit post title: %v", err)
		}
		// update post description in meilisearch
		err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": id, "title": title})
		if err != nil {
			return nil, fmt.Errorf("failed to update post title in meilisearch: %v", err)
		}
	}

	if tier != nil {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update post set tier = ?, embedded = ? where _id = ?", tier, false, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit tier: %v", err)
		}
		// update post description in meilisearch
		err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": id, "tier": title})
		if err != nil {
			return nil, fmt.Errorf("failed to update post tier in meilisearch: %v", err)
		}
	}

	if challengeType != nil {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update post set post_type = ?, embedded = ? where _id = ?", challengeType, false, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit challenge type: %v", err)
		}
		// update post description in meilisearch
		err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": id, "post_type": title})
		if err != nil {
			return nil, fmt.Errorf("failed to update post challenge type in meilisearch: %v", err)
		}
	}

	if addedTags != nil && len(addedTags) > 0 {
		newTags := make([]interface{}, 0)
		// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
		for _, tag := range addedTags {
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
				_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where _id =?", tag.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
				}
			}

			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "insert ignore into post_tags(post_id, tag_id) values(?, ?)", id, tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}
		}
		// conditionally attempt to insert the tags into the search engine to make it discoverable
		if len(newTags) > 0 {
			err = meili.AddDocuments("tags", newTags...)
			if err != nil {
				return nil, fmt.Errorf("failed to add new tags to search engine: %v", err)
			}
		}
	}

	if removedTags != nil && len(removedTags) > 0 {
		// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
		for _, tag := range removedTags {
			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count - 1 where _id =?", tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}

			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "delete from post_tags where post_id = ? and tag_id = ?", id, tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to remove tag usage count: %v", err)
			}
		}
	}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update post set updated_at = ? where _id =?", time.Now(), id)
	if err != nil {
		return nil, fmt.Errorf("failed to increment updated at: %v", err)
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	return map[string]interface{}{"message": "success"}, nil
}

func EditAttempt(ctx context.Context, tidb *ti.Database, id int64, storageEngine storage.Storage, thumbnailPath *string, title *string, challengeType *models.ChallengeType, tier *models.TierType, meili *search.MeiliSearchEngine, addedTags []*models.Tag, removedTags []*models.Tag, sf *snowflake.Node) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-attempt")
	defer span.End()
	callerName := "EditAttempt"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	if thumbnailPath != nil {
		// get temp thumbnail file from storage
		thumbnailTempFile, err := storageEngine.GetFile(*thumbnailPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
		}
		defer thumbnailTempFile.Close()

		// sanitize thumbnail image
		thumbnailBuffer := bytes.NewBuffer([]byte{})
		_, err = utils.PrepImageFile(thumbnailTempFile, ioutil.WriteNopCloser(thumbnailBuffer), false, false)
		if err != nil {
			return nil, fmt.Errorf("failed to prep thumbnail file: %v", err)
		}

		// write thumbnail to final location
		idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
		if err != nil {
			return nil, fmt.Errorf("failed to hash post id: %v", err)
		}
		err = storageEngine.CreateFile(
			fmt.Sprintf("attempt/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash),
			thumbnailBuffer.Bytes(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
		}
	}

	if title != nil {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update attempt set title = ? where _id = ?", title, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit post title: %v", err)
		}
		// update post description in meilisearch
		err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": id, "title": title})
		if err != nil {
			return nil, fmt.Errorf("failed to update post title in meilisearch: %v", err)
		}
	}

	if tier != nil {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update attempt set tier = ?, embedded = ? where _id = ?", tier, false, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit tier: %v", err)
		}
		// update post description in meilisearch
		err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": id, "tier": title})
		if err != nil {
			return nil, fmt.Errorf("failed to update post tier in meilisearch: %v", err)
		}
	}

	if challengeType != nil {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update attempt set post_type = ?, embedded = ? where _id = ?", challengeType, false, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit challenge type: %v", err)
		}
	}

	if addedTags != nil && len(addedTags) > 0 {
		newTags := make([]interface{}, 0)
		// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
		for _, tag := range addedTags {
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
				_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where _id =?", tag.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
				}
			}

			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "insert ignore into post_tags(post_id, tag_id) values(?, ?)", id, tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}
		}
		// conditionally attempt to insert the tags into the search engine to make it discoverable
		if len(newTags) > 0 {
			err = meili.AddDocuments("tags", newTags...)
			if err != nil {
				return nil, fmt.Errorf("failed to add new tags to search engine: %v", err)
			}
		}
	}

	if removedTags != nil && len(removedTags) > 0 {
		// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
		for _, tag := range removedTags {
			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count - 1 where _id =?", tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}

			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "delete from post_tags where post_id = ? and tag_id = ?", id, tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to remove tag usage count: %v", err)
			}
		}
	}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update attempt set updated_at = ? where _id =?", time.Now(), id)
	if err != nil {
		return nil, fmt.Errorf("failed to increment updated at: %v", err)
	}

	// commit insertion transaction to database
	err = tx.Commit(&callerName)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new user: %v", err)
	}

	return map[string]interface{}{"message": "success"}, nil
}

func DeleteProject(ctx context.Context, tidb *ti.Database, callingUser *models.User, meili *search.MeiliSearchEngine, projectID int64, logger logging.Logger) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "delete-project")
	defer span.End()
	callerName := "DeleteProject"

	logger.Infof("attempting to delete project with id: %d from user: %v", projectID, callingUser.UserName)
	res, err := tidb.ExecContext(ctx, &span, &callerName, "update post set deleted = true, published = false where _id = ? and author_id = ?", projectID, callingUser.ID)
	if err != nil {
		logger.Errorf("failed to delete project: %v by updating database row: %v", projectID, err)
		return nil, fmt.Errorf("failed to delete project by updating database row: %v", err)
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		logger.Errorf("failed to delete project: %v by updating database row, cannot retrieve rows: %v", projectID, err)
		return nil, fmt.Errorf("failed to delete project by updating database row, cannot retrieve rows: %v", err)
	}

	if numRows == 0 {
		logger.Errorf("failed to delete project: %v no project found for user: %v", projectID, callingUser.UserName)
		return nil, fmt.Errorf("failed to delete project by updating database row, no rows affected")
	}

	err = meili.DeleteDocuments("posts", projectID)
	if err != nil {
		logger.Errorf("failed to delete project: %v by updating search engine: %v", projectID, err)
		return nil, fmt.Errorf("failed to delete project by updating search engine: %v", err)
	}

	logger.Infof("deleted project: %v from user: %v", projectID, callingUser.UserName)
	return map[string]interface{}{"message": "Project has been deleted.", "project": projectID}, nil
}

func StartAttempt(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, js *mq.JetstreamClient, rdb redis.UniversalClient, stripeSubConfig config.StripeSubscriptionConfig, callingUser *models.User, userSession *models.UserSession,
	sf *snowflake.Node, postId int64, parentAttempt *int64, logger logging.Logger, storageEngine storage.Storage) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "start-attempt-core")
	defer span.End()
	callerName := "StartAttempt"

	// ensure this user doesn't have an attempt already
	var existingAttemptId int64
	err := tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id from attempt where post_id = ? and author_id = ? limit 1", postId, callingUser.ID,
	).Scan(&existingAttemptId)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get attempt count: %v", err)
	}

	// exit if this user already has made an attempt
	if existingAttemptId > 0 {
		return map[string]interface{}{"message": "You have already started an attempt. Keep working on that one!", "attempt": fmt.Sprintf("%d", existingAttemptId)}, nil
	}

	//// write thumbnail to final location
	//idHashOriginal, err := utils2.HashData([]byte(fmt.Sprintf("%d", postId)))
	//if err != nil {
	//	return nil, fmt.Errorf("failed to hash post id: %v", err)
	//}
	//
	//// get temp thumbnail file from storage
	//thumbnailTempFile, err := storageEngine.GetFile(fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHashOriginal[:3], idHashOriginal[3:6], idHashOriginal))
	//if err != nil {
	//	return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	//}
	//defer thumbnailTempFile.Close()
	//
	//// sanitize thumbnail image
	//thumbnailBuffer := bytes.NewBuffer([]byte{})
	//err = utils.PrepImageFile(thumbnailTempFile, ioutil.WriteNopCloser(thumbnailBuffer))
	//if err != nil {
	//	return nil, fmt.Errorf("failed to prep thumbnail file: %v", err)
	//}

	// create variables to hold post data
	var postTitle string
	var postDesc string
	var postAuthorId int64
	var postVisibility models.PostVisibility
	var postType models.ChallengeType
	var workspaceConfig int64
	var workspaceConfigRevision int64
	var tier models.TierType

	// retrieve post
	err = tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id, title, description, author_id, visibility, post_type, workspace_config, workspace_config_revision, tier from post where _id = ? limit 1", postId,
	).Scan(&postId, &postTitle, &postDesc, &postAuthorId, &postVisibility, &postType, &workspaceConfig, &workspaceConfigRevision, &tier)
	if err != nil {
		return nil, fmt.Errorf("failed to query for post: %v\n    query: %s\n    params: %v", err,
			"select repo_id from post where _id = ?", []interface{}{postId})
	}

	// ensure that post is not Exclusive
	if postVisibility == models.ExclusiveVisibility {
		return nil, fmt.Errorf("You can't start this attempt yet. This Challenge is an Exclusive Challenge " +
			"and must be purchased.")
	}

	// ensure that the user is premium if this is a Premium challenge
	if postVisibility == models.PremiumVisibility && callingUser.UserStatus != models.UserStatusPremium {
		return nil, fmt.Errorf("You can't start this attempt yet. This Challenge is a Premium Challenge and " +
			"is only accessible to Premium users. Go tou the Account Settings page to upgrade your account.")
	}

	// create source repo path
	repoOwner := fmt.Sprintf("%d", postAuthorId)
	repoName := fmt.Sprintf("%d", postId)

	// conditionally load parent attempt data for repo
	if parentAttempt != nil {
		var attemptOwner int64
		var closed bool
		err = tidb.QueryRowContext(ctx, &span, &callerName,
			"select author_id, closed from attempt where _id = ? limit 1",
			*parentAttempt,
		).Scan(&attemptOwner, &closed)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{
					"message": "We couldn't find that Attempt. We're sorry! We'll get hustlin' and bustlin' on fixing that!",
				}, fmt.Errorf("no attempt found: %d", *parentAttempt)
			}
			return nil, fmt.Errorf("failed to query for parent attempt: %v", err)
		}
		repoOwner = fmt.Sprintf("%d", attemptOwner)
		repoName = fmt.Sprintf("%d", *parentAttempt)

		if !closed {
			return map[string]interface{}{"message": "Attempt must be published first."}, nil
		}
	}

	// create a new attempt
	attempt, err := models.CreateAttempt(sf.Generate().Int64(), postTitle, postDesc, callingUser.UserName,
		callingUser.ID, time.Now(), time.Now(), -1, callingUser.Tier, nil, 0, postId, tier, parentAttempt, postType)
	if err != nil {
		return nil, fmt.Errorf("failed to create attemp struct: %v", err)
	}

	// retrieve the service password from the session
	servicePassword, err := userSession.GetServiceKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get service key: %v", err)
	}

	// grant read access to challenge repository for calling user so that they can fork it
	readAccess := gitea.AccessModeRead
	_, err = vcsClient.GiteaClient.AddCollaborator(repoOwner, repoName, fmt.Sprintf("%d", callingUser.ID), gitea.AddCollaboratorOption{
		Permission: &readAccess,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to grant read access to repository: %v", err)
	}

	// defer removal of read access
	defer vcsClient.GiteaClient.DeleteCollaborator(repoOwner, repoName, fmt.Sprintf("%d", callingUser.ID))

	// login to git client to create a token
	userGitClient, err := vcsClient.LoginAsUser(fmt.Sprintf("%d", callingUser.ID), servicePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to login to git client: %v", err)
	}

	// fork post repo into user owned attempt repo
	newRepoId := fmt.Sprintf("%d", attempt.ID)
	repo, gitRes, err := userGitClient.CreateFork(repoOwner, repoName, gitea.CreateForkOption{Name: &newRepoId})
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fork post %s/%s -> %d/%s repo: %v\n    res: %s",
			repoOwner, repoName, callingUser.ID, newRepoId, err, JsonifyGiteaResponse(gitRes),
		)
	}

	// revoke read access to challenge repository for calling user since we have forked it
	_, err = vcsClient.GiteaClient.DeleteCollaborator(repoOwner, repoName, fmt.Sprintf("%d", callingUser.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to revoke read access to repository: %v", err)
	}

	// update attempt with new repo id
	attempt.RepoID = repo.ID

	// format attempt for insertion
	insertStatements, err := attempt.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format attempt into insert statements: %v", err)
	}

	// open tx for attempt insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for attempt insertion: %v", err)
	}

	defer tx.Rollback()

	// iterate over insert statements executing them in sql
	for _, statement := range insertStatements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute insertion statement for attempt: %v\n    query: %s\n    params: %v",
				err, statement.Statement, statement.Values)
		}
	}
	// write thumbnail to final location
	idHashOriginal, err := utils2.HashData([]byte(fmt.Sprintf("%d", postId)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash post id: %v", err)
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", attempt.ID)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash post id: %v", err)
	}

	// get temp thumbnail file from storage
	err = storageEngine.CopyFile(fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHashOriginal[:3], idHashOriginal[3:6], idHashOriginal), fmt.Sprintf("attempt/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash))
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update post set attempts = attempts + 1 where _id = ?", postId)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	// update any recommendations for this user and project to be marked as accepted
	_, err = tx.ExecContext(ctx, &callerName, "update recommended_post set accepted = true where user_id = ? and post_id = ?", callingUser.ID, postId)
	if err != nil {
		return nil, fmt.Errorf("failed to update recommendations: %v", err)
	}

	/////////////////

	//update workspace config for the new use
	if workspaceConfig > 0 {
		_, err = tidb.ExecContext(ctx, &span, &callerName, "Update workspace_config SET uses = uses + 1 Where _id = ? and revision = ?", workspaceConfig, workspaceConfigRevision)
		if err != nil {
			return nil, fmt.Errorf("failed to update workspace config uses: %v", err)
		}
	}

	////////////////////

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit attempt insertion: %v", err)
	}

	// add xp to user for making an attempt
	xpRes, err := AddXP(ctx, tidb, js, rdb, sf, stripeSubConfig, callingUser.ID, "attempt", &attempt.Tier, nil, logger, callingUser)
	if err != nil {
		return map[string]interface{}{"message": "Attempt created successfully.", "attempt": attempt.ToFrontend()},
			fmt.Errorf("failed to add xp to user: %v", err)
	}

	// TODO broadcast xp gain to project owner for attempt made on their project
	_, err = AddXP(ctx, tidb, js, rdb, sf, stripeSubConfig, postAuthorId, "challenge_is_attempted", &attempt.Tier, nil, logger, callingUser)
	if err != nil {
		return map[string]interface{}{"message": "Attempt created successfully.", "attempt": attempt.ToFrontend()},
			fmt.Errorf("failed to add xp to user: %v", err)
	}

	return map[string]interface{}{"message": "Attempt created successfully.", "attempt": attempt.ToFrontend(), "xp": xpRes}, nil
}

func StartEAttempt(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, callingUser *models.User, userSession *models.UserSession,
	sf *snowflake.Node, postId int64, parentAttempt *int64, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "start-attempt-core")
	defer span.End()
	callerName := "StartAttempt"

	// ensure this user doesn't have an attempt already
	var existingAttemptId int64
	err := tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id from attempt where post_id = ? and author_id = ? limit 1", postId, callingUser.ID,
	).Scan(&existingAttemptId)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get attempt count: %v", err)
	}

	// exit if this user already has made an attempt
	if existingAttemptId > 0 {
		return nil, fmt.Errorf("existing attempt found %d", existingAttemptId)
	}

	// create variables to hold post data
	var postTitle string
	var postDesc string
	var postAuthorId int64
	var postVisibility models.PostVisibility
	var postType models.ChallengeType

	// retrieve post
	err = tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id, title, description, author_id, visibility, post_type from post where _id = ? limit 1", postId,
	).Scan(&postId, &postTitle, &postDesc, &postAuthorId, &postVisibility, &postType)
	if err != nil {
		return nil, fmt.Errorf("failed to query for post: %v\n    query: %s\n    params: %v", err,
			"select repo_id from post where _id = ?", []interface{}{postId})
	}

	// ensure that post is not Exclusive
	if postVisibility == models.ExclusiveVisibility {
		return nil, fmt.Errorf("You can't start this attempt yet. This Challenge is an Exclusive Challenge " +
			"and must be purchased.")
	}

	// ensure that the user is premium if this is a Premium challenge
	if postVisibility == models.PremiumVisibility && callingUser.UserStatus != models.UserStatusPremium {
		return nil, fmt.Errorf("You can't start this attempt yet. This Challenge is a Premium Challenge and " +
			"is only accessible to Premium users. Go tou the Account Settings page to upgrade your account.")
	}

	// create source repo path
	repoOwner := fmt.Sprintf("%d", postAuthorId)
	repoName := fmt.Sprintf("%d", postId)

	// conditionally load parent attempt data for repo
	if parentAttempt != nil {
		var attemptOwner int64
		err = tidb.QueryRowContext(ctx, &span, &callerName,
			"select author_id from attempt where _id = ? limit 1",
			*parentAttempt,
		).Scan(&attemptOwner)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[string]interface{}{
					"message": "We couldn't find that Attempt. We're sorry! We'll get hustlin' and bustlin' on fixing that!",
				}, fmt.Errorf("no attempt found: %d", *parentAttempt)
			}
			return nil, fmt.Errorf("failed to query for parent attempt: %v", err)
		}
		repoOwner = fmt.Sprintf("%d", attemptOwner)
		repoName = fmt.Sprintf("%d", *parentAttempt)
	}

	// create a new attempt
	attempt, err := models.CreateAttempt(sf.Generate().Int64(), postTitle, postDesc, callingUser.UserName,
		callingUser.ID, time.Now(), time.Now(), -1, callingUser.Tier, nil, 0, postId, 0, parentAttempt, postType)
	if err != nil {
		return nil, fmt.Errorf("failed to create attemp struct: %v", err)
	}

	// retrieve the service password from the session
	servicePassword, err := userSession.GetServiceKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get service key: %v", err)
	}

	// grant read access to challenge repository for calling user so that they can fork it
	readAccess := gitea.AccessModeRead
	_, err = vcsClient.GiteaClient.AddCollaborator(repoOwner, repoName, fmt.Sprintf("%d", callingUser.ID), gitea.AddCollaboratorOption{
		Permission: &readAccess,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to grant read access to repository: %v -- repoOwner: %v, repoName: %v, collaborator: %v", err, repoOwner, repoName, callingUser.ID)
	}

	// defer removal of read access
	defer vcsClient.GiteaClient.DeleteCollaborator(repoOwner, repoName, fmt.Sprintf("%d", callingUser.ID))

	// login to git client to create a token
	userGitClient, err := vcsClient.LoginAsUser(fmt.Sprintf("%d", callingUser.ID), servicePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to login to git client: %v", err)
	}

	// fork post repo into user owned attempt repo
	newRepoId := fmt.Sprintf("%d", attempt.ID)
	repo, gitRes, err := userGitClient.CreateFork(repoOwner, repoName, gitea.CreateForkOption{Name: &newRepoId})
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fork post %s/%s -> %d/%s repo: %v\n    res: %s",
			repoOwner, repoName, callingUser.ID, newRepoId, err, JsonifyGiteaResponse(gitRes),
		)
	}

	// revoke read access to challenge repository for calling user since we have forked it
	_, err = vcsClient.GiteaClient.DeleteCollaborator(repoOwner, repoName, fmt.Sprintf("%d", callingUser.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to revoke read access to repository: %v", err)
	}

	// update attempt with new repo id
	attempt.RepoID = repo.ID

	// format attempt for insertion
	insertStatements, err := attempt.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format attempt into insert statements: %v", err)
	}

	// open tx for attempt insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for attempt insertion: %v", err)
	}

	defer tx.Rollback()

	// iterate over insert statements executing them in sql
	for _, statement := range insertStatements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute insertion statement for attempt: %v\n    query: %s\n    params: %v",
				err, statement.Statement, statement.Values)
		}
	}

	//// write thumbnail to final location
	//idHashOriginal, err := utils2.HashData([]byte(fmt.Sprintf("%d", postId)))
	//if err != nil {
	//	return nil, fmt.Errorf("failed to hash post id: %v", err)
	//}
	//
	//// write thumbnail to final location
	//idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", attempt.ID)))
	//if err != nil {
	//	return nil, fmt.Errorf("failed to hash post id: %v", err)
	//}
	//
	//// get temp thumbnail file from storage
	//err = storageEngine.CopyFile(fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHashOriginal[:3], idHashOriginal[3:6], idHashOriginal), fmt.Sprintf("attempt/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash))
	//if err != nil {
	//	return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	//}

	// increment tag column usage_count in database
	_, err = tx.ExecContext(ctx, &callerName, "update post set attempts = attempts + 1 where _id = ?", postId)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	// update any recommendations for this user and project to be marked as accepted
	_, err = tx.ExecContext(ctx, &callerName, "update recommended_post set accepted = true where user_id = ? and post_id = ?", callingUser.ID, postId)
	if err != nil {
		return nil, fmt.Errorf("failed to update recommendations: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit attempt insertion: %v", err)
	}

	return map[string]interface{}{"message": "Attempt created successfully.", "attempt": attempt.ToFrontend()}, nil
}

func PublishProject(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, postId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "publish-project-core")
	defer span.End()
	callerName := "PublishProject"

	// open tx to perform update on post
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for publishing: %v", err)
	}

	defer tx.Rollback()

	// update post in sql
	_, err = tx.ExecContext(ctx, &callerName, "update post set published = true where _id =?", postId)
	if err != nil {
		return nil, fmt.Errorf("failed to update post: %v", err)
	}

	// update post in meilisearch
	err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": postId, "published": true})
	if err != nil {
		return nil, fmt.Errorf("failed to update post in meilisearch: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit publishing: %v", err)
	}

	return map[string]interface{}{"message": "Post published successfully.", "post": fmt.Sprintf("%d", postId)}, nil
}

func CreatePublicConfigTemplate(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine,
	sf *snowflake.Node, workspaceConfigTags []*models.Tag, workspaceConfigTitle string,
	workspaceConfigDescription string, callingUser *models.User, workspaceConfigContent string,
	workspaceConfigLangs []models.ProgrammingLanguage) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-public-workspace-config-template-core")
	defer span.End()
	callerName := "CreatePublicWorkspaceConfigTemplate"

	// validate that the config is in the right format
	var testCfg workspace_config.GigoWorkspaceConfig
	err := yaml.Unmarshal([]byte(workspaceConfigContent), &testCfg)
	if err != nil {
		return map[string]interface{}{"message": "config is not the right format"}, err
	}

	if testCfg.Version != 0.1 {
		return map[string]interface{}{"message": "version must be 0.1"}, nil
	}

	if testCfg.BaseContainer == "" {
		return map[string]interface{}{"message": "must have a base container"}, nil
	}

	if testCfg.WorkingDirectory == "" {
		return map[string]interface{}{"message": "must have a working directory"}, nil
	}

	// make sure cpu cores are set
	if testCfg.Resources.CPU <= 0 {
		return map[string]interface{}{"message": "must provide cpu cores"}, nil
	}

	// make sure memory is set
	if testCfg.Resources.Mem <= 0 {
		return map[string]interface{}{"message": "must provide memory"}, nil
	}

	// make sure disk is set
	if testCfg.Resources.Disk <= 0 {
		return map[string]interface{}{"message": "must provide disk"}, nil
	}

	// make sure no more than 6 cores are used
	if testCfg.Resources.CPU > 6 {
		return map[string]interface{}{"message": "cannot use more than 6 CPU cores"}, nil
	}

	// make sure no more than 8gb of memory is used
	if testCfg.Resources.Mem > 8 {
		return map[string]interface{}{"message": "cannot use more than 8 GB of RAM"}, nil
	}

	// make sure no more than 100gb of storage is used
	if testCfg.Resources.Disk > 100 {
		return map[string]interface{}{"message": "cannot use more than 100 GB of disk space"}, nil
	}

	// create boolean to track failure
	failed := true

	// create variable to track the id of a config template that is created
	var configTemplateId *int64

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		if configTemplateId != nil {
			_ = meili.DeleteDocuments("workspace_configs", *configTemplateId)
		}
	}()

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// create slice to hold tag ids for workspace config tags
	wsConfigTagIds := make([]int64, len(workspaceConfigTags))

	// iterate over the workspace config tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
	for _, tag := range workspaceConfigTags {

		// conditionally create a new id and insert tag into database if it does not already exist
		if tag.ID == -1 {
			// generate new tag id
			tag.ID = sf.Generate().Int64()

			// iterate statements inserting the new tag into the database
			for _, statement := range tag.ToSQLNative() {
				_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
				if err != nil {
					return nil, fmt.Errorf("failed to perform tag insertion: %v", err)
				}
			}

		} else {
			// increment tag column usage_count in database
			_, err := tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where _id =?", tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}
		}

		// append tag id to tag ids slice
		wsConfigTagIds = append(wsConfigTagIds, tag.ID)
	}

	// create a new workspace config template
	wsCfg := models.CreateWorkspaceConfig(
		sf.Generate().Int64(),
		workspaceConfigTitle,
		workspaceConfigDescription,
		workspaceConfigContent,
		callingUser.ID,
		0,
		wsConfigTagIds,
		workspaceConfigLangs,
		0,
	)

	// format to sql insertion statements
	statements, err := wsCfg.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format workspace config to sql: %v", err)
	}

	// perform sql insertion
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform workspace config insertion: %v", err)
		}
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit workspace config insertion: %v", err)
	}

	// perform search engine insertion
	err = meili.AddDocuments("workspace_configs", wsCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to insert workspace config to search engine: %v", err)
	}

	return map[string]interface{}{"message": "workspace config template created successfully", "config": wsCfg}, nil
}

func EditPublicConfigTemplate(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, callingUser *models.User, workspaceConfigID int64, content string, workspaceConfigTags []*models.Tag, description string, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-public-config-core")
	defer span.End()
	callerName := "EditPublicConfigTemplate"

	res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from workspace_config where _id = ? and author_id =? order by revision desc limit 1", workspaceConfigID, callingUser.ID)
	if err != nil {
		return map[string]interface{}{"message": "no config found matching parameters"}, fmt.Errorf("failed to query workspace config: %v", err)
	}

	var conf *models.WorkspaceConfig

	for res.Next() {
		conf, err = models.WorkspaceConfigFromSQLNative(tidb, res)
		if err != nil {
			return nil, fmt.Errorf("failed to parse workspace config: %v", err)
		}
	}

	isSame := true

	// validate that the config is in the right format
	var confCfg workspace_config.GigoWorkspaceConfig
	var wsCfg workspace_config.GigoWorkspaceConfig
	err = yaml.Unmarshal([]byte(content), &wsCfg)
	if err != nil {
		return map[string]interface{}{"message": "passed config is not the right format"}, err
	}

	err = yaml.Unmarshal([]byte(conf.Content), &confCfg)
	if err != nil {
		return map[string]interface{}{"message": "retrieved config is not the right format"}, err
	}

	if wsCfg.Version != 0.1 {
		return map[string]interface{}{"message": "version must be 0.1"}, fmt.Errorf("version must be 0.1")
	}

	if wsCfg.BaseContainer == "" {
		return map[string]interface{}{"message": "must have a base container"}, fmt.Errorf("must have a base container")
	}

	if wsCfg.BaseContainer != confCfg.BaseContainer {
		isSame = false
	}

	if wsCfg.WorkingDirectory == "" {
		return map[string]interface{}{"message": "must have a working directory"}, fmt.Errorf("must have a working directory")
	}

	if wsCfg.WorkingDirectory != confCfg.WorkingDirectory {
		isSame = false
	}

	// make sure cpu cores are set
	if wsCfg.Resources.CPU <= 0 {
		return map[string]interface{}{"message": "must provide cpu cores"}, fmt.Errorf("must provide cpu cores")
	}

	if wsCfg.Resources.CPU != confCfg.Resources.CPU {
		isSame = false
	}

	// make sure memory is set
	if wsCfg.Resources.Mem <= 0 {
		return map[string]interface{}{"message": "must provide memory"}, fmt.Errorf("must provide memory")
	}

	if wsCfg.Resources.Mem != confCfg.Resources.Mem {
		isSame = false
	}

	// make sure disk is set
	if wsCfg.Resources.Disk <= 0 {
		return map[string]interface{}{"message": "must provide disk"}, fmt.Errorf("must provide disk")
	}

	if wsCfg.Resources.Disk != confCfg.Resources.Disk {
		isSame = false
	}

	// make sure no more than 6 cores are used
	if wsCfg.Resources.CPU > 6 {
		return map[string]interface{}{"message": "cannot use more than 6 CPU cores"}, fmt.Errorf("cannot use more than 6 CPU cores")
	}

	// make sure no more than 8gb of memory is used
	if wsCfg.Resources.Mem > 8 {
		return map[string]interface{}{"message": "cannot use more than 8 GB of RAM"}, fmt.Errorf("cannot use more than 8 GB of RAM")
	}

	// make sure no more than 100gb of storage is used
	if wsCfg.Resources.Disk > 100 {
		return map[string]interface{}{"message": "cannot use more than 100 GB of disk space"}, fmt.Errorf("cannot use more than 100 GB of disk space")
	}

	meiliUpdate := map[string]interface{}{
		"_id": workspaceConfigID,
	}

	if content != conf.Content {
		isSame = false
		meiliUpdate["content"] = content
	}

	newTags := make([]int64, 0)

	for _, tag := range workspaceConfigTags {
		for _, existingTag := range conf.Tags {
			if existingTag == tag.ID {
				isSame = false
				break
			}
			newTags = append(newTags, tag.ID)
		}
		meiliUpdate["tags"] = newTags
	}

	if description != conf.Description {
		isSame = false
		meiliUpdate["description"] = description
	}

	if isSame {
		logger.Errorf("no changes found in EditPublicConfigTemplate for config: %v", conf.ID)
		return map[string]interface{}{"message": "no changes made"}, nil
	}

	meiliUpdate["revision"] = conf.Revision + 1

	conf.Content = content
	conf.Tags = newTags
	conf.Revision += 1
	conf.Description = description
	statements, err := conf.ToSQLNative()

	logger.Debugf("EditPublicConfigTemplate revision number: %v", conf.Revision)
	if err != nil {
		logger.Errorf("failed to format edit workspace config to sql: %v", err)
		return nil, fmt.Errorf("failed to format workspace config to sql: %v", err)
	}

	for _, statement := range statements {
		_, err = tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			logger.Errorf("failed to perform edit workspace config update: %v", err)
			return nil, fmt.Errorf("failed to perform workspace config update: %v", err)
		}

	}

	// perform search engine insertion
	err = meili.UpdateDocuments("workspace_configs", meiliUpdate)
	if err != nil {
		logger.Errorf("failed to insert edited workspace config to search engine: %v", err)
		return nil, fmt.Errorf("failed to insert workspace config to search engine: %v", err)
	}

	logger.Infof("successfully updated workspace config: %v", conf.ID)
	return map[string]interface{}{"message": "successfully updated workspace config", "config": conf}, nil

}

func EditConfig(ctx context.Context, vcsClient *git.VCSClient, callingUser *models.User, repoId int64, content string, commit string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-config-core")
	defer span.End()

	// validate that the config is in the right format
	var wsCfg workspace_config.GigoWorkspaceConfig
	err := yaml.Unmarshal([]byte(content), &wsCfg)
	if err != nil {
		return map[string]interface{}{"message": "config is not the right format"}, err
	}

	if wsCfg.Version != 0.1 {
		return map[string]interface{}{"message": "version must be 0.1"}, fmt.Errorf("version must be 0.1")
	}

	if wsCfg.BaseContainer == "" {
		return map[string]interface{}{"message": "must have a base container"}, fmt.Errorf("must have a base container")
	}

	if wsCfg.WorkingDirectory == "" {
		return map[string]interface{}{"message": "must have a working directory"}, fmt.Errorf("must have a working directory")
	}

	// make sure cpu cores are set
	if wsCfg.Resources.CPU <= 0 {
		return map[string]interface{}{"message": "must provide cpu cores"}, fmt.Errorf("must provide cpu cores")
	}

	// make sure memory is set
	if wsCfg.Resources.Mem <= 0 {
		return map[string]interface{}{"message": "must provide memory"}, fmt.Errorf("must provide memory")
	}

	// make sure disk is set
	if wsCfg.Resources.Disk <= 0 {
		return map[string]interface{}{"message": "must provide disk"}, fmt.Errorf("must provide disk")
	}

	// make sure no more than 6 cores are used
	if wsCfg.Resources.CPU > 6 {
		return map[string]interface{}{"message": "cannot use more than 6 CPU cores"}, fmt.Errorf("cannot use more than 6 CPU cores")
	}

	// make sure no more than 8gb of memory is used
	if wsCfg.Resources.Mem > 8 {
		return map[string]interface{}{"message": "cannot use more than 8 GB of RAM"}, fmt.Errorf("cannot use more than 8 GB of RAM")
	}

	// make sure no more than 100gb of storage is used
	if wsCfg.Resources.Disk > 100 {
		return map[string]interface{}{"message": "cannot use more than 100 GB of disk space"}, fmt.Errorf("cannot use more than 100 GB of disk space")
	}

	// TODO make sure docker container is valid

	// get repository name from repo id
	repo, _, err := vcsClient.GiteaClient.GetRepoByID(repoId)
	if err != nil {
		return map[string]interface{}{"message": "failed to locate repo"}, fmt.Errorf("failed to locate repo %d: %v", repoId, err)
	}

	// convert the username back into an int64 and compare it to the calling user id
	repoOwnerId, err := strconv.ParseInt(repo.Owner.UserName, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo owner id %d: %v", repoId, err)
	}

	if repoOwnerId != callingUser.ID {
		return map[string]interface{}{"message": "you do not have permission to edit this repo"}, fmt.Errorf("you do not have permission to edit this repo")
	}

	// retrieve file from existing repo head
	fileMeta, _, err := vcsClient.GiteaClient.GetContents(
		fmt.Sprintf("%d", callingUser.ID),
		repo.Name,
		"main",
		".gigo/workspace.yaml",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file from repo %d: %v", repoId, err)
	}

	res, err := GetConfig(ctx, vcsClient, callingUser, repoId, commit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve existing config %d: %v", repoId, err)
	}

	oldCfgContent := res["ws_config"].(string)

	var oldCfg workspace_config.GigoWorkspaceConfig
	err = yaml.Unmarshal([]byte(oldCfgContent), &oldCfg)
	if err == nil {
		if len(compareStructs(wsCfg, oldCfg)) < 1 {
			return map[string]interface{}{"message": "config is the same"}, nil
		}
	}

	// encode workspace config content to base64
	workspaceConfigContentBase64 := base64.StdEncoding.EncodeToString([]byte(content))

	_, gitRes, err := vcsClient.GiteaClient.UpdateFile(
		fmt.Sprintf("%d", callingUser.ID),
		repo.Name,
		".gigo/workspace.yaml",
		gitea.UpdateFileOptions{
			SHA:     fileMeta.SHA,
			Content: workspaceConfigContentBase64,
			FileOptions: gitea.FileOptions{
				BranchName: "main",
				Author: gitea.Identity{
					Name:  "Gigo",
					Email: "gigo@gigo.dev",
				},
				Committer: gitea.Identity{
					Name:  "Gigo",
					Email: "gigo@gigo.dev",
				},
			},
		},
	)
	if err != nil {
		buf, _ := io.ReadAll(gitRes.Body)
		return map[string]interface{}{"message": "failed to update the workspace config in repo"}, fmt.Errorf("failed to update the workspace config in repo: %v\n    res: %v", err, string(buf))
	}

	return map[string]interface{}{"message": "repo config updated successfully.", "repo": fmt.Sprintf("%d", repoId)}, nil
}

func ConfirmEditConfig(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, wsStatusUpdater *utils.WorkspaceStatusUpdater, callingUser *models.User, projectID int64, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "confirm-edit-config-core")
	defer span.End()

	callerName := "ConfirmEditConfig"

	queryRes, err := tidb.QueryContext(ctx, &span, &callerName, "select _id, code_source_type, state from workspaces where code_source_id = ? and owner_id = ?", projectID, callingUser.ID)
	if err != nil {
		logger.Errorf("failed to retrieve workspace: %v, err: %v", projectID, zap.Error(err))
		return map[string]interface{}{"message": "failed to retrieve workspace"}, fmt.Errorf("failed to retrieve workspace: %v", err)
	}

	defer queryRes.Close()

	var wsId int64
	var codeSourceType int
	var state int
	for queryRes.Next() {
		err = queryRes.Scan(&wsId, &codeSourceType, &state)
		if err != nil {
			return map[string]interface{}{"message": "failed to retrieve workspace"}, fmt.Errorf("failed to retrieve workspace: %v", err)
		}
	}

	logger.Debugf("inside confirm-edit-config-core: %v", codeSourceType)

	if codeSourceType == 1 {

		_, err := tidb.ExecContext(ctx, &span, &callerName, "update attempt set updated_at = ? where _id = ?", time.Now(), projectID)
		if err != nil {
			return map[string]interface{}{"message": "failed to update attempt in edit config"}, fmt.Errorf("failed to update attempt in edit config: %v", err)
			//logger.Errorf("failed to update attempt in edit config: %v, err: %v", wsId, zap.Error(err))
		}
	} else {
		_, err := tidb.ExecContext(ctx, &span, &callerName, "update post set updated_at = ? where _id = ?", time.Now(), projectID)
		if err != nil {
			return map[string]interface{}{"message": "failed to update post in edit config"}, fmt.Errorf("failed to update attempt in edit config: %v", err)
			//logger.Errorf("failed to update post in edit config: %v, err: %v", wsId, zap.Error(err))
		}
	}

	logger.Debugf("project id inside of ConfirmEditConfig: %v", projectID)

	if wsId <= 0 {
		return map[string]interface{}{"message": "config edit confirmed successfully"}, nil
	}

	logger.Debug("workspace id inside confirm edit: %v", wsId)

	if state < 4 {
		res, err := DestroyWorkspace(ctx, tidb, js, wsStatusUpdater, callingUser, wsId)
		if err != nil {
			logger.Errorf("failed to destroy workspace: %v, err: %v", wsId, zap.Error(err))
			return map[string]interface{}{"message": "failed to destroy workspace"}, fmt.Errorf("failed to destroy workspace: %v", err)
		}

		logger.Debug("workspace message: %v", res["message"])
		logger.Debug("workspace message string: %v", res["message"].(string))

		if res["message"].(string) != "Workspace is destroying." {
			logger.Errorf("failed to destroy workspace: %v, err: incorrect response from destroy workspace func: %v", wsId, res["message"].(string))
			return map[string]interface{}{"message": "failed to destroy workspace"}, fmt.Errorf("failed to destroy workspace: %v", res["message"])
		}
	}

	return map[string]interface{}{"message": "config edit confirmed successfully"}, nil
}

func GetConfig(ctx context.Context, vcsClient *git.VCSClient, callingUser *models.User, repo int64, commit string) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-config-core")
	defer span.End()

	// get repository name from repo id
	repository, _, err := vcsClient.GiteaClient.GetRepoByID(repo)
	if err != nil {
		fmt.Errorf("failed to locate repo %d: %v", repo, err)
	}

	// retrieve the gigo workspace config from the passed branch
	configBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", callingUser.ID),
		repository.Name,
		commit,
		".gigo/workspace.yaml",
	)
	if err != nil {
		if gitRes.StatusCode != http.StatusNotFound {
			fmt.Errorf("workspace config not found")
		}
		buf, _ := io.ReadAll(gitRes.Body)
		return nil, fmt.Errorf("failed to retrieve gigoconfig: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
	}

	return map[string]interface{}{"ws_config": string(configBytes)}, nil
}

func compareStructs(a, b interface{}) []string {
	var diffs []string

	aValue := reflect.ValueOf(a)
	bValue := reflect.ValueOf(b)

	if aValue.Kind() != bValue.Kind() {
		diffs = append(diffs, "Different types")
		return diffs
	}

	if aValue.Type() != bValue.Type() {
		diffs = append(diffs, "Different struct types")
		return diffs
	}

	for i := 0; i < aValue.NumField(); i++ {
		aField := aValue.Field(i)
		bField := bValue.Field(i)

		if !reflect.DeepEqual(aField.Interface(), bField.Interface()) {
			fieldName := aValue.Type().Field(i).Name
			diffs = append(diffs, fmt.Sprintf("Different value for field '%s'", fieldName))
		}
	}

	return diffs
}

func CloseAttempt(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, callingUser *models.User, attemptId int64, title string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "close-attempt-core")
	defer span.End()
	callerName := "CloseAttempt"

	// query for the attempt the user is trying to close
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from attempt where _id = ? and author_id =  ? limit 1", attemptId, callingUser.ID)
	if err != nil {
		return map[string]interface{}{"message": "failed to locate attempt"}, fmt.Errorf("failed to locate attempt %d: %v", attemptId, err)
	}

	// check if attempt was found with the given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no attempt found with given id: %v", err)
	}

	// attempt to decode res into attempt model
	attempt, err := models.AttemptFromSQLNative(tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for attempt. CloseAttempt core. Error: %v", err)
	}

	// close
	_ = res.Close()

	// mark the selected attempt as closed and mark the date
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update attempt set closed = ?, closed_date = ?, title = ? where author_id = ? and _id = ?",
		true, time.Now().Format("2006-01-02"), title, callingUser.ID, attemptId)
	if err != nil {
		return map[string]interface{}{"message": "failed to close attempt"}, fmt.Errorf("failed to close attempt: %v", err)
	}

	trueBool := true

	// archive the attempt in git so that it can no longer be edited
	_, gitRes, err := vcsClient.GiteaClient.EditRepo(
		fmt.Sprintf("%d", attempt.AuthorID),
		fmt.Sprintf("%d", attempt.ID),
		gitea.EditRepoOption{Archived: &trueBool},
	)
	if err != nil {
		if gitRes.StatusCode != 404 {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve attempt repo: %v\n     response: %d - %q", err, gitRes.StatusCode, string(buf))
		}
	}

	// load user timezone
	timeLocation, err := time.LoadLocation(callingUser.Timezone)
	if err != nil {
		return map[string]interface{}{"message": "failed to get user timezone"}, fmt.Errorf("failed to get user timezone: %v", err)

	}

	// calculate the beginning of the day for the user
	date := now.BeginningOfDay().In(timeLocation)

	// mark the selected attempt as closed and mark the date
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update user_stats set challenges_completed = (challenges_completed + 1) where user_id = ? and date = ?", callingUser.ID, date)
	if err != nil {
		return map[string]interface{}{"message": "failed to close attempt"}, fmt.Errorf("failed to close attempt: %v", err)
	}

	return map[string]interface{}{"message": "Attempt Closed Successfully"}, nil
}

func MarkSuccess(ctx context.Context, tidb *ti.Database, js *mq.JetstreamClient, rdb redis.UniversalClient, sf *snowflake.Node, stripeSubConfig config.StripeSubscriptionConfig, attemptId int64, logger logging.Logger, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "mark-success-core")
	defer span.End()
	callerName := "MarkSuccess"

	res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from attempt where _id = ?", attemptId)
	if err != nil {
		return map[string]interface{}{"message": "failed to query for attempt"},
			fmt.Errorf("failed to query for attempt %d: %v", attemptId, err)
	}

	// check if attempt was found with the given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no attempt found with given id: %v", err)
	}

	// attempt to decode res into attempt model
	attempt, err := models.AttemptFromSQLNative(tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for attempt. CloseAttempt core. Error: %v", err)
	}

	// close
	_ = res.Close()

	// make sure the attempt is closed before marking it as success
	if attempt.Closed != true {
		return map[string]interface{}{"message": "attempt not closed"}, fmt.Errorf("attempt not closed: %v", err)
	}

	// mark the selected attempt as a successful attempt to the challenge
	_, err = tidb.ExecContext(ctx, &span, &callerName, "update attempt set success = true where _id = ?", attemptId)
	if err != nil {
		return map[string]interface{}{"message": "failed to close attempt"}, fmt.Errorf("failed to close attempt: %v", err)
	}

	// add xp to user for logging in
	xpRes, err := AddXP(ctx, tidb, js, rdb, sf, stripeSubConfig, attempt.AuthorID, "successful", &attempt.Tier, nil, logger, callingUser)
	if err != nil {
		return map[string]interface{}{"message": "Attempt Marked as a Success"}, fmt.Errorf("failed to add xp to user: %v", err)
	}

	return map[string]interface{}{"message": "Attempt Marked as a Success", "xp": xpRes}, nil
}

func ShareLink(ctx context.Context, tidb *ti.Database, postId int64, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "post-share-link")
	defer span.End()
	callerName := "postShareLink"

	var shareHash sql.NullString
	err := tidb.QueryRowContext(ctx, &span, &callerName, "select bin_to_uuid(share_hash) from post where _id = ? and author_id = ?", postId, callingUser.ID).Scan(&shareHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check for share hash for post: %v", err)
	}

	if !shareHash.Valid {
		newShareHash := uuid.New()
		_, err = tidb.ExecContext(ctx, &span, &callerName, "update post set share_hash = uuid_to_bin(?) where _id = ? and author_id = ?", newShareHash, postId, callingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update share hash for post: %v", err)
		}
		return map[string]interface{}{"message": newShareHash.String()}, nil
	}

	return map[string]interface{}{"message": shareHash.String}, nil
}

func VerifyLink(ctx context.Context, tidb *ti.Database, postId int64, shareLink string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "verify-share-link")
	defer span.End()
	callerName := "verifyShareLink"

	// Check if shareLink is a valid UUID
	_, err := uuid.Parse(shareLink)
	if err != nil {
		return map[string]interface{}{"message": "invalid share link"}, nil
	}

	var verified bool
	err = tidb.QueryRowContext(ctx, &span, &callerName, "select exists(select 1 from post where _id = ? and share_hash = uuid_to_bin(?))", postId, shareLink).Scan(&verified)
	if err != nil {
		return nil, fmt.Errorf("failed to verify share link: %v", err)
	}

	if !verified {
		return map[string]interface{}{"message": "invalid share link"}, nil
	}

	return map[string]interface{}{"message": "valid share link"}, nil
}
