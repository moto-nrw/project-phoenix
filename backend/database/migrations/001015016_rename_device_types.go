package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	renameDeviceTypesVersion     = "1.15.16"
	renameDeviceTypesDescription = "Rename legacy device types (rfid_reader, rfid_scanner) to terminal"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     renameDeviceTypesVersion,
		Description: renameDeviceTypesDescription,
		DependsOn:   []string{"1.3.9"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return renameDeviceTypes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return revertDeviceTypes(ctx, db)
		},
	)
}

func renameDeviceTypes(ctx context.Context, db *bun.DB) error {
	result, err := db.ExecContext(ctx, `
		UPDATE iot.devices
		SET device_type = 'terminal'
		WHERE device_type IN ('rfid_reader', 'rfid_scanner', 'scanner', 'tablet', 'sensor', 'camera', 'beacon')
	`)
	if err != nil {
		return fmt.Errorf("error renaming device types: %w", err)
	}

	rows, _ := result.RowsAffected()
	migrationLog().InfoContext(ctx, "legacy device types renamed to terminal",
		"rows", rows,
	)
	return nil
}

func revertDeviceTypes(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		UPDATE iot.devices
		SET device_type = 'rfid_reader'
		WHERE device_type = 'terminal'
	`)
	if err != nil {
		return fmt.Errorf("error reverting device types: %w", err)
	}

	return nil
}
