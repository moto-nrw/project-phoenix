package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	addSuggestionStatusesVersion     = "1.11.2"
	addSuggestionStatusesDescription = "Add in_progress and need_info statuses to suggestions"
)

func init() {
	MigrationRegistry[addSuggestionStatusesVersion] = &Migration{
		Version:     addSuggestionStatusesVersion,
		Description: addSuggestionStatusesDescription,
		DependsOn:   []string{"1.11.1"}, // Depends on platform schema (suggestions at 1.9.1 runs before by file order)
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addSuggestionStatuses(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeSuggestionStatuses(ctx, db)
		},
	)
}

func addSuggestionStatuses(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.11.2: Adding new suggestion statuses...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop existing constraint
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.posts
		DROP CONSTRAINT IF EXISTS posts_status_check;
	`)
	if err != nil {
		return fmt.Errorf("error dropping status constraint: %w", err)
	}

	// Add new constraint with additional statuses
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.posts
		ADD CONSTRAINT posts_status_check
		CHECK (status IN ('open', 'planned', 'in_progress', 'done', 'rejected', 'need_info'));
	`)
	if err != nil {
		return fmt.Errorf("error adding new status constraint: %w", err)
	}

	fmt.Println("Migration 1.11.2: Successfully added new suggestion statuses")
	return tx.Commit()
}

func removeSuggestionStatuses(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.11.2: Reverting suggestion statuses...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Update any posts with new statuses back to 'open'
	_, err = tx.ExecContext(ctx, `
		UPDATE suggestions.posts
		SET status = 'open'
		WHERE status IN ('in_progress', 'need_info');
	`)
	if err != nil {
		return fmt.Errorf("error reverting status values: %w", err)
	}

	// Drop new constraint
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.posts
		DROP CONSTRAINT IF EXISTS posts_status_check;
	`)
	if err != nil {
		return fmt.Errorf("error dropping new status constraint: %w", err)
	}

	// Restore original constraint
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.posts
		ADD CONSTRAINT posts_status_check
		CHECK (status IN ('open', 'planned', 'done', 'rejected'));
	`)
	if err != nil {
		return fmt.Errorf("error restoring original status constraint: %w", err)
	}

	fmt.Println("Migration 1.11.2: Successfully reverted suggestion statuses")
	return tx.Commit()
}
