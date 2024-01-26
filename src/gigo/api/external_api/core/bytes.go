package core

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gigo-core/gigo/config"
	"gigo-core/gigo/utils"
	"github.com/gage-technologies/gigo-lib/logging"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/storage"
	utils2 "github.com/gage-technologies/gigo-lib/utils"
	"github.com/go-git/go-git/v5/utils/ioutil"
	"go.opentelemetry.io/otel"
)

type CreateByteParams struct {
	Ctx                    context.Context
	Tidb                   *ti.Database
	Sf                     *snowflake.Node
	CallingUser            *models.User
	StorageEngine          storage.Storage
	Meili                  *search.MeiliSearchEngine
	Name                   string
	DescriptionEasy        string
	DescriptionMedium      string
	DescriptionHard        string
	OutlineEasy            string
	OutlineMedium          string
	OutlineHard            string
	DevelopmentStepsEasy   string
	DevelopmentStepsMedium string
	DevelopmentStepsHard   string
	QuestionsEasy          []string
	QuestionsMedium        []string
	QuestionsHard          []string
	Language               models.ProgrammingLanguage
	Thumbnail              string
}

type BytesRecFrontend struct {
	ID                   string                     `json:"_id" sql:"_id"`
	Name                 string                     `json:"name" sql:"name"`
	DescriptionEasy      string                     `json:"description_easy" sql:"description_easy"`
	DescriptionMedium    string                     `json:"description_medium" sql:"description_medium"`
	DescriptionHard      string                     `json:"description_hard" sql:"description_hard"`
	OutlineContentEasy   string                     `json:"outline_content_easy" sql:"outline_content_easy"`
	OutlineContentMedium string                     `json:"outline_content_medium" sql:"outline_content_medium"`
	OutlineContentHard   string                     `json:"outline_content_hard" sql:"outline_content_hard"`
	DevStepsEasy         string                     `json:"dev_steps_easy" sql:"dev_steps_easy"`
	DevStepsMedium       string                     `json:"dev_steps_medium" sql:"dev_steps_medium"`
	DevStepsHard         string                     `json:"dev_steps_hard" sql:"dev_steps_hard"`
	QuestionsEasy        []string                   `json:"questions_easy" sql:"questions_easy"`
	QuestionsMedium      []string                   `json:"questions_medium" sql:"questions_medium"`
	QuestionsHard        []string                   `json:"questions_hard" sql:"questions_hard"`
	Lang                 models.ProgrammingLanguage `json:"lang" sql:"lang"`
	Published            bool                       `json:"published" sql:"published"`
	Color                string                     `json:"color" sql:"color"`
	CompletedEasy        bool                       `json:"completed_easy" sql:"completed_easy"`
	CompletedMedium      bool                       `json:"completed_medium" sql:"completed_medium"`
	CompletedHard        bool                       `json:"completed_hard" sql:"completed_hard"`
	Modified             bool                       `json:"modified" sql:"modified"`
}

//type Difficulty int
//
//const (
//	Easy Difficulty = iota
//	Medium
//	Hard
//)
//
//func (d Difficulty) ToString() string {
//	switch d {
//	case Easy:
//		return "easy"
//	case Medium:
//		return "medium"
//	case Hard:
//		return "hard"
//	default:
//		return "medium"
//	}
//}

func CreateByte(params CreateByteParams) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(params.Ctx, "create-byte-core")
	defer span.End()
	callerName := "CreateByte"

	// create a new id for the post
	id := params.Sf.Generate().Int64()

	// get temp thumbnail file from storage
	thumbnailTempFile, err := params.StorageEngine.GetFile(params.Thumbnail)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	}
	defer thumbnailTempFile.Close()

	// sanitize thumbnail image
	thumbnailBuffer := bytes.NewBuffer([]byte{})
	color, err := utils.PrepImageFile(thumbnailTempFile, ioutil.WriteNopCloser(thumbnailBuffer), true, true)
	if err != nil {
		return nil, fmt.Errorf("failed to prep thumbnail file: %v", err)
	}

	// record failure state to cleanup on exit
	failed := true

	// defer function to cleanup repo on failure
	defer func() {
		// skip cleanup if we succeeded
		if !failed {
			return
		}

		_ = params.Meili.DeleteDocuments("bytes", id)
	}()

	// create a new byte
	bytes, err := models.CreateBytes(
		id,
		params.Name,
		params.DescriptionEasy,
		params.DescriptionMedium,
		params.DescriptionHard,
		params.OutlineEasy,
		params.OutlineMedium,
		params.OutlineHard,
		params.DevelopmentStepsEasy,
		params.DevelopmentStepsMedium,
		params.DevelopmentStepsHard,
		params.QuestionsEasy,
		params.QuestionsMedium,
		params.QuestionsHard,
		params.Language,
		color,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte: %v", err)
	}

	// format the post into sql insert statements
	statements, err := bytes.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte into insert statements: %v", err)
	}

	// create transaction for byte insertion
	tx, err := params.Tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// iterate over insert statements performing insertion into sql
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to perform insertion statement for byte: %v\n    statement: %s\n    params: %v", err, statement.Statement, statement.Values)
		}
	}

	// write thumbnail to final location
	idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
	if err != nil {
		return nil, fmt.Errorf("failed to hash bytes id: %v", err)
	}
	err = params.StorageEngine.CreateFile(
		fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash),
		thumbnailBuffer.Bytes(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to write thumbnail to final location: %v", err)
	}

	// attempt to insert the prost into the search engine to make it discoverable
	err = params.Meili.AddDocuments("bytes", bytes.ToSearch())
	if err != nil {
		return nil, fmt.Errorf("failed to add bytes to search engine: %v", err)
	}

	// format byte to frontend object
	fp := bytes.ToFrontend()
	if err != nil {
		return nil, fmt.Errorf("failed to format byte to frontend object: %v", err)
	}

	// commit insert tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit")
	}

	// set success flag
	failed = false

	return map[string]interface{}{"message": "Byte created successfully.", "byte": fp}, nil
}

// StartByteAttempt creates a new `ByteAttempt` from the passed `Byte` and creates a new workspace.
func StartByteAttempt(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, callingUser *models.User,
	byteId int64) (map[string]interface{}, error) {

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "start-byte-attempt-core")
	defer span.End()
	callerName := "StartByteAttempt"

	// ensure this user doesn't have an attempt already
	var existingByteAttempt models.ByteAttempts
	err := tidb.QueryRowContext(ctx, &span, &callerName,
		"select _id, byte_id, author_id, content_easy, content_medium, content_hard, modified from byte_attempts where byte_id = ? and author_id = ? limit 1", byteId, callingUser.ID,
	).Scan(&existingByteAttempt.ID, &existingByteAttempt.ByteID, &existingByteAttempt.AuthorID, &existingByteAttempt.ContentEasy, &existingByteAttempt.ContentMedium, &existingByteAttempt.ContentHard, &existingByteAttempt.Modified)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get attempt count: %v", err)
	}

	// if they already have an attempt, return the content of the attempt
	if existingByteAttempt.ID > 0 {
		return map[string]interface{}{
			"message":      "Existing attempt found.",
			"byte_attempt": existingByteAttempt.ToFrontend(),
		}, nil
	}

	// Fetch outline_content for the byte
	var outlineContentEasy string
	var outlineContentMedium string
	var outlineContentHard string
	err = tidb.QueryRowContext(ctx, &span, &callerName,
		"select outline_content_easy, outline_content_medium, outline_content_hard from bytes where _id = ?", byteId,
	).Scan(&outlineContentEasy, &outlineContentMedium, &outlineContentHard)
	if err != nil {
		return nil, fmt.Errorf("failed to get byte outline content: %v", err)
	}

	byteAttempt, err := models.CreateByteAttempts(sf.Generate().Int64(), byteId, callingUser.ID, outlineContentEasy, outlineContentMedium, outlineContentHard)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte attempt struct: %v", err)
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
		err = res.Scan(&attempt.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan byte attempts: %v", err)
		}

		byteAttempt = append(byteAttempt, attempt)
	}

	return map[string]interface{}{"byte_attempt": byteAttempt}, nil
}

// GetRecommendedBytes for now return the top 50 bytes but do not include the content or plan content
func GetRecommendedBytes(ctx context.Context, tidb *ti.Database, authorID *int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-recommended-bytes-core")
	defer span.End()
	callerName := "GetRecommendedBytes"

	bytes := make([]*BytesRecFrontend, 0)

	if authorID != nil {
		// query for 50 bytes
		res, err := tidb.QueryContext(ctx, &span, &callerName, "SELECT b._id, b.name, b.description_easy, b.description_medium, b.description_hard, b.lang, IFNULL(ba.completed_easy, false) as completed_easy,  IFNULL(ba.completed_medium, false) as completed_medium, IFNULL(ba.completed_hard, false) as completed_hard, IFNULL(ba.modified, false) as modified FROM bytes b LEFT JOIN byte_attempts ba ON b._id = ba.byte_id AND ba.author_id = ? WHERE b.published = true LIMIT 50;", *authorID)
		if err != nil {
			return nil, fmt.Errorf("failed to query recommended bytes: %v", err)
		}

		// ensure the closure of the rows
		defer res.Close()

		for res.Next() {
			b := BytesRecFrontend{}
			err := res.Scan(&b.ID, &b.Name, &b.DescriptionEasy, &b.DescriptionMedium, &b.DescriptionHard, &b.Lang, &b.CompletedEasy, &b.CompletedMedium, &b.CompletedHard, &b.Modified)
			if err != nil {
				return nil, fmt.Errorf("failed to scan bytes: %v", err)
			}

			bytes = append(bytes, &b)
		}
	} else {
		// query for 50 bytes
		res, err := tidb.QueryContext(ctx, &span, &callerName, "SELECT _id, name, description_easy, description_medium, description_hard, lang FROM bytes WHERE published = true LIMIT 50;")
		if err != nil {
			return nil, fmt.Errorf("failed to query recommended bytes: %v", err)
		}

		// ensure the closure of the rows
		defer res.Close()

		for res.Next() {
			b := BytesRecFrontend{}
			err := res.Scan(&b.ID, &b.Name, &b.DescriptionEasy, &b.DescriptionMedium, &b.DescriptionHard, &b.Lang)
			if err != nil {
				return nil, fmt.Errorf("failed to scan bytes: %v", err)
			}

			bytes = append(bytes, &b)
		}
	}

	return map[string]interface{}{"rec_bytes": bytes}, nil
}

// GetByte returns the full metadata of the Byte
func GetByte(ctx context.Context, tidb *ti.Database, byteId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-bytes-core")
	defer span.End()
	callerName := "GetBytes"

	// query for the byte with the given ID
	res, err := tidb.QueryContext(ctx, &span, &callerName, "select * from bytes where _id = ?", byteId)
	if err != nil {
		return nil, fmt.Errorf("failed to query for the byte metadata: %v", err)
	}
	defer res.Close()

	if !res.Next() {
		return nil, fmt.Errorf("no byte found with id: %d", byteId)
	}

	byte, err := models.BytesFromSQLNative(res)
	if err != nil {
		return nil, fmt.Errorf("failed to convert byte to SQL native type: %v", err)
	}

	// Check for more rows. If there are, it's an unexpected situation.
	if res.Next() {
		return nil, fmt.Errorf("unexpected multiple rows for byte id: %d", byteId)
	}

	return map[string]interface{}{"rec_bytes": byte.ToFrontend()}, nil
}

func SetByteCompleted(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, stripeSubConfig config.StripeSubscriptionConfig, callingUser *models.User, byteAttemptId int64, difficulty string, logger logging.Logger) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-bytes-core")
	defer span.End()
	callerName := "SetByteCompleted"

	res, err := tidb.ExecContext(ctx, &span, &callerName, fmt.Sprintf("update byte_attempts set completed_%s = TRUE where byte_id = ? and author_id = ?", difficulty), byteAttemptId, callingUser.ID)
	if err != nil {
		return nil, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rows != 1 {
		return nil, errors.New(fmt.Sprintf("failed to update byte attempt completed for difficulty: %v, err: no rows update", difficulty))
	}

	// add xp to user for logging in
	xpRes, err := AddXP(ctx, tidb, nil, nil, sf, stripeSubConfig, callingUser.ID, "successful", nil, nil, logger, callingUser)
	if err != nil {
		return map[string]interface{}{"message": fmt.Sprintf("Byte Marked as a Success for difficulty: %s", difficulty)}, fmt.Errorf("failed to add xp to user: %v", err)
	}

	return map[string]interface{}{"success": true, "xp": xpRes}, nil

}

func PublishByte(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, byteId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "publish-byte-core")
	defer span.End()
	callerName := "PublishByte"

	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer tx.Rollback()

	// update post in sql
	_, err = tx.ExecContext(ctx, &callerName, "update bytes set published = true where _id =?", byteId)
	if err != nil {
		return nil, fmt.Errorf("failed to update byte: %v", err)
	}

	// update post in meilisearch
	err = meili.UpdateDocuments("bytes", map[string]interface{}{"_id": byteId, "published": true})
	if err != nil {
		return nil, fmt.Errorf("failed to update byte in meilisearch: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit publishing: %v", err)
	}

	return map[string]interface{}{"success": true}, nil

}

func UnPublishByte(ctx context.Context, tidb *ti.Database, meili *search.MeiliSearchEngine, byteId int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "unpublish-byte-core")
	defer span.End()
	callerName := "UnPublishByte"

	tx, err := tidb.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}

	defer tx.Rollback()

	// update post in sql
	_, err = tx.ExecContext(ctx, &callerName, "update bytes set published = false where _id =?", byteId)
	if err != nil {
		return nil, fmt.Errorf("failed to update byte: %v", err)
	}

	// update post in meilisearch
	err = meili.UpdateDocuments("bytes", map[string]interface{}{"_id": byteId, "published": false})
	if err != nil {
		return nil, fmt.Errorf("failed to update byte in meilisearch: %v", err)
	}

	// commit tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit unpublishing: %v", err)
	}

	return map[string]interface{}{"success": true}, nil

}
