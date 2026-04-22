package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendanceDeviceIDNullableVersion     = "1.15.41"
	attendanceDeviceIDNullableDescription = "Make active.attendance.device_id nullable for non-device check-ins (web school-checkin)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendanceDeviceIDNullableVersion,
		Description: attendanceDeviceIDNullableDescription,
		DependsOn: []string{
			AttendanceVersion, // 1.6.5 — active.attendance + fk_attendance_device
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return attendanceDeviceIDNullableUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return attendanceDeviceIDNullableDown(ctx, db)
		},
	)
}

func attendanceDeviceIDNullableUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.41: dropping NOT NULL on active.attendance.device_id...")

	// Web-based school check-ins don't originate from a kiosk, so device_id
	// must be allowed to be NULL. The existing FK constraint still enforces
	// referential integrity for IoT-originated rows; a NULL device_id skips
	// the FK check per SQL semantics.
	_, err := db.NewRaw(`
		ALTER TABLE active.attendance
		ALTER COLUMN device_id DROP NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping NOT NULL on active.attendance.device_id: %w", err)
	}

	return nil
}

func attendanceDeviceIDNullableDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.41: restoring NOT NULL on active.attendance.device_id...")

	// Rollback only succeeds if no NULL rows exist — surface that clearly
	// rather than silently inserting a sentinel value.
	_, err := db.NewRaw(`
		ALTER TABLE active.attendance
		ALTER COLUMN device_id SET NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring NOT NULL on active.attendance.device_id (NULL rows from web check-ins may exist): %w", err)
	}

	return nil
}
