package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewDatabase() func(ctx context.Context) (bun.IDB, error) {
	return func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("enrollment postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("enrollment postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	}
}
