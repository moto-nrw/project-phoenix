package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	IoTDevicesVersion     = "1.3.5" // Version maintained to preserve compatibility with room_occupancy
	IoTDevicesDescription = "Create iot.devices table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[IoTDevicesVersion] = &Migration{
		Version:     IoTDevicesVersion,
		Description: IoTDevicesDescription,
		DependsOn:   []string{"1.2.1"}, // Depends on users.persons
	}

	// Migration 1.3.5: Create iot.devices table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createIoTDevicesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropIoTDevicesTable(ctx, db)
		},
	)
}

// createIoTDevicesTable creates the iot.devices table
func createIoTDevicesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.5: Creating iot.devices table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create IoT schema if it doesn't exist
	_, err = tx.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS iot;
	`)
	if err != nil {
		return fmt.Errorf("error creating IoT schema: %w", err)
	}

	// First, drop the type if it exists to ensure we can recreate it
	_, err = tx.ExecContext(ctx, `
		DROP TYPE IF EXISTS device_status CASCADE;
		
		-- Create device status enum
		CREATE TYPE device_status AS ENUM (
			'active',    -- Device is fully operational
			'inactive',  -- Device is powered on but not in use
			'maintenance', -- Device is undergoing maintenance
			'offline'    -- Device is not connected or powered off
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating device_status type: %w", err)
	}

	// Create IoT devices table
	_, err = tx.ExecContext(ctx, `
		-- IoT device management table
		CREATE TABLE IF NOT EXISTS iot.devices (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			device_type TEXT NOT NULL,
			name TEXT,
			status device_status NOT NULL DEFAULT 'active',
			last_seen TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			registered_by_id BIGINT,
			CONSTRAINT fk_iot_devices_registered_by FOREIGN KEY (registered_by_id)
				REFERENCES users.persons(id) ON DELETE SET NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating IoT devices table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_iot_devices_device_id ON iot.devices(device_id);
		CREATE INDEX IF NOT EXISTS idx_iot_devices_status ON iot.devices(status);
		CREATE INDEX IF NOT EXISTS idx_iot_devices_device_type ON iot.devices(device_type);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for IoT devices table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for devices
		DROP TRIGGER IF EXISTS update_iot_devices_updated_at ON iot.devices;
		CREATE TRIGGER update_iot_devices_updated_at
		BEFORE UPDATE ON iot.devices
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for IoT devices: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropIoTDevicesTable drops the iot.devices table
func dropIoTDevicesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.5: Removing iot.devices table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_iot_devices_updated_at ON iot.devices;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for iot.devices table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS iot.devices CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping iot.devices table: %w", err)
	}

	// Drop the device_status type
	_, err = tx.ExecContext(ctx, `
		DROP TYPE IF EXISTS device_status CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping device_status type: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
