package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("enrollment.holiday_home_school_bookings", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatableEnrollmentHolidayHomeSchoolBookingsSQL)
		return err
	})
}

// repeatableEnrollmentHolidayHomeSchoolBookingsSQL is the current definition
// of enrollment.holiday_home_school_bookings() (ADR 0223, first defined by
// 1.15.957 for #6679). Change the function here; numbered migrations no
// longer define it.
//
// It lets the home school of a child read the confirmed holiday bookings of
// its own children in the weeks another school of its Träger hosts: names
// and date of birth, the week, the hosting school, the booked time model
// and days and whether the child eats lunch there (#5631). No health
// information, no contacts, no answers, no guardian, no price. The return
// type gained the lunch; the migration of #5631 dropped the older function,
// so CREATE OR REPLACE installs this one.
const repeatableEnrollmentHolidayHomeSchoolBookingsSQL = `
CREATE OR REPLACE FUNCTION enrollment.holiday_home_school_bookings()
	RETURNS TABLE (organization_id BIGINT, child_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE,
		week_id BIGINT, period_name TEXT, starts_on DATE, ends_on DATE, host_school_id BIGINT,
		time_model_name TEXT, starts_at TEXT, ends_at TEXT, days DATE[], lunch BOOLEAN)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT child.organization_id, child.id, child.first_name, child.last_name, child.birth_date,
		week.id, period.name, week.starts_on, week.ends_on, week.host_school_id,
		model.name, to_char(model.starts_at, 'HH24:MI'), to_char(model.ends_at, 'HH24:MI'), booking.days,
		booking.lunch AND week.lunch_price_cents IS NOT NULL
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
	'Confirmed, placed holiday bookings of the home school''s own children in open weeks other schools of its Träger host, with their lunch (#6679, #5631).';
`
