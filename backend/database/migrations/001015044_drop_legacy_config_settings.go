package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropLegacyConfigSettingsVersion     = "1.15.44"
	dropLegacyConfigSettingsDescription = "Drop legacy config.settings table (replaced by registry-driven config.setting_values)"
)

func init() {
	MigrationRegistry[dropLegacyConfigSettingsVersion] = &Migration{
		Version:     dropLegacyConfigSettingsVersion,
		Description: dropLegacyConfigSettingsDescription,
		DependsOn:   []string{"1.15.25"}, // depends on new tables existing
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error { return dropLegacyConfigSettings(ctx, db) },
		func(ctx context.Context, db *bun.DB) error { return recreateLegacyConfigSettings(ctx, db) },
	)
}

func dropLegacyConfigSettings(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.44: Dropping legacy config.settings table...")
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS config.settings CASCADE;`); err != nil {
		return fmt.Errorf("drop config.settings: %w", err)
	}
	// Defensive: config.retention_policies has no creation migration in this repo,
	// but may exist in older prod databases from the pre-multi-tenant era.
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS config.retention_policies CASCADE;`); err != nil {
		return fmt.Errorf("drop config.retention_policies: %w", err)
	}
	return tx.Commit()
}

func recreateLegacyConfigSettings(ctx context.Context, db *bun.DB) error {
	// Minimal recreate so the migration is reversible. Rollback intentionally does
	// NOT restore data — it just lets the migration runner walk backwards.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.settings (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT,
			requires_restart BOOLEAN NOT NULL DEFAULT FALSE,
			requires_db_reset BOOLEAN NOT NULL DEFAULT FALSE,
			tenant_id BIGINT REFERENCES platform.schools(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT settings_tenant_key_unique UNIQUE (tenant_id, key)
		);
	`)
	if err != nil {
		return fmt.Errorf("recreate config.settings: %w", err)
	}
	return tx.Commit()
}
