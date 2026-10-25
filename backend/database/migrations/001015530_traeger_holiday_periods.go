package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerHolidayPeriodsVersion = "1.15.530"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerHolidayPeriodsVersion,
		Description: "Träger-Portal: Ferienzeitraum mit Wochen, ausrichtender OGS und Zeitmodellen in der Träger-Transaktion (#4659)",
		DependsOn: []string{
			accountAnalyticsConsentVersion,     // the last migration before this one
			traegerPortalVersion,               // the Träger-Portal this table family serves
			createEnrollmentFormSchemasVersion, // the enrollment schema and its default privileges
		},
	})
	Migrations.MustRegister(traegerHolidayPeriodsUp, traegerHolidayPeriodsDown)
}

// traegerHolidayPeriodsUpSQL creates the holiday periods of a Träger (#4659,
// spec #4611 user stories 22 to 25):
//
//   - enrollment.holiday_periods: name, registration window, whether a
//     registration is binding, hints for parents, draft or published.
//   - enrollment.holiday_period_weeks: the bookable weeks, at most seven
//     days each, with the school of the Träger that hosts the week.
//   - enrollment.holiday_week_time_models: the care times of a week with
//     their price in whole cent.
//
// The tables belong to the Träger, not to a school: they carry
// organization_id and no tenant_id. They are the first tables of the Träger
// transaction (ADR 0086 section 5, ADR 0105): the request role sees and
// writes only the rows of the Träger in app.current_org_id, and a school
// transaction, which never sets it, sees none. The administrative role
// bypasses RLS as for every table. The composite keys tie each week and time
// model to the Träger of its period, so a row can never move to a foreign
// Träger's period. A trigger admits only an undeleted school of the
// period's Träger as host; the application names that refusal. The host is
// no foreign key: the rows belong to the Träger and go with it, and the
// school erasure (ADR 0092) plans only school-scoped rows. A week whose host
// school is later erased keeps the bare ID until the Träger-Büro changes it.
const traegerHolidayPeriodsUpSQL = `
CREATE TABLE enrollment.holiday_periods (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	registration_opens_on DATE NOT NULL,
	registration_closes_on DATE NOT NULL,
	binding BOOLEAN NOT NULL DEFAULT false,
	notes TEXT NOT NULL DEFAULT '',
	status VARCHAR(16) NOT NULL DEFAULT 'draft',
	published_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_holiday_periods_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_holiday_periods_name CHECK (BTRIM(name) <> '' AND char_length(name) <= 120),
	CONSTRAINT chk_holiday_periods_notes CHECK (char_length(notes) <= 2000),
	CONSTRAINT chk_holiday_periods_registration CHECK (registration_opens_on <= registration_closes_on),
	CONSTRAINT chk_holiday_periods_status CHECK (status IN ('draft', 'published')),
	CONSTRAINT chk_holiday_periods_published CHECK ((status = 'published') = (published_at IS NOT NULL))
);
COMMENT ON TABLE enrollment.holiday_periods IS
	'Holiday period of a Träger with its registration window (#4659); read and written in the Träger transaction (ADR 0105).';

CREATE TABLE enrollment.holiday_period_weeks (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	starts_on DATE NOT NULL,
	ends_on DATE NOT NULL,
	host_school_id BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_holiday_period_weeks_organization UNIQUE (organization_id, id),
	CONSTRAINT fk_holiday_period_weeks_period FOREIGN KEY (organization_id, holiday_period_id)
		REFERENCES enrollment.holiday_periods (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_period_weeks_days CHECK (starts_on <= ends_on AND ends_on - starts_on < 7)
);
CREATE INDEX idx_holiday_period_weeks_period ON enrollment.holiday_period_weeks (holiday_period_id, starts_on);
CREATE INDEX idx_holiday_period_weeks_host_school ON enrollment.holiday_period_weeks (host_school_id);
COMMENT ON TABLE enrollment.holiday_period_weeks IS
	'Bookable week of a holiday period with the school of the Träger that hosts it (#4659).';

CREATE TABLE enrollment.holiday_week_time_models (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_week_id BIGINT NOT NULL,
	name TEXT NOT NULL,
	starts_at TIME NOT NULL,
	ends_at TIME NOT NULL,
	price_cents BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_week_time_models_week FOREIGN KEY (organization_id, holiday_period_week_id)
		REFERENCES enrollment.holiday_period_weeks (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_week_time_models_name CHECK (BTRIM(name) <> '' AND char_length(name) <= 80),
	CONSTRAINT chk_holiday_week_time_models_times CHECK (starts_at < ends_at),
	CONSTRAINT chk_holiday_week_time_models_price CHECK (price_cents >= 0)
);
CREATE INDEX idx_holiday_week_time_models_week ON enrollment.holiday_week_time_models (holiday_period_week_id);
COMMENT ON TABLE enrollment.holiday_week_time_models IS
	'Care time of a holiday week with its price in whole cent (#4659); prices never live in code.';

CREATE TRIGGER update_enrollment_holiday_periods_updated_at
	BEFORE UPDATE ON enrollment.holiday_periods
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER update_enrollment_holiday_period_weeks_updated_at
	BEFORE UPDATE ON enrollment.holiday_period_weeks
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER update_enrollment_holiday_week_time_models_updated_at
	BEFORE UPDATE ON enrollment.holiday_week_time_models
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

CREATE FUNCTION enrollment.check_holiday_week_host_school()
	RETURNS trigger
	LANGUAGE plpgsql
	SET search_path = pg_catalog, pg_temp
	AS $$
	BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM platform.schools AS school
			WHERE school.id = NEW.host_school_id
				AND school.organization_id = NEW.organization_id
				AND school.deleted_at IS NULL
		) THEN
			RAISE EXCEPTION 'host school % is not a school of Träger %', NEW.host_school_id, NEW.organization_id
				USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_holiday_period_weeks_host_school';
		END IF;
		RETURN NEW;
	END;
	$$;
CREATE TRIGGER check_holiday_week_host_school
	BEFORE INSERT OR UPDATE OF host_school_id, organization_id ON enrollment.holiday_period_weeks
	FOR EACH ROW EXECUTE FUNCTION enrollment.check_holiday_week_host_school();

ALTER TABLE enrollment.holiday_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_periods FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_periods ON enrollment.holiday_periods FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_period_weeks ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_period_weeks FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_period_weeks ON enrollment.holiday_period_weeks FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_week_time_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_week_time_models FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_week_time_models ON enrollment.holiday_week_time_models FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.holiday_periods, enrollment.holiday_period_weeks,
	enrollment.holiday_week_time_models TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_periods_id_seq, enrollment.holiday_period_weeks_id_seq,
	enrollment.holiday_week_time_models_id_seq TO phoenix_tenant;
`

const traegerHolidayPeriodsDownSQL = `
DROP TABLE IF EXISTS enrollment.holiday_week_time_models;
DROP TABLE IF EXISTS enrollment.holiday_period_weeks;
DROP TABLE IF EXISTS enrollment.holiday_periods;
DROP FUNCTION IF EXISTS enrollment.check_holiday_week_host_school();
`

func traegerHolidayPeriodsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerHolidayPeriodsUpSQL); err != nil {
		return fmt.Errorf("create Träger holiday periods: %w", err)
	}
	return nil
}

func traegerHolidayPeriodsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerHolidayPeriodsDownSQL); err != nil {
		return fmt.Errorf("drop Träger holiday periods: %w", err)
	}
	return nil
}
