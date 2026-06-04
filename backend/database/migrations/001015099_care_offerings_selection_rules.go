package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingsSelectionRulesVersion     = "1.15.99"
	careOfferingsSelectionRulesDescription = "Add selection_group + selection_rule to enrollment.care_offerings so admins can group offerings and require a choice (exactly_one / at_least_one / at_most_one)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingsSelectionRulesVersion,
		Description: careOfferingsSelectionRulesDescription,
		DependsOn:   []string{createCareOfferingsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.99: Adding selection_group + selection_rule to enrollment.care_offerings...")
			// selection_group is a free-text group name (NULL = ungrouped).
			// selection_rule constrains how many offerings in the group a
			// parent must pick. Existing rows default to 'optional' (today's
			// behavior). IF NOT EXISTS keeps the migration idempotent.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offerings
					ADD COLUMN IF NOT EXISTS selection_group text,
					ADD COLUMN IF NOT EXISTS selection_rule text NOT NULL DEFAULT 'optional'
						CHECK (selection_rule IN ('optional', 'exactly_one', 'at_least_one', 'at_most_one'));
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding selection columns to enrollment.care_offerings: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.99: dropping selection columns on enrollment.care_offerings...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offerings
					DROP COLUMN IF EXISTS selection_rule,
					DROP COLUMN IF EXISTS selection_group;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping selection columns: %w", err)
			}
			return nil
		},
	)
}
