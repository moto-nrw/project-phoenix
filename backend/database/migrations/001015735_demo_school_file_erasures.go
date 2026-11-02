package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoSchoolFileErasuresVersion = "1.15.735"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoSchoolFileErasuresVersion,
		Description: "Demo-Betrieb: Vermerk, dass die Dateien einer ausgemusterten Demo-Schule gelöscht sind (#5544)",
		DependsOn: []string{
			payrollWageTypesVersion,   // the last migration before this one
			"1.15.403",                // the demo role this grants to
			demoVisitorErasureVersion, // visitor_school_ids, recorded as demo origin
		},
	})
	Migrations.MustRegister(demoSchoolFileErasuresUp, demoSchoolFileErasuresDown)
}

// demoSchoolFileErasuresUpSQL records, per retired demo school, that the
// demo process removed every upload namespace of the school and counted none
// left (#5544). The row is the work marker: a retired school without one is
// still to be done, so a failed run leaves no row and the next run retries.
// file_counts holds the number of objects removed per namespace, never a
// file name. Only the demo process reads and writes the table; it never
// updates or deletes a row.
//
// school_id has no foreign key on purpose: the school erasure fingerprints
// every incoming foreign key of platform.schools and refuses to run when one
// is added without a reviewed plan (school_erasure_preflight). The marker
// holds no personal data; a marker whose school is gone names nobody.
//
// platform.demo_school_origins proves that a school is a demo school: the
// visitor erasure (ADR 0062) deletes the order row, so the order cannot be
// the proof an hour later. A trigger records every school an order names,
// current or given up, the moment the demo control plane writes it; no
// caller can add a row. A soft-deleted school without that origin, such as a
// real school restored into the demo database, is never selected.
const demoSchoolFileErasuresUpSQL = `
	CREATE TABLE platform.demo_school_file_erasures (
		school_id BIGINT PRIMARY KEY,
		erased_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		file_counts JSONB NOT NULL CHECK (jsonb_typeof(file_counts) = 'object')
	);
	COMMENT ON TABLE platform.demo_school_file_erasures IS
		'Demo process only (#5544): the uploads of this retired demo school are removed and verified empty. Counts per namespace, no file names.';
	REVOKE ALL ON platform.demo_school_file_erasures FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
	GRANT SELECT, INSERT ON platform.demo_school_file_erasures TO phoenix_demo;

	CREATE TABLE platform.demo_school_origins (
		school_id BIGINT PRIMARY KEY,
		recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	COMMENT ON TABLE platform.demo_school_origins IS
		'Demo origin (#5544): the demo control plane created or used this school for a demo order. Recorded by trigger, never by a caller; only these schools get their uploads erased.';
	REVOKE ALL ON platform.demo_school_origins FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
	GRANT SELECT ON platform.demo_school_origins TO phoenix_demo;

	CREATE FUNCTION platform.record_demo_school_origin()
	RETURNS TRIGGER
	LANGUAGE plpgsql
	SECURITY DEFINER
	SET search_path = pg_catalog
	AS $$
	BEGIN
		INSERT INTO platform.demo_school_origins (school_id)
		SELECT DISTINCT school_id
		FROM unnest(array_remove(NEW.visitor_school_ids || ARRAY[NEW.tenant_id], NULL)) AS school_id
		ON CONFLICT (school_id) DO NOTHING;
		RETURN NULL;
	END;
	$$;
	REVOKE ALL ON FUNCTION platform.record_demo_school_origin() FROM PUBLIC;
	CREATE TRIGGER demo_school_states_record_origin
		AFTER INSERT OR UPDATE OF tenant_id, visitor_school_ids ON platform.demo_school_states
		FOR EACH ROW EXECUTE FUNCTION platform.record_demo_school_origin();

	-- Backfill: every school an order still names. Schools of orders already
	-- erased (#4048) have no provable demo origin and are left alone.
	INSERT INTO platform.demo_school_origins (school_id)
	SELECT DISTINCT school_id
	FROM platform.demo_school_states AS state,
		unnest(array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL)) AS school_id
	ON CONFLICT (school_id) DO NOTHING;
`

func demoSchoolFileErasuresUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoSchoolFileErasuresUpSQL); err != nil {
		return fmt.Errorf("create the file erasure marker of retired demo schools: %w", err)
	}
	return nil
}

func demoSchoolFileErasuresDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS demo_school_states_record_origin ON platform.demo_school_states;
		DROP FUNCTION IF EXISTS platform.record_demo_school_origin();
		DROP TABLE IF EXISTS platform.demo_school_origins;
		DROP TABLE IF EXISTS platform.demo_school_file_erasures`); err != nil {
		return fmt.Errorf("drop the file erasure marker of retired demo schools: %w", err)
	}
	return nil
}
