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
			ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'direct';
	`); err != nil {
		return fmt.Errorf("add offering adjustment source column: %w", err)
	}
	// Backfill 1 of 2 — rows an approved offering request wrote. They carry the
	// reason that approval generates, so the match is against that generated
	// shape exactly ("Elternanfrage #12 freigegeben (gültig ab 01.09.2026)")
	// rather than a loose prefix: a hand-typed correction reason cannot collide
	// with it.
	if _, err := db.ExecContext(ctx, `
		UPDATE audit.enrollment_offering_adjustments
			SET source = 'request'
			WHERE reason ~ '^Elternanfrage #[0-9]+ freigegeben \(gültig ab [0-9]{2}\.[0-9]{2}\.[0-9]{4}\)';
	`); err != nil {
		return fmt.Errorf("backfill offering adjustment source: %w", err)
	}
	// Backfill 2 of 2 — rows an approved Anmeldungsänderung wrote on the side.
	// Those carry the reviewer's free text and are unrecognizable from the
	// reason alone, but they are not guesswork either: the adjustment is written
	// inside the approval's own transaction, so it shares the enrollment, the
	// deciding account and the decision instant. Without this pass they would
	// read as the office's own correction, which is exactly the "what actually
	// happened here?" confusion this history exists to end (#2413).
	//
	// The join runs on request_id, not request_child_id: a change request hangs
	// off the enrollment and leaves request_child_id NULL unless it targets one
	// child, so a child-level join would miss the very rows this pass is for.
	if _, err := db.ExecContext(ctx, `
		UPDATE audit.enrollment_offering_adjustments AS a
			SET source = 'request'
			FROM enrollment.change_requests AS cr
			WHERE a.source = 'direct'
			  AND cr.status = 'approved'
			  AND cr.reviewed_at IS NOT NULL
			  AND cr.request_id = a.request_id
			  AND (cr.request_child_id IS NULL OR cr.request_child_id = a.request_child_id)
			  AND cr.reviewed_by_account_id = a.actor_account_id
			  AND a.changed_at BETWEEN cr.reviewed_at - INTERVAL '10 seconds'
			                       AND cr.reviewed_at + INTERVAL '10 seconds';
	`); err != nil {
		return fmt.Errorf("backfill offering adjustment source from change requests: %w", err)
	}
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
