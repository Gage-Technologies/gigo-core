package core

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"go.opentelemetry.io/otel"
)

type RecordWebUsageParams struct {
	UserID    *int64                  `json:"user_id"`
	IP        string                  `json:"ip"`
	Host      string                  `json:"host" validate:"required"`
	Event     models.WebTrackingEvent `json:"event" validate:"required"`
	Timestamp time.Time               `json:"timestamp" validate:"required"`
	TimeSpent *time.Duration          `json:"timespent"`
	Path      string                  `json:"path" validate:"required"`
	Latitude  *float64                `json:"latitude"`
	Longitude *float64                `json:"longitude"`
	Metadata  map[string]interface{}  `json:"metadata"`
}

func RecordWebUsage(ctx context.Context, db *ti.Database, sf *snowflake.Node, params *RecordWebUsageParams) (*models.WebTracking, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "record-web-usage")
	defer span.End()
	callerName := "RecordWebUsage"

	// parse the ip into a net IP
	ip := net.ParseIP(params.IP)
	if ip == nil {
		return nil, fmt.Errorf("could not parse ip address: %q", params.IP)
	}

	// create a new web usage
	usage := models.CreateWebTracking(
		sf.Generate().Int64(),
		params.UserID,
		ip,
		params.Host,
		params.Event,
		params.Timestamp,
		params.TimeSpent,
		params.Path,
		params.Latitude,
		params.Longitude,
		params.Metadata,
	)

	// save the web usage
	statements, err := usage.ToSqlNative()
	if err != nil {
		return nil, fmt.Errorf("failed to load insert statement for new web usage creation: %v", err)
	}

	// open transaction for insertion
	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open insertion transaction while creating new web usage: %v", err)
	}
	defer tx.Rollback()

	// execute insert for usage
	for _, statement := range statements {
		_, err = tx.Exec(&callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert new web usage: %v", err)
		}
	}

	// commit changes and close transaction
	err = tx.Commit(&callerName)
	if err != nil {
		return nil, fmt.Errorf("failed to commit insertion transaction while creating new web usage: %v", err)
	}

	return usage, nil
}
