package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ScheduleTimeframesVersion     = "1.1.2"
	ScheduleTimeframesDescription = "Create schedule.timeframes table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ScheduleTimeframesVersion] = &Migration{
		Version:     ScheduleTimeframesVersion,
		Description: ScheduleTimeframesDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth tables
	}

	// Migration 1.1.2: Create schedule.timeframes table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createScheduleTimeframesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropScheduleTimeframesTable(ctx, db)
		},
	)
}

// createScheduleTimeframesTable creates the schedule.timeframes table
func createScheduleTimeframesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.2: Creating schedule.timeframes table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the timeframes table - core time periods
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.timeframes (
			id BIGSERIAL PRIMARY KEY,
			start_time TIMESTAMPTZ NOT NULL,
			end_time TIMESTAMPTZ,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating timeframes table: %w", err)
	}

	// Create indexes for timeframes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_timeframes_active ON schedule.timeframes(is_active);
		CREATE INDEX IF NOT EXISTS idx_timeframes_time_range ON schedule.timeframes(start_time, end_time);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for timeframes table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for timeframes
		DROP TRIGGER IF EXISTS update_timeframes_updated_at ON schedule.timeframes;
		CREATE TRIGGER update_timeframes_updated_at
		BEFORE UPDATE ON schedule.timeframes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropScheduleTimeframesTable drops the schedule.timeframes table
func dropScheduleTimeframesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.2: Removing schedule.timeframes table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_timeframes_updated_at ON schedule.timeframes;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for timeframes table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.timeframes;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.timeframes table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
