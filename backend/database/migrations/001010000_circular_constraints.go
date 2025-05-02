package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	CircularConstraintsVersion     = "1.10.0"
	CircularConstraintsDescription = "Resolving circular dependencies in foreign key constraints"
)

func init() {
	// Migration 10: Resolving circular dependencies in foreign key constraints
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return circularConstraintsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return circularConstraintsDown(ctx, db)
		},
	)
}

// circularConstraintsUp adds foreign key constraints that couldn't be added earlier due to circular dependencies
func circularConstraintsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Adding foreign key constraints with circular dependencies...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Add Group.representative_id →  students(id)
	// This creates a circular dependency because students has group_id → Group(id)
	_, err = tx.ExecContext(ctx, `
		-- First, add the representative_id column to the groups table if it doesn't exist
		ALTER TABLE groups 
		ADD COLUMN IF NOT EXISTS representative_id BIGINT;

		-- Add the foreign key constraint to students table
		ALTER TABLE groups
		ADD CONSTRAINT fk_groups_representative FOREIGN KEY (representative_id) 
		REFERENCES  students(id) ON DELETE SET NULL;

		-- Create an index for the foreign key
		CREATE INDEX IF NOT EXISTS idx_groups_representative_id ON groups(representative_id);
	`)
	if err != nil {
		return fmt.Errorf("error adding representative_id foreign key to groups table: %w", err)
	}

	// 2. Add Room_occupancy.ag_id → Ag(id)
	// This creates an indirect circular dependency through other related tables
	_, err = tx.ExecContext(ctx, `
		-- First, add the ag_id column to the room_occupancy table if it doesn't exist
		ALTER TABLE room_occupancy 
		ADD COLUMN IF NOT EXISTS ag_id BIGINT;

		-- Add the foreign key constraint
		ALTER TABLE room_occupancy
		ADD CONSTRAINT fk_room_occupancy_ag FOREIGN KEY (ag_id) 
		REFERENCES ag(id) ON DELETE SET NULL;

		-- Create an index for the foreign key
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_ag_id ON room_occupancy(ag_id);
	`)
	if err != nil {
		return fmt.Errorf("error adding ag_id foreign key to room_occupancy table: %w", err)
	}

	// 3. Add Room_occupancy.group_id → Group(id)
	// This creates an indirect circular dependency
	_, err = tx.ExecContext(ctx, `
		-- First, add the group_id column to the room_occupancy table if it doesn't exist
		ALTER TABLE room_occupancy 
		ADD COLUMN IF NOT EXISTS group_id BIGINT;

		-- Add the foreign key constraint
		ALTER TABLE room_occupancy
		ADD CONSTRAINT fk_room_occupancy_group FOREIGN KEY (group_id) 
		REFERENCES groups(id) ON DELETE SET NULL;

		-- Create an index for the foreign key
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_group_id ON room_occupancy(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error adding group_id foreign key to room_occupancy table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// circularConstraintsDown removes the foreign key constraints added in circularConstraintsUp
func circularConstraintsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back foreign key constraints with circular dependencies...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove the foreign key constraints and indexes in reverse order

	// 1. Remove Room_occupancy.group_id → Group(id)
	_, err = tx.ExecContext(ctx, `
		-- Drop the foreign key constraint
		ALTER TABLE room_occupancy
		DROP CONSTRAINT IF EXISTS fk_room_occupancy_group;

		-- Drop the index
		DROP INDEX IF EXISTS idx_room_occupancy_group_id;
		
		-- Drop the column
		ALTER TABLE room_occupancy
		DROP COLUMN IF EXISTS group_id;
	`)
	if err != nil {
		return fmt.Errorf("error removing group_id foreign key from room_occupancy table: %w", err)
	}

	// 2. Remove Room_occupancy.ag_id → Ag(id)
	_, err = tx.ExecContext(ctx, `
		-- Drop the foreign key constraint
		ALTER TABLE room_occupancy
		DROP CONSTRAINT IF EXISTS fk_room_occupancy_ag;

		-- Drop the index
		DROP INDEX IF EXISTS idx_room_occupancy_ag_id;
		
		-- Drop the column
		ALTER TABLE room_occupancy
		DROP COLUMN IF EXISTS ag_id;
	`)
	if err != nil {
		return fmt.Errorf("error removing ag_id foreign key from room_occupancy table: %w", err)
	}

	// 3. Remove Group.representative_id →  students(id)
	_, err = tx.ExecContext(ctx, `
		-- Drop the foreign key constraint
		ALTER TABLE groups
		DROP CONSTRAINT IF EXISTS fk_groups_representative;

		-- Drop the index
		DROP INDEX IF EXISTS idx_groups_representative_id;
		
		-- Drop the column
		ALTER TABLE groups
		DROP COLUMN IF EXISTS representative_id;
	`)
	if err != nil {
		return fmt.Errorf("error removing representative_id foreign key from groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
