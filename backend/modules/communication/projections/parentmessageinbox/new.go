package projection

import postgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentinbox"

// New binds the projection constructor to its root-owned inputs.
func New(database postgres.Database, links postgres.GuardianLinks) *postgres.Projection {
	return postgres.New(database, links)
}
