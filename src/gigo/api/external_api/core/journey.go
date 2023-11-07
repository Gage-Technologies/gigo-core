package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"go.opentelemetry.io/otel"
)

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
