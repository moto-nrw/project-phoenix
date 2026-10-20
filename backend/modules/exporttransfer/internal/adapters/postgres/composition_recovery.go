package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func NewRecoveryDatabase(current func(context.Context) (any, bool)) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := current(ctx)
		if !ok {
			return nil, 0, fmt.Errorf("export transfer recovery requires an administrative transaction")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("unsupported export transfer recovery transaction %T", transaction)
		}
		return tx, 0, nil
	}
}
