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
	"strconv"
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
	title string, unitFocus models.UnitFocus, visibility models.PostVisibility, languages []models.ProgrammingLanguage, description string,
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
	journey, err := models.CreateJourneyUnit(id, title, unitFocus, callingUser.ID, visibility, languages, description,
		repo.ID, time.Now(), time.Now(), tags, tier, workspaceSettings, wsCfg.ID, estimatedTutorialTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey unit: %v", err)
	}

	journeyInsertion, err := journey.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey unit, failed to call journey unit to sql native, err: %v", err)
	}

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

	if challengeCost != nil {
		cost, err := strconv.ParseInt(*challengeCost, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to copnvert journey unit cost to int, err: %v", err)
		}

		fullProjectCost := cost * 100
		productCost, err := CreateProduct(ctx, fullProjectCost, tiDB, journey.ID, callingUser)
		if err != nil {
			return map[string]interface{}{"message": "Journey Unit has been created. But there was an issue creating the pricing for it", "journey": journey}, fmt.Errorf("failed to create stripe price: %v", err)
		}

		if productCost["message"] != "Product has been created." {
			return map[string]interface{}{"message": "Project has been created. But there was an issue creating the pricing for it on function", "journey": journey}, fmt.Errorf("failed to create stripe price in function: %v", productCost["message"])
		}
	}

	return map[string]interface{}{"message": "journey unit created", "journey": journey}, nil
}

// todo create journey unit projects
func CreateJourneyProject(ctx context.Context, tiDB *ti.Database, sf *snowflake.Node, callingUser *models.User,
	unitID int64, workingDirectory string, title string, description string, language models.ProgrammingLanguage, tags []string,
	dependencies []int64, estimatedTutorialTime *time.Duration) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-journey-project-core")
	defer span.End()
	callerName := "CreateJourneyProject"

	// validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" || callingUser.AuthRole != models.Admin {
		return nil, fmt.Errorf("callinguser is not an admin. CreateJourneyProject core")
	}

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM journey_units WHERE _id = ?)"
	err := tiDB.QueryRowContext(ctx, &span, &callerName, query, unitID).Scan(&exists)
	if err != nil {
		// Handle the error, which could be because of a variety of reasons, like no rows in result set
		return nil, fmt.Errorf("failed to check for existsing journey unit: %v", err)
	}

	if !exists {
		return map[string]interface{}{"message": "journey unit does not exist"}, nil
	}

	id := sf.Generate().Int64()

	// create new journey unit
	journey, err := models.CreateJourneyUnitProjects(id, unitID, workingDirectory, 0, title, description,
		language, tags, dependencies, estimatedTutorialTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey project: %v", err)
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

	return map[string]interface{}{"message": "journey project created"}, nil
}

// todo create journey unit attempt
func CreateJourneyUnitAttempt(ctx context.Context, tiDB *ti.Database, vcsClient *git.VCSClient, userSession *models.UserSession, sf *snowflake.Node, callingUser *models.User,
	parentUnit int64, title string, unitFocus models.UnitFocus, languages []models.ProgrammingLanguage,
	description string, repoID int64, tags []string, tier models.TierType,
	workspaceConfig int64, parentUnitAuthorID int64, parentUnitVisibility models.PostVisibility,
	workspaceConfigRevision int, workspaceSettings *models.WorkspaceSettings,
	estimatedTutorialTime *time.Duration) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "check-journey-info-core")
	defer span.End()
	callerName := "CreateJourneyUnitAttempt"

	// ensure that post is not Exclusive
	if parentUnitVisibility == models.ExclusiveVisibility {
		return nil, fmt.Errorf("You can't start this attempt yet. This Journey Unit is an Exclusive Unit " +
			"and must be purchased.")
	}

	// ensure that the user is premium if this is a Premium challenge
	if parentUnitVisibility == models.PremiumVisibility && callingUser.UserStatus != models.UserStatusPremium {
		return nil, fmt.Errorf("You can't start this attempt yet. This Unit is a Premium Unit and " +
			"is only accessible to Premium users. Go tou the Account Settings page to upgrade your account.")
	}

	// create variables to hold post data
	var unitTitle string
	var unitDesc string
	var unitAuthorId int64
	var unitVisibility models.PostVisibility

	// retrieve post
	err := tiDB.QueryRowContext(ctx, &span, &callerName,
		"select title, description, author_id, visibility from journey_units where _id = ? limit 1", parentUnit,
	).Scan(&unitTitle, &unitDesc, &unitAuthorId, &unitVisibility)
	if err != nil {
		return nil, fmt.Errorf("failed to query for post: %v\n    query: %s\n    params: %v", err,
			"select repo_id from journey_units where _id = ?", []interface{}{parentUnit})
	}

	// create source repo path
	repoOwner := fmt.Sprintf("%d", unitAuthorId)
	repoName := fmt.Sprintf("%d", parentUnit)

	id := sf.Generate().Int64()

	// create new journey unit
	journey, err := models.CreateJourneyUnitAttempt(id, callingUser.ID, parentUnit, title, unitFocus, languages,
		description, repoID, time.Now(), time.Now(), tags, tier, workspaceSettings, workspaceConfig, workspaceConfigRevision, estimatedTutorialTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey project: %v", err)
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
	newRepoId := fmt.Sprintf("%d", journey.ID)
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

	journey.RepoID = repo.ID

	journeyInsertion, err := journey.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey project, failed to call journey project to sql native, err: %v", err)
	}

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

	return map[string]interface{}{"message": "journey unit attempt created"}, nil

}

// todo create journey unit project attempt
func CreateJourneyProjectAttempt(ctx context.Context, tiDB *ti.Database, sf *snowflake.Node, callingUser *models.User,
	unitID int64, parentProject int64, workingDirectory string, title string, description string,
	language models.ProgrammingLanguage, tags []string, dependencies []int64,
	estimatedTutorialTime *time.Duration) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-journey-project-attempt-core")
	defer span.End()
	callerName := "CreateJourneyProjectAttempt"

	// validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" || callingUser.AuthRole != models.Admin {
		return nil, fmt.Errorf("callinguser is not an admin. CreateJourneyProjectAttempt core")
	}

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM journey_unit_attempts WHERE _id = ?)"
	err := tiDB.QueryRowContext(ctx, &span, &callerName, query, unitID).Scan(&exists)
	if err != nil {
		// Handle the error, which could be because of a variety of reasons, like no rows in result set
		return nil, fmt.Errorf("failed to check for existsing journey unit: %v", err)
	}

	if !exists {
		return map[string]interface{}{"message": "journey unit does not exist"}, nil
	}

	exists = false
	query = "SELECT EXISTS(SELECT 1 FROM journey_unit_projects WHERE _id = ?)"
	err = tiDB.QueryRowContext(ctx, &span, &callerName, query, parentProject).Scan(&exists)
	if err != nil {
		// Handle the error, which could be because of a variety of reasons, like no rows in result set
		return nil, fmt.Errorf("failed to check for existsing journey unit: %v", err)
	}

	if !exists {
		return map[string]interface{}{"message": "journey unit does not exist"}, nil
	}

	id := sf.Generate().Int64()

	// create new journey unit
	journey, err := models.CreateJourneyUnitProjectAttempts(id, unitID, parentProject, false, workingDirectory, title, description,
		language, tags, dependencies, estimatedTutorialTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create new journey project: %v", err)
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

	return map[string]interface{}{"message": "journey project attempt created"}, nil

}

// todo delete journey unit

// todo delete journey unit projects

// todo delete journey unit attempt

// todo delete journey unit project attempt
