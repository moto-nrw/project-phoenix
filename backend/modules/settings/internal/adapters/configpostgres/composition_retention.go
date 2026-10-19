package config

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type settingAuditTransaction struct{ tx bun.Tx }

func (r settingAuditTransaction) DB(context.Context) (bun.IDB, error) { return r.tx, nil }
func DeleteComposedSettingAudit[I interface{ Int64() int64 }](ctx context.Context, clock func() time.Time, years int, school func(context.Context) (I, error), current func(context.Context) (any, bool)) (int, int, error) {
	tenantID, err := school(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("settings retention: tenant is required: %w", err)
	}
	transaction, ok := current(ctx)
	if !ok {
		return 0, 0, errors.New("settings retention: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return 0, 0, fmt.Errorf("settings retention: unsupported transaction %T", transaction)
	}
	now := clock()
	cutoff := now.AddDate(-years, 0, 0)
	deleted, err := NewSettingAuditRetention(settingAuditTransaction{tx: tx}).DeleteChangedBefore(ctx, tenantID.Int64(), cutoff)
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(now.Sub(cutoff).Hours() / 24), nil
}
