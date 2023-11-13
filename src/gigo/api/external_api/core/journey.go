package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/workspace_config"
	"github.com/gage-technologies/gitea-go/gitea"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
	"io"
	"time"
)

// SaveJourneyInfo saves the user's form and optional aptitude level to the database
func SaveJourneyInfo(ctx context.Context, tiDB *ti.Database, snowflakeNode *snowflake.Node, callingUser *models.User, journeyInfo models.JourneyInfo) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "save-journey-info-core")
	defer span.End()
	callerName := "SaveJourneyInfo"

	// build query to check if journey info with specific user_id already exists
	journeyInfoQuery := "select _id from journey_info where user_id = ?"

	// query journey_info to check for existence of entry with the user_id
	response, err := tiDB.QueryContext(ctx, &span, &callerName, journeyInfoQuery, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing journey_info: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	// check if any rows exist for that user_id
	if response.Next() {
		// a row exists for this user_id
		return map[string]interface{}{
			"message": "Journey info for this user already exists",
		}, nil
	}

	journey, err := models.CreateJourneyInfo(snowflakeNode.Generate().Int64(), callingUser.ID, journeyInfo.LearningGoal,
		journeyInfo.SelectedLanguage, journeyInfo.EndGoal, journeyInfo.ExperienceLevel, journeyInfo.FamiliarityIDE,
		journeyInfo.FamiliarityLinux, journeyInfo.Tried, journeyInfo.TriedOnline, journeyInfo.AptitudeLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create journey info: %v", err)
	}

	journeyInsertion := journey.ToSQLNative()

	// create tx for insertion
	tx, err := tiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer rollback in case we fail
	defer tx.Rollback()

	// iterate insertion statements executing insertions via tx
	for _, statement := range journeyInsertion {
		_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert journey info: %v", err)
		}
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	return map[string]interface{}{"message": "journey info saved"}, nil

}

// CheckJourneyInfo only checks if a user has info so the frontend knows to skip the form
func CheckJourneyInfo(ctx context.Context, tiDB *ti.Database, callingUser *models.User) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-journey-info-core")
	defer span.End()
	callerName := "CheckJourneyInfo"

	// build query to check if user already has journey information saved
	nameQuery := "select * from journey_info where user_id = ?"

	// query journey_info to ensure info for this user does not already exist
	response, err := tiDB.QueryContext(ctx, &span, &callerName, nameQuery, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for journey_info: %v", err)
	}

	// ensure the closure of the rows
	defer response.Close()

	if response.Next() {
		return map[string]interface{}{"message": "journey info exists"}, nil
	}

	return map[string]interface{}{"message": "no journey info"}, nil
}

// TODO make sure all necessary workspace config functionality is implemented

func CreateJourneyUnit(ctx context.Context, tiDB *ti.Database, sf *snowflake.Node, callingUser *models.User,
	title string, unitFocus models.UnitFocus, languages []models.ProgrammingLanguage, description string,
	tags []string, tier models.TierType, challengeCost *string, vcsClient *git.VCSClient, workspaceConfigContent string,
	workspaceConfigTitle string, workspaceConfigDescription string, workspaceConfigLangs []models.ProgrammingLanguage,
	workspaceSettings *models.WorkspaceSettings, estimatedTutorialTime *time.Duration) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-journey-unit-core")
	defer span.End()
	callerName := "CreateJourneyUnit"

	// validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" || callingUser.AuthRole != models.Admin {
		return nil, fmt.Errorf("callinguser is not an admin. CreateJourneyUnit core")
	}

	id := sf.Generate().Int64()

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

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = vcsClient.DeleteRepo(fmt.Sprintf("%d", callingUser.ID), fmt.Sprintf("%d", id))

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

	wsCfg := models.CreateWorkspaceConfig(
		sf.Generate().Int64(),
		workspaceConfigTitle,
		workspaceConfigDescription,
		workspaceConfigContent,
		callingUser.ID,
		0,
		nil,
		workspaceConfigLangs,
	)

	// create new journey unit
	journey, err := models.CreateJourneyUnit(id, title, unitFocus, languages, description,
		repo.ID, time.Now(), time.Now(), tags, tier, workspaceSettings, wsCfg.ID, estimatedTutorialTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey unit: %v", err)
	}

	journeyInsertion := journey.ToSQLNative()

	// create tx for insertion
	tx, err := tiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer rollback in case we fail
	defer tx.Rollback()

	// iterate insertion statements executing insertions via tx
	for _, statement := range journeyInsertion {
		_, err := tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert journey unit: %v", err)
		}
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	return map[string]interface{}{"message": "journey unit created"}, nil
}

// todo create journey unit projects
func CreateJourneyProject(ctx context.Context, tiDB *ti.Database, sf *snowflake.Node, callingUser *models.User,
	unitID int64, title string, description string, language *models.ProgrammingLanguage, tags []string,
	dependencies []models.JourneyDependencies) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-journey-project-core")
	defer span.End()
	callerName := "CreateJourneyProject"

	// validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" || callingUser.AuthRole != models.Admin {
		return nil, fmt.Errorf("callinguser is not an admin. CreateJourneyProject core")
	}

	// ID:
	// Completions:
	// EstimatedTutorialTime:

	return map[string]interface{}{"message": "journey project created"}, nil
}

// todo create journey unit attempt
func CreateJourneyUnitAttempt(ctx context.Context, tiDB *ti.Database, sf *snowflake.Node, callingUser *models.User,
	parentUnit int64, title string, unitFocus models.UnitFocus, languages []models.JourneyUnitLanguages,
	description string, repoID int64, challengeCost *string, tags []string, tier models.TierType,
	workspaceConfigContent string, workspaceConfigTitle string, workspaceConfigDescription string,
	workspaceConfigLangs []models.ProgrammingLanguage, workspaceSettings *models.WorkspaceSettings,
	estimatedTutorialTime *time.Duration) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-journey-info-core")
	defer span.End()
	callerName := "CreateJourneyUnitAttempt"

	// validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" || callingUser.AuthRole != models.Admin {
		return nil, fmt.Errorf("callinguser is not an admin. CreateJourneyUnitAttempt core")
	}
}

// todo create journey unit project attempt
func CreateJourneyProjectAttempt(ctx context.Context, tiDB *ti.Database, sf *snowflake.Node, callingUser *models.User,
	unitID int64, parentProject int64, workingDirectory string, title string, description string,
	language *models.ProgrammingLanguage, tags []string, dependencies []models.JourneyDependencies,
	estimatedTutorialTime *time.Duration) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-journey-project-attempt-core")
	defer span.End()
	callerName := "CreateJourneyProjectAttempt"

	// validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" || callingUser.AuthRole != models.Admin {
		return nil, fmt.Errorf("callinguser is not an admin. CreateJourneyProjectAttempt core")
	}
}

// todo delete journey unit

// todo delete journey unit projects

// todo delete journey unit attempt

// todo delete journey unit project attempt
