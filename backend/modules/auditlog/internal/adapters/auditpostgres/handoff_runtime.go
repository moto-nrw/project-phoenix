package audit

import (
	"context"

	auditModel "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/uptrace/bun"
)

// Handoff reads what a caller's unit of work put on the context: its
// transaction, whether there is one, and its tenant.
type Handoff func(context.Context) (transaction any, ok bool, tenantID int64)

// ContextHandoff reads the Audit handoff the transaction runtime installs as
// its context adapter.
func ContextHandoff(ctx context.Context) (any, bool, int64) {
	transaction, ok := auditModel.TransactionFromContext(ctx)
	return transaction, ok, auditModel.TenantIDFromContext(ctx)
}

// NewHandoffRuntime binds the Audit repositories to the transaction that
// handoff reads. Missing or unusable transactions resolve no database, so
// repositories return an error before constructing SQL. The pool is never used.
func NewHandoffRuntime(handoff Handoff) Runtime {
	if handoff == nil {
		panic("audit repository: handoff is required")
	}
	return func(ctx context.Context) (bun.IDB, int64) {
		raw, ok, tenantID := handoff(ctx)
		if !ok {
			return nil, tenantID
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx, tenantID
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID
			}
		}
		return nil, tenantID
	}
}
