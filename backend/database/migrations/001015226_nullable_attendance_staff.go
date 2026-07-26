package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	nullableAttendanceStaffVersion     = "1.15.226"
	nullableAttendanceStaffDescription = "Allow device-authenticated attendance without unverified staff attribution"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     nullableAttendanceStaffVersion,
		Description: nullableAttendanceStaffDescription,
		DependsOn: []string{
			attendanceUniqueOpenPerDayVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return nullableAttendanceStaffUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return nullableAttendanceStaffDown(ctx, db)
		},
	)
}

func nullableAttendanceStaffUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.226: Allowing device-authenticated attendance without staff attribution...")

	_, err := db.NewRaw(`
		ALTER TABLE active.attendance
			ALTER COLUMN checked_in_by DROP NOT NULL;
		COMMENT ON COLUMN active.attendance.checked_in_by IS
			'Authenticated staff actor; NULL when an authenticated device records attendance without a verified staff credential';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed making active.attendance.checked_in_by nullable: %w", err)
	}

	return nil
}

func nullableAttendanceStaffDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.226: Restoring required attendance staff attribution...")

	var nullCount int
	err := db.NewRaw(`SELECT COUNT(*) FROM active.attendance WHERE checked_in_by IS NULL`).Scan(ctx, &nullCount)
	if err != nil {
		return fmt.Errorf("failed counting attendance rows without staff attribution: %w", err)
	}
	if nullCount > 0 {
		return fmt.Errorf(
			"cannot restore NOT NULL on active.attendance.checked_in_by: %d rows have no staff attribution; resolve them before rolling back migration 1.15.226",
			nullCount,
		)
	}

	_, err = db.NewRaw(`
		ALTER TABLE active.attendance
			ALTER COLUMN checked_in_by SET NOT NULL;
		COMMENT ON COLUMN active.attendance.checked_in_by IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring NOT NULL on active.attendance.checked_in_by: %w", err)
	}

	return nil
}
