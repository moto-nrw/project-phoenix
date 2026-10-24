package postgres

import (
	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

func DatabaseRuntime(db *bun.DB) Database {
	return tenant.CurrentTenantTransaction[bun.IDB]
}
