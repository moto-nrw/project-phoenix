package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayChildIdentitiesVersion = "20261114072003"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayChildIdentitiesVersion,
		Description: "Elternabrechnung: bekannte Ferienkinder beim Wechsel zur Kind-Debitorennummer behalten (#7007)",
		DependsOn:   []string{childDebtorNumbersVersion},
	})
	Migrations.MustRegister(holidayChildIdentitiesUp, holidayChildIdentitiesDown)
}

// Backfill identities from confirmed registrations that predate child
// numbers. The shared registration/payer number never becomes a child's
// number. Existing individual numbers stay unchanged; their last use can
// only move forward. The single statement records every inserted/extended
// identity without attributing the migration to an office account.
const holidayChildIdentitiesSQL = `WITH identities AS (
	SELECT registration.organization_id, LOWER(BTRIM(registration.guardian_email)) AS family_email,
		LOWER(BTRIM(child.first_name)) || chr(31) || LOWER(BTRIM(child.last_name)) || chr(31) || child.birth_date::text AS child_key,
		MIN(BTRIM(child.first_name)) AS first_name, MIN(BTRIM(child.last_name)) AS last_name, child.birth_date,
		GREATEST((CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date, MAX(week.ends_on)) AS last_used_on
	FROM enrollment.holiday_registrations AS registration
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = registration.organization_id AND child.holiday_registration_id = registration.id
	LEFT JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
	LEFT JOIN enrollment.holiday_period_weeks AS week
		ON week.organization_id = booking.organization_id AND week.id = booking.holiday_period_week_id
	WHERE registration.status = 'confirmed'
	GROUP BY registration.organization_id, LOWER(BTRIM(registration.guardian_email)),
		LOWER(BTRIM(child.first_name)), LOWER(BTRIM(child.last_name)), child.birth_date
), written AS (
	INSERT INTO parent_billing.child_debtor_numbers
		(organization_id, family_email, child_key, child_first_name, child_last_name, child_birth_date, debtor_number, last_used_on)
	SELECT organization_id, family_email, child_key, first_name, last_name, birth_date, '', last_used_on FROM identities
	ON CONFLICT (organization_id, family_email, child_key) DO UPDATE
		SET last_used_on = GREATEST(child_debtor_numbers.last_used_on, EXCLUDED.last_used_on)
		WHERE EXCLUDED.last_used_on > child_debtor_numbers.last_used_on
	RETURNING organization_id, id
)
INSERT INTO parent_billing.child_debtor_number_events (organization_id, child_debtor_number_id, action)
SELECT organization_id, id, 'save' FROM written`

func holidayChildIdentitiesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayChildIdentitiesSQL); err != nil {
		return fmt.Errorf("backfill the identities of confirmed holiday children: %w", err)
	}
	return nil
}

// An identity can have received an individual number since the upgrade.
// Rolling back this data-only migration must not delete such user data.
func holidayChildIdentitiesDown(_ context.Context, _ *bun.DB) error { return nil }
