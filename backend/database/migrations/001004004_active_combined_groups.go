package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActiveCombinedGroupsVersion     = "1.4.4"
	ActiveCombinedGroupsDescription = "Create active.active_combined_groups table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActiveCombinedGroupsVersion] = &Migration{
		Version:     ActiveCombinedGroupsVersion,
		Description: ActiveCombinedGroupsDescription,
		DependsOn:   []string{"1.4.1"}, // Depends on active.active_groups table
	}

	// Migration 1.4.4: Create active.active_combined_groups table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActiveCombinedGroupsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActiveCombinedGroupsTable(ctx, db)
		},
	)
}

// createActiveCombinedGroupsTable creates the active.active_combined_groups table
func createActiveCombinedGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.4: Creating active.active_combined_groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the active_combined_groups table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.active_combined_groups (
			id BIGSERIAL PRIMARY KEY,
			start_time TIMESTAMPTZ NOT NULL, -- Required start time
			end_time TIMESTAMPTZ,           -- Optional end time
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating active.active_combined_groups table: %w", err)
	}

	// Create indexes for active_combined_groups
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_active_combined_groups_start_time
			ON active.active_combined_groups(start_time);
		CREATE INDEX IF NOT EXISTS idx_active_combined_groups_end_time
			ON active.active_combined_groups(end_time);

		-- Index for finding active sessions (where end_time is null)
		CREATE INDEX IF NOT EXISTS idx_active_combined_groups_currently_active
			ON active.active_combined_groups(id)
			WHERE end_time IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for active_combined_groups table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for active_combined_groups
		DROP TRIGGER IF EXISTS update_active_combined_groups_updated_at ON active.active_combined_groups;
		CREATE TRIGGER update_active_combined_groups_updated_at
		BEFORE UPDATE ON active.active_combined_groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for active_combined_groups: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActiveCombinedGroupsTable drops the active.active_combined_groups table
func dropActiveCombinedGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.4: Removing active.active_combined_groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_active_combined_groups_updated_at ON active.active_combined_groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for active_combined_groups table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.active_combined_groups CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.active_combined_groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
