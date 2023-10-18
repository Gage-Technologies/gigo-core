package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"go.opentelemetry.io/otel"
	"time"
)

func CompleteSearch(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, userID int64, searchRecModelID *int64, postID int64, query string, logger logging.Logger) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "complete-search-core")
	defer span.End()
	callerName := "CompleteSearch"

	logger.Debugf("called CompleteSearch with search rec id: %v and post id: %v", searchRecModelID, postID)
	var postName string
	err := tidb.QueryRowContext(ctx, &span, &callerName, "select title from post where _id = ?", postID).Scan(&postName)
	if err != nil {
		return fmt.Errorf("failed to query post name in Complete Search, err: %v", err)
	}
	if searchRecModelID == nil {
		searchRec := models.CreateSearchRec(sf.Generate().Int64(), userID, []int64{postID}, query, &postID, &postName, time.Now())
		statements := searchRec.ToSQLNative()
		for _, statement := range statements {
			_, err = tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return fmt.Errorf("failed to execute insert statement for search rec insertion in Complete Search, err: %v", err)
			}
		}

		return nil
	}

	_, err = tidb.ExecContext(ctx, &span, &callerName, "update search_rec set selected_post_id = ?, selected_post_name = ? where _id =?", postID, postName, *searchRecModelID)
	if err != nil {
		return fmt.Errorf("failed to update search rec in Complete Search, err: %v", err)
	}

	return nil
}
