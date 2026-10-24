package compose

import (
	communicationPostgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// InstrumentMessageQueries attaches the message facade's query hook.
// For a shared handle, it returns a private clone over the same SQL pool.
// Otherwise it instruments the input handle before the root starts using it.
func InstrumentMessageQueries(db *bun.DB, shared bool) *bun.DB {
	return communicationPostgres.InstrumentMessageQueries(db, shared)
}
