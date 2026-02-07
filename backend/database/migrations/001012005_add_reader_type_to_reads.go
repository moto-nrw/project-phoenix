package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	readerTypeVersion     = "1.12.5"
	readerTypeDescription = "Add reader_type column to comment_reads and post_reads for namespace isolation"
)

func init() {
	MigrationRegistry[readerTypeVersion] = &Migration{
		Version:     readerTypeVersion,
		Description: readerTypeDescription,
		DependsOn:   []string{"1.12.3"}, // Depends on post_reads
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createReaderType(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackReaderType(ctx, db)
		},
	)
}

func createReaderType(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.5: Adding reader_type column to comment_reads and post_reads...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Add reader_type to comment_reads (existing rows default to 'user')
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.comment_reads
		ADD COLUMN IF NOT EXISTS reader_type TEXT NOT NULL DEFAULT 'user';
	`)
	if err != nil {
		return fmt.Errorf("error adding reader_type to comment_reads: %w", err)
	}

	// Drop old PK and create new one including reader_type
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.comment_reads
		DROP CONSTRAINT IF EXISTS comment_reads_pkey;
	`)
	if err != nil {
		return fmt.Errorf("error dropping old comment_reads PK: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.comment_reads
		ADD PRIMARY KEY (account_id, post_id, reader_type);
	`)
	if err != nil {
		return fmt.Errorf("error creating new comment_reads PK: %w", err)
	}

	// Add reader_type to post_reads (existing rows default to 'user')
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.post_reads
		ADD COLUMN IF NOT EXISTS reader_type TEXT NOT NULL DEFAULT 'user';
	`)
	if err != nil {
		return fmt.Errorf("error adding reader_type to post_reads: %w", err)
	}

	// Drop old PK and create new one including reader_type
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.post_reads
		DROP CONSTRAINT IF EXISTS post_reads_pkey;
	`)
	if err != nil {
		return fmt.Errorf("error dropping old post_reads PK: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.post_reads
		ADD PRIMARY KEY (account_id, post_id, reader_type);
	`)
	if err != nil {
		return fmt.Errorf("error creating new post_reads PK: %w", err)
	}

	// Update indexes to include reader_type
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS suggestions.idx_comment_reads_account;
		CREATE INDEX idx_comment_reads_account_type ON suggestions.comment_reads(account_id, reader_type);
	`)
	if err != nil {
		return fmt.Errorf("error updating comment_reads index: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS suggestions.idx_post_reads_account;
		CREATE INDEX idx_post_reads_account_type ON suggestions.post_reads(account_id, reader_type);
	`)
	if err != nil {
		return fmt.Errorf("error updating post_reads index: %w", err)
	}

	fmt.Println("Migration 1.12.5: Successfully added reader_type to comment_reads and post_reads")
	return tx.Commit()
}

func rollbackReaderType(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.5: Removing reader_type from comment_reads and post_reads...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Restore comment_reads
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.comment_reads DROP CONSTRAINT IF EXISTS comment_reads_pkey;
		DROP INDEX IF EXISTS suggestions.idx_comment_reads_account_type;
		ALTER TABLE suggestions.comment_reads DROP COLUMN IF EXISTS reader_type;
		ALTER TABLE suggestions.comment_reads ADD PRIMARY KEY (account_id, post_id);
		CREATE INDEX IF NOT EXISTS idx_comment_reads_account ON suggestions.comment_reads(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring comment_reads: %w", err)
	}

	// Restore post_reads
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.post_reads DROP CONSTRAINT IF EXISTS post_reads_pkey;
		DROP INDEX IF EXISTS suggestions.idx_post_reads_account_type;
		ALTER TABLE suggestions.post_reads DROP COLUMN IF EXISTS reader_type;
		ALTER TABLE suggestions.post_reads ADD PRIMARY KEY (account_id, post_id);
		CREATE INDEX IF NOT EXISTS idx_post_reads_account ON suggestions.post_reads(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring post_reads: %w", err)
	}

	fmt.Println("Migration 1.12.5: Successfully removed reader_type from comment_reads and post_reads")
	return tx.Commit()
}
