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
	MigrationRegistry[renameDeviceTypesVersion] = &Migration{
		Version:     renameDeviceTypesVersion,
		Description: renameDeviceTypesDescription,
		DependsOn:   []string{"1.3.9"},
	}

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
	fmt.Println("Migration 1.15.16: Renaming legacy device types to 'terminal'...")

	// Temporarily disable RLS on iot.devices so this cross-tenant UPDATE can reach all rows.
	// Without this, the tenant isolation policy filters out every row (no app.current_tenant_id set).
	if _, err := db.ExecContext(ctx, `ALTER TABLE iot.devices DISABLE ROW LEVEL SECURITY`); err != nil {
		return fmt.Errorf("error disabling RLS on iot.devices: %w", err)
	}

	result, err := db.ExecContext(ctx, `
		UPDATE iot.devices
		SET device_type = 'terminal'
		WHERE device_type IN ('rfid_reader', 'rfid_scanner', 'scanner', 'tablet', 'sensor', 'camera', 'beacon')
	`)
	if err != nil {
		// Re-enable RLS even on failure
		_, _ = db.ExecContext(ctx, `ALTER TABLE iot.devices ENABLE ROW LEVEL SECURITY`)
		_, _ = db.ExecContext(ctx, `ALTER TABLE iot.devices FORCE ROW LEVEL SECURITY`)
		return fmt.Errorf("error renaming device types: %w", err)
	}

	// Re-enable RLS + FORCE (matches the state set by migration 1.15.1)
	if _, err := db.ExecContext(ctx, `ALTER TABLE iot.devices ENABLE ROW LEVEL SECURITY`); err != nil {
		return fmt.Errorf("error re-enabling RLS on iot.devices: %w", err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE iot.devices FORCE ROW LEVEL SECURITY`); err != nil {
		return fmt.Errorf("error re-forcing RLS on iot.devices: %w", err)
	}

	rows, _ := result.RowsAffected()
	fmt.Printf("Migration 1.15.16: Updated %d devices from rfid_reader/rfid_scanner to terminal\n", rows)
	return nil
}

func revertDeviceTypes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.16: Reverting device types to 'rfid_reader'...")

	if _, err := db.ExecContext(ctx, `ALTER TABLE iot.devices DISABLE ROW LEVEL SECURITY`); err != nil {
		return fmt.Errorf("error disabling RLS on iot.devices: %w", err)
	}

	_, err := db.ExecContext(ctx, `
		UPDATE iot.devices
		SET device_type = 'rfid_reader'
		WHERE device_type = 'terminal'
	`)
	if err != nil {
		_, _ = db.ExecContext(ctx, `ALTER TABLE iot.devices ENABLE ROW LEVEL SECURITY`)
		_, _ = db.ExecContext(ctx, `ALTER TABLE iot.devices FORCE ROW LEVEL SECURITY`)
		return fmt.Errorf("error reverting device types: %w", err)
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE iot.devices ENABLE ROW LEVEL SECURITY`); err != nil {
		return fmt.Errorf("error re-enabling RLS on iot.devices: %w", err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE iot.devices FORCE ROW LEVEL SECURITY`); err != nil {
		return fmt.Errorf("error re-forcing RLS on iot.devices: %w", err)
	}

	fmt.Println("Migration 1.15.16: Successfully reverted device types")
	return nil
}
