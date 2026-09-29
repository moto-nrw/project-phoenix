package test

import (
	"context"
	_ "embed"
	"testing"

	"github.com/uptrace/bun"
)

//go:embed testdata/student_storage_before_contract.sql
var studentStorageBeforeContract string

//go:embed testdata/student_compatibility_398.sql
var studentCompatibilityBeforeContract string

// RestoreStudentCompatibilityBeforeContract restores the frozen 1.15.398
// rollback shape only inside a disposable historical application-test clone.
// It is not a production recovery path and never copies current owner data.
func RestoreStudentCompatibilityBeforeContract(tb testing.TB, db *bun.DB) {
	tb.Helper()
	requireIsolatedStudentStorage(tb, db)
	var missing bool
	if err := db.NewRaw(`SELECT to_regclass('users.students') IS NULL
		AND to_regclass('users.students_legacy') IS NULL`).Scan(context.Background(), &missing); err != nil {
		tb.Fatalf("inspect retired student compatibility storage: %v", err)
	}
	if !missing {
		tb.Fatal("historical compatibility fixture requires contracted storage")
	}
	if _, err := db.ExecContext(context.Background(), studentStorageBeforeContract+studentCompatibilityBeforeContract); err != nil {
		tb.Fatalf("restore historical student compatibility schema: %v", err)
	}
}

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
	restoreGuardianStorageIfCutOver(tb, db)
	var archiveMissing bool
	if err := db.NewRaw(`SELECT to_regclass('users.students_legacy') IS NULL`).Scan(context.Background(), &archiveMissing); err != nil {
		tb.Fatalf("inspect historical student archive: %v", err)
	}
	if archiveMissing {
		if _, err := db.ExecContext(context.Background(), studentStorageBeforeContract); err != nil {
			tb.Fatalf("restore historical student archive schema: %v", err)
		}
	}
	if _, err := db.ExecContext(context.Background(), `
		DROP VIEW IF EXISTS users.expired_privacy_consents;
  DROP VIEW IF EXISTS users.students;
  -- Return to the expand schema as well as the pre-cutover table. The
  -- absence columns were added only by the application-owner switch.
  ALTER TABLE users.student_care_profiles
   DROP COLUMN IF EXISTS sick, DROP COLUMN IF EXISTS sick_since,
   DROP COLUMN IF EXISTS excused, DROP COLUMN IF EXISTS excused_since;
		DROP FUNCTION IF EXISTS users.route_student_compatibility();
		DROP SEQUENCE IF EXISTS users.student_compatibility_reads, users.student_compatibility_writes;
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
				  -- Tables created AFTER the cutover never referenced
				  -- users.students, so moving their keys onto it would restore
				  -- a shape that never existed — and the cutover's own guard
				  -- would then rightly refuse a key its static list does not
				  -- name. Their key is dropped below instead.
				  AND con.conrelid <> to_regclass('users.student_notes')
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
		-- CASCADE, because the owner tables are emptied while the post-cutover
		-- tables above still reference the profile. Those did not exist before
		-- the cutover, so a restored historical state holds none of their rows
		-- either; every other student reference was repointed onto
		-- users.students a few lines up and is untouched by this.
		-- The Kindnotizen came long after the cutover, so the restored history
		-- holds neither their rows nor their key. Keeping the key would block
		-- the expand migration's own rollback, which drops the profile table;
		-- repointing it onto users.students would invent a shape that never
		-- existed and trip the cutover's static key list.
		ALTER TABLE IF EXISTS users.student_notes
			DROP CONSTRAINT IF EXISTS fk_student_notes_student;
		TRUNCATE users.student_care_profiles, users.student_school_memberships, users.student_profiles CASCADE;
		TRUNCATE users.student_notes;
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
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatal("restore student storage before cutover requires this test's isolated database")
	}
	var relkind string
	if err := db.NewRaw(`SELECT coalesce((SELECT relkind::text FROM pg_class WHERE oid = to_regclass('users.students')), '')`).
		Scan(context.Background(), &relkind); err != nil {
		tb.Fatalf("restore student storage before cutover: inspect users.students: %v", err)
	}
	if relkind != "v" && relkind != "" {
		tb.Fatalf("restore student storage before cutover: users.students is already a base table (relkind %q)", relkind)
	}
}
