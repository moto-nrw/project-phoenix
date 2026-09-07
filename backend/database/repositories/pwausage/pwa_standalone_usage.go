// Package pwausage persists the PWA standalone usage signal. The table
// iot.pwa_standalone_usage belongs to Observability, so this adapter lives
// beside that owner rather than inside the Device Fleet packages (#2676).
package pwausage

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// PWAStandaloneUsageRepository persists the PWA usage signal through the
// table owner's database functions. Callers never receive its storage model.
type PWAStandaloneUsageRepository struct {
	db *bun.DB
}

// NewPWAStandaloneUsageRepository builds the usage adapter.
func NewPWAStandaloneUsageRepository(db *bun.DB) *PWAStandaloneUsageRepository {
	return &PWAStandaloneUsageRepository{db: db}
}

// database resolves the ambient transaction, falling back to the pool.
func (r *PWAStandaloneUsageRepository) database(ctx context.Context) (bun.IDB, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return r.db, nil
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
		return r.db, nil
	default:
		return nil, fmt.Errorf("pwa usage: unsupported transaction %T", raw)
	}
}

// RecordSeen marks one account as having opened its portal as a standalone PWA.
func (r *PWAStandaloneUsageRepository) RecordSeen(ctx context.Context, tenantID, accountID int64, portal string) error {
	db, err := r.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(
		"SELECT iot.record_pwa_standalone_usage(?, ?, ?)",
		tenantID, accountID, portal,
	).Exec(ctx); err != nil {
		return fmt.Errorf("record pwa standalone usage: %w", err)
	}
	return nil
}

// DeleteLastSeenBefore removes usage rows older than cutoff for one tenant.
func (r *PWAStandaloneUsageRepository) DeleteLastSeenBefore(ctx context.Context, tenantID int64, cutoff time.Time) (int, error) {
	db, err := r.database(ctx)
	if err != nil {
		return 0, err
	}
	var affected int
	if err := db.NewRaw(
		"SELECT iot.delete_pwa_standalone_usage_before(?, ?)",
		tenantID, cutoff,
	).Scan(ctx, &affected); err != nil {
		return 0, fmt.Errorf("delete expired pwa standalone usage: %w", err)
	}
	return affected, nil
}
