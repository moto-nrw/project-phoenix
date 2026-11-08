package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCarePriceFormsVersion = "1.15.817"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCarePriceFormsVersion,
		Description: "Elternabrechnung: Staffel nach Tagen je Woche, Geschwisterpreis je Modul und beitragsfreie Monate im Tarif (#6196)",
		DependsOn: []string{
			parentBillingCareReviewVersion,     // the last migration of the monthly billing before this one
			parentBillingCareReductionsVersion, // the care rules and reductions this one extends
		},
	})
	Migrations.MustRegister(parentBillingCarePriceFormsUp, parentBillingCarePriceFormsDown)
}

// parentBillingCarePriceFormsUpSQL extends the one price list of the
// monthly care contribution (#6196, ADR 0119 addendum 2026-11-08):
//
//   - care_tariffs: a price is either the monthly price of one booked
//     weekday (weekday) or, for a module priced by a scale, the monthly
//     price for a number of booked days per week (days_per_week, 1 to 7).
//     sibling_cents is the end price of the same weekday or scale step for
//     every further child; a sibling rule in the price-list mode reads it.
//     meal_extra marks a module whose price excludes the meal; the meal
//     is billed by the Mensa, never as a silent zero.
//   - care_price_lists: the head of the price list of a school from a month
//     on, with the months it bills nothing for (free_months, e.g. August).
//     It holds until the next price list of the school.
//   - care_invoice_positions: an issued position keeps the scale step it
//     billed (days_per_week, tier_monthly_cents) and the meal note.
//   - contribution_rules and care_invoice_reductions: a care sibling rule
//     may take the sibling price of the price list (tariff_sibling_price)
//     instead of a share, an amount or one end price; the reduction keeps
//     a copy of that mode.
//
// No price stands in the code; issued invoices keep their copies.
const parentBillingCarePriceFormsUpSQL = `
ALTER TABLE parent_billing.care_tariffs
	ALTER COLUMN weekday DROP NOT NULL,
	ADD COLUMN days_per_week SMALLINT,
	ADD COLUMN sibling_cents BIGINT,
	ADD COLUMN meal_extra BOOLEAN NOT NULL DEFAULT false,
	DROP CONSTRAINT uq_care_tariffs_price,
	DROP CONSTRAINT chk_care_tariffs_weekday;
ALTER TABLE parent_billing.care_tariffs
	ADD CONSTRAINT chk_care_tariffs_key CHECK (num_nonnulls(weekday, days_per_week) = 1),
	ADD CONSTRAINT chk_care_tariffs_weekday CHECK (weekday IS NULL OR weekday IN ('mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun')),
	ADD CONSTRAINT chk_care_tariffs_days CHECK (days_per_week IS NULL OR days_per_week BETWEEN 1 AND 7),
	ADD CONSTRAINT chk_care_tariffs_sibling CHECK (sibling_cents IS NULL OR sibling_cents BETWEEN 0 AND 100000000);
CREATE UNIQUE INDEX uq_care_tariffs_weekday_price ON parent_billing.care_tariffs
	(organization_id, school_id, care_offering_id, weekday, valid_from) WHERE weekday IS NOT NULL;
CREATE UNIQUE INDEX uq_care_tariffs_scale_price ON parent_billing.care_tariffs
	(organization_id, school_id, care_offering_id, days_per_week, valid_from) WHERE days_per_week IS NOT NULL;
COMMENT ON TABLE parent_billing.care_tariffs IS
	'Monthly price of one booked weekday or one scale step (days per week) of a care module at a school, with the price for every further child, from the first day of a month until the next price (#4684, #6196).';

CREATE TABLE parent_billing.care_price_lists (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	school_id BIGINT NOT NULL,
	valid_from DATE NOT NULL,
	free_months SMALLINT[] NOT NULL DEFAULT '{}',
	PRIMARY KEY (organization_id, school_id, valid_from),
	CONSTRAINT chk_care_price_lists_month CHECK (EXTRACT(DAY FROM valid_from) = 1),
	CONSTRAINT chk_care_price_lists_free CHECK (free_months <@ ARRAY[1,2,3,4,5,6,7,8,9,10,11,12]::SMALLINT[])
);
COMMENT ON TABLE parent_billing.care_price_lists IS
	'Head of the price list of a school from a month on: the months the monthly run bills nothing for (#6196).';
INSERT INTO parent_billing.care_price_lists (organization_id, school_id, valid_from)
SELECT DISTINCT organization_id, school_id, valid_from FROM parent_billing.care_tariffs;

ALTER TABLE parent_billing.care_invoice_positions
	ADD COLUMN days_per_week SMALLINT,
	ADD COLUMN tier_monthly_cents BIGINT,
	ADD COLUMN meal_extra BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE parent_billing.care_invoice_positions
	ADD CONSTRAINT chk_care_invoice_positions_scale CHECK (
		(days_per_week IS NULL) = (tier_monthly_cents IS NULL)
		AND (days_per_week IS NULL OR days_per_week BETWEEN 1 AND 7)
		AND (tier_monthly_cents IS NULL OR tier_monthly_cents BETWEEN 0 AND 100000000));

ALTER TABLE parent_billing.contribution_rules
	ADD COLUMN tariff_sibling_price BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT chk_contribution_rules_care;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_care CHECK (service <> 'care' OR (
	BTRIM(name) <> '' AND char_length(name) <= 120 AND BTRIM(note) = ''
	AND cardinality(school_ids) BETWEEN 1 AND 200 AND cardinality(care_offering_ids) <= 500
	AND EXTRACT(DAY FROM valid_from) = 1
	AND (valid_until IS NULL OR EXTRACT(DAY FROM valid_until + 1) = 1)
	AND (kind = 'sibling' OR cardinality(count_school_ids) = 0) AND cardinality(count_school_ids) <= 200
	AND (kind = 'sibling') = (sibling_from_child IS NOT NULL)
	AND (sibling_from_child IS NULL OR sibling_from_child BETWEEN 2 AND 9)
	AND (discount_basis_points IS NULL OR discount_basis_points BETWEEN 1 AND 10000)
	AND (discount_cents IS NULL OR discount_cents BETWEEN 1 AND 100000000)
	AND (price_cents IS NULL OR price_cents BETWEEN 0 AND 100000000)
	AND (cap_cents IS NULL OR cap_cents BETWEEN 0 AND 100000000)
	AND (kind = 'sibling' OR NOT tariff_sibling_price)
	AND CASE kind
		WHEN 'cap' THEN cap_cents IS NOT NULL AND num_nonnulls(discount_basis_points, discount_cents, price_cents) = 0
		WHEN 'single_parent' THEN cap_cents IS NULL AND price_cents IS NULL
			AND num_nonnulls(discount_basis_points, discount_cents) = 1
		ELSE cap_cents IS NULL
			AND num_nonnulls(discount_basis_points, discount_cents, price_cents) + tariff_sibling_price::int = 1
	END));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT chk_contribution_rules_holiday;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_holiday CHECK (service <> 'holiday' OR (
	BTRIM(name) = '' AND cardinality(school_ids) = 0 AND cardinality(care_offering_ids) = 0
	AND cardinality(count_school_ids) = 0 AND price_cents IS NULL AND cap_cents IS NULL AND NOT tariff_sibling_price));

ALTER TABLE parent_billing.care_invoice_reductions
	ADD COLUMN tariff_sibling_price BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE parent_billing.care_invoice_reductions
	ADD CONSTRAINT chk_care_invoice_reductions_tariff CHECK (kind = 'sibling' OR NOT tariff_sibling_price);

ALTER TABLE parent_billing.care_price_lists ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_price_lists FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_price_lists ON parent_billing.care_price_lists FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
GRANT SELECT, INSERT, DELETE ON parent_billing.care_price_lists TO phoenix_tenant;
GRANT ALL ON parent_billing.care_price_lists TO phoenix_admin;
`

// The down migration drops what only the new forms carry: scale prices,
// the heads of the price lists and the price-list mode of the rules. An
// issued position or reduction that used them keeps its amounts, losing
// only the copy of the form.
const parentBillingCarePriceFormsDownSQL = `
ALTER TABLE parent_billing.care_invoice_reductions DROP CONSTRAINT chk_care_invoice_reductions_tariff;
ALTER TABLE parent_billing.care_invoice_reductions DROP COLUMN tariff_sibling_price;
DELETE FROM parent_billing.contribution_rules WHERE tariff_sibling_price;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT chk_contribution_rules_holiday;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_holiday CHECK (service <> 'holiday' OR (
	BTRIM(name) = '' AND cardinality(school_ids) = 0 AND cardinality(care_offering_ids) = 0
	AND cardinality(count_school_ids) = 0 AND price_cents IS NULL AND cap_cents IS NULL));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT chk_contribution_rules_care;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_care CHECK (service <> 'care' OR (
	BTRIM(name) <> '' AND char_length(name) <= 120 AND BTRIM(note) = ''
	AND cardinality(school_ids) BETWEEN 1 AND 200 AND cardinality(care_offering_ids) <= 500
	AND EXTRACT(DAY FROM valid_from) = 1
	AND (valid_until IS NULL OR EXTRACT(DAY FROM valid_until + 1) = 1)
	AND (kind = 'sibling' OR cardinality(count_school_ids) = 0) AND cardinality(count_school_ids) <= 200
	AND (kind = 'sibling') = (sibling_from_child IS NOT NULL)
	AND (sibling_from_child IS NULL OR sibling_from_child BETWEEN 2 AND 9)
	AND (discount_basis_points IS NULL OR discount_basis_points BETWEEN 1 AND 10000)
	AND (discount_cents IS NULL OR discount_cents BETWEEN 1 AND 100000000)
	AND (price_cents IS NULL OR price_cents BETWEEN 0 AND 100000000)
	AND (cap_cents IS NULL OR cap_cents BETWEEN 0 AND 100000000)
	AND CASE kind
		WHEN 'cap' THEN cap_cents IS NOT NULL AND num_nonnulls(discount_basis_points, discount_cents, price_cents) = 0
		WHEN 'single_parent' THEN cap_cents IS NULL AND price_cents IS NULL
			AND num_nonnulls(discount_basis_points, discount_cents) = 1
		ELSE cap_cents IS NULL AND num_nonnulls(discount_basis_points, discount_cents, price_cents) = 1
	END));
ALTER TABLE parent_billing.contribution_rules DROP COLUMN tariff_sibling_price;
ALTER TABLE parent_billing.care_invoice_positions DROP CONSTRAINT chk_care_invoice_positions_scale;
ALTER TABLE parent_billing.care_invoice_positions
	DROP COLUMN days_per_week, DROP COLUMN tier_monthly_cents, DROP COLUMN meal_extra;
DROP TABLE parent_billing.care_price_lists;
DELETE FROM parent_billing.care_tariffs WHERE days_per_week IS NOT NULL;
DROP INDEX parent_billing.uq_care_tariffs_weekday_price;
DROP INDEX parent_billing.uq_care_tariffs_scale_price;
ALTER TABLE parent_billing.care_tariffs
	DROP CONSTRAINT chk_care_tariffs_key, DROP CONSTRAINT chk_care_tariffs_weekday,
	DROP CONSTRAINT chk_care_tariffs_days, DROP CONSTRAINT chk_care_tariffs_sibling,
	DROP COLUMN days_per_week, DROP COLUMN sibling_cents, DROP COLUMN meal_extra,
	ALTER COLUMN weekday SET NOT NULL;
ALTER TABLE parent_billing.care_tariffs
	ADD CONSTRAINT uq_care_tariffs_price UNIQUE (organization_id, school_id, care_offering_id, weekday, valid_from),
	ADD CONSTRAINT chk_care_tariffs_weekday CHECK (weekday IN ('mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'));
`

func parentBillingCarePriceFormsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCarePriceFormsUpSQL); err != nil {
		return fmt.Errorf("add the price forms of the monthly care contribution: %w", err)
	}
	return nil
}

func parentBillingCarePriceFormsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCarePriceFormsDownSQL); err != nil {
		return fmt.Errorf("drop the price forms of the monthly care contribution: %w", err)
	}
	return nil
}
