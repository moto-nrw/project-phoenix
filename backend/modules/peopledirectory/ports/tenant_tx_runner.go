package ports

import "context"

// TenantTxRunner runs fn in the tenant transaction of the context, joining
// the request transaction when one is open.
type TenantTxRunner interface {
	InTenantTx(ctx context.Context, fn func(context.Context) error) error
}

// StaffDirectoryRuntime supplies the tenant transaction and advisory lock that
// order staff creation with the reviewed staff-person merge workflow.
type StaffDirectoryRuntime interface {
	TenantTxRunner
	RequireTenantID(ctx context.Context) (int64, error)
	AcquireLock(ctx context.Context, key string, shared bool) error
}
