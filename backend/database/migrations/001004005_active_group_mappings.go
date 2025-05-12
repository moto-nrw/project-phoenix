package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActiveGroupMappingsVersion     = "1.4.5"
	ActiveGroupMappingsDescription = "Create active.group_mappings table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActiveGroupMappingsVersion] = &Migration{
		Version:     ActiveGroupMappingsVersion,
		Description: ActiveGroupMappingsDescription,
		DependsOn:   []string{"1.4.4", "1.4.1"}, // Depends on active.combined_groups and active.groups tables
	}

	// Migration 1.4.5: Create active.group_mappings table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActiveGroupMappingsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActiveGroupMappingsTable(ctx, db)
		},
	)
}

// createActiveGroupMappingsTable creates the active.group_mappings table
func createActiveGroupMappingsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.5: Creating active.group_mappings table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the active_group_mappings junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.group_mappings (
			id BIGSERIAL PRIMARY KEY,
			active_combined_group_id BIGINT NOT NULL,
			active_group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Foreign key constraints
			CONSTRAINT fk_active_group_mappings_active_combined_group FOREIGN KEY (active_combined_group_id)
				REFERENCES active.combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_active_group_mappings_active_group FOREIGN KEY (active_group_id)
				REFERENCES active.groups(id) ON DELETE CASCADE,

			-- Ensure an active_group can only be added once to an active_combined_group
			CONSTRAINT uq_active_group_mappings UNIQUE (active_combined_group_id, active_group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating active.group_mappings table: %w", err)
	}

	// Create indexes for active_group_mappings
	_, err = tx.ExecContext(ctx, `
		-- Indexes to speed up lookups
		CREATE INDEX IF NOT EXISTS idx_active_group_mappings_active_combined_group_id
			ON active.group_mappings(active_combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_active_group_mappings_active_group_id
			ON active.group_mappings(active_group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for active_group_mappings table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActiveGroupMappingsTable drops the active.group_mappings table
func dropActiveGroupMappingsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.5: Removing active.group_mappings table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.group_mappings CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.group_mappings table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
