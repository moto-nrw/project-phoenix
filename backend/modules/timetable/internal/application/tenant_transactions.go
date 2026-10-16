package application

import "context"

// TenantTransactions runs a callback in the tenant transaction of tenantID.
// The template, materialization and series conversion services take it
// instead of the database handle; the composition binds it to the tenant
// runtime.
type TenantTransactions interface {
	InTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
}

// inTenant runs fn in the tenant transaction of tenantID, the way the
// services called the tenant runtime with their database handle before the
// port existed.
func inTenant(ctx context.Context, transactions TenantTransactions, tenantID int64, fn func(context.Context) error) error {
	return transactions.InTenant(ctx, tenantID, fn)
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
