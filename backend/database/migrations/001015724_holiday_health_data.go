package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
)

const holidayHealthDataVersion = "1.15.724"

// HealthDataKeysEnv names the key ring the health information of the holiday
// registrations is sealed with (#4722). The server reads the same variable.
const healthDataKeysEnv = "HEALTH_DATA_KEYS"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHealthDataVersion,
		Description: "Gesundheitsangaben der Ferienanmeldung verschlüsselt speichern und an die ausrichtende OGS weitergeben (#4722)",
		DependsOn: []string{
			"1.15.723",                       // the last migration before this one (message send idempotency)
			holidayRegistrationProofsVersion, // the audit actions this extends
			holidayGuestAnswerFilterVersion,  // the guest copy and its read-time filter
		},
		Precondition: holidayHealthDataPrecondition,
	})
	Migrations.MustRegister(holidayHealthDataUp, holidayHealthDataDown)
}

// The health information of a holiday registration (#4722) leaves the
// plaintext columns:
//
//   - enrollment.holiday_registration_children.health_data seals, per child,
//     allergies, medication, further health notes and the answers to the
//     child questions its form version marks as health information (a
//     missing or unclear marker counts as health information, as in the
//     hosting school's functions). allergies, medication and health_notes go.
//   - enrollment.holiday_registrations.health_data seals the answers to the
//     family questions marked as health information.
//   - enrollment.holiday_registrations.consent_version is the version of the
//     consent texts the parents agreed to: 2 names the hosting OGS as a
//     reader of the health information; earlier registrations keep NULL and
//     share none.
//   - enrollment.holiday_guests.health_data seals the health information the
//     guest copy carries; it goes with the contacts.
//
// Each value is sealed with AES-256-GCM by Security Runtime's field cipher
// and bound to its table, Träger or school and row, so a value copied into
// another row does not open. The contexts below are the ones Enrollment
// opens with (enrollment.HolidayChildHealthContext and friends).
const holidayHealthDataAddSQL = `
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN health_data BYTEA,
	ADD COLUMN consent_version SMALLINT,
	ADD CONSTRAINT chk_holiday_registrations_health_data CHECK (octet_length(health_data) <= 1048576),
	ADD CONSTRAINT chk_holiday_registrations_consent_version CHECK (consent_version >= 2);
COMMENT ON COLUMN enrollment.holiday_registrations.health_data IS
	'Sealed answers to the family questions marked as health information (#4722); never plaintext.';
COMMENT ON COLUMN enrollment.holiday_registrations.consent_version IS
	'Version of the consent texts (#4722): 2 and later let the hosting OGS see the health information; NULL shares none.';
ALTER TABLE enrollment.holiday_registration_children
	ADD COLUMN health_data BYTEA,
	ADD CONSTRAINT chk_holiday_registration_children_health_data CHECK (octet_length(health_data) <= 1048576);
COMMENT ON COLUMN enrollment.holiday_registration_children.health_data IS
	'Sealed allergies, medication, health notes and answers to the child questions marked as health information (#4722); never plaintext.';
ALTER TABLE enrollment.holiday_guests
	ADD COLUMN health_data BYTEA,
	ADD CONSTRAINT chk_holiday_guests_health_data CHECK (octet_length(health_data) <= 1048576);
COMMENT ON COLUMN enrollment.holiday_guests.health_data IS
	'Sealed health information the Träger shares with the hosting school (#4722, ADR 0120); deleted with the contacts.';
`

// holidayHealthChildrenQuery reads every child with what is to seal: the
// plaintext health fields, the answers and the keys of the child questions
// its version marks as health information.
const holidayHealthChildrenQuery = `SELECT child.id, child.organization_id, child.holiday_registration_id, child.position,
		child.allergies, child.medication, child.health_notes, child.custom_data,
		COALESCE((SELECT jsonb_agg(field.definition->>'key')
			FROM enrollment.holiday_period_forms AS form
			CROSS JOIN LATERAL jsonb_array_elements(form.fields) AS field(definition)
			WHERE form.organization_id = registration.organization_id
				AND form.holiday_period_id = registration.holiday_period_id
				AND form.version = registration.form_version
				AND field.definition->'applies_to_child' = 'true'::jsonb
				AND field.definition->'health' IS DISTINCT FROM 'false'::jsonb), '[]'::jsonb) AS health_keys
	FROM enrollment.holiday_registration_children AS child
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	ORDER BY child.id`

const holidayHealthRegistrationsQuery = `SELECT registration.id, registration.organization_id, registration.custom_data,
		COALESCE((SELECT jsonb_agg(field.definition->>'key')
			FROM enrollment.holiday_period_forms AS form
			CROSS JOIN LATERAL jsonb_array_elements(form.fields) AS field(definition)
			WHERE form.organization_id = registration.organization_id
				AND form.holiday_period_id = registration.holiday_period_id
				AND form.version = registration.form_version
				AND field.definition->'applies_to_child' IS DISTINCT FROM 'true'::jsonb
				AND field.definition->'health' IS DISTINCT FROM 'false'::jsonb), '[]'::jsonb) AS health_keys
	FROM enrollment.holiday_registrations AS registration
	ORDER BY registration.id`

const holidayHealthDataDropPlaintextSQL = `
ALTER TABLE enrollment.holiday_registration_children
	DROP CONSTRAINT chk_holiday_registration_children_texts,
	DROP COLUMN allergies,
	DROP COLUMN medication,
	DROP COLUMN health_notes;
ALTER TABLE enrollment.holiday_registration_children
	ADD CONSTRAINT chk_holiday_registration_children_texts CHECK (char_length(guest_school_name) <= 120);

ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health'));
`

// holidayHostParticipantsHealthSQL gives the hosting school the health
// information of its confirmed children (#4722, user decision of 01.11.2026
// "keine Datensilos", ADR 0116 and ADR 0120 addenda). It keeps every column
// and rule of #5202 and adds per child:
//
//   - registration_id and child_position, which the sealed values are bound
//     to;
//   - health_data and family_health_data, the sealed values as stored: the
//     database never sees them in plaintext, Enrollment opens them in the
//     school's own transaction;
//   - health_fields, the definitions of the questions marked as health
//     information (a missing or unclear marker counts as one) that the
//     registration's version shares with the hosting school, in form order,
//     so Enrollment can name each answer.
//
// All three are NULL unless the parents agreed to the health information and
// to consent texts of version 2 or later, which name the hosting OGS as a
// reader. Earlier registrations share nothing. The access still ends after
// the last day of the week (enrollment.holiday_host_week).
const holidayHostParticipantsHealthSQL = `
DROP FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT);
CREATE FUNCTION enrollment.holiday_host_participants(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (
		child_id BIGINT, first_name TEXT, last_name TEXT, birth_date DATE,
		home_school_id BIGINT, guest_school_name TEXT, time_model_id BIGINT,
		second_guardian_name TEXT, second_guardian_phone TEXT, answers JSONB,
		registration_id BIGINT, child_position SMALLINT,
		health_data BYTEA, family_health_data BYTEA, health_fields JSONB
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
	WHERE registration.status = 'confirmed'
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) IS
	'Minimum fields, second guardian (name, phone), shared answers and, with the consent of version 2, the sealed health information of the confirmed children of a week the school of app.current_tenant_id hosts (#4665, #5202, #4722, ADR 0116); no e-mail address, no plaintext health information.';
REVOKE ALL ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_participants(BIGINT, BIGINT) TO phoenix_tenant;
`

// holidayHealthRecord is the sealed JSON of one child or family. Its field
// names are the ones of enrollment.HolidayHealthRecord.
type holidayHealthRecord struct {
	Allergies   string                     `json:"allergies,omitempty"`
	Medication  string                     `json:"medication,omitempty"`
	HealthNotes string                     `json:"health_notes,omitempty"`
	Answers     map[string]json.RawMessage `json:"answers,omitempty"`
}

func (r holidayHealthRecord) empty() bool {
	return r.Allergies == "" && r.Medication == "" && r.HealthNotes == "" && len(r.Answers) == 0
}

// The contexts the values are bound to; Enrollment opens with the same.
func holidayChildHealthContext(organizationID, registrationID int64, position int) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_registration_children.health_data:%d:%d:%d", organizationID, registrationID, position)
}

func holidayFamilyHealthContext(organizationID, registrationID int64) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_registrations.health_data:%d:%d", organizationID, registrationID)
}

// holidayHealthCipher reads the key ring from the environment.
func holidayHealthCipher() (holidayFieldCipher, error) {
	spec := strings.TrimSpace(os.Getenv(healthDataKeysEnv))
	if spec == "" {
		return nil, fmt.Errorf("%s environment variable is required to seal the health information of the holiday registrations (#4722)", healthDataKeysEnv)
	}
	cipher, err := securityRuntimeCompose.NewFieldCipher(spec)
	if err != nil {
		return nil, err
	}
	return cipher, nil
}

// holidayFieldCipher seals and opens one value bound to its context.
type holidayFieldCipher interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// optionalHolidayHealthCipher reads the key ring when it is set; without it
// the migration runs only where there is nothing to seal or open.
func optionalHolidayHealthCipher() (holidayFieldCipher, error) {
	if strings.TrimSpace(os.Getenv(healthDataKeysEnv)) == "" {
		return nil, nil
	}
	return holidayHealthCipher()
}

// holidayHealthDataPrecondition refuses before the release when there is
// health information to seal and no key to seal it with.
func holidayHealthDataPrecondition(ctx context.Context, db *bun.DB) error {
	var pending bool
	if err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM enrollment.holiday_registration_children
		WHERE allergies <> '' OR medication <> '' OR health_notes <> '' OR custom_data <> '{}'::jsonb)
		OR EXISTS (SELECT 1 FROM enrollment.holiday_registrations WHERE custom_data <> '{}'::jsonb)`).Scan(ctx, &pending); err != nil {
		return fmt.Errorf("check holiday health information to seal: %w", err)
	}
	if !pending {
		return nil
	}
	_, err := holidayHealthCipher()
	return err
}

func holidayHealthDataUp(ctx context.Context, db *bun.DB) error {
	cipher, err := optionalHolidayHealthCipher()
	if err != nil {
		return err
	}
	return holidayHealthDataUpWith(ctx, db, cipher)
}

// holidayHealthDataUpWith adds the sealed columns, seals every row with
// cipher, drops the plaintext and replaces the hosting school's function, in
// one transaction. A nil cipher is allowed only without anything to seal.
func holidayHealthDataUpWith(ctx context.Context, db *bun.DB, cipher holidayFieldCipher) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayHealthDataAddSQL); err != nil {
			return fmt.Errorf("add sealed holiday health columns: %w", err)
		}
		if err := sealHolidayChildrenHealth(ctx, tx, cipher); err != nil {
			return err
		}
		if err := sealHolidayFamilyHealth(ctx, tx, cipher); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, holidayHealthDataDropPlaintextSQL); err != nil {
			return fmt.Errorf("drop plaintext holiday health columns: %w", err)
		}
		if _, err := tx.ExecContext(ctx, holidayHostParticipantsHealthSQL); err != nil {
			return fmt.Errorf("replace holiday host participants: %w", err)
		}
		return nil
	})
}

type holidayHealthChildRow struct {
	ID             int64                      `bun:"id"`
	OrganizationID int64                      `bun:"organization_id"`
	RegistrationID int64                      `bun:"holiday_registration_id"`
	Position       int                        `bun:"position"`
	Allergies      string                     `bun:"allergies"`
	Medication     string                     `bun:"medication"`
	HealthNotes    string                     `bun:"health_notes"`
	CustomData     map[string]json.RawMessage `bun:"custom_data,type:jsonb"`
	HealthKeys     []string                   `bun:"health_keys,type:jsonb"`
}

type holidayHealthRegistrationRow struct {
	ID             int64                      `bun:"id"`
	OrganizationID int64                      `bun:"organization_id"`
	CustomData     map[string]json.RawMessage `bun:"custom_data,type:jsonb"`
	HealthKeys     []string                   `bun:"health_keys,type:jsonb"`
}

// takeHealthAnswers moves the answers to the health questions out of the
// answers and returns them with the keys that moved.
func takeHealthAnswers(answers map[string]json.RawMessage, keys []string) (map[string]json.RawMessage, []string) {
	taken := map[string]json.RawMessage{}
	moved := []string{}
	for _, key := range keys {
		if value, ok := answers[key]; ok {
			taken[key] = value
			moved = append(moved, key)
		}
	}
	return taken, moved
}

func sealHolidayRecord(cipher holidayFieldCipher, record holidayHealthRecord, context []byte) ([]byte, error) {
	if cipher == nil {
		return nil, errors.New("holiday health information to seal without " + healthDataKeysEnv)
	}
	plaintext, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode holiday health information: %w", err)
	}
	return cipher.Seal(plaintext, context)
}

func sealHolidayChildrenHealth(ctx context.Context, tx bun.Tx, cipher holidayFieldCipher) error {
	var rows []holidayHealthChildRow
	if err := tx.NewRaw(holidayHealthChildrenQuery).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("read holiday children to seal: %w", err)
	}
	for _, row := range rows {
		answers, moved := takeHealthAnswers(row.CustomData, row.HealthKeys)
		record := holidayHealthRecord{Allergies: row.Allergies, Medication: row.Medication, HealthNotes: row.HealthNotes, Answers: answers}
		if record.empty() {
			continue
		}
		sealed, err := sealHolidayRecord(cipher, record, holidayChildHealthContext(row.OrganizationID, row.RegistrationID, row.Position))
		if err != nil {
			return fmt.Errorf("seal holiday child %d: %w", row.ID, err)
		}
		if _, err := tx.NewRaw(`UPDATE enrollment.holiday_registration_children
			SET health_data = ?, custom_data = custom_data - ?::text[] WHERE id = ?`,
			sealed, pgdialect.Array(moved), row.ID).Exec(ctx); err != nil {
			return fmt.Errorf("store sealed holiday child %d: %w", row.ID, err)
		}
	}
	return nil
}

func sealHolidayFamilyHealth(ctx context.Context, tx bun.Tx, cipher holidayFieldCipher) error {
	var rows []holidayHealthRegistrationRow
	if err := tx.NewRaw(holidayHealthRegistrationsQuery).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("read holiday registrations to seal: %w", err)
	}
	for _, row := range rows {
		answers, moved := takeHealthAnswers(row.CustomData, row.HealthKeys)
		if len(moved) == 0 {
			continue
		}
		sealed, err := sealHolidayRecord(cipher, holidayHealthRecord{Answers: answers}, holidayFamilyHealthContext(row.OrganizationID, row.ID))
		if err != nil {
			return fmt.Errorf("seal holiday registration %d: %w", row.ID, err)
		}
		if _, err := tx.NewRaw(`UPDATE enrollment.holiday_registrations
			SET health_data = ?, custom_data = custom_data - ?::text[] WHERE id = ?`,
			sealed, pgdialect.Array(moved), row.ID).Exec(ctx); err != nil {
			return fmt.Errorf("store sealed holiday registration %d: %w", row.ID, err)
		}
	}
	return nil
}

// The down path restores the plaintext columns and answers from the sealed
// values and the function of #5202. It needs the key ring whenever a value
// is sealed.
const holidayHealthDataRestoreColumnsSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'health';
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof'));

ALTER TABLE enrollment.holiday_registration_children
	ADD COLUMN allergies TEXT NOT NULL DEFAULT '',
	ADD COLUMN medication TEXT NOT NULL DEFAULT '',
	ADD COLUMN health_notes TEXT NOT NULL DEFAULT '',
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_children_texts;
ALTER TABLE enrollment.holiday_registration_children
	ADD CONSTRAINT chk_holiday_registration_children_texts CHECK (
		char_length(guest_school_name) <= 120 AND char_length(allergies) <= 1000
		AND char_length(medication) <= 1000 AND char_length(health_notes) <= 2000);
`

const holidayHealthDataDropFunctionSQL = `
DROP FUNCTION IF EXISTS enrollment.holiday_host_participants(BIGINT, BIGINT);
`

const holidayHealthDataDropSealedSQL = `
ALTER TABLE enrollment.holiday_guests
	DROP CONSTRAINT IF EXISTS chk_holiday_guests_health_data,
	DROP COLUMN IF EXISTS health_data;
ALTER TABLE enrollment.holiday_registration_children
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_children_health_data,
	DROP COLUMN IF EXISTS health_data;
ALTER TABLE enrollment.holiday_registrations
	DROP CONSTRAINT IF EXISTS chk_holiday_registrations_health_data,
	DROP CONSTRAINT IF EXISTS chk_holiday_registrations_consent_version,
	DROP COLUMN IF EXISTS health_data,
	DROP COLUMN IF EXISTS consent_version;
`

func holidayHealthDataDown(ctx context.Context, db *bun.DB) error {
	cipher, err := optionalHolidayHealthCipher()
	if err != nil {
		return err
	}
	return holidayHealthDataDownWith(ctx, db, cipher)
}

func holidayHealthDataDownWith(ctx context.Context, db *bun.DB, cipher holidayFieldCipher) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayHealthDataRestoreColumnsSQL); err != nil {
			return fmt.Errorf("restore plaintext holiday health columns: %w", err)
		}
		if err := openHolidayHealth(ctx, tx, cipher); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, holidayHealthDataDropFunctionSQL); err != nil {
			return fmt.Errorf("drop holiday host participants with health: %w", err)
		}
		if err := restoreHolidayHostParticipantsSharedAnswers(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, holidayHealthDataDropSealedSQL); err != nil {
			return fmt.Errorf("drop sealed holiday health columns: %w", err)
		}
		return nil
	})
}

type holidaySealedRow struct {
	ID             int64  `bun:"id"`
	OrganizationID int64  `bun:"organization_id"`
	RegistrationID int64  `bun:"registration_id"`
	Position       int    `bun:"position"`
	HealthData     []byte `bun:"health_data"`
}

func openHolidayRecord(cipher holidayFieldCipher, sealed, context []byte) (holidayHealthRecord, error) {
	if cipher == nil {
		return holidayHealthRecord{}, errors.New("sealed holiday health information without " + healthDataKeysEnv)
	}
	plaintext, err := cipher.Open(sealed, context)
	if err != nil {
		return holidayHealthRecord{}, err
	}
	var record holidayHealthRecord
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return holidayHealthRecord{}, fmt.Errorf("decode holiday health information: %w", err)
	}
	return record, nil
}

func encodeRestoredAnswers(answers map[string]json.RawMessage) (string, error) {
	if answers == nil {
		answers = map[string]json.RawMessage{}
	}
	encoded, err := json.Marshal(answers)
	return string(encoded), err
}

func openHolidayHealth(ctx context.Context, tx bun.Tx, cipher holidayFieldCipher) error {
	var children []holidaySealedRow
	if err := tx.NewRaw(`SELECT id, organization_id, holiday_registration_id AS registration_id, position, health_data
		FROM enrollment.holiday_registration_children WHERE health_data IS NOT NULL ORDER BY id`).Scan(ctx, &children); err != nil {
		return fmt.Errorf("read sealed holiday children: %w", err)
	}
	for _, row := range children {
		record, err := openHolidayRecord(cipher, row.HealthData, holidayChildHealthContext(row.OrganizationID, row.RegistrationID, row.Position))
		if err != nil {
			return fmt.Errorf("open holiday child %d: %w", row.ID, err)
		}
		answers, err := encodeRestoredAnswers(record.Answers)
		if err != nil {
			return err
		}
		if _, err := tx.NewRaw(`UPDATE enrollment.holiday_registration_children
			SET allergies = ?, medication = ?, health_notes = ?, custom_data = custom_data || ?::jsonb WHERE id = ?`,
			record.Allergies, record.Medication, record.HealthNotes, answers, row.ID).Exec(ctx); err != nil {
			return fmt.Errorf("restore holiday child %d: %w", row.ID, err)
		}
	}
	return openHolidayFamilyHealth(ctx, tx, cipher)
}

// openHolidayFamilyHealth restores the family answers of every registration.
func openHolidayFamilyHealth(ctx context.Context, tx bun.Tx, cipher holidayFieldCipher) error {
	var registrations []holidaySealedRow
	if err := tx.NewRaw(`SELECT id, organization_id, id AS registration_id, 0 AS position, health_data
		FROM enrollment.holiday_registrations WHERE health_data IS NOT NULL ORDER BY id`).Scan(ctx, &registrations); err != nil {
		return fmt.Errorf("read sealed holiday registrations: %w", err)
	}
	for _, row := range registrations {
		record, err := openHolidayRecord(cipher, row.HealthData, holidayFamilyHealthContext(row.OrganizationID, row.ID))
		if err != nil {
			return fmt.Errorf("open holiday registration %d: %w", row.ID, err)
		}
		answers, err := encodeRestoredAnswers(record.Answers)
		if err != nil {
			return err
		}
		if _, err := tx.NewRaw(`UPDATE enrollment.holiday_registrations SET custom_data = custom_data || ?::jsonb WHERE id = ?`,
			answers, row.ID).Exec(ctx); err != nil {
			return fmt.Errorf("restore holiday registration %d: %w", row.ID, err)
		}
	}
	return nil
}
