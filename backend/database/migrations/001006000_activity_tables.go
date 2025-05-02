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

	// 2. Create the ag (activity group) table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ag (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			max_participant INTEGER NOT NULL,
			is_open_ag BOOLEAN NOT NULL DEFAULT false,
			supervisor_id BIGINT NOT NULL,
			ag_categories_id BIGINT NOT NULL,
			datespan_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_ag_supervisor FOREIGN KEY (supervisor_id) REFERENCES pedagogical_specialists(id) ON DELETE RESTRICT,
			CONSTRAINT fk_ag_categories FOREIGN KEY (ag_categories_id) REFERENCES ag_categories(id) ON DELETE RESTRICT,
			CONSTRAINT fk_ag_datespan FOREIGN KEY (datespan_id) REFERENCES timespan(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ag table: %w", err)
	}

	// 3. Create the ag_time table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ag_time (
			id BIGSERIAL PRIMARY KEY,
			weekday TEXT NOT NULL,
			timespan_id BIGINT NOT NULL,
			ag_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_ag_time_timespan FOREIGN KEY (timespan_id) REFERENCES timespan(id) ON DELETE CASCADE,
			CONSTRAINT fk_ag_time_ag FOREIGN KEY (ag_id) REFERENCES ag(id) ON DELETE CASCADE,
			CONSTRAINT check_weekday CHECK (weekday IN ('Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'))
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ag_time table: %w", err)
	}

	// Create indexes for ag
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ag_supervisor_id ON ag(supervisor_id);
		CREATE INDEX IF NOT EXISTS idx_ag_categories_id ON ag(ag_categories_id);
		CREATE INDEX IF NOT EXISTS idx_ag_datespan_id ON ag(datespan_id);
		CREATE INDEX IF NOT EXISTS idx_ag_name ON ag(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for ag table: %w", err)
	}

	// Create indexes for ag_time
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ag_time_ag_id ON ag_time(ag_id);
		CREATE INDEX IF NOT EXISTS idx_ag_time_timespan_id ON ag_time(timespan_id);
		CREATE INDEX IF NOT EXISTS idx_ag_time_weekday ON ag_time(weekday);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for ag_time table: %w", err)
	}

	// Create trigger for updated_at column in ag table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_ag_modified_at ON ag;
		CREATE TRIGGER update_ag_modified_at
		BEFORE UPDATE ON ag
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for ag table: %w", err)
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
		DROP TABLE IF EXISTS ag_time;
		DROP TABLE IF EXISTS ag;
		DROP TABLE IF EXISTS ag_categories;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activity group tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
