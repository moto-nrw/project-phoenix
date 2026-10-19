package tenant

import (
	"context"
	"fmt"
	"reflect"
)

// CurrentTransaction returns a driver's view of the ambient transaction, never
// a connection pool. Driver types stay with the owner composition, not here.
func CurrentTransaction[T any](ctx context.Context) (T, error) {
	var zero T
	if err := RequireTransaction(ctx); err != nil {
		return zero, err
	}
	raw, _ := TransactionFromContext(ctx)
	_, transaction := raw.(interface {
		Commit() error
		Rollback() error
	})
	value := reflect.ValueOf(raw)
	if !transaction || (value.Kind() == reflect.Pointer && value.IsNil()) {
		return zero, ErrTransactionRequired
	}
	typed, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("tenant: unsupported transaction %T", raw)
	}
	return typed, nil
}

// CurrentTenantTransaction also requires a validated school. School-owned
// adapters cannot select a school-less administrative transaction.
func CurrentTenantTransaction[T any](ctx context.Context) (T, int64, error) {
	var zero T
	id, err := TenantFromContext(ctx)
	if err != nil {
		return zero, 0, err
	}
	tx, err := CurrentTransaction[T](ctx)
	return tx, id.Int64(), err
}
