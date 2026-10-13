package transactionruntime

import "github.com/moto-nrw/project-phoenix/services/database"

// The dashboard's database statistics (#2747): the counts it reads through
// the owners' capabilities and the permission flags the caller holds. The
// legacy composition binds the counters through these names instead of the
// application package.
type (
	DatabaseStats             = database.DatabaseService
	DatabaseStatsDependencies = database.StatsDependencies
	DatabaseStatsCapabilities = database.StatsCapabilities
	DatabaseStatsResponse     = database.StatsResponse
)

// NewDatabaseStats builds the statistics service over its counters.
var NewDatabaseStats = database.NewService
