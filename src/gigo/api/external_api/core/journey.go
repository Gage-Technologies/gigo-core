package core

import (
	"bytes"
	"context"
	"fmt"
	"gigo-core/gigo/utils"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/storage"
	utils2 "github.com/gage-technologies/gigo-lib/utils"
	"github.com/go-errors/errors"
	"github.com/go-git/go-git/v5/utils/ioutil"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
	"strings"
	"time"
)

type CreateJourneyUnitParams struct {
	Ctx           context.Context
	TiDB          *ti.Database
	Sf            *snowflake.Node
	Name          string
	UnitAbove     *int64
	UnitBelow     *int64
	Description   string
	Langs         []models.ProgrammingLanguage
	Tags          []string
	StorageEngine storage.Storage
	Meili         *search.MeiliSearchEngine
	Thumbnail     string
}

type PublishJourneyUnitParams struct {
	Ctx       context.Context
	TiDB      *ti.Database
	JourneyID int64
	Meili     *search.MeiliSearchEngine
}

type DeleteJourneyUnitParams struct {
	Ctx       context.Context
	TiDB      *ti.Database
	JourneyID int64
	Meili     *search.MeiliSearchEngine
}

type UnPublishJourneyUnitParams struct {
	Ctx       context.Context
	TiDB      *ti.Database
	JourneyID int64
	Meili     *search.MeiliSearchEngine
}

type CreateJourneyTaskParams struct {
	Ctx            context.Context
	TiDB           *ti.Database
	Sf             *snowflake.Node
	JourneyUnitID  int64
	Name           string
	NodeAbove      *int64
	NodeBelow      *int64
	Description    string
	CodeSourceType models.CodeSource
	CodeSourceID   int64
	Lang           models.ProgrammingLanguage
}

type GetJourneyUnitMetadataParams struct {
	Ctx           context.Context
	TiDB          *ti.Database
	JourneyUnitID int64
}

type GetUserJourneyTaskParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	TaskID int64
	UserID int64
}

type GetAllTasksInUnitParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	UnitID int64
	UserID int64
}

type GetAllTasksInUnitReturn struct {
	JourneyUnitID string              `json:"journey_unit_id" sql:"ju._id"`
	Tasks         []*UserTaskFrontend `json:"tasks"`
	UnitCompleted bool                `json:"unit_completed"`
	UnitAbove     *string             `json:"unit_above_id" sql:"ju.node_above"`
	UnitBelow     *string             `json:"unit_below_id" sql:"ju.node_below"`
	Langs         []string            `json:"languages"`
	Name          string              `json:"name" sql:"ju.name"`
	Description   string              `json:"description" sql:"ju.description"`
}

type GetAllJourneyUnitsParams struct {
	Ctx        context.Context
	TiDB       *ti.Database
	UserID     int64
	SearchText *string
}

type GetUserJourneyTaskReturn struct {
	TaskID      string `json:"task_id" sql:"jt._id"`
	Name        string `json:"name" sql:"jt.name"`
	Description string `json:"description" sql:"jt.description"`
	Lang        string `json:"lang" sql:"jt.lang"`
	Completed   bool   `json:"completed" sql:"completed"`
}

type PublishJourneyTaskParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	TaskID int64
}

type UnPublishJourneyTaskParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	TaskID int64
}

type DeleteJourneyTaskParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	TaskID int64
}

type CreateJourneyDetourParams struct {
	Ctx          context.Context
	TiDB         *ti.Database
	Sf           *snowflake.Node
	DetourUnitID int64
	UserID       int64
	TaskID       int64
	StartedAt    time.Time
}

type DeleteJourneyDetourParams struct {
	Ctx          context.Context
	TiDB         *ti.Database
	Sf           *snowflake.Node
	DetourUnitID int64
	UserID       int64
}

type CreateJourneyUserMapParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	UserID int64
	Units  []int64
}

type GetJourneyUserMapParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	UserID int64
}

type CreateDetourRecommendationParams struct {
	Ctx        context.Context
	TiDB       *ti.Database
	Sf         *snowflake.Node
	RecUnitID  int64
	UserID     int64
	FromTaskID int64
	CreatedAt  time.Time
}

type DeleteDetourRecommendationParams struct {
	Ctx     context.Context
	TiDB    *ti.Database
	UserID  int64
	RecUnit int64
}

type GetUserJourneyStatsParams struct {
	Ctx    context.Context
	TiDB   *ti.Database
	UserID int64
}

type UserJourneyStats struct {
	CompletedTasks  int64 `json:"completed_tasks"`
	CompletedUnits  int64 `json:"completed_units"`
	DetoursTaken    int64 `json:"detours_taken"`
	TasksLeftInUnit int64 `json:"tasks_left_in_unit"`
}

type UserTask struct {
	ID             int64             `json:"_id" sql:"jt._id"`
	Name           string            `json:"name" sql:"jt.name"`
	Description    string            `json:"description" sql:"jt.description"`
	Lang           string            `json:"lang" sql:"jt.lang"`
	JourneyUnitID  int64             `json:"journey_unit_id" sql:"jt.journey_unit_id"`
	NodeAbove      *int64            `json:"node_above" sql:"jt.node_above"`
	NodeBelow      *int64            `json:"node_below" sql:"jt.node_below"`
	CodeSourceID   int64             `json:"code_source_id" sql:"jt.code_source_id"`
	CodeSourceType models.CodeSource `json:"code_source_type" sql:"jt.code_source_type"`
	Completed      *bool             `json:"completed" sql:"completed"`
}

func (u *UserTask) ToFrontend() *UserTaskFrontend {

	var nodeBelow *string

	if u.NodeBelow != nil {
		belowStr := fmt.Sprintf("%v", *u.NodeBelow)
		nodeBelow = &belowStr
	}

	var nodeAbove *string

	if u.NodeAbove != nil {
		aboveStr := fmt.Sprintf("%v", *u.NodeAbove)
		nodeAbove = &aboveStr
	}

	return &UserTaskFrontend{
		ID:             fmt.Sprintf("%v", u.ID),
		Name:           fmt.Sprintf("%v", u.Name),
		Description:    fmt.Sprintf("%v", u.Description),
		Lang:           fmt.Sprintf("%v", u.Lang),
		JourneyUnitID:  fmt.Sprintf("%v", u.JourneyUnitID),
		NodeAbove:      nodeAbove,
		NodeBelow:      nodeBelow,
		CodeSourceID:   fmt.Sprintf("%v", u.CodeSourceID),
		CodeSourceType: int(u.CodeSourceType),
		Completed:      u.Completed,
	}
}

type UserTaskFrontend struct {
	ID             string  `json:"_id" sql:"jt._id"`
	Name           string  `json:"name" sql:"jt.name"`
	Description    string  `json:"description" sql:"jt.description"`
	Lang           string  `json:"lang" sql:"jt.lang"`
	JourneyUnitID  string  `json:"journey_unit_id" sql:"jt.journey_unit_id"`
	NodeAbove      *string `json:"node_above" sql:"jt.node_above"`
	NodeBelow      *string `json:"node_below" sql:"jt.node_below"`
	CodeSourceID   string  `json:"code_source_id" sql:"jt.code_source_id"`
	CodeSourceType int     `json:"code_source_type" sql:"jt.code_source_type"`
	Completed      *bool   `json:"completed" sql:"completed"`
}

type UpdateJourneyUnitTreeParams struct {
	Ctx       context.Context
	TiDB      *ti.Database
	UnitID    int64
	UnitAbove *int64
	UnitBelow *int64
}

type UpdateJourneyTaskUnitTreeParams struct {
	Ctx       context.Context
	TiDB      *ti.Database
	TaskID    int64
	TaskAbove *int64
	TaskBelow *int64
}

func CreateJourneyUnit(params CreateJourneyUnitParams) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-unit-core")
	defer span.End()
	callerName := "CreateJourneyUnit"

	// create a new id for the post
	id := params.Sf.Generate().Int64()

	// get temp thumbnail file from storage
	thumbnailTempFile, _, err := params.StorageEngine.GetFile(params.Thumbnail)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	}
	defer thumbnailTempFile.Close()

	// sanitize thumbnail image
	thumbnailBuffer := bytes.NewBuffer([]byte{})
	color, err := utils.PrepImageFile(thumbnailTempFile, ioutil.WriteNopCloser(thumbnailBuffer), true, true)
	if err != nil {
		return nil, fmt.Errorf("failed to prep thumbnail file: %v", err)
	}

	// record failure state to cleanup on exit
	failed := true

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}
	}()

	// create a new journey unit
	journeyUnit, err := models.CreateJourneyUnit(
		id,
		params.Name,
		params.UnitAbove,
		params.UnitBelow,
		params.Description,
		params.Langs,
		params.Tags,
		false,
		color,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte: %v", err)
	}

	// format the post into sql insert statements
	statements, err := journeyUnit.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte into insert statements: %v", err)
	}

	// create transaction for byte insertion
	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for byte: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash journeyUnit id: %v", err)
	}
	err = params.StorageEngine.CreateFile(
		fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash),
		thumbnailBuffer.Bytes(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	// format byte to frontend object
	fp := journeyUnit.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte to frontend object: %v", err)
	}

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit")
	}

	// set success flag
	failed = false

	return map[string]interface{}{"message": "Byte created successfully.", "byte": fp}, nil

}

func GetJourneyUnitMetadata(params GetJourneyUnitMetadataParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "publish-journey-unit-core")
	defer span.End()
	callerName := "PublishJourneyUnit"

	res, err := params.TiDB.QueryContext(ctx, &span, &callerName, "select * from journey_units where published = 1 and _id = ?", params.JourneyUnitID)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to query for journey units by id: %v, err: %v", params.JourneyUnitID, err))
	}

	defer res.Close()

	for res.Next() {
		unit, err := models.JourneyUnitFromSQLNative(ctx, &span, params.TiDB, res)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to marhsal unit with from sql native, err : %v", err))
		}

		return map[string]interface{}{"success": true, "unit": unit.ToFrontend()}, nil

	}

	return nil, errors.New(fmt.Sprintf("failed to find journey unit with id: %v, err: %v", params.JourneyUnitID, err))

}

func PublishJourneyUnit(params PublishJourneyUnitParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "publish-journey-unit-core")
	defer span.End()
	callerName := "PublishJourneyUnit"

	failed := true

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = params.Meili.DeleteDocuments("journey_units", params.JourneyID)
	}()

	res, err := params.TiDB.ExecContext(ctx, &span, &callerName, "update journey_units set published = 1 where _id = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to execute joueny_unit update, err: %v", err))
	}

	numChanged, err := res.RowsAffected()
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to get rows affected, err: %v", err))
	}

	if numChanged != 1 {
		failed = true
		return nil, errors.New(fmt.Sprintf("expected one row changed but got %d", numChanged))
	}

	err = params.Meili.AddDocuments("journey_units", map[string]interface{}{"_id": params.JourneyID, "published": true})
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to add journey unit: %v to search, err: %v", params.JourneyID, err))
	}

	failed = false

	return map[string]interface{}{"success": true}, nil

}

func UnPublishJourneyUnit(params UnPublishJourneyUnitParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "un-publish-journey-unit-core")
	defer span.End()
	callerName := "UnPublishJourneyUnit"

	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction, err: %v", err))
	}

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "update journey_units set published = 0 where _id = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to delete journey unit with ID '%d', err: %v", params.JourneyID, err))
	}

	numChanged, err := res.RowsAffected()
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to retrieve number of changed rows after deleting journey unit with ID '%d', err: %v", params.JourneyID, err))
	}

	if numChanged != 1 {
		failed = true
		return nil, errors.New(fmt.Sprintf("expected one row changed but got %d", numChanged))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit delete journey unit: %v tx: %v", params.JourneyID, err))
	}

	_ = params.Meili.DeleteDocuments("journey_units", params.JourneyID)

	failed = false

	return map[string]interface{}{"success": true}, nil
}

func DeleteJourneyUnit(params DeleteJourneyUnitParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "delete-journey-unit-core")
	defer span.End()
	callerName := "DeleteJourneyUnit"

	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction, err: %v", err))
	}

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "delete from journey_units where _id = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to delete journey unit: %v, err: %v", params.JourneyID, err))
	}

	numChanged, err := res.RowsAffected()
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to retrieve number of rows affect when deleting journey unit: %v, err: %v", params.JourneyID, err))
	}

	if numChanged != 1 {
		failed = true
		return nil, errors.New(fmt.Sprintf("incorrect number of rows affected by delet journey unit: %v, err: %v", params.JourneyID, err))
	}

	_, err = tx.ExecContext(ctx, &callerName, "delete from journey_detour where detour_unit_id = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to delete journey detour with unit id: %v, err: %v", params.JourneyID, err))
	}

	_, err = tx.ExecContext(ctx, &callerName, "delete from journey_detour_recommendation where recommended_unit = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to delete joruney detour recommendation with unit id: %v, err: %v", params.JourneyID, err))
	}

	_, err = tx.ExecContext(ctx, &callerName, "delete from journey_user_map where unit_id = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to delete journey user map with unit id: %v, err: %v", params.JourneyID, err))
	}

	_, err = tx.ExecContext(ctx, &callerName, "delete from journey_tasks where journey_unit_id = ?", params.JourneyID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to delete journey tasks with unit id: %v, err: %v", params.JourneyID, err))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to commit delete journey unit tx: %v", err))
	}

	_ = params.Meili.DeleteDocuments("journey_units", params.JourneyID)

	failed = false

	return map[string]interface{}{"success": true}, nil

}

func CreateJourneyTask(params CreateJourneyTaskParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-task-core")
	defer span.End()
	callerName := "CreateJourneyTask"

	// create a new id for the post
	id := params.Sf.Generate().Int64()

	// record failure state to cleanup on exit
	failed := true

	// create a new journey unit
	journeyTask, err := models.CreateJourneyTask(
		id,
		params.Name,
		params.Description,
		params.JourneyUnitID,
		params.NodeAbove,
		params.NodeBelow,
		params.CodeSourceID,
		params.CodeSourceType,
		params.Lang,
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte: %v", err)
	}

	// format the post into sql insert statements
	statements, err := journeyTask.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte into insert statements: %v", err)
	}

	// create transaction for byte insertion
	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, &callerName, statements.Statement, statements.Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to perform insertion statement for byte: %v\n    statement: %s\n    params: %v", err, statements.Statement, statements.Values)
	}

	// format byte to frontend object
	fp := journeyTask.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte to frontend object: %v", err)
	}

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit")
	}

	// set success flag
	failed = false

	return map[string]interface{}{"message": "Journey Task created successfully.", "journey_task": fp}, nil
}

func GetUserJourneyTask(params GetUserJourneyTaskParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "get-journey-task-core")
	defer span.End()
	callerName := "GetJourneyTask"

	resType := make([]*GetUserJourneyTaskReturn, 0)

	query := `select jt._id, jt.name, jt.description, jt.lang, completed, 
				    CASE
						WHEN ba.complete_easy = 1 OR ba.completed_medium = 1 OR ba.completed_hard = 1 THEN 1
						ELSE 0
					END AS completed
 					from journey_tasks INNER JOIN bytes b ON jt.code_source_id = b._id 
					LEFT JOIN byte_attempts ba ON ba.byte_id = b._id 
 					WHERE jt.published IS TRUE and ba.author_id = ? and jt._id = ? limit 1`

	res, err := params.TiDB.QueryContext(ctx, &span, &callerName, query, params.UserID, params.TaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user journey task with query: %v: %v", query, err)
	}

	defer res.Close()

	for res.Next() {
		var r GetUserJourneyTaskReturn
		err = sqlstruct.Scan(&r, res)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		resType = append(resType, &r)
	}

	if resType == nil {
		return nil, errors.New("No tasks were returned.")
	}

	return map[string]interface{}{"success": true, "task": resType}, nil

}

func PublishJourneyTask(params PublishJourneyTaskParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "publish-journey-task-core")
	defer span.End()
	callerName := "PublishJourneyTask"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "update journey_tasks set published = 1 where _id = ?", params.TaskID)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to publish task: %v, : %v", params.TaskID, err))
	}

	numChanges, err := res.RowsAffected()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get number of changes from result: %v", err))
	}

	if numChanges != 1 {
		return nil, errors.New(fmt.Sprintf("expected one change but got %v instead", numChanges))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to commit transaction: %v", err))
	}

	failed = false

	return map[string]interface{}{"message": "Journey Task updated successfully."}, nil

}

func UnPublishJourneyTask(params UnPublishJourneyTaskParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "un-publish-journey-task-core")
	defer span.End()
	callerName := "UnPublishJourneyTask"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "update journey_tasks set published = 0 where _id = ?", params.TaskID)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to un publish task: %v, : %v", params.TaskID, err))
	}

	numChanges, err := res.RowsAffected()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get number of changes from result: %v", err))
	}

	if numChanges != 1 {
		return nil, errors.New(fmt.Sprintf("expected one change but got %v instead", numChanges))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to commit transaction: %v", err))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false

	return map[string]interface{}{"message": "Journey Task updated successfully."}, nil
}

func DeleteJourneyTask(params DeleteJourneyTaskParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "delete-journey-task-core")
	defer span.End()
	callerName := "DeleteJourneyTask"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "delete from journey_tasks where _id = ?", params.TaskID)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to un publish task: %v, : %v", params.TaskID, err))
	}

	numChanges, err := res.RowsAffected()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get number of changes from result: %v", err))
	}

	if numChanges != 1 {
		return nil, errors.New(fmt.Sprintf("expected one change but got %v instead", numChanges))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false

	return map[string]interface{}{"message": "Journey Task deleted successfully."}, nil
}

func CreateJourneyDetour(params CreateJourneyDetourParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-detour-core")
	defer span.End()
	callerName := "CreateJourneyDetour"

	// record failure state to cleanup on exit
	failed := true

	// create a new journey unit
	journeyTask, err := models.CreateJourneyDetour(
		params.DetourUnitID,
		params.UserID,
		params.TaskID,
		params.StartedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte: %v", err)
	}

	// format the post into sql insert statements
	statements, err := journeyTask.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte into insert statements: %v", err)
	}

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	for _, s := range statements {
		_, err := tx.ExecContext(ctx, &callerName, s.Statement, s.Values...)
		if err != nil {
			failed = true
			return nil, errors.New(fmt.Sprintf("failed to execute statement: %v, err: %v", s.Statement, err))
		}

	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false
	return map[string]interface{}{"success": true}, err
}

func DeleteJourneyDetour(params DeleteJourneyDetourParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-detour-core")
	defer span.End()
	callerName := "CreateJourneyDetour"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "delete from journey_detour where detour_unit_id = ? and user_id = ?", params.DetourUnitID, params.UserID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to execute delet for journey_detour: %v, err: %v", params.DetourUnitID, err))
	}

	numChanged, err := res.RowsAffected()
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to retrieve number of rows chnaged in delete operation, err: %v", err))
	}

	if numChanged != 1 {
		failed = true
		return nil, errors.New(fmt.Sprintf("inccorrect number of rows affeceteed by delete, expected 1 got: %v", numChanged))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false
	return map[string]interface{}{"success": true}, err

}

func CreateJourneyUserMap(params CreateJourneyUserMapParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-user-map-core")
	defer span.End()
	callerName := "CreateJourneyUserMap"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	paramSlots := make([]string, 0)
	unitIDs := make([]interface{}, 0)

	for _, u := range params.Units {
		unitIDs = append(unitIDs, u)
		paramSlots = append(paramSlots, "?")
	}

	res, err := tx.QueryContext(ctx, &callerName, fmt.Sprintf("select * from journey_units where _id in (%s)", strings.Join(paramSlots, ",")), unitIDs...)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to query: %v for journey units in user map, err: %v", fmt.Sprintf("select * from journey_units where _id in (%s)", strings.Join(paramSlots, ",")), err))
	}

	defer res.Close()

	fullUnits := make([]models.JourneyUnit, 0)

	for res.Next() {
		j, err := models.JourneyUnitFromSQLNative(ctx, &span, params.TiDB, res)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to marhsal journey unit with from sql in user map, err: %v", err))
		}

		if j == nil {
			return nil, errors.New(fmt.Sprintf("failed to marhsal journey unit with from sql in user map, err: nmo journey nit return from function"))
		}

		fullUnits = append(fullUnits, *j)
	}

	// create a new journey unit
	journeyMap, err := models.CreateJourneyUserMap(
		params.UserID,
		fullUnits,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create journeyUserMap: %v", err)
	}

	// format the post into sql insert statements
	statements, err := journeyMap.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format user map into insert statements: %v", err)
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	for _, s := range statements {
		_, err := tx.ExecContext(ctx, &callerName, s.Statement, s.Values...)
		if err != nil {
			failed = true
			return nil, errors.New(fmt.Sprintf("failed to execute statement: %v, err: %v", s.Statement, err))
		}

	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false
	return map[string]interface{}{"success": true}, err
}

func GetJourneyUserMap(params GetJourneyUserMapParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-user-map-core")
	defer span.End()
	callerName := "GetJourneyUserMap"

	res, err := params.TiDB.QueryContext(ctx, &span, &callerName, `SELECT * from journey_user_map where user_id = ? order by started_at`, params.UserID)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to query for journey user map, err: %v", err))
	}

	final, err := models.JourneyUserMapFromSQLNative(ctx, &span, params.TiDB, res)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to call user map from sql native, err: %v", err))
	}

	if final == nil || len(final.Units) < 1 {
		return nil, errors.New(fmt.Sprintf("no units returned from user map with userID: %v", params.UserID))
	}

	return map[string]interface{}{"success": true, "user_map": final.ToFrontend()}, nil
}

func CreateJourneyDetourRecommendation(params CreateDetourRecommendationParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-journey-detour-recommendation-core")
	defer span.End()
	callerName := "CreateJourneyDetourRecommendation"

	// record failure state to cleanup on exit
	failed := true

	id := params.Sf.Generate().Int64()

	// create a new journey unit
	journeyMap, err := models.CreateJourneyDetourRecommendation(
		id,
		params.UserID,
		params.RecUnitID,
		params.FromTaskID,
		false,
		params.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create journeyUserMap: %v", err)
	}

	// format the post into sql insert statements
	statements, err := journeyMap.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format user map into insert statements: %v", err)
	}

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	for _, s := range statements {
		_, err := tx.ExecContext(ctx, &callerName, s.Statement, s.Values...)
		if err != nil {
			failed = true
			return nil, errors.New(fmt.Sprintf("failed to execute statement: %v, err: %v", s.Statement, err))
		}

	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false
	return map[string]interface{}{"success": true}, err
}

func DeleteJourneyDetourRecommendation(params DeleteDetourRecommendationParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "delete-journey-detour-rec-core")
	defer span.End()
	callerName := "DeleteJourneyDetourRecommendation"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.ExecContext(ctx, &callerName, "delete from journey_detour_recommendation where recommended_unit = ? and user_id = ?", params.RecUnit, params.UserID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to execute delete for journey_detour_rec: %v, err: %v", params.RecUnit, err))
	}

	numChanged, err := res.RowsAffected()
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to retrieve number of rows chnaged in delete operation, err: %v", err))
	}

	if numChanged != 1 {
		failed = true
		return nil, errors.New(fmt.Sprintf("inccorrect number of rows affeceteed by delete, expected 1 got: %v", numChanged))
	}

	err = tx.Commit(&callerName)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	failed = false
	return map[string]interface{}{"success": true}, err
}

func GetAllTasksInUnit(params GetAllTasksInUnitParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "get-all-tasks-core")
	defer span.End()
	callerName := "GetAllTasksInUnit"

	// record failure state to cleanup on exit
	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	query := `select * from journey_units where _id = ? and published = 1`

	res, err := tx.QueryContext(ctx, &callerName, query, params.UnitID)
	if err != nil {
		failed = true
		return nil, fmt.Errorf("failed to query user journey task with query: %v: %v", query, err)
	}

	defer res.Close()

	if !res.Next() {
		failed = false
		return nil, errors.New(fmt.Sprintf("no Journey Unit found"))
	}

	journeyUnit, err := models.JourneyUnitFromSQLNative(ctx, &span, params.TiDB, res)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to decode query for results \n Error: %v", err))
	}

	finalReturn := GetAllTasksInUnitReturn{
		JourneyUnitID: fmt.Sprintf("%v", journeyUnit.ID),
		Name:          journeyUnit.Name,
		Description:   journeyUnit.Description,
	}

	if journeyUnit.UnitBelow != nil {
		belowStr := fmt.Sprintf("%v", *journeyUnit.UnitBelow)
		finalReturn.UnitBelow = &belowStr
	}

	if journeyUnit.UnitAbove != nil {
		aboveStr := fmt.Sprintf("%v", *journeyUnit.UnitAbove)
		finalReturn.UnitAbove = &aboveStr
	}

	for _, l := range journeyUnit.Langs {
		finalReturn.Langs = append(finalReturn.Langs, l.String())
	}

	res.Close()

	query = `SELECT jt._id, jt.name, jt.description, jt.lang, jt.journey_unit_id, jt.code_source_id, jt.code_source_type, jt.node_above, jt.node_below,
				CASE
					WHEN ba.completed_easy = 1 OR ba.completed_medium = 1 OR ba.completed_hard = 1 THEN 1
					ELSE 0
				END AS completed
			FROM journey_tasks AS jt
			INNER JOIN bytes AS b ON jt.code_source_id = b._id 
			LEFT JOIN byte_attempts AS ba ON ba.byte_id = b._id AND ba.author_id = ?
			WHERE jt.published = TRUE AND jt.journey_unit_id = ?
			`

	res, err = tx.QueryContext(ctx, &callerName, query, params.UserID, journeyUnit.ID)
	if err != nil {
		failed = true
		return nil, errors.New(fmt.Sprintf("failed to query for tasks in journet unit: %v, err: %v", params.UnitID, err))
	}

	defer res.Close()

	for res.Next() {
		var id int64
		var name, desc string
		var lang int
		var junit int64
		var nodeAboveInt, nodeBelowInt *int64 // Use pointers to int64 for nullable columns
		var completedBool int                 // Use an int to capture the boolean value, then convert it to *bool later
		var codeSourceID int64
		var codeSourceType int
		var completed *bool

		// Initialize pointers for nullable fields
		nodeAbove := new(int64) // Initialize nodeAbove as a pointer to int64
		nodeBelow := new(int64) // Initialize nodeBelow as a pointer to int64

		// Scan the result into variables, including the newly initialized pointers
		err = res.Scan(&id, &name, &desc, &lang, &junit, &codeSourceID, &codeSourceType, &nodeAboveInt, &nodeBelowInt, &completedBool)
		if err != nil {
			failed = true
			return nil, errors.New(fmt.Sprintf("failed to decode query for user task results \n Error: %v", err))
		}

		// Convert completedBool to *bool
		if completedBool == 1 {
			completed = new(bool)
			*completed = true
		} else {
			completed = new(bool)
			*completed = false
		}

		// Check if nodeAboveInt or nodeBelowInt are nil and handle accordingly
		if nodeAboveInt == nil {
			nodeAbove = nil // Ensure nodeAbove is nil if the database returned NULL
		} else {
			nodeAbove = nodeAboveInt
		}

		if nodeBelowInt == nil {
			nodeBelow = nil // Ensure nodeBelow is nil if the database returned NULL
		} else {
			nodeBelow = nodeBelowInt
		}

		userTask := UserTask{
			ID:             id,
			Name:           name,
			Description:    desc,
			Lang:           models.ProgrammingLanguage(lang).String(),
			JourneyUnitID:  junit,
			NodeAbove:      nodeAbove,
			NodeBelow:      nodeBelow,
			CodeSourceID:   codeSourceID,
			CodeSourceType: models.CodeSource(codeSourceType),
			Completed:      completed,
		}

		finalReturn.Tasks = append(finalReturn.Tasks, userTask.ToFrontend())
	}

	if finalReturn.Tasks == nil {
		return nil, errors.New(fmt.Sprintf("user tasks returned nil for unit: %v", params.UnitID))
	}

	if finalReturn.Tasks[0].ID == "0" {
		return nil, errors.New(fmt.Sprintf("user tasks returned nil for unit: %v", params.UnitID))
	}

	finalReturn.UnitCompleted = true

	for _, t := range finalReturn.Tasks {
		if t.Completed == nil || !*t.Completed {
			finalReturn.UnitCompleted = false
		}
	}

	err = tx.Commit(&callerName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to close transaction, err: %v", err))
	}

	failed = false

	return map[string]interface{}{"success": true, "data": finalReturn}, nil

}

func GetAllJourneyUnits(params GetAllJourneyUnitsParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "get-all-journey-units-core")
	defer span.End()
	callerName := "GetAllJourneyUnits"

	failed := true

	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to create transaction to query for journey units, err: %v", err))
	}

	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = tx.Rollback()
	}()

	res, err := tx.QueryContext(ctx, &callerName, "SELECT unit_id from journey_user_map where user_id = ?", params.UserID)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get all users units, err: %v", err))
	}

	defer res.Close()

	units := make([]interface{}, 0)
	qParams := make([]string, 0)

	for res.Next() {
		var unit int64
		err := res.Scan(&unit)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to scan row into variable, err: %v", err))
		}
		units = append(units, unit)
		qParams = append(qParams, "?")
	}

	err = res.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to close result set, err: %v", err))
	}

	query := "SELECT * from journey_units where published = 1"

	if params.SearchText != nil {
		query += " and name like '%" + *params.SearchText + "%'"
	}

	if len(units) > 0 {
		query += " and _id not in (" + strings.Join(qParams, ",") + ")"
		res, err = tx.QueryContext(ctx, &callerName, query, units...)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to query database for journey units: %v, err: %v", query, err))
		}
	} else {
		res, err = tx.QueryContext(ctx, &callerName, query)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to query database for journey units: %v, err: %v", query, err))
		}
	}

	defer res.Close()

	unitsFrontend := make([]*models.JourneyUnitFrontend, 0)

	for res.Next() {
		unit, err := models.JourneyUnitFromSQLNative(ctx, &span, params.TiDB, res)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to scan row into variable, err: %v", err))
		}

		if unit == nil {
			return nil, errors.New(fmt.Sprintf("failed to scan row into variable, err: no unit returned from scan"))
		}

		unitsFrontend = append(unitsFrontend, unit.ToFrontend())
	}

	if unitsFrontend == nil || len(unitsFrontend) < 1 {
		return map[string]interface{}{"success": true, "units": unitsFrontend}, nil
	}

	failed = false
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to commit transaction, err: %v", err))
	}

	return map[string]interface{}{"success": true, "units": unitsFrontend}, nil
}

func UpdateJourneyUnitTree(params UpdateJourneyUnitTreeParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "get-all-tasks-core")
	defer span.End()
	callerName := "UpdateJourneyUnitTree"

	paramsQ := make([]string, 0)
	val := make([]interface{}, 0)

	if params.UnitAbove != nil {
		paramsQ = append(paramsQ, " unit_above = ?")
		val = append(val, *params.UnitAbove)
	}

	if params.UnitBelow != nil {
		paramsQ = append(paramsQ, " unit_below = ?")
		val = append(val, *params.UnitBelow)
	}

	val = append(val, params.UnitID)

	res, err := params.TiDB.ExecContext(ctx, &span, &callerName, "update journey_units set"+strings.Join(paramsQ, ",")+" where _id = ?", val...)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to execute journey unit update for above/below: %v, err: %v", "update journey_units set"+strings.Join(paramsQ, ",")+" where _id = ?", err))
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to retrieve number of rows affected by unit update, err: %v", err))
	}

	if rows != 1 {
		return nil, errors.New(fmt.Sprintf("incorrect number of rows updated for journey_units, rows: %v", rows))
	}

	return map[string]interface{}{"success": true}, nil
}

func UpdateJourneyTaskTree(params UpdateJourneyTaskUnitTreeParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "get-all-tasks-core")
	defer span.End()
	callerName := "UpdateJourneyTaskTree"

	paramsQ := make([]string, 0)
	val := make([]interface{}, 0)

	if params.TaskAbove != nil {
		paramsQ = append(paramsQ, " node_above = ?")
		val = append(val, *params.TaskAbove)
	}

	if params.TaskBelow != nil {
		paramsQ = append(paramsQ, " node_below = ?")
		val = append(val, *params.TaskBelow)
	}

	val = append(val, params.TaskID)

	res, err := params.TiDB.ExecContext(ctx, &span, &callerName, "update journey_tasks set"+strings.Join(paramsQ, ",")+" where _id = ?", val...)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to execute journey tasks update for above/below: %v, err: %v", "update journey_tasks set"+strings.Join(paramsQ, ",")+" where _id = ?", err))
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to retrieve number of rows affected by unit update, err: %v", err))
	}

	if rows != 1 {
		return nil, errors.New(fmt.Sprintf("incorrect number of rows updated for journey_tasks, rows: %v", rows))
	}

	return map[string]interface{}{"success": true}, nil
}

// func GetUserJourneyStats(params GetUserJourneyStatsParams) (map[string]interface{}, error) {
//	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "get-user-journey-stats-core")
//	defer span.End()
//	callerName := "GetUserJourneyStats"
//
//	var final UserJourneyStats
//
//	// record failure state to cleanup on exit
//	failed := true
//
//	tx, err := params.TiDB.BeginTx(ctx, &span, &callerName, nil)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("failed to create transaction: %v", err))
//	}
//
//	defer func() {
//		// skip cleanup if we succeeded
//		if !failed {
//			return
//		}
//
//		_ = tx.Rollback()
//	}()
//
//	// select completed tasks
//	res, err := tx.QueryContext(ctx, &callerName, "select * from journey_user_map where user_id = ? order by started_at", params.UserID)
//	if err != nil {
//		failed = true
//		return nil, errors.New(fmt.Sprintf("failed to query for journey user map, err: %v", err))
//	}
//
//	defer res.Close()
//
//	userMap, err := models.JourneyUserMapFromSQLNative(ctx, &span, params.TiDB, res)
//	if err != nil {
//		failed = true
//		return nil, errors.New(fmt.Sprintf("failed journey user map from sql native, err: %v", err))
//	}
//
//	if userMap == nil {
//		failed = true
//		return nil, errors.New("failed to get userMap, err: userMap is nil")
//	}
//
//	//qParams := make([]string, 0)
//	//unitIds := make([]interface{}, 0)
//
//	//query := "SELECT jt._id, " +
//	//	"jt.journey_unit_id, " +
//	//	"jt.node_above, " +
//	//	"jt.node_below, " +
//	//	"completed, " +
//	//	"CASE " +
//	//	"WHEN ba.complete_easy = 1 OR ba.completed_medium = 1 OR ba.completed_hard = 1 THEN 1 " +
//	//	"ELSE 0 " +
//	//	"END AS completed " +
//	//	"FROM journey_tasks jt " +
//	//	"INNER JOIN bytes b ON jt.code_source_id = b._id " +
//	//	"LEFT JOIN byte_attempts ba ON b._id = ba.byte_id " +
//	//	"WHERE jt.published = 1 " +
//	//	"AND jt.code_source_type = 2 " +
//	//	"AND jt.journey_unit_id IN (" + strings.Join(qParams, ",") + ")"
//
//	for _, uId := range userMap.Units {
//		//unitIds = append(unitIds, uId.ID)
//		//qParams = append(qParams, "?")
//
//		taskRes := make([]UserTask, 0)
//
//		query := `
//			SELECT
//			    jt._id,
//				jt.journey_unit_id,
//				jt.node_above,
//				jt.node_below,
//				completed,
//				CASE
//				WHEN ba.complete_easy = 1 OR ba.completed_medium = 1 OR ba.completed_hard = 1 THEN 1
//				ELSE 0
//				END AS completed
//			FROM journey_tasks jt
//				INNER JOIN bytes b ON jt.code_source_id = b._id
//				LEFT JOIN byte_attempts ba ON b._id = ba.byte_id
//			WHERE
//			    jt.published = 1
//				AND jt.code_source_type = 2
//				AND jt.journey_unit_id = ?
//		`
//
//		res, err = tx.QueryContext(ctx, &callerName, query, uId)
//		if err != nil {
//			failed = true
//			return nil, errors.New(fmt.Sprintf("failed to query for journey tasks with query: %v, err :%v", query, err))
//		}
//
//		for res.Next() {
//			var task UserTask
//			err = sqlstruct.Scan(&task, res)
//			if err != nil {
//				failed = true
//				return nil, errors.New(fmt.Sprintf("failed to scan for journey tasks, err :%v", err))
//			}
//
//			taskRes = append(taskRes, task)
//		}
//
//		totalTaskInUnit := 0
//		for _, v := range taskRes {
//			totalTaskInUnit++
//			if v.Completed != nil && *v.Completed {
//				final.CompletedTasks++
//			}
//		}
//
//	}
//
// }
