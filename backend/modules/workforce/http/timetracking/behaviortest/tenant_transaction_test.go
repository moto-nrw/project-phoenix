package behaviortest

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/workforce/http/timetracking"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// testWithinTenant runs the routes' tenant transactions over db.
func testWithinTenant(db *bun.DB) timetracking.TenantTransaction {
	return func(ctx context.Context, tenantID int64, run func(context.Context) error) error {
		return tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return run(txCtx) })
	}
}
