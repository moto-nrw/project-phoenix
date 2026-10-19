package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

func NewActiveStudentCounts(transaction func(context.Context) (any, bool), admin func(context.Context) bool) *activeStudentCounts {
	return &activeStudentCounts{transaction: transaction, admin: admin}
}

type activeStudentCounts struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

func (c activeStudentCounts) CountActiveStudentsByTenant(ctx context.Context, capturedAt time.Time) (map[int64]int, error) {
	transaction, ok := c.transaction(ctx)
	if !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	if !c.admin(ctx) {
		return nil, errors.New("school membership: active student counts need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return CountActiveStudentsByTenant(ctx, tx, capturedAt)
	case *bun.Tx:
		if tx != nil {
			return CountActiveStudentsByTenant(ctx, tx, capturedAt)
		}
	}
	return nil, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
}
