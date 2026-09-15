package audit

import "context"

type tenantIDKey struct{}
type transactionKey struct{}

// WithTenantID is the Audit adapter's persistence-neutral tenant handoff.
func WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

func TenantIDFromContext(ctx context.Context) int64 {
	tenantID, _ := ctx.Value(tenantIDKey{}).(int64)
	return tenantID
}

func WithTransaction(ctx context.Context, transaction any) context.Context {
	return context.WithValue(ctx, transactionKey{}, transaction)
}

func TransactionFromContext(ctx context.Context) (any, bool) {
	transaction := ctx.Value(transactionKey{})
	return transaction, transaction != nil
}
