package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHostOwnBookingsVersion = "1.15.796"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHostOwnBookingsVersion,
		Description: "Ferienbetreuung: die ausrichtende Schule liest die bestätigten Buchungen ihrer eigenen Kinder für Soll und Ist (#5994)",
		DependsOn: []string{
			careContractSignaturesVersion, // the last migration before this one
			holidayHealthDataVersion,      // the host functions this one sits beside
		},
	})
	Migrations.MustRegister(holidayHostOwnBookingsUp, holidayHostOwnBookingsDown)
}

// holidayHostOwnBookingsUpSQL lets the school that hosts a holiday week read
// the confirmed bookings of its own children in the weeks it hosts that
// overlap [p_from, p_to] (#5994, ADR 0156 addendum): the booked time model is
// the Soll of Soll und Ist on those days. A child of the school is no holiday
// guest and has no copy (ADR 0120); the registration names the school as its
// home school. The function answers in the tenant transaction of the school
// only, for published periods, also after the week (the comparison looks
// back), and returns only what the comparison needs to find the child and
// its Soll: names, date of birth, the week and the time model. No health
// information, no contacts, no answers. The registrations themselves go 92
// days after their period (#4720), and with them these rows.
const holidayHostOwnBookingsUpSQL = `
CREATE FUNCTION enrollment.holiday_host_own_bookings(p_from DATE, p_to DATE)
	RETURNS TABLE (first_name TEXT, last_name TEXT, birth_date DATE, starts_on DATE, ends_on DATE,
		starts_at TIME, ends_at TIME)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT child.first_name, child.last_name, child.birth_date, week.starts_on, week.ends_on,
		model.starts_at, model.ends_at
	FROM enrollment.holiday_period_weeks AS week
	JOIN enrollment.holiday_periods AS period
		ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
	JOIN platform.schools AS school
		ON school.id = week.host_school_id AND school.organization_id = week.organization_id
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
	JOIN enrollment.holiday_week_time_models AS model
		ON model.organization_id = booking.organization_id AND model.holiday_period_week_id = week.id
			AND model.id = booking.holiday_week_time_model_id
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	WHERE week.host_school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND child.home_school_id = week.host_school_id
		AND period.status = 'published'
		AND registration.status = 'confirmed'
		AND week.starts_on <= p_to AND week.ends_on >= p_from
	$$;
REVOKE ALL ON FUNCTION enrollment.holiday_host_own_bookings(DATE, DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_own_bookings(DATE, DATE) TO phoenix_tenant;
COMMENT ON FUNCTION enrollment.holiday_host_own_bookings(DATE, DATE) IS
	'Confirmed holiday bookings of the hosting school''s own children in the weeks it hosts, for the Soll of Soll und Ist (#5994).';
`

const holidayHostOwnBookingsDownSQL = `DROP FUNCTION IF EXISTS enrollment.holiday_host_own_bookings(DATE, DATE);`

func holidayHostOwnBookingsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostOwnBookingsUpSQL); err != nil {
		return fmt.Errorf("add the hosting school's read of its own children's holiday bookings: %w", err)
	}
	return nil
}

func holidayHostOwnBookingsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostOwnBookingsDownSQL); err != nil {
		return fmt.Errorf("drop the hosting school's read of its own children's holiday bookings: %w", err)
	}
	return nil
}
