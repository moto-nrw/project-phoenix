package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationsVersion = "1.15.545"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationsVersion,
		Description: "Öffentliche Ferienanmeldung ohne Elternkonto mit Feriengästen und Träger-Mail in der Outbox (#4661)",
		DependsOn: []string{
			traegerErasureOrderVersion,   // the last migration before this one
			traegerHolidayPeriodsVersion, // the holiday periods the registrations book
			systemEmailOutboxVersion,     // the idempotency index of the email outbox this migration widens
		},
	})
	Migrations.MustRegister(holidayRegistrationsUp, holidayRegistrationsDown)
}

// holidayRegistrationsUpSQL adds the public holiday registration (#4661, spec
// #4611 user stories 26 and 28 to 31, ADR 0106):
//
//   - enrollment.holiday_periods.public_token: the one link of a period for
//     all its schools, two random UUIDs without dashes.
//   - enrollment.holiday_registrations: the parent who registers, the
//     consents and the time of submission.
//   - enrollment.holiday_registration_children: up to three siblings with
//     their health information. A child without a home school of the Träger
//     is a holiday guest (home_school_id IS NULL); no child is linked to a
//     student of a school.
//   - enrollment.holiday_registration_contacts: emergency contacts and
//     persons allowed to collect the children.
//   - enrollment.holiday_registration_bookings: one time model per child and
//     week. A week or time model with bookings cannot be removed. The keys
//     to week and time model are checked at commit, so a deleted period or
//     Träger takes its bookings along in any cascade order; the change of a
//     period checks them at once (SET CONSTRAINTS ... IMMEDIATE) and names
//     the refusal.
//   - enrollment.holiday_registration_rate_limits: the abuse counters of the
//     public link per Träger, keyed by SHA-256 digests only.
//   - platform.email_outbox.organization_id: a mail of a Träger, which the
//     Träger transaction writes in the same unit as the registration.
//
// Every new table carries organization_id and the Träger policy of ADR 0105.
// The composite keys tie every row to the Träger and the period of its
// registration; a trigger admits only an undeleted school of the Träger as a
// home school, like the host school of a week.
const holidayRegistrationsUpSQL = `
ALTER TABLE enrollment.holiday_periods
	ADD COLUMN public_token TEXT NOT NULL
		DEFAULT (replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', ''));
ALTER TABLE enrollment.holiday_periods
	ADD CONSTRAINT uq_holiday_periods_public_token UNIQUE (public_token);
ALTER TABLE enrollment.holiday_period_weeks
	ADD CONSTRAINT uq_holiday_period_weeks_period UNIQUE (organization_id, holiday_period_id, id);
ALTER TABLE enrollment.holiday_week_time_models
	ADD CONSTRAINT uq_holiday_week_time_models_week UNIQUE (organization_id, holiday_period_week_id, id);

CREATE TABLE enrollment.holiday_registrations (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	holiday_period_id BIGINT NOT NULL,
	guardian_first_name TEXT NOT NULL,
	guardian_last_name TEXT NOT NULL,
	guardian_email TEXT NOT NULL,
	guardian_phone TEXT NOT NULL,
	status VARCHAR(16) NOT NULL DEFAULT 'received',
	privacy_consent_at TIMESTAMPTZ NOT NULL,
	health_consent_at TIMESTAMPTZ,
	binding_confirmed_at TIMESTAMPTZ,
	submitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_holiday_registrations_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_holiday_registrations_period UNIQUE (organization_id, holiday_period_id, id),
	CONSTRAINT fk_holiday_registrations_period FOREIGN KEY (organization_id, holiday_period_id)
		REFERENCES enrollment.holiday_periods (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_registrations_guardian CHECK (
		BTRIM(guardian_first_name) <> '' AND char_length(guardian_first_name) <= 100
		AND BTRIM(guardian_last_name) <> '' AND char_length(guardian_last_name) <= 100
		AND BTRIM(guardian_email) <> '' AND char_length(guardian_email) <= 254
		AND BTRIM(guardian_phone) <> '' AND char_length(guardian_phone) <= 40),
	CONSTRAINT chk_holiday_registrations_status CHECK (status IN ('received'))
);
CREATE INDEX idx_holiday_registrations_period ON enrollment.holiday_registrations (holiday_period_id);
COMMENT ON TABLE enrollment.holiday_registrations IS
	'Public holiday registration of a parent without an account (#4661); read and written in the Träger transaction (ADR 0106).';

CREATE TABLE enrollment.holiday_registration_children (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	birth_date DATE NOT NULL,
	home_school_id BIGINT,
	guest_school_name TEXT NOT NULL DEFAULT '',
	allergies TEXT NOT NULL DEFAULT '',
	medication TEXT NOT NULL DEFAULT '',
	health_notes TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_holiday_registration_children_period UNIQUE (organization_id, holiday_period_id, id),
	CONSTRAINT uq_holiday_registration_children_position UNIQUE (holiday_registration_id, position),
	CONSTRAINT fk_holiday_registration_children_registration
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_registration_children_position CHECK (position BETWEEN 1 AND 3),
	CONSTRAINT chk_holiday_registration_children_name CHECK (
		BTRIM(first_name) <> '' AND char_length(first_name) <= 100
		AND BTRIM(last_name) <> '' AND char_length(last_name) <= 100),
	CONSTRAINT chk_holiday_registration_children_guest CHECK (home_school_id IS NULL OR guest_school_name = ''),
	CONSTRAINT chk_holiday_registration_children_texts CHECK (
		char_length(guest_school_name) <= 120 AND char_length(allergies) <= 1000
		AND char_length(medication) <= 1000 AND char_length(health_notes) <= 2000)
);
CREATE INDEX idx_holiday_registration_children_home_school ON enrollment.holiday_registration_children (home_school_id)
	WHERE home_school_id IS NOT NULL;
COMMENT ON TABLE enrollment.holiday_registration_children IS
	'Child of a holiday registration (#4661); without a home school of the Träger it is a holiday guest, never a student of a school.';

CREATE TABLE enrollment.holiday_registration_contacts (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	position SMALLINT NOT NULL,
	name TEXT NOT NULL,
	phone TEXT NOT NULL DEFAULT '',
	relationship TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_registration_contacts_registration FOREIGN KEY (organization_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_holiday_registration_contacts_position UNIQUE (holiday_registration_id, kind, position),
	CONSTRAINT chk_holiday_registration_contacts_kind CHECK (kind IN ('emergency', 'pickup')),
	CONSTRAINT chk_holiday_registration_contacts_texts CHECK (
		BTRIM(name) <> '' AND char_length(name) <= 120 AND char_length(phone) <= 40 AND char_length(relationship) <= 60)
);
COMMENT ON TABLE enrollment.holiday_registration_contacts IS
	'Emergency contact or person allowed to collect the children of a holiday registration (#4661).';

CREATE TABLE enrollment.holiday_registration_bookings (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_child_id BIGINT NOT NULL,
	holiday_period_week_id BIGINT NOT NULL,
	holiday_week_time_model_id BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_registration_bookings_child
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_child_id)
		REFERENCES enrollment.holiday_registration_children (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_registration_bookings_week
		FOREIGN KEY (organization_id, holiday_period_id, holiday_period_week_id)
		REFERENCES enrollment.holiday_period_weeks (organization_id, holiday_period_id, id)
		DEFERRABLE INITIALLY DEFERRED,
	CONSTRAINT fk_holiday_registration_bookings_time_model
		FOREIGN KEY (organization_id, holiday_period_week_id, holiday_week_time_model_id)
		REFERENCES enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, id)
		DEFERRABLE INITIALLY DEFERRED,
	CONSTRAINT uq_holiday_registration_bookings_week UNIQUE (holiday_registration_child_id, holiday_period_week_id)
);
CREATE INDEX idx_holiday_registration_bookings_week ON enrollment.holiday_registration_bookings (holiday_period_week_id);
CREATE INDEX idx_holiday_registration_bookings_time_model ON enrollment.holiday_registration_bookings (holiday_week_time_model_id);
COMMENT ON TABLE enrollment.holiday_registration_bookings IS
	'Time model a child of a holiday registration books in one week (#4661); keeps the week and the time model from being removed.';

CREATE TABLE enrollment.holiday_registration_rate_limits (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	key_type VARCHAR(16) NOT NULL,
	key_hash TEXT NOT NULL,
	attempts INT NOT NULL DEFAULT 1,
	window_start TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, key_type, key_hash),
	CONSTRAINT chk_holiday_registration_rate_limits_key_type CHECK (key_type IN ('ip', 'email'))
);
COMMENT ON TABLE enrollment.holiday_registration_rate_limits IS
	'Abuse counter of the public holiday registration per Träger (#4661), keyed by SHA-256 digests; expired rows go on the next count.';

CREATE TRIGGER update_enrollment_holiday_registrations_updated_at
	BEFORE UPDATE ON enrollment.holiday_registrations
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER update_enrollment_holiday_registration_children_updated_at
	BEFORE UPDATE ON enrollment.holiday_registration_children
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

CREATE FUNCTION enrollment.check_holiday_child_home_school()
	RETURNS trigger
	LANGUAGE plpgsql
	SET search_path = pg_catalog, pg_temp
	AS $$
	BEGIN
		IF NEW.home_school_id IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM platform.schools AS school
			WHERE school.id = NEW.home_school_id
				AND school.organization_id = NEW.organization_id
				AND school.deleted_at IS NULL
		) THEN
			RAISE EXCEPTION 'home school % is not a school of Träger %', NEW.home_school_id, NEW.organization_id
				USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_holiday_registration_children_home_school';
		END IF;
		RETURN NEW;
	END;
	$$;
CREATE TRIGGER check_holiday_child_home_school
	BEFORE INSERT OR UPDATE OF home_school_id, organization_id ON enrollment.holiday_registration_children
	FOR EACH ROW EXECUTE FUNCTION enrollment.check_holiday_child_home_school();

ALTER TABLE enrollment.holiday_registrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registrations FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registrations ON enrollment.holiday_registrations FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_registration_children ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_children FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_children ON enrollment.holiday_registration_children FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_registration_contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_contacts FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_contacts ON enrollment.holiday_registration_contacts FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_registration_bookings ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_bookings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_bookings ON enrollment.holiday_registration_bookings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_registration_rate_limits ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_rate_limits FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_rate_limits ON enrollment.holiday_registration_rate_limits FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.holiday_registrations, enrollment.holiday_registration_children,
	enrollment.holiday_registration_contacts, enrollment.holiday_registration_bookings,
	enrollment.holiday_registration_rate_limits TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_registrations_id_seq, enrollment.holiday_registration_children_id_seq,
	enrollment.holiday_registration_contacts_id_seq, enrollment.holiday_registration_bookings_id_seq TO phoenix_tenant;

ALTER TABLE platform.email_outbox
	ADD COLUMN organization_id BIGINT REFERENCES platform.organizations(id) ON DELETE CASCADE;
ALTER TABLE platform.email_outbox
	ADD CONSTRAINT chk_email_outbox_one_scope CHECK (tenant_id IS NULL OR organization_id IS NULL);
CREATE INDEX idx_email_outbox_organization ON platform.email_outbox (organization_id)
	WHERE organization_id IS NOT NULL;
DROP INDEX platform.uq_email_outbox_tenant_idempotency;
CREATE UNIQUE INDEX uq_email_outbox_tenant_idempotency
	ON platform.email_outbox (tenant_id, organization_id, idempotency_key) NULLS NOT DISTINCT
	WHERE idempotency_key IS NOT NULL;
CREATE POLICY traeger_isolation_platform_email_outbox ON platform.email_outbox FOR ALL
	USING (tenant_id IS NULL AND organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (tenant_id IS NULL AND organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
`

const holidayRegistrationsDownSQL = `
DROP POLICY IF EXISTS traeger_isolation_platform_email_outbox ON platform.email_outbox;
DELETE FROM platform.email_outbox WHERE organization_id IS NOT NULL;
DROP INDEX IF EXISTS platform.uq_email_outbox_tenant_idempotency;
CREATE UNIQUE INDEX uq_email_outbox_tenant_idempotency
	ON platform.email_outbox (tenant_id, idempotency_key) NULLS NOT DISTINCT
	WHERE idempotency_key IS NOT NULL;
DROP INDEX IF EXISTS platform.idx_email_outbox_organization;
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_one_scope;
ALTER TABLE platform.email_outbox DROP COLUMN IF EXISTS organization_id;
DROP TABLE IF EXISTS enrollment.holiday_registration_rate_limits;
DROP TABLE IF EXISTS enrollment.holiday_registration_bookings;
DROP TABLE IF EXISTS enrollment.holiday_registration_contacts;
DROP TABLE IF EXISTS enrollment.holiday_registration_children;
DROP TABLE IF EXISTS enrollment.holiday_registrations;
DROP FUNCTION IF EXISTS enrollment.check_holiday_child_home_school();
ALTER TABLE enrollment.holiday_week_time_models DROP CONSTRAINT IF EXISTS uq_holiday_week_time_models_week;
ALTER TABLE enrollment.holiday_period_weeks DROP CONSTRAINT IF EXISTS uq_holiday_period_weeks_period;
ALTER TABLE enrollment.holiday_periods DROP CONSTRAINT IF EXISTS uq_holiday_periods_public_token;
ALTER TABLE enrollment.holiday_periods DROP COLUMN IF EXISTS public_token;
`

func holidayRegistrationsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationsUpSQL); err != nil {
		return fmt.Errorf("create holiday registrations: %w", err)
	}
	return nil
}

func holidayRegistrationsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationsDownSQL); err != nil {
		return fmt.Errorf("drop holiday registrations: %w", err)
	}
	return nil
}
