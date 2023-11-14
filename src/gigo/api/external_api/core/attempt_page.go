package core

import (
	"context"
	"fmt"
	"github.com/gage-technologies/gigo-lib/storage"
	utils2 "github.com/gage-technologies/gigo-lib/utils"
	"io"

	"go.opentelemetry.io/otel"

	"gigo-core/gigo/api/external_api/core/query_models"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/search"
)

func ProjectAttemptInformation(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, attemptId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "project-attempt-information-core")
	defer span.End()
	callerName := "ProjectAttemptInformation"
	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select p.author_id as author_id, p._id as _id, p.challenge_cost as challenge_cost, p.start_time as start_time from post p join attempt a on a.post_id = p._id where a._id = ? limit 1", attemptId)
	if err != nil {
		return nil, fmt.Errorf("failed to query attempt: %v", err)
	}

	// check if post was found with given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no post found with given id: %v", err)
	}

	// attempt to decode res into post model
	post, err := models.PostFromSQLNative(tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for post. ProjectInformation core. Error: %v", err)
	}

	// close explicitly
	_ = res.Close()

	// retrieve the readme and evaluation documents from the corresponding repository
	readMeBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", post.AuthorID),
		fmt.Sprintf("%d", post.ID),
		"main",
		"README.md",
	)
	if err != nil {
		if gitRes.StatusCode != 404 {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve readme: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
		}
		readMeBytes = []byte("")
	}

	return map[string]interface{}{
		"description": string(readMeBytes),
		"exclusive":   post.ChallengeCost,
	}, nil
}

func getExistingFilePath(storageEngine storage.Storage, postId int64, attemptId int64) (string, error) {
	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", attemptId)))
	if err != nil {
		fmt.Printf("failed on id hash: %v\n", err)
		return fmt.Sprintf("/static/posts/t/%v", postId), nil
	}

	// write thumbnail to final location
	idHashOriginal, err := utils2.HashData([]byte(fmt.Sprintf("%d", postId)))
	if err != nil {
		fmt.Printf("failed on original id hash: %v\n", err)
		return fmt.Sprintf("/static/posts/t/%v", postId), nil
	}

	primaryPath := fmt.Sprintf("attempt/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash)

	secondaryPath := fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHashOriginal[:3], idHashOriginal[3:6], idHashOriginal)

	// Check primary path
	existsPrimary, _, err := storageEngine.Exists(primaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to check existence of %s: %v", primaryPath, err)
	}
	if existsPrimary {
		return fmt.Sprintf("/static/attempts/t/%v", attemptId), nil
	}

	// Check secondary path
	existsSecondary, _, err := storageEngine.Exists(secondaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to check existence of %s: %v", secondaryPath, err)
	}
	if existsSecondary {
		return fmt.Sprintf("/static/posts/t/%v", postId), nil
	}

	return fmt.Sprintf("/static/posts/t/%v", postId), fmt.Errorf("neither %s nor %s exist", primaryPath, secondaryPath)
}

func AttemptInformation(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, attemptId int64, storageEngine storage.Storage) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "attempt-information-core")
	defer span.End()
	callerName := "AttemptInformation"
	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select a._id as _id, post_title, description, author, author_id, a.created_at as created_at, updated_at, repo_id, author_tier, a.coffee as coffee, post_id, closed, success, closed_date, a.tier as tier, parent_attempt, a.workspace_settings, r._id as reward_id, color_palette, render_in_front, name, a.post_type as post_type, a.start_time as start_time, title from attempt a left join users u on a.author_id = u._id left join rewards r on r._id = u.avatar_reward where a._id = ? limit 1", attemptId)
	if err != nil {
		return nil, fmt.Errorf("failed to query attempt information: %v", err)
	}

	// check if post was found with given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no post found with given id: %v", err)
	}

	// attempt to decode res into post model
	attempt, err := query_models.AttemptUserBackgroundFromSQLNative(ctx, tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for post. AttemptInformation core. Error: %v", err)
	}

	// close explicitly
	_ = res.Close()

	// retrieve the readme and evaluation documents from the corresponding repository
	readMeBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", attempt.AuthorID),
		fmt.Sprintf("%d", attempt.ID),
		"main",
		"README.md",
	)
	if err != nil {
		if gitRes.StatusCode != 404 {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve readme: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
		}
		readMeBytes = []byte("")
	}

	// format post to frontend
	fp := attempt.ToFrontend()

	thumbnail, err := getExistingFilePath(storageEngine, attempt.PostID, attempt.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve thumbnail: %v", err)
	}

	fp.Thumbnail = thumbnail

	return map[string]interface{}{
		"post":        fp,
		"description": string(readMeBytes),
	}, nil
}

func GetAttemptCode(ctx context.Context, vcsClient *git.VCSClient, callingUser *models.User, repo string, ref string, filePath string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-attempt-code-core")
	defer span.End()

	ownerId := fmt.Sprintf("%d", callingUser.ID)

	project, _, err := vcsClient.GiteaClient.ListContents(ownerId, repo, ref, filePath)
	if err != nil {
		return map[string]interface{}{"message": "Unable to get project contents"}, err
	}

	return map[string]interface{}{"message": project}, nil
}

func EditDescription(ctx context.Context, id int64, meili *search.MeiliSearchEngine, project bool, newDescription string, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "edit-description-http")
	defer span.End()
	callerName := "EditDescription"

	// create transaction for image insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	if project {
		// update post description if user is the original owner
		_, err := tx.ExecContext(ctx, &callerName, "update post set description = ?, embedded = ? where _id = ?", newDescription, false, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit post description: %v", err)
		}
		// update post description in meilisearch
		err = meili.UpdateDocuments("posts", map[string]interface{}{"_id": id, "description": newDescription})
		if err != nil {
			return nil, fmt.Errorf("failed to update post description in meilisearch: %v", err)
		}
	} else {
		// update attempt description if the user is not original owner
		_, err := tx.ExecContext(ctx, &callerName, "update attempt set description = ? where _id = ?", newDescription, id)
		if err != nil {
			return nil, fmt.Errorf("failed to edit post description: %v", err)
		}
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
	}

	return map[string]interface{}{"message": "Edit successful"}, nil
}
