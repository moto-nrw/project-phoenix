package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

func NewChildQuotaCounts(today func() string, transaction func(context.Context) (any, bool), admin func(context.Context) bool) *childQuotaCounts {
	return &childQuotaCounts{today: today, transaction: transaction, admin: admin}
}

type childQuotaCounts struct {
	today       func() string
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

func (c childQuotaCounts) CountChildQuotaByTenant(ctx context.Context) (map[int64]int, error) {
	transaction, ok := c.transaction(ctx)
	if !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	if !c.admin(ctx) {
		return nil, errors.New("school membership: child quota counts need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return CountChildQuotaByTenant(ctx, tx, c.today())
	case *bun.Tx:
		if tx != nil {
			return CountChildQuotaByTenant(ctx, tx, c.today())
		}
	}
	return nil, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
}
