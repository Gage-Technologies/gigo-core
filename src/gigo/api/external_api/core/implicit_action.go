package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"time"
)

func RecordImplicitAction(ctx context.Context, tidb *ti.Database, sf *snowflake.Node, callingUser *models.User, postId int64, sessionId uuid.UUID, action models.ImplicitAction) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "record-implicit-action-core")
	defer span.End()
	callerName := "RecordImplicitAction"
	impl := models.CreateImplicitRec(sf.Generate().Int64(), callingUser.ID, postId, sessionId, action, time.Now(), callingUser.Tier)
	statement := impl.ToSQLNative()
	_, err := tidb.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
	if err != nil {
		return fmt.Errorf("failed to insert implicit action, err: %v", err)
	}
	return nil
}
