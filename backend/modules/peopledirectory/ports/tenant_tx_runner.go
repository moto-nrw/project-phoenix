package ports

import "context"

// TenantTxRunner runs fn in the tenant transaction of the context, joining
// the request transaction when one is open.
type TenantTxRunner interface {
	InTenantTx(ctx context.Context, fn func(context.Context) error) error
}
