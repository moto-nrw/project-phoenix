package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

func NewAmbientDatabase(resolve func(context.Context) (any, bool)) func(context.Context) (bun.IDB, error) {
	return func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := resolve(ctx)
		if !ok {
			return nil, errors.New("school structure postgres: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return tx, nil
			}
			return nil, errors.New("school structure postgres: transaction is required")
		default:
			return nil, fmt.Errorf("school structure postgres: unsupported transaction %T", transaction)
		}

	}

}
