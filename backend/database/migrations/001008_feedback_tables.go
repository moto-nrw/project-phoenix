package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FeedbackTablesVersion     = "1.8.0"
	FeedbackTablesDescription = "Student feedback tables"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FeedbackTablesVersion] = &Migration{
		Version:     FeedbackTablesVersion,
		Description: FeedbackTablesDescription,
		DependsOn:   []string{"1.7.0"}, // Depends on IoT tables
	}

	// Migration 1.8.0: Feedback schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return feedbackTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return feedbackTablesDown(ctx, db)
		},
	)
}

// feedbackTablesUp creates the feedback schema tables
func feedbackTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.0: Creating feedback schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create feedback schema
	_, err = tx.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS feedback;
	`)
	if err != nil {
		return fmt.Errorf("error creating feedback schema: %w", err)
	}

	// Create feedback entries table
	_, err = tx.ExecContext(ctx, `
		-- Student feedback tables
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

// feedbackTablesDown removes the feedback schema tables
func feedbackTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.0: Removing feedback schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS feedback.entries CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping feedback entries table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
