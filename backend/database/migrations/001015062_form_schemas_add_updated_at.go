package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	formSchemasAddUpdatedAtVersion     = "1.15.62"
	formSchemasAddUpdatedAtDescription = "Add updated_at column to enrollment.form_schemas to satisfy base.Model embedding (parent-enrollment PR 5)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemasAddUpdatedAtVersion,
		Description: formSchemasAddUpdatedAtDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
				ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed adding updated_at to form_schemas: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewRaw(`ALTER TABLE enrollment.form_schemas DROP COLUMN IF EXISTS updated_at;`).Exec(ctx)
			return err
		},
	)
}
