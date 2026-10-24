//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func database(db *bun.DB) postgres.Database { return postgres.DatabaseBinding(db) }
