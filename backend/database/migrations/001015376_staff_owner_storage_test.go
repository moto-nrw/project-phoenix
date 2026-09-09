package migrations

import (
	"context"
	"strconv"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffOwnerStorageStartsEmpty(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Expand", "Compatibility")
	_, err := db.ExecContext(ctx, `UPDATE users.staff SET staff_notes = 'old-table write' WHERE id = ?`, staff.ID)
	require.NoError(t, err)
	for _, table := range []string{"users.staff_school_memberships", "users.staff_employment_profiles"} {
		var count int
		err := db.NewRaw("SELECT count(*) FROM "+table).Scan(ctx, &count)
		require.NoError(t, err)
		require.Zero(t, count, "Expand must not copy old-table writes into %s", table)
	}
	var notes string
	require.NoError(t, db.NewRaw(`SELECT staff_notes FROM users.staff WHERE id = ?`, staff.ID).Scan(ctx, &notes))
	require.Equal(t, "old-table write", notes)
}

func TestStaffOwnerStorageRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Before", "Expand")
	schemaBefore := staffSourceSchema(t, db)
	var before string
	require.NoError(t, db.NewRaw(`SELECT row_to_json(s)::text FROM users.staff s WHERE id = ?`, staff.ID).Scan(ctx, &before))
	require.NoError(t, staffOwnerStorageDown(ctx, db))
	for _, table := range []string{"users.staff_school_memberships", "users.staff_employment_profiles"} {
		var absent bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NULL`, table).Scan(ctx, &absent))
		require.True(t, absent)
	}
	require.NoError(t, staffOwnerStorageUp(ctx, db))
	var after string
	require.NoError(t, db.NewRaw(`SELECT row_to_json(s)::text FROM users.staff s WHERE id = ?`, staff.ID).Scan(ctx, &after))
	require.Equal(t, before, after)
	require.Equal(t, schemaBefore, staffSourceSchema(t, db), "up/down must preserve the old table's columns, constraints, indexes, triggers, RLS and grants")
	membershipID := insertStaffOwnerMembership(t, db, testpkg.Tenant(t), staff.PersonID)
	require.ErrorContains(t, staffOwnerStorageDown(ctx, db), "staff owner storage is not empty")
	var survivingID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.staff_school_memberships WHERE id = ?`, membershipID).Scan(ctx, &survivingID))
	require.Equal(t, membershipID, survivingID)
}

func staffSourceSchema(t *testing.T, db *testpkg.DB) string {
	t.Helper()
	var snapshot string
	require.NoError(t, db.NewRaw(`SELECT jsonb_build_object(
		'columns', (SELECT jsonb_agg(to_jsonb(c) ORDER BY ordinal_position)
			FROM information_schema.columns c WHERE table_schema = 'users' AND table_name = 'staff'),
		'constraints', (SELECT jsonb_agg(pg_get_constraintdef(oid) ORDER BY conname)
			FROM pg_constraint WHERE conrelid = 'users.staff'::regclass),
		'indexes', (SELECT jsonb_agg(indexdef ORDER BY indexname)
			FROM pg_indexes WHERE schemaname = 'users' AND tablename = 'staff'),
		'triggers', (SELECT jsonb_agg(pg_get_triggerdef(oid) ORDER BY tgname)
			FROM pg_trigger WHERE tgrelid = 'users.staff'::regclass),
		'policies', (SELECT jsonb_agg(to_jsonb(p) ORDER BY policyname)
			FROM pg_policies p WHERE schemaname = 'users' AND tablename = 'staff'),
		'rls_enabled', relrowsecurity, 'rls_forced', relforcerowsecurity, 'grants', relacl
	)::text FROM pg_class WHERE oid = 'users.staff'::regclass`).Scan(testpkg.Ctx(t), &snapshot))
	return snapshot
}

func insertStaffOwnerMembership(t *testing.T, db *testpkg.DB, tenantID, personID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?) RETURNING id`, tenantID, personID).Scan(testpkg.Ctx(t), &id))
	return id
}

func TestStaffOwnerStorageTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.OwnTenantRows(t, db, tenantB)
	personA := testpkg.CreateTestPerson(t, db, "Owner", "A")
	personB := testpkg.CreateTestPersonForTenant(t, db, tenantB, "Owner", "B")
	membershipA := insertStaffOwnerMembership(t, db, tenantA, personA.ID)
	membershipB := insertStaffOwnerMembership(t, db, tenantB, personB.ID)
	_, err := db.ExecContext(ctx, `INSERT INTO users.staff_employment_profiles (tenant_id, membership_id) VALUES (?, ?), (?, ?)`, tenantA, membershipA, tenantB, membershipB)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?)`, tenantA, personB.ID)
	require.ErrorContains(t, err, "fk_staff_school_memberships_person")
	_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET tenant_id = ? WHERE membership_id = ?`, tenantA, membershipB)
	require.ErrorContains(t, err, "fk_staff_employment_profiles_membership")
	for _, tenantID := range []int64{tenantA, tenantB} {
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error {
			_, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, strconv.FormatInt(tenantID, 10))
			if err != nil {
				return err
			}
			for _, table := range []string{"users.staff_school_memberships", "users.staff_employment_profiles"} {
				var tenants []int64
				require.NoError(t, tx.NewRaw("SELECT tenant_id FROM "+table).Scan(ctx, &tenants))
				require.Equal(t, []int64{tenantID}, tenants)
				result, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE tenant_id <> ?", tenantID)
				require.NoError(t, err)
				count, err := result.RowsAffected()
				require.NoError(t, err)
				require.Zero(t, count)
			}
			return nil
		})
		require.NoError(t, err)
	}
}

func TestStaffOwnerStorageColumnMapping(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	// Every source field has exactly one target owner. Profile tenant_id is
	// duplicated only to enforce the membership FK and RLS boundary.
	mapping := map[string][]string{
		"staff_school_memberships":  {"id", "tenant_id", "person_id", "created_at", "updated_at", "deleted_at"},
		"staff_employment_profiles": {"tenant_id", "staff_notes", "employment_type", "work_time_model_id", "personnel_number", "rotation_anchor_date", "birthday_display_opt_out"},
	}
	type column struct {
		Name     string
		Type     string
		Nullable bool
	}
	for table, names := range mapping {
		for _, name := range names {
			var source, target column
			query := `SELECT column_name AS name, format_type(a.atttypid, a.atttypmod) AS type, NOT a.attnotnull AS nullable
				FROM information_schema.columns c JOIN pg_attribute a ON a.attrelid = ('users.' || c.table_name)::regclass AND a.attname = c.column_name
				WHERE c.table_schema = 'users' AND c.table_name = ? AND c.column_name = ?`
			require.NoError(t, db.NewRaw(query, "staff", name).Scan(ctx, &source))
			require.NoError(t, db.NewRaw(query, table, name).Scan(ctx, &target))
			require.Equal(t, source, target, "%s.%s must preserve the source storage contract", table, name)
		}
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'users' AND table_name = ?`, table).Scan(ctx, &count))
		expected := len(names)
		if table == "staff_employment_profiles" {
			expected++ // membership_id replaces the old staff row identity.
		}
		require.Equal(t, expected, count)
	}
	var triggers int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_trigger WHERE tgrelid IN ('users.staff_school_memberships'::regclass, 'users.staff_employment_profiles'::regclass) AND NOT tgisinternal`).Scan(ctx, &triggers))
	require.Zero(t, triggers, "Expand must not introduce application or synchronization triggers")
}

func TestStaffOwnerStorageConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	person := testpkg.CreateTestPerson(t, db, "Constraints", "Staff")
	membershipID := insertStaffOwnerMembership(t, db, tenantID, person.ID)
	_, err := db.ExecContext(ctx, `INSERT INTO users.staff_employment_profiles (tenant_id, membership_id) VALUES (?, ?)`, tenantID, membershipID)
	require.NoError(t, err)
	var defaults bool
	require.NoError(t, db.NewRaw(`SELECT staff_notes IS NULL AND employment_type IS NULL AND work_time_model_id IS NULL
		AND personnel_number IS NULL AND rotation_anchor_date IS NULL AND NOT birthday_display_opt_out
		FROM users.staff_employment_profiles WHERE membership_id = ?`, membershipID).Scan(ctx, &defaults))
	require.True(t, defaults)
	for _, employmentType := range []string{"full_time", "part_time", "minijob"} {
		_, err := db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET employment_type = ?, rotation_anchor_date = '2026-09-09', staff_notes = 'notes', personnel_number = '0042', birthday_display_opt_out = true WHERE membership_id = ?`, employmentType, membershipID)
		require.NoError(t, err)
	}
	for _, query := range []string{
		`UPDATE users.staff_employment_profiles SET employment_type = 'invalid' WHERE membership_id = ?`,
		`UPDATE users.staff_employment_profiles SET birthday_display_opt_out = NULL WHERE membership_id = ?`,
		`UPDATE users.staff_school_memberships SET person_id = NULL WHERE id = ?`,
		`UPDATE users.staff_school_memberships SET tenant_id = NULL WHERE id = ?`,
	} {
		_, err := db.ExecContext(ctx, query, membershipID)
		require.Error(t, err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users.staff_employment_profiles (tenant_id, membership_id) VALUES (?, ?)`, tenantID, membershipID)
	require.ErrorContains(t, err, "23505")
	_, err = db.ExecContext(ctx, `INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?)`, tenantID, person.ID)
	require.ErrorContains(t, err, "23505")
	_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE id = ?`, membershipID)
	require.NoError(t, err)
	newID := insertStaffOwnerMembership(t, db, tenantID, person.ID)
	require.NotEqual(t, membershipID, newID, "soft-deleted membership does not prevent rejoining")
	_, err = db.ExecContext(ctx, `DELETE FROM users.staff_school_memberships WHERE id = ?`, membershipID)
	require.NoError(t, err)
	var profiles int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_employment_profiles WHERE membership_id = ?`, membershipID).Scan(ctx, &profiles))
	require.Zero(t, profiles, "physical membership deletion cascades to its profile")
}

func TestStaffOwnerStorageTenantWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.OwnTenantRows(t, db, tenantB)
	personA := testpkg.CreateTestPerson(t, db, "Writes", "A")
	personB := testpkg.CreateTestPersonForTenant(t, db, tenantB, "Writes", "B")
	var modelA, modelB int64
	for tenantID, id := range map[int64]*int64{tenantA: &modelA, tenantB: &modelB} {
		require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date) VALUES (?, 'Expand test', '2026-09-09') RETURNING id`, tenantID).Scan(ctx, id))
	}
	run := func(tenantID string, action func(context.Context, testpkg.Tx) error) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, tenantID); err != nil {
				return err
			}
			return action(ctx, tx)
		})
	}
	var membershipID int64
	require.NoError(t, run(strconv.FormatInt(tenantA, 10), func(ctx context.Context, tx testpkg.Tx) error {
		if err := tx.NewRaw(`INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?) RETURNING id`, tenantA, personA.ID).Scan(ctx, &membershipID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO users.staff_employment_profiles (tenant_id, membership_id, work_time_model_id) VALUES (?, ?, ?)`, tenantA, membershipID, modelA)
		return err
	}))
	for _, query := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users.staff_school_memberships (tenant_id, person_id) VALUES (?, ?)`, []any{tenantB, personB.ID}},
		{`UPDATE users.staff_employment_profiles SET tenant_id = ? WHERE membership_id = ?`, []any{tenantB, membershipID}},
		{`UPDATE users.staff_employment_profiles SET work_time_model_id = ? WHERE membership_id = ?`, []any{modelB, membershipID}},
	} {
		err := run(strconv.FormatInt(tenantA, 10), func(ctx context.Context, tx testpkg.Tx) error {
			_, err := tx.ExecContext(ctx, query.sql, query.args...)
			return err
		})
		require.ErrorContains(t, err, "42501", "tenant policy must reject %s", query.sql)
	}
	require.NoError(t, run("", func(ctx context.Context, tx testpkg.Tx) error {
		for _, table := range []string{"users.staff_school_memberships", "users.staff_employment_profiles"} {
			var count int
			require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+table).Scan(ctx, &count))
			require.Zero(t, count, "missing tenant must fail closed")
		}
		return nil
	}))
	_, err := db.ExecContext(ctx, `DELETE FROM config.work_time_models WHERE id = ?`, modelA)
	require.ErrorContains(t, err, "23503", "referenced model must remain protected")
}
