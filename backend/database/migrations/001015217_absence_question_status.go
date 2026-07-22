package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	absenceQuestionStatusVersion     = "1.15.217"
	absenceQuestionStatusDescription = "Add 'question' (Rückfrage) status to active.staff_absences status check"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     absenceQuestionStatusVersion,
		Description: absenceQuestionStatusDescription,
		DependsOn:   []string{"1.15.108"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addAbsenceQuestionStatus(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeAbsenceQuestionStatus(ctx, db)
		},
	)
}

func addAbsenceQuestionStatus(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.217: Adding 'question' to staff_absences status check...")

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
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS staff_absences_status_check;
	`)
	if err != nil {
		return fmt.Errorf("error dropping staff_absences_status_check: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT staff_absences_status_check
			CHECK (status IN ('reported','requested','approved','declined','canceled','question'));
	`)
	if err != nil {
		return fmt.Errorf("error adding staff_absences_status_check: %w", err)
	}

	fmt.Println("Migration 1.15.217: staff_absences_status_check now includes 'question'")
	return tx.Commit()
}

func removeAbsenceQuestionStatus(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.217: Removing 'question' from staff_absences status check...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Rows still in 'question' would violate the restored constraint — move
	// them back to 'requested' so rollback is always possible.
	_, err = tx.ExecContext(ctx, `
		UPDATE active.staff_absences SET status = 'requested' WHERE status = 'question';
	`)
	if err != nil {
		return fmt.Errorf("error resetting question rows: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS staff_absences_status_check;
	`)
	if err != nil {
		return fmt.Errorf("error dropping staff_absences_status_check: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT staff_absences_status_check
			CHECK (status IN ('reported','requested','approved','declined','canceled'));
	`)
	if err != nil {
		return fmt.Errorf("error restoring staff_absences_status_check: %w", err)
	}

	return tx.Commit()
}
