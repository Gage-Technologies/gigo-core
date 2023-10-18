package core

import (
	"context"
	"fmt"
	"gigo-core/gigo/api/external_api/core/query_models"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
)

const queryPastWeekActive = `
select 
    _id as post_id, 
    title as post_title, 
    description, 
    tier, 
    coffee, 
    updated_at, 
    -1 as _id,
    post_type
from post 
where 
    author_id = ? and 
    updated_at > ? 
union 
select 
    post_id, 
    post_title, 
    description, 
    tier, 
    coffee, 
    updated_at, 
    _id,
    post_type
from attempt 
where 
    author_id = ? and 
    closed is false and 
    updated_at > ? 
order by updated_at desc 
limit ? offset ?
`

const queryMostChallengingActive = `
select 
    * 
from attempt 
where 
    author_id = ? and 
    closed is false 
order by 
    tier desc, 
    updated_at desc 
limit ? offset ?
`

const queryDontGiveUpActive = `
select 
    _id as post_id,
    title as post_title,
    description,
    tier,
    coffee,
    updated_at,
    -1 as _id,
    post_type
from post 
where 
    author_id = ? and 
    updated_at < ?
union 
select 
    post_id,
    post_title,
    description,
    tier,
    coffee,
    updated_at,
    _id,
    post_type
from attempt 
where 
    author_id = ? and 
    closed = false and 
    updated_at < ? 
order by updated_at desc 
limit ? offset ?
`

func PastWeekActive(ctx context.Context, callingUser *models.User, tidb *ti.Database, skip int, limit int) (map[string]interface{}, error) {
	weekEarlier := time.Now().AddDate(0, 0, -7)

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "past-week-active-core")
	callerName := "PastWeekActive"
	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, queryPastWeekActive, callingUser.ID, weekEarlier, callingUser.ID, weekEarlier, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
	}

	projects := make([]query_models.AttemptPostMergeFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project query_models.AttemptPostMerge

		err = res.Scan(&project.PostId, &project.PostTitle, &project.Description, &project.Tier, &project.Coffee, &project.UpdatedAt, &project.ID, &project.PostType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post from cursor: %v", err)
		}

		project.Thumbnail = fmt.Sprintf("/static/posts/t/%v", project.PostId)

		projects = append(projects, project.ToFrontend())
	}

	return map[string]interface{}{"projects": projects}, nil
}

func MostChallengingActive(ctx context.Context, callingUser *models.User, tidb *ti.Database, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "most-challenging-active-http")
	callerName := "MostChallengingActive"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, queryMostChallengingActive, callingUser.ID, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
	}

	projects := make([]*models.AttemptFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project models.Attempt

		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Active Project Home core.    Error: %v", err)
		}

		projects = append(projects, project.ToFrontend())
	}

	return map[string]interface{}{"projects": projects}, nil
}

func DontGiveUpActive(ctx context.Context, callingUser *models.User, tidb *ti.Database, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "cont-give-up-active-core")
	callerName := "DontGiveUpActive"

	weekEarlier := time.Now().AddDate(0, 0, -7)

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, queryDontGiveUpActive, callingUser.ID, weekEarlier, callingUser.ID, weekEarlier, limit, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Active Project Home core.    Error: %v", err)
	}

	projects := make([]query_models.AttemptPostMergeFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project query_models.AttemptPostMerge

		err = res.Scan(&project.PostId, &project.PostTitle, &project.Description, &project.Tier, &project.Coffee, &project.UpdatedAt, &project.ID, &project.PostType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post from cursor: %v", err)
		}

		if project.ID != -1 {
			project.Thumbnail = fmt.Sprintf("/static/posts/t/%v", project.ID)
		} else {
			project.Thumbnail = fmt.Sprintf("/static/posts/t/%v", project.PostId)
		}

		projects = append(projects, project.ToFrontend())
	}

	return map[string]interface{}{"projects": projects}, nil
}
