package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// parentDatabase resolves the caller's ambient transaction and tenant for the
// tenant-scoped parent stores.
//
// It is deliberately a copy of the same resolution in
// modules/communication/parentstore: both packages are Communication compose
// seams, and one compose package may not import another. The two stay in step
// because they resolve the same two context values; if that resolution ever
// grows a rule, it has to change in both places.
//
// Unlike the platform-announcement store it does NOT require a transaction:
// producers and scheduler loops read consent and message state outside a
// request transaction, exactly as the repositories this replaces did when
// no ambient transaction was installed. RLS still applies; the returned tenant
// only adds the defense-in-depth tenant_id filter, and is zero for
// administrative and cross-tenant work.
func parentDatabase(db *bun.DB) parentpostgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db, tenantID, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
		}
		return db, tenantID, nil
	}
}
