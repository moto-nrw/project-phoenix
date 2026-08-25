package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendanceCheckoutDeviceVersion     = "1.15.331"
	attendanceCheckoutDeviceDescription = "Record the authenticated device used to check out attendance"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendanceCheckoutDeviceVersion,
		Description: attendanceCheckoutDeviceDescription,
		DependsOn: []string{
			nullableAttendanceStaffVersion,
			compositeFKsVersion,
		},
	})

	Migrations.MustRegister(attendanceCheckoutDeviceUp, attendanceCheckoutDeviceDown)
}

func attendanceCheckoutDeviceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.331: Adding attendance checkout device attribution...")

	_, err := db.NewRaw(`
		ALTER TABLE active.attendance
			ADD COLUMN checked_out_device_id BIGINT,
			ADD CONSTRAINT fk_attendance_checked_out_device_tenant
				FOREIGN KEY (tenant_id, checked_out_device_id)
				REFERENCES iot.devices(tenant_id, id) ON DELETE RESTRICT;
		COMMENT ON COLUMN active.attendance.checked_out_device_id IS
			'Authenticated kiosk device that recorded the checkout; NULL for non-device checkouts';
		CREATE INDEX idx_attendance_checked_out_device_id
			ON active.attendance (checked_out_device_id)
			WHERE checked_out_device_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add attendance checkout device attribution: %w", err)
	}
	return nil
}

func attendanceCheckoutDeviceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.331: Removing attendance checkout device attribution...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS active.idx_attendance_checked_out_device_id;
		ALTER TABLE active.attendance
			DROP CONSTRAINT IF EXISTS fk_attendance_checked_out_device_tenant,
			DROP COLUMN IF EXISTS checked_out_device_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove attendance checkout device attribution: %w", err)
	}
	return nil
}
