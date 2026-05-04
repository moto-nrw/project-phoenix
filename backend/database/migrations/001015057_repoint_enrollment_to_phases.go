package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	repointEnrollmentToPhasesVersion     = "1.15.57"
	repointEnrollmentToPhasesDescription = "Repoint enrollment.requests + enrollment.care_offerings from calendar_period_id to phase_id; drop now-redundant application_window_* on offerings (phase owns the window)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repointEnrollmentToPhasesVersion,
		Description: repointEnrollmentToPhasesDescription,
		DependsOn: []string{
			createEnrollmentPhasesVersion,
			createCareOfferingsVersion,
			createEnrollmentRequestsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.57: Repointing enrollment tables to phase_id...")

			// Wipe any rows in dependent tables — there is no production data
			// yet, and partially-pointed rows would otherwise block the schema
			// rewrite. Cleaner than juggling NULLable phase_id during the
			// transition.
			if _, err := db.NewRaw(`
				DELETE FROM enrollment.request_child_offerings;
				DELETE FROM enrollment.request_children;
				DELETE FROM enrollment.requests;
				DELETE FROM enrollment.care_offerings;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed clearing dependent rows: %w", err)
			}

			// requests
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
					DROP COLUMN IF EXISTS calendar_period_id,
					ADD COLUMN phase_id BIGINT NOT NULL REFERENCES enrollment.phases(id) ON DELETE CASCADE;
				CREATE INDEX IF NOT EXISTS idx_enrollment_requests_phase
					ON enrollment.requests (phase_id);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed repointing requests: %w", err)
			}

			// care_offerings — repoint AND drop redundant application_window_*
			// + drop service_start_date / service_end_date (phase owns those).
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offerings
					DROP COLUMN IF EXISTS calendar_period_id,
					DROP COLUMN IF EXISTS application_window_start,
					DROP COLUMN IF EXISTS application_window_end,
					DROP COLUMN IF EXISTS service_start_date,
					DROP COLUMN IF EXISTS service_end_date,
					ADD COLUMN phase_id BIGINT NOT NULL REFERENCES enrollment.phases(id) ON DELETE CASCADE;
				CREATE INDEX IF NOT EXISTS idx_care_offerings_phase
					ON enrollment.care_offerings (phase_id);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed repointing care_offerings: %w", err)
			}

			// Drop the tenant-wide settings that the phase model now owns.
			// setting_values / setting_audit may carry orphaned rows for
			// tenants that touched these keys before the phase model;
			// remove them for cleanliness.
			if _, err := db.NewRaw(`
				DELETE FROM config.setting_values WHERE setting_key IN (
					'enrollment.open_window_start',
					'enrollment.open_window_end',
					'enrollment.show_status_reason_to_parent',
					'enrollment.care_overflow_mode'
				);
				DELETE FROM config.setting_audit WHERE setting_key IN (
					'enrollment.open_window_start',
					'enrollment.open_window_end',
					'enrollment.show_status_reason_to_parent',
					'enrollment.care_overflow_mode'
				);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed cleaning deprecated setting rows: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.57: Restoring calendar_period_id columns...")
			// Best-effort rollback. Restores the columns but does not refill
			// data — there was no data to preserve.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
					DROP COLUMN IF EXISTS phase_id,
					ADD COLUMN calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL;
				ALTER TABLE enrollment.care_offerings
					DROP COLUMN IF EXISTS phase_id,
					ADD COLUMN calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL,
					ADD COLUMN application_window_start TIMESTAMPTZ,
					ADD COLUMN application_window_end TIMESTAMPTZ,
					ADD COLUMN service_start_date DATE,
					ADD COLUMN service_end_date DATE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("rollback failed: %w", err)
			}
			return nil
		},
	)
}
