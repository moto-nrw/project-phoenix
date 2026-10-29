package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHostSharedAnswersVersion = "1.15.712"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHostSharedAnswersVersion,
		Description: "Ausrichtende OGS sieht freigegebene Antworten und die zweite sorgeberechtigte Person ihrer Ferienkinder (#5202)",
		DependsOn: []string{
			holidayFormDocumentsVersion,  // the last migration before this one (#5199)
			holidayPeriodFormsVersion,    // the form versions and answers (#5198)
			holidaySecondGuardianVersion, // the second guardian of a registration (#5197)
			holidayGuestsVersion,         // the guest copy and enrollment.holiday_host_guest_contacts
		},
	})
	Migrations.MustRegister(holidayHostSharedAnswersUp, holidayHostSharedAnswersDown)
}

// holidayHostSharedAnswersUpSQL gives the school that hosts a holiday week
// the answers without health information the Träger shares with it, and the
// second guardian (#5202, user decision of 28.10.2026 for option B, ADR 0116
// and ADR 0120 addenda). It replaces the two narrow functions of ADR 0116 and
// ADR 0120 and keeps every other rule of them:
//
//   - enrollment.holiday_host_participants adds per child the name and phone
//     of the second guardian and the answers to the shared questions of the
//     form version the child's registration was submitted with.
//   - enrollment.holiday_host_guest_contacts adds the second guardian as an
//     emergency contact "Eltern" after the registering parent.
//   - enrollment.holiday_guests.answers carries the shared answers of the
//     copy; they end with its contacts.
//
// A question leaves the Träger only when all of this holds, so a missing or
// unclear marker keeps it with the Träger:
//
//  1. its version is the one the registration keeps, and that version marks
//     it share_with_host = true;
//  2. that version marks it health = false, as a JSON boolean;
//  3. no version of the period marks a question with the same key anything
//     but health = false: a key that was ever a health question, or whose
//     marker is missing, never leaves;
//  4. it is a question type with an answer (no information block).
//
// A boolean answer leaves as a boolean, every other answer as a string; a
// selection leaves as the label of the chosen option. An answer of any other
// JSON type leaves as null. The functions select no e-mail address, no
// allergy, medication or health note, no consent and no other column; the
// access ends after the last day of the week as before
// (enrollment.holiday_host_week). Existing form versions carry no
// share_with_host, so nothing they collected leaves.
const holidayHostSharedAnswersUpSQL = `
DROP FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT);
CREATE FUNCTION enrollment.holiday_host_participants(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (
		child_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE,
		home_school_id BIGINT, guest_school_name TEXT, time_model_id BIGINT,
		second_guardian_name TEXT, second_guardian_phone TEXT, answers JSONB
	)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
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
		), '[]'::jsonb)
	FROM hosted
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	WHERE registration.status = 'confirmed'
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) IS
	'Minimum fields, second guardian (name, phone) and shared answers without health information of the confirmed children of a week the school of app.current_tenant_id hosts (#4665, #5202, ADR 0116); no e-mail address, no health information.';
REVOKE ALL ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) TO phoenix_tenant;

DROP FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT);
CREATE FUNCTION enrollment.holiday_host_guest_contacts(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (child_id BIGINT, kind TEXT, "position" SMALLINT, name TEXT, phone TEXT, relationship TEXT)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
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
			AND registration.status = 'confirmed'
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
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) IS
	'Emergency and pickup contacts of the confirmed holiday guests of a week the school of app.current_tenant_id hosts, the second guardian included (#4666, #5202, ADR 0120); none for its own children, no e-mail address.';
REVOKE ALL ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) TO phoenix_tenant;

ALTER TABLE enrollment.holiday_guests
	ADD COLUMN answers JSONB NOT NULL DEFAULT '[]'::jsonb,
	ADD CONSTRAINT chk_holiday_guests_answers CHECK (jsonb_typeof(answers) = 'array');
COMMENT ON COLUMN enrollment.holiday_guests.answers IS
	'Answers without health information the Träger shares with the hosting school (#5202, ADR 0120); deleted with the contacts.';
`

// holidayHostSharedAnswersDownSQL restores the functions of #4665 and #4666
// with their signatures and drops the copied answers.
const holidayHostSharedAnswersDownSQL = `
ALTER TABLE enrollment.holiday_guests
	DROP CONSTRAINT IF EXISTS chk_holiday_guests_answers,
	DROP COLUMN IF EXISTS answers;

DROP FUNCTION IF EXISTS enrollment.holiday_host_guest_contacts(BIGINT, BIGINT);
CREATE FUNCTION enrollment.holiday_host_guest_contacts(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (child_id BIGINT, kind TEXT, "position" SMALLINT, name TEXT, phone TEXT, relationship TEXT)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	WITH guest AS (
		SELECT child.id AS child_id, registration.id AS registration_id, registration.guardian_first_name,
			registration.guardian_last_name, registration.guardian_phone
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
			AND registration.status = 'confirmed'
			AND child.home_school_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND enrollment.holiday_host_week(p_organization_id, p_week_id)
	)
	SELECT guest.child_id, 'emergency'::text, 0::smallint,
		guest.guardian_first_name || ' ' || guest.guardian_last_name, guest.guardian_phone, 'Eltern'::text
	FROM guest
	UNION ALL
	SELECT guest.child_id, contact.kind::text, contact.position, contact.name, contact.phone, contact.relationship
	FROM guest
	JOIN enrollment.holiday_registration_contacts AS contact
		ON contact.organization_id = p_organization_id AND contact.holiday_registration_id = guest.registration_id
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) IS
	'Emergency and pickup contacts of the confirmed holiday guests of a week the school of app.current_tenant_id hosts (#4666, ADR 0120); none for its own children.';
REVOKE ALL ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) TO phoenix_tenant;

DROP FUNCTION IF EXISTS enrollment.holiday_host_participants(BIGINT, BIGINT);
CREATE FUNCTION enrollment.holiday_host_participants(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (
		child_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE,
		home_school_id BIGINT, guest_school_name TEXT, time_model_id BIGINT
	)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT child.id, child.first_name, child.last_name, child.birth_date,
		child.home_school_id, child.guest_school_name, booking.holiday_week_time_model_id
	FROM enrollment.holiday_registration_bookings AS booking
	JOIN enrollment.holiday_registration_children AS child
		ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	WHERE booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
		AND registration.status = 'confirmed'
		AND enrollment.holiday_host_week(p_organization_id, p_week_id)
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) IS
	'Minimum fields of the confirmed children of a week the school of app.current_tenant_id hosts (#4665, ADR 0116); no health information, no contacts.';
REVOKE ALL ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) TO phoenix_tenant;
`

func holidayHostSharedAnswersUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayHostSharedAnswersUpSQL); err != nil {
			return fmt.Errorf("add holiday host shared answers: %w", err)
		}
		return nil
	})
}

func holidayHostSharedAnswersDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayHostSharedAnswersDownSQL); err != nil {
			return fmt.Errorf("drop holiday host shared answers: %w", err)
		}
		return nil
	})
}
