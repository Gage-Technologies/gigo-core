package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"go.opentelemetry.io/otel"
	"time"
)

type UpdateConnectionTimesOptions struct {
	DB                *ti.Database
	AgentId           int64
	FirstConnect      time.Time
	LastConnect       *time.Time
	LastDisconnect    *time.Time
	LastConnectedNode int64
}

func UpdateConnectionTimes(ctx context.Context, opts UpdateConnectionTimesOptions) func() error {
	return func() error {
		ctx, span := otel.Tracer("gigo-core").Start(ctx, "update-connection-times-core")
		callerName := "UpdateConnectionTimes"

		// perform updated on agent
		_, err := opts.DB.ExecContext(ctx, &span, &callerName,
			"update workspace_agent set first_connect = ?, last_connect = ?, last_disconnect = ?, updated_at = ?, last_connected_node = ? where _id = ?",
			opts.FirstConnect, opts.LastConnect, opts.LastDisconnect, time.Now(), opts.LastConnectedNode, opts.AgentId,
		)
		if err != nil {
			return fmt.Errorf("failed to update agent connection details: %v", err)
		}
		return nil
	}
}
