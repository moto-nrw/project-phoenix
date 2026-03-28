package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addSuggestionsHiddenVersion     = "1.15.18"
	addSuggestionsHiddenDescription = "Add is_hidden column to suggestions.posts for operator moderation"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addSuggestionsHiddenVersion,
		Description: addSuggestionsHiddenDescription,
		DependsOn:   []string{"1.15.17"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE suggestions.posts
				ADD COLUMN is_hidden BOOLEAN NOT NULL DEFAULT FALSE
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE suggestions.posts
				DROP COLUMN IF EXISTS is_hidden
			`)
			return err
		},
	)
}
