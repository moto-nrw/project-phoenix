package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	removeIsInternalVersion     = "1.12.6"
	removeIsInternalDescription = "Remove unused is_internal column from suggestions.comments"
)

func init() {
	MigrationRegistry[removeIsInternalVersion] = &Migration{
		Version:     removeIsInternalVersion,
		Description: removeIsInternalDescription,
		DependsOn:   []string{"1.12.1"}, // Depends on unified_suggestion_comments
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return removeIsInternal(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackRemoveIsInternal(ctx, db)
		},
	)
}

func removeIsInternal(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.6: Removing is_internal column from suggestions.comments...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.comments
		DROP COLUMN IF EXISTS is_internal;
	`)
	if err != nil {
		return fmt.Errorf("error dropping is_internal column: %w", err)
	}

	fmt.Println("Migration 1.12.6: Successfully removed is_internal from suggestions.comments")
	return nil
}

func rollbackRemoveIsInternal(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.6: Re-adding is_internal column to suggestions.comments...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE suggestions.comments
		ADD COLUMN IF NOT EXISTS is_internal BOOLEAN NOT NULL DEFAULT false;
	`)
	if err != nil {
		return fmt.Errorf("error re-adding is_internal column: %w", err)
	}

	fmt.Println("Migration 1.12.6: Successfully re-added is_internal to suggestions.comments")
	return nil
}
