// Package staffstore is Communication's composition seam for the OGS-internal
// staff conversation stores (#3221).
//
// It exists next to modules/communication/composition rather than inside it
// for the same reason as parentstore: the repository factory builds these
// stores, and the full composition package reaches services that in turn
// build repositories. This package depends only on the owner's own adapters,
// so that seam stays acyclic.
//
// It preserves the existing models/users repository contracts while the SQL
// lives with its owner. Once every caller reads Communication through the
// module facade, the contracts and this package go with them.
package staffstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffinbox"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// resolve returns the caller's ambient transaction and school.
//
// It is deliberately a copy of the same resolution in parentstore and
// modules/communication/composition: all three are Communication compose
// seams, and one compose package may not import another. They stay in step
// because they resolve the same two context values; if that resolution ever
// grows a rule, it has to change in every copy.
//
// It does not require a transaction: the retention sweep and notification
// producers read conversation state outside a request transaction, exactly as
// the repositories this replaces did. RLS still applies; the returned school
// only adds the defense-in-depth tenant_id filter and is zero for
// administrative work.
func resolve(ctx context.Context, db *bun.DB) (bun.IDB, int64, error) {
	if db == nil {
		return nil, 0, errors.New("communication staff store: database is required")
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
	// A transaction is in the context but is not one this adapter can run in.
	// Falling back to the plain pool would silently take the conversation
	// statements out of the caller's transaction — the append lock would be
	// released immediately and message order would stop matching commit order.
	// The legacy repositories panicked here; failing the call is the same
	// decision without taking the process down.
	return nil, 0, fmt.Errorf("communication staff store: unsupported transaction type %T", transaction)
}

func postgresDatabase(db *bun.DB) staffpostgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}

func inboxDatabase(db *bun.DB) staffinbox.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}
