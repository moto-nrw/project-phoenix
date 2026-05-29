package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	formSchemasAddNameVersion     = "1.15.74"
	formSchemasAddNameDescription = "Add a 'name' column to enrollment.form_schemas so admins can maintain multiple named form variants per tenant (e.g. 'Schuljahr', 'Ferienbetreuung') instead of one global version chain. Drops the singleton 'one active per tenant' partial unique index — phases pin by id, so the active flag is no longer load-bearing."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemasAddNameVersion,
		Description: formSchemasAddNameDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.74: Adding name column to enrollment.form_schemas...")

			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
				ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding name column: %w", err)
			}

			// Backfill: existing rows came from the single-schema era. Tag
			// them with a stable, human-readable name so the new list UI
			// has something to display.
			if _, err := db.NewRaw(`
				UPDATE enrollment.form_schemas
				SET name = 'Standardformular'
				WHERE name = '';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed backfilling name: %w", err)
			}

			// The singleton constraint is gone. is_active stays as a
			// soft flag (we keep it for backward compat in the API),
			// but multiple schemas can be active simultaneously now.
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS enrollment.uq_form_schemas_one_active_per_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping one-active-per-tenant index: %w", err)
			}

			// Useful index: looking up schemas by (tenant_id, name) is
			// the new admin-list grouping. Partial on non-empty to skip
			// the brief migration-time empty state.
			if _, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_form_schemas_tenant_name
				ON enrollment.form_schemas (tenant_id, name)
				WHERE name <> '';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating tenant_name index: %w", err)
			}

			// Swap the old (tenant_id, version) uniqueness for a
			// per-name version chain — two different named schemas
			// can each start at version 1.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
				DROP CONSTRAINT IF EXISTS unique_form_schema_version;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping unique_form_schema_version: %w", err)
			}
			if _, err := db.NewRaw(`
				CREATE UNIQUE INDEX IF NOT EXISTS uq_form_schemas_tenant_name_version
				ON enrollment.form_schemas (tenant_id, name, version);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating tenant_name_version unique index: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.74...")

			if _, err := db.NewRaw(`DROP INDEX IF EXISTS enrollment.uq_form_schemas_tenant_name_version;`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping tenant_name_version unique index: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
				ADD CONSTRAINT unique_form_schema_version UNIQUE (tenant_id, version);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed restoring unique_form_schema_version: %w", err)
			}

			if _, err := db.NewRaw(`DROP INDEX IF EXISTS enrollment.idx_form_schemas_tenant_name;`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping tenant_name index: %w", err)
			}

			// Recreating the singleton index is only safe if at most one
			// row per tenant has is_active=true. If multiple coexist,
			// the rollback fails — dev-only path, deal with it manually.
			if _, err := db.NewRaw(`
				CREATE UNIQUE INDEX IF NOT EXISTS uq_form_schemas_one_active_per_tenant
				ON enrollment.form_schemas (tenant_id)
				WHERE is_active = TRUE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed restoring one-active-per-tenant index: %w", err)
			}

			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas DROP COLUMN IF EXISTS name;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping name column: %w", err)
			}

			return nil
		},
	)
}
