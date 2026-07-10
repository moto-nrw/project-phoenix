package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupTargetGroupVersion     = "1.15.182"
	groupTargetGroupDescription = "Add target_group_type/target_grade_level/target_school_class to activities.groups for Betreuungsplan Zielgruppe modeling"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupTargetGroupVersion,
		Description: groupTargetGroupDescription,
		DependsOn: []string{
			groupCalendarPeriodVersion, // 1.15.181
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return groupTargetGroupUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return groupTargetGroupDown(ctx, db)
		},
	)
}

func groupTargetGroupUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.182: Adding target-group columns to activities.groups...")

	// target_group_type is TEXT+CHECK rather than a DB enum, matching the
	// established convention in this domain (activities.groups.type,
	// schedule.activity_instances.status, schedule.activity_exceptions.
	// exception_type) so adding a new target-group type later doesn't
	// require an ALTER TYPE. "gruppe" deliberately does not get its own
	// value column - it reuses the existing education_group_id FK.
	// "angebot" (Angebotsauswahl) needs no value column either: its roster
	// comes from the existing CareOffering.ActivityGroupID bridge, not from
	// a value stored on the Group itself.
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS target_group_type TEXT NOT NULL DEFAULT 'none',
			ADD COLUMN IF NOT EXISTS target_grade_level SMALLINT,
			ADD COLUMN IF NOT EXISTS target_school_class TEXT;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding target-group columns to activities.groups: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE activities.groups
			ADD CONSTRAINT chk_activities_groups_target_group_type
			CHECK (target_group_type IN ('jahrgang', 'klasse', 'gruppe', 'angebot', 'none'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding target_group_type check constraint: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activities_groups_target_group
			ON activities.groups (tenant_id, target_group_type)
			WHERE target_group_type <> 'none';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating activities.groups target_group_type index: %w", err)
	}

	fmt.Println("Migration 1.15.182: Completed successfully")
	return nil
}

func groupTargetGroupDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.182: Dropping target-group columns from activities.groups...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_activities_groups_target_group;

		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_group_type;

		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS target_group_type,
			DROP COLUMN IF EXISTS target_grade_level,
			DROP COLUMN IF EXISTS target_school_class;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping target-group columns from activities.groups: %w", err)
	}

	return nil
}
