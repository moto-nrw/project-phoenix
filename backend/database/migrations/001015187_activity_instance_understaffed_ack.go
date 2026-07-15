package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activityInstanceUnderstaffedAckVersion     = "1.15.187"
	activityInstanceUnderstaffedAckDescription = "Add understaffed_ack + understaffed_note to schedule.activity_instances so an admin can deliberately leave a block unstaffed (Vertretungsplan, issue #1840) without it being reported as a gap."
)

// NOTE ON VERSION NUMBER: development has since advanced through 1.15.186
// (the recurring care-plan blocks feature took 1.15.181–1.15.185 for
// activities.groups changes, and 1.15.186 for the enrollment exclusive-end
// constraint). To avoid a duplicate-version panic at init, this
// Vertretungsplan migration is renumbered to 1.15.187.

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityInstanceUnderstaffedAckVersion,
		Description: activityInstanceUnderstaffedAckDescription,
		DependsOn: []string{
			createActivityInstancesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.187: Adding understaffed_ack + understaffed_note to schedule.activity_instances...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.activity_instances
				ADD COLUMN IF NOT EXISTS understaffed_ack BOOLEAN NOT NULL DEFAULT FALSE,
				ADD COLUMN IF NOT EXISTS understaffed_note TEXT;
				COMMENT ON COLUMN schedule.activity_instances.understaffed_ack IS
					'Admin deliberately accepts this block running with zero staff (Vertretungsplan). Suppresses the gap warning without hiding the shortfall.';
				COMMENT ON COLUMN schedule.activity_instances.understaffed_note IS
					'Optional reason the block is intentionally left unstaffed.';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding understaffed columns to schedule.activity_instances: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.187...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.activity_instances
				DROP COLUMN IF EXISTS understaffed_note,
				DROP COLUMN IF EXISTS understaffed_ack;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping understaffed columns from schedule.activity_instances: %w", err)
			}
			return nil
		},
	)
}
