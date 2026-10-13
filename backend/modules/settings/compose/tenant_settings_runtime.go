package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantSettingsRuntime is the tenant runtime the settings service, its
// persistence and the legacy composition's settings consumers run in: the
// tenant of the context, its transaction or the root database, the tenant
// and administrative transactions, advisory locks and commit hooks. unit,
// when set, is the unit of work the transactions attach (#2747).
type TenantSettingsRuntime struct {
	db   *bun.DB
	unit *tenant.UnitOfWork
}

// NewTenantSettingsRuntime binds the runtime to the root database; a nil
// unit leaves the transactions on the unit of work of the context.
func NewTenantSettingsRuntime(db *bun.DB, unit *tenant.UnitOfWork) TenantSettingsRuntime {
	return TenantSettingsRuntime{db: db, unit: unit}
}

func (r TenantSettingsRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (r TenantSettingsRuntime) HasTransaction(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

func (r TenantSettingsRuntime) DB(ctx context.Context) bun.IDB {
	if raw, ok := tenant.TransactionFromContext(ctx); ok {
		if tx, valid := raw.(bun.Tx); valid {
			return tx
		}
	}
	return r.db
}

func (r TenantSettingsRuntime) LockStaffBalance(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("staff id is required")
	}
	tenantID := r.TenantID(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if err := r.AcquireLock(ctx, fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID), false); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
}

func (TenantSettingsRuntime) TodayTime() time.Time { return timezone.TodayDate().UTCMidnight() }

func (r TenantSettingsRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	if r.unit != nil {
		ctx = tenant.WithUnitOfWork(ctx, *r.unit)
	}
	return tenant.WithTenantTx(ctx, r.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (r TenantSettingsRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	if r.unit != nil {
		ctx = tenant.WithUnitOfWork(ctx, *r.unit)
	}
	return tenant.WithAdminTx(ctx, r.db, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (r TenantSettingsRuntime) AcquireLock(ctx context.Context, key string, shared bool) error {
	if r.unit != nil {
		ctx = tenant.WithUnitOfWork(ctx, *r.unit)
	}
	return tenant.AcquireLock(ctx, key, shared)
}

func (r TenantSettingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}
