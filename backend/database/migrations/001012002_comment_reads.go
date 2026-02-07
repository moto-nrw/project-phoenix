package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	commentReadsVersion     = "1.12.2"
	commentReadsDescription = "Create comment_reads table for tracking unread comments"
)

func init() {
	MigrationRegistry[commentReadsVersion] = &Migration{
		Version:     commentReadsVersion,
		Description: commentReadsDescription,
		DependsOn:   []string{"1.12.1"}, // Depends on unified comments
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createCommentReads(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackCommentReads(ctx, db)
		},
	)
}

func createCommentReads(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.2: Creating comment_reads table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create comment_reads table to track when users last read comments on a post
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS suggestions.comment_reads (
			account_id   BIGINT NOT NULL,
			post_id      BIGINT NOT NULL REFERENCES suggestions.posts(id) ON DELETE CASCADE,
			last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (account_id, post_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating suggestions.comment_reads table: %w", err)
	}

	// Create index for account lookups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_comment_reads_account ON suggestions.comment_reads(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating index for suggestions.comment_reads: %w", err)
	}

	fmt.Println("Migration 1.12.2: Successfully created comment_reads table")
	return tx.Commit()
}

func rollbackCommentReads(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.2: Dropping comment_reads table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS suggestions.comment_reads CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping suggestions.comment_reads table: %w", err)
	}

	fmt.Println("Migration 1.12.2: Successfully dropped comment_reads table")
	return tx.Commit()
}
