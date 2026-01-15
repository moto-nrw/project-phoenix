package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ScheduleDateframesVersion     = "1.1.3"
	ScheduleDateframesDescription = "Create schedule.dateframes table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ScheduleDateframesVersion] = &Migration{
		Version:     ScheduleDateframesVersion,
		Description: ScheduleDateframesDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth tables
	}

	// Migration 1.1.3: Create schedule.dateframes table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createScheduleDateframesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropScheduleDateframesTable(ctx, db)
		},
	)
}

// createScheduleDateframesTable creates the schedule.dateframes table
func createScheduleDateframesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.3: Creating schedule.dateframes table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackError(err)
		}
	}()

	// Create the dateframes table - date ranges for planning
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.dateframes (
			id BIGSERIAL PRIMARY KEY,
			start_date TIMESTAMPTZ NOT NULL,
			end_date TIMESTAMPTZ NOT NULL,
			name TEXT,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating dateframes table: %w", err)
	}

	// Create indexes for dateframes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_dateframes_date_range ON schedule.dateframes(start_date, end_date);
		CREATE INDEX IF NOT EXISTS idx_dateframes_name ON schedule.dateframes(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for dateframes table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for dateframes
		DROP TRIGGER IF EXISTS update_dateframes_updated_at ON schedule.dateframes;
		CREATE TRIGGER update_dateframes_updated_at
		BEFORE UPDATE ON schedule.dateframes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropScheduleDateframesTable drops the schedule.dateframes table
func dropScheduleDateframesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.3: Removing schedule.dateframes table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackError(err)
		}
	}()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_dateframes_updated_at ON schedule.dateframes;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for dateframes table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.dateframes;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.dateframes table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
