package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	deviationReasonsVersion     = "1.15.188"
	deviationReasonsDescription = "Add optional reason columns for Vertretungsplan deviations (#1840): instance_staff.absence_reason and activity_instances.cancel_reason so an absence or cancellation can carry a short 'why'."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     deviationReasonsVersion,
		Description: deviationReasonsDescription,
		DependsOn: []string{
			createActivityInstancesVersion,
			activityInstanceUnderstaffedAckVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.188: Adding absence_reason + cancel_reason columns...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.instance_staff
				ADD COLUMN IF NOT EXISTS absence_reason TEXT;
				COMMENT ON COLUMN schedule.instance_staff.absence_reason IS
					'Optional reason why this staff member is absent (Vertretungsplan).';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding absence_reason to schedule.instance_staff: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.activity_instances
				ADD COLUMN IF NOT EXISTS cancel_reason TEXT;
				COMMENT ON COLUMN schedule.activity_instances.cancel_reason IS
					'Optional reason why this block was cancelled (Vertretungsplan).';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding cancel_reason to schedule.activity_instances: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.188...")
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.activity_instances DROP COLUMN IF EXISTS cancel_reason;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping cancel_reason: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.instance_staff DROP COLUMN IF EXISTS absence_reason;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping absence_reason: %w", err)
			}
			return nil
		},
	)
}
