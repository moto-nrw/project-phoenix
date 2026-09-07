// Package parentstore is Communication's composition seam for the parent
// conversation stores.
//
// It exists next to modules/communication/composition rather than inside it
// because the repository factory builds these stores, and the full
// composition package reaches services that in turn build repositories. This
// package deliberately depends on nothing but the owner's own adapters, so
// that seam stays acyclic.
//
// It preserves the existing repository contracts while the SQL lives with its
// owner. Once every caller reads Communication through the module facade, the
// contracts and this package go with them.
package parentstore

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentinbox"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// resolve returns the caller's ambient transaction and school.
//
// It is deliberately a copy of the same resolution in
// modules/communication/composition: both packages are Communication compose
// seams, and one compose package may not import another. The two stay in step
// because they resolve the same two context values; if that resolution ever
// grows a rule, it has to change in both places.
//
// It does not require a transaction: producers and scheduler loops read
// conversation state outside a request transaction, exactly as the
// repositories this replaces did. RLS still applies; the returned school only
// adds the defense-in-depth tenant_id filter and is zero for administrative and
// cross-tenant work.
func resolve(ctx context.Context, db *bun.DB) (bun.IDB, int64, error) {
	if db == nil {
		return nil, 0, errors.New("communication parent store: database is required")
	}
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

func postgresDatabase(db *bun.DB) parentpostgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}

func inboxDatabase(db *bun.DB) parentinbox.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}
