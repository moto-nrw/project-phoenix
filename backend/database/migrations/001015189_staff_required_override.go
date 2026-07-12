package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffRequiredOverrideVersion     = "1.15.189"
	staffRequiredOverrideDescription = "Add nullable required_staff (Personalbedarf manual override) to activities.groups + schedule.activity_instances (issue #1839)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffRequiredOverrideVersion,
		Description: staffRequiredOverrideDescription,
		DependsOn: []string{
			ActivitiesGroupsVersion,
			createActivityInstancesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffRequiredOverrideUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffRequiredOverrideDown(ctx, db)
		},
	)
}

func staffRequiredOverrideUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.189: Adding required_staff override columns to activities.groups and schedule.activity_instances...")

	// NULLABLE by design: NULL = no manual requirement, fall back to the
	// Betreuungsschlüssel-derived value (timetable.children_per_staff_ratio,
	// issue #1869). A non-NULL value is an explicit admin override that wins
	// over the derived requirement (issue #1839).
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
		ADD COLUMN IF NOT EXISTS required_staff INT;

		ALTER TABLE schedule.activity_instances
		ADD COLUMN IF NOT EXISTS required_staff INT;

		ALTER TABLE activities.groups
		ADD CONSTRAINT chk_groups_required_staff_nonneg CHECK (required_staff IS NULL OR required_staff >= 0);

		ALTER TABLE schedule.activity_instances
		ADD CONSTRAINT chk_activity_instances_required_staff_nonneg CHECK (required_staff IS NULL OR required_staff >= 0);

		COMMENT ON COLUMN activities.groups.required_staff IS 'Manual Personalbedarf override for this template; NULL = derive from Betreuungsschlüssel. Inherited by materialized instances at read time.';
		COMMENT ON COLUMN schedule.activity_instances.required_staff IS 'Per-occurrence Personalbedarf pin; NULL = inherit template override, then derive from Betreuungsschlüssel. Materialization leaves this NULL; a set value survives ReplanWeek.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding required_staff columns: %w", err)
	}

	return nil
}

func staffRequiredOverrideDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.189: Removing required_staff override columns...")

	_, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances DROP CONSTRAINT IF EXISTS chk_activity_instances_required_staff_nonneg;
		ALTER TABLE activities.groups DROP CONSTRAINT IF EXISTS chk_groups_required_staff_nonneg;
		ALTER TABLE schedule.activity_instances DROP COLUMN IF EXISTS required_staff;
		ALTER TABLE activities.groups DROP COLUMN IF EXISTS required_staff;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping required_staff columns: %w", err)
	}

	return nil
}
