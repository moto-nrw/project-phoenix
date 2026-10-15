package audit

import (
	"context"
	"fmt"

	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
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
// handoff reads. Outside a transaction it resolves fallback; without one it
// resolves no database, so every write fails. A transaction of another type
// is a wiring error.
func NewHandoffRuntime(handoff Handoff, fallback *bun.DB) Runtime {
	if handoff == nil {
		panic("audit repository: handoff is required")
	}
	return func(ctx context.Context) (bun.IDB, int64) {
		raw, ok, tenantID := handoff(ctx)
		if !ok {
			if fallback == nil {
				return nil, tenantID
			}
			return fallback, tenantID
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx, tenantID
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID
			}
		}
		panic(fmt.Sprintf("audit command: unsupported transaction %T", raw))
	}
}
