package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	configSettingValuesVersion     = "1.15.24"
	configSettingValuesDescription = "Create config.setting_values and config.setting_audit tables for the settings system"
)

func init() {
	MigrationRegistry[configSettingValuesVersion] = &Migration{
		Version:     configSettingValuesVersion,
		Description: configSettingValuesDescription,
		DependsOn:   []string{"1.15.23", "1.6.1"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createSettingValuesTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropSettingValuesTables(ctx, db)
		},
	)
}

func createSettingValuesTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.19: Creating config.setting_values and config.setting_audit tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// ---------- config.setting_values ----------
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE config.setting_values (
			id          BIGSERIAL    PRIMARY KEY,
			tenant_id   BIGINT       NOT NULL REFERENCES platform.schools(id),
			setting_key VARCHAR(255) NOT NULL,
			value       JSONB        NOT NULL,
			updated_by  BIGINT       REFERENCES auth.accounts(id),
			created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

			CONSTRAINT uq_tenant_setting UNIQUE (tenant_id, setting_key)
		);

		CREATE INDEX idx_setting_values_tenant ON config.setting_values (tenant_id);

		CREATE TRIGGER update_setting_values_updated_at
			BEFORE UPDATE ON config.setting_values
			FOR EACH ROW
			EXECUTE FUNCTION update_modified_column();

		ALTER TABLE config.setting_values ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.setting_values FORCE ROW LEVEL SECURITY;

		CREATE POLICY tenant_isolation_config_setting_values ON config.setting_values
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`)
	if err != nil {
		return fmt.Errorf("error creating config.setting_values: %w", err)
	}
	fmt.Println("  ✓ config.setting_values — table, index, trigger, RLS created")

	// ---------- config.setting_audit ----------
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE config.setting_audit (
			id          BIGSERIAL    PRIMARY KEY,
			tenant_id   BIGINT       NOT NULL REFERENCES platform.schools(id),
			setting_key VARCHAR(255) NOT NULL,
			old_value   JSONB,
			new_value   JSONB,
			action      VARCHAR(20)  NOT NULL CHECK (action IN ('set', 'reset', 'delete')),
			changed_by  BIGINT       REFERENCES auth.accounts(id),
			changed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_setting_audit_key ON config.setting_audit (setting_key);
		CREATE INDEX idx_setting_audit_tenant ON config.setting_audit (tenant_id);

		ALTER TABLE config.setting_audit ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.setting_audit FORCE ROW LEVEL SECURITY;

		CREATE POLICY tenant_isolation_config_setting_audit ON config.setting_audit
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`)
	if err != nil {
		return fmt.Errorf("error creating config.setting_audit: %w", err)
	}
	fmt.Println("  ✓ config.setting_audit — table, indexes, RLS created")

	return tx.Commit()
}

func dropSettingValuesTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.19: Dropping config.setting_values and config.setting_audit...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS config.setting_audit CASCADE;
		DROP TABLE IF EXISTS config.setting_values CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping settings tables: %w", err)
	}

	return tx.Commit()
}
