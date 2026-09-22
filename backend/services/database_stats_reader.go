package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/services/database"
)

type DatabaseStatsReader func(context.Context) (*database.StatsResponse, error)

func NewDatabaseStatsReader(service database.DatabaseService, capabilities func(context.Context) database.StatsCapabilities) DatabaseStatsReader {
	return func(ctx context.Context) (*database.StatsResponse, error) {
		return service.GetStats(ctx, capabilities(ctx))
	}
}

// DatabaseStatsPermissions lists every permission the stats capabilities
// honour; the stats route opens for any of them (#3469).
func DatabaseStatsPermissions() []string {
	return securityruntime.DatabaseStatsPermissions()
}
