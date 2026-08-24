package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careRequestDecisionSnapshotVersion     = "1.15.307"
	careRequestDecisionSnapshotDescription = "Freeze the alt→neu diff of decided care-schedule change requests (#2430)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careRequestDecisionSnapshotVersion,
		Description: careRequestDecisionSnapshotDescription,
		DependsOn:   []string{classListEntriesVersion},
	})
	Migrations.MustRegister(careRequestDecisionSnapshotUp, careRequestDecisionSnapshotDown)
}

func careRequestDecisionSnapshotUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.307: Adding the care-request decision snapshot column...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.care_schedule_change_requests
			ADD COLUMN IF NOT EXISTS decision_snapshot JSONB;
	`)
	if err != nil {
		return fmt.Errorf("add care request decision snapshot column: %w", err)
	}
	return nil
}

func careRequestDecisionSnapshotDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.307: Removing the care-request decision snapshot column...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.care_schedule_change_requests
			DROP COLUMN IF EXISTS decision_snapshot;
	`)
	if err != nil {
		return fmt.Errorf("remove care request decision snapshot column: %w", err)
	}
	return nil
}
