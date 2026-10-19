package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

func NewDatabase[T interface{ Int64() int64 }](transactionFromContext func(context.Context) (any, bool), tenantFromContext func(context.Context) (T, error)) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := transactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("feedback postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("feedback postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := tenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	}
}
