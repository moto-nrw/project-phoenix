package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActiveGroupMappingsUpdatedAtVersion     = "1.4.11"
	ActiveGroupMappingsUpdatedAtDescription = "Add updated_at column to active.group_mappings"
)

var ActiveGroupMappingsUpdatedAtDependencies = []string{
	"1.4.5", // Depends on group_mappings table creation
}

func init() {
	MigrationRegistry[ActiveGroupMappingsUpdatedAtVersion] = &Migration{
		Version:     ActiveGroupMappingsUpdatedAtVersion,
		Description: ActiveGroupMappingsUpdatedAtDescription,
		DependsOn:   ActiveGroupMappingsUpdatedAtDependencies,
	}

	Migrations.MustRegister(addGroupMappingsUpdatedAt, removeGroupMappingsUpdatedAt)
}

func addGroupMappingsUpdatedAt(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.11: Adding updated_at column to active.group_mappings...")

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Add updated_at column
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.group_mappings
		ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	`)
	if err != nil {
		return fmt.Errorf("error adding updated_at column: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	fmt.Println("Migration 1.4.11: Successfully added updated_at column to active.group_mappings")
	return nil
}

func removeGroupMappingsUpdatedAt(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rollback 1.4.11: Removing updated_at column from active.group_mappings...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.group_mappings DROP COLUMN IF EXISTS updated_at
	`)
	if err != nil {
		return fmt.Errorf("error removing updated_at column: %w", err)
	}

	fmt.Println("Rollback 1.4.11: Successfully removed updated_at column from active.group_mappings")
	return nil
}
