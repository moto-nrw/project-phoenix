package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	hierarchicalSettingsVersion     = "1.8.2"
	hierarchicalSettingsDescription = "Create hierarchical settings tables with audit logging"
)

// HierarchicalSettingsDependsOn defines migration dependencies
var HierarchicalSettingsDependsOn = []string{
	ConfigSettingsVersion, // Depends on config schema (1.6.1)
	AuthAccountsVersion,   // Depends on auth.accounts (1.0.1)
	IoTDevicesVersion,     // Depends on iot.devices (1.3.9)
}

func init() {
	MigrationRegistry[hierarchicalSettingsVersion] = &Migration{
		Version:     hierarchicalSettingsVersion,
		Description: hierarchicalSettingsDescription,
		DependsOn:   HierarchicalSettingsDependsOn,
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return hierarchicalSettingsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return hierarchicalSettingsDown(ctx, db)
		},
	)
}

func hierarchicalSettingsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.2: Creating hierarchical settings tables...")

	// Create setting_tabs table for UI organization
	_, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS config.setting_tabs (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL,
			name TEXT NOT NULL,
			icon TEXT,
			display_order INT NOT NULL DEFAULT 0,
			required_permission TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);

		-- Partial unique index to allow re-creating deleted tabs with same key
		CREATE UNIQUE INDEX IF NOT EXISTS idx_setting_tabs_key_active
			ON config.setting_tabs(key) WHERE deleted_at IS NULL;

		-- Trigger for updated_at
		DROP TRIGGER IF EXISTS update_setting_tabs_updated_at ON config.setting_tabs;
		CREATE TRIGGER update_setting_tabs_updated_at
			BEFORE UPDATE ON config.setting_tabs
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating setting_tabs table: %w", err)
	}

	// Create setting_definitions table for metadata
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS config.setting_definitions (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL,
			value_type TEXT NOT NULL,
			default_value TEXT NOT NULL,
			category TEXT NOT NULL,
			tab TEXT NOT NULL DEFAULT 'general',
			display_order INT NOT NULL DEFAULT 0,
			label TEXT,
			description TEXT,
			allowed_scopes TEXT[] NOT NULL DEFAULT ARRAY['system'],
			view_permission TEXT,
			edit_permission TEXT,
			validation_schema JSONB,
			enum_values TEXT[],
			object_ref_type TEXT,
			object_ref_filter JSONB,
			requires_restart BOOLEAN NOT NULL DEFAULT FALSE,
			is_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);

		-- Partial unique index to allow re-creating deleted definitions with same key
		CREATE UNIQUE INDEX IF NOT EXISTS idx_setting_definitions_key_active
			ON config.setting_definitions(key) WHERE deleted_at IS NULL;

		-- Index for tab lookups
		CREATE INDEX IF NOT EXISTS idx_setting_definitions_tab
			ON config.setting_definitions(tab) WHERE deleted_at IS NULL;

		-- Index for category lookups
		CREATE INDEX IF NOT EXISTS idx_setting_definitions_category
			ON config.setting_definitions(category) WHERE deleted_at IS NULL;

		-- Trigger for updated_at
		DROP TRIGGER IF EXISTS update_setting_definitions_updated_at ON config.setting_definitions;
		CREATE TRIGGER update_setting_definitions_updated_at
			BEFORE UPDATE ON config.setting_definitions
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating setting_definitions table: %w", err)
	}

	// Create setting_values table for scoped overrides
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS config.setting_values (
			id BIGSERIAL PRIMARY KEY,
			definition_id BIGINT NOT NULL REFERENCES config.setting_definitions(id) ON DELETE CASCADE,
			scope_type TEXT NOT NULL,
			scope_id BIGINT,
			value TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,

			-- Constraint: scope_id must be NULL for system scope
			CONSTRAINT check_scope_id_for_system
				CHECK (scope_type != 'system' OR scope_id IS NULL)
		);

		-- Partial unique index for active values (allows re-creating after soft delete)
		CREATE UNIQUE INDEX IF NOT EXISTS idx_setting_values_scope_active
			ON config.setting_values(definition_id, scope_type, COALESCE(scope_id, -1))
			WHERE deleted_at IS NULL;

		-- Index for scope lookups
		CREATE INDEX IF NOT EXISTS idx_setting_values_scope
			ON config.setting_values(scope_type, scope_id) WHERE deleted_at IS NULL;

		-- Index for definition lookups
		CREATE INDEX IF NOT EXISTS idx_setting_values_definition
			ON config.setting_values(definition_id) WHERE deleted_at IS NULL;

		-- Trigger for updated_at
		DROP TRIGGER IF EXISTS update_setting_values_updated_at ON config.setting_values;
		CREATE TRIGGER update_setting_values_updated_at
			BEFORE UPDATE ON config.setting_values
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating setting_values table: %w", err)
	}

	// Create setting_audit_log table for GDPR-compliant change tracking
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS config.setting_audit_log (
			id BIGSERIAL PRIMARY KEY,
			definition_id BIGINT NOT NULL REFERENCES config.setting_definitions(id) ON DELETE CASCADE,
			setting_key TEXT NOT NULL,
			scope_type TEXT NOT NULL,
			scope_id BIGINT,
			old_value TEXT,
			new_value TEXT,
			action TEXT NOT NULL,
			changed_by_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			changed_by_name TEXT NOT NULL,
			changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ip_address TEXT,
			user_agent TEXT
		);

		-- Index for querying history by definition
		CREATE INDEX IF NOT EXISTS idx_setting_audit_log_definition
			ON config.setting_audit_log(definition_id);

		-- Index for querying history by time
		CREATE INDEX IF NOT EXISTS idx_setting_audit_log_changed_at
			ON config.setting_audit_log(changed_at);

		-- Index for querying history by user
		CREATE INDEX IF NOT EXISTS idx_setting_audit_log_changed_by
			ON config.setting_audit_log(changed_by_account_id);

		-- Index for querying by scope
		CREATE INDEX IF NOT EXISTS idx_setting_audit_log_scope
			ON config.setting_audit_log(scope_type, scope_id);

		-- Index for querying by setting key
		CREATE INDEX IF NOT EXISTS idx_setting_audit_log_key
			ON config.setting_audit_log(setting_key);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating setting_audit_log table: %w", err)
	}

	// Note: Tabs are now registered via code in settings/definitions/tabs.go
	// and synced to the database on server startup. No seed data here.

	fmt.Println("Migration 1.8.2: Hierarchical settings tables created successfully")
	return nil
}

func hierarchicalSettingsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.2: Dropping hierarchical settings tables...")

	// Drop audit log first (no dependencies except definition)
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS config.setting_audit_log CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping setting_audit_log table: %w", err)
	}

	// Drop values table
	_, err = db.NewRaw(`
		DROP TABLE IF EXISTS config.setting_values CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping setting_values table: %w", err)
	}

	// Drop definitions table
	_, err = db.NewRaw(`
		DROP TABLE IF EXISTS config.setting_definitions CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping setting_definitions table: %w", err)
	}

	// Drop tabs table
	_, err = db.NewRaw(`
		DROP TABLE IF EXISTS config.setting_tabs CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping setting_tabs table: %w", err)
	}

	fmt.Println("Migration 1.8.2: Hierarchical settings tables dropped successfully")
	return nil
}
