package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayWeekCapacityVersion = "1.15.868"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayWeekCapacityVersion,
		Description: "Ferien: Kapazität je Woche, Platz oder Warteliste je Buchung, Teilnehmerliste nur mit Platz, Nachrück-Mail (#4699)",
		DependsOn: []string{
			guardianInvitationAddressBindingVersion, // the newest migration on development
			demoVisitorErasureOutboxKindsVersion,    // the outbox CHECK and erasure this one extends
		},
	})
	Migrations.MustRegister(holidayWeekCapacityUp, holidayWeekCapacityDown)
}

// holidayWeekCapacityUpSQL gives a holiday week its places and every booking
// its placement (#4699). capacity counts the places of the week across its
// time models; NULL is unlimited. A booking is "placed" or "waitlisted". The
// registrations that count are received and confirmed ones: the place is
// given when the parents send the form, in the order the registrations came
// in. Every booking that exists before this migration keeps its place.
//
// The application inserts a booking as "waitlisted" and decides on its place
// only while it holds the week's row with SELECT ... FOR UPDATE, so two
// registrations at the same time cannot both take the last place. The default
// "placed" serves the rows written before (and test fixtures); the
// application always names the placement.
//
// promoted_at is when a booking moved up from the waiting list.
const holidayWeekCapacityUpSQL = `
ALTER TABLE enrollment.holiday_period_weeks ADD COLUMN IF NOT EXISTS capacity INTEGER;
ALTER TABLE enrollment.holiday_period_weeks DROP CONSTRAINT IF EXISTS chk_holiday_period_weeks_capacity;
ALTER TABLE enrollment.holiday_period_weeks ADD CONSTRAINT chk_holiday_period_weeks_capacity
	CHECK (capacity IS NULL OR capacity BETWEEN 0 AND 10000);
COMMENT ON COLUMN enrollment.holiday_period_weeks.capacity IS
	'Places of the week across its time models, NULL for unlimited (#4699). Placement decisions hold this row FOR UPDATE.';

ALTER TABLE enrollment.holiday_registration_bookings ADD COLUMN IF NOT EXISTS placement TEXT NOT NULL DEFAULT 'placed';
ALTER TABLE enrollment.holiday_registration_bookings ADD COLUMN IF NOT EXISTS promoted_at TIMESTAMPTZ;
ALTER TABLE enrollment.holiday_registration_bookings DROP CONSTRAINT IF EXISTS chk_holiday_registration_bookings_placement;
ALTER TABLE enrollment.holiday_registration_bookings ADD CONSTRAINT chk_holiday_registration_bookings_placement
	CHECK (placement IN ('placed', 'waitlisted'));
CREATE INDEX IF NOT EXISTS idx_holiday_registration_bookings_week_placement
	ON enrollment.holiday_registration_bookings (organization_id, holiday_period_week_id, placement);
COMMENT ON COLUMN enrollment.holiday_registration_bookings.placement IS
	'placed: the child has a place in the week; waitlisted: it waits for one (#4699). Only placed bookings reach the host school and the contribution.';
COMMENT ON COLUMN enrollment.holiday_registration_bookings.promoted_at IS
	'When the booking moved up from the waiting list to a place (#4699).';

-- The audit trail records a booking that moved up ('promote', of its
-- registration), a changed capacity and a read of the places of a period
-- ('capacity' and 'places', of the period like a list).
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action,
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_subject;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number', 'edit',
		'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel',
		'promote', 'capacity', 'places')),
	ADD CONSTRAINT chk_holiday_registration_audit_subject CHECK (
		(action IN ('list', 'export', 'capacity', 'places')) = (holiday_registration_id IS NULL));

-- A base line of a frozen contribution names the week it prices by its id,
-- so a booking is priced at most once even when the Büro later moves the
-- dates of its week (#4699). Lines frozen before take the id of the one
-- week of their period with their dates; a line whose week has moved since,
-- or whose dates several parallel weeks share, keeps NULL and is matched by
-- its dates as before.
ALTER TABLE parent_billing.holiday_contribution_lines ADD COLUMN IF NOT EXISTS holiday_period_week_id BIGINT;
COMMENT ON COLUMN parent_billing.holiday_contribution_lines.holiday_period_week_id IS
	'The week a base line prices (#4699), without a foreign key: the line keeps its price when the week goes.';
`

const holidayWeekCapacityDownSQL = `
ALTER TABLE parent_billing.holiday_contribution_lines DROP COLUMN IF EXISTS holiday_period_week_id;
DELETE FROM enrollment.holiday_registration_audit WHERE action IN ('promote', 'capacity', 'places');
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action,
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_subject;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number', 'edit',
		'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel')),
	ADD CONSTRAINT chk_holiday_registration_audit_subject CHECK (
		(action IN ('list', 'export')) = (holiday_registration_id IS NULL));
DROP INDEX IF EXISTS enrollment.idx_holiday_registration_bookings_week_placement;
ALTER TABLE enrollment.holiday_registration_bookings DROP CONSTRAINT IF EXISTS chk_holiday_registration_bookings_placement;
ALTER TABLE enrollment.holiday_registration_bookings DROP COLUMN IF EXISTS promoted_at;
ALTER TABLE enrollment.holiday_registration_bookings DROP COLUMN IF EXISTS placement;
ALTER TABLE enrollment.holiday_period_weeks DROP CONSTRAINT IF EXISTS chk_holiday_period_weeks_capacity;
ALTER TABLE enrollment.holiday_period_weeks DROP COLUMN IF EXISTS capacity;
`

// The host school reads only placed bookings (#4699, ADR 0116): its booking
// check, its participant list, the contacts of its guests and the care
// windows of its guests. The bodies are those of migrations 1.15.635,
// 1.15.712, 1.15.725 and 1.15.806 with one more condition; the down
// migration writes them back without it. A rebooking keeps a waitlisted
// booking it replaced out of the care windows too.
const (
	holidayPlacedBooking  = "\n\t\t\tAND booking.placement = 'placed'"
	holidayPlacedPrevious = "\n\t\tWHERE COALESCE((previous->>'waitlisted')::boolean, false) = false"
)

const (
	holidayHostSQL0 = `CREATE OR REPLACE FUNCTION enrollment.holiday_host_booking(p_organization_id bigint, p_week_id bigint, p_child_id bigint)
 RETURNS boolean
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'pg_temp'
AS $function$
	SELECT enrollment.holiday_host_week(p_organization_id, p_week_id) AND EXISTS (
		SELECT 1
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
			AND booking.holiday_registration_child_id = p_child_id
			AND registration.status = 'confirmed'`
	holidayHostSQL1 = `
	)
	$function$;

CREATE OR REPLACE FUNCTION enrollment.holiday_host_participants(p_organization_id bigint, p_week_id bigint)
 RETURNS TABLE(child_id bigint, first_name text, last_name text, birth_date date, home_school_id bigint, guest_school_name text, time_model_id bigint, second_guardian_name text, second_guardian_phone text, answers jsonb, registration_id bigint, child_position smallint, health_data bytea, family_health_data bytea, health_fields jsonb)
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'pg_temp'
AS $function$
	WITH hosted AS (
		SELECT week.holiday_period_id
		FROM enrollment.holiday_period_weeks AS week
		WHERE week.organization_id = p_organization_id AND week.id = p_week_id
			AND enrollment.holiday_host_week(p_organization_id, p_week_id)
	),
	period_field AS (
		SELECT form.version, field.definition, field.ordinality
		FROM hosted
		JOIN enrollment.holiday_period_forms AS form
			ON form.organization_id = p_organization_id AND form.holiday_period_id = hosted.holiday_period_id
		CROSS JOIN LATERAL jsonb_array_elements(form.fields) WITH ORDINALITY AS field(definition, ordinality)
	),
	shared_field AS (
		SELECT candidate.version, candidate.definition, candidate.ordinality
		FROM period_field AS candidate
		WHERE candidate.definition->'share_with_host' = 'true'::jsonb
			AND candidate.definition->'health' = 'false'::jsonb
			AND candidate.definition->>'type' IN ('boolean', 'text', 'textarea', 'date', 'select')
			AND NOT EXISTS (
				SELECT 1 FROM period_field AS other
				WHERE other.definition->>'key' = candidate.definition->>'key'
					AND other.definition->'health' IS DISTINCT FROM 'false'::jsonb)
	),
	health_field AS (
		SELECT candidate.version, candidate.definition, candidate.ordinality
		FROM period_field AS candidate
		WHERE candidate.definition->'share_with_host' = 'true'::jsonb
			AND candidate.definition->'health' IS DISTINCT FROM 'false'::jsonb
			AND candidate.definition->>'type' IN ('boolean', 'text', 'textarea', 'date', 'select')
	)
	SELECT child.id, child.first_name, child.last_name, child.birth_date,
		child.home_school_id, child.guest_school_name, booking.holiday_week_time_model_id,
		CASE WHEN registration.second_guardian_first_name IS NOT NULL
			THEN registration.second_guardian_first_name || ' ' || registration.second_guardian_last_name END,
		CASE WHEN registration.second_guardian_first_name IS NOT NULL
			THEN COALESCE(registration.second_guardian_phone, '') END,
		COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
					'key', shared.definition->>'key',
					'label', shared.definition->>'label',
					'type', shared.definition->>'type',
					'value', CASE
						WHEN shared.definition->>'type' = 'boolean' THEN
							CASE WHEN jsonb_typeof(given.answer) = 'boolean' THEN given.answer END
						WHEN jsonb_typeof(given.answer) IS DISTINCT FROM 'string' THEN NULL
						WHEN shared.definition->>'type' = 'select' THEN to_jsonb(COALESCE((
							SELECT choice->>'label'
							FROM jsonb_array_elements(CASE WHEN jsonb_typeof(shared.definition->'options') = 'array'
								THEN shared.definition->'options' ELSE '[]'::jsonb END) AS choice
							WHERE choice->>'value' = given.answer #>> '{}'
							LIMIT 1), given.answer #>> '{}'))
						ELSE given.answer
					END)
				ORDER BY shared.ordinality)
			FROM shared_field AS shared
			CROSS JOIN LATERAL (
				SELECT (CASE WHEN shared.definition->'applies_to_child' = 'true'::jsonb
					THEN child.custom_data ELSE registration.custom_data END) -> (shared.definition->>'key') AS answer
			) AS given
			WHERE shared.version = registration.form_version
		), '[]'::jsonb),
		registration.id, child.position,
		CASE WHEN consented.health THEN child.health_data END,
		CASE WHEN consented.health THEN registration.health_data END,
		CASE WHEN consented.health THEN COALESCE((
			SELECT jsonb_agg(health.definition ORDER BY health.ordinality)
			FROM health_field AS health
			WHERE health.version = registration.form_version
		), '[]'::jsonb) END
	FROM hosted
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	CROSS JOIN LATERAL (
		SELECT registration.consent_version >= 2 AND registration.health_consent_at IS NOT NULL AS health
	) AS consented
	WHERE registration.status = 'confirmed'`
	holidayHostSQL2 = `
	$function$;

CREATE OR REPLACE FUNCTION enrollment.holiday_host_guest_contacts(p_organization_id bigint, p_week_id bigint)
 RETURNS TABLE(child_id bigint, kind text, "position" smallint, name text, phone text, relationship text)
 LANGUAGE sql
 STABLE SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'pg_temp'
AS $function$
	WITH guest AS (
		SELECT child.id AS child_id, registration.id AS registration_id, registration.guardian_first_name,
			registration.guardian_last_name, registration.guardian_phone, registration.second_guardian_first_name,
			registration.second_guardian_last_name, registration.second_guardian_phone
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
			AND registration.status = 'confirmed'`
	holidayHostSQL3 = `
			AND child.home_school_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND enrollment.holiday_host_week(p_organization_id, p_week_id)
	)
	SELECT guest.child_id, 'emergency'::text, 0::smallint,
		guest.guardian_first_name || ' ' || guest.guardian_last_name, guest.guardian_phone, 'Eltern'::text
	FROM guest
	UNION ALL
	SELECT guest.child_id, 'emergency'::text, 1::smallint,
		guest.second_guardian_first_name || ' ' || guest.second_guardian_last_name,
		COALESCE(guest.second_guardian_phone, ''), 'Eltern'::text
	FROM guest
	WHERE guest.second_guardian_first_name IS NOT NULL
	UNION ALL
	SELECT guest.child_id, contact.kind::text, (contact.position + 1)::smallint, contact.name, contact.phone, contact.relationship
	FROM guest
	JOIN enrollment.holiday_registration_contacts AS contact
		ON contact.organization_id = p_organization_id AND contact.holiday_registration_id = guest.registration_id
	$function$;

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
			`
	holidayCurrentStart = `(SELECT max(rebook.on_date) FROM rebook WHERE rebook.child_id = own.child_id)`
	holidayHostSQL3End  = ` AS valid_from,
			NULL::date AS valid_until
		FROM own
		JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = own.organization_id AND booking.holiday_registration_child_id = own.child_id`
	holidayHostSQL4 = `
		UNION ALL
		SELECT rebook.child_id, (previous->>'week_id')::bigint, (previous->>'time_model_id')::bigint,
			`
	holidayPreviousStart = `(SELECT max(earlier.on_date) FROM rebook AS earlier
				WHERE earlier.child_id = rebook.child_id AND earlier.occurred_at < rebook.occurred_at)`
	holidayHostSQL4End = `,
			rebook.on_date - 1
		FROM rebook
		CROSS JOIN LATERAL jsonb_array_elements(COALESCE(rebook.details->'before', '[]'::jsonb)) AS previous`
	holidayHostSQL5 = `
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
)

// holidayHostPlacedSQL are the functions with the placement condition,
// holidayHostUnplacedSQL those of before (the down migration).
const (
	holidayHostPlacedSQL = holidayHostSQL0 +
		holidayPlacedBooking +
		holidayHostSQL1 +
		holidayPlacedBooking +
		holidayHostSQL2 +
		holidayPlacedBooking +
		holidayHostSQL3 +
		"GREATEST(" + holidayCurrentStart + ", (booking.promoted_at AT TIME ZONE 'Europe/Berlin')::date)" + holidayHostSQL3End +
		holidayPlacedBooking +
		holidayHostSQL4 +
		"GREATEST(" + holidayPreviousStart + ", ((previous->>'promoted_at')::timestamptz AT TIME ZONE 'Europe/Berlin')::date)" + holidayHostSQL4End +
		holidayPlacedPrevious +
		holidayHostSQL5
	holidayHostUnplacedSQL = holidayHostSQL0 +
		holidayHostSQL1 +
		holidayHostSQL2 +
		holidayHostSQL3 + holidayCurrentStart + holidayHostSQL3End +
		holidayHostSQL4 + holidayPreviousStart + holidayHostSQL4End +
		holidayHostSQL5
)

// holidayContributionLineWeeksSQL gives the base lines frozen before the
// id of their week, where one week of their period has their dates.
const holidayContributionLineWeeksSQL = `
UPDATE parent_billing.holiday_contribution_lines AS line
SET holiday_period_week_id = week.id
FROM parent_billing.holiday_contributions AS contribution
JOIN enrollment.holiday_period_weeks AS week
	ON week.organization_id = contribution.organization_id AND week.holiday_period_id = contribution.holiday_period_id
WHERE line.kind = 'base' AND line.holiday_period_week_id IS NULL
	AND contribution.organization_id = line.organization_id AND contribution.id = line.holiday_contribution_id
	AND week.starts_on = line.week_starts_on AND week.ends_on = line.week_ends_on
	AND NOT EXISTS (
		SELECT 1 FROM enrollment.holiday_period_weeks AS parallel
		WHERE parallel.organization_id = week.organization_id AND parallel.holiday_period_id = week.holiday_period_id
			AND parallel.id <> week.id AND parallel.starts_on = week.starts_on AND parallel.ends_on = week.ends_on);
`

// The parents of a registration get a mail when a booking moves up from the
// waiting list (#4699). It is a mail of the Träger, so the CHECK of 1.15.843
// declares its kind and the demo erasure deletes it with the registration it
// names, like the receipt.
var holidayPlaceEmailOutboxTraegerKinds = []string{"care_contract", "care_contract_notice", "holiday_place_granted", "holiday_registration_received", "parent_billing_letter"}

const holidayPlaceEmailOutboxTraegerKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_place_granted', 'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// A rollback deletes the mails the CHECK of 1.15.843 would refuse.
const deleteHolidayPlaceMailsSQL = `DELETE FROM platform.email_outbox
	WHERE organization_id IS NOT NULL AND kind = 'holiday_place_granted'`

// holidayPlaceErasureSQL is the erasure of 1.15.843 whose statement for the
// receipt also deletes the mail of a booking that moved up: both name the
// registration.
// The erasure itself is assembled from the constants of 1.15.843 in
// demo_visitor_erasure_of_outbox_kinds.go (installHolidayPlaceErasure).

const validateHolidayPlaceEmailOutboxTraegerKindSQL = `ALTER TABLE platform.email_outbox VALIDATE CONSTRAINT chk_email_outbox_traeger_kind`

func holidayWeekCapacityUp(ctx context.Context, db *bun.DB) error {
	if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayWeekCapacityUpSQL); err != nil {
			return fmt.Errorf("add holiday week capacity and booking placement: %w", err)
		}
		if _, err := tx.ExecContext(ctx, holidayHostPlacedSQL); err != nil {
			return fmt.Errorf("let the host school read only placed holiday bookings: %w", err)
		}
		if _, err := tx.ExecContext(ctx, holidayContributionLineWeeksSQL); err != nil {
			return fmt.Errorf("name the week of frozen holiday contribution lines: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	// As in 1.15.843: the ADD holds the table only for the catalog change,
	// the VALIDATE scans it in a statement of its own.
	if _, err := db.ExecContext(ctx, holidayPlaceEmailOutboxTraegerKindCheckSQL); err != nil {
		return fmt.Errorf("declare the holiday place mail of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayPlaceEmailOutboxTraegerKindSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	return installHolidayPlaceErasure(ctx, db)
}

func holidayWeekCapacityDown(ctx context.Context, db *bun.DB) error {
	if err := installOutboxKindErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, deleteHolidayPlaceMailsSQL); err != nil {
		return fmt.Errorf("delete the holiday place mails: %w", err)
	}
	if err := restoreEmailOutboxTraegerKinds(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayHostUnplacedSQL); err != nil {
			return fmt.Errorf("restore the host school's holiday booking reads: %w", err)
		}
		if _, err := tx.ExecContext(ctx, holidayWeekCapacityDownSQL); err != nil {
			return fmt.Errorf("drop holiday week capacity and booking placement: %w", err)
		}
		return nil
	})
}
