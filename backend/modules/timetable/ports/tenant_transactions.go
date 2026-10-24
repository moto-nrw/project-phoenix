package ports

import (
	"context"
)

// TenantTransactions runs a callback in the tenant transaction of tenantID.
// The template, materialization and series conversion services take it
// instead of the database handle; the composition binds it to the tenant
// runtime.
type TenantTransactions interface {
	InTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
}

// TransactionLocks takes a transaction-scoped advisory lock by key.
type TransactionLocks interface {
	AcquireTransactionLock(ctx context.Context, key string) error
}

// TenantDatabase is what the instance lifecycle needs from the database: the
// tenant transaction and the transaction-scoped advisory locks.
type TenantDatabase interface {
	TenantTransactions
	TransactionLocks
}
