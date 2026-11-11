package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const holidayBookingExceptionSourceVersion = "1.15.923"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayBookingExceptionSourceVersion,
		Description: "Arrival and pickup exceptions planned from a holiday booking carry their own source without a staff author (ADR 0120 addendum)",
		DependsOn: []string{
			parentAuthoredExceptionsVersion,   // chk_exception_source and chk_exception_author
			dataReturnErasureOrderLockVersion, // the newest on development (1.15.919)
		},
	})
	Migrations.MustRegister(holidayBookingExceptionSourceUp, holidayBookingExceptionSourceDown)
}

// A day a holiday guest booked is planned by Enrollment from the booking
// (ADR 0120, addendum of 2026-11-11), not by a person of the school: the row
// carries the source holiday_booking and no staff author, so the days are
// planned even when the account that picked the group has no staff record in
// the hosting school.
func holidayBookingExceptionSourceUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.student_arrival_exceptions
			DROP CONSTRAINT IF EXISTS chk_exception_source,
			ADD CONSTRAINT chk_exception_source CHECK (source IN ('staff', 'guardian', 'holiday_booking')),
			DROP CONSTRAINT IF EXISTS chk_exception_author,
			ADD CONSTRAINT chk_exception_author CHECK (
				(source = 'staff' AND created_by IS NOT NULL)
				OR source = 'guardian'
				OR source = 'holiday_booking'
			);
		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS chk_exception_source,
			ADD CONSTRAINT chk_exception_source CHECK (source IN ('staff', 'guardian', 'holiday_booking')),
			DROP CONSTRAINT IF EXISTS chk_exception_author,
			ADD CONSTRAINT chk_exception_author CHECK (
				(source = 'staff' AND created_by IS NOT NULL)
				OR source = 'guardian'
				OR source = 'holiday_booking'
			);
	`)
	return err
}

// The staff-or-guardian shape cannot hold a row without an author: the down
// migration deletes the planned holiday days, as 1.15.136's down migration
// deletes the guardian rows.
func holidayBookingExceptionSourceDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM schedule.student_arrival_exceptions WHERE source = 'holiday_booking';
		DELETE FROM schedule.student_pickup_exceptions WHERE source = 'holiday_booking';
		ALTER TABLE schedule.student_arrival_exceptions
			DROP CONSTRAINT IF EXISTS chk_exception_source,
			ADD CONSTRAINT chk_exception_source CHECK (source IN ('staff', 'guardian')),
			DROP CONSTRAINT IF EXISTS chk_exception_author,
			ADD CONSTRAINT chk_exception_author CHECK (
				(source = 'staff' AND created_by IS NOT NULL)
				OR source = 'guardian'
			);
		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS chk_exception_source,
			ADD CONSTRAINT chk_exception_source CHECK (source IN ('staff', 'guardian')),
			DROP CONSTRAINT IF EXISTS chk_exception_author,
			ADD CONSTRAINT chk_exception_author CHECK (
				(source = 'staff' AND created_by IS NOT NULL)
				OR source = 'guardian'
			);
	`)
	return err
}
