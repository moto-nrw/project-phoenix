package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayLunchVersion = "20261114151835"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayLunchVersion,
		Description: "Ferien: Mittagessen je Woche anbieten, je Kind buchen und als eigene Position abrechnen (#5631)",
		DependsOn: []string{
			holidayDayBookingVersion,            // bookings by days and booked_days on the contribution lines (1.15.885)
			parentBillingBookingAccountsVersion, // the revenue accounts and their scopes (1.15.906)
			holidayHostSchoolCostCenterVersion,  // the newest change of document_positions (1.15.915)
			holidayHomeSchoolBookingsVersion,    // the home school function this one drops for its repeatable definition (1.15.957)
		},
	})
	Migrations.MustRegister(holidayLunchUp, holidayLunchDown)
}

// holidayLunchUpSQL lets the Träger-Büro offer lunch in a holiday week and
// bills it as a position of its own (#5631, ADR 0115 addendum).
//
//   - holiday_period_weeks.lunch_price_cents: the price of one lunch; NULL
//     offers no lunch. A booking with lunch takes one lunch on each of its
//     care days.
//   - holiday_registration_bookings.lunch: the child eats lunch on the care
//     days of this booking. The application books it only in a week that
//     offers lunch.
//   - holiday_guests.lunch: the copy for the hosting school, like days.
//   - holiday_contribution_lines / document_positions kind 'meal': the lunch
//     of one booked week, outside the rule chain: no sibling discount, no
//     exemption and no cost takeover reaches it. booked_days is the number
//     of lunches the line prices; document_positions gets the column so the
//     invoice can name it.
//   - revenue_accounts scope 'holiday_meals': the revenue account of the
//     holiday lunch; without one the account of the holiday care applies.
//   - enrollment.holiday_home_school_bookings() is dropped: its return type
//     gains the lunch, and its one current definition now lives in
//     repeatable_enrollment_holiday_home_school_bookings.go (ADR 0223).
const holidayLunchUpSQL = `
-- IF NOT EXISTS: the down migration keeps these two columns for the
-- repeatable home school function.
ALTER TABLE enrollment.holiday_period_weeks ADD COLUMN IF NOT EXISTS lunch_price_cents INTEGER;
ALTER TABLE enrollment.holiday_period_weeks DROP CONSTRAINT IF EXISTS chk_holiday_period_weeks_lunch_price;
ALTER TABLE enrollment.holiday_period_weeks ADD CONSTRAINT chk_holiday_period_weeks_lunch_price
	CHECK (lunch_price_cents IS NULL OR lunch_price_cents BETWEEN 0 AND 1000000);
COMMENT ON COLUMN enrollment.holiday_period_weeks.lunch_price_cents IS
	'Price of one lunch in the week; NULL offers no lunch (#5631).';

ALTER TABLE enrollment.holiday_registration_bookings ADD COLUMN IF NOT EXISTS lunch BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN enrollment.holiday_registration_bookings.lunch IS
	'The child eats lunch on the care days of this booking (#5631).';

ALTER TABLE enrollment.holiday_guests ADD COLUMN lunch BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN enrollment.holiday_guests.lunch IS
	'Lunch of the booking copied to the hosting school (#5631).';

ALTER TABLE parent_billing.holiday_contribution_lines
	DROP CONSTRAINT chk_holiday_contribution_lines_kind,
	ADD CONSTRAINT chk_holiday_contribution_lines_kind
		CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'cost_takeover', 'meal')),
	DROP CONSTRAINT chk_holiday_contribution_lines_discount,
	ADD CONSTRAINT chk_holiday_contribution_lines_discount CHECK (kind IN ('base', 'meal') OR amount_cents <= 0),
	ADD CONSTRAINT chk_holiday_contribution_lines_meal CHECK (kind <> 'meal' OR (
		amount_cents >= 0 AND week_starts_on IS NOT NULL AND week_ends_on IS NOT NULL AND booked_days IS NOT NULL));

ALTER TABLE parent_billing.document_positions
	ADD COLUMN booked_days SMALLINT,
	DROP CONSTRAINT chk_document_positions_kind,
	ADD CONSTRAINT chk_document_positions_kind
		CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'cost_takeover', 'correction', 'credit', 'meal')),
	ADD CONSTRAINT chk_document_positions_booked_days CHECK (booked_days IS NULL OR booked_days BETWEEN 1 AND 7);
COMMENT ON COLUMN parent_billing.document_positions.booked_days IS
	'Number of lunches a meal position prices (#5631); NULL for every other position.';

ALTER TABLE parent_billing.revenue_accounts
	DROP CONSTRAINT chk_revenue_accounts_scope,
	ADD CONSTRAINT chk_revenue_accounts_scope CHECK (scope IN ('care', 'meals', 'holiday', 'holiday_meals', 'offering'));

DROP FUNCTION IF EXISTS enrollment.holiday_home_school_bookings();
`

// holidayLunchDownSQL takes the billing of the lunch away again. It refuses
// while a line or a position bills a lunch: an issued invoice never changes,
// and dropping the kind would lose what it billed. The lunch price of the
// weeks and the lunch of the bookings stay: the repeatable definition of
// enrollment.holiday_home_school_bookings() reads them and is installed
// after every migrate run (ADR 0223), and nothing bills them without the
// kind 'meal'.
const holidayLunchDownSQL = `
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM parent_billing.holiday_contribution_lines WHERE kind = 'meal')
		OR EXISTS (SELECT 1 FROM parent_billing.document_positions WHERE kind = 'meal') THEN
		RAISE EXCEPTION 'holiday lunch is billed; the migration cannot be reverted';
	END IF;
END $$;

DELETE FROM parent_billing.revenue_accounts WHERE scope = 'holiday_meals';
ALTER TABLE parent_billing.revenue_accounts
	DROP CONSTRAINT chk_revenue_accounts_scope,
	ADD CONSTRAINT chk_revenue_accounts_scope CHECK (scope IN ('care', 'meals', 'holiday', 'offering'));

ALTER TABLE parent_billing.document_positions
	DROP CONSTRAINT chk_document_positions_booked_days,
	DROP CONSTRAINT chk_document_positions_kind,
	ADD CONSTRAINT chk_document_positions_kind
		CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'cost_takeover', 'correction', 'credit')),
	DROP COLUMN booked_days;

ALTER TABLE parent_billing.holiday_contribution_lines
	DROP CONSTRAINT chk_holiday_contribution_lines_meal,
	DROP CONSTRAINT chk_holiday_contribution_lines_discount,
	ADD CONSTRAINT chk_holiday_contribution_lines_discount CHECK (kind = 'base' OR amount_cents <= 0),
	DROP CONSTRAINT chk_holiday_contribution_lines_kind,
	ADD CONSTRAINT chk_holiday_contribution_lines_kind
		CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'cost_takeover'));

ALTER TABLE enrollment.holiday_guests DROP COLUMN lunch;
`

func holidayLunchUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayLunchUpSQL); err != nil {
			return fmt.Errorf("add holiday lunch: %w", err)
		}
		return nil
	})
}

func holidayLunchDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayLunchDownSQL); err != nil {
			return fmt.Errorf("drop holiday lunch: %w", err)
		}
		return nil
	})
}
