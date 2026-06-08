package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentStatusDaysParentSourceVersion     = "1.15.110"
	studentStatusDaysParentSourceDescription = "Allow parent source for active.student_status_days"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentStatusDaysParentSourceVersion,
		Description: studentStatusDaysParentSourceDescription,
		DependsOn:   []string{studentStatusDaysPlannedSourceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysParentSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysParentSourceDown(ctx, db)
		},
	)
}

func studentStatusDaysParentSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.110: Allowing parent student status day source...")

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
			DROP CONSTRAINT IF EXISTS chk_student_status_days_source;

		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_source
			CHECK (source IN ('manual', 'planned', 'next_checkin', 'end_of_day', 'parent'));
	`)
	if err != nil {
		return fmt.Errorf("error allowing parent student status day source: %w", err)
	}

	return tx.Commit()
}

func studentStatusDaysParentSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.110: Removing parent student status day source...")

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
		UPDATE active.student_status_days
		SET source = 'manual'
		WHERE source = 'parent';

		ALTER TABLE active.student_status_days
			DROP CONSTRAINT IF EXISTS chk_student_status_days_source;

		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_source
			CHECK (source IN ('manual', 'planned', 'next_checkin', 'end_of_day'));
	`)
	if err != nil {
		return fmt.Errorf("error removing parent student status day source: %w", err)
	}

	return tx.Commit()
}
