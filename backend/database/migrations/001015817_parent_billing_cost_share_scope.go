package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCostShareScopeVersion = "1.15.817"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCostShareScopeVersion,
		Description: "Elternabrechnung: Kostenträger-Anteile wirken auf Module, Mensa oder beides (#4686)",
		DependsOn: []string{
			parentBillingCareCostSharesVersion, // the shares that get a scope
			mealBillingMonthsVersion,           // the Mensa positions a share can cover
		},
	})
	Migrations.MustRegister(parentBillingCostShareScopeUp, parentBillingCostShareScopeDown)
}

// parentBillingCostShareScopeUpSQL says what a cost bearer share covers
// (#4686, ADR 0168 point 12): the modules of the care, the Mensa or both.
// Every share so far covered the care and keeps that scope. An issued
// invoice copies the scope with its share; the column default leaves the
// rows of 1.15.816 as they were billed, without an update the immutability
// trigger would refuse.
const parentBillingCostShareScopeUpSQL = `
ALTER TABLE parent_billing.care_cost_shares
	ADD COLUMN IF NOT EXISTS scope VARCHAR(8) NOT NULL DEFAULT 'care'
	CONSTRAINT chk_care_cost_shares_scope CHECK (scope IN ('care', 'meal', 'both'));
ALTER TABLE parent_billing.care_invoice_cost_shares
	ADD COLUMN IF NOT EXISTS scope VARCHAR(8) NOT NULL DEFAULT 'care'
	CONSTRAINT chk_care_invoice_cost_shares_scope CHECK (scope IN ('care', 'meal', 'both'));
`

const parentBillingCostShareScopeDownSQL = `
ALTER TABLE parent_billing.care_invoice_cost_shares DROP COLUMN IF EXISTS scope;
ALTER TABLE parent_billing.care_cost_shares DROP COLUMN IF EXISTS scope;
`

func parentBillingCostShareScopeUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCostShareScopeUpSQL); err != nil {
		return fmt.Errorf("add the scope of the cost bearer shares: %w", err)
	}
	return nil
}

func parentBillingCostShareScopeDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCostShareScopeDownSQL); err != nil {
		return fmt.Errorf("drop the scope of the cost bearer shares: %w", err)
	}
	return nil
}
