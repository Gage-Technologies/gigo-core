package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
)

func CreateReportIssue(ctx context.Context, userId int64, db *ti.Database, page string, issue string, sf *snowflake.Node) (map[string]interface{}, error) {
	// create a new discussion
	issueReport, err := models.CreateReportIssue(userId, page, issue, sf.Generate().Int64())
	if err != nil {
		return nil, fmt.Errorf("failed to create new discussion struct: %v", err)
	}

	statement := issueReport.ToSQLNative()

	_, err = db.DB.Exec(statement.Statement, statement.Values...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new discussion struct: %v", err)
	}

	return map[string]interface{}{"message": "Thank you for your feedback!"}, nil
}
