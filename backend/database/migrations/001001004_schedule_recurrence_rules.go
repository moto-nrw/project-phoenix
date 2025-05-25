package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ScheduleRecurrenceRulesVersion     = "1.1.4"
	ScheduleRecurrenceRulesDescription = "Create schedule.recurrence_rules table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ScheduleRecurrenceRulesVersion] = &Migration{
		Version:     ScheduleRecurrenceRulesVersion,
		Description: ScheduleRecurrenceRulesDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth tables
	}

	// Migration 1.1.4: Create schedule.recurrence_rules table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createScheduleRecurrenceRulesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropScheduleRecurrenceRulesTable(ctx, db)
		},
	)
}

// createScheduleRecurrenceRulesTable creates the schedule.recurrence_rules table
func createScheduleRecurrenceRulesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.4: Creating schedule.recurrence_rules table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the recurrence_rules table - for recurring events
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

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for recurrence_rules
		DROP TRIGGER IF EXISTS update_recurrence_rules_updated_at ON schedule.recurrence_rules;
		CREATE TRIGGER update_recurrence_rules_updated_at
		BEFORE UPDATE ON schedule.recurrence_rules
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropScheduleRecurrenceRulesTable drops the schedule.recurrence_rules table
func dropScheduleRecurrenceRulesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.4: Removing schedule.recurrence_rules table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_recurrence_rules_updated_at ON schedule.recurrence_rules;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for recurrence_rules table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.recurrence_rules;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.recurrence_rules table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
