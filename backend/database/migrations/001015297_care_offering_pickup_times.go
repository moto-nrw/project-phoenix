package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingPickupTimesVersion     = "1.15.297"
	careOfferingPickupTimesDescription = "Optional per-weekday pickup times on care offerings + source tracking on student pickup schedules"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingPickupTimesVersion,
		Description: careOfferingPickupTimesDescription,
		DependsOn:   []string{pushSubscriptionTokenFamilyVersion},
	})
	Migrations.MustRegister(careOfferingPickupTimesUp, careOfferingPickupTimesDown)
}

func careOfferingPickupTimesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.297: Adding care offering pickup times + pickup schedule source...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE enrollment.care_offerings
			ADD COLUMN IF NOT EXISTS pickup_times JSONB;
		COMMENT ON COLUMN enrollment.care_offerings.pickup_times IS
			'Optional Gehzeit per weekday ({"mon":"14:30",...}); rolled out onto schedule.student_pickup_schedules (#2290)';

		ALTER TABLE schedule.student_pickup_schedules
			ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'staff',
			ADD COLUMN IF NOT EXISTS care_offering_id BIGINT REFERENCES enrollment.care_offerings(id) ON DELETE SET NULL;
		ALTER TABLE schedule.student_pickup_schedules
			ALTER COLUMN created_by DROP NOT NULL;
		ALTER TABLE schedule.student_pickup_schedules
			DROP CONSTRAINT IF EXISTS student_pickup_schedules_source_check;
		ALTER TABLE schedule.student_pickup_schedules
			ADD CONSTRAINT student_pickup_schedules_source_check
				CHECK (
					source IN ('staff', 'care_offering')
					AND (source = 'care_offering' OR created_by IS NOT NULL)
				);
		COMMENT ON COLUMN schedule.student_pickup_schedules.source IS
			'staff = manually maintained; care_offering = materialized from a care offering Gehzeit (#2290)';
		CREATE INDEX IF NOT EXISTS idx_student_pickup_schedules_care_offering
			ON schedule.student_pickup_schedules (care_offering_id)
			WHERE care_offering_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("add care offering pickup times: %w", err)
	}
	return nil
}

func careOfferingPickupTimesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS schedule.idx_student_pickup_schedules_care_offering;
		ALTER TABLE schedule.student_pickup_schedules
			DROP CONSTRAINT IF EXISTS student_pickup_schedules_source_check;
		DELETE FROM schedule.student_pickup_schedules
			WHERE created_by IS NULL;
		ALTER TABLE schedule.student_pickup_schedules
			ALTER COLUMN created_by SET NOT NULL;
		ALTER TABLE schedule.student_pickup_schedules
			DROP COLUMN IF EXISTS care_offering_id,
			DROP COLUMN IF EXISTS source;
		ALTER TABLE enrollment.care_offerings
			DROP COLUMN IF EXISTS pickup_times;
	`)
	if err != nil {
		return fmt.Errorf("drop care offering pickup times: %w", err)
	}
	return nil
}
