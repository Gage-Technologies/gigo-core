package core

import (
	"context"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/types"
	"go.opentelemetry.io/otel"
)

// TODO: needs testing

func GiteaWebhookPush(ctx context.Context, db *ti.Database, vcsClient *git.VCSClient, sf *snowflake.Node,
	req *types.GiteaWebhookPush) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "gitea-webhook-push")
	defer span.End()
	return nil
}

// func GiteaWebhookPush(db *ti.Database, vcsClient *git.VCSClient, coderClient *coder.CoderClient, sf *snowflake.Node,
// 	req *types.GiteaWebhookPush) error {
// 	// skip if the workspace file wasn't modified in anyway
// 	modified := false
// 	for _, commit := range req.Commits {
// 		for _, file := range commit.Modified {
// 			if file == ".gigo/workspace.yaml" {
// 				modified = true
// 				break
// 			}
// 		}
// 		if modified {
// 			break
// 		}
//
// 		for _, file := range commit.Added {
// 			if file == ".gigo/workspace.yaml" {
// 				modified = true
// 				break
// 			}
// 		}
// 		if modified {
// 			break
// 		}
//
// 		for _, file := range commit.Removed {
// 			if file == ".gigo/workspace.yaml" {
// 				modified = true
// 				break
// 			}
// 		}
// 		if modified {
// 			break
// 		}
// 	}
//
// 	// exit quietly if nothing has been modified
// 	if !modified {
// 		return nil
// 	}
//
// 	// generate the template update
// 	err := generateWorkspaceTemplate(
// 		db, sf, vcsClient, coderClient, req.Repository.Owner.Username, req.Repository.Name, req.After,
// 	)
// 	if err != nil {
// 		return fmt.Errorf("failed to generate workspace template: %v", err)
// 	}
//
// 	return nil
// }
