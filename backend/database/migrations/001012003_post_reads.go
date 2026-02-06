package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	postReadsVersion     = "1.12.3"
	postReadsDescription = "Create post_reads table for tracking viewed posts"
)

func init() {
	MigrationRegistry[postReadsVersion] = &Migration{
		Version:     postReadsVersion,
		Description: postReadsDescription,
		DependsOn:   []string{"1.12.2"}, // Depends on comment_reads
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createPostReads(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackPostReads(ctx, db)
		},
	)
}

func createPostReads(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.3: Creating post_reads table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create post_reads table to track when operators viewed a post
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS suggestions.post_reads (
			account_id  BIGINT NOT NULL,
			post_id     BIGINT NOT NULL REFERENCES suggestions.posts(id) ON DELETE CASCADE,
			viewed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (account_id, post_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating suggestions.post_reads table: %w", err)
	}

	// Create index for account lookups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_post_reads_account ON suggestions.post_reads(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating index for suggestions.post_reads: %w", err)
	}

	// Add comment
	_, err = tx.ExecContext(ctx, `
		COMMENT ON TABLE suggestions.post_reads IS 'Tracks which posts have been viewed by which operator accounts';
	`)
	if err != nil {
		return fmt.Errorf("error adding comment to suggestions.post_reads: %w", err)
	}

	fmt.Println("Migration 1.12.3: Successfully created post_reads table")
	return tx.Commit()
}

func rollbackPostReads(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.3: Dropping post_reads table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS suggestions.post_reads CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions.post_reads table: %w", err)
	}

	fmt.Println("Migration 1.12.3: Successfully dropped post_reads table")
	return tx.Commit()
}
