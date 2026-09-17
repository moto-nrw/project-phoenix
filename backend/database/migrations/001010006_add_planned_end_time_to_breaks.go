package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	plannedEndTimeVersion     = "1.10.6"
	plannedEndTimeDescription = "Add planned_end_time column to work_session_breaks for auto-end support"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     plannedEndTimeVersion,
		Description: plannedEndTimeDescription,
		DependsOn:   []string{"1.10.5"}, // Depends on work_session_edits migration
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPlannedEndTimeColumn(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropPlannedEndTimeColumn(ctx, db)
		},
	)
}

func addPlannedEndTimeColumn(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// Add planned_end_time column (nullable - NULL means no auto-end)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.work_session_breaks
		ADD COLUMN IF NOT EXISTS planned_end_time TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error adding planned_end_time column: %w", err)
	}

	// Create index for efficient querying of expired breaks
	// Partial index only on active breaks (ended_at IS NULL) with a planned end time
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_wsb_planned_end_time
		ON active.work_session_breaks(planned_end_time)
		WHERE ended_at IS NULL AND planned_end_time IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating planned_end_time index: %w", err)
	}

	return tx.Commit()
}

func dropPlannedEndTimeColumn(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// Drop the index first
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_wsb_planned_end_time;
	`)
	if err != nil {
		return fmt.Errorf("error dropping planned_end_time index: %w", err)
	}

	// Drop the column
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.work_session_breaks
		DROP COLUMN IF EXISTS planned_end_time;
	`)
	if err != nil {
		return fmt.Errorf("error dropping planned_end_time column: %w", err)
	}

	return tx.Commit()
}
