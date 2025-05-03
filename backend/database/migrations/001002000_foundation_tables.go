package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FoundationTablesVersion     = "1.2.0"
	FoundationTablesDescription = "Foundation tables with no or minimal dependencies"
)

func init() {
	// Migration 1.2.0: Foundation tables with no or minimal dependencies
	Migrations.MustRegister(
		// Up function
		func(ctx context.Context, db *bun.DB) error {
			return foundationTablesUp(ctx, db)
		},
		// Down function
		func(ctx context.Context, db *bun.DB) error {
			return foundationTablesDown(ctx, db)
		},
	)
}

// foundationTablesUp creates foundation tables with minimal dependencies
func foundationTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the timespans table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS timespans (
			id BIGSERIAL PRIMARY KEY,
			start_time TIMESTAMPTZ NOT NULL,
			end_time TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating timespans table: %w", err)
	}

	// Create indexes for timespans
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_timespans_start_time ON timespans(start_time);
		CREATE INDEX IF NOT EXISTS idx_timespans_end_time ON timespans(end_time);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for timespans table: %w", err)
	}

	// 2. Create the datespan table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS datespan (
			id BIGSERIAL PRIMARY KEY,
			start_date TIMESTAMPTZ NOT NULL,
			end_date TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating datespan table: %w", err)
	}

	// Create indexes for datespan
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_datespan_start_date ON datespan(start_date);
		CREATE INDEX IF NOT EXISTS idx_datespan_end_date ON datespan(end_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for datespan table: %w", err)
	}

	// 3. Create the ag_categories table (Activity Group categories)
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

	// Create indexes for ag_categories
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ag_categories_name ON ag_categories(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for ag_categories table: %w", err)
	}

	// 4. Create the settings table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS settings (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT,
			requires_restart BOOLEAN NOT NULL DEFAULT FALSE,
			requires_db_reset BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating settings table: %w", err)
	}

	// Create indexes for settings
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_settings_key ON settings(key);
		CREATE INDEX IF NOT EXISTS idx_settings_category ON settings(category);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for settings table: %w", err)
	}

	// 5. Create the rooms table (basic structure only)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS rooms (
			id BIGSERIAL PRIMARY KEY,
			room_name TEXT NOT NULL UNIQUE,
			building TEXT,
			floor INTEGER NOT NULL DEFAULT 0,
			capacity INTEGER NOT NULL DEFAULT 0,
			category TEXT NOT NULL DEFAULT 'Other',
			color TEXT NOT NULL DEFAULT '#FFFFFF',
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating rooms table: %w", err)
	}

	// Create indexes for rooms
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_rooms_room_name ON rooms(room_name);
		CREATE INDEX IF NOT EXISTS idx_rooms_building ON rooms(building);
		CREATE INDEX IF NOT EXISTS idx_rooms_category ON rooms(category);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for rooms table: %w", err)
	}

	// Create triggers for updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Function to update updated_at column already created in previous migration
		
		-- Trigger for settings
		DROP TRIGGER IF EXISTS update_settings_modified_at ON settings;
		CREATE TRIGGER update_settings_modified_at
		BEFORE UPDATE ON settings
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Trigger for rooms
		DROP TRIGGER IF EXISTS update_rooms_modified_at ON rooms;
		CREATE TRIGGER update_rooms_modified_at
		BEFORE UPDATE ON rooms
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// foundationTablesDown removes the foundation tables
func foundationTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS rooms;
		DROP TABLE IF EXISTS settings;
		DROP TABLE IF EXISTS ag_categories;
		DROP TABLE IF EXISTS datespan;
		DROP TABLE IF EXISTS timespans;
	`)
	if err != nil {
		return fmt.Errorf("error dropping foundation tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
