package transactionruntime

import (
	"log/slog"

	statsApplication "github.com/moto-ogs/project-phoenix/modules/transactionruntime/internal/application/stats"
	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/stats"
)

// NewDatabaseStats builds the statistics service over its counters.
func NewDatabaseStats(deps stats.StatsDependencies, logger *slog.Logger) stats.Service {
	return statsApplication.NewService(deps, logger)
}
