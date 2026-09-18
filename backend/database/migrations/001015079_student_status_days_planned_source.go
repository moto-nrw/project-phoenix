package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentStatusDaysPlannedSourceVersion     = "1.15.79"
	studentStatusDaysPlannedSourceDescription = "Allow planned source for active.student_status_days"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentStatusDaysPlannedSourceVersion,
		Description: studentStatusDaysPlannedSourceDescription,
		DependsOn:   []string{studentStatusDaysVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysPlannedSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysPlannedSourceDown(ctx, db)
		},
	)
}

func studentStatusDaysPlannedSourceUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.student_status_days
			DROP CONSTRAINT IF EXISTS chk_student_status_days_source;

		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_source
			CHECK (source IN ('manual', 'planned', 'next_checkin', 'end_of_day'));
	`)
	if err != nil {
		return fmt.Errorf("error allowing planned student status day source: %w", err)
	}

	return tx.Commit()
}

func studentStatusDaysPlannedSourceDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		UPDATE active.student_status_days
		SET source = 'manual'
		WHERE source = 'planned';

		ALTER TABLE active.student_status_days
			DROP CONSTRAINT IF EXISTS chk_student_status_days_source;

		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_source
			CHECK (source IN ('manual', 'next_checkin', 'end_of_day'));
	`)
	if err != nil {
		return fmt.Errorf("error removing planned student status day source: %w", err)
	}

	return tx.Commit()
}
