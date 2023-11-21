package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	"go.opentelemetry.io/otel"
	"strconv"
	"strings"
)

func CreateWorkspaceConfig(ctx context.Context, db *ti.Database, meili *search.MeiliSearchEngine, sf *snowflake.Node,
	callingUser *models.User, title string, description string, content string, tags []*models.Tag,
	languages []models.ProgrammingLanguage) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "create-workspace-config")
	defer span.End()
	callerName := "CreateWorkspaceConfig"

	// create id for new workspace config
	id := sf.Generate().Int64()

	// create slice to hold tag ids
	tagIds := make([]int64, len(tags))
	newTags := make([]interface{}, 0)

	// open tx to perform insertion
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for insertion: %v", err)
	}

	// create boolean to track failure
	failed := true

	// defer cleanup function
	defer func() {
		// skip for success
		if !failed {
			return
		}

		_ = tx.Rollback()
		_ = meili.DeleteDocuments("workspace_configs", id)
		for _, tag := range newTags {
			_ = meili.DeleteDocuments("tags", tag.(*models.TagSearch).ID)
		}
	}()

	// iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
	for _, tag := range tags {
		// conditionally create a new id and insert tag into database if it does not already exist
		if tag.ID == -1 {
			// generate new tag id
			tag.ID = sf.Generate().Int64()

			// iterate statements inserting the new tag into the database
			for _, statement := range tag.ToSQLNative() {
				_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
				if err != nil {
					return nil, fmt.Errorf("failed to perform tag insertion: %v", err)
				}
			}

			// add tag to new tags for search engine insertion
			newTags = append(newTags, tag.ToSearch())
		} else {
			// increment tag column usage_count in database
			_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where id =?", tag.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
			}
		}

		// append tag id to tag ids slice
		tagIds = append(tagIds, tag.ID)
	}

	// create new workspace config
	wc := models.CreateWorkspaceConfig(
		id,
		title,
		description,
		content,
		callingUser.ID,
		0,
		tagIds,
		languages,
		0,
	)

	// format to sql insert statements
	statements, err := wc.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format for insertion: %v", err)
	}

	// iterate through statements performing insertions
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert statement: %v", err)
		}
	}

	// perform insertion into search engine
	err = meili.AddDocuments("workspace_configs", wc)
	if err != nil {
		return nil, fmt.Errorf("failed to insert workspace config: %v", err)
	}

	// conditionally attempt to insert the tags into the search engine to make it discoverable
	if len(newTags) > 0 {
		err = meili.AddDocuments("tags", newTags...)
		if err != nil {
			return nil, fmt.Errorf("failed to add new tags to search engine: %v", err)
		}
	}

	// commit insertion tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// mark failed as false
	failed = false

	return map[string]interface{}{
		"message":          "Workspace Config Created.",
		"workspace_config": wc.ToFrontend(),
	}, nil
}

func UpdateWorkspaceConfig(ctx context.Context, db *ti.Database, meili *search.MeiliSearchEngine, sf *snowflake.Node,
	callingUser *models.User, id int64, description *string, content *string, tags []*models.Tag,
	languages []models.ProgrammingLanguage) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-workspace-config")
	defer span.End()
	callerName := "UpdateWorkspaceConfig"

	// create slice to hold tag ids
	tagIds := make([]int64, len(tags))
	newTags := make([]interface{}, 0)

	// open tx to perform insertion
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for insertion: %v", err)
	}

	// create boolean to track failure
	failed := true

	// defer cleanup function
	defer func() {
		// skip for success
		if !failed {
			return
		}

		_ = tx.Rollback()
		_ = meili.DeleteDocuments("workspace_configs", id)
		for _, tag := range newTags {
			_ = meili.DeleteDocuments("tags", tag.(*models.TagSearch).ID)
		}
	}()

	// conditionally iterate over the tags creating new tag structs for tags that do not already exist and adding ids to the slice created above
	if tags != nil {
		for _, tag := range tags {
			// conditionally create a new id and insert tag into database if it does not already exist
			if tag.ID == -1 {
				// generate new tag id
				tag.ID = sf.Generate().Int64()

				// iterate statements inserting the new tag into the database
				for _, statement := range tag.ToSQLNative() {
					_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
					if err != nil {
						return nil, fmt.Errorf("failed to perform tag insertion: %v", err)
					}
				}

				// add tag to new tags for search engine insertion
				newTags = append(newTags, tag.ToSearch())
			} else {
				// increment tag column usage_count in database
				_, err = tx.ExecContext(ctx, &callerName, "update tag set usage_count = usage_count + 1 where id =?", tag.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to increment tag usage count: %v", err)
				}
			}

			// append tag id to tag ids slice
			tagIds = append(tagIds, tag.ID)
		}
	}

	// query for existing config
	res, err := db.QueryContext(ctx, &span, &callerName, "select * from workspace_config where _id = ? and author_id = ? order by revision desc limit 1", id, callingUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing workspace config: %v", err)
	}

	defer res.Close()

	// load value into the first position of the cursor
	if !res.Next() {
		return nil, fmt.Errorf("could not find existing workspace config: %v", err)
	}

	// load workspace config from cursor
	workspaceConfig, err := models.WorkspaceConfigFromSQLNative(db, res)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing workspace config from cursor: %v", err)
	}

	// marshall and unmarshall workspace config using json to create a deep copy
	buf, err := json.Marshal(workspaceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshall workspace config: %v", err)
	}
	var newWorkspaceConfig models.WorkspaceConfig
	err = json.Unmarshal(buf, &newWorkspaceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall workspace config: %v", err)
	}

	// increment revision of the workspace config
	newWorkspaceConfig.Revision++

	// modify the workspace config based on the update
	if description != nil {
		newWorkspaceConfig.Description = *description
	}
	if content != nil {
		newWorkspaceConfig.Content = *content
	}
	if tags != nil {
		newWorkspaceConfig.Tags = tagIds
	}
	if languages != nil {
		newWorkspaceConfig.Languages = languages
	}

	// format to sql insert statements
	statements, err := newWorkspaceConfig.ToSQLNative()
	if err != nil {
		return nil, fmt.Errorf("failed to format for insertion: %v", err)
	}

	// iterate over insertion statements
	for _, statement := range statements {
		_, err = tx.ExecContext(ctx, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert statement: %v", err)
		}
	}

	// replace document in search engine
	err = meili.AddDocuments("workspace_configs", newWorkspaceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to insert workspace config to search engine: %v", err)
	}

	// conditionally attempt to insert the tags into the search engine to make it discoverable
	if len(newTags) > 0 {
		err = meili.AddDocuments("tags", newTags...)
		if err != nil {
			return nil, fmt.Errorf("failed to add new tags to search engine: %v", err)
		}
	}

	// commit insertion tx
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit tx: %v", err)
	}

	// mark failed as false
	failed = false

	return map[string]interface{}{
		"message":          "Workspace Config Updated.",
		"workspace_config": newWorkspaceConfig.ToFrontend(),
	}, nil
}

func GetWorkspaceConfig(ctx context.Context, db *ti.Database, _id int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-workspace-config")
	defer span.End()
	callerName := "GetWorkspaceConfig"

	// query for existing config
	res, err := db.QueryContext(ctx, &span, &callerName, "select * from workspace_config where _id = ? order by revision", _id)
	if err != nil {
		return nil, fmt.Errorf("failed to query for existing workspace config: %v", err)
	}

	defer res.Close()

	// create slice to hold workspace config revisions
	workspaceConfigRevisions := make([]*models.WorkspaceConfigFrontend, 0)

	// iterate cursor loading revisions and appending to the outer slice
	for res.Next() {
		// load workspace config from cursor
		workspaceConfig, err := models.WorkspaceConfigFromSQLNative(db, res)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing workspace config from cursor: %v", err)
		}

		workspaceConfigRevisions = append(workspaceConfigRevisions, workspaceConfig.ToFrontend())
	}

	// return failure if we have no revisions
	if len(workspaceConfigRevisions) == 0 {
		return map[string]interface{}{
			"message": "Workspace Config not found.",
		}, fmt.Errorf("failed to load existing workspace config from cursor: no configs found")
	}

	revisionWithTags := make([]map[string]interface{}, 0)

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open tx for query: %v", err)
	}

	defer tx.Commit(&callerName)

	for _, w := range workspaceConfigRevisions {
		finalTags := make([]string, 0)

		var tagParams []interface{}
		tagParamSlots := make([]string, 0)

		for _, t := range w.Tags {
			tagid, err := strconv.ParseInt(t, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tag id: %v", err)
			}
			tagParams = append(tagParams, tagid)
			tagParamSlots = append(tagParamSlots, "?")
		}

		if len(tagParams) > 0 {
			query := fmt.Sprintf("SELECT value FROM tag WHERE _id IN (%s)", strings.Join(tagParamSlots, ", "))
			rows, err := tx.QueryContext(ctx, &callerName, query, tagParams...)
			if err != nil {
				return nil, fmt.Errorf("failed to execute query: %v", err)
			}
			defer rows.Close()

			for rows.Next() {
				var value string
				err = rows.Scan(&value)
				if err != nil {
					return nil, fmt.Errorf("failed to scan row: %v", err)
				}

				finalTags = append(finalTags, value)
			}

			revisionWithTags = append(revisionWithTags, map[string]interface{}{"revision": w, "tags": finalTags})
		}
	}

	return map[string]interface{}{
		"workspace_config":   workspaceConfigRevisions[0],
		"revisions_and_tags": revisionWithTags,
	}, nil
}
