package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/otel"

	"gigo-core/gigo/api/external_api/core/query_models"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
)

type TutorialContentFrontend struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

// ProjectInformation
// Grabs information for specified project:
//
// Args:
//
//	tidb       - *ti.Database, tidb
//	projectId  - int64, id of selected project
//
// Returns:
//
//	out        - *models.Post, Post model
//			   - error
func ProjectInformation(ctx context.Context, tidb *ti.Database, vcsClient *git.VCSClient, callingUser *models.User, projectId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "project-information-core")
	defer span.End()
	callerName := "ProjectInformation"

	var callerId int64

	if callingUser != nil {
		callerId = callingUser.ID
	}

	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName,
		"select p._id as _id, title, description, author, p.deleted as deleted, author_id, p.created_at as created_at, updated_at, repo_id, p.tier as tier, top_reply, p.coffee as coffee, post_type, views, completions, attempts, published, stripe_price_id,challenge_cost, workspace_config, p.workspace_settings, leads, embedded, r._id as reward_id, name, color_palette, render_in_front, exclusive_description, estimated_tutorial_time, p.start_time as start_time from post p join users u on p.author_id = u._id left join rewards r on r._id = u.avatar_reward where p._id = ? and ((visibility = ? and author_id = ?) or visibility = ?) limit 1",
		projectId, models.PrivateVisibility, callerId, models.PublicVisibility,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query post: %v", err)
	}

	// check if post was found with given id
	if res == nil || !res.Next() {
		return nil, fmt.Errorf("no post found with given id: %v", err)
	}

	// attempt to decode res into post model
	post, err := query_models.PostUserBackgroundFromSQLNative(ctx, tidb, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for post. ProjectInformation core. Error: %v", err)
	}

	// close explicitly
	_ = res.Close()

	// create variable to hold attempt
	var attemptFrontend *models.AttemptFrontend

	// query to see if the calling user has started an attempt
	if callingUser != nil {
		// query for attempt by user
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from attempt where post_id = ? and author_id = ? limit 1", projectId, callingUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query attempt: %v", err)
		}

		// conditionally load attempt
		if res.Next() {
			// attempt to decode res into attempt model
			attempt, err := models.AttemptFromSQLNative(tidb, res)
			if err != nil {
				return nil, fmt.Errorf("failed to load attempt from cursor: %v", err)
			}

			// close explicitly
			_ = res.Close()

			// assign frontend formatted attempt to outer scope
			attemptFrontend = attempt.ToFrontend()
		}
	}

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

	evaluationBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", post.AuthorID),
		fmt.Sprintf("%d", post.ID),
		"main",
		"EVALUATION.md",
	)
	if err != nil {
		if gitRes.StatusCode != 404 {
			buf, _ := io.ReadAll(gitRes.Body)
			return nil, fmt.Errorf("failed to retrieve evaluation: %v\n    response: %d - %q", err, gitRes.StatusCode, string(buf))
		}
		evaluationBytes = []byte("")
	}

	// if this is an interactive then we want to get the first 5 tutorials
	tutorials := make([]TutorialContentFrontend, 0)
	if post.PostType == models.InteractiveChallenge {
		// list the contents of the tutorials directory
		tutorialFiles, giteaRes, err := vcsClient.GiteaClient.ListContents(
			fmt.Sprintf("%d", post.AuthorID),
			fmt.Sprintf("%d", post.ID),
			"main",
			".gigo/.tutorials",
		)
		if err != nil {
			if giteaRes == nil || giteaRes.StatusCode != 404 {
				return nil, fmt.Errorf("failed to retrieve tutorials: %v", err)
			}
		}

		// filter the files for only those with the tutorial-X.md format and are below 6
		selectedFiles := make([]string, 0)
		for _, file := range tutorialFiles {
			if strings.HasPrefix(file.Name, "tutorial-") && strings.HasSuffix(file.Name, ".md") {
				// validate that the tutorial number is below 6
				if len(file.Name) != 13 || file.Name[9] >= '6' {
					continue
				}
				selectedFiles = append(selectedFiles, file.Name)
			}
		}

		// sort the files
		sort.Strings(selectedFiles)

		// retrieve the contents of the files and add them to the tutorials slice
		for _, file := range selectedFiles {
			fileBytes, giteaRes, err := vcsClient.GiteaClient.GetFile(
				fmt.Sprintf("%d", post.AuthorID),
				fmt.Sprintf("%d", post.ID),
				"main",
				".gigo/.tutorials/"+file,
			)
			if err != nil {
				if giteaRes == nil || giteaRes.StatusCode != 404 {
					return nil, fmt.Errorf("failed to retrieve tutorial: %v", err)
				}
			}

			tutorials = append(tutorials, TutorialContentFrontend{
				// this is a cute trick to convert the rune to an int - this only works on single digit numbers
				Number:  int(file[9] - '0'),
				Content: string(fileBytes),
			})
		}
	}

	// format post to frontend
	fp, err := post.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format post to frontend: %v", err)
	}

	if post.Published != true && callingUser.ID != post.AuthorID {
		return map[string]interface{}{
			"post": "user is not authorized to view this post.",
		}, nil
	}

	if post.ChallengeCost != nil {
		// query attempt and projects with the user id as author id and sort by date last edited
		hasAccessRes, err := tidb.QueryContext(ctx, &span, &callerName, "select count(*) as count from exclusive_content_purchases where user_id = ? and post = ? ", callingUser.ID, post.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
		}

		defer hasAccessRes.Close()

		var dataObject int64

		for hasAccessRes.Next() {
			// attempt to load count from row
			err = hasAccessRes.Scan(&dataObject)
			if err != nil {
				return nil, fmt.Errorf("failed to get follower count: %v", err)
			}
		}

		trueBool := true
		falseBool := false

		fmt.Println("access is: ", dataObject)

		if dataObject > 0 || callingUser.ID == post.ID {
			fp.HasAccess = &trueBool
		} else {
			fp.HasAccess = &falseBool
		}
	} else {
		fmt.Println("access is null")
		fp.HasAccess = nil
	}

	return map[string]interface{}{
		"post":        fp,
		"description": string(readMeBytes),
		"evaluation":  string(evaluationBytes),
		"attempt":     attemptFrontend,
		"tutorials":   tutorials,
	}, nil
}

// ProjectAttempts
// Grabs attempts for specified project sorted by most recent attempt:
//
// Args:
//
//		tidb       - *ti.Database, tidb
//		projectId  - int64, id of selected project
//	 limit	   - *int, optional integer to limit the number of returned attempts
//
// Returns:
//
//	out        - []*models.Attempt, an array of attempts for specified project sorted by most recent creation
//			   - error
func ProjectAttempts(ctx context.Context, tidb *ti.Database, projectId int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "project-attempts-core")
	defer span.End()
	callerName := "ProjectAttempts"

	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select a._id as _id, post_title, description, author, author_id, a.created_at as created_at, updated_at, repo_id, author_tier, a.coffee as coffee, post_id, closed, success, closed_date, a.tier as tier, parent_attempt, a.workspace_settings as workspace_settings, r._id as reward_id, name, color_palette, render_in_front from attempt a join users u on a.author_id = u._id left join rewards r on u.avatar_reward = r._id where post_id = ? order by created_at desc limit ? offset ?", projectId, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query post: %v\n    query: %s\n    values: %v", err,
			"select * from attempt where post_id = ? order by created_at desc limit ? offset ?",
			[]interface{}{projectId, limit, skip})
	}

	// ensure the closure of the rows
	defer res.Close()

	// check if post was found with given id
	if res == nil {
		return nil, fmt.Errorf("no post found with given id: %v", projectId)
	}

	// make slice to hold attempt
	attempts := make([]*query_models.AttemptUserBackgroundFrontend, 0)

	// iterate cursor loading attempts from the rows
	for res.Next() {
		attempt, err := query_models.AttemptUserBackgroundFromSQLNative(ctx, tidb, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for results. recommended Project Home core.    Error: %v", err)
		}

		attempts = append(attempts, attempt.ToFrontend())
	}

	return map[string]interface{}{"attempts": attempts}, nil
}

func isBinary(data []byte) bool {
	count := 0
	for _, b := range data {
		if b < 0x20 || b > 0x7E {
			count++
		}

		// exit if greater than 25% of bytes are non-ascii
		if count >= len(data)/4 {
			return true
		}
	}
	return false
}

func GetProjectCode(ctx context.Context, vcsClient *git.VCSClient, repo int64, ref string, filePath string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-project-core")
	defer span.End()

	fmt.Sprintf("we just started")

	// TODO make sure that is checks that this is public repo

	repositories, _, err := vcsClient.GiteaClient.GetRepoByID(repo)

	project, _, err := vcsClient.GiteaClient.ListContents(repositories.Owner.UserName, repositories.Name, ref, filePath)
	if err != nil {
		return map[string]interface{}{"message": "Unable to get project contents"}, err
	}

	for i := range project {
		if project[i].Type == "file" {
			project[i].Content = nil
		}
	}

	return map[string]interface{}{"message": project}, nil
}

// function will get file contents by repo and file information using gitea
func GetProjectFile(ctx context.Context, vcsClient *git.VCSClient, repo int64, ref string, filePath string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-project-file-core")
	defer span.End()

	repositories, _, err := vcsClient.GiteaClient.GetRepoByID(repo)

	backupContent := "This cannot be displayed here."

	file, _, err := vcsClient.GiteaClient.GetContents(repositories.Owner.UserName, repositories.Name, ref, filePath)
	if err != nil {
		return map[string]interface{}{"message": "Unable to get project contents"}, err
	}

	valid := utf8.ValidString(*file.Content)

	fmt.Sprintf("the validity is: %v      the file name is: %v", valid, file.Name)

	if file.Size <= 64 * 1024 {
		rawDecodedText, err := base64.StdEncoding.DecodeString(*file.Content)
		if err != nil {
			return map[string]interface{}{"message": "Unable to decode contents"}, err
		}

		if isBinary(rawDecodedText) {
			backupContent = "This file is binary content and connot be displayed here."
			file.Content = &backupContent
		} else {
			finalText := string(rawDecodedText)

			file.Content = &finalText
		}
	} else {
		backupContent = "This file is too large to be displayed here."
		file.Content = &backupContent
	}

	return map[string]interface{}{"message": file}, nil
}

// function will get file contents by repo and file information using gitea
func GetProjectDirectories(ctx context.Context, vcsClient *git.VCSClient, repo int64, ref string, filePath string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-project-directories-core")
	defer span.End()

	repositories, _, err := vcsClient.GiteaClient.GetRepoByID(repo)

	project, _, err := vcsClient.GiteaClient.ListContents(repositories.Owner.UserName, repositories.Name, ref, filePath)
	if err != nil {
		return map[string]interface{}{"message": "Unable to get project contents"}, err
	}

	backupContent := "This cannot be displayed here."

	for i := range project {
		if project[i].Type == "file" {
			file, _, err := vcsClient.GiteaClient.GetContents(repositories.Owner.UserName, repositories.Name, ref, project[i].Path)
			if err != nil {
				return map[string]interface{}{"message": "Unable to get project contents"}, err
			}

			//valid := utf8.ValidString(*file.Content)
			//
			//fmt.Sprintf("the validity is: %v      the file name is: %v", valid, file.Name)

			//fileSize := .000001 * file.Size
			if file.Size <= 64 * 1024 {
				rawDecodedText, err := base64.StdEncoding.DecodeString(*file.Content)
				if err != nil {
					return map[string]interface{}{"message": "Unable to decode contents"}, err
				}

				if isBinary(rawDecodedText) {
					project[i].Content = &backupContent
				} else {
					finalText := string(rawDecodedText)

					project[i].Content = &finalText
				}
			} else {
				project[i].Content = &backupContent
			}
		}
	}

	return map[string]interface{}{"message": project}, nil
}

func GetClosedAttempts(ctx context.Context, tidb *ti.Database, projectId int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-closed-attempts-core")
	defer span.End()
	callerName := "GetClosedAttempts"

	// query for all active projects for specified user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select a._id as _id, post_title, description, author, author_id, a.created_at as created_at, updated_at, repo_id, author_tier, a.coffee as coffee, post_id, closed, success, closed_date, a.tier as tier, parent_attempt, a.workspace_settings as workspace_settings, r._id as reward_id, name, color_palette, render_in_front from attempt a join users u on a.author_id = u._id left join rewards r on u.avatar_reward = r._id where post_id = ? and closed = true order by created_at desc limit ? offset ?", projectId, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query attempt: %v\n    query: %s\n    values: %v", err,
			"select * from attempt where post_id = ? and closed = true order by created_at desc limit ? offset ?",
			[]interface{}{projectId, limit, skip})
	}

	// ensure the closure of the rows
	defer res.Close()

	// check if post was found with given id
	if res == nil {
		return nil, fmt.Errorf("no post found with given id: %v", projectId)
	}

	// make slice to hold attempt
	attempts := make([]*query_models.AttemptUserBackgroundFrontend, 0)

	// iterate cursor loading attempts from the rows
	for res.Next() {
		attempt, err := query_models.AttemptUserBackgroundFromSQLNative(ctx, tidb, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for results. recommended Project Home core.    Error: %v", err)
		}

		attempts = append(attempts, attempt.ToFrontend())
	}

	return map[string]interface{}{"attempts": attempts}, nil
}
