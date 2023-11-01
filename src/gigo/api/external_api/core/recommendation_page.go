package core

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"

	"gigo-core/gigo/api/external_api/core/query_models"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
)

// TopRecommendation
// Retrieves the top recommended project for callingUser:
//
// Args:
//
//	tidb       		- *ti.Database, tidb
//	callingUser     - models.User, the user the function is being called for
//
// Returns:
//
//	out        - map[string]interface{}, "top_recommendation" - the top recommendation based on similarity
//			   - error
func TopRecommendation(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "top-recommendation-core")
	callerName := "TopRecommendation"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select p._id as _id, rp._id as recommended_id, p.created_at as created_at, p.updated_at as updated_at, score as similarity, title, author, author_id, repo_id, tier, coffee, post_type, views, completions, attempts, description, p.estimated_tutorial_time as estimated_tutorial_time from recommended_post rp join post p on p._id = rp.post_id where user_id = ? order by score desc limit 1", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
	}

	// create variable to hold top recommendation
	var topProject *query_models.RecommendedPostMergeFrontend

	// ensure the closure of the rows
	defer res.Close()

	for res.Next() {
		// create model to scan res into
		var project query_models.RecommendedPostMerge

		// attempt to scan row into RecommendedPostMerge model
		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for top recommendation: Error: %v", err)
		}

		// save project post scan
		topProject = project.ToFrontend()
	}

	return map[string]interface{}{"top_project": topProject}, nil
}

// RecommendByAttempt
// Retrieves recommendations based on previous attempts, and returns recommendations queried with user id if no attempts found:
//
// Args:
//
//	tidb       		- *ti.Database, tidb
//	callingUser     - models.User, the user the function is being called for
//
// Returns:
//
//	out        - map[string]interface{},
//					- "recommended_one" - the top recommendation based on attempt 1
//					- *"recommended_one" - the top recommendation based on attempt 2
//			   - error
func RecommendByAttempt(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "recommended-by-attempt-core")
	callerName := "RecommendByAttempt"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select rp.reference_id as recommended_id, post_title as title from attempt a join recommended_post rp on a.post_id = rp.reference_id where a.author_id = ? group by reference_id order by score desc limit 2", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
	}

	// create variable to hold top recommendation
	var topProject []query_models.RecommendedPostMerge

	// ensure the closure of the rows
	defer res.Close()

	for res.Next() {
		// create model to scan res into
		var project query_models.RecommendedPostMerge

		// attempt to scan row into RecommendedPostMerge model
		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for top recommendation: Error: %v", err)
		}

		topProject = append(topProject, project)
	}

	// query attempt and projects with the user id as author id and sort by date last edited
	res1, err := tidb.QueryContext(ctx, &span, &callerName, "select p._id as _id, rp._id as recommended_id, p.created_at as created_at, p.updated_at as updated_at, score as similarity, title, p.author_id as author, p.repo_id as repo_id, p.tier as tier, p.coffee as coffee, post_type, views, completions, attempts, p.description as description, p.estimated_tutorial_time as estimated_tutorial_time from recommended_post rp join post p on p._id = rp.post_id left join attempt a on a.post_id = rp.post_id where a.author_id is null and rp.reference_id = ? order by score desc limit 15", topProject[0].RecommendedID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
	}

	// create variable to hold top recommendation
	firsRec := make([]*query_models.RecommendedPostMergeFrontend, 0)

	// ensure the closure of the rows
	defer res1.Close()

	for res1.Next() {
		// create model to scan res into
		var project query_models.RecommendedPostMerge

		// attempt to scan row into RecommendedPostMerge model
		err = sqlstruct.Scan(&project, res1)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for top recommendation: Error: %v", err)
		}

		firsRec = append(firsRec, project.ToFrontend())
	}

	firstTitle := topProject[0].Title

	// query attempt and projects with the user id as author id and sort by date last edited
	res2, err := tidb.QueryContext(ctx, &span, &callerName, "select p._id as _id, rp._id as recommended_id, p.created_at as created_at, p.updated_at as updated_at, score as similarity, title, p.author_id as author, p.repo_id as repo_id, p.tier as tier, p.coffee as coffee, post_type, views, completions, attempts, p.description as description,	p.estimated_tutorial_time as estimated_tutorial_time from recommended_post rp join post p on p._id = rp.post_id left join attempt a on a.post_id = rp.post_id where a.author_id is null and rp.reference_id = ? order by score desc limit 15", topProject[1].RecommendedID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
	}

	// create variable to hold top recommendation
	secondRec := make([]*query_models.RecommendedPostMergeFrontend, 0)

	// ensure the closure of the rows
	defer res2.Close()

	for res2.Next() {
		// create model to scan res into
		var project query_models.RecommendedPostMerge

		// attempt to scan row into RecommendedPostMerge model
		err = sqlstruct.Scan(&project, res2)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for top recommendation: Error: %v", err)
		}

		secondRec = append(secondRec, project.ToFrontend())
	}

	secondTitle := topProject[1].Title

	return map[string]interface{}{"first_rec_name": firstTitle, "first_rec_data": firsRec, "second_rec_name": secondTitle, "second_rec_data": secondRec}, nil
}

// query attempt and projects with the user id as author id and sort by date last edited
//func RecommendByAttempt(tidb *ti.Database, callingUser *models.User, limitRecommendations *int) (map[string]interface{}, error) {
//
//	// query for all active projects for specified user
//	res, err := tidb.DB.Query("select * from attempt where author_id = ? order by completed desc limit 5", callingUser.ID)
//	if err != nil {
//		return nil, fmt.Errorf("failed to query active projects: %v", err)
//	}
//
//	// ensure the closure of the rows
//	defer res.Close()
//
//	// bool to indicate whether user has any attempts
//	hasAttempt := false
//
//	// check if any active projects were found
//	if res != nil {
//		hasAttempt = true
//	}
//
//	// placeholder for returned attempts
//	attempts := make([]*models.Attempt, 0)
//
//	// decode query results if any attempts were returned
//	if hasAttempt {
//		// iterate over all returned attempts
//		for res.Next() {
//			attempt, err := models.AttemptFromSQLNative(tidb, res)
//			if err != nil {
//				return nil, fmt.Errorf("failed to decode query for attempts: %v", err)
//			}
//
//			attempts = append(attempts, attempt)
//		}
//
//		// sort project slice by most recently updated
//		sort.Slice(attempts, func(i, j int) bool {
//			return attempts[i].UpdatedAt.After(attempts[j].UpdatedAt)
//		})
//	}
//
//	// close explicitly
//	_ = res.Close()
//
//	// create bool to indicate whether limit was specified
//	hasLimit := false
//
//	// determine whether limit is specified
//	if limitRecommendations != nil && *limitRecommendations > 0 {
//		hasLimit = true
//	}
//
//	// determine length of attempt slice
//	attemptCount := len(attempts)
//
//	// create slices to hold the recommendations
//	finalFrontendRecommendationOne := make([]*query_models.RecommendedPostMergeFrontend, 0)
//	finalFrontendRecommendationTwo := make([]*query_models.RecommendedPostMergeFrontend, 0)
//
//	// query for at least two attempts
//	if attemptCount >= 2 {
//		// create variables to hold row results
//		var rowResOne *sql.Rows
//		var rowResTwo *sql.Rows
//
//		if hasLimit == false {
//			//query attempt and projects with the post id and sort by similarity
//			rowResOne, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.post_id = ? order by similarity desc", attempts[0].PostID)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//			rowResTwo, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.post_id = ? order by similarity desc", attempts[1].PostID)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//
//			for rowResOne.Next() {
//				// create variable to hold recommendations for first post
//				var recommendOne *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendOne, rowResOne)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationOne = append(finalFrontendRecommendationOne, recommendOne.ToFrontend())
//			}
//			rowResOne.Close()
//
//			for rowResTwo.Next() {
//				// create variable to hold recommendations for first post
//				var recommendTwo *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendTwo, rowResTwo)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationTwo = append(finalFrontendRecommendationTwo, recommendTwo.ToFrontend())
//			}
//			rowResTwo.Close()
//			return map[string]interface{}{"recommended_one": finalFrontendRecommendationOne, "recommended_two": finalFrontendRecommendationTwo}, nil
//		} else {
//			//query attempt and projects with the post id and sort by similarity
//			rowResOne, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, languages, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.post_id = ? order by similarity desc limit ?", attempts[0].PostID, limitRecommendations)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//			rowResTwo, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, languages, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.post_id = ? order by similarity desc limit ?", attempts[1].PostID, limitRecommendations)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//
//			for rowResOne.Next() {
//				// create variable to hold recommendations for first post
//				var recommendOne *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendOne, rowResOne)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationOne = append(finalFrontendRecommendationOne, recommendOne.ToFrontend())
//			}
//			rowResOne.Close()
//
//			for rowResTwo.Next() {
//				// create variable to hold recommendations for first post
//				var recommendTwo *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendTwo, rowResTwo)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationTwo = append(finalFrontendRecommendationTwo, recommendTwo.ToFrontend())
//			}
//			rowResTwo.Close()
//			return map[string]interface{}{"recommended_one": finalFrontendRecommendationOne, "recommended_two": finalFrontendRecommendationTwo}, nil
//		}
//	} else if attemptCount == 1 {
//		// create variables to hold row results
//		var rowResOne *sql.Rows
//		var rowResTwo *sql.Rows
//
//		if hasLimit == false {
//			//query attempt and projects with the post id and sort by similarity
//			rowResOne, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.post_id = ? order by similarity desc", attempts[0].PostID)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//			//query by user id if no second attempt available
//			rowResTwo, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.author_id = ? order by similarity desc", callingUser.ID)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//
//			for rowResOne.Next() {
//				// create variable to hold recommendations for first post
//				var recommendOne *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendOne, rowResOne)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationOne = append(finalFrontendRecommendationOne, recommendOne.ToFrontend())
//			}
//			rowResOne.Close()
//
//			for rowResTwo.Next() {
//				// create variable to hold recommendations for first post
//				var recommendTwo *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendTwo, rowResTwo)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationTwo = append(finalFrontendRecommendationTwo, recommendTwo.ToFrontend())
//			}
//			rowResTwo.Close()
//			return map[string]interface{}{"recommended_one": finalFrontendRecommendationOne, "recommended_two": finalFrontendRecommendationTwo}, nil
//		} else {
//			//query attempt and projects with the post id and sort by similarity
//			rowResOne, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.post_id = ? order by similarity desc limit ?", attempts[0].PostID, limitRecommendations)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//			//query by user id if no second attempt available
//			rowResTwo, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.author_id = ? order by similarity desc limit ?", callingUser.ID, limitRecommendations)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//
//			for rowResOne.Next() {
//				// create variable to hold recommendations for first post
//				var recommendOne *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendOne, rowResOne)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationOne = append(finalFrontendRecommendationOne, recommendOne.ToFrontend())
//			}
//			rowResOne.Close()
//
//			for rowResTwo.Next() {
//				// create variable to hold recommendations for first post
//				var recommendTwo *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendTwo, rowResTwo)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationTwo = append(finalFrontendRecommendationTwo, recommendTwo.ToFrontend())
//			}
//			rowResTwo.Close()
//			return map[string]interface{}{"recommended_one": finalFrontendRecommendationOne, "recommended_two": finalFrontendRecommendationTwo}, nil
//		}
//	} else {
//		// create variables to hold row results
//		var rowResOne *sql.Rows
//
//		if hasLimit == false {
//			//query by user id if no attempt available
//			rowResOne, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.author_id = ? order by similarity desc", callingUser.ID)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//
//			for rowResOne.Next() {
//				// create variable to hold recommendations for first post
//				var recommendOne *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendOne, rowResOne)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationOne = append(finalFrontendRecommendationOne, recommendOne.ToFrontend())
//			}
//			rowResOne.Close()
//
//			return map[string]interface{}{"recommended_one": finalFrontendRecommendationOne}, nil
//		} else {
//			//query by user id if no second attempt available
//			rowResOne, err = tidb.DB.Query("select p._id as _id, title, description, author, p.author_id as author_id, created_at, updated_at, repo_id, tier, top_reply, coffee, post_type, views, completions, attempts, similarity, rp.author_id as recommended_author_id from recommended_post rp join post p on rp.post_id = p._id where rp.author_id = ? order by similarity desc limit ?", callingUser.ID, limitRecommendations)
//			if err != nil {
//				return nil, fmt.Errorf("failed to query for recommended posts. RecommendByAttempt core. Error: %v", err)
//			}
//
//			for rowResOne.Next() {
//				// create variable to hold recommendations for first post
//				var recommendOne *query_models.RecommendedPostMerge
//
//				// attempt to scan row into RecommendedPostMerge model
//				err = sqlstruct.Scan(&recommendOne, rowResOne)
//				if err != nil {
//					return nil, fmt.Errorf("failed to decode query for RecommendByAttempt core: Error: %v", err)
//				}
//
//				finalFrontendRecommendationOne = append(finalFrontendRecommendationOne, recommendOne.ToFrontend())
//			}
//			rowResOne.Close()
//			return map[string]interface{}{"recommended_one": finalFrontendRecommendationOne}, nil
//		}
//	}
//}

// HarderRecommendation
// Retrieves recommended projects one tier higher than callingUser:
//
// Args:
//
//	tidb       		- *ti.Database, tidb
//	callingUser     - models.User, the user the function is being called for
//
// Returns:
//
//	out        - map[string]interface{}, "harder_projects" - recommendation one tier higher than user
//			   - error
func HarderRecommendation(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "harder-recommendation-core")
	callerName := "HarderRecommendation"

	// query attempt and projects with the user id as author id and sort by date last edited
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select p._id as _id, rp._id as recommended_id, p.created_at as created_at, p.updated_at as updated_at, score as similarity, title, author, author_id, repo_id, tier, coffee, post_type, views, completions, attempts, description, p.estimated_tutorial_time as estimated_tutorial_time from recommended_post rp join post p on p._id = rp.post_id where user_id = ? order by reference_tier desc, score desc limit 15", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for any attempts. recommended Project Home core.    Error: %v", err)
	}

	projects := make([]*query_models.RecommendedPostMergeFrontend, 0)

	defer res.Close()

	for res.Next() {
		var project query_models.RecommendedPostMerge

		err = sqlstruct.Scan(&project, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. recommended Project Home core.    Error: %v", err)
		}

		projects = append(projects, project.ToFrontend())
	}
	//
	//harderProjects := make([]*query_models.RecommendedPostMergeFrontend, 0)
	//
	//for i := 0; i < len(projects); i++ {
	//	if projects[i].Tier == callingUser.Tier+1 {
	//		harderProjects = append(harderProjects, projects[i])
	//	}
	//}

	//// sort project slice by similarity
	//sort.Slice(harderProjects, func(i, j int) bool {
	//	return harderProjects[i].Similarity > harderProjects[j].Similarity
	//})

	return map[string]interface{}{"harder_projects": projects}, nil
}
