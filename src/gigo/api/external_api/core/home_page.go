package core

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gage-technologies/gigo-lib/storage"
	"sort"
	"strings"

	"github.com/gage-technologies/gigo-lib/logging"
	"go.opentelemetry.io/otel"

	"gigo-core/gigo/api/external_api/core/query_models"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
)

type ReccommendedProjectsHomeRequest struct {
	Skip int  `json:"skip" validate:"gte=0"`
	Test bool `json:"test"`
}

const activeProjectsHomeQuery = `
select 
	a.* 
from attempt a 
	left join post p on a.post_id = p._id 
where 
	a.author_id = ?
order by a.updated_at desc 
limit 32
`

const recommendedProjectsHomeQuery = `
select 
	p._id as _id, 
	rp._id as recommended_id, 
	p.created_at as created_at, 
	p.updated_at as updated_at, 
	rp.score as similarity, 
	p.title as title, 
	p.author as author, 
	p.author_id as author_id, 
	p.repo_id as repo_id,
	p.challenge_cost as challenge_cost,
	p.tier as tier, 
	p.coffee as coffee,
	p.post_type as post_type, 
	p.views as views, 
	p.completions as completions, 
	p.attempts as attempts, 
	p.description as description, 
	u.tier as user_tier,
	r.name as background_name,
	r.color_palette as background_palette,
	r.render_in_front as background_render,
	u.user_status as user_status,
	p.estimated_tutorial_time as estimated_tutorial_time
from recommended_post rp 
	join post p on p._id = rp.post_id 
	join users u on u._id = p.author_id
	left join rewards r on u.avatar_reward = r._id
where 
	rp.user_id = ?
	and rp.accepted = false
	and p.published = true
	and p.deleted = false
order by score desc 
limit 32
offset ?`

const recommendedProjectsHomeQueryNoLogin = `
select 
	p._id as _id, 
	p.created_at as created_at, 
	p.updated_at as updated_at, 
	p.title as title, 
	p.author as author, 
	p.author_id as author_id, 
	p.repo_id as repo_id,
	p.challenge_cost as challenge_cost,
	p.tier as tier, 
	p.coffee as coffee,
	p.post_type as post_type, 
	p.views as views, 
	p.completions as completions, 
	p.attempts as attempts, 
	p.description as description, 
	u.tier as user_tier,
	r.name as background_name,
	r.color_palette as background_palette,
	r.render_in_front as background_render,
	u.user_status as user_status,
	p.estimated_tutorial_time as estimated_tutorial_time
from post p 
	join users u on u._id = p.author_id
	left join rewards r on u.avatar_reward = r._id
where 
	p.published = 1
order by p.attempts desc 
limit 32
offset ?`

const languageBasicsProjectQuery = `
select
    p._id as _id,
    p.created_at as created_at,
    p.updated_at as updated_at,
    p.title as title,
    p.author as author,
    p.author_id as author_id,
    p.repo_id as repo_id,
    p.challenge_cost as challenge_cost,
    p.tier as tier,
    p.coffee as coffee,
    p.post_type as post_type,
    p.views as views,
    p.completions as completions,
    p.attempts as attempts,
    p.description as description,
    u.tier as user_tier,
    r.name as background_name,
    r.color_palette as background_palette,
    r.render_in_front as background_render,
    u.user_status as user_status,
	p.estimated_tutorial_time as estimated_tutorial_time
from post p
         join users u on u._id = p.author_id
         left join rewards r on u.avatar_reward = r._id
where
        p._id in (1688570643030736896, 1688617436791701504, 1688638972722413568, 1688656093628071936, 1688914007987060736, 1688940677359992832, 1688982281277931520, 1689003147793530880, 1689029578237935616, 1689326500572037120, 1689350506096361472)
order by p.attempts desc`

func ActiveProjectsHome(ctx context.Context, callingUser *models.User, tidb *ti.Database, storageEngine storage.Storage) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "active-projects-home-core")
	defer span.End()
	callerName := "ActiveProjectsHome"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, activeProjectsHomeQuery, callingUser.ID)
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

		projectFrontend := project.ToFrontend()

		thumbnail, err := getExistingFilePath(storageEngine, project.PostID, project.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve thumbnail: %v", err)
		}

		projectFrontend.Thumbnail = thumbnail

		projects = append(projects, projectFrontend)
	}

	return map[string]interface{}{"projects": projects}, nil
}

func LanguageBasicsProjects(ctx context.Context, tidb *ti.Database, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "language-basics-projects-core")
	defer span.End()
	callerName := "LanguageBasicsProjects"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, languageBasicsProjectQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. Language Basics Projects core.    Error: %v", err)
	}

	projects := make([]*query_models.RecommendedPostMergeFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project query_models.RecommendedPostMerge

		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Language Basics Projects core.    Error: %v", err)
		}

		projects = append(projects, project.ToFrontend())
	}

	return map[string]interface{}{"projects": projects}, nil
}

func RecommendedProjectsHome(ctx context.Context, callingUser *models.User, tidb *ti.Database, logger logging.Logger,
	req *ReccommendedProjectsHomeRequest) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "recommended-projects-home-core")
	defer span.End()
	callerName := "RecommendedProjectsHome"

	var res *sql.Rows
	var err error

	if callingUser == nil {
		// query attempt and projects with the user id as author id and sort by date last edited
		res, err = tidb.QueryContext(ctx, &span, &callerName, recommendedProjectsHomeQueryNoLogin, req.Skip)
		if err != nil {
			return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core no login.    Error: %v", err)
		}

	} else {
		// query attempt and projects with the user id as author id and sort by date last edited
		res, err = tidb.QueryContext(ctx, &span, &callerName, recommendedProjectsHomeQuery, callingUser.ID, req.Skip)
		if err != nil {
			return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
		}
	}

	projects := make([]*query_models.RecommendedPostMergeFrontend, 0)

	defer res.Close()

	// create a slice to hold the project ids that should be updated for views
	recIds := make([]interface{}, 0)
	recIdSlots := make([]string, 0)

	// iterate the cursor loading the recommended posts
	for res.Next() {
		var project query_models.RecommendedPostMerge

		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. recommended Project Home core.    Error: %v", err)
		}

		// add the project id to the slice
		recIds = append(recIds, project.RecommendedID)
		recIdSlots = append(recIdSlots, "?")

		projects = append(projects, project.ToFrontend())
	}

	// launch a go routine to update the views for the recommended posts
	if len(recIds) > 0 {
		go func() {
			// format the query to update the views
			updateQuery := fmt.Sprintf(
				"update recommended_post set views = views + 1 where _id in (%s)",
				strings.Join(recIdSlots, ","),
			)

			// execute the update query
			_, err := tidb.DB.Exec(updateQuery, recIds...)
			if err != nil {
				logger.Errorf("failed to update recommended post views: %v", err)
			}
		}()
	}

	if len(projects) == 0 && req.Skip == 0 {
		// query attempt and projects with the user id as author id and sort by date last edited
		res, err = tidb.QueryContext(ctx, &span, &callerName, recommendedProjectsHomeQueryNoLogin, req.Skip)
		if err != nil {
			return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core no login.    Error: %v", err)
		}

		defer res.Close()

		// iterate the cursor loading the recommended posts
		for res.Next() {
			var project query_models.RecommendedPostMerge

			err = sqlstruct.Scan(&project, res)
			if err != nil {
				return nil, fmt.Errorf("failed to decode query for resulsts. recommended Project Home core.    Error: %v", err)
			}

			projects = append(projects, project.ToFrontend())
		}
	}

	return map[string]interface{}{"projects": projects}, nil
}

// Mapping from string to ProgrammingLanguage enum
var languageMap = map[string]models.ProgrammingLanguage{
	"Any":          models.AnyProgrammingLanguage,
	"I'm not sure": models.AnyProgrammingLanguage,
	"Go":           models.Go,
	"Python":       models.Python,
	"JavaScript":   models.JavaScript,
	"Typescript":   models.TypeScript,
	"Rust":         models.Rust,
	"Java":         models.Java,
	"C#":           models.Csharp,
	"SQL":          models.SQL,
	"HTML":         models.Html,
	"Swift":        models.Swift,
	"Ruby":         models.Ruby,
	"C++":          models.Cpp,
	"Other":        models.AnyProgrammingLanguage,
}

// TopRecommendations fetches the top 5 curated project recommendations for a user based on proficiency and preferred programming language.
func TopRecommendations(ctx context.Context, callingUser *models.User, tidb *ti.Database) (map[string]interface{}, error) {
	// Start a new trace for monitoring with OpenTelemetry.
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "top-recommendations-core")
	defer span.End()
	callerName := "TopRecommendations"

	// Validate if StartUserInfo is available
	if callingUser.StartUserInfo == nil {
		return nil, fmt.Errorf("StartUserInfo is nil")
	}

	// Extract and store user's preferred language and proficiency for easier access.
	preferredLang := callingUser.StartUserInfo.PreferredLanguage
	userProficiency := callingUser.StartUserInfo.Proficiency

	// Translate the user's preferred language to its enum representation.
	// If the language is not found in 'languageMap', use a default enum value.
	langEnum, exists := languageMap[preferredLang]
	if !exists {
		langEnum = models.AnyProgrammingLanguage
	}

	// SQL query to fetch curated projects filtered by the user's proficiency level.
	// The query is limited to fetch 20 records to further refine based on language preference.
	query := "SELECT * FROM curated_post WHERE EXISTS (SELECT 1 FROM curated_post_type WHERE curated_id = curated_post._id AND proficiency_type = ?) LIMIT 20;"
	rows, err := tidb.QueryContext(ctx, &span, &callerName, query, userProficiency)
	if err != nil {
		return nil, fmt.Errorf("failed to query curated post table: %v", err)
	}
	defer rows.Close() // Ensure rows are closed after use.

	var projects []models.CuratedPost
	// Populate the 'projects' slice by iterating through the SQL rows.
	for rows.Next() {
		project, err := models.CuratedPostFromSQLNative(tidb, rows)
		if err != nil {
			return nil, fmt.Errorf("error reading row: %v", err)
		}
		projects = append(projects, *project)
	}

	// Sort the projects to bring the ones matching the user's preferred language to the top.
	sort.SliceStable(projects, func(i, j int) bool {
		a := projects[i]
		b := projects[j]
		if a.PostLanguage == langEnum {
			return true
		}
		if b.PostLanguage == langEnum {
			return false
		}
		return false
	})

	// Truncate the list to only include the top 5 projects.
	if len(projects) > 5 {
		projects = projects[:5]
	}

	// Extract the Post IDs from the sorted and truncated list of projects.
	var postIDs []interface{}
	for _, project := range projects {
		postIDs = append(postIDs, project.PostID)
	}

	// Handle case where no projects match the criteria
	if len(postIDs) == 0 {
		return map[string]interface{}{"message": "No projects found"}, nil
	}

	// Build the SQL query to fetch posts by their IDs.
	var postQueryBuilder strings.Builder
	postQueryBuilder.WriteString("SELECT * FROM post WHERE _id IN (?")
	for range postIDs[1:] {
		postQueryBuilder.WriteString(",?")
	}
	postQueryBuilder.WriteString(")")
	postQuery := postQueryBuilder.String()

	// Execute the query to fetch the posts.
	res, err := tidb.QueryContext(ctx, &span, &callerName, postQuery, postIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute post query: %v", err)
	}
	defer res.Close()

	// Initialize slice to hold resulting frontend posts.
	frontendPosts := make([]*models.PostFrontend, 0)
	for res.Next() {
		post, err := models.PostFromSQLNative(tidb, res)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post row: %v", err)
		}
		fp, err := post.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert post to frontend model: %v", err)
		}
		frontendPosts = append(frontendPosts, fp)
	}

	return map[string]interface{}{"projects": frontendPosts}, nil
}
