package postgres

import (
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// ResolveDatabase accepts only the ambient SQL unit of one required school.
func ResolveDatabase(raw any, present bool, schoolID int64) (bun.IDB, int64, error) {
	if schoolID <= 0 {
		return nil, 0, errors.New("care plan: tenant is required")
	}
	if !present {
		return nil, 0, errors.New("care plan: transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, schoolID, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, schoolID, nil
		}
		return nil, 0, errors.New("care plan: transaction is required")
	default:
		return nil, 0, fmt.Errorf("care plan postgres: unsupported transaction %T", raw)
	}
}
