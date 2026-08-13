package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const activityCompletionSnapshotVersion = "1.15.291"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityCompletionSnapshotVersion,
		Description: "Store exact activity state for completion undo (#2266)",
		DependsOn:   []string{activityInstanceReopenVersion},
	})
	Migrations.MustRegister(activityCompletionSnapshotUp, activityCompletionSnapshotDown)
}

func activityCompletionSnapshotUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.291: Adding activity completion snapshots (#2266)...")
	_, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances
			ADD COLUMN IF NOT EXISTS completion_snapshot JSONB;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add activity completion snapshot: %w", err)
	}
	return nil
}

func activityCompletionSnapshotDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances
			DROP COLUMN IF EXISTS completion_snapshot;
	`).Exec(ctx)
	return err
}
