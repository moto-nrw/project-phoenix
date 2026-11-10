package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayCareWishVersion = "1.15.886"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayCareWishVersion,
		Description: "Ferienwunsch aus der OGS-Anmeldung, Vorrang je Ferienzeitraum und Prüfkennzeichen je angemeldetem Kind (#4703)",
		DependsOn: []string{
			holidayWeekCapacityVersion, // the newest migration on development (1.15.878)
		},
	})
	Migrations.MustRegister(holidayCareWishUp, holidayCareWishDown)
}

// holidayCareWishUpSQL adds the holiday care wish (#4703):
//
//   - request_children.holiday_care_wish: the answer of the parents in the
//     online registration of a school whether the child needs care in the
//     holidays. NULL when the form did not ask. It counts once the child is
//     approved, for the school year of the phase.
//   - holiday_periods.holiday_wish_rule: the Träger-Büro gives children with
//     a holiday care wish the precedence in the period. Off by default.
//   - holiday_registration_children.holiday_wish and holiday_wish_source:
//     what is known about the wish of a registered child of a school of the
//     Träger. 'account' is the wish a parent with the relationship right
//     brought from the prefill (ADR 0113); 'office' is the decision of the
//     Träger-Büro. NULL is unknown: the office checks it. A holiday guest
//     has no school of the Träger and so no wish.
const holidayCareWishUpSQL = `
ALTER TABLE enrollment.request_children ADD COLUMN IF NOT EXISTS holiday_care_wish BOOLEAN;
COMMENT ON COLUMN enrollment.request_children.holiday_care_wish IS
	'Holiday care wish of the parents in the online registration (#4703); NULL when the form did not ask. Counts for the school year of the phase once the child is approved.';

ALTER TABLE enrollment.holiday_periods ADD COLUMN IF NOT EXISTS holiday_wish_rule BOOLEAN NOT NULL DEFAULT false;
COMMENT ON COLUMN enrollment.holiday_periods.holiday_wish_rule IS
	'Children of the Träger''s schools without a holiday care wish wait behind the others (#4703); the Träger setting decides whether they may register at all.';

ALTER TABLE enrollment.holiday_registration_children ADD COLUMN IF NOT EXISTS holiday_wish TEXT;
ALTER TABLE enrollment.holiday_registration_children ADD COLUMN IF NOT EXISTS holiday_wish_source TEXT;
ALTER TABLE enrollment.holiday_registration_children DROP CONSTRAINT IF EXISTS chk_holiday_registration_children_wish;
ALTER TABLE enrollment.holiday_registration_children ADD CONSTRAINT chk_holiday_registration_children_wish CHECK (
	(holiday_wish IS NULL AND holiday_wish_source IS NULL)
	OR (holiday_wish IN ('yes', 'no') AND holiday_wish_source IN ('account', 'office') AND home_school_id IS NOT NULL));
COMMENT ON COLUMN enrollment.holiday_registration_children.holiday_wish IS
	'yes or no: the holiday care wish of the child for the school year (#4703); NULL is unknown and the Träger-Büro checks it. Never set for a holiday guest.';
COMMENT ON COLUMN enrollment.holiday_registration_children.holiday_wish_source IS
	'account: from the prefill of a parent with the relationship right; office: set by the Träger-Büro (#4703).';
`

const holidayCareWishDownSQL = `
ALTER TABLE enrollment.holiday_registration_children DROP CONSTRAINT IF EXISTS chk_holiday_registration_children_wish;
ALTER TABLE enrollment.holiday_registration_children DROP COLUMN IF EXISTS holiday_wish_source;
ALTER TABLE enrollment.holiday_registration_children DROP COLUMN IF EXISTS holiday_wish;
ALTER TABLE enrollment.holiday_periods DROP COLUMN IF EXISTS holiday_wish_rule;
ALTER TABLE enrollment.request_children DROP COLUMN IF EXISTS holiday_care_wish;
`

func holidayCareWishUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.886: holiday care wish (#4703)...")
	_, err := db.ExecContext(ctx, holidayCareWishUpSQL)
	return err
}

func holidayCareWishDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.886: holiday care wish...")
	_, err := db.ExecContext(ctx, holidayCareWishDownSQL)
	return err
}
