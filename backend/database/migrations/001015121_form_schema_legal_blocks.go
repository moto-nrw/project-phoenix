package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	formSchemaLegalBlocksVersion     = "1.15.121"
	formSchemaLegalBlocksDescription = "Add template-owned legal consent blocks to enrollment form schemas"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemaLegalBlocksVersion,
		Description: formSchemaLegalBlocksDescription,
		DependsOn: []string{
			"1.15.59", // enrollment.form_schemas
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.121: Adding legal blocks to enrollment.form_schemas...")
			_, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
					ADD COLUMN IF NOT EXISTS legal_blocks JSONB NOT NULL DEFAULT '[]'::jsonb;
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed adding form_schemas.legal_blocks: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.121: Dropping form_schemas.legal_blocks...")
			_, err := db.NewRaw(`
				ALTER TABLE enrollment.form_schemas
					DROP COLUMN IF EXISTS legal_blocks;
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed dropping form_schemas.legal_blocks: %w", err)
			}
			return nil
		},
	)
}
