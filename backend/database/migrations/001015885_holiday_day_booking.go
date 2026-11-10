package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayDayBookingVersion = "1.15.885"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayDayBookingVersion,
		Description: "Ferien: Tagesbuchung je Woche und Grenze in Betreuungstagen je Kind und Zeitraum (#4701)",
		DependsOn: []string{
			parentBillingDatevExportFilesVersion, // the newest migration on development
			holidayWeekCapacityVersion,           // the booked windows this one extends
		},
	})
	Migrations.MustRegister(holidayDayBookingUp, holidayDayBookingDown)
}

// holidayDayBookingUpSQL lets the Träger-Büro offer a week by days and bound
// the care days of a child in a period (#4701, decision E-f A).
//
//   - holiday_periods.max_days_per_child: the care days a child may book in
//     the period, over all its weeks, time models and host schools; NULL is
//     no bound. A week booking counts its care days (Monday to Friday of the
//     week), a booking of days its days.
//   - holiday_period_weeks.booking_unit: "week" books the whole week, "day"
//     single days. The capacity of a week by days holds for each of its days.
//   - holiday_registration_bookings.days: the days a booking of a week by
//     days names, NULL for a booking of the whole week. The application
//     checks them against the week; the CHECK bounds their number.
//   - parent_billing.holiday_contribution_lines.booked_days: the number of
//     days a base line prices, NULL for a whole week. The price of a time
//     model of a week by days is the price of one day.
const holidayDayBookingUpSQL = `
ALTER TABLE enrollment.holiday_periods ADD COLUMN IF NOT EXISTS max_days_per_child INTEGER;
ALTER TABLE enrollment.holiday_periods DROP CONSTRAINT IF EXISTS chk_holiday_periods_max_days_per_child;
ALTER TABLE enrollment.holiday_periods ADD CONSTRAINT chk_holiday_periods_max_days_per_child
	CHECK (max_days_per_child IS NULL OR max_days_per_child BETWEEN 1 AND 366);
COMMENT ON COLUMN enrollment.holiday_periods.max_days_per_child IS
	'Care days one child may book in the period over all its weeks, NULL for no bound (#4701).';

ALTER TABLE enrollment.holiday_period_weeks ADD COLUMN IF NOT EXISTS booking_unit TEXT NOT NULL DEFAULT 'week';
ALTER TABLE enrollment.holiday_period_weeks DROP CONSTRAINT IF EXISTS chk_holiday_period_weeks_booking_unit;
ALTER TABLE enrollment.holiday_period_weeks ADD CONSTRAINT chk_holiday_period_weeks_booking_unit
	CHECK (booking_unit IN ('week', 'day'));
COMMENT ON COLUMN enrollment.holiday_period_weeks.booking_unit IS
	'week: a booking takes the whole week; day: a booking names single days, and the capacity holds for each day (#4701).';

ALTER TABLE enrollment.holiday_registration_bookings ADD COLUMN IF NOT EXISTS days DATE[];
ALTER TABLE enrollment.holiday_registration_bookings DROP CONSTRAINT IF EXISTS chk_holiday_registration_bookings_days;
ALTER TABLE enrollment.holiday_registration_bookings ADD CONSTRAINT chk_holiday_registration_bookings_days
	CHECK (days IS NULL OR (cardinality(days) BETWEEN 1 AND 7 AND array_position(days, NULL) IS NULL));
COMMENT ON COLUMN enrollment.holiday_registration_bookings.days IS
	'The booked days of a week offered by days, ascending; NULL books the whole week (#4701).';

ALTER TABLE parent_billing.holiday_contribution_lines ADD COLUMN IF NOT EXISTS booked_days SMALLINT;
ALTER TABLE parent_billing.holiday_contribution_lines DROP CONSTRAINT IF EXISTS chk_holiday_contribution_lines_booked_days;
ALTER TABLE parent_billing.holiday_contribution_lines ADD CONSTRAINT chk_holiday_contribution_lines_booked_days
	CHECK (booked_days IS NULL OR booked_days BETWEEN 1 AND 7);
COMMENT ON COLUMN parent_billing.holiday_contribution_lines.booked_days IS
	'Days a base line prices at the price of one day, NULL for a whole week (#4701).';
`

const holidayDayBookingDownSQL = `
ALTER TABLE parent_billing.holiday_contribution_lines DROP CONSTRAINT IF EXISTS chk_holiday_contribution_lines_booked_days;
ALTER TABLE parent_billing.holiday_contribution_lines DROP COLUMN IF EXISTS booked_days;
ALTER TABLE enrollment.holiday_registration_bookings DROP CONSTRAINT IF EXISTS chk_holiday_registration_bookings_days;
ALTER TABLE enrollment.holiday_registration_bookings DROP COLUMN IF EXISTS days;
ALTER TABLE enrollment.holiday_period_weeks DROP CONSTRAINT IF EXISTS chk_holiday_period_weeks_booking_unit;
ALTER TABLE enrollment.holiday_period_weeks DROP COLUMN IF EXISTS booking_unit;
ALTER TABLE enrollment.holiday_periods DROP CONSTRAINT IF EXISTS chk_holiday_periods_max_days_per_child;
ALTER TABLE enrollment.holiday_periods DROP COLUMN IF EXISTS max_days_per_child;
`

// holidayBookedDayWindowsSQL is the booked windows function of 1.15.878 that
// keeps a booking of days to its days (#4701): each booked day is a window
// of its own, a booking of the whole week stays one window. A rebooking
// trail entry names the days it held under "days". The Soll of the host
// school, the guests' care windows and their retention read it.
const holidayBookedDayWindowsSQL = `
CREATE OR REPLACE FUNCTION enrollment.holiday_host_booked_windows_for_retention(p_from date, p_to date, p_school bigint, p_registration bigint)
 RETURNS TABLE(organization_id bigint, child_id bigint, week_id bigint, home_school_id bigint, first_name text, last_name text, birth_date date, starts_on date, ends_on date, starts_at time without time zone, ends_at time without time zone)
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'pg_temp'
AS $function$
	WITH host AS (
		SELECT p_school AS school_id
	),
 guest_access AS (
  SELECT guest.id, guest.organization_id, guest.holiday_registration_child_id,
   guest.holiday_period_week_id, guest.starts_on, guest.ends_on
  FROM enrollment.holiday_guests AS guest
  WHERE guest.tenant_id = (SELECT school_id FROM host)
 ),
	own AS (
		SELECT child.id AS child_id, child.organization_id, child.home_school_id, child.first_name, child.last_name, child.birth_date,
			registration.id AS registration_id, cancellation.cancelled_on
		FROM host
		JOIN platform.schools AS host_school ON host_school.id = host.school_id
		JOIN enrollment.holiday_registration_children AS child
   ON child.organization_id = host_school.organization_id OR EXISTS (
    SELECT 1 FROM guest_access AS granted
    WHERE granted.organization_id = child.organization_id AND granted.holiday_registration_child_id = child.id)
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = registration.organization_id AND period.id = registration.holiday_period_id
		LEFT JOIN LATERAL (
			SELECT (cancel.occurred_at AT TIME ZONE 'Europe/Berlin')::date AS cancelled_on
			FROM enrollment.holiday_registration_audit AS cancel
			WHERE cancel.organization_id = registration.organization_id AND cancel.holiday_registration_id = registration.id
				AND cancel.action = 'cancel'
				AND EXISTS (
					SELECT 1 FROM enrollment.holiday_registration_audit AS confirm
					WHERE confirm.organization_id = cancel.organization_id
						AND confirm.holiday_registration_id = cancel.holiday_registration_id
						AND confirm.action = 'confirm' AND confirm.occurred_at < cancel.occurred_at)
			ORDER BY cancel.occurred_at DESC
			LIMIT 1
		) AS cancellation ON true
		WHERE period.status = 'published' AND (p_registration IS NULL OR registration.id = p_registration)
			AND (registration.status = 'confirmed'
				OR registration.status = 'cancelled' AND cancellation.cancelled_on IS NOT NULL)
	),
	rebook AS (
		SELECT own.child_id, audit.occurred_at, (audit.occurred_at AT TIME ZONE 'Europe/Berlin')::date AS on_date, audit.details
		FROM own
		JOIN enrollment.holiday_registration_audit AS audit
			ON audit.organization_id = own.organization_id AND audit.holiday_registration_id = own.registration_id
				AND audit.action = 'rebook' AND audit.details->>'child_id' = own.child_id::text
	),
	segment AS (
		SELECT own.child_id, booking.holiday_period_week_id AS week_id, booking.holiday_week_time_model_id AS model_id,
			GREATEST((SELECT max(rebook.on_date) FROM rebook WHERE rebook.child_id = own.child_id), (booking.promoted_at AT TIME ZONE 'Europe/Berlin')::date) AS valid_from,
			NULL::date AS valid_until,
			booking.days
		FROM own
		JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = own.organization_id AND booking.holiday_registration_child_id = own.child_id
			AND booking.placement = 'placed'
		UNION ALL
		SELECT rebook.child_id, (previous->>'week_id')::bigint, (previous->>'time_model_id')::bigint,
			GREATEST((SELECT max(earlier.on_date) FROM rebook AS earlier
				WHERE earlier.child_id = rebook.child_id AND earlier.occurred_at < rebook.occurred_at), ((previous->>'promoted_at')::timestamptz AT TIME ZONE 'Europe/Berlin')::date),
			rebook.on_date - 1,
			CASE WHEN jsonb_typeof(previous->'days') = 'array'
				THEN ARRAY(SELECT booked_day::date FROM jsonb_array_elements_text(previous->'days') AS booked_day) END
		FROM rebook
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(rebook.details->'before', '[]'::jsonb)) AS previous
		WHERE COALESCE((previous->>'waitlisted')::boolean, false) = false
	),
	booked AS (
		SELECT own.organization_id, own.child_id, week.id AS week_id, own.home_school_id, own.first_name, own.last_name, own.birth_date, care_window.starts_at, care_window.ends_at,
			GREATEST(COALESCE(granted.starts_on, week.starts_on), segment.valid_from, care_window.valid_from, booked_day.day) AS first_day,
			LEAST(COALESCE(granted.ends_on, week.ends_on), segment.valid_until, own.cancelled_on - 1, care_window.valid_until - 1, booked_day.day) AS last_day
		FROM segment
		JOIN own ON own.child_id = segment.child_id
		JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = own.organization_id AND week.id = segment.week_id
		JOIN platform.schools AS school
			ON school.id = week.host_school_id AND school.organization_id = week.organization_id
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = week.organization_id AND model.holiday_period_week_id = week.id
				AND model.id = segment.model_id
  LEFT JOIN guest_access AS granted ON granted.organization_id = own.organization_id
   AND granted.holiday_registration_child_id = own.child_id AND granted.holiday_period_week_id = week.id
		CROSS JOIN LATERAL enrollment.holiday_time_model_windows(model.care_time_history, model.starts_at, model.ends_at) AS care_window
		CROSS JOIN LATERAL (
			SELECT chosen.day FROM unnest(segment.days) AS chosen(day) WHERE segment.days IS NOT NULL
			UNION ALL
			SELECT NULL::date WHERE segment.days IS NULL
		) AS booked_day
		WHERE week.host_school_id = (SELECT host.school_id FROM host) OR granted.id IS NOT NULL
	)
	SELECT organization_id, child_id, week_id, home_school_id, first_name, last_name, birth_date, first_day, last_day, starts_at, ends_at
	FROM booked
	WHERE first_day <= last_day AND first_day <= p_to AND last_day >= p_from
	$function$;

`

// holidayBookedWeekWindowsSQL is the booked windows function of 1.15.878, as
// the down migration writes it back.
const holidayBookedWeekWindowsSQL = `
CREATE OR REPLACE FUNCTION enrollment.holiday_host_booked_windows_for_retention(p_from date, p_to date, p_school bigint, p_registration bigint)
 RETURNS TABLE(organization_id bigint, child_id bigint, week_id bigint, home_school_id bigint, first_name text, last_name text, birth_date date, starts_on date, ends_on date, starts_at time without time zone, ends_at time without time zone)
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'pg_temp'
AS $function$
	WITH host AS (
		SELECT p_school AS school_id
	),
 guest_access AS (
  SELECT guest.id, guest.organization_id, guest.holiday_registration_child_id,
   guest.holiday_period_week_id, guest.starts_on, guest.ends_on
  FROM enrollment.holiday_guests AS guest
  WHERE guest.tenant_id = (SELECT school_id FROM host)
 ),
	own AS (
		SELECT child.id AS child_id, child.organization_id, child.home_school_id, child.first_name, child.last_name, child.birth_date,
			registration.id AS registration_id, cancellation.cancelled_on
		FROM host
		JOIN platform.schools AS host_school ON host_school.id = host.school_id
		JOIN enrollment.holiday_registration_children AS child
   ON child.organization_id = host_school.organization_id OR EXISTS (
    SELECT 1 FROM guest_access AS granted
    WHERE granted.organization_id = child.organization_id AND granted.holiday_registration_child_id = child.id)
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = registration.organization_id AND period.id = registration.holiday_period_id
		LEFT JOIN LATERAL (
			SELECT (cancel.occurred_at AT TIME ZONE 'Europe/Berlin')::date AS cancelled_on
			FROM enrollment.holiday_registration_audit AS cancel
			WHERE cancel.organization_id = registration.organization_id AND cancel.holiday_registration_id = registration.id
				AND cancel.action = 'cancel'
				AND EXISTS (
					SELECT 1 FROM enrollment.holiday_registration_audit AS confirm
					WHERE confirm.organization_id = cancel.organization_id
						AND confirm.holiday_registration_id = cancel.holiday_registration_id
						AND confirm.action = 'confirm' AND confirm.occurred_at < cancel.occurred_at)
			ORDER BY cancel.occurred_at DESC
			LIMIT 1
		) AS cancellation ON true
		WHERE period.status = 'published' AND (p_registration IS NULL OR registration.id = p_registration)
			AND (registration.status = 'confirmed'
				OR registration.status = 'cancelled' AND cancellation.cancelled_on IS NOT NULL)
	),
	rebook AS (
		SELECT own.child_id, audit.occurred_at, (audit.occurred_at AT TIME ZONE 'Europe/Berlin')::date AS on_date, audit.details
		FROM own
		JOIN enrollment.holiday_registration_audit AS audit
			ON audit.organization_id = own.organization_id AND audit.holiday_registration_id = own.registration_id
				AND audit.action = 'rebook' AND audit.details->>'child_id' = own.child_id::text
	),
	segment AS (
		SELECT own.child_id, booking.holiday_period_week_id AS week_id, booking.holiday_week_time_model_id AS model_id,
			GREATEST((SELECT max(rebook.on_date) FROM rebook WHERE rebook.child_id = own.child_id), (booking.promoted_at AT TIME ZONE 'Europe/Berlin')::date) AS valid_from,
			NULL::date AS valid_until
		FROM own
		JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = own.organization_id AND booking.holiday_registration_child_id = own.child_id
			AND booking.placement = 'placed'
		UNION ALL
		SELECT rebook.child_id, (previous->>'week_id')::bigint, (previous->>'time_model_id')::bigint,
			GREATEST((SELECT max(earlier.on_date) FROM rebook AS earlier
				WHERE earlier.child_id = rebook.child_id AND earlier.occurred_at < rebook.occurred_at), ((previous->>'promoted_at')::timestamptz AT TIME ZONE 'Europe/Berlin')::date),
			rebook.on_date - 1
		FROM rebook
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(rebook.details->'before', '[]'::jsonb)) AS previous
		WHERE COALESCE((previous->>'waitlisted')::boolean, false) = false
	),
	booked AS (
		SELECT own.organization_id, own.child_id, week.id AS week_id, own.home_school_id, own.first_name, own.last_name, own.birth_date, care_window.starts_at, care_window.ends_at,
			GREATEST(COALESCE(granted.starts_on, week.starts_on), segment.valid_from, care_window.valid_from) AS first_day,
			LEAST(COALESCE(granted.ends_on, week.ends_on), segment.valid_until, own.cancelled_on - 1, care_window.valid_until - 1) AS last_day
		FROM segment
		JOIN own ON own.child_id = segment.child_id
		JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = own.organization_id AND week.id = segment.week_id
		JOIN platform.schools AS school
			ON school.id = week.host_school_id AND school.organization_id = week.organization_id
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = week.organization_id AND model.holiday_period_week_id = week.id
				AND model.id = segment.model_id
  LEFT JOIN guest_access AS granted ON granted.organization_id = own.organization_id
   AND granted.holiday_registration_child_id = own.child_id AND granted.holiday_period_week_id = week.id
		CROSS JOIN LATERAL enrollment.holiday_time_model_windows(model.care_time_history, model.starts_at, model.ends_at) AS care_window
		WHERE week.host_school_id = (SELECT host.school_id FROM host) OR granted.id IS NOT NULL
	)
	SELECT organization_id, child_id, week_id, home_school_id, first_name, last_name, birth_date, first_day, last_day, starts_at, ends_at
	FROM booked
	WHERE first_day <= last_day AND first_day <= p_to AND last_day >= p_from
	$function$;

`

func holidayDayBookingUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayDayBookingUpSQL); err != nil {
			return fmt.Errorf("add holiday day booking and child limit: %w", err)
		}
		if _, err := tx.ExecContext(ctx, holidayBookedDayWindowsSQL); err != nil {
			return fmt.Errorf("keep booked holiday windows to the booked days: %w", err)
		}
		return nil
	})
}

// holidayDayBookingDown writes back the booked windows of 1.15.878 before the
// days go.
func holidayDayBookingDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayBookedWeekWindowsSQL); err != nil {
			return fmt.Errorf("restore the booked holiday windows: %w", err)
		}
		if _, err := tx.ExecContext(ctx, holidayDayBookingDownSQL); err != nil {
			return fmt.Errorf("drop holiday day booking and child limit: %w", err)
		}
		return nil
	})
}
