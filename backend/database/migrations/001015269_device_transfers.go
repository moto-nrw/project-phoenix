package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	deviceTransfersVersion     = "1.15.269"
	deviceTransfersDescription = "Allow operator device transfers while retaining tenant history"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     deviceTransfersVersion,
		Description: deviceTransfersDescription,
		DependsOn:   []string{enrollmentRestorationsAuditVersion},
	})
	Migrations.MustRegister(deviceTransfersUp, deviceTransfersDown)
}

func deviceTransfersUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.269: Adding device transfer history...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE iot.devices
			ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS transferred_to_device_id BIGINT;

		ALTER TABLE iot.devices
			DROP CONSTRAINT IF EXISTS fk_iot_devices_transferred_to_device;
		ALTER TABLE iot.devices
			ADD CONSTRAINT fk_iot_devices_transferred_to_device
			FOREIGN KEY (transferred_to_device_id)
			REFERENCES iot.devices(id) ON DELETE SET NULL;

		DROP INDEX IF EXISTS iot.idx_devices_tenant_device_id;
		CREATE UNIQUE INDEX idx_devices_tenant_device_id_active
			ON iot.devices (tenant_id, device_id)
			WHERE archived_at IS NULL;

		CREATE INDEX IF NOT EXISTS idx_iot_devices_transfer_target
			ON iot.devices (transferred_to_device_id)
			WHERE transferred_to_device_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("add device transfer history: %w", err)
	}
	return nil
}

func deviceTransfersDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.269: Removing device transfer history...")
	_, err := db.ExecContext(ctx, `
		UPDATE iot.devices
		SET device_id = LEFT(device_id, 220) || '-ARCHIVED-' || id
		WHERE archived_at IS NOT NULL;

		DROP INDEX IF EXISTS iot.idx_iot_devices_transfer_target;
		DROP INDEX IF EXISTS iot.idx_devices_tenant_device_id_active;

		ALTER TABLE iot.devices
			DROP CONSTRAINT IF EXISTS fk_iot_devices_transferred_to_device,
			DROP COLUMN IF EXISTS transferred_to_device_id,
			DROP COLUMN IF EXISTS archived_at;

		CREATE UNIQUE INDEX idx_devices_tenant_device_id
			ON iot.devices (tenant_id, device_id);
	`)
	if err != nil {
		return fmt.Errorf("remove device transfer history: %w", err)
	}
	return nil
}
