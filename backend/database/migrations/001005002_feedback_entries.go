package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FeedbackEntriesVersion     = "1.5.2"
	FeedbackEntriesDescription = "Create feedback.entries table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FeedbackEntriesVersion] = &Migration{
		Version:     FeedbackEntriesVersion,
		Description: FeedbackEntriesDescription,
		DependsOn:   []string{"1.2.0"}, // Depends on users_rfid_cards table
	}

	// Migration 1.5.2: Create feedback.entries table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFeedbackEntriesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFeedbackEntriesTable(ctx, db)
		},
	)
}

// createFeedbackEntriesTable creates the feedback.entries table
func createFeedbackEntriesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.5.2: Creating feedback.entries table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create feedback schema if it doesn't exist
	_, err = tx.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS feedback;
	`)
	if err != nil {
		return fmt.Errorf("error creating feedback schema: %w", err)
	}

	// Create feedback entries table
	_, err = tx.ExecContext(ctx, `
		-- Student feedback table
		CREATE TABLE IF NOT EXISTS feedback.entries (
			id BIGSERIAL PRIMARY KEY,
			value TEXT NOT NULL,
			day DATE NOT NULL,
			time TIME NOT NULL,
			student_id BIGINT NOT NULL,
			is_mensa_feedback BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_feedback_entries_student FOREIGN KEY (student_id)
				REFERENCES users.students(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating feedback entries table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_feedback_entries_student_id ON feedback.entries(student_id);
		CREATE INDEX IF NOT EXISTS idx_feedback_entries_day ON feedback.entries(day);
		CREATE INDEX IF NOT EXISTS idx_feedback_entries_is_mensa_feedback ON feedback.entries(is_mensa_feedback);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for feedback entries table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for feedback entries
		DROP TRIGGER IF EXISTS update_feedback_entries_updated_at ON feedback.entries;
		CREATE TRIGGER update_feedback_entries_updated_at
		BEFORE UPDATE ON feedback.entries
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for feedback entries: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropFeedbackEntriesTable drops the feedback.entries table
func dropFeedbackEntriesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.5.2: Removing feedback.entries table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_feedback_entries_updated_at ON feedback.entries;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for feedback.entries table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS feedback.entries CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping feedback.entries table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
