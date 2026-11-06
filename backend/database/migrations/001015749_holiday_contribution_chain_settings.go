package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayContributionChainSettingsVersion = "1.15.749"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayContributionChainSettingsVersion,
		Description: "Elternabrechnung: festgehaltener Beitrag hält Rang der Geschwister und Reihenfolge der Kostenübernahme fest (#5664)",
		DependsOn: []string{
			parentBillingCarriedCorrectionsVersion,   // the last migration before this one
			parentBillingHolidayContributionsVersion, // the frozen contributions this extends
		},
	})
	Migrations.MustRegister(holidayContributionChainSettingsUp, holidayContributionChainSettingsDown)
}

// holidayContributionChainSettingsUpSQL records with a frozen contribution
// how it was computed (#5664, ADR 0115 addendum). A group computes again with
// what its frozen contributions hold, so a later change of the settings of
// the Träger changes none of them. Every existing contribution was computed
// with the fixed chain, which the defaults name.
const holidayContributionChainSettingsUpSQL = `
ALTER TABLE parent_billing.holiday_contributions
	ADD COLUMN sibling_rank VARCHAR(32) NOT NULL DEFAULT 'highest_contribution',
	ADD COLUMN cost_takeover_order VARCHAR(32) NOT NULL DEFAULT 'after_discount',
	ADD CONSTRAINT chk_holiday_contributions_sibling_rank
		CHECK (sibling_rank IN ('highest_contribution', 'oldest_child')),
	ADD CONSTRAINT chk_holiday_contributions_cost_takeover_order
		CHECK (cost_takeover_order IN ('after_discount', 'before_discount'));
COMMENT ON COLUMN parent_billing.holiday_contributions.sibling_rank IS
	'Which child counted as the first child when this contribution was computed (#5664).';
COMMENT ON COLUMN parent_billing.holiday_contributions.cost_takeover_order IS
	'Whether a cost bearer paid after the sibling discount or the full price when this contribution was computed (#5664).';
`

const holidayContributionChainSettingsDownSQL = `
ALTER TABLE parent_billing.holiday_contributions
	DROP COLUMN sibling_rank,
	DROP COLUMN cost_takeover_order;
`

func holidayContributionChainSettingsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayContributionChainSettingsUpSQL); err != nil {
		return fmt.Errorf("holiday contribution chain settings up: %w", err)
	}
	return nil
}

func holidayContributionChainSettingsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayContributionChainSettingsDownSQL); err != nil {
		return fmt.Errorf("holiday contribution chain settings down: %w", err)
	}
	return nil
}
