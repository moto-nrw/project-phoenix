package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	formSchemasCoreRequirementsVersion     = "1.15.95"
	formSchemasCoreRequirementsDescription = "Add core_requirements JSONB to enrollment.form_schemas so each form template can mark supported built-in enrollment fields such as guardian phone as required."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemasCoreRequirementsVersion,
		Description: formSchemasCoreRequirementsDescription,
		DependsOn: []string{
			formSchemasAddNameVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.95: Adding core_requirements to enrollment.form_schemas...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
				ADD COLUMN IF NOT EXISTS core_requirements JSONB NOT NULL DEFAULT '{}'::jsonb;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding core_requirements column: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.95...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
				DROP COLUMN IF EXISTS core_requirements;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping core_requirements column: %w", err)
			}
			return nil
		},
	)
}
