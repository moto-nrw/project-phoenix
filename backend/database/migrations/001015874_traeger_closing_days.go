package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerClosingDaysVersion = "1.15.874"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerClosingDaysVersion,
		Description: "Schulkalender: Schließtage des Trägers für ausgewählte Schulen (#4702)",
		DependsOn: []string{
			parentBillingBelegRetentionVersion, // the newest migration on development
			closingDaysVersion,                 // the school closing days these extend
			traegerHolidayPeriodsVersion,       // the Träger policy this table follows (ADR 0105)
		},
	})
	Migrations.MustRegister(traegerClosingDaysUp, traegerClosingDaysDown)
}

// traegerClosingDaysUpSQL creates the closing days a Träger plans for some or
// all of its schools (#4702), such as a bridge day or a staff outing.
//
// The rows belong to the Träger: they carry organization_id and follow the
// Träger policy of ADR 0105, so the Träger-Büro reads and writes them in its
// Träger transaction. A school reads, never writes, the rows that name it in
// school_ids: School Calendar merges them into the school's closing days, so
// they act exactly like a closing day the school entered itself (Soll,
// Dienstplan, Regeltermine, statistics). A trigger admits only undeleted
// schools of the row's Träger, each once. The school IDs are no foreign key:
// the rows go with the Träger, and a school erased later (ADR 0092) leaves
// its bare ID, which no school transaction can match again.
//
// The rows hold dates and a reason, no personal data, so the demo erasure
// does not plan them; they go with their Träger.
const traegerClosingDaysUpSQL = `
CREATE TABLE schedule.traeger_closing_days (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	start_date DATE NOT NULL,
	end_date DATE NOT NULL,
	reason TEXT NOT NULL,
	school_ids BIGINT[] NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_closing_days_range CHECK (start_date <= end_date AND end_date - start_date < 366),
	CONSTRAINT chk_traeger_closing_days_reason CHECK (BTRIM(reason) <> '' AND char_length(reason) <= 120),
	CONSTRAINT chk_traeger_closing_days_schools CHECK (cardinality(school_ids) BETWEEN 1 AND 500)
);
CREATE INDEX idx_traeger_closing_days_organization_range
	ON schedule.traeger_closing_days (organization_id, start_date, end_date);
CREATE INDEX idx_traeger_closing_days_schools
	ON schedule.traeger_closing_days USING GIN (school_ids);
COMMENT ON TABLE schedule.traeger_closing_days IS
	'Closing days a Träger plans for some of its schools (#4702); written in the Träger transaction, read by the named schools.';

CREATE TRIGGER update_traeger_closing_days_updated_at
	BEFORE UPDATE ON schedule.traeger_closing_days
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

CREATE FUNCTION schedule.check_traeger_closing_day_schools()
	RETURNS trigger
	LANGUAGE plpgsql
	SET search_path = pg_catalog, pg_temp
	AS $$
	DECLARE
		v_outside BIGINT;
	BEGIN
		IF (SELECT count(*) FROM unnest(NEW.school_ids) AS id) <> (SELECT count(DISTINCT id) FROM unnest(NEW.school_ids) AS id) THEN
			RAISE EXCEPTION 'a school is named twice'
				USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_traeger_closing_days_school_member';
		END IF;
		SELECT requested.id INTO v_outside
		FROM unnest(NEW.school_ids) AS requested(id)
		WHERE NOT EXISTS (
			SELECT 1 FROM platform.schools AS school
			WHERE school.id = requested.id
				AND school.organization_id = NEW.organization_id
				AND school.deleted_at IS NULL
		)
		LIMIT 1;
		IF v_outside IS NOT NULL THEN
			RAISE EXCEPTION 'school % is not a school of Träger %', v_outside, NEW.organization_id
				USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_traeger_closing_days_school_member';
		END IF;
		RETURN NEW;
	END;
	$$;
CREATE TRIGGER check_traeger_closing_day_schools
	BEFORE INSERT OR UPDATE OF school_ids, organization_id ON schedule.traeger_closing_days
	FOR EACH ROW EXECUTE FUNCTION schedule.check_traeger_closing_day_schools();

ALTER TABLE schedule.traeger_closing_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE schedule.traeger_closing_days FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_schedule_traeger_closing_days ON schedule.traeger_closing_days FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
CREATE POLICY school_read_schedule_traeger_closing_days ON schedule.traeger_closing_days FOR SELECT
	USING (NULLIF(current_setting('app.current_tenant_id', true), '')::bigint = ANY (school_ids)
		AND EXISTS (SELECT 1 FROM platform.schools AS school
			WHERE school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
				AND school.organization_id = traeger_closing_days.organization_id AND school.deleted_at IS NULL));

GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.traeger_closing_days TO phoenix_tenant;
GRANT USAGE ON SEQUENCE schedule.traeger_closing_days_id_seq TO phoenix_tenant;
`

const traegerClosingDaysDownSQL = `
DROP TABLE IF EXISTS schedule.traeger_closing_days;
DROP FUNCTION IF EXISTS schedule.check_traeger_closing_day_schools();
`

func traegerClosingDaysUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerClosingDaysUpSQL); err != nil {
		return fmt.Errorf("create Träger closing days: %w", err)
	}
	return nil
}

func traegerClosingDaysDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerClosingDaysDownSQL); err != nil {
		return fmt.Errorf("drop Träger closing days: %w", err)
	}
	return nil
}
