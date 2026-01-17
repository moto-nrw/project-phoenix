package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AddWebManualDeviceVersion     = "1.7.5"
	AddWebManualDeviceDescription = "Add virtual device for manual web check-ins"
)

func init() {
	MigrationRegistry[AddWebManualDeviceVersion] = &Migration{
		Version:     AddWebManualDeviceVersion,
		Description: AddWebManualDeviceDescription,
		DependsOn:   []string{"1.3.9"}, // Depends on iot.devices table
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addWebManualDevice(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeWebManualDevice(ctx, db)
		},
	)
}

// addWebManualDevice creates the virtual device used for manual web check-ins.
// This device is referenced by the active service when staff performs check-ins
// through the web portal instead of using a physical RFID scanner.
func addWebManualDevice(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.5: Adding virtual device for web manual check-ins...")

	_, err := db.ExecContext(ctx, `
		INSERT INTO iot.devices (device_id, device_type, name, status)
		VALUES ('WEB-MANUAL-001', 'virtual', 'Web-Portal (Manuell)', 'active')
		ON CONFLICT (device_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting web manual device: %w", err)
	}

	fmt.Println("Migration 1.7.5: Successfully added web manual device (WEB-MANUAL-001)")
	return nil
}

// removeWebManualDevice removes the virtual web device
func removeWebManualDevice(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.5: Removing web manual device...")

	_, err := db.ExecContext(ctx, `
		DELETE FROM iot.devices WHERE device_id = 'WEB-MANUAL-001'
	`)
	if err != nil {
		return fmt.Errorf("error removing web manual device: %w", err)
	}

	fmt.Println("Migration 1.7.5: Successfully removed web manual device")
	return nil
}
