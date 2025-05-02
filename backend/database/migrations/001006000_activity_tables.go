package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActivityTablesVersion     = "1.6.0"
	ActivityTablesDescription = "Activity group and time tables"
)

func init() {
	// Migration 6: Activity group and time tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return activityTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return activityTablesDown(ctx, db)
		},
	)
}

// activityTablesUp creates the activity group related tables
func activityTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating activity group tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the ag_categories table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ag_categories (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ag_categories table: %w", err)
	}

	// 2. Create the ags (activity group) table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ags (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			max_participant INTEGER NOT NULL,
			is_open_ags BOOLEAN NOT NULL DEFAULT false,
			supervisor_id BIGINT NOT NULL,
			ag_categories_id BIGINT NOT NULL,
			datespan_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_ag_supervisor FOREIGN KEY (supervisor_id) REFERENCES pedagogical_specialists(id) ON DELETE RESTRICT,
			CONSTRAINT fk_ag_categories FOREIGN KEY (ag_categories_id) REFERENCES ag_categories(id) ON DELETE RESTRICT,
			CONSTRAINT fk_ag_datespan FOREIGN KEY (datespan_id) REFERENCES timespans(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ags table: %w", err)
	}

	// 3. Create the ag_times table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ag_times (
			id BIGSERIAL PRIMARY KEY,
			weekday TEXT NOT NULL,
			timespans_id BIGINT NOT NULL,
			ag_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_ag_time_timespans FOREIGN KEY (timespans_id) REFERENCES timespans(id) ON DELETE CASCADE,
			CONSTRAINT fk_ag_time_ags FOREIGN KEY (ag_id) REFERENCES ags(id) ON DELETE CASCADE,
			CONSTRAINT check_weekday CHECK (weekday IN ('Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'))
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ag_times table: %w", err)
	}

	// Create indexes for ag
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ag_supervisor_id ON ags(supervisor_id);
		CREATE INDEX IF NOT EXISTS idx_ag_categories_id ON ags(ag_categories_id);
		CREATE INDEX IF NOT EXISTS idx_ag_datespan_id ON ags(datespan_id);
		CREATE INDEX IF NOT EXISTS idx_ag_name ON ags(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for ags table: %w", err)
	}

	// Create indexes for ag_times
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ag_time_ag_id ON ag_times(ag_id);
		CREATE INDEX IF NOT EXISTS idx_ag_time_timespans_id ON ag_times(timespans_id);
		CREATE INDEX IF NOT EXISTS idx_ag_time_weekday ON ag_times(weekday);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for ag_times table: %w", err)
	}

	// Create trigger for updated_at column in ags table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_ag_modified_at ON ags;
		CREATE TRIGGER update_ag_modified_at
		BEFORE UPDATE ON ags
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for ags table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// activityTablesDown removes the activity group related tables
func activityTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back activity group tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS ag_times;
		DROP TABLE IF EXISTS ags;
		DROP TABLE IF EXISTS ag_categories;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activity group tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
