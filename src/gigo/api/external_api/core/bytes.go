package core

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"go.opentelemetry.io/otel"
)

// StartByteAttempt creates a new `ByteAttempt` from the passed `Byte` and creates a new workspace.
func StartByteAttempt(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, callingUser *models.User,
	byteId int64) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "start-byte-attempt-core")
	defer span.End()
	callerName := "StartByteAttempt"

	// ensure this user doesn't have an attempt already
	var existingByteAttemptId int64
	err := tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id from byte_attempts where byte_id = ? and author_id = ? limit 1", byteId, callingUser.ID,
	).Scan(&existingByteAttemptId)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get attempt count: %v", err)
	}

	// if they already have an attempt, return the content of the attempt
	if existingByteAttemptId > 0 {
		var byteContent string
		err := tidb.QueryRowContext(ctx, &span, &callerName,
			"select content from byte_attempts where _id = ?", existingByteAttemptId,
		).Scan(&byteContent)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing byte attempt content: %v", err)
		}

		return map[string]interface{}{"content": byteContent}, nil
	}

	byteAttempt, err := models.CreateByteAttempts(sf.Generate().Int64(), byteId, callingUser.ID, "")
	if err != nil {
		fmt.Errorf("failed to create byte attempt struct: %v", err)
	}

	// format byte attempt for insertion
	insertStatements, err := byteAttempt.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte attempt for insertion: %v", err)
	}

	// open tx for byte attempt insertion
	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for byte attempt insertion: %v", err)
	}

	defer tx.Rollback()

	// iterate over insert statements executing them in sql
	for _, statement := range insertStatements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute insertion statement for byte attempt: %v\n    query: %s\n    params: %v",
				err, statement.Statement, statement.Values)
		}
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit byte attempt insertion: %v", err)
	}

	return map[string]interface{}{"message": "Attempt created successfully.", "byte_attempt": byteAttempt.ToFrontend()}, nil
}

// GetByteAttempt returns the ByteAttempt info for the passed user OR a clearly defined "not found" so the frontend can handle the not found case
func GetByteAttempt(ctx context.Context, tidb *ti.Database, callingUser *models.User) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-byte-attempts-core")
	defer span.End()
	callerName := "GetByteAttempt"

	// query for all active byte attempts for the user
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select _id from byte_attempts where author_id = ?", callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query byte attempt: %v", err)
	}

	// ensure the closure of the rows
	defer res.Close()

	// check if post was found with given id
	if res == nil {
		return map[string]interface{}{"byte_attempt": "not found"}, nil
	}

	byteAttempt := make([]*models.ByteAttemptsFrontend, 0)

	for res.Next() {
		attempt := &models.ByteAttemptsFrontend{}
		err = res.Scan(&attempt.ID, &attempt.ByteID, &attempt.AuthorID, &attempt.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to scan byte attempts: %v", err)
		}

		byteAttempt = append(byteAttempt, attempt)
	}

	return map[string]interface{}{"byte_attempt": byteAttempt}, nil
}

// GetRecommendedBytes for now return the top 50 bytes but do not include the content or plan content
func GetRecommendedBytes(ctx context.Context, tidb *ti.Database) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-recommended-bytes-core")
	defer span.End()
	callerName := "GetRecommendedBytes"

	// query for 50 bytes
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select _id, name, description from bytes limit 50")
	if err != nil {
		return nil, fmt.Errorf("failed to query recommended bytes: %v", err)
	}

	// ensure the closure of the rows
	defer res.Close()

	bytes := make([]*models.BytesFrontend, 0)

	for res.Next() {
		byte := &models.BytesFrontend{}
		err = res.Scan(&byte.ID, &byte.Name, &byte.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bytes: %v", err)
		}

		bytes = append(bytes, byte)
	}

	return map[string]interface{}{"rec_bytes": bytes}, nil
}

// GetByte returns the full metadata of the Byte
func GetByte(ctx context.Context, tidb *ti.Database, byteId int64) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-bytes-core")
	defer span.End()
	callerName := "GetBytes"

	// query for 50 bytes
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from bytes where _id = ?", byteId)
	if err != nil {
		return nil, fmt.Errorf("failed to query for the byte metadata: %v", err)
	}

	// ensure the closure of the rows
	defer res.Close()

	byte := &models.BytesFrontend{}
	err = res.Scan(&byte.ID, &byte.Name, &byte.Description, &byte.OutlineContent, &byte.DevSteps)
	if err != nil {
		return nil, fmt.Errorf("failed to scan byte: %v", err)
	}

	return map[string]interface{}{"rec_bytes": byte}, nil
}
