package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	offeringChangeDecisionSnapshotVersion     = "1.15.299"
	offeringChangeDecisionSnapshotDescription = "Freeze the review diff of decided offering change requests (#2365, #2370)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     offeringChangeDecisionSnapshotVersion,
		Description: offeringChangeDecisionSnapshotDescription,
		DependsOn:   []string{staffParentMessageNotificationDebounceVersion},
	})
	Migrations.MustRegister(offeringChangeDecisionSnapshotUp, offeringChangeDecisionSnapshotDown)
}

func offeringChangeDecisionSnapshotUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.299: Adding the offering-change decision snapshot column...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE enrollment.offering_change_requests
			ADD COLUMN IF NOT EXISTS decision_snapshot JSONB;
	`)
	if err != nil {
		return fmt.Errorf("add offering change decision snapshot column: %w", err)
	}
	return nil
}

func offeringChangeDecisionSnapshotDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.299: Removing the offering-change decision snapshot column...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE enrollment.offering_change_requests
			DROP COLUMN IF EXISTS decision_snapshot;
	`)
	if err != nil {
		return fmt.Errorf("remove offering change decision snapshot column: %w", err)
	}
	return nil
}
