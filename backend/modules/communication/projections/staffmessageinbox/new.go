package projection

import postgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffinbox"

// New binds the projection constructor to its root-owned inputs.
func New(database postgres.Database) *postgres.Projection { return postgres.New(database) }
