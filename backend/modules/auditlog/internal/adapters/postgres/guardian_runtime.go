package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

func NewGuardianTransaction[ID interface{ Int64() int64 }](school func(context.Context) (ID, error), resolve func(context.Context) (any, bool)) func(context.Context) (bun.IDB, int64, error) {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := school(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("audit guardian changes: tenant is required: %w", err)
		}
		transaction, found := resolve(ctx)
		if !found {
			return nil, 0, errors.New("audit guardian changes: tenant transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return *tx, tenantID.Int64(), nil
			}
		}
		return nil, 0, fmt.Errorf("audit guardian changes: unsupported transaction %T", transaction)
	}

}
