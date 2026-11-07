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
		Description: "Ferienbetreuung: die ausrichtende Schule liest die bestätigten Buchungen ihrer eigenen Kinder für Soll und Ist, gebuchte Zeitmodelle lassen sich archivieren (#5994)",
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
//
// A registration the Träger-Büro cancels after it confirmed it keeps the
// days before the cancellation, as a guest copy keeps them through
// ended_on: those days were booked, and the comparison must not fall back
// to the contract for them. The cancellation and the confirmation before it
// come from the registration's trail (ADR 0114); a registration that was
// never confirmed books nothing. A rebooking changes the bookings from its
// day on; the days before keep the bookings the trail names as "before".
// starts_on and ends_on are the first and the last booked day.
const holidayHostOwnBookingsUpSQL = `
CREATE FUNCTION enrollment.holiday_host_own_bookings(p_from DATE, p_to DATE)
	RETURNS TABLE (first_name TEXT, last_name TEXT, birth_date DATE, starts_on DATE, ends_on DATE,
		starts_at TIME, ends_at TIME)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	WITH host AS (
		SELECT NULLIF(current_setting('app.current_tenant_id', true), '')::bigint AS school_id
	),
	own AS (
		SELECT child.id AS child_id, child.organization_id, child.first_name, child.last_name, child.birth_date,
			registration.id AS registration_id, cancellation.cancelled_on
		FROM host
		JOIN enrollment.holiday_registration_children AS child ON child.home_school_id = host.school_id
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
		WHERE period.status = 'published'
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
			(SELECT max(rebook.on_date) FROM rebook WHERE rebook.child_id = own.child_id) AS valid_from,
			NULL::date AS valid_until
		FROM own
		JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = own.organization_id AND booking.holiday_registration_child_id = own.child_id
		UNION ALL
		SELECT rebook.child_id, (previous->>'week_id')::bigint, (previous->>'time_model_id')::bigint,
			(SELECT max(earlier.on_date) FROM rebook AS earlier
				WHERE earlier.child_id = rebook.child_id AND earlier.occurred_at < rebook.occurred_at),
			rebook.on_date - 1
		FROM rebook
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(rebook.details->'before', '[]'::jsonb)) AS previous
	),
	booked AS (
		SELECT own.first_name, own.last_name, own.birth_date, model.starts_at, model.ends_at,
			GREATEST(week.starts_on, segment.valid_from) AS first_day,
			LEAST(week.ends_on, segment.valid_until, own.cancelled_on - 1) AS last_day
		FROM segment
		JOIN own ON own.child_id = segment.child_id
		JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = own.organization_id AND week.id = segment.week_id
		JOIN platform.schools AS school
			ON school.id = week.host_school_id AND school.organization_id = week.organization_id
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = week.organization_id AND model.holiday_period_week_id = week.id
				AND model.id = segment.model_id
		WHERE week.host_school_id = (SELECT host.school_id FROM host)
	)
	SELECT first_name, last_name, birth_date, first_day, last_day, starts_at, ends_at
	FROM booked
	WHERE first_day <= last_day AND first_day <= p_to AND last_day >= p_from
	$$;
REVOKE ALL ON FUNCTION enrollment.holiday_host_own_bookings(DATE, DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_own_bookings(DATE, DATE) TO phoenix_tenant;
-- A time model the trail of a rebooking names keeps the Soll of the days
-- booked on it before the rebooking: it cannot be removed while its period
-- lives, as the foreign keys of the bookings keep a booked one. The error is
-- a foreign-key violation, so the Träger-Büro gets the same 409 as for a
-- booked time model. When the period goes (retention after 92 days, the
-- erasure of a Träger), its trail goes with it and nothing is held.
CREATE FUNCTION enrollment.holiday_time_model_keeps_booked_history()
	RETURNS trigger
	LANGUAGE plpgsql
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	BEGIN
		IF EXISTS (
			SELECT 1
			FROM enrollment.holiday_registration_audit AS audit
			JOIN enrollment.holiday_periods AS period
				ON period.organization_id = audit.organization_id AND period.id = audit.holiday_period_id
			WHERE audit.organization_id = OLD.organization_id AND audit.action = 'rebook'
				AND (audit.details->'before' @> jsonb_build_array(jsonb_build_object('time_model_id', OLD.id))
					OR audit.details->'after' @> jsonb_build_array(jsonb_build_object('time_model_id', OLD.id))))
		THEN
			RAISE EXCEPTION 'holiday time model % is booked in the trail of a registration', OLD.id
				USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'holiday_time_model_booked_history';
		END IF;
		RETURN OLD;
	END
	$$;
REVOKE ALL ON FUNCTION enrollment.holiday_time_model_keeps_booked_history() FROM PUBLIC;
CREATE TRIGGER holiday_time_model_booked_history
	BEFORE DELETE ON enrollment.holiday_week_time_models
	FOR EACH ROW EXECUTE FUNCTION enrollment.holiday_time_model_keeps_booked_history();
-- A time model that cannot go can be archived instead: archived_at takes it
-- out of the offer and of every new booking, while the bookings and the
-- trail that name it keep reading it for their days. There is no way back.
ALTER TABLE enrollment.holiday_week_time_models ADD COLUMN archived_at TIMESTAMPTZ;
COMMENT ON COLUMN enrollment.holiday_week_time_models.archived_at IS
	'When the Träger-Büro archived the time model: no longer offered or bookable, still read by past bookings (#5994).';
COMMENT ON FUNCTION enrollment.holiday_host_own_bookings(DATE, DATE) IS
	'Confirmed holiday bookings of the hosting school''s own children in the weeks it hosts, for the Soll of Soll und Ist (#5994).';
`

const holidayHostOwnBookingsDownSQL = `
ALTER TABLE enrollment.holiday_week_time_models DROP COLUMN IF EXISTS archived_at;
DROP TRIGGER IF EXISTS holiday_time_model_booked_history ON enrollment.holiday_week_time_models;
DROP FUNCTION IF EXISTS enrollment.holiday_time_model_keeps_booked_history();
DROP FUNCTION IF EXISTS enrollment.holiday_host_own_bookings(DATE, DATE);`

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
