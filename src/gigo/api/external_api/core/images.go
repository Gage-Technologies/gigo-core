package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/storage"
	utils2 "github.com/gage-technologies/gigo-lib/utils"
	"go.opentelemetry.io/otel"
	"io"
	"net/http"
)

func SiteImages(ctx context.Context, callingUser *models.User, tidb *ti.Database, id int64, username string, post bool, attempt bool, storageEngine storage.Storage) (io.ReadCloser, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "site-images-core")
	callerName := "SiteImages"

	path := ""
	if post {
		// throw an error if no id is provided
		if id == 0 {
			return nil, fmt.Errorf("id is required")
		}

		// query for the post to validate its availability
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select author_id, published from post where _id = ? limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query post: %v", err)
		}

		defer res.Close()

		// check if post was found with given id
		if res == nil || !res.Next() {
			return nil, fmt.Errorf("not found")
		}

		// create variables to hold values from cursor
		var authorID int64
		var published bool

		// attempt to decode res into variables
		err = res.Scan(&authorID, &published)
		if err != nil {
			return nil, fmt.Errorf("failed to scan values from cursor: %v", err)
		}

		if published != true && (callingUser == nil || authorID != callingUser.ID) {
			return nil, fmt.Errorf("not found")
		}

		// write thumbnail to final location
		idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
		if err != nil {
			return nil, fmt.Errorf("failed to hash post id: %v", err)
		}

		path = fmt.Sprintf("post/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash)
	} else if attempt {
		// throw an error if no id is provided
		if id == 0 {
			return nil, fmt.Errorf("id is required")
		}

		// query for the post to validate its availability
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select author_id, closed from attempt where _id = ? limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query post: %v", err)
		}

		defer res.Close()

		// check if post was found with given id
		if res == nil || !res.Next() {
			return nil, fmt.Errorf("not found")
		}

		// create variables to hold values from cursor
		var authorID int64
		var closed bool

		// attempt to decode res into variables
		err = res.Scan(&authorID, &closed)
		if err != nil {
			return nil, fmt.Errorf("failed to scan values from cursor: %v", err)
		}

		if closed != true && (callingUser == nil || authorID != callingUser.ID) {
			return nil, fmt.Errorf("not found")
		}

		// write thumbnail to final location
		idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
		if err != nil {
			return nil, fmt.Errorf("failed to hash post id: %v", err)
		}

		path = fmt.Sprintf("attempt/%s/%s/%s/thumbnail.jpg", idHash[:3], idHash[3:6], idHash)
	} else {
		// if the username is provided retrieve the user's id from the database
		if username != "" {
			err := tidb.QueryRowContext(ctx, &span, &callerName, "select _id from user where user_name = ? limit 1", username).Scan(&id)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, fmt.Errorf("not found")
				}
				return nil, fmt.Errorf("failed to query user: %v", err)
			}
		}

		// write thumbnail to final location
		idHash, err := utils2.HashData([]byte(fmt.Sprintf("%d", id)))
		if err != nil {
			return nil, fmt.Errorf("failed to hash post id: %v", err)
		}

		path = fmt.Sprintf("user/%s/%s/%s/profile-pic.svg", idHash[:3], idHash[3:6], idHash)
	}

	// get temp thumbnail file from storage
	thumbnailTempFile, err := storageEngine.GetFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnail file from temp path: %v", err)
	}
	// defer thumbnailTempFile.Close()

	return thumbnailTempFile, nil
}

func GitImages(ctx context.Context, callingUser *models.User, tidb *ti.Database, id int64, post bool, path string, vcsClient *git.VCSClient) ([]byte, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "git-images-core")
	callerName := "GitImages"

	// create value to hold author
	var authorId int64

	if post {
		// query for post using the id
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select author_id, published from post where _id = ? limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query post: %v", err)
		}

		defer res.Close()

		// check if post was found with given id
		if res == nil || !res.Next() {
			return nil, fmt.Errorf("not found")
		}

		// create variables to hold values from cursor
		var published bool

		// attempt to decode res into variables
		err = res.Scan(&authorId, &published)
		if err != nil {
			return nil, fmt.Errorf("failed to scan values from cursor: %v", err)
		}

		if authorId != callingUser.ID && published != true {
			return nil, fmt.Errorf("not found")
		}
	} else {
		// query for all active projects for specified user
		res, err := tidb.QueryContext(ctx, &span, &callerName, "select author_id from attempt where _id = ? limit 1", id)
		if err != nil {
			return nil, fmt.Errorf("failed to query attempt: %v", err)
		}

		defer res.Close()

		// check if attempt was found with given id
		if res == nil || !res.Next() {
			return nil, fmt.Errorf("not found")
		}

		// attempt to decode res into variables
		err = res.Scan(&authorId)
		if err != nil {
			return nil, fmt.Errorf("failed to scan values from cursor: %v", err)
		}
	}

	// retrieve file from git
	imgBytes, gitRes, err := vcsClient.GiteaClient.GetFile(
		fmt.Sprintf("%d", authorId),
		fmt.Sprintf("%d", id),
		"main",
		path,
	)
	if err != nil {
		if gitRes.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("failed to get file: %v", err)
	}

	return imgBytes, nil
}

func GetGeneratedImage(callingUser *models.User, imageId int64, storageEngine storage.Storage) (io.ReadCloser, error) {
	if callingUser == nil {
		return nil, fmt.Errorf("calling user is nil")
	}

	// create generated image path
	path := fmt.Sprintf("temp_proj_images/%v/%v.jpg", callingUser.ID, imageId)

	// get generated image from storage
	generatedTempFile, err := storageEngine.GetFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get temp file for generated project image: %v", err)
	}

	if generatedTempFile == nil {
		return nil, fmt.Errorf("failed to get temp file for generated project image")
	}

	return generatedTempFile, nil
}
