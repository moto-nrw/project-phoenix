package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	IoTTablesVersion     = "1.7.0"
	IoTTablesDescription = "IoT device management tables"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[IoTTablesVersion] = &Migration{
		Version:     IoTTablesVersion,
		Description: IoTTablesDescription,
		DependsOn:   []string{"1.6.0"}, // Depends on facilities tables
	}

	// Migration 1.7.0: IoT schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return iotTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return iotTablesDown(ctx, db)
		},
	)
}

// iotTablesUp creates the IoT schema tables
func iotTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.0: Creating IoT schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create IoT schema
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
		CREATE TYPE device_status AS ENUM ('active', 'inactive', 'maintenance', 'offline');
	`)
	if err != nil {
		return fmt.Errorf("error creating device_status type: %w", err)
	}

	// Create IoT devices table
	_, err = tx.ExecContext(ctx, `
		-- IoT device management tables
		CREATE TABLE IF NOT EXISTS iot.devices (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			device_type TEXT NOT NULL,
			name TEXT,
			location TEXT,
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

// iotTablesDown removes the IoT schema tables
func iotTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.0: Removing IoT schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS iot.devices CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping IoT devices table: %w", err)
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
