//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func databaseRuntime(db *bun.DB) postgres.Database { return postgres.DatabaseRuntime(db) }
