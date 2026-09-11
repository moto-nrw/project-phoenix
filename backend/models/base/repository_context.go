package base

import "context"

type repositoryTenantIDKey struct{}
type repositoryTransactionKey struct{}

// WithRepositoryContext is the persistence-neutral handoff from the
// transaction adapter to generic repositories. The adapter remains
// responsible for validating the concrete transaction type.
func WithRepositoryContext(ctx context.Context, tenantID int64, transaction any) context.Context {
	ctx = WithRepositoryTenantID(ctx, tenantID)
	return context.WithValue(ctx, repositoryTransactionKey{}, transaction)
}

func WithRepositoryTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, repositoryTenantIDKey{}, tenantID)
}

func WithoutRepositoryTransaction(ctx context.Context) context.Context {
	return context.WithValue(ctx, repositoryTransactionKey{}, nil)
}

func RepositoryTenantID(ctx context.Context) int64 {
	id, _ := ctx.Value(repositoryTenantIDKey{}).(int64)
	return id
}

func RepositoryTransaction(ctx context.Context) (any, bool) {
	transaction := ctx.Value(repositoryTransactionKey{})
	return transaction, transaction != nil
}
