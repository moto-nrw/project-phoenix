package compose

import (
	communicationPostgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// InstallMessageQueryInstrumentation registers the query hook used by the
// public message facade. Repeated registration does not double-count a query,
// but the application composition root normally calls this once per database.
func InstallMessageQueryInstrumentation(db *bun.DB) {
	communicationPostgres.InstallMessageQueryInstrumentation(db)
}
