package base

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ConfigRuntime adapts the transaction runtime for config repositories.
type ConfigRuntime struct{ db *bun.DB }

func NewConfigRuntime(db *bun.DB) ConfigRuntime { return ConfigRuntime{db: db} }

func (r ConfigRuntime) DB(ctx context.Context) bun.IDB     { return GetDB(ctx, r.db) }
func (r ConfigRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }
func (r ConfigRuntime) LockStaffBalance(ctx context.Context, staffID int64) error {
	return AcquireStaffBalanceLock(ctx, r.db, staffID)
}
func (r ConfigRuntime) Today() timezone.Date { return timezone.TodayDate() }
