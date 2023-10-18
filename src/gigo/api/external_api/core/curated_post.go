package core

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"strings"
)

func AddPostToCurated(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, callingUser *models.User,
	postId int64, proficiencyTypes []models.ProficiencyType, postLanguage models.ProgrammingLanguage) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "add-post-to-curated-core")
	defer span.End()
	callerName := "AddPostToCurated"

	// abort if not gigo admin
	if callingUser == nil || callingUser.UserName != "gigo" {
		return map[string]interface{}{"message": "Failed to add curated project"}, fmt.Errorf("callinguser is not an admin. AddPostToCurated core")
	}

	// create new CuratedPost
	curatedPost, err := models.CreateCuratedPost(sf.Generate().Int64(), postId, proficiencyTypes, postLanguage)
	if err != nil {
		return map[string]interface{}{"message": "Failed to add curated project"}, fmt.Errorf("failed to create new curated post: %v", err)
	}

	// create transaction for curated insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return map[string]interface{}{"message": "Failed to add curated project"}, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// create insert statement
	statement := curatedPost.ToSQLNative()

	for _, stmt := range statement {
		// insert statement
		_, err = tx.ExecContext(ctx, &callerName, stmt.Statement, stmt.Values...)
		if err != nil {
			return map[string]interface{}{"message": "Failed to add curated project"}, fmt.Errorf("failed to perform insertion statement for curated post: %v\n    statement: %s\n    params: %v", err, stmt.Statement, stmt.Values)
		}
	}

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return map[string]interface{}{"message": "Failed to add curated project"}, fmt.Errorf("failed to commit transaction for curated post: %v", err)
	}

	return map[string]interface{}{"message": "Added curated project successfully"}, nil
}

func RemoveCuratedPost(ctx context.Context, tidb *ti.Database, callingUser *models.User, curatedPostID int64) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "remove-curated-post-core")
	defer span.End()
	callerName := "RemoveCuratedPost"

	// abort if not gigo admin
	if callingUser == nil || callingUser.UserName != "gigo" {
		return map[string]interface{}{"message": "Failed to remove curated project"}, fmt.Errorf("callinguser is not an admin. RemoveCuratedPost core")
	}

	// Create a variable to hold the _id
	var curatedID int64

	// Query to get _id from curated_post using post_id
	row := tidb.QueryRowContext(ctx, &span, &callerName, "SELECT _id FROM curated_post WHERE post_id = ?;", curatedPostID)

	// Scan the _id from the row
	err := row.Scan(&curatedID)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{"message": "Curated post ID not found"}, err
		}
		return map[string]interface{}{"message": "Failed to remove curated project"}, fmt.Errorf("failed to execute query to retrieve _id: %v", err)
	}

	// create transaction for curated deletion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return map[string]interface{}{"message": "Failed to remove curated project"}, fmt.Errorf("failed to create delete tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	// execute delete statement for curated post
	_, err = tx.ExecContext(ctx, &callerName, "DELETE FROM curated_post WHERE post_id = ?;", curatedPostID)
	if err != nil {
		return map[string]interface{}{"message": "Failed to remove curated project"}, fmt.Errorf("failed to perform deletion statement for curated post: %v", err)
	}

	// execute delete statement for curated post proficiencies
	_, err = tx.ExecContext(ctx, &callerName, "DELETE FROM curated_post_type WHERE curated_id = ?;", curatedID)
	if err != nil {
		return map[string]interface{}{"message": "Failed to remove curated project type from sub table"}, fmt.Errorf("failed to perform deletion statement for curated post type sub table: %v", err)
	}

	// commit delete tx
	err = tx.Commit(&callerName)
	if err != nil {
		return map[string]interface{}{"message": "Failed to remove curated project"}, fmt.Errorf("failed to commit transaction for curated post deletion: %v", err)
	}

	return map[string]interface{}{"message": "Successfully removed curated project"}, nil
}

// GetCuratedPostsAdmin fetches posts curated for administrators.
// It uses proficiency and programming language filters.
func GetCuratedPostsAdmin(ctx context.Context, tidb *ti.Database, callingUser *models.User,
	proficiencyFilter models.ProficiencyType, languageFilter models.ProgrammingLanguage) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-curated-posts-admin")
	defer span.End()
	callerName := "GetCuratedPostsAdmin"

	// Validate if the calling user is an admin
	if callingUser == nil || callingUser.UserName != "gigo" {
		return nil, fmt.Errorf("callinguser is not an admin. GetCuratedPostsAdmin core")
	}

	// Initialize SQL query builder and argument list
	var queryBuilder strings.Builder
	var args []interface{}

	// Start building SQL query for fetching curated posts
	queryBuilder.WriteString(`SELECT cp._id, cp.post_id, cp.post_language 
		FROM curated_post cp`)

	// If filters are provided, update the SQL query to include them
	if proficiencyFilter != 3 || languageFilter != 0 {
		// Join with curated_post_type to fetch proficiency levels
		queryBuilder.WriteString(" INNER JOIN curated_post_type cpt ON cp._id = cpt.curated_id WHERE ")

		firstCondition := false

		// Add proficiency filter to SQL query
		if proficiencyFilter != 3 {
			queryBuilder.WriteString("cpt.proficiency_type = ?")
			args = append(args, proficiencyFilter)
			firstCondition = true
		}

		// Add programming language filter to SQL query
		if languageFilter != 0 {
			if firstCondition {
				queryBuilder.WriteString(" AND ")
			}
			queryBuilder.WriteString("cp.post_language = ?")
			args = append(args, languageFilter)
		}
	}
	queryBuilder.WriteString(";")
	query := queryBuilder.String()

	// Execute the query and fetch rows
	rows, err := tidb.QueryContext(ctx, &span, &callerName, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	// Initialize slice to hold curated posts
	var curatedPosts []*models.CuratedPost
	for rows.Next() {
		var curatedPost models.CuratedPost

		// Scan each row into a CuratedPost object
		err := rows.Scan(
			&curatedPost.ID,
			&curatedPost.PostID,
			&curatedPost.PostLanguage,
		)
		if err != nil {
			span.SetAttributes(attribute.String("sql.error", err.Error()))
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		curatedPosts = append(curatedPosts, &curatedPost)
	}

	// Check for errors in row iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating over rows: %v", err)
	}

	// Extract post IDs from curated posts
	var postIDs []interface{}
	for _, curatedPost := range curatedPosts {
		postIDs = append(postIDs, curatedPost.PostID)
	}

	// Handle case where no posts match the criteria
	if len(postIDs) == 0 {
		return map[string]interface{}{"message": "No projects found"}, nil
	}

	// Build query to fetch posts by IDs
	var postQueryBuilder strings.Builder
	postQueryBuilder.WriteString("SELECT * FROM post WHERE _id IN (?")
	for range postIDs[1:] {
		postQueryBuilder.WriteString(",?")
	}
	postQueryBuilder.WriteString(") ORDER BY attempts DESC")
	postQuery := postQueryBuilder.String()

	// Execute the post query
	rows, err = tidb.QueryContext(ctx, &span, &callerName, postQuery, postIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute post query: %v", err)
	}
	defer rows.Close()

	// Initialize slice to hold resulting posts
	posts := make([]*models.PostFrontend, 0)
	for rows.Next() {
		// Scan each row into a Post object
		post, err := models.PostFromSQLNative(tidb, rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %v", err)
		}
		fp, err := post.ToFrontend()
		if err != nil {
			return nil, fmt.Errorf("failed to convert post to frontend object: %v", err)
		}
		posts = append(posts, fp)
	}

	// Handle case where no posts match the criteria
	if len(postIDs) == 0 {
		return map[string]interface{}{
			"message": "No projects found",
		}, nil
	}

	return map[string]interface{}{"curated_posts": posts}, nil
}

func CurationAuth(ctx context.Context, callingUser *models.User, curateSecret string, password string) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "curation-auth-core")
	defer span.End()

	// abort if not gigo admin
	if callingUser == nil || callingUser.UserName != "gigo" {
		return map[string]interface{}{"message": "Incorrect calling user", "auth": false}, nil
	}

	// check password against secret
	if password != curateSecret {
		return map[string]interface{}{"message": "Incorrect password", "auth": false}, nil
	} else {
		return map[string]interface{}{"message": "Access Granted", "auth": true}, nil
	}

}
