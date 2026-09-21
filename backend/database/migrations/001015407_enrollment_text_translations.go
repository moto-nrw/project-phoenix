package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.407",
		Description: "Store school-written translations of phase and care offering texts (#3377)",
		DependsOn:   []string{"1.15.405"},
	})
	Migrations.MustRegister(enrollmentTextTranslationsUp, enrollmentTextTranslationsDown)
}

// Form schema texts translate inside the schema's JSONB columns; phases and
// care offerings are plain rows and need a column of their own. The shape is
// locale → attribute → {text, source}.
func enrollmentTextTranslationsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
			ADD COLUMN IF NOT EXISTS translations JSONB NOT NULL DEFAULT '{}'::jsonb;
		ALTER TABLE enrollment.care_offerings
			ADD COLUMN IF NOT EXISTS translations JSONB NOT NULL DEFAULT '{}'::jsonb;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add enrollment text translations: %w", err)
	}
	return nil
}

func enrollmentTextTranslationsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.care_offerings DROP COLUMN IF EXISTS translations;
		ALTER TABLE enrollment.phases DROP COLUMN IF EXISTS translations;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop enrollment text translations: %w", err)
	}
	return nil
}
