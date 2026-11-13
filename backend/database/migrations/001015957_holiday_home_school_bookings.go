package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHomeSchoolBookingsVersion = "1.15.957"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHomeSchoolBookingsVersion,
		Description: "Ferienbetreuung: die Heimatschule liest die bestätigten Ferienbuchungen ihrer Kinder an anderen OGS des Trägers (#6679)",
		DependsOn: []string{
			studentNoteTeamNoticesVersion, // the newest migration on development (1.15.956)
			holidayWeekCapacityVersion,    // the placement of a booking this one reads
			holidayDayBookingVersion,      // the booked days of a week by days
		},
	})
	Migrations.MustRegister(holidayHomeSchoolBookingsUp, holidayHomeSchoolBookingsDown)
}

// holidayHomeSchoolBookingsUpSQL lets the home school of a child read the
// confirmed holiday bookings of its own children in the weeks another school
// of its Träger hosts (#6679, ADR 0116 addendum): the home school plans its
// own care and informs its families, so it needs to know which of its
// children are looked after elsewhere and when. The function answers in the
// tenant transaction of the home school only and returns what that task
// needs: the child's names and date of birth (to find the child in the
// school's own records), the week, the hosting school and the booked time
// model and days. No health information, no contacts, no answers, no
// guardian. Only a placed booking of a confirmed registration counts, in a
// week of a published period that has not ended by the Berlin calendar day;
// a cancellation or rebooking by the Träger-Büro takes the row away at once.
// Home school, hosting school and registration belong to the same Träger. A
// week the home school hosts itself is left out: its participant list shows
// those children already.
const holidayHomeSchoolBookingsUpSQL = `
CREATE FUNCTION enrollment.holiday_home_school_bookings()
	RETURNS TABLE (organization_id BIGINT, child_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE,
		week_id BIGINT, period_name TEXT, starts_on DATE, ends_on DATE, host_school_id BIGINT,
		time_model_name TEXT, starts_at TEXT, ends_at TEXT, days DATE[])
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT child.organization_id, child.id, child.first_name, child.last_name, child.birth_date,
		week.id, period.name, week.starts_on, week.ends_on, week.host_school_id,
		model.name, to_char(model.starts_at, 'HH24:MI'), to_char(model.ends_at, 'HH24:MI'), booking.days
	FROM platform.schools AS home
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = home.organization_id AND child.home_school_id = home.id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
	JOIN enrollment.holiday_period_weeks AS week
		ON week.organization_id = booking.organization_id AND week.id = booking.holiday_period_week_id
	JOIN enrollment.holiday_periods AS period
		ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
	JOIN platform.schools AS host
		ON host.id = week.host_school_id AND host.organization_id = week.organization_id
	JOIN enrollment.holiday_week_time_models AS model
		ON model.organization_id = booking.organization_id AND model.holiday_period_week_id = week.id
			AND model.id = booking.holiday_week_time_model_id
	WHERE home.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND registration.status = 'confirmed'
		AND booking.placement = 'placed'
		AND period.status = 'published'
		AND week.host_school_id <> home.id
		AND week.ends_on >= (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date
	$$;
REVOKE ALL ON FUNCTION enrollment.holiday_home_school_bookings() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_home_school_bookings() TO phoenix_tenant;
COMMENT ON FUNCTION enrollment.holiday_home_school_bookings() IS
	'Confirmed, placed holiday bookings of the home school''s own children in open weeks other schools of its Träger host (#6679).';
`

const holidayHomeSchoolBookingsDownSQL = `DROP FUNCTION IF EXISTS enrollment.holiday_home_school_bookings();`

func holidayHomeSchoolBookingsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHomeSchoolBookingsUpSQL); err != nil {
		return fmt.Errorf("add the home school's read of its children's holiday bookings: %w", err)
	}
	return nil
}

func holidayHomeSchoolBookingsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHomeSchoolBookingsDownSQL); err != nil {
		return fmt.Errorf("drop the home school's read of its children's holiday bookings: %w", err)
	}
	return nil
}
