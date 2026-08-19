package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tablePWAStandaloneUsage = "iot.pwa_standalone_usage"

// PWAStandaloneUsageRepository implements iot.PWAStandaloneUsageRepository.
type PWAStandaloneUsageRepository struct {
	*base.Repository[*iot.PWAStandaloneUsage]
}

// NewPWAStandaloneUsageRepository creates a new PWAStandaloneUsageRepository.
func NewPWAStandaloneUsageRepository(db *bun.DB) iot.PWAStandaloneUsageRepository {
	repo := base.NewRepository[*iot.PWAStandaloneUsage](db, tablePWAStandaloneUsage, "PwaStandaloneUsage")
	repo.TenantScoped = true
	return &PWAStandaloneUsageRepository{Repository: repo}
}

// RecordSeen inserts or refreshes a usage row keyed by
// (tenant_id, account_id, portal). Repeated reports from the same session
// only advance last_seen_at — the write is idempotent by design.
func (r *PWAStandaloneUsageRepository) RecordSeen(ctx context.Context, usage *iot.PWAStandaloneUsage) error {
	base.EnsureTenantID(ctx, usage)
	_, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(usage).
		ModelTableExpr(tablePWAStandaloneUsage).
		On("CONFLICT (tenant_id, account_id, portal) DO UPDATE").
		Set("last_seen_at = NOW()").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "record pwa standalone usage", Err: err}
	}
	return nil
}

// DeleteLastSeenBefore removes rows whose last_seen_at is before cutoff and
// returns the number of deleted rows. Scoped to the current tenant when one
// is set on the context (the GDPR cleanup job runs per tenant). Named apart
// from the generic DeleteOlderThan, which compares a DATE column via
// timezone.Date — last_seen_at is a TIMESTAMPTZ instant.
func (r *PWAStandaloneUsageRepository) DeleteLastSeenBefore(ctx context.Context, cutoff time.Time) (int, error) {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*iot.PWAStandaloneUsage)(nil)).
		ModelTableExpr(tablePWAStandaloneUsage).
		Where("last_seen_at < ?", cutoff)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete expired pwa standalone usage", Err: err}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count deleted pwa standalone usage", Err: err}
	}
	return int(affected), nil
}
