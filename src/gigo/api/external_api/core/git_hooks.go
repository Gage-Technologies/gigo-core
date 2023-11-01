package core

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/types"
	"github.com/sourcegraph/conc"
	"go.opentelemetry.io/otel"
)

const (
	timeCostPerLargeWord  = 450 * time.Millisecond
	timeCostPerMediumWord = 300 * time.Millisecond
	timeCostPerSmallWord  = 100 * time.Millisecond
)

var (
	tutorialDetectionRegex = regexp.MustCompile("\\.gigo\\/\\.tutorials\\/tutorial-\\d+.md")
)

func GiteaWebhookPush(ctx context.Context, db *ti.Database, vcsClient *git.VCSClient, sf *snowflake.Node, wg *conc.WaitGroup,
	logger logging.Logger, req *types.GiteaWebhookPush) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "gitea-webhook-push")
	defer span.End()

	// handle tutorial modifications
	err := handleTutorialMod(ctx, wg, db, vcsClient, logger, req)
	if err != nil {
		logger.Errorf("failed to perform tutorial mod on %q: %v", req.Repository.FullName, err)
	}

	return nil
}

// handleTutorialMod
//
//	Detect when a tutorial is modified and perform time estimation
//	updates on the post in the database
func handleTutorialMod(ctx context.Context, wg *conc.WaitGroup, db *ti.Database, vcsClient *git.VCSClient, logger logging.Logger, req *types.GiteaWebhookPush) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "gitea-webhook-push-handle-mod-tutorial")
	defer span.End()
	callingName := "handleTutorialMod"
	logger.Debugf("(git webhook) handling tutorial modification: %q", req.Repository.FullName)

	// extract the code source id from the req
	id, err := strconv.ParseInt(req.Repository.Name, 10, 64)
	if err != nil {
		return err
	}

	// check if the id is a post
	err = db.QueryRow(
		ctx, &span, &callingName,
		"select _id from post where _id = ?", id,
	).Scan(&id)
	if err != nil {
		// return early if not found
		if err == sql.ErrNoRows {
			logger.Debugf("(git webhook) repo is not a post: %q", req.Repository.FullName)
			return nil
		}
		return fmt.Errorf("failed to check for post: %w", err)
	}

	// determine if any files in the .gigo/.tutorials directory were modified
	modified := false
	for _, commit := range req.Commits {
		// iterate the files that have been added, deleted or modified and detect
		// if a tutorial has been modified
		for _, added := range commit.Added {
			if tutorialDetectionRegex.MatchString(added) {
				modified = true
				break
			}
		}

		for _, removed := range commit.Removed {
			if tutorialDetectionRegex.MatchString(removed) {
				modified = true
				break
			}
		}

		for _, m := range commit.Modified {
			if tutorialDetectionRegex.MatchString(m) {
				modified = true
				break
			}
		}
	}

	// exit quietly if nothing has been modified
	if !modified {
		logger.Debugf("(git webhook) no tutorials mod: %q", req.Repository.FullName)
		return nil
	}

	// launch a go routine to perform the estimation updates
	wg.Go(func() {
		ctx, span := otel.Tracer("gigo-core").Start(context.TODO(), "gitea-webhook-push-handle-mod-tutorial-internal")
		defer span.End()
		callingName := "handleTutorialMod-internal"
		logger.Debugf("(git webhook) launching tutorial mod internal handler: %q", req.Repository.FullName)

		// retrieve the list of tutorials in the repo's tutorial directory
		files, _, err := vcsClient.GiteaClient.ListContents(
			req.Repository.Owner.Username,
			req.Repository.Name,
			req.Ref,
			".gigo/.tutorials",
		)
		if err != nil {
			logger.Warnf("failed to get contents of tutorial dir: %v", err)
			return
		}

		// create a duration to hold the time estimates
		duration := time.Duration(0)

		// loop through each file, determine if it is a valid tutorial file, and count the time estimates
		for _, f := range files {
			// check against the tutorial detection regex
			if !tutorialDetectionRegex.MatchString(f.Path) {
				continue
			}

			// retrieve the contents of the file
			file, _, err := vcsClient.GiteaClient.GetContents(
				req.Repository.Owner.Username,
				req.Repository.Name,
				req.Ref,
				f.Path,
			)
			if err != nil {
				logger.Warnf("failed to get contents of tutorial file: %v", err)
				continue
			}

			// ensure the contents are valid UTF-8
			valid := utf8.ValidString(*file.Content)
			if !valid {
				continue
			}

			// ensure that the file size does not exceed 1MB
			if file.Size > 1024*1024 {
				continue
			}

			// decode the raw text from base64
			rawDecodedText, err := base64.StdEncoding.DecodeString(*file.Content)
			if err != nil {
				logger.Warnf("failed to decode contents: %v", err)
				continue
			}

			// check if the decoded text is binary
			if isBinary(rawDecodedText) {
				continue
			}

			// count the words in the file
			words := strings.FieldsFunc(string(rawDecodedText), func(r rune) bool {
				switch r {
				// whitespace
				case ' ', '\t', '\n':
					return true
				// code characters
				case '<', '>', '{', '}', '(', ')', '[', ']':
					return true
				// punctuation
				case '.', ',', ';', '!', '?', '"', '-', ':':
					return true
				// special characters
				case '/', '\\', '_', '#', '@', '%', '^', '&', '=', '+', '$', '~', '`', '|', '*':
					return true
				default:
					return false
				}
			})

			// multiply the word count with the average time cost per word
			for _, word := range words {
				if len(word) < 3 {
					duration += timeCostPerSmallWord
					continue
				}
				if len(word) < 12 {
					duration += timeCostPerMediumWord
					continue
				}
				duration += timeCostPerLargeWord
			}
		}

		// update the estimated time on the database
		_, err = db.Exec(
			ctx, &span, &callingName,
			"update projects set estimated_tutorial_time = ? where _id = ?",
			duration, id,
		)
		if err != nil {
			logger.Warnf("failed to update estimated time for project: %v", err)
			return
		}
	})

	return nil
}
