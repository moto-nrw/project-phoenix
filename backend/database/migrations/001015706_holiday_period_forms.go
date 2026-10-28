package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayPeriodFormsVersion = "1.15.706"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayPeriodFormsVersion,
		Description: "Ferienanmeldung: Formular-Baukasten je Ferienzeitraum mit Versionen und Antworten (#5198)",
		DependsOn: []string{
			visitRevisionEventsVersion,  // the last migration before this one
			holidayRegistrationsVersion, // the registrations that keep their form version
		},
	})
	Migrations.MustRegister(holidayPeriodFormsUp, holidayPeriodFormsDown)
}

// holidayPeriodFormsUpSQL adds the registration form of a holiday period
// (#5198, spec #4611 B7/B8):
//
//   - enrollment.holiday_period_forms: one row per saved version of the
//     form of a period, with its questions as JSONB. A version never
//     changes: the request role may read and add versions, not change or
//     delete them. The versions go with their period; a version a
//     registration keeps cannot go on its own. The key is checked at commit,
//     so a deleted period or Träger takes versions and registrations along
//     in any cascade order, like the bookings of #4661.
//   - enrollment.holiday_registrations.form_version: the version a
//     registration was submitted with, and custom_data with its answers to
//     the questions per family.
//   - enrollment.holiday_registration_children.custom_data: the answers per
//     child.
//
// Every existing period gets version 1 without questions, and every
// existing registration points to it, so a published period shows parents
// exactly the form it showed before. A deferred trigger keeps that true for
// every later period: at commit it writes version 1 without questions for a
// period that has no version, while a new period of the Träger-Portal
// brings version 1 with the standard template itself. form_version
// defaults to 1 for writers of before #5198. The table carries
// organization_id and the Träger policy of ADR 0105; a school transaction
// sees no row.
const holidayPeriodFormsUpSQL = `
CREATE TABLE enrollment.holiday_period_forms (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	version INT NOT NULL,
	fields JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_period_forms_period FOREIGN KEY (organization_id, holiday_period_id)
		REFERENCES enrollment.holiday_periods (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_holiday_period_forms_version UNIQUE (organization_id, holiday_period_id, version),
	CONSTRAINT chk_holiday_period_forms_version CHECK (version >= 1),
	CONSTRAINT chk_holiday_period_forms_fields CHECK (jsonb_typeof(fields) = 'array')
);
COMMENT ON TABLE enrollment.holiday_period_forms IS
	'Version of the registration form of a Träger holiday period (#5198); a registration keeps the version it was submitted with. Read and written in the Träger transaction (ADR 0105).';

ALTER TABLE enrollment.holiday_period_forms ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_period_forms FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_period_forms ON enrollment.holiday_period_forms FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON enrollment.holiday_period_forms FROM PUBLIC, phoenix_tenant;
GRANT SELECT, INSERT ON enrollment.holiday_period_forms TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_period_forms_id_seq TO phoenix_tenant;

INSERT INTO enrollment.holiday_period_forms (organization_id, holiday_period_id, version)
SELECT organization_id, id, 1 FROM enrollment.holiday_periods;

CREATE FUNCTION enrollment.ensure_holiday_period_form()
	RETURNS trigger
	LANGUAGE plpgsql
	SET search_path = pg_catalog, pg_temp
	AS $$
	BEGIN
		INSERT INTO enrollment.holiday_period_forms (organization_id, holiday_period_id, version)
		SELECT NEW.organization_id, NEW.id, 1
		WHERE EXISTS (
				SELECT 1 FROM enrollment.holiday_periods AS period
				WHERE period.organization_id = NEW.organization_id AND period.id = NEW.id)
			AND NOT EXISTS (
				SELECT 1 FROM enrollment.holiday_period_forms AS form
				WHERE form.organization_id = NEW.organization_id AND form.holiday_period_id = NEW.id);
		RETURN NULL;
	END;
	$$;
CREATE CONSTRAINT TRIGGER ensure_holiday_period_form
	AFTER INSERT ON enrollment.holiday_periods
	DEFERRABLE INITIALLY DEFERRED
	FOR EACH ROW EXECUTE FUNCTION enrollment.ensure_holiday_period_form();

ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN form_version INT DEFAULT 1,
	ADD COLUMN custom_data JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE enrollment.holiday_registrations
	ALTER COLUMN form_version SET NOT NULL,
	ADD CONSTRAINT fk_holiday_registrations_form FOREIGN KEY (organization_id, holiday_period_id, form_version)
		REFERENCES enrollment.holiday_period_forms (organization_id, holiday_period_id, version)
		DEFERRABLE INITIALLY DEFERRED,
	ADD CONSTRAINT chk_holiday_registrations_custom_data CHECK (jsonb_typeof(custom_data) = 'object');
COMMENT ON COLUMN enrollment.holiday_registrations.form_version IS
	'Version of the period''s form the registration was submitted with (#5198).';

ALTER TABLE enrollment.holiday_registration_children
	ADD COLUMN custom_data JSONB NOT NULL DEFAULT '{}'::jsonb,
	ADD CONSTRAINT chk_holiday_registration_children_custom_data CHECK (jsonb_typeof(custom_data) = 'object');
`

func holidayPeriodFormsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayPeriodFormsUpSQL); err != nil {
		return fmt.Errorf("add enrollment.holiday_period_forms: %w", err)
	}
	return nil
}

const holidayPeriodFormsDownSQL = `
ALTER TABLE enrollment.holiday_registration_children
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_children_custom_data,
	DROP COLUMN IF EXISTS custom_data;
ALTER TABLE enrollment.holiday_registrations
	DROP CONSTRAINT IF EXISTS chk_holiday_registrations_custom_data,
	DROP CONSTRAINT IF EXISTS fk_holiday_registrations_form,
	DROP COLUMN IF EXISTS custom_data,
	DROP COLUMN IF EXISTS form_version;
DROP TRIGGER IF EXISTS ensure_holiday_period_form ON enrollment.holiday_periods;
DROP FUNCTION IF EXISTS enrollment.ensure_holiday_period_form();
DROP TABLE IF EXISTS enrollment.holiday_period_forms;
`

func holidayPeriodFormsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayPeriodFormsDownSQL); err != nil {
		return fmt.Errorf("drop enrollment.holiday_period_forms: %w", err)
	}
	return nil
}
