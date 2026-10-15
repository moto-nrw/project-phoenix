package test

import (
	"context"
	_ "embed"
	"testing"

	"github.com/uptrace/bun"
)

//go:embed testdata/guardian_storage_before_contract.sql
var guardianStorageBeforeContract string

// RestoreGuardianStorageBeforeContract rebuilds the rollback-only mirror the
// Contract (#2757) removed: the frozen users.students_guardians schema with its
// routing, single-primary and mirror triggers and the write counter, filled
// from the joined owner rows. Contracts written for the Cutover (#2756) need
// that world; production never reverses the Contract.
//
// It requires a per-test database (SetupIsolatedTestDB, or a package that
// opted into PerTestDatabases).
func RestoreGuardianStorageBeforeContract(tb testing.TB, db *bun.DB) {
	tb.Helper()
	requireIsolatedGuardianRestore(tb, db, "restore guardian storage before contract")
	restoreGuardianMirror(tb, db)
}

func requireIsolatedGuardianRestore(tb testing.TB, db *bun.DB, action string) {
	tb.Helper()
	if db == nil {
		tb.Fatalf("%s: database is required", action)
	}
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatalf("%s requires this test's isolated database", action)
	}
}

func restoreGuardianMirror(tb testing.TB, db *bun.DB) {
	tb.Helper()
	var contracted bool
	if err := db.NewRaw(`SELECT to_regclass('users.students_guardians') IS NULL
		AND to_regclass('users.students_guardians_compatibility_writes') IS NULL`).
		Scan(context.Background(), &contracted); err != nil {
		tb.Fatalf("restore guardian mirror: inspect schema: %v", err)
	}
	if !contracted {
		tb.Fatal("restore guardian mirror requires the contracted schema")
	}
	// The mirror marker keeps the routing trigger from copying the restored
	// rows back into the owners they come from.
	if err := db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, guardianStorageBeforeContract); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			SELECT set_config('users.guardian_compat_mirror', 'on', true);
			INSERT INTO users.students_guardians
				(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
				 is_primary, is_emergency_contact, emergency_priority, is_payer,
				 can_pickup, pickup_notes, permissions, created_at, updated_at)
			SELECT r.id, r.tenant_id, r.student_id, r.guardian_profile_id, r.relationship_type, r.guardian_role,
				r.is_primary, r.is_emergency_contact, r.emergency_priority, r.is_payer,
				coalesce(p.can_pickup, false), p.pickup_notes, coalesce(a.permissions, '{}'::jsonb), r.created_at, r.updated_at
			FROM users.student_guardian_relationships AS r
			LEFT JOIN users.student_guardian_pickup_permissions AS p ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
			LEFT JOIN auth.guardian_student_access AS a ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id;
			SELECT setval('users.students_guardians_id_seq',
				(SELECT last_value FROM users.student_guardian_relationships_id_seq), true);
			SELECT set_config('users.guardian_compat_mirror', 'off', true);`)
		return err
	}); err != nil {
		tb.Fatalf("restore guardian mirror: %v", err)
	}
}

// RestoreGuardianStorageBeforeCutover turns users.students_guardians back into
// the authoritative storage it was before migration 1.15.417 (#2756).
//
// Contracts written for the Expand (#2716) and Backfill (#2755) migrations
// describe a world in which users.students_guardians holds the rows and the
// three owner tables are empty or trail it. The cutover made the owners
// authoritative and left the old table as a rollback-only mirror kept by
// triggers, so those tests restore their world inside their own disposable
// clone first: the mirror already holds every row, so dropping the triggers,
// the counter and the targets and moving the dependent objects back is the
// whole restore once RestoreGuardianStorageBeforeContract has rebuilt the
// mirror the Contract (#2757) removed. Production never reverses either
// migration.
//
// It requires a per-test database (SetupIsolatedTestDB, or a package that
// opted into PerTestDatabases).
func RestoreGuardianStorageBeforeCutover(tb testing.TB, db *bun.DB) {
	tb.Helper()
	requireIsolatedGuardianRestore(tb, db, "restore guardian storage before cutover")
	restoreGuardianMirror(tb, db)
	restoreGuardianStorage(tb, db)
	if _, err := db.ExecContext(context.Background(),
		`DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = 'guardian-owner'`); err != nil {
		tb.Fatalf("restore guardian storage before cutover: reset checkpoints: %v", err)
	}
}

// restoreGuardianStorageIfCutOver is the guardian half of a restore to a
// schema older than 1.15.417: a world before the student cutover is also a
// world before the guardian cutover. Unlike the guardian contracts, the older
// contracts keep the backfill checkpoints the migration ladder wrote, as they
// found them before the cutover existed.
func restoreGuardianStorageIfCutOver(tb testing.TB, db *bun.DB) {
	tb.Helper()
	var cutOver bool
	if err := db.NewRaw(`SELECT to_regclass('users.students_guardians') IS NULL OR EXISTS (SELECT 1 FROM pg_trigger
		WHERE tgname = 'students_guardians_route_compatibility' AND tgrelid = to_regclass('users.students_guardians'))`).
		Scan(context.Background(), &cutOver); err != nil {
		tb.Fatalf("restore guardian storage before cutover: inspect schema: %v", err)
	}
	if !cutOver {
		return
	}
	var contracted bool
	if err := db.NewRaw(`SELECT to_regclass('users.students_guardians') IS NULL`).Scan(context.Background(), &contracted); err != nil {
		tb.Fatalf("restore guardian storage before cutover: inspect schema: %v", err)
	}
	if contracted {
		restoreGuardianMirror(tb, db)
	}
	restoreGuardianStorage(tb, db)
}

func restoreGuardianStorage(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if _, err := db.ExecContext(context.Background(), `
		DROP TRIGGER students_guardians_route_compatibility ON users.students_guardians;
		DROP TRIGGER student_guardian_relationships_mirror ON users.student_guardian_relationships;
		DROP TRIGGER student_guardian_pickup_permissions_mirror ON users.student_guardian_pickup_permissions;
		DROP TRIGGER guardian_student_access_mirror ON auth.guardian_student_access;
		DROP TRIGGER guardian_profiles_bind_student_access ON users.guardian_profiles;
		DROP TRIGGER update_student_guardian_relationships_updated_at ON users.student_guardian_relationships;
		DROP TRIGGER update_student_guardian_pickup_permissions_updated_at ON users.student_guardian_pickup_permissions;
		DROP TRIGGER update_guardian_student_access_updated_at ON auth.guardian_student_access;
		DROP TRIGGER invalidate_parent_student_consent_permission_grant ON auth.guardian_student_access;
		DROP TRIGGER invalidate_meal_participation_permission_grant ON auth.guardian_student_access;
		DROP FUNCTION users.route_students_guardians_compatibility();
		DROP FUNCTION users.mirror_student_guardian_relationship();
		DROP FUNCTION users.mirror_student_guardian_pickup_permission();
		DROP FUNCTION auth.mirror_guardian_student_access();
		DROP FUNCTION auth.bind_guardian_student_access_account();
		DROP FUNCTION meta.invalidate_parent_student_consent_access_grant();
		DROP FUNCTION meta.invalidate_meal_participation_access_grant();
		DROP SEQUENCE users.students_guardians_compatibility_writes;

		CREATE TRIGGER invalidate_parent_student_consent_permission_grant
			AFTER UPDATE OF permissions ON users.students_guardians
			FOR EACH ROW
			WHEN ((OLD.permissions -> 'parent_portal.consent.manage') IS DISTINCT FROM (NEW.permissions -> 'parent_portal.consent.manage'))
			EXECUTE FUNCTION meta.invalidate_parent_student_consent_permission_grant();
		CREATE TRIGGER invalidate_meal_participation_permission_grant
			AFTER UPDATE OF permissions ON users.students_guardians
			FOR EACH ROW
			WHEN ((OLD.permissions -> 'parent_portal.meal_participation.manage') IS DISTINCT FROM (NEW.permissions -> 'parent_portal.meal_participation.manage'))
			EXECUTE FUNCTION meta.invalidate_meal_participation_permission_grant();

		ALTER TABLE meta.parent_student_consent_permission_grants
			DROP CONSTRAINT parent_student_consent_permission_gran_student_guardian_id_fkey;
		ALTER TABLE meta.parent_student_consent_permission_grants
			ADD CONSTRAINT parent_student_consent_permission_gran_student_guardian_id_fkey
			FOREIGN KEY (student_guardian_id) REFERENCES users.students_guardians(id) ON DELETE CASCADE NOT VALID;
		ALTER TABLE meta.meal_participation_permission_grants
			DROP CONSTRAINT meal_participation_permission_grants_student_guardian_id_fkey;
		ALTER TABLE meta.meal_participation_permission_grants
			ADD CONSTRAINT meal_participation_permission_grants_student_guardian_id_fkey
			FOREIGN KEY (student_guardian_id) REFERENCES users.students_guardians(id) ON DELETE CASCADE NOT VALID;

		SELECT setval('users.students_guardians_id_seq', GREATEST(
			(SELECT last_value FROM users.students_guardians_id_seq),
			(SELECT last_value FROM users.student_guardian_relationships_id_seq)), true);
		ALTER TABLE users.students_guardians
			ALTER COLUMN id SET DEFAULT nextval('users.students_guardians_id_seq');

		DO $$
		DECLARE previous_path text := current_setting('search_path');
		BEGIN
			-- pg_get_viewdef omits the schema of anything the search path
			-- already resolves, so the rewrite below would miss a session that
			-- has users on its path.
			PERFORM set_config('search_path', 'pg_catalog', true);
			EXECUTE 'CREATE OR REPLACE VIEW platform.delivery_push_subscriptions WITH (security_invoker = true, security_barrier = true) AS '
				|| replace(regexp_replace(pg_get_viewdef('platform.delivery_push_subscriptions'::regclass),
					'\(\(users\.student_guardian_relationships child_link\s+JOIN auth\.guardian_student_access child_access ON \(\(\(child_access\.tenant_id = child_link\.tenant_id\) AND \(child_access\.relationship_id = child_link\.id\)\)\)\)',
					'(users.students_guardians child_link'),
					'child_access.permissions', 'child_link.permissions');
			PERFORM set_config('search_path', previous_path, true);
		END $$;

		ALTER TABLE users.student_guardian_relationships
			DROP CONSTRAINT fk_student_guardian_relationships_guardian,
			ADD CONSTRAINT fk_student_guardian_relationships_guardian FOREIGN KEY (tenant_id, guardian_profile_id)
				REFERENCES users.guardian_profiles(tenant_id, id) ON DELETE CASCADE;
		REVOKE SELECT ON users.student_guardian_relationships, users.student_guardian_pickup_permissions,
			auth.guardian_student_access FROM phoenix_auth;
		TRUNCATE auth.guardian_student_access, users.student_guardian_pickup_permissions, users.student_guardian_relationships;
		COMMENT ON TABLE users.students_guardians IS NULL;
		COMMENT ON TABLE users.student_guardian_relationships IS
			'People-owned target. Empty during Expand #2716; users.students_guardians remains authoritative until Cutover.';
		COMMENT ON TABLE users.student_guardian_pickup_permissions IS
			'Care-Plan-owned target. Empty during Expand #2716; users.students_guardians remains authoritative until Cutover.';
		COMMENT ON TABLE auth.guardian_student_access IS
			'Identity-owned target. Empty during Expand #2716; users.students_guardians remains authoritative until Cutover.';
	`); err != nil {
		tb.Fatalf("restore guardian storage before cutover: %v", err)
	}
	var restoredView bool
	if err := db.NewRaw(`SELECT pg_get_viewdef('platform.delivery_push_subscriptions'::regclass) LIKE '%students_guardians%'`).
		Scan(context.Background(), &restoredView); err != nil {
		tb.Fatalf("restore guardian storage before cutover: inspect push subscription view: %v", err)
	}
	if !restoredView {
		tb.Fatal("restore guardian storage before cutover: the push subscription view still reads the owner tables")
	}
}
