package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

func NewAnnouncementDatabase(resolve func(context.Context) (any, bool)) func(context.Context) (bun.IDB, error) {
	return func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := resolve(ctx)
		if !ok {
			return nil, errors.New("communication postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("communication postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	}

}
