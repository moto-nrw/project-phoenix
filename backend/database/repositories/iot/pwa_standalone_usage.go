package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// PWAStandaloneUsageRepository persists the PWA usage signal through the
// table owner's database functions. Callers never receive its storage model.
type PWAStandaloneUsageRepository struct {
	db *bun.DB
}

func NewPWAStandaloneUsageRepository(db *bun.DB) *PWAStandaloneUsageRepository {
	return &PWAStandaloneUsageRepository{db: db}
}

func (r *PWAStandaloneUsageRepository) RecordSeen(ctx context.Context, tenantID, accountID int64, portal string) error {
	_, err := base.GetDB(ctx, r.db).NewRaw(
		"SELECT iot.record_pwa_standalone_usage(?, ?, ?)",
		tenantID, accountID, portal,
	).Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "record pwa standalone usage", Err: err}
	}
	return nil
}

func (r *PWAStandaloneUsageRepository) DeleteLastSeenBefore(ctx context.Context, tenantID int64, cutoff time.Time) (int, error) {
	var affected int
	err := base.GetDB(ctx, r.db).NewRaw(
		"SELECT iot.delete_pwa_standalone_usage_before(?, ?)",
		tenantID, cutoff,
	).Scan(ctx, &affected)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete expired pwa standalone usage", Err: err}
	}
	return affected, nil
}
