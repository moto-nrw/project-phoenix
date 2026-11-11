package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerCatalogVersion = "1.15.937"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerCatalogVersion,
		Description: "Trägerphase und Trägerangebot mit Versionen, Schulauswahl und Kopie je Schule (#4704, ADR 0195)",
		DependsOn: []string{
			dataReturnErasureOrderLockVersion, // data-return erasure serialization
			traegerHolidayPeriodsVersion,      // the Träger table pattern this one follows
			careOfferingTemplateGapAckVersion, // the last change of enrollment.care_offerings
		},
	})
	Migrations.MustRegister(traegerCatalogUp, traegerCatalogDown)
}

// The fields the Träger sets on a copy (ADR 0195 section 4). They are the
// only keys traeger_values and traeger_deviations may carry.
const (
	traegerPhaseCopyFields = `ARRAY['name','kind','service_start_date','service_end_date','enrollment_open_at',` +
		`'enrollment_close_at','audience','care_overflow_mode','care_offering_selection_mode','is_active']`
	traegerOfferingCopyFields = `ARRAY['name','description','days_of_week_mode','available_days',` +
		`'includes_holiday_care','includes_lunch','capacity','price_cents','is_active','is_required',` +
		`'counts_as_care','selection_group','selection_rule','sort_order','pickup_times']`
)

// traegerCatalogUpSQL creates the Träger side of ADR 0195:
//
//   - traeger_phases / traeger_care_offerings hold the draft, the released
//     version and the withdrawal. They follow ADR 0105: organization_id, no
//     tenant_id, FORCE RLS on app.current_org_id, gone with the Träger.
//   - A Träger care offering keeps its Träger phase (NO ACTION, so the
//     Träger's own cascade still takes both).
//   - *_versions never change once written (trigger).
//   - *_schools name one school per row; a trigger admits only an undeleted
//     school of the row's Träger, like the host school of a holiday week.
//
// And the school side: the copy columns on enrollment.phases and
// enrollment.care_offerings. traeger_values holds the values of the applied
// version per column; a trigger recomputes traeger_deviations from the row on
// every write, so every school write path counts as a deviation and a change
// back to the Träger's value removes it. A unique index per school stops a
// second copy after a partial failure. A trigger on platform.schools drops the
// Träger link of every copy when a school changes its Träger (section 8).
const traegerCatalogUpSQL = `
CREATE TABLE enrollment.traeger_phases (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	draft JSONB NOT NULL,
	all_schools BOOLEAN NOT NULL DEFAULT false,
	released_version INT,
	withdrawn_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_traeger_phases_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_traeger_phases_draft CHECK (jsonb_typeof(draft) = 'object'),
	CONSTRAINT chk_traeger_phases_version CHECK (released_version IS NULL OR released_version >= 1)
);
COMMENT ON TABLE enrollment.traeger_phases IS
	'Enrollment phase a Träger keeps once and copies into its schools after release (#4704, ADR 0195).';

CREATE TABLE enrollment.traeger_phase_versions (
	organization_id BIGINT NOT NULL,
	traeger_phase_id BIGINT NOT NULL,
	version INT NOT NULL,
	fields JSONB NOT NULL,
	released_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	released_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	PRIMARY KEY (organization_id, traeger_phase_id, version),
	FOREIGN KEY (organization_id, traeger_phase_id)
		REFERENCES enrollment.traeger_phases (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_phase_versions_version CHECK (version >= 1),
	CONSTRAINT chk_traeger_phase_versions_fields CHECK (jsonb_typeof(fields) = 'object')
);
COMMENT ON TABLE enrollment.traeger_phase_versions IS
	'Released, immutable version of a Träger phase (#4704, ADR 0195).';

CREATE TABLE enrollment.traeger_phase_schools (
	organization_id BIGINT NOT NULL,
	traeger_phase_id BIGINT NOT NULL,
	school_id BIGINT NOT NULL,
	selected BOOLEAN NOT NULL DEFAULT true,
	applied_version INT,
	outcome VARCHAR(16),
	error_code TEXT,
	recorded_at TIMESTAMPTZ,
	PRIMARY KEY (organization_id, traeger_phase_id, school_id),
	FOREIGN KEY (organization_id, traeger_phase_id)
		REFERENCES enrollment.traeger_phases (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_phase_schools_outcome CHECK (outcome IS NULL OR outcome IN ('applied', 'rejected', 'stale', 'withdrawn')),
	CONSTRAINT chk_traeger_phase_schools_recorded CHECK ((outcome IS NULL) = (recorded_at IS NULL)),
	CONSTRAINT chk_traeger_phase_schools_error CHECK (error_code IS NULL OR char_length(error_code) <= 120)
);
COMMENT ON TABLE enrollment.traeger_phase_schools IS
	'School a Träger phase is copied into, with the last outcome of the copy there (#4704, ADR 0195).';

CREATE TABLE enrollment.traeger_care_offerings (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	traeger_phase_id BIGINT NOT NULL,
	draft JSONB NOT NULL,
	released_version INT,
	withdrawn_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT uq_traeger_care_offerings_organization UNIQUE (organization_id, id),
	CONSTRAINT fk_traeger_care_offerings_phase FOREIGN KEY (organization_id, traeger_phase_id)
		REFERENCES enrollment.traeger_phases (organization_id, id),
	CONSTRAINT chk_traeger_care_offerings_draft CHECK (jsonb_typeof(draft) = 'object'),
	CONSTRAINT chk_traeger_care_offerings_version CHECK (released_version IS NULL OR released_version >= 1)
);
CREATE INDEX idx_traeger_care_offerings_phase ON enrollment.traeger_care_offerings (organization_id, traeger_phase_id);
COMMENT ON TABLE enrollment.traeger_care_offerings IS
	'Care offering a Träger keeps once for the schools of its Träger phase, with price and contract template (#4704, ADR 0195).';

CREATE TABLE enrollment.traeger_care_offering_versions (
	organization_id BIGINT NOT NULL,
	traeger_care_offering_id BIGINT NOT NULL,
	version INT NOT NULL,
	fields JSONB NOT NULL,
	released_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	released_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	PRIMARY KEY (organization_id, traeger_care_offering_id, version),
	FOREIGN KEY (organization_id, traeger_care_offering_id)
		REFERENCES enrollment.traeger_care_offerings (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_care_offering_versions_version CHECK (version >= 1),
	CONSTRAINT chk_traeger_care_offering_versions_fields CHECK (jsonb_typeof(fields) = 'object')
);
COMMENT ON TABLE enrollment.traeger_care_offering_versions IS
	'Released, immutable version of a Träger care offering (#4704, ADR 0195).';

CREATE TABLE enrollment.traeger_care_offering_schools (
	organization_id BIGINT NOT NULL,
	traeger_care_offering_id BIGINT NOT NULL,
	school_id BIGINT NOT NULL,
	applied_version INT,
	applied_copy_id BIGINT CHECK (applied_copy_id IS NULL OR (applied_copy_id > 0 AND applied_version IS NOT NULL)),
	outcome VARCHAR(16),
	error_code TEXT,
	recorded_at TIMESTAMPTZ,
	PRIMARY KEY (organization_id, traeger_care_offering_id, school_id),
	FOREIGN KEY (organization_id, traeger_care_offering_id)
		REFERENCES enrollment.traeger_care_offerings (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_care_offering_schools_outcome CHECK (outcome IS NULL OR outcome IN ('applied', 'rejected', 'stale', 'withdrawn')),
	CONSTRAINT chk_traeger_care_offering_schools_recorded CHECK ((outcome IS NULL) = (recorded_at IS NULL)),
	CONSTRAINT chk_traeger_care_offering_schools_error CHECK (error_code IS NULL OR char_length(error_code) <= 120)
);
COMMENT ON TABLE enrollment.traeger_care_offering_schools IS
	'School a Träger care offering was copied into, with the last outcome of the copy there (#4704, ADR 0195).';

CREATE TRIGGER update_enrollment_traeger_phases_updated_at
	BEFORE UPDATE ON enrollment.traeger_phases
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER update_enrollment_traeger_care_offerings_updated_at
	BEFORE UPDATE ON enrollment.traeger_care_offerings
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

CREATE FUNCTION enrollment.refuse_traeger_version_change()
	RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog, pg_temp AS $$
	BEGIN
		-- Only the account reference may go, when the account is deleted.
		IF NEW.released_by IS NULL AND (to_jsonb(NEW) - 'released_by') = (to_jsonb(OLD) - 'released_by') THEN
			RETURN NEW;
		END IF;
		RAISE EXCEPTION 'a released Träger version never changes'
			USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_traeger_version_immutable';
	END; $$;
CREATE TRIGGER refuse_traeger_phase_version_change
	BEFORE UPDATE ON enrollment.traeger_phase_versions
	FOR EACH ROW EXECUTE FUNCTION enrollment.refuse_traeger_version_change();
CREATE TRIGGER refuse_traeger_care_offering_version_change
	BEFORE UPDATE ON enrollment.traeger_care_offering_versions
	FOR EACH ROW EXECUTE FUNCTION enrollment.refuse_traeger_version_change();

CREATE FUNCTION enrollment.check_traeger_catalog_school()
	RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog, pg_temp AS $$
	BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM platform.schools AS school
			WHERE school.id = NEW.school_id
				AND school.organization_id = NEW.organization_id
				AND school.deleted_at IS NULL
		) THEN
			RAISE EXCEPTION 'school % is not a school of Träger %', NEW.school_id, NEW.organization_id
				USING ERRCODE = 'check_violation', CONSTRAINT = 'chk_traeger_catalog_school';
		END IF;
		RETURN NEW;
	END; $$;
CREATE TRIGGER check_traeger_phase_school
	BEFORE INSERT OR UPDATE OF school_id, organization_id ON enrollment.traeger_phase_schools
	FOR EACH ROW EXECUTE FUNCTION enrollment.check_traeger_catalog_school();
CREATE TRIGGER check_traeger_care_offering_school
	BEFORE INSERT OR UPDATE OF school_id, organization_id ON enrollment.traeger_care_offering_schools
	FOR EACH ROW EXECUTE FUNCTION enrollment.check_traeger_catalog_school();

ALTER TABLE enrollment.traeger_phases ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.traeger_phases FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_traeger_phases ON enrollment.traeger_phases FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.traeger_phase_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.traeger_phase_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_traeger_phase_versions ON enrollment.traeger_phase_versions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.traeger_phase_schools ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.traeger_phase_schools FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_traeger_phase_schools ON enrollment.traeger_phase_schools FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.traeger_care_offerings ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.traeger_care_offerings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_traeger_care_offerings ON enrollment.traeger_care_offerings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.traeger_care_offering_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.traeger_care_offering_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_traeger_care_offering_versions ON enrollment.traeger_care_offering_versions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.traeger_care_offering_schools ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.traeger_care_offering_schools FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_traeger_care_offering_schools ON enrollment.traeger_care_offering_schools FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.traeger_phases, enrollment.traeger_phase_schools,
	enrollment.traeger_care_offerings, enrollment.traeger_care_offering_schools TO phoenix_tenant;
GRANT SELECT, INSERT ON enrollment.traeger_phase_versions, enrollment.traeger_care_offering_versions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.traeger_phases_id_seq, enrollment.traeger_care_offerings_id_seq TO phoenix_tenant;
GRANT ALL ON enrollment.traeger_phases, enrollment.traeger_phase_versions, enrollment.traeger_phase_schools,
	enrollment.traeger_care_offerings, enrollment.traeger_care_offering_versions,
	enrollment.traeger_care_offering_schools TO phoenix_admin;

ALTER TABLE enrollment.phases
	ADD COLUMN traeger_source_id BIGINT,
	ADD COLUMN traeger_source_version INT,
	ADD COLUMN traeger_values JSONB,
	ADD COLUMN traeger_deviations TEXT[] NOT NULL DEFAULT '{}',
	ADD CONSTRAINT chk_phases_traeger_source CHECK (
		num_nonnulls(traeger_source_id, traeger_source_version, traeger_values) IN (0, 3)),
	ADD CONSTRAINT chk_phases_traeger_fields CHECK (
		(traeger_values IS NULL OR (jsonb_typeof(traeger_values) = 'object'
			AND traeger_values - ` + traegerPhaseCopyFields + ` = '{}'::jsonb))
		AND traeger_deviations <@ ` + traegerPhaseCopyFields + `);
CREATE UNIQUE INDEX uq_phases_traeger_source ON enrollment.phases (tenant_id, traeger_source_id)
	WHERE traeger_source_id IS NOT NULL;

ALTER TABLE enrollment.care_offerings
	ADD COLUMN traeger_source_id BIGINT,
	ADD COLUMN traeger_source_version INT,
	ADD COLUMN traeger_values JSONB,
	ADD COLUMN traeger_deviations TEXT[] NOT NULL DEFAULT '{}',
	ADD CONSTRAINT chk_care_offerings_traeger_source CHECK (
		num_nonnulls(traeger_source_id, traeger_source_version, traeger_values) IN (0, 3)),
	ADD CONSTRAINT chk_care_offerings_traeger_fields CHECK (
		(traeger_values IS NULL OR (jsonb_typeof(traeger_values) = 'object'
			AND traeger_values - ` + traegerOfferingCopyFields + ` = '{}'::jsonb))
		AND traeger_deviations <@ ` + traegerOfferingCopyFields + `);
CREATE UNIQUE INDEX uq_care_offerings_traeger_source ON enrollment.care_offerings (tenant_id, phase_id, traeger_source_id)
	WHERE traeger_source_id IS NOT NULL;

-- Recomputes the deviations of a copy from the row itself. traeger_values is
-- overlaid on the row through the row type, so both sides are compared in
-- the canonical JSON of their column type.
CREATE FUNCTION enrollment.track_traeger_deviations()
	RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog, pg_temp AS $$
	DECLARE
		traeger JSONB;
		current_row JSONB;
	BEGIN
		IF NEW.traeger_source_id IS NULL THEN
			NEW.traeger_source_version := NULL;
			NEW.traeger_values := NULL;
			NEW.traeger_deviations := '{}';
			RETURN NEW;
		END IF;
		traeger := to_jsonb(jsonb_populate_record(NEW, NEW.traeger_values));
		current_row := to_jsonb(NEW);
		SELECT COALESCE(jsonb_object_agg(field, traeger -> field), '{}'::jsonb),
			COALESCE(array_agg(field ORDER BY field) FILTER (WHERE current_row -> field IS DISTINCT FROM traeger -> field), '{}')
			INTO NEW.traeger_values, NEW.traeger_deviations
			FROM jsonb_object_keys(NEW.traeger_values) AS field;
		RETURN NEW;
	END; $$;
CREATE TRIGGER track_traeger_deviations
	BEFORE INSERT OR UPDATE ON enrollment.phases
	FOR EACH ROW EXECUTE FUNCTION enrollment.track_traeger_deviations();
CREATE TRIGGER track_traeger_deviations
	BEFORE INSERT OR UPDATE ON enrollment.care_offerings
	FOR EACH ROW EXECUTE FUNCTION enrollment.track_traeger_deviations();

-- A school that changes its Träger keeps its copies as ordinary phases and
-- offerings (ADR 0195 section 8).
CREATE FUNCTION enrollment.release_traeger_copies()
	RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog, pg_temp AS $$
	BEGIN
		UPDATE enrollment.care_offerings SET traeger_source_id = NULL
			WHERE tenant_id = NEW.id AND traeger_source_id IS NOT NULL;
		UPDATE enrollment.phases SET traeger_source_id = NULL
			WHERE tenant_id = NEW.id AND traeger_source_id IS NOT NULL;
		RETURN NULL;
	END; $$;
CREATE TRIGGER release_traeger_copies
	AFTER UPDATE OF organization_id ON platform.schools
	FOR EACH ROW WHEN (OLD.organization_id IS DISTINCT FROM NEW.organization_id)
	EXECUTE FUNCTION enrollment.release_traeger_copies();
`

const traegerCatalogDownSQL = `
DROP TRIGGER release_traeger_copies ON platform.schools;
DROP FUNCTION enrollment.release_traeger_copies();
DROP TRIGGER track_traeger_deviations ON enrollment.care_offerings;
DROP TRIGGER track_traeger_deviations ON enrollment.phases;
DROP FUNCTION enrollment.track_traeger_deviations();
DROP INDEX enrollment.uq_care_offerings_traeger_source;
ALTER TABLE enrollment.care_offerings
	DROP CONSTRAINT chk_care_offerings_traeger_fields,
	DROP CONSTRAINT chk_care_offerings_traeger_source,
	DROP COLUMN traeger_deviations, DROP COLUMN traeger_values,
	DROP COLUMN traeger_source_version, DROP COLUMN traeger_source_id;
DROP INDEX enrollment.uq_phases_traeger_source;
ALTER TABLE enrollment.phases
	DROP CONSTRAINT chk_phases_traeger_fields,
	DROP CONSTRAINT chk_phases_traeger_source,
	DROP COLUMN traeger_deviations, DROP COLUMN traeger_values,
	DROP COLUMN traeger_source_version, DROP COLUMN traeger_source_id;
DROP TABLE enrollment.traeger_care_offering_schools;
DROP TABLE enrollment.traeger_care_offering_versions;
DROP TABLE enrollment.traeger_care_offerings;
DROP TABLE enrollment.traeger_phase_schools;
DROP TABLE enrollment.traeger_phase_versions;
DROP TABLE enrollment.traeger_phases;
DROP FUNCTION enrollment.check_traeger_catalog_school();
DROP FUNCTION enrollment.refuse_traeger_version_change();
`

func traegerCatalogUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerCatalogUpSQL); err != nil {
		return fmt.Errorf("traeger catalog: %w", err)
	}
	return nil
}

func traegerCatalogDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerCatalogDownSQL); err != nil {
		return fmt.Errorf("traeger catalog rollback: %w", err)
	}
	return nil
}
