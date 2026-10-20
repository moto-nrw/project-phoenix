package postgres

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

func NewDatabase(current func(context.Context) (any, bool)) Database {
	return func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := current(ctx)
		if !ok {
			return nil, errors.New("request feed compose: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return tx, nil
			}
		}
		return nil, errors.New("request feed compose: unsupported transaction")
	}
}
