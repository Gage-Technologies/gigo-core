package core

import (
	"context"
	"fmt"
	"gigo-core/gigo/api/external_api/core/query_models"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/kisielk/sqlstruct"
	"go.opentelemetry.io/otel"
)

// TODO: needs testing

const (
	MAX_SEARCH_DEPTH = 1000
	MAX_SEARCH_SIZE  = 100
)

// conditionallyAddRangeFilter
//
//	Helper function to conditionally add a range filter
//	for the configured attribute. The function will
//	only add a range if there is at least one bound
//	set.
//
//	Args:
//	 - conditions ([]search.FilterCondition): filter conditions that will be appended to
//	 - attribute (string): the attribute to filter on
//	 - lower (interface{}): the lower bound of the range (pointer safe)
//	 - upper (interface{}): the upper bound of the range (pointer safe)
//	Returns:
//	 - []search.FilterCondition: the new filter conditions containing the added filter
func conditionallyAddRangeFilter(conditionals []search.FilterCondition, attribute string, lower interface{}, upper interface{}) []search.FilterCondition {
	// conditionally add filter range
	if !reflect.ValueOf(lower).IsNil() || !reflect.ValueOf(upper).IsNil() {
		// create filter condition for range
		filterCondition := search.FilterCondition{
			// use a logical and merge so that we can create a range
			And: true,
		}

		// conditionally add lower bound to filter condition
		if !reflect.ValueOf(lower).IsNil() {
			filterCondition.Filters = append(filterCondition.Filters, search.Filter{
				Attribute: attribute,
				Operator:  search.OperatorGreaterThanOrEquals,
				Value:     lower,
			})
		}

		// conditionally add upper bound to filter condition
		if !reflect.ValueOf(upper).IsNil() {
			filterCondition.Filters = append(filterCondition.Filters, search.Filter{
				Attribute: attribute,
				Operator:  search.OperatorLessThanOrEquals,
				Value:     upper,
			})
		}

		// append filter condition to conditions
		conditionals = append(conditionals, filterCondition)
	}

	return conditionals
}

// SearchPosts
//
//	 Searches for posts (challenges) using the passed query
//	 and filter criteria
//
//	   Args:
//	       - meili (*search.MeiliSearchEngine): meilisearch client wrapper form gigo-lib
//		      - callingUser (*models.User): user that is calling this function
//		      - query (string): query that will be searched across posts
//		      - languages ([]models.ProgrammingLanguage): (optional) languages that should be used in the post (logical OR)
//		      - author (*int64): (optional) author that created the post
//		      - attemptsMin (*int64): (optional) minimum attempts for the post
//		      - attemptsMax (*int64): (optional) maximum attempts for the post
//		      - completionsMin (*int64): (optional) minimum completions for the post
//		      - completionsMax (*int64): (optional) maximum completions for the post
//		      - coffeeMin (*int64): (optional) minimum coffee for the post
//		      - coffeeMax (*int64): (optional) maximum coffee for the post
//		      - viewsMin (*int64): (optional) minimum views for the post
//		      - viewsMax (*int64): (optional) maximum views for the post
//		      - tags ([]int64): (optional) tags that should be included in the post (logical OR)
//		      - challengeType (*models.ChallengeType): (optional) challenge type of the post
//		      - visibility (*models.PostVisibility): (optional) (admin-only: models.PrivateVisibility, models.FriendsVisibility) visibility status of the post
//		      - since (*time.Time): (optional) earliest creation date of the post
//		      - until (*time.Time): (optional) latest creation date of the post
//		      - published (*bool): (optional) (admin-only) published status of the post
//		      - tier (*models.TierType): (optional) difficulty tier of the post
//		      - skip (int): amount of results to skip
//		      - limit (int): amount of results to return after the skip
//	 Returns:
//	     - map[string]interface{}: (optional: error) json that will be returned to the caller if nil on error the default response is used
func SearchPosts(
	ctx context.Context,
	tidb *ti.Database,
	sf *snowflake.Node,
	meili *search.MeiliSearchEngine,
	callingUser *models.User,
	query string,
	languages []models.ProgrammingLanguage,
	author *int64,
	attemptsMin *int64,
	attemptsMax *int64,
	completionsMin *int64,
	completionsMax *int64,
	coffeeMin *int64,
	coffeeMax *int64,
	viewsMin *int64,
	viewsMax *int64,
	tags []int64,
	challengeType *models.ChallengeType,
	visibility *models.PostVisibility,
	since *time.Time,
	until *time.Time,
	published *bool,
	tier *models.TierType,
	skip int,
	limit int,
	searchRecModelID *int64,
	logger logging.Logger,
) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-posts-core")
	defer span.End()
	callerName := "SearchPosts"

	// restrict skip and limit to max size
	if limit > MAX_SEARCH_SIZE {
		limit = MAX_SEARCH_SIZE
	}
	if skip > MAX_SEARCH_DEPTH-limit {
		skip = MAX_SEARCH_DEPTH - limit
	}

	var searchRequest *search.Request

	if query == "" {
		// create request for post search operation
		searchRequest = &search.Request{
			Query: query,
			// initialize filter with an AND logical merge
			Filter: &search.FilterGroup{
				And: true,
			},
			Sort: &search.SortGroup{
				Sorts: []search.Sort{
					{
						Attribute: "created_at",
						Desc:      true,
					},
				},
			},
			// set skip and limit
			Offset: skip,
			Limit:  limit,
		}
	} else {
		// create request for post search operation
		searchRequest = &search.Request{
			Query: query,
			// initialize filter with an AND logical merge
			Filter: &search.FilterGroup{
				And: true,
			},
			// set skip and limit
			Offset: skip,
			Limit:  limit,
		}
	}

	// conditionally add language filter
	if len(languages) > 0 {
		// create interface slice to hold languages
		languagesInterface := make([]interface{}, len(languages))

		// add languages to interface slice
		for i, language := range languages {
			languagesInterface[i] = language
		}

		// append languages filter condition to
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "languages",
					Operator:  search.OperatorIn,
					Values:    languagesInterface,
				},
			},
		})
	}

	// conditionally add author filter
	if author != nil {
		// append filter condition for author id
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "author_id",
					Operator:  search.OperatorEquals,
					Value:     author,
				},
			},
		})
	}

	// conditionally add attempts filter range
	searchRequest.Filter.Filters = conditionallyAddRangeFilter(searchRequest.Filter.Filters, "attempts", attemptsMin, attemptsMax)

	// conditionally add completions filter range
	searchRequest.Filter.Filters = conditionallyAddRangeFilter(searchRequest.Filter.Filters, "completions", completionsMin, completionsMax)

	// conditionally add coffee filter range
	searchRequest.Filter.Filters = conditionallyAddRangeFilter(searchRequest.Filter.Filters, "coffee", coffeeMin, coffeeMax)

	// conditionally add views filter range
	searchRequest.Filter.Filters = conditionallyAddRangeFilter(searchRequest.Filter.Filters, "views", viewsMin, viewsMax)

	// conditionally add time filter range
	searchRequest.Filter.Filters = conditionallyAddRangeFilter(searchRequest.Filter.Filters, "created_at", since, until)

	// conditionally add tags filter
	if len(tags) > 0 {
		// create interface slice to hold tags
		tagsInterface := make([]interface{}, len(tags))

		// add tags to interface slice
		for i, tag := range tags {
			tagsInterface[i] = tag
		}

		// append tags filter condition to filters
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "tags",
					Operator:  search.OperatorIn,
					Values:    tagsInterface,
				},
			},
			// use a logical OR for merging the language filters
			And: false,
		})
	}

	// conditionally add challenge type filter
	if challengeType != nil {
		// append filter condition for challenge type
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "post_type",
					Operator:  search.OperatorEquals,
					Value:     challengeType,
				},
			},
		})
	}

	// conditionally add visibility filter
	if visibility != nil {
		// ensure that only admins can search for private posts
		if (*visibility == models.PrivateVisibility || *visibility == models.FriendsVisibility) && (callingUser == nil || callingUser.AuthRole != models.Admin) {
			err := fmt.Errorf("anon user attempted to search private content: %v", *visibility)
			if callingUser == nil {
				err = fmt.Errorf("user %d attempted to search private content: %v", callingUser.ID, *visibility)
			}
			return map[string]interface{}{"message": "user cannot search private content"}, err
		}

		// append filter condition for visibility
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "visibility",
					Operator:  search.OperatorEquals,
					Value:     visibility,
				},
			},
		})
	} else {
		// default to non-private posts

		// append filter condition for visibility
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "visibility",
					Operator:  search.OperatorNotIn,
					Values:    []interface{}{models.PrivateVisibility, models.FriendsVisibility},
				},
			},
		})
	}

	// conditionally add tier filter
	if tier != nil {
		// append filter condition for tier
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "tier",
					Operator:  search.OperatorEquals,
					Value:     tier,
				},
			},
		})
	}

	var user int64
	if callingUser != nil {
		user = callingUser.ID
	}

	// conditionally add published filter for admins
	if published != nil && callingUser != nil && callingUser.AuthRole == models.Admin {
		// append filter condition for published
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "published",
					Operator:  search.OperatorEquals,
					Value:     published,
				},
			},
		})
	} else if published != nil && callingUser != nil && callingUser.AuthRole != models.Admin && user == *author {
		// append filter condition for published
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "published",
					Operator:  search.OperatorEquals,
					Value:     published,
				},
			},
		})
	} else {
		// default to only published content

		// append filter condition for published
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "published",
					Operator:  search.OperatorEquals,
					Value:     true,
				},
			},
		})
	}

	// execute search
	searchResult, err := meili.Search("posts", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute posts search: %v", err)
	}

	// create slice to hold posts
	posts := make([]*models.PostFrontend, 0)

	// iterate results cursor scanning results into post structs and appending the to the posts slice
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load post into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create post to scan into
		var post models.Post

		// attempt to scan the post from the cursor
		err = searchResult.Scan(&post)
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

	logger.Debugf("search_rec_id: %v inside of Search", searchRecModelID)

	if searchRecModelID == nil {
		logger.Debugf("search_rec_id: %v inside of Search, executing create search rec", searchRecModelID)

		searcRcID := sf.Generate().Int64()
		postIDs := make([]int64, 0)
		for _, post := range posts {
			id, err := strconv.ParseInt(post.ID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse post ID: %v", err)
			}
			postIDs = append(postIDs, id)
		}
		searchRc := models.CreateSearchRec(searcRcID, user, postIDs, query, nil, nil, time.Now())
		statements := searchRc.ToSQLNative()
		for _, statement := range statements {
			_, err = tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
			if err != nil {
				return nil, fmt.Errorf("failed to execute insert statement for search_rec: %v", err)
			}
		}

		logger.Debugf("search_rec_id: %v created", searchRc.ID)
		return map[string]interface{}{"challenges": posts, "search_rec_id": fmt.Sprintf("%v", searchRc.ID)}, nil

	}

	insertStatement := "insert into search_rec_posts(search_id, post_id) values"
	paramSlots := make([]interface{}, 0)
	if *searchRecModelID > 0 && len(posts) > 0 {
		logger.Debugf("search_rec_id: %v inside of Search, executing update to existing search rec", *searchRecModelID)
		for i, post := range posts {
			if i == 0 {
				insertStatement += " (?,?)"
			} else {
				insertStatement += ", (?,?)"
			}
			paramSlots = append(paramSlots, searchRecModelID, post.ID)
		}

		_, err = tidb.ExecContext(ctx, &span, &callerName, insertStatement, paramSlots...)
		if err != nil {
			logger.Info("insert search rec post failed because the ids are duplicates")
		}

		return map[string]interface{}{"challenges": posts, "search_rec_id": fmt.Sprintf("%v", *searchRecModelID)}, nil
	}

	logger.Debugf("search_rec_id: %v inside of Search, executing with no search rec", *searchRecModelID)

	return map[string]interface{}{"challenges": posts, "search_rec_id": nil}, nil
}

func SearchUsers(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, query string, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-users-core")
	defer span.End()
	callerName := "SearchUsers"

	// restrict skip and limit to max size
	if limit > MAX_SEARCH_SIZE {
		limit = MAX_SEARCH_SIZE
	}
	if skip > MAX_SEARCH_DEPTH-limit {
		skip = MAX_SEARCH_DEPTH - limit
	}

	// create search request for query
	searchRequest := &search.Request{
		Query:  query,
		Offset: skip,
		Limit:  limit,
	}

	// execute search
	searchResult, err := meili.Search("users", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute posts search: %v", err)
	}

	// create slice to hold user ids
	userIds := make([]interface{}, 0)
	paramSlots := make([]string, 0)

	// iterate results cursor scanning results into user search structs and appending the id to the users slice
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load user into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create user to scan into
		var user models.UserSearch

		// attempt to scan the user from the cursor
		err = searchResult.Scan(&user)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}

		// append id to outer slice
		userIds = append(userIds, user.ID)

		// append new param slot
		paramSlots = append(paramSlots, "?")
	}

	// exit if there are no users returned
	if len(userIds) == 0 {
		return map[string]interface{}{"users": []interface{}{}}, nil
	}

	// format query for multi-user query
	query = "select u._id as _id, user_name, user_rank, render_in_front, color_palette, name, level, tier, user_status from users u left join rewards r on r._id = u.avatar_reward where u._id in (" + strings.Join(paramSlots, ",") + ")"

	// query database for users
	res, err := tidb.QueryContext(ctx, &span, &callerName, query, userIds...)
	if err != nil {
		return nil, fmt.Errorf("failed to query database for users: %v", err)
	}

	defer res.Close()

	// create slice to hold users
	users := make([]*query_models.UserBackgroundFrontend, 0)

	// iterate results cursor scanning results into user structs and appending the frontend formatted user objects to the users slice
	for res.Next() {

		var user query_models.UserBackground
		// decode row results
		err = sqlstruct.Scan(&user, res)
		if err != nil {
			return nil, fmt.Errorf("failed to query database for users: %v", err)
		}
		// // load user from sql
		// user, err := models.UserFromSQLNative(tidb, res)
		// if err != nil {
		//	return nil, fmt.Errorf("failed to load user from sql: %v", err)
		// }

		// format the user to its frontend value
		fp, err := user.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert user to frontend object: %v", err)
		}

		users = append(users, fp)
	}

	return map[string]interface{}{"users": users}, nil
}

func SearchTags(ctx context.Context, meili *search.MeiliSearchEngine, query string, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-tags-core")
	defer span.End()
	// restrict skip and limit to max size
	if limit > MAX_SEARCH_SIZE {
		limit = MAX_SEARCH_SIZE
	}
	if skip > MAX_SEARCH_DEPTH-limit {
		skip = MAX_SEARCH_DEPTH - limit
	}

	// create search request for query
	searchRequest := &search.Request{
		Query:  query,
		Offset: skip,
		Limit:  limit,
	}

	// execute search
	searchResult, err := meili.Search("tags", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tags search: %v", err)
	}

	// create map to hold unique tags
	tagsMap := make(map[string]*models.TagFrontend)

	// iterate results cursor scanning results into tag structs
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load tag into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create tag to scan into
		var tag models.Tag

		// attempt to scan the tag from the cursor
		err = searchResult.Scan(&tag)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}

		// Convert tag value to lower case for comparison
		lowerValue := strings.ToLower(tag.Value)

		// if this tag value is already in map and its UsageCount is higher, replace it, else just insert it
		if val, exists := tagsMap[lowerValue]; !exists || val.UsageCount < tag.UsageCount {
			tagsMap[lowerValue] = tag.ToFrontend()
		}
	}

	// Convert map to slice
	tags := make([]*models.TagFrontend, 0, len(tagsMap))
	for _, tag := range tagsMap {
		tags = append(tags, tag)
	}

	// Sort tags slice by Value desc bc go maps mess up the order returned from meili
	sort.Slice(tags, func(i, j int) bool {
		return strings.ToLower(tags[i].Value) < strings.ToLower(tags[j].Value)
	})

	return map[string]interface{}{"tags": tags}, nil
}

func SearchDiscussions(ctx context.Context, meili *search.MeiliSearchEngine, query string, skip int, limit int, postId *int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-discussions-core")
	defer span.End()

	// restrict skip and limit to max size
	if limit > MAX_SEARCH_SIZE {
		limit = MAX_SEARCH_SIZE
	}
	if skip > MAX_SEARCH_DEPTH-limit {
		skip = MAX_SEARCH_DEPTH - limit
	}

	// create search request for query
	searchRequest := &search.Request{
		Query: query,
		// initialize filter with an AND logical merge
		Filter: &search.FilterGroup{
			And: true,
		},
		Offset: skip,
		Limit:  limit,
	}

	// add search filter for discussions on given post
	if postId != nil {
		// search filter interface
		filter := make([]interface{}, 0)
		filter = append(filter, *postId)

		// append post id filter condition
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "post_id",
					Operator:  search.OperatorIn,
					Values:    filter,
				},
			},
		})
	}

	// execute search
	searchResult, err := meili.Search("discussion", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute discussions search: %v", err)
	}

	// create slice to hold discussions
	discussions := make([]*models.DiscussionFrontend, 0)

	// iterate results cursor scanning results into discussion structs and appending the id to the discussions slice
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load discussions into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create discussion to scan into
		var disc models.Discussion

		// attempt to scan the discussion from the cursor
		err = searchResult.Scan(&disc)
		if err != nil {
			return nil, fmt.Errorf("failed to scan discussions: %v", err)
		}

		// append discussion to outer slice
		discussions = append(discussions, disc.ToFrontend())
	}

	return map[string]interface{}{"discussions": discussions}, nil
}

func SearchComments(ctx context.Context, meili *search.MeiliSearchEngine, query string, skip int, limit int, discussionId *int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-comment-core")
	defer span.End()

	// restrict skip and limit to max size
	if limit > MAX_SEARCH_SIZE {
		limit = MAX_SEARCH_SIZE
	}
	if skip > MAX_SEARCH_DEPTH-limit {
		skip = MAX_SEARCH_DEPTH - limit
	}

	// create search request for query
	searchRequest := &search.Request{
		Query: query,
		// initialize filter with an AND logical merge
		Filter: &search.FilterGroup{
			And: true,
		},
		Offset: skip,
		Limit:  limit,
	}

	// add search filter for comment on given post
	if discussionId != nil {
		// search filter interface
		filter := make([]interface{}, 0)
		filter = append(filter, *discussionId)

		// append post id filter condition
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "discussion_id",
					Operator:  search.OperatorIn,
					Values:    filter,
				},
			},
		})
	}

	// execute search
	searchResult, err := meili.Search("comment", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute comment search: %v", err)
	}

	// create slice to hold comment
	comments := make([]*models.CommentFrontend, 0)

	// iterate results cursor scanning results into comment structs and appending the id to the comment slice
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load comment into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create comment to scan into
		var comment models.Comment

		// attempt to scan the comment from the cursor
		err = searchResult.Scan(&comment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %v", err)
		}

		// append comment to outer slice
		comments = append(comments, comment.ToFrontend())
	}

	return map[string]interface{}{"comment": comments}, nil
}

func SearchWorkspaceConfigs(ctx context.Context, db *ti.Database, meili *search.MeiliSearchEngine, query string,
	languages []models.ProgrammingLanguage, tags []int64, skip int, limit int) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-workspace-configs-core")
	defer span.End()
	callerName := "SearchWorkspaceConfigs"

	// restrict skip and limit to max size
	if limit > MAX_SEARCH_SIZE {
		limit = MAX_SEARCH_SIZE
	}
	if skip > MAX_SEARCH_DEPTH-limit {
		skip = MAX_SEARCH_DEPTH - limit
	}

	// create request for post search operation
	searchRequest := &search.Request{
		Query: query,
		// initialize filter with an AND logical merge
		Filter: &search.FilterGroup{
			And: true,
		},
		// set skip and limit
		Offset: skip,
		Limit:  limit,
	}

	// conditionally add language filter
	if len(languages) > 0 {
		// create interface slice to hold languages
		languagesInterface := make([]interface{}, len(languages))

		// add languages to interface slice
		for i, language := range languages {
			languagesInterface[i] = language
		}

		// append languages filter condition to
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "languages",
					Operator:  search.OperatorIn,
					Values:    languagesInterface,
				},
			},
		})
	}

	// conditionally add tags filter
	if len(tags) > 0 {
		// create interface slice to hold tags
		tagsInterface := make([]interface{}, len(tags))

		// add tags to interface slice
		for i, tag := range tags {
			tagsInterface[i] = tag
		}

		// append tags filter condition to filters
		searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
			Filters: []search.Filter{
				{
					Attribute: "tags",
					Operator:  search.OperatorIn,
					Values:    tagsInterface,
				},
			},
			// use a logical OR for merging the language filters
			And: false,
		})
	}

	// execute search
	searchResult, err := meili.Search("workspace_configs", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute workspace config search: %v", err)
	}

	// create slice to hold workspace configs
	workspaceConfigs := make([]*models.WorkspaceConfig, 0)

	// create slice to hold author ids
	authorIds := make([]interface{}, 0)
	authorParamSlots := make([]string, 0)

	// create slice to hold tag ids
	tagIds := make([]interface{}, 0)
	tagParamSlots := make([]string, 0)

	// iterate results cursor scanning results into tag structs and appending the id to the slice
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load tag into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create workspace config to scan into
		var workspaceConfig models.WorkspaceConfig

		// attempt to scan the workspace config from the cursor
		err = searchResult.Scan(&workspaceConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %v", err)
		}

		// append workspace config to outer slice
		workspaceConfigs = append(workspaceConfigs, &workspaceConfig)

		// append author id to outer slice
		authorIds = append(authorIds, workspaceConfig.AuthorID)
		authorParamSlots = append(authorParamSlots, "?")

		// iterate tags appending ids to outer slice
		for _, tag := range workspaceConfig.Tags {
			tagIds = append(tagIds, tag)
			tagParamSlots = append(tagParamSlots, "?")
		}
	}

	// handle empty configs
	if len(workspaceConfigs) == 0 {
		return map[string]interface{}{"workspace_configs": []interface{}{}, "tags": map[string]interface{}{}}, nil
	}

	// format query for author name lookup
	query = fmt.Sprintf("select _id, user_name from users where _id in (%s)", strings.Join(authorParamSlots, ","))

	// execute query
	res, err := db.QueryContext(ctx, &span, &callerName, query, authorIds...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v\n    query: %s\n    params: %v", err, query, authorIds)
	}

	// defer closure of cursor
	defer res.Close()

	// create map to correlate ids to user names
	authorMap := make(map[int64]string)

	// iterate results cursor scanning results into author structs and assigning the id to the map
	for res.Next() {
		var userId int64
		var userName string

		err = res.Scan(&userId, &userName)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user name: %v", err)
		}

		authorMap[userId] = userName
	}

	// create slice to hold workspace configs formatted for the frontend
	workspaceConfigsFrontend := make([]*models.WorkspaceConfigFrontend, len(workspaceConfigs))

	// iterate the workspace configs retrieving the author name for each author id
	for i, workspaceConfig := range workspaceConfigs {
		authorName, ok := authorMap[workspaceConfig.AuthorID]
		if !ok {
			authorName = "[deleted]"
		}
		workspaceConfigsFrontend[i] = workspaceConfig.ToFrontend()
		workspaceConfigsFrontend[i].Author = authorName
	}

	// close cursor
	_ = res.Close()

	// create map to correlate tags to tag ids
	tagMap := make(map[string]*models.TagFrontend)

	if len(tagParamSlots) > 0 {
		// format query for author name lookup
		query = fmt.Sprintf("select * from tag where _id in (%s)", strings.Join(tagParamSlots, ","))

		// execute query
		res, err = db.QueryContext(ctx, &span, &callerName, query, tagIds...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %v\n    query: %s\n    params: %v", err, query, tagIds)
		}

		defer res.Close()

		// iterate results cursor scanning results into tag structs and assigning the id to the map
		for res.Next() {
			tag, err := models.TagFromSQLNative(res)
			if err != nil {
				return nil, fmt.Errorf("failed to scan tag: %v", err)
			}

			// format tag to frontend format
			tagFrontend := tag.ToFrontend()

			tagMap[tagFrontend.ID] = tagFrontend
		}
	}

	return map[string]interface{}{"workspace_configs": workspaceConfigsFrontend, "tags": tagMap}, nil
}

func SimpleSearchPosts(meili *search.MeiliSearchEngine, query string) (map[string]interface{}, error) {

	const MAX_SEARCH_SIZE = 100   // Replace this with the actual value
	const MAX_SEARCH_DEPTH = 1000 // Replace this with the actual value

	// restrict skip and limit to max size
	limit := MAX_SEARCH_SIZE
	skip := 0 // Set to zero or whatever your default skip value is

	// create request for post search operation
	searchRequest := &search.Request{
		Query: query,
		// initialize filter with an AND logical merge
		Filter: &search.FilterGroup{
			And: true,
		},
		// default sorting by created_at in descending order
		Sort: &search.SortGroup{
			Sorts: []search.Sort{
				{
					Attribute: "created_at",
					Desc:      true,
				},
			},
		},
		// set skip and limit
		Offset: skip,
		Limit:  limit,
	}

	// default to only published content
	searchRequest.Filter.Filters = append(searchRequest.Filter.Filters, search.FilterCondition{
		Filters: []search.Filter{
			{
				Attribute: "published",
				Operator:  search.OperatorEquals,
				Value:     true,
			},
		},
	})

	// execute search
	searchResult, err := meili.Search("posts", searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute posts search: %v", err)
	}

	// create slice to hold posts
	posts := make([]*models.PostFrontend, 0)

	// iterate results cursor, scanning results into post structs and appending them to the posts slice
	for {
		// attempt to load next value into the first position of the cursor
		ok, err := searchResult.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to load post into cursor buffer: %v", err)
		}

		// exit if we are done
		if !ok {
			break
		}

		// create post to scan into
		var post models.Post

		// attempt to scan the post from the cursor
		err = searchResult.Scan(&post)
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

	return map[string]interface{}{"posts": posts}, nil
}

const SearchFriendsQuery = `
select 
    u._id, 
    u.user_name, 
    u.user_rank, 
    r.render_in_front, 
    r.color_palette, 
    r.name, 
    u.level, 
    u.tier, 
    u.user_status 
from users u
    left join rewards r on r._id = u.avatar_reward
	join friends f on f.friend = u._id
where f.user_id = ? and lower(u.user_name) like ?
`

func SearchFriends(ctx context.Context, db *ti.Database, callingUser *models.User, query string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-friends-core")
	defer span.End()
	callerName := "SearchFriends"

	// prep the query for search
	query = strings.ToLower(query) + "%"

	// query the friends by username
	res, err := db.QueryContext(ctx, &span, &callerName, SearchFriendsQuery, callingUser.ID, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query database for friends: %v", err)
	}

	// defer closure of cursor
	defer res.Close()

	// create slice to hold users
	users := make([]*query_models.UserBackgroundFrontend, 0)

	// iterate results cursor scanning results into user structs and appending the frontend formatted user objects to the users slice
	for res.Next() {
		var user query_models.UserBackground
		// decode row results
		err = sqlstruct.Scan(&user, res)
		if err != nil {
			return nil, fmt.Errorf("failed to query database for users: %v", err)
		}

		// format the user to its frontend value
		fp, err := user.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert user to frontend object: %v", err)
		}

		users = append(users, fp)
	}

	return map[string]interface{}{"users": users}, nil
}

const SearchChatUsersQuery = `
select 
    u._id, 
    u.user_name, 
    u.user_rank, 
    r.render_in_front, 
    r.color_palette, 
    r.name, 
    u.level, 
    u.tier, 
    u.user_status 
from users u
    left join rewards r on r._id = u.avatar_reward
	join chat_users cu on cu.user_id = u._id
where cu.chat_id = ? and lower(u.user_name) like ?
`

func SearchChatUsers(ctx context.Context, db *ti.Database, callingUser *models.User, chatId int64, query string) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "search-chat-users-core")
	defer span.End()
	callerName := "SearchChatUsers"

	// ensure that the user is a member of the chat
	var permitted bool
	err := db.QueryRowContext(ctx, &span, &callerName, "select count(*) from chat_users where user_id = ? and chat_id = ?", callingUser.ID, chatId).Scan(&permitted)
	if err != nil {
		return nil, fmt.Errorf("failed to query database for chat user: %v", err)
	}

	// prep the query for search
	query = strings.ToLower(query) + "%"

	// query the friends by username
	res, err := db.QueryContext(ctx, &span, &callerName, SearchChatUsersQuery, chatId, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query database for friends: %v", err)
	}

	// defer closure of cursor
	defer res.Close()

	// create slice to hold users
	users := make([]*query_models.UserBackgroundFrontend, 0)

	// iterate results cursor scanning results into user structs and appending the frontend formatted user objects to the users slice
	for res.Next() {
		var user query_models.UserBackground
		// decode row results
		err = sqlstruct.Scan(&user, res)
		if err != nil {
			return nil, fmt.Errorf("failed to query database for users: %v", err)
		}

		// format the user to its frontend value
		fp, err := user.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert user to frontend object: %v", err)
		}

		users = append(users, fp)
	}

	return map[string]interface{}{"users": users}, nil
}
