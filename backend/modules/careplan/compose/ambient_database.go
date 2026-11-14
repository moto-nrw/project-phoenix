package compose

import (
	postgresNative "github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func TenantAmbientDatabase(db *bun.DB) AmbientDatabase {
	return postgresNative.TenantAmbientDatabase(db)
}
