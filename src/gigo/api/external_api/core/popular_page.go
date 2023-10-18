package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"go.opentelemetry.io/otel"
)

func PopularPageFeed(ctx context.Context, skip int, limit int, tidb *ti.Database) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "popular-page-feed-core")
	callerName := "PopularPageFeed"

	res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from post order by coffee desc, attempts desc limit ? offset ?", limit, skip)
	if err != nil {
		return map[string]interface{}{"feed": "There was an issue querying for feed"}, err
	}

	// create slice to hold posts
	posts := make([]*models.PostFrontend, 0)

	defer res.Close()

	// iterate through the result rows
	for res.Next() {
		// decode row results
		post, err := models.PostFromSQLNative(tidb, res)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %v", err)
		}

		// format the post to its frontend value
		fp, err := post.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert post to frontend object: %v", err)
		}

		posts = append(posts, fp)
	}

	return map[string]interface{}{"feed": posts}, nil
}
