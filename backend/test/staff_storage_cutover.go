package test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// RestoreStaffStorageBeforeCutover turns users.staff back into the
// authoritative base table it was before migration 1.15.409 (#2753).
//
// Contracts written for the Expand (#2715) and Backfill (#2752) migrations
// describe a world in which users.staff holds the rows and the two owner
// tables are empty or trail it. The cutover replaced it with a rollback-only
// view over users.staff_school_memberships and users.staff_employment_profiles,
// so those tests restore their world inside their own disposable clone first.
// Every staff row the clone holds (the bootstrap staff member included) moves
// back into users.staff before the owner tables are emptied. Production
// rollback deliberately keeps the compatibility shape instead of reversing it.
//
// It requires a per-test database (SetupIsolatedTestDB, or a package that
// opted into PerTestDatabases).
func RestoreStaffStorageBeforeCutover(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if db == nil {
		tb.Fatal("restore staff storage before cutover: database is required")
	}
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatal("restore staff storage before cutover requires this test's isolated database")
	}
	var relkind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.staff'::regclass`).
		Scan(context.Background(), &relkind); err != nil {
		tb.Fatalf("restore staff storage before cutover: inspect users.staff: %v", err)
	}
	if relkind != "v" {
		tb.Fatalf("restore staff storage before cutover: users.staff is already a base table (relkind %q)", relkind)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO users.staff_legacy (id, tenant_id, person_id, created_at, updated_at, deleted_at,
			staff_notes, employment_type, work_time_model_id, personnel_number,
			rotation_anchor_date, birthday_display_opt_out)
		SELECT m.id, m.tenant_id, m.person_id, m.created_at, m.updated_at, m.deleted_at,
			p.staff_notes, p.employment_type, p.work_time_model_id, p.personnel_number,
			p.rotation_anchor_date, p.birthday_display_opt_out
		FROM users.staff_school_memberships AS m
		JOIN users.staff_employment_profiles AS p ON p.tenant_id = m.tenant_id AND p.membership_id = m.id
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id, person_id = EXCLUDED.person_id,
			created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at,
			deleted_at = EXCLUDED.deleted_at, staff_notes = EXCLUDED.staff_notes,
			employment_type = EXCLUDED.employment_type, work_time_model_id = EXCLUDED.work_time_model_id,
			personnel_number = EXCLUDED.personnel_number, rotation_anchor_date = EXCLUDED.rotation_anchor_date,
			birthday_display_opt_out = EXCLUDED.birthday_display_opt_out;
		SELECT setval('users.staff_id_seq', GREATEST(
			(SELECT last_value FROM users.staff_id_seq),
			(SELECT last_value FROM users.staff_school_memberships_id_seq)), true);

		DROP VIEW users.staff;
		DROP FUNCTION users.route_staff_compatibility();
		DROP TRIGGER staff_employment_profiles_personnel_number ON users.staff_employment_profiles;
		DROP FUNCTION users.enforce_staff_personnel_number();
		DROP SEQUENCE users.staff_compatibility_reads, users.staff_compatibility_writes;
		DROP TRIGGER update_staff_school_memberships_updated_at ON users.staff_school_memberships;
		ALTER INDEX users.idx_staff_tenant_person RENAME TO uq_staff_school_memberships_active_person;
		ALTER INDEX users.idx_staff_legacy_tenant_person RENAME TO idx_staff_tenant_person;
		ALTER INDEX users.uq_staff_legacy_tenant_personnel_number RENAME TO uq_staff_tenant_personnel_number;
		ALTER TABLE users.staff_legacy RENAME TO staff;
		COMMENT ON TABLE users.staff IS NULL;
		ALTER TABLE users.staff ADD CONSTRAINT fk_staff_work_time_model
			FOREIGN KEY (work_time_model_id) REFERENCES config.work_time_models(id) ON DELETE RESTRICT;
		-- Tables created after the cutover (1.15.419, #3259) never referenced
		-- users.staff. The restored pre-cutover world must not give them that
		-- reference, or the cutover's repoint guard sees an unknown staff key.
		ALTER TABLE config.staff_target_overrides
			DROP CONSTRAINT fk_staff_target_overrides_staff,
			DROP CONSTRAINT fk_staff_target_overrides_created_by;
		TRUNCATE config.staff_target_overrides;
		DO $$
		DECLARE constraint_row RECORD;
		        previous_path text := current_setting('search_path');
		BEGIN
			-- pg_get_constraintdef and pg_get_viewdef omit the schema of anything
			-- the search path already resolves, so the rewrites below would miss
			-- a session that has users on its path.
			PERFORM set_config('search_path', 'pg_catalog', true);
			FOR constraint_row IN
				SELECT con.conname, con.conrelid::regclass::text AS child,
					pg_get_constraintdef(con.oid) AS def
				FROM pg_constraint AS con
				WHERE con.confrelid = 'users.staff_school_memberships'::regclass AND con.contype = 'f'
				  -- The profile's own link to its membership is owner storage,
				  -- not a reference the cutover moved off users.staff.
				  AND con.conrelid <> 'users.staff_employment_profiles'::regclass
				ORDER BY con.conrelid::regclass::text, con.conname
			LOOP
				EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', constraint_row.child, constraint_row.conname);
				-- NOT VALID keeps the restore off every dependent table's rows;
				-- the referential actions are installed either way.
				EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s NOT VALID', constraint_row.child, constraint_row.conname,
					replace(constraint_row.def, 'REFERENCES users.staff_school_memberships(', 'REFERENCES users.staff('));
			END LOOP;
			EXECUTE 'CREATE OR REPLACE VIEW audit.time_tracking_audit_log WITH (security_invoker = true) AS '
				|| replace(pg_get_viewdef('audit.time_tracking_audit_log'::regclass),
					'users.staff_school_memberships st', 'users.staff st');
			PERFORM set_config('search_path', previous_path, true);
		END $$;
		TRUNCATE users.staff_employment_profiles, users.staff_school_memberships;
		DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = 'staff-owner';
	`); err != nil {
		tb.Fatalf("restore staff storage before cutover: %v", err)
	}
}
