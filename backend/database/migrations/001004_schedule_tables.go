package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ScheduleTablesVersion     = "1.4.0"
	ScheduleTablesDescription = "Schedule schema tables for time management"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ScheduleTablesVersion] = &Migration{
		Version:     ScheduleTablesVersion,
		Description: ScheduleTablesDescription,
		DependsOn:   []string{"1.3.0"}, // Depends on groups tables
	}

	// Migration 1.4.0: Schedule schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return scheduleTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return scheduleTablesDown(ctx, db)
		},
	)
}

// scheduleTablesUp creates the schedule schema tables
func scheduleTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.0: Creating schedule schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the timeframes table - core time periods
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

	// 2. Create the dateframes table - date ranges for planning
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

	// 3. Create the recurrence_rules table - for recurring events
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.recurrence_rules (
			id BIGSERIAL PRIMARY KEY,
			frequency TEXT NOT NULL, -- daily, weekly, monthly, etc.
			interval_count INT NOT NULL DEFAULT 1,
			weekdays TEXT[], -- array of weekdays (e.g., ['MON', 'WED', 'FRI'])
			month_days INT[], -- array of days of month
			end_date TIMESTAMPTZ,
			count INT, -- number of occurrences
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating recurrence_rules table: %w", err)
	}

	// Create triggers for updating updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Trigger for timeframes
		DROP TRIGGER IF EXISTS update_timeframes_updated_at ON schedule.timeframes;
		CREATE TRIGGER update_timeframes_updated_at
		BEFORE UPDATE ON schedule.timeframes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for dateframes
		DROP TRIGGER IF EXISTS update_dateframes_updated_at ON schedule.dateframes;
		CREATE TRIGGER update_dateframes_updated_at
		BEFORE UPDATE ON schedule.dateframes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for recurrence_rules
		DROP TRIGGER IF EXISTS update_recurrence_rules_updated_at ON schedule.recurrence_rules;
		CREATE TRIGGER update_recurrence_rules_updated_at
		BEFORE UPDATE ON schedule.recurrence_rules
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// scheduleTablesDown removes the schedule schema tables
func scheduleTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.0: Removing schedule schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.recurrence_rules;
		DROP TABLE IF EXISTS schedule.dateframes;
		DROP TABLE IF EXISTS schedule.timeframes;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule schema tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
