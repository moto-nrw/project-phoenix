// Package parentstore composes Communication's native parent-conversation
// stores and audience projections. Consumer-owned ports receive the other
// owners' public facts from the root; SQL stays in the existing owned adapters.
package parentstore

import (
	"context"
	"errors"

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

// GuardianLinkRows is the student-guardian relationship row source the
// inbox and audience projections join in their own statements; the roots
// bind the guardian-link projection (ADR 0058).
type GuardianLinkRows interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
}

func postgresDatabase(db *bun.DB) parentpostgres.Database {
	return func(ctx context.Context) (bun.IDB, parentpostgres.TenantScope, error) {
		connection, schoolID, err := resolve(ctx, db)
		scope := parentpostgres.ScopeFor(schoolID, tenant.IsAdminTx(ctx))
		if err != nil {
			return nil, scope, err
		}
		return connection, scope, scope.Validate()
	}
}

// ProjectionDatabase exposes this store's existing ambient runtime for root binding.
func ProjectionDatabase(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}
