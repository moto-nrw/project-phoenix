package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	configRepo "github.com/moto-nrw/project-phoenix/modules/settings/internal/adapters/configpostgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewAuditRetention composes the setting protocol retention of the nightly
// cleanup (#4044). It runs on the tenant transaction of the school in
// context and refuses to run without one instead of falling back to a pool.
func NewAuditRetention() settings.AuditRetention {
	return auditRetention{now: time.Now}
}

type auditRetention struct{ now func() time.Time }

func (r auditRetention) DeleteExpiredSettingChanges(ctx context.Context) (int, int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("settings retention: tenant is required: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return 0, 0, errors.New("settings retention: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return 0, 0, fmt.Errorf("settings retention: unsupported transaction %T", transaction)
	}
	now := r.now()
	cutoff := now.AddDate(-settings.SettingAuditRetentionYears, 0, 0)
	deleted, err := configRepo.NewSettingAuditRetention(transactionRuntime{tx: tx}).DeleteChangedBefore(ctx, tenantID.Int64(), cutoff)
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(now.Sub(cutoff).Hours() / 24), nil
}

// transactionRuntime is the settings runtime of one tenant transaction.
type transactionRuntime struct{ tx bun.Tx }

func (r transactionRuntime) DB(context.Context) (bun.IDB, error) { return r.tx, nil }
