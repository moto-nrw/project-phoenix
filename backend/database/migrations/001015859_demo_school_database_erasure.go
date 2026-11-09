package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoSchoolDatabaseErasureVersion = "1.15.859"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoSchoolDatabaseErasureVersion,
		Description: "Demo-Betrieb: ausgemusterte Demo-Schulen 90 Tage nach dem Stilllegen ganz aus der Datenbank löschen (#5559)",
		DependsOn: []string{
			parentBillingOnlineSEPAMandatesVersion, // the last migration before this one
			demoSchoolFileErasuresVersion,          // the marker and the demo origin this one reads
			schoolErasureVersion,                   // the preflight and deletion it reuses
		},
	})
	Migrations.MustRegister(demoSchoolDatabaseErasureUp, demoSchoolDatabaseErasureDown)
}

// demoSchoolDatabaseErasureUpSQL lets the demo process erase a retired demo
// school from the database (#5559, ADR 0189). It reuses the school erasure's
// preflight and deletion (ADR 0092), not its operator order: the demo has no
// customer school and no client.
//
// A school is due when all of these hold:
//   - it has a demo origin (platform.demo_school_origins, ADR 0148): neither a
//     name nor an e-mail pattern nor a soft delete proves a demo school;
//   - its uploads are removed and verified empty (the #5544 file marker), so
//     the database erasure never takes away work the file phase still needs;
//   - it was retired at least 90 days ago: sign-in events of a demo visit keep
//     their 90 days (#5543), and the school erasure deletes them with the
//     school;
//   - no demo order names it any more, which excludes the standing school, a
//     school still being seeded and a school whose visitor is not erased yet.
//
// The evidence is two columns on the file marker: the time and the deleted
// row count per table, no names. The marker has no foreign key and outlives
// the school (ADR 0148).
//
// Each call erases one school in its own statement and so in its own
// transaction. Every deletion runs in a subtransaction; a failure rolls all
// of it back and returns only the SQLSTATE, never a message that could carry
// a personal value. The next run retries. lock_timeout bounds each lock wait
// to 100 ms like the rest of the hourly demo run; a school whose locks are
// not free in time is "busy" and simply waits for the next run.
const demoSchoolDatabaseErasureUpSQL = `
	ALTER TABLE platform.demo_school_file_erasures
		ADD COLUMN db_erased_at TIMESTAMPTZ,
		ADD COLUMN db_row_counts JSONB,
		ADD CONSTRAINT demo_school_file_erasures_db_evidence CHECK (
			(db_erased_at IS NULL AND db_row_counts IS NULL)
			OR (db_erased_at IS NOT NULL AND jsonb_typeof(db_row_counts) = 'object'));
	COMMENT ON COLUMN platform.demo_school_file_erasures.db_erased_at IS
		'Demo process only (#5559): the school was erased from the database at this time. Set only by platform.erase_retired_demo_school.';
	COMMENT ON COLUMN platform.demo_school_file_erasures.db_row_counts IS
		'Deleted rows per table of the database erasure (#5559). Counts only, no values.';
	-- phoenix_demo inserts file markers; it must not claim a database erasure.
	CREATE FUNCTION platform.demo_school_file_erasure_starts_without_db_evidence()
	RETURNS TRIGGER LANGUAGE plpgsql SET search_path = pg_catalog
	AS $body$
	BEGIN
		IF NEW.db_erased_at IS NOT NULL OR NEW.db_row_counts IS NOT NULL THEN
			RAISE EXCEPTION 'a file marker starts without database erasure evidence' USING ERRCODE = '42501';
		END IF;
		RETURN NEW;
	END $body$;
	REVOKE ALL ON FUNCTION platform.demo_school_file_erasure_starts_without_db_evidence() FROM PUBLIC;
	CREATE TRIGGER demo_school_file_erasures_start_without_db_evidence
		BEFORE INSERT ON platform.demo_school_file_erasures
		FOR EACH ROW EXECUTE FUNCTION platform.demo_school_file_erasure_starts_without_db_evidence();

	CREATE FUNCTION platform.retired_demo_school_database_due(p_school_id BIGINT)
	RETURNS BOOLEAN LANGUAGE sql VOLATILE SECURITY INVOKER SET search_path = pg_catalog
	AS $body$
		SELECT EXISTS (
			SELECT 1
			FROM platform.schools AS school
			JOIN platform.demo_school_origins AS origin ON origin.school_id = school.id
			JOIN platform.demo_school_file_erasures AS files ON files.school_id = school.id
			WHERE school.id = p_school_id
				AND files.db_erased_at IS NULL
				AND school.deleted_at < clock_timestamp() - INTERVAL '90 days'
				AND NOT EXISTS (SELECT 1 FROM platform.demo_school_states AS state
					WHERE state.tenant_id = school.id OR school.id = ANY(state.visitor_school_ids)))
	$body$;
	REVOKE ALL ON FUNCTION platform.retired_demo_school_database_due(BIGINT) FROM PUBLIC;

	CREATE FUNCTION platform.retired_demo_schools_due_for_database_erasure(p_limit INTEGER)
	RETURNS SETOF BIGINT LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
	AS $body$
		-- Random order: a school that keeps failing never holds up the others.
		SELECT files.school_id
		FROM platform.demo_school_file_erasures AS files
		WHERE platform.retired_demo_school_database_due(files.school_id)
		ORDER BY random()
		LIMIT LEAST(GREATEST(p_limit, 0), 50)
	$body$;

	CREATE FUNCTION platform.erase_retired_demo_school(p_school_id BIGINT)
	RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY DEFINER
	SET search_path = pg_catalog SET lock_timeout = '100ms'
	AS $body$
	DECLARE
		v_organization_id BIGINT;
		v_counts JSONB;
		v_error TEXT;
	BEGIN
		IF p_school_id IS NULL OR p_school_id <= 0 THEN
			RAISE EXCEPTION 'demo school erasure requires a school' USING ERRCODE = '22023';
		END IF;
		-- The same key as the operator's school erasure: never both at once.
		IF NOT pg_try_advisory_xact_lock(hashtextextended('school-erasure:' || p_school_id::text, 0)) THEN
			RETURN jsonb_build_object('status', 'busy');
		END IF;
		BEGIN
			-- The row lock keeps the school retired until the deletion ends.
			SELECT school.organization_id INTO v_organization_id
			FROM platform.schools AS school
			WHERE school.id = p_school_id AND platform.retired_demo_school_database_due(p_school_id)
			FOR UPDATE;
			IF NOT FOUND THEN
				RETURN jsonb_build_object('status', 'not_due');
			END IF;
			PERFORM platform.school_erasure_preflight(p_school_id, v_organization_id);
			-- The preflight now holds every school table, demo orders included:
			-- check again that no order named the school in the meantime.
			IF NOT platform.retired_demo_school_database_due(p_school_id) THEN
				RETURN jsonb_build_object('status', 'not_due');
			END IF;
			v_counts := platform.school_erasure_delete(p_school_id);
			UPDATE platform.demo_school_file_erasures
			SET db_erased_at = clock_timestamp(), db_row_counts = v_counts
			WHERE school_id = p_school_id;
		EXCEPTION
			WHEN lock_not_available THEN
				-- Another transaction holds a school table: try again next run.
				RETURN jsonb_build_object('status', 'busy');
			WHEN OTHERS OR query_canceled THEN
				GET STACKED DIAGNOSTICS v_error = RETURNED_SQLSTATE;
				RETURN jsonb_build_object('status', 'failed', 'error_code', v_error);
		END;
		RETURN jsonb_build_object('status', 'erased', 'row_counts', v_counts);
	END $body$;

	REVOKE ALL ON FUNCTION platform.retired_demo_schools_due_for_database_erasure(INTEGER),
		platform.erase_retired_demo_school(BIGINT)
		FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_school_erasure;
	GRANT EXECUTE ON FUNCTION platform.retired_demo_schools_due_for_database_erasure(INTEGER),
		platform.erase_retired_demo_school(BIGINT) TO phoenix_demo;
`

func demoSchoolDatabaseErasureUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, demoSchoolDatabaseErasureUpSQL); err != nil {
			return fmt.Errorf("install the database erasure of retired demo schools: %w", err)
		}
		return nil
	})
}

// demoSchoolDatabaseErasureDown refuses once a school is erased: the rows are
// gone, and dropping the columns would discard the only evidence.
func demoSchoolDatabaseErasureDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var erased int
		if err := tx.NewRaw(`SELECT count(*) FROM platform.demo_school_file_erasures WHERE db_erased_at IS NOT NULL`).Scan(ctx, &erased); err != nil {
			return err
		}
		if erased != 0 {
			return fmt.Errorf("demo school database erasure rollback would discard evidence; use a forward migration")
		}
		if _, err := tx.ExecContext(ctx, `
			DROP FUNCTION IF EXISTS platform.erase_retired_demo_school(BIGINT);
			DROP FUNCTION IF EXISTS platform.retired_demo_schools_due_for_database_erasure(INTEGER);
			DROP FUNCTION IF EXISTS platform.retired_demo_school_database_due(BIGINT);
			DROP TRIGGER IF EXISTS demo_school_file_erasures_start_without_db_evidence ON platform.demo_school_file_erasures;
			DROP FUNCTION IF EXISTS platform.demo_school_file_erasure_starts_without_db_evidence();
			ALTER TABLE platform.demo_school_file_erasures
				DROP CONSTRAINT IF EXISTS demo_school_file_erasures_db_evidence,
				DROP COLUMN IF EXISTS db_row_counts,
				DROP COLUMN IF EXISTS db_erased_at`); err != nil {
			return fmt.Errorf("drop the database erasure of retired demo schools: %w", err)
		}
		return nil
	})
}
