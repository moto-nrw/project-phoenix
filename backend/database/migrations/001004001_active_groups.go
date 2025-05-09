package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActiveGroupsVersion     = "1.4.1"
	ActiveGroupsDescription = "Create activities.active_groups table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActiveGroupsVersion] = &Migration{
		Version:     ActiveGroupsVersion,
		Description: ActiveGroupsDescription,
		DependsOn:   []string{"1.3.9", "1.3.2", "1.1.1"}, // Depends on IoT devices, activity groups, and rooms
	}

	// Migration 1.3.6: Create activities.active_groups table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActiveGroupsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActiveGroupsTable(ctx, db)
		},
	)
}

// createActiveGroupsTable creates the activities.active_groups table
func createActiveGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.1: Creating activities.active_groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the active_groups table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.active_groups (
			id BIGSERIAL PRIMARY KEY,
			start_time TIMESTAMPTZ NOT NULL, -- Required start time
			end_time TIMESTAMPTZ,           -- Optional end time
			group_id BIGINT NOT NULL,        -- Reference to activities.groups
			device_id BIGINT NOT NULL,       -- Reference to iot.devices
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
		return fmt.Errorf("error creating activities.active_groups table: %w", err)
	}

	// Create indexes for active_groups - improve query performance
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_active_groups_start_time ON activities.active_groups(start_time);
		CREATE INDEX IF NOT EXISTS idx_active_groups_end_time ON activities.active_groups(end_time);
		CREATE INDEX IF NOT EXISTS idx_active_groups_group_id ON activities.active_groups(group_id);
		CREATE INDEX IF NOT EXISTS idx_active_groups_device_id ON activities.active_groups(device_id);
		CREATE INDEX IF NOT EXISTS idx_active_groups_room_id ON activities.active_groups(room_id);
		
		-- Index for finding active sessions (where end_time is null)
		CREATE INDEX IF NOT EXISTS idx_active_groups_currently_active ON activities.active_groups(group_id) 
		WHERE end_time IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for active_groups table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for active_groups
		DROP TRIGGER IF EXISTS update_active_groups_updated_at ON activities.active_groups;
		CREATE TRIGGER update_active_groups_updated_at
		BEFORE UPDATE ON activities.active_groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for active_groups: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActiveGroupsTable drops the activities.active_groups table
func dropActiveGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.1: Removing activities.active_groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_active_groups_updated_at ON activities.active_groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for active_groups table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.active_groups CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.active_groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
