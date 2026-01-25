package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ScopedSettingsVersion     = "1.8.1"
	ScopedSettingsDescription = "Create scoped settings infrastructure"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ScopedSettingsVersion] = &Migration{
		Version:     ScopedSettingsVersion,
		Description: ScopedSettingsDescription,
		DependsOn:   []string{"1.6.1"}, // Depends on config.settings
	}

	// Migration 1.8.1: Create scoped settings tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createScopedSettingsTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropScopedSettingsTables(ctx, db)
		},
	)
}

func createScopedSettingsTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.1: Creating scoped settings infrastructure...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create setting_definitions table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.setting_definitions (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			default_value JSONB NOT NULL,
			category TEXT NOT NULL,
			description TEXT,
			validation JSONB,
			allowed_scopes TEXT[] NOT NULL,
			scope_permissions JSONB NOT NULL,
			depends_on JSONB,
			group_name TEXT,
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		COMMENT ON TABLE config.setting_definitions IS 'Registry of available settings with metadata, types, and validation rules';
		COMMENT ON COLUMN config.setting_definitions.key IS 'Unique setting key, e.g., session.timeout_minutes';
		COMMENT ON COLUMN config.setting_definitions.type IS 'Data type: bool, int, float, string, enum, time, json';
		COMMENT ON COLUMN config.setting_definitions.default_value IS 'Default value as JSONB, e.g., {"value": 30}';
		COMMENT ON COLUMN config.setting_definitions.validation IS 'Validation rules: {min, max, options, pattern}';
		COMMENT ON COLUMN config.setting_definitions.allowed_scopes IS 'Scopes that can override: system, school, og, user, device';
		COMMENT ON COLUMN config.setting_definitions.scope_permissions IS 'Permission required per scope: {"system": "config:manage", "user": "self"}';
		COMMENT ON COLUMN config.setting_definitions.depends_on IS 'Dependency: {key, condition, value}';
		COMMENT ON COLUMN config.setting_definitions.group_name IS 'UI grouping for related settings';
	`)
	if err != nil {
		return fmt.Errorf("error creating setting_definitions table: %w", err)
	}

	// Create setting_values table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.setting_values (
			id BIGSERIAL PRIMARY KEY,
			definition_id BIGINT NOT NULL REFERENCES config.setting_definitions(id) ON DELETE CASCADE,
			scope_type TEXT NOT NULL,
			scope_id BIGINT,
			value JSONB NOT NULL,
			set_by BIGINT REFERENCES users.persons(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(definition_id, scope_type, scope_id)
		);

		COMMENT ON TABLE config.setting_values IS 'Scoped setting values with inheritance';
		COMMENT ON COLUMN config.setting_values.scope_type IS 'Scope type: system, school, og, user, device';
		COMMENT ON COLUMN config.setting_values.scope_id IS 'Entity ID for scope (NULL for system scope)';
		COMMENT ON COLUMN config.setting_values.value IS 'Setting value as JSONB';
		COMMENT ON COLUMN config.setting_values.set_by IS 'Person who set this value';
	`)
	if err != nil {
		return fmt.Errorf("error creating setting_values table: %w", err)
	}

	// Create audit.setting_changes table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.setting_changes (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			setting_key TEXT NOT NULL,
			scope_type TEXT NOT NULL,
			scope_id BIGINT,
			change_type TEXT NOT NULL,
			old_value JSONB,
			new_value JSONB,
			ip_address INET,
			user_agent TEXT,
			reason TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		COMMENT ON TABLE audit.setting_changes IS 'Audit log for setting modifications';
		COMMENT ON COLUMN audit.setting_changes.change_type IS 'Type of change: create, update, delete, reset';
	`)
	if err != nil {
		return fmt.Errorf("error creating setting_changes table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		-- setting_definitions indexes
		CREATE INDEX IF NOT EXISTS idx_setting_definitions_key ON config.setting_definitions(key);
		CREATE INDEX IF NOT EXISTS idx_setting_definitions_category ON config.setting_definitions(category);
		CREATE INDEX IF NOT EXISTS idx_setting_definitions_group ON config.setting_definitions(group_name);

		-- setting_values indexes
		CREATE INDEX IF NOT EXISTS idx_setting_values_definition ON config.setting_values(definition_id);
		CREATE INDEX IF NOT EXISTS idx_setting_values_scope ON config.setting_values(scope_type, scope_id);
		CREATE INDEX IF NOT EXISTS idx_setting_values_scope_type ON config.setting_values(scope_type);

		-- setting_changes indexes (audit)
		CREATE INDEX IF NOT EXISTS idx_setting_changes_account ON audit.setting_changes(account_id);
		CREATE INDEX IF NOT EXISTS idx_setting_changes_key ON audit.setting_changes(setting_key);
		CREATE INDEX IF NOT EXISTS idx_setting_changes_scope ON audit.setting_changes(scope_type, scope_id);
		CREATE INDEX IF NOT EXISTS idx_setting_changes_created ON audit.setting_changes(created_at);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes: %w", err)
	}

	// Create triggers for updated_at
	_, err = tx.ExecContext(ctx, `
		-- Trigger for setting_definitions
		DROP TRIGGER IF EXISTS update_setting_definitions_updated_at ON config.setting_definitions;
		CREATE TRIGGER update_setting_definitions_updated_at
		BEFORE UPDATE ON config.setting_definitions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		-- Trigger for setting_values
		DROP TRIGGER IF EXISTS update_setting_values_updated_at ON config.setting_values;
		CREATE TRIGGER update_setting_values_updated_at
		BEFORE UPDATE ON config.setting_values
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating triggers: %w", err)
	}

	return tx.Commit()
}

func dropScopedSettingsTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.1: Removing scoped settings infrastructure...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop triggers
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_setting_definitions_updated_at ON config.setting_definitions;
		DROP TRIGGER IF EXISTS update_setting_values_updated_at ON config.setting_values;
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers: %w", err)
	}

	// Drop tables in reverse order (respecting foreign keys)
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS audit.setting_changes CASCADE;
		DROP TABLE IF EXISTS config.setting_values CASCADE;
		DROP TABLE IF EXISTS config.setting_definitions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping tables: %w", err)
	}

	return tx.Commit()
}
