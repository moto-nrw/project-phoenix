package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingSelectionRulePerDayVersion     = "1.15.472"
	careOfferingSelectionRulePerDayDescription = "Allow selection_rule at_most_one_per_day on enrollment.care_offerings (#4514)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingSelectionRulePerDayVersion,
		Description: careOfferingSelectionRulePerDayDescription,
		DependsOn:   []string{careOfferingsSelectionRulesVersion},
	})
	Migrations.MustRegister(careOfferingSelectionRulePerDayUp, careOfferingSelectionRulePerDayDown)
}

// Existing rows keep their rule; the new value only widens the set, so no
// booking or offering changes.
func careOfferingSelectionRulePerDayUp(ctx context.Context, db *bun.DB) error {
	// 1.15.99 declared the CHECK inline; PostgreSQL named it
	// care_offerings_selection_rule_check.
	_, err := db.ExecContext(ctx, `
		ALTER TABLE enrollment.care_offerings
			DROP CONSTRAINT IF EXISTS care_offerings_selection_rule_check;
		ALTER TABLE enrollment.care_offerings
			ADD CONSTRAINT care_offerings_selection_rule_check
				CHECK (selection_rule IN ('optional', 'exactly_one', 'at_least_one', 'at_most_one', 'at_most_one_per_day'));
	`)
	if err != nil {
		return fmt.Errorf("allow per-day selection rule: %w", err)
	}
	return nil
}

// The rollback turns a per-day group into an optional one: the narrower
// constraint cannot hold the value, and optional is the only rule that does
// not newly reject bookings the per-day rule accepted.
func careOfferingSelectionRulePerDayDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		UPDATE enrollment.care_offerings
			SET selection_rule = 'optional'
			WHERE selection_rule = 'at_most_one_per_day';
		ALTER TABLE enrollment.care_offerings
			DROP CONSTRAINT IF EXISTS care_offerings_selection_rule_check;
		ALTER TABLE enrollment.care_offerings
			ADD CONSTRAINT care_offerings_selection_rule_check
				CHECK (selection_rule IN ('optional', 'exactly_one', 'at_least_one', 'at_most_one'));
	`)
	if err != nil {
		return fmt.Errorf("restore selection rule constraint: %w", err)
	}
	return nil
}
