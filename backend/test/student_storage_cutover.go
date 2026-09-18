package test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// RestoreStudentStorageBeforeCutover turns users.students back into the
// authoritative base table it was before migration 1.15.397 (#2759).
//
// Contracts written for the Expand (#2717) and Backfill (#2758) migrations, and
// for schemas older than either, describe a world in which users.students holds
// the rows. The cutover replaced it with a rollback-only view over
// users.student_profiles, users.student_school_memberships and
// users.student_care_profiles, so those tests restore their world inside their
// own disposable clone first. Production rollback deliberately keeps the
// compatibility shape instead of reversing it.
//
// It requires a per-test database (SetupIsolatedTestDB, or a package that opted
// into PerTestDatabases): it changes the schema and empties the three owner
// tables, which is exactly the post-Expand state the callers expect.
func RestoreStudentStorageBeforeCutover(tb testing.TB, db *bun.DB) {
	tb.Helper()
	requireIsolatedStudentStorage(tb, db)
	if _, err := db.ExecContext(context.Background(), `
		DROP VIEW users.expired_privacy_consents;
		DROP VIEW users.students;
		DROP FUNCTION users.route_student_compatibility();
		DROP SEQUENCE users.student_compatibility_reads, users.student_compatibility_writes;
		DROP TRIGGER update_student_profiles_updated_at ON users.student_profiles;
		DROP TRIGGER update_student_school_memberships_updated_at ON users.student_school_memberships;
		DROP TRIGGER update_student_care_profiles_updated_at ON users.student_care_profiles;
		ALTER TABLE users.students_legacy RENAME TO students;
		DO $$
		DECLARE constraint_row RECORD;
		        previous_path text := current_setting('search_path');
		        definition text;
		BEGIN
			-- pg_get_constraintdef omits the schema of anything the search path
			-- already resolves, so the rewrite below would miss a session that
			-- has users on its path.
			PERFORM set_config('search_path', 'pg_catalog', true);
			FOR constraint_row IN
				SELECT con.conname, con.conrelid::regclass::text AS child,
					pg_get_constraintdef(con.oid) AS def
				FROM pg_constraint AS con
				WHERE con.confrelid = 'users.student_profiles'::regclass AND con.contype = 'f'
				  -- The membership's own link to its profile is owner storage,
				  -- not a reference the cutover moved off users.students.
				  AND con.conrelid <> 'users.student_school_memberships'::regclass
				ORDER BY con.conrelid::regclass::text, con.conname
			LOOP
				definition := replace(constraint_row.def,
					'REFERENCES users.student_profiles(', 'REFERENCES users.students(');
				EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I',
					constraint_row.child, constraint_row.conname);
				-- NOT VALID keeps the restore off every dependent table's
				-- rows. The referential actions the historical contracts rely
				-- on — the delete cascades above all — are installed either
				-- way; only the scan of rows that already exist is skipped,
				-- and the clone is emptied of students right below.
				EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s NOT VALID',
					constraint_row.child, constraint_row.conname, definition);
			END LOOP;
			PERFORM set_config('search_path', previous_path, true);
		END $$;
		CREATE VIEW users.expired_privacy_consents WITH (security_invoker = true) AS
		SELECT pc.id, pc.student_id, pc.policy_version, pc.accepted, pc.accepted_at,
			pc.expires_at, pc.duration_days, pc.renewal_required, pc.data_retention_days,
			pc.details, pc.created_at, pc.updated_at, pc.tenant_id, s.person_id,
			s.guardian_name, s.guardian_email, s.guardian_phone
		FROM users.privacy_consents pc
		JOIN users.students s ON pc.student_id = s.id
		WHERE pc.expires_at < CURRENT_TIMESTAMP AND pc.accepted = true AND pc.renewal_required = true;
		TRUNCATE users.student_care_profiles, users.student_school_memberships, users.student_profiles;
		DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = 'student-owner';
	`); err != nil {
		tb.Fatalf("restore student storage before cutover: %v", err)
	}
}

// requireIsolatedStudentStorage refuses to rewrite a database other tests share.
func requireIsolatedStudentStorage(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if db == nil {
		tb.Fatal("restore student storage before cutover: database is required")
	}
	var relkind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students'::regclass`).
		Scan(context.Background(), &relkind); err != nil {
		tb.Fatalf("restore student storage before cutover: inspect users.students: %v", err)
	}
	if relkind != "v" {
		tb.Fatalf("restore student storage before cutover: users.students is already a base table (relkind %q)", relkind)
	}
}
