package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ActiveGroupsVersion     = "1.4.1"
	ActiveGroupsDescription = "Create active.groups table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActiveGroupsVersion] = &Migration{
		Version:     ActiveGroupsVersion,
		Description: ActiveGroupsDescription,
		DependsOn:   []string{"1.3.9", "1.3.2", "1.1.1"}, // Depends on IoT devices, activity groups, and rooms
	}

	// Migration 1.4.1: Create active.groups table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActiveGroupsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActiveGroupsTable(ctx, db)
		},
	)
}

// createActiveGroupsTable creates the active.groups table
func createActiveGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.1: Creating active.groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the active_groups table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.groups (
			id BIGSERIAL PRIMARY KEY,
			start_time TIMESTAMPTZ NOT NULL, -- Required start time
			end_time TIMESTAMPTZ,           -- Optional end time
			group_id BIGINT NOT NULL,        -- Reference to activities.groups
			device_id BIGINT,                -- Reference to iot.devices (optional for RFID)
			room_id BIGINT NOT NULL,         -- Reference to facilities.rooms
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			
			-- Foreign key constraints
			CONSTRAINT fk_active_groups_group FOREIGN KEY (group_id)
				REFERENCES activities.groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_active_groups_device FOREIGN KEY (device_id)
				REFERENCES iot.devices(id) ON DELETE RESTRICT,
			CONSTRAINT fk_active_groups_room FOREIGN KEY (room_id)
				REFERENCES facilities.rooms(id) ON DELETE RESTRICT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating active.groups table: %w", err)
	}

	// Create indexes for active_groups - improve query performance
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_active_groups_start_time ON active.groups(start_time);
		CREATE INDEX IF NOT EXISTS idx_active_groups_end_time ON active.groups(end_time);
		CREATE INDEX IF NOT EXISTS idx_active_groups_group_id ON active.groups(group_id);
		CREATE INDEX IF NOT EXISTS idx_active_groups_device_id ON active.groups(device_id);
		CREATE INDEX IF NOT EXISTS idx_active_groups_room_id ON active.groups(room_id);

		-- Index for finding active sessions (where end_time is null)
		CREATE INDEX IF NOT EXISTS idx_active_groups_currently_active ON active.groups(group_id)
		WHERE end_time IS NULL;

		-- Performance indexes for session conflict detection
		-- Composite index for activity conflict detection: WHERE group_id = ? AND end_time IS NULL
		CREATE INDEX IF NOT EXISTS idx_active_groups_conflict_detection
		ON active.groups(group_id, device_id, end_time) WHERE end_time IS NULL;

		-- Index for device session lookup: WHERE device_id = ? AND end_time IS NULL
		CREATE INDEX IF NOT EXISTS idx_active_groups_device_session
		ON active.groups(device_id, end_time) WHERE device_id IS NOT NULL AND end_time IS NULL;

		-- Index for room-based active queries: WHERE room_id = ? AND end_time IS NULL  
		CREATE INDEX IF NOT EXISTS idx_active_groups_room_active
		ON active.groups(room_id, end_time) WHERE end_time IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for active_groups table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for active_groups
		DROP TRIGGER IF EXISTS update_active_groups_updated_at ON active.groups;
		CREATE TRIGGER update_active_groups_updated_at
		BEFORE UPDATE ON active.groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for active_groups: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActiveGroupsTable drops the active.groups table
func dropActiveGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.1: Removing active.groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_active_groups_updated_at ON active.groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for active_groups table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.groups CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
