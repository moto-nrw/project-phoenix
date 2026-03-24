package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	suggestionsPerformanceIndexesVersion     = "1.15.15"
	suggestionsPerformanceIndexesDescription = "Add composite indexes for suggestions list query performance"
)

func init() {
	MigrationRegistry[suggestionsPerformanceIndexesVersion] = &Migration{
		Version:     suggestionsPerformanceIndexesVersion,
		Description: suggestionsPerformanceIndexesDescription,
		DependsOn:   []string{"1.15.14"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createSuggestionsPerformanceIndexes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropSuggestionsPerformanceIndexes(ctx, db)
		},
	)
}

func createSuggestionsPerformanceIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.15: Adding suggestions performance indexes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Composite index on votes for aggregating upvotes/downvotes per post.
	// The List query counts votes by (post_id, direction) — this index turns
	// two sequential scans per row into two index-only scans.
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_suggestions_votes_post_direction
		ON suggestions.votes(post_id, direction);
	`)
	if err != nil {
		return fmt.Errorf("error creating votes composite index: %w", err)
	}

	// Partial index on active (non-deleted) comments per post.
	// The List query counts comments WHERE deleted_at IS NULL per post_id.
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_suggestions_comments_post_active
		ON suggestions.comments(post_id) WHERE deleted_at IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating comments partial index: %w", err)
	}

	// Composite index on comment_reads for the unread-count LATERAL join.
	// The join matches (post_id, account_id, reader_type) and reads last_read_at,
	// so including last_read_at makes this an index-only scan.
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_suggestions_comment_reads_post_account
		ON suggestions.comment_reads(post_id, account_id, reader_type)
		INCLUDE (last_read_at);
	`)
	if err != nil {
		return fmt.Errorf("error creating comment_reads composite index: %w", err)
	}

	// post_reads already has PRIMARY KEY (account_id, post_id, reader_type)
	// which covers the NOT EXISTS probe — no additional index needed.

	fmt.Println("Migration 1.15.15: Successfully added suggestions performance indexes")
	return tx.Commit()
}

func dropSuggestionsPerformanceIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.15: Removing suggestions performance indexes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS suggestions.idx_suggestions_votes_post_direction;
		DROP INDEX IF EXISTS suggestions.idx_suggestions_comments_post_active;
		DROP INDEX IF EXISTS suggestions.idx_suggestions_comment_reads_post_account;
	`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions performance indexes: %w", err)
	}

	fmt.Println("Migration 1.15.15: Successfully removed suggestions performance indexes")
	return tx.Commit()
}
