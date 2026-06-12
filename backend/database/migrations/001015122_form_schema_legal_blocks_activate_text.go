package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	formSchemaLegalBlocksActivateTextVersion     = "1.15.122"
	formSchemaLegalBlocksActivateTextDescription = "Activate contentful enrollment form legal blocks that were saved disabled"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemaLegalBlocksActivateTextVersion,
		Description: formSchemaLegalBlocksActivateTextDescription,
		DependsOn:   []string{formSchemaLegalBlocksVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return formSchemaLegalBlocksActivateTextUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			// Data-only repair. Rolling this back would require knowing
			// whether each block was intentionally disabled or accidentally
			// saved inactive after text entry, so the down migration is a no-op.
			return nil
		},
	)
}

func formSchemaLegalBlocksActivateTextUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.122: Activating contentful enrollment form legal blocks...")

	if _, err := db.NewRaw(`
		WITH rewritten AS (
			SELECT
				fs.id,
				jsonb_agg(
					CASE
						WHEN COALESCE(block.elem->>'enabled', 'false') <> 'true'
							AND btrim(COALESCE(block.elem->>'text', '')) <> ''
							AND btrim(COALESCE(block.elem->>'title', '')) <> ''
							AND btrim(COALESCE(block.elem->>'label', '')) <> ''
						THEN jsonb_set(block.elem, '{enabled}', 'true'::jsonb, true)
						ELSE block.elem
					END
					ORDER BY block.ord
				) AS legal_blocks
			FROM enrollment.form_schemas AS fs
			CROSS JOIN LATERAL jsonb_array_elements(fs.legal_blocks) WITH ORDINALITY AS block(elem, ord)
			WHERE jsonb_typeof(fs.legal_blocks) = 'array'
			GROUP BY fs.id
			HAVING NOT bool_or(COALESCE(block.elem->>'enabled', 'false') = 'true')
				AND bool_or(btrim(COALESCE(block.elem->>'text', '')) <> '')
		)
		UPDATE enrollment.form_schemas AS fs
		SET legal_blocks = rewritten.legal_blocks
		FROM rewritten
		WHERE fs.id = rewritten.id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed activating contentful legal blocks: %w", err)
	}

	return nil
}
