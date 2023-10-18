package core

import (
	"context"
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core/query_models"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
)

const FollowingFeedQuery = `
select
	p._id as _id,
	title,
	description,
	author,
	p.author_id as author_id,
	p.created_at as created_at,
	updated_at,
	repo_id,
	p.tier as tier,
	top_reply,
	p.coffee as coffee,
	post_type,
	views,
	completions,
	attempts,
	user_status,
	r.name as background_name,
	r.color_palette as background_palette,
	r.render_in_front as background_render
from post p
	join follower f on f.following = p.author_id
	join users u on u._id = f.following
	left join rewards r on u.avatar_reward = r._id
where
	f.follower = ?
	and p.deleted = false
	and p.published = true
order by p.updated_at desc
limit ?
offset ?
`

func FeedPage(ctx context.Context, callingUser *models.User, tidb *ti.Database, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "feed-page-core")
	callerName := "FeedPage"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, FollowingFeedQuery, callingUser.ID, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
	}

	projects := make([]*query_models.PostUserStatusFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project query_models.PostUserStatus

		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. recommended Project Home core.    Error: %v", err)
		}

		fp, err := project.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert post to frontend object: %v", err)
		}

		projects = append(projects, fp)
	}

	return map[string]interface{}{"projects": projects}, nil
}
