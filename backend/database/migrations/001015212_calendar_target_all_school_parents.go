package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarTargetAllSchoolParentsVersion     = "1.15.212"
	calendarTargetAllSchoolParentsDescription = "Allow the all_school_parents calendar appointment target type"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarTargetAllSchoolParentsVersion,
		Description: calendarTargetAllSchoolParentsDescription,
		DependsOn:   []string{calendarFeedTokenVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.212: Adding all_school_parents to the calendar target check...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointment_targets
					DROP CONSTRAINT IF EXISTS chk_calendar_targets_type,
					ADD CONSTRAINT chk_calendar_targets_type CHECK (
						target_type IN (
							'staff',
							'guardian_profile',
							'all_staff',
							'all_school_parents',
							'parents_by_class',
							'parents_by_group',
							'parents_by_student'
						)
					);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed updating calendar target check: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.212: Removing all_school_parents from the calendar target check...")
			// Existing all_school_parents rows would violate the tightened CHECK and
			// abort the ALTER, so drop them first. Rolling this migration back removes
			// the target type entirely, so those rows have no valid representation.
			if _, err := db.NewRaw(`
				DELETE FROM calendar.appointment_targets
				WHERE target_type = 'all_school_parents';

				ALTER TABLE calendar.appointment_targets
					DROP CONSTRAINT IF EXISTS chk_calendar_targets_type,
					ADD CONSTRAINT chk_calendar_targets_type CHECK (
						target_type IN (
							'staff',
							'guardian_profile',
							'all_staff',
							'parents_by_class',
							'parents_by_group',
							'parents_by_student'
						)
					);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed reverting calendar target check: %w", err)
			}
			return nil
		},
	)
}
