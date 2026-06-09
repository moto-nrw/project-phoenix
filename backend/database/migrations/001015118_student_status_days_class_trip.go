package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentStatusDaysClassTripVersion     = "1.15.118"
	studentStatusDaysClassTripDescription = "Allow class trip status for active.student_status_days"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentStatusDaysClassTripVersion,
		Description: studentStatusDaysClassTripDescription,
		DependsOn:   []string{studentStatusDaysParentSourceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysClassTripUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysClassTripDown(ctx, db)
		},
	)
}

func studentStatusDaysClassTripUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.118: Allowing class trip student status days...")

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
			DROP CONSTRAINT IF EXISTS chk_student_status_days_status;

		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_status
			CHECK (status IN ('sick', 'excused', 'class_trip'));
	`)
	if err != nil {
		return fmt.Errorf("error allowing class trip student status days: %w", err)
	}

	return tx.Commit()
}

func studentStatusDaysClassTripDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.118: Removing class trip student status days...")

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
		DELETE FROM active.student_status_days
		WHERE status = 'class_trip';

		ALTER TABLE active.student_status_days
			DROP CONSTRAINT IF EXISTS chk_student_status_days_status;

		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_status
			CHECK (status IN ('sick', 'excused'));
	`)
	if err != nil {
		return fmt.Errorf("error removing class trip student status days: %w", err)
	}

	return tx.Commit()
}
