package projection

import postgres "github.com/moto-nrw/project-phoenix/modules/staffmessageinbox/internal/adapters/postgres"

// New binds the projection constructor to its root-owned inputs.
func New(database postgres.Database) *postgres.Projection { return postgres.New(database) }
