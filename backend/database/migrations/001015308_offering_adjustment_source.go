package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	offeringAdjustmentSourceVersion     = "1.15.308"
	offeringAdjustmentSourceDescription = "Mark whether an offering adjustment is an admin direct correction or the application of an approved request (#2436)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     offeringAdjustmentSourceVersion,
		Description: offeringAdjustmentSourceDescription,
		DependsOn:   []string{careRequestDecisionSnapshotVersion},
	})
	Migrations.MustRegister(offeringAdjustmentSourceUp, offeringAdjustmentSourceDown)
}

func offeringAdjustmentSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.308: Adding the offering adjustment source column...")
	// The central history shows admin direct corrections as their own row kind
	// (#2436). Approved parent requests write an adjustment row through the very
	// same path, so without this discriminator they would appear twice: once as
	// the decided request, once as a correction.
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.enrollment_offering_adjustments
			ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'unknown';
	`); err != nil {
		return fmt.Errorf("add offering adjustment source column: %w", err)
	}
	// Historical rows do not persist their origin. A reason, actor, or timestamp
	// match is ambiguous and can hide a real direct correction, so legacy rows
	// deliberately remain "unknown". New writers always store direct or request.
	if _, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_enrollment_offering_adjustments_direct_history
			ON audit.enrollment_offering_adjustments (tenant_id, changed_at DESC, id DESC)
			WHERE source = 'direct';
	`); err != nil {
		return fmt.Errorf("create offering adjustment direct history index: %w", err)
	}
	return nil
}

func offeringAdjustmentSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.308: Removing the offering adjustment source column...")
	if _, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS audit.idx_enrollment_offering_adjustments_direct_history;
	`); err != nil {
		return fmt.Errorf("drop offering adjustment direct history index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.enrollment_offering_adjustments
			DROP COLUMN IF EXISTS source;
	`); err != nil {
		return fmt.Errorf("remove offering adjustment source column: %w", err)
	}
	return nil
}
