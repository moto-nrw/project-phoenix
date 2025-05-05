package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ConfigTablesVersion     = "1.9.0"
	ConfigTablesDescription = "System configuration tables"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ConfigTablesVersion] = &Migration{
		Version:     ConfigTablesVersion,
		Description: ConfigTablesDescription,
		DependsOn:   []string{"1.8.0"}, // Depends on feedback tables
	}

	// Migration 1.9.0: Config schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return configTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return configTablesDown(ctx, db)
		},
	)
}

// configTablesUp creates the config schema tables
func configTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.9.0: Creating config schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create config schema
	_, err = tx.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS config;
	`)
	if err != nil {
		return fmt.Errorf("error creating config schema: %w", err)
	}

	// Create config settings table
	_, err = tx.ExecContext(ctx, `
		-- System configuration tables
		CREATE TABLE IF NOT EXISTS config.settings (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT,
			requires_restart BOOLEAN NOT NULL DEFAULT FALSE,
			requires_db_reset BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating config settings table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_config_settings_key ON config.settings(key);
		CREATE INDEX IF NOT EXISTS idx_config_settings_category ON config.settings(category);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for config settings table: %w", err)
	}

	// Insert default settings
	_, err = tx.ExecContext(ctx, `
		-- Insert some default settings
		INSERT INTO config.settings (key, value, category, description, requires_restart, requires_db_reset)
		VALUES 
			('system.name', 'Project Phoenix', 'system', 'Name of the system', FALSE, FALSE),
			('system.version', '1.0.0', 'system', 'Current system version', FALSE, FALSE),
			('email.from', 'noreply@example.com', 'email', 'Default sender email address', TRUE, FALSE),
			('email.enabled', 'true', 'email', 'Whether email sending is enabled', TRUE, FALSE),
			('session.timeout', '30', 'security', 'Session timeout in minutes', TRUE, FALSE)
		ON CONFLICT (key) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error inserting default config settings: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for config settings
		DROP TRIGGER IF EXISTS update_config_settings_updated_at ON config.settings;
		CREATE TRIGGER update_config_settings_updated_at
		BEFORE UPDATE ON config.settings
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for config settings: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// configTablesDown removes the config schema tables
func configTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.9.0: Removing config schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS config.settings CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping config settings table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
