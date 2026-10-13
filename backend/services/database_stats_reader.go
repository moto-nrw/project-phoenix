package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

type DatabaseStatsReader func(context.Context) (*transactionruntime.DatabaseStatsResponse, error)

func NewDatabaseStatsReader(service transactionruntime.DatabaseStats, capabilities func(context.Context) transactionruntime.DatabaseStatsCapabilities) DatabaseStatsReader {
	return func(ctx context.Context) (*transactionruntime.DatabaseStatsResponse, error) {
		return service.GetStats(ctx, capabilities(ctx))
	}
}

// DatabaseStatsPermissions lists every permission the stats capabilities
// honour; the stats route opens for any of them (#3469).
func DatabaseStatsPermissions() []string {
	return securityruntime.DatabaseStatsPermissions()
}
