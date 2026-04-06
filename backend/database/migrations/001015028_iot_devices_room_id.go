package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	iotDevicesRoomIDVersion     = "1.15.28"
	iotDevicesRoomIDDescription = "Add room_id FK to iot.devices"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     iotDevicesRoomIDVersion,
		Description: iotDevicesRoomIDDescription,
		DependsOn:   []string{"1.15.27", "1.3.9"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addIoTDevicesRoomID(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropIoTDevicesRoomID(ctx, db)
		},
	)
}

func addIoTDevicesRoomID(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.28: Adding room_id to iot.devices...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in iot devices room_id migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE iot.devices ADD COLUMN IF NOT EXISTS room_id BIGINT;

		ALTER TABLE iot.devices
			ADD CONSTRAINT fk_iot_devices_room
			FOREIGN KEY (room_id) REFERENCES facilities.rooms(id) ON DELETE SET NULL;

		CREATE INDEX IF NOT EXISTS idx_iot_devices_room_id ON iot.devices(room_id);
	`)
	if err != nil {
		return fmt.Errorf("error adding room_id to iot.devices: %w", err)
	}

	return tx.Commit()
}

func dropIoTDevicesRoomID(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.28: Removing room_id from iot.devices...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in iot devices room_id down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS idx_iot_devices_room_id;
		ALTER TABLE iot.devices DROP CONSTRAINT IF EXISTS fk_iot_devices_room;
		ALTER TABLE iot.devices DROP COLUMN IF EXISTS room_id;
	`)
	if err != nil {
		return fmt.Errorf("error removing room_id from iot.devices: %w", err)
	}

	return tx.Commit()
}
