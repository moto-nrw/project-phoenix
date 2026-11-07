package migrations

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

const holidayCareTimeHistoryVersion = "1.15.806"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayCareTimeHistoryVersion,
		Description: "Ferien: Zeitänderungen gelten ab dem Änderungstag, vergangenes Soll bleibt erhalten (#6087)",
		DependsOn:   []string{holidayHostOwnBookingsVersion},
	})
	Migrations.MustRegister(holidayCareTimeHistoryUp, holidayCareTimeHistoryDown)
}

// History belongs to the time model, not to a child. Each Berlin change day
// keeps the times BEFORE its first edit. The current columns hold the latest
// times, including further edits on that day. Existing organization RLS and
// period retention/erasure cover the added column; no new personal table.
const holidayCareTimeHistorySQL = `
ALTER TABLE enrollment.holiday_week_time_models
 ADD COLUMN care_time_history JSONB NOT NULL DEFAULT '{}'::jsonb
 CHECK (jsonb_typeof(care_time_history) = 'object');

-- Existing guest copies may be older than their already edited model. Keep
-- their stored past baseline at cutover; reconciliation never overwrites it.
ALTER TABLE enrollment.holiday_guests ADD COLUMN care_time_baseline JSONB
 CHECK (care_time_baseline IS NULL OR jsonb_typeof(care_time_baseline) = 'object');
ALTER TABLE enrollment.holiday_guests ADD COLUMN retained_care_windows JSONB
 CHECK (retained_care_windows IS NULL OR jsonb_typeof(retained_care_windows) = 'array');
UPDATE enrollment.holiday_guests SET care_time_baseline = jsonb_build_object(
 'valid_until', ?::date, 'starts_on', starts_on, 'ends_on', ends_on,
 'starts_at', time_model_starts_at, 'ends_at', time_model_ends_at);

CREATE FUNCTION enrollment.holiday_time_model_windows(p_history JSONB, p_start TIME, p_end TIME)
 RETURNS TABLE (valid_from DATE, valid_until DATE, starts_at TIME, ends_at TIME)
 LANGUAGE sql IMMUTABLE
 SET search_path = pg_catalog, pg_temp
 AS $$
 WITH history AS (
  SELECT key::date AS until_day, (value->>'starts_at')::time AS starts_at, (value->>'ends_at')::time AS ends_at
  FROM jsonb_each(p_history)
 )
 SELECT lag(until_day) OVER (ORDER BY until_day), until_day, starts_at, ends_at FROM history
 UNION ALL
 SELECT max(until_day), NULL::date, p_start, p_end FROM history
 $$;
REVOKE ALL ON FUNCTION enrollment.holiday_time_model_windows(JSONB, TIME, TIME) FROM PUBLIC;
`

// The same booking/rebooking/cancellation segments serve own children and
// guests. Only the hosting school in app.current_tenant_id may read them;
// wrappers expose either own identities or already granted guest student IDs.
const holidayHostBookedWindowsSQL = `
CREATE FUNCTION enrollment.holiday_host_booked_windows_for_retention(p_from DATE, p_to DATE, p_school BIGINT, p_registration BIGINT)
	RETURNS TABLE (organization_id BIGINT, child_id BIGINT, week_id BIGINT, home_school_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE, starts_on DATE, ends_on DATE,
		starts_at TIME, ends_at TIME)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
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
	$$;
REVOKE ALL ON FUNCTION enrollment.holiday_host_booked_windows_for_retention(DATE, DATE, BIGINT, BIGINT) FROM PUBLIC;

-- Only the tenant-bound wrapper is used by school projections. The explicit
-- school/registration helper is private to the deletion trigger below.
CREATE FUNCTION enrollment.holiday_host_booked_windows(p_from DATE, p_to DATE)
 RETURNS TABLE (organization_id BIGINT, child_id BIGINT, week_id BIGINT, home_school_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE, starts_on DATE, ends_on DATE, starts_at TIME, ends_at TIME)
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp
 AS $$ SELECT * FROM enrollment.holiday_host_booked_windows_for_retention(p_from, p_to,
  NULLIF(current_setting('app.current_tenant_id', true), '')::bigint, NULL) $$;
REVOKE ALL ON FUNCTION enrollment.holiday_host_booked_windows(DATE, DATE) FROM PUBLIC;

-- A school guest follows the school's child retention, not the source
-- registration's 92 days. Keep only the resolved care intervals, no source
-- identity, contact, health data or rebooking audit. The copy goes with the
-- existing guest row/student, without extending either retention period.
CREATE FUNCTION enrollment.holiday_guest_keeps_care_windows() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp
 AS $$
 BEGIN
  UPDATE enrollment.holiday_guests AS guest SET retained_care_windows = (
   SELECT COALESCE(jsonb_agg(jsonb_build_object(
    'starts_on', GREATEST(booked.starts_on, guest.starts_on, (guest.care_time_baseline->>'valid_until')::date),
    'ends_on', LEAST(booked.ends_on, guest.ends_on, guest.ended_on - 1),
    'starts_at', booked.starts_at, 'ends_at', booked.ends_at)), '[]'::jsonb)
   FROM enrollment.holiday_host_booked_windows_for_retention(guest.starts_on, guest.ends_on, guest.tenant_id, OLD.id) AS booked
   WHERE booked.organization_id = guest.organization_id
    AND booked.child_id = guest.holiday_registration_child_id AND booked.week_id = guest.holiday_period_week_id
    AND GREATEST(booked.starts_on, guest.starts_on, (guest.care_time_baseline->>'valid_until')::date)
     <= LEAST(booked.ends_on, guest.ends_on, guest.ended_on - 1))
  WHERE guest.organization_id = OLD.organization_id AND EXISTS (
   SELECT 1 FROM enrollment.holiday_registration_children AS child
   WHERE child.organization_id = OLD.organization_id AND child.holiday_registration_id = OLD.id
    AND child.id = guest.holiday_registration_child_id);
  RETURN OLD;
 END $$;
REVOKE ALL ON FUNCTION enrollment.holiday_guest_keeps_care_windows() FROM PUBLIC;
CREATE TRIGGER holiday_guest_keeps_care_windows BEFORE DELETE ON enrollment.holiday_registrations
 FOR EACH ROW EXECUTE FUNCTION enrollment.holiday_guest_keeps_care_windows();

CREATE OR REPLACE FUNCTION enrollment.holiday_host_own_bookings(p_from DATE, p_to DATE)
 RETURNS TABLE (first_name TEXT, last_name TEXT, birth_date DATE, starts_on DATE, ends_on DATE, starts_at TIME, ends_at TIME)
 LANGUAGE sql STABLE SECURITY DEFINER
 SET search_path = pg_catalog, pg_temp
 AS $$
 SELECT first_name, last_name, birth_date, starts_on, ends_on, starts_at, ends_at
 FROM enrollment.holiday_host_booked_windows(p_from, p_to) AS booked
 WHERE home_school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND EXISTS (SELECT 1 FROM enrollment.holiday_period_weeks AS week
   WHERE week.organization_id = booked.organization_id AND week.id = booked.week_id
    AND week.host_school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
 $$;

CREATE FUNCTION enrollment.holiday_host_guest_bookings(p_from DATE, p_to DATE)
 RETURNS TABLE (student_id BIGINT, starts_on DATE, ends_on DATE, starts_at TIME, ends_at TIME)
 LANGUAGE sql STABLE SECURITY DEFINER
 SET search_path = pg_catalog, pg_temp
 AS $$
 SELECT guest.student_id, GREATEST(booked.starts_on, guest.starts_on, (guest.care_time_baseline->>'valid_until')::date),
  LEAST(booked.ends_on, guest.ends_on, guest.ended_on - 1), booked.starts_at, booked.ends_at
 FROM enrollment.holiday_host_booked_windows(p_from, p_to) AS booked
 JOIN enrollment.holiday_guests AS guest ON guest.organization_id = booked.organization_id
  AND guest.holiday_registration_child_id = booked.child_id AND guest.holiday_period_week_id = booked.week_id
 WHERE guest.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND GREATEST(booked.starts_on, guest.starts_on, (guest.care_time_baseline->>'valid_until')::date) <= LEAST(booked.ends_on, guest.ends_on, guest.ended_on - 1)
 UNION ALL
 -- Copies without a surviving registration (legacy/imported guests) retain
 -- their existing tenant-scoped care window. Never overlay a real booking.
 SELECT guest.student_id, GREATEST(guest.starts_on, (guest.care_time_baseline->>'valid_until')::date), LEAST(guest.ends_on, guest.ended_on - 1),
  guest.time_model_starts_at::time, guest.time_model_ends_at::time
 FROM enrollment.holiday_guests AS guest
 WHERE guest.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND GREATEST(guest.starts_on, (guest.care_time_baseline->>'valid_until')::date) <= p_to AND guest.ends_on >= p_from
  AND guest.retained_care_windows IS NULL
  AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS child
   WHERE child.organization_id = guest.organization_id AND child.id = guest.holiday_registration_child_id)
 UNION ALL
 SELECT guest.student_id, saved.starts_on, saved.ends_on, saved.starts_at, saved.ends_at
 FROM enrollment.holiday_guests AS guest
 CROSS JOIN LATERAL jsonb_to_recordset(guest.retained_care_windows)
  AS saved(starts_on DATE, ends_on DATE, starts_at TIME, ends_at TIME)
 WHERE guest.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND saved.starts_on <= p_to AND saved.ends_on >= p_from
  AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS child
   WHERE child.organization_id = guest.organization_id AND child.id = guest.holiday_registration_child_id)
 UNION ALL
 -- Cutover snapshots keep exactly the past care the school could read
 -- before deployment, independently of later model edits or reconciliation.
 SELECT guest.student_id, (guest.care_time_baseline->>'starts_on')::date,
  LEAST((guest.care_time_baseline->>'ends_on')::date,
   (guest.care_time_baseline->>'valid_until')::date - 1, guest.ended_on - 1),
  (guest.care_time_baseline->>'starts_at')::time, (guest.care_time_baseline->>'ends_at')::time
 FROM enrollment.holiday_guests AS guest
 WHERE guest.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND guest.care_time_baseline IS NOT NULL
  AND (guest.care_time_baseline->>'starts_on')::date <= p_to
  AND LEAST((guest.care_time_baseline->>'ends_on')::date,
   (guest.care_time_baseline->>'valid_until')::date - 1, guest.ended_on - 1) >= p_from
  AND (guest.care_time_baseline->>'starts_on')::date <= LEAST((guest.care_time_baseline->>'ends_on')::date,
   (guest.care_time_baseline->>'valid_until')::date - 1, guest.ended_on - 1)
 $$;
REVOKE ALL ON FUNCTION enrollment.holiday_host_guest_bookings(DATE, DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_guest_bookings(DATE, DATE) TO phoenix_tenant;
`

func holidayCareTimeHistoryUp(ctx context.Context, db *bun.DB) error {
	return holidayCareTimeHistoryUpOn(ctx, db, calendar.TodayDate())
}

func holidayCareTimeHistoryUpOn(ctx context.Context, db *bun.DB, cutover calendar.Date) error {
	if _, err := db.ExecContext(ctx, holidayCareTimeHistorySQL+holidayHostBookedWindowsSQL, cutover); err != nil {
		return fmt.Errorf("add effective holiday care times: %w", err)
	}
	return nil
}

// Rollback restores the prior projection before removing the new inputs.
const holidayOwnBookingsRestoreSQL = `
CREATE OR REPLACE FUNCTION enrollment.holiday_host_own_bookings(p_from DATE, p_to DATE)
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
`

const holidayCareTimeHistoryDownSQL = `
 DROP TRIGGER holiday_guest_keeps_care_windows ON enrollment.holiday_registrations;
 DROP FUNCTION enrollment.holiday_guest_keeps_care_windows();
 DROP FUNCTION enrollment.holiday_host_guest_bookings(DATE, DATE);
 DROP FUNCTION enrollment.holiday_host_booked_windows(DATE, DATE);
 DROP FUNCTION enrollment.holiday_host_booked_windows_for_retention(DATE, DATE, BIGINT, BIGINT);
 DROP FUNCTION enrollment.holiday_time_model_windows(JSONB, TIME, TIME);
 ALTER TABLE enrollment.holiday_week_time_models DROP COLUMN care_time_history;
 ALTER TABLE enrollment.holiday_guests DROP COLUMN care_time_baseline, DROP COLUMN retained_care_windows;`

func holidayCareTimeHistoryDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayOwnBookingsRestoreSQL+holidayCareTimeHistoryDownSQL); err != nil {
		return fmt.Errorf("remove effective holiday care times: %w", err)
	}
	return nil
}
