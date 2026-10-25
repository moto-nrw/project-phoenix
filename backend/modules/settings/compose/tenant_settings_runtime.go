package compose

import (
	"context"
	"time"

	app "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantSettingsRuntime binds Settings consumers to the required ambient unit.
// It provides school/admin boundaries, locks and commit hooks; unit optionally
// binds them to the root's unit of work (#2747).
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
	return app.SettingTransactionPresent(ctx, tenant.TransactionFromContext)
}

func (r TenantSettingsRuntime) DB(ctx context.Context) (bun.IDB, error) {
	return tenant.CurrentTransaction[bun.IDB](ctx)
}

func (r TenantSettingsRuntime) LockStaffBalance(ctx context.Context, staffID int64) error {
	return app.LockSettingStaffBalance(ctx, staffID, r.TenantID, r.AcquireLock)
}

func (TenantSettingsRuntime) TodayTime() time.Time { return timezone.TodayDate().UTCMidnight() }

func (r TenantSettingsRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(app.AttachSettingUnit(ctx, r.unit, tenant.WithUnitOfWork), r.db, tenantID, app.SettingContextCallback[bun.Tx](fn))
}

func (r TenantSettingsRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithAdminTx(app.AttachSettingUnit(ctx, r.unit, tenant.WithUnitOfWork), r.db, app.SettingContextCallback[bun.Tx](fn))
}

// WithinTraeger opens the Träger transaction of ADR 0086 section 5.
func (r TenantSettingsRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(app.AttachSettingUnit(ctx, r.unit, tenant.WithUnitOfWork), organizationID, fn)
}

func (r TenantSettingsRuntime) AcquireLock(ctx context.Context, key string, shared bool) error {
	return tenant.AcquireLock(app.AttachSettingUnit(ctx, r.unit, tenant.WithUnitOfWork), key, shared)
}

func (r TenantSettingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}
