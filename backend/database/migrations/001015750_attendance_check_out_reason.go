package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendanceCheckOutReasonVersion     = "1.15.750"
	attendanceCheckOutReasonDescription = "Add active.attendance.check_out_reason - a checkout that sends the child to class instead of home (#5656)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendanceCheckOutReasonVersion,
		Description: attendanceCheckOutReasonDescription,
		DependsOn: []string{
			AttendanceVersion,                       // active.attendance
			holidayContributionChainSettingsVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(attendanceCheckOutReasonUp, attendanceCheckOutReasonDown)
}

// attendanceCheckOutReasonUp records why a stay ended when the child comes back
// later (#5656). The reason belongs to one stay: the return opens a new row, so
// it never has to be cleared. NULL is every other checkout, so existing rows
// keep their meaning and nothing is backfilled. Every existing row is NULL, so
// NOT VALID spares the table scan under the exclusive lock; new writes are checked.
func attendanceCheckOutReasonUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.attendance
			ADD COLUMN IF NOT EXISTS check_out_reason TEXT;
		ALTER TABLE active.attendance
			DROP CONSTRAINT IF EXISTS chk_attendance_check_out_reason;
		ALTER TABLE active.attendance
			ADD CONSTRAINT chk_attendance_check_out_reason
			CHECK (check_out_reason IS NULL OR check_out_reason = 'class') NOT VALID;
	`)
	if err != nil {
		return fmt.Errorf("error adding active.attendance.check_out_reason: %w", err)
	}
	return nil
}

func attendanceCheckOutReasonDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.attendance DROP CONSTRAINT IF EXISTS chk_attendance_check_out_reason;
		ALTER TABLE active.attendance DROP COLUMN IF EXISTS check_out_reason;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.attendance.check_out_reason: %w", err)
	}
	return nil
}
