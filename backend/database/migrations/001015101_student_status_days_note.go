package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentStatusDaysNoteVersion     = "1.15.101"
	studentStatusDaysNoteDescription = "Add note column to active.student_status_days for parent-supplied sick-note reasons"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentStatusDaysNoteVersion,
		Description: studentStatusDaysNoteDescription,
		DependsOn:   []string{studentStatusDaysVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysNoteUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysNoteDown(ctx, db)
		},
	)
}

func studentStatusDaysNoteUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.101: Adding note column to active.student_status_days...")

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
		ALTER TABLE active.student_status_days
			ADD COLUMN IF NOT EXISTS note TEXT;
	`)
	if err != nil {
		return fmt.Errorf("error adding note column to active.student_status_days: %w", err)
	}

	return tx.Commit()
}

func studentStatusDaysNoteDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.101: Dropping note column from active.student_status_days...")

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
		ALTER TABLE active.student_status_days
			DROP COLUMN IF EXISTS note;
	`)
	if err != nil {
		return fmt.Errorf("error dropping note column from active.student_status_days: %w", err)
	}

	return tx.Commit()
}
