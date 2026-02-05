package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActionSupportVersion     = "1.10.1"
	ActionSupportDescription = "Add action support to settings system with audit logging"
)

// ActionSupportDependsOn defines migration dependencies
var ActionSupportDependsOn = []string{
	enumOptionsVersion, // Depends on enum_options (1.8.3)
}

func init() {
	MigrationRegistry[ActionSupportVersion] = &Migration{
		Version:     ActionSupportVersion,
		Description: ActionSupportDescription,
		DependsOn:   ActionSupportDependsOn,
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return actionSupportUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return actionSupportDown(ctx, db)
		},
	)
}

func actionSupportUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.1: Adding action support to settings system...")

	// Extend setting_definitions with action-specific columns
	_, err := db.NewRaw(`
		-- Add action-specific columns to setting_definitions
		ALTER TABLE config.setting_definitions
			ADD COLUMN IF NOT EXISTS action_endpoint TEXT,
			ADD COLUMN IF NOT EXISTS action_method TEXT DEFAULT 'POST',
			ADD COLUMN IF NOT EXISTS action_requires_confirmation BOOLEAN DEFAULT true,
			ADD COLUMN IF NOT EXISTS action_confirmation_title TEXT,
			ADD COLUMN IF NOT EXISTS action_confirmation_message TEXT,
			ADD COLUMN IF NOT EXISTS action_confirmation_button TEXT,
			ADD COLUMN IF NOT EXISTS action_success_message TEXT,
			ADD COLUMN IF NOT EXISTS action_error_message TEXT,
			ADD COLUMN IF NOT EXISTS action_is_dangerous BOOLEAN DEFAULT false,
			ADD COLUMN IF NOT EXISTS icon TEXT;

		-- Add comments for documentation
		COMMENT ON COLUMN config.setting_definitions.action_endpoint IS
			'API endpoint for action execution (only for value_type=action)';
		COMMENT ON COLUMN config.setting_definitions.action_method IS
			'HTTP method for action (POST, DELETE, etc.)';
		COMMENT ON COLUMN config.setting_definitions.action_requires_confirmation IS
			'Whether to show confirmation dialog before execution';
		COMMENT ON COLUMN config.setting_definitions.action_confirmation_title IS
			'Title for confirmation dialog';
		COMMENT ON COLUMN config.setting_definitions.action_confirmation_message IS
			'Message body for confirmation dialog';
		COMMENT ON COLUMN config.setting_definitions.action_confirmation_button IS
			'Label for confirm button';
		COMMENT ON COLUMN config.setting_definitions.action_success_message IS
			'Message shown on successful execution';
		COMMENT ON COLUMN config.setting_definitions.action_error_message IS
			'Message shown on failed execution';
		COMMENT ON COLUMN config.setting_definitions.action_is_dangerous IS
			'Whether this action is destructive (affects button styling)';
		COMMENT ON COLUMN config.setting_definitions.icon IS
			'Icon name for display';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding action columns to setting_definitions: %w", err)
	}

	// Create action execution audit log table
	_, err = db.NewRaw(`
		-- Action execution audit (append-only for GDPR compliance)
		CREATE TABLE IF NOT EXISTS config.action_audit_log (
			id BIGSERIAL PRIMARY KEY,
			action_key TEXT NOT NULL,
			executed_by_account_id BIGINT REFERENCES auth.accounts(id),
			executed_by_name TEXT NOT NULL,
			executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			duration_ms BIGINT,
			success BOOLEAN NOT NULL,
			error_message TEXT,
			result_summary TEXT,
			ip_address TEXT,
			user_agent TEXT
		);

		-- Index for querying by action key
		CREATE INDEX IF NOT EXISTS idx_action_audit_log_key
			ON config.action_audit_log(action_key);

		-- Index for querying by time
		CREATE INDEX IF NOT EXISTS idx_action_audit_log_executed_at
			ON config.action_audit_log(executed_at);

		-- Index for querying by user
		CREATE INDEX IF NOT EXISTS idx_action_audit_log_executed_by
			ON config.action_audit_log(executed_by_account_id);

		-- Add comment
		COMMENT ON TABLE config.action_audit_log IS
			'Audit log for action executions (append-only for compliance)';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating action_audit_log table: %w", err)
	}

	// Add system tab if not exists (for maintenance actions)
	_, err = db.NewRaw(`
		INSERT INTO config.setting_tabs (key, name, icon, display_order, required_permission)
		VALUES ('system', 'System', 'server', 100, 'config:manage')
		ON CONFLICT DO NOTHING;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed inserting system tab: %w", err)
	}

	fmt.Println("Migration 1.10.1: Action support added successfully")
	return nil
}

func actionSupportDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.1: Removing action support...")

	// Drop action audit log table
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS config.action_audit_log CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping action_audit_log table: %w", err)
	}

	// Remove action columns from setting_definitions
	_, err = db.NewRaw(`
		ALTER TABLE config.setting_definitions
			DROP COLUMN IF EXISTS action_endpoint,
			DROP COLUMN IF EXISTS action_method,
			DROP COLUMN IF EXISTS action_requires_confirmation,
			DROP COLUMN IF EXISTS action_confirmation_title,
			DROP COLUMN IF EXISTS action_confirmation_message,
			DROP COLUMN IF EXISTS action_confirmation_button,
			DROP COLUMN IF EXISTS action_success_message,
			DROP COLUMN IF EXISTS action_error_message,
			DROP COLUMN IF EXISTS action_is_dangerous,
			DROP COLUMN IF EXISTS icon;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing action columns from setting_definitions: %w", err)
	}

	// Remove system tab (only if we added it)
	_, err = db.NewRaw(`
		DELETE FROM config.setting_tabs WHERE key = 'system';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing system tab: %w", err)
	}

	fmt.Println("Migration 1.10.1: Action support removed successfully")
	return nil
}
