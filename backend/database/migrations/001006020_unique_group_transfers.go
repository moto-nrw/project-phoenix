package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UniqueGroupTransfersVersion     = "1.6.20"
	UniqueGroupTransfersDescription = "Add unique constraint for group transfers to prevent duplicate transfers"
)

func init() {
	MigrationRegistry[UniqueGroupTransfersVersion] = &Migration{
		Version:     UniqueGroupTransfersVersion,
		Description: UniqueGroupTransfersDescription,
		DependsOn: []string{
			"1.3.2", // Depends on education.group_substitution table
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addUniqueGroupTransfersConstraintUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addUniqueGroupTransfersConstraintDown(ctx, db)
		},
	)
}

func addUniqueGroupTransfersConstraintUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.20: Adding unique constraint for group transfers...")

	// Add unique constraint to prevent duplicate transfers
	// A transfer is identified by: group_id + substitute_staff_id + start_date
	// Only applies to transfers (regular_staff_id IS NULL)
	_, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_no_duplicate_group_transfers
		ON education.group_substitution(group_id, substitute_staff_id, start_date)
		WHERE regular_staff_id IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating unique index for group transfers: %w", err)
	}

	fmt.Println("Migration 1.6.20: Successfully added unique constraint for group transfers")
	return nil
}

func addUniqueGroupTransfersConstraintDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.20: Removing unique constraint for group transfers...")

	// Drop the unique index
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS education.idx_no_duplicate_group_transfers;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping unique index for group transfers: %w", err)
	}

	fmt.Println("Rolling back migration 1.6.20: Successfully removed unique constraint")
	return nil
}
