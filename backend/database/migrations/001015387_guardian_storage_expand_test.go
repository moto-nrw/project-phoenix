package migrations

// Migration tests stay internal: the active architecture policy forbids an
// external module-behavior-test from importing a migration adapter. Exercise
// the registered deployment entry points below without widening that policy.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

const guardianStorageExpandName = "001015387"

var guardianStorageTargets = []string{
	"users.student_guardian_relationships",
	"users.student_guardian_pickup_permissions",
	"auth.guardian_student_access",
}

func TestGuardianStorageExpandColumnMapping(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, tc := range []struct {
		target  string
		columns []string
	}{
		{"users.student_guardian_relationships", []string{
			"id", "tenant_id", "student_id", "guardian_profile_id", "relationship_type", "guardian_role",
			"is_primary", "is_emergency_contact", "emergency_priority", "is_payer", "created_at", "updated_at",
		}},
		{"users.student_guardian_pickup_permissions", []string{
			"id", "tenant_id", "relationship_id", "can_pickup", "pickup_notes", "created_at", "updated_at",
		}},
		{"auth.guardian_student_access", []string{
			"id", "tenant_id", "relationship_id", "account_id", "permissions", "created_at", "updated_at",
		}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			schema, table, _ := strings.Cut(tc.target, ".")
			var columns []string
			require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
				WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`, schema, table).Scan(t.Context(), &columns))
			require.ElementsMatch(t, tc.columns, columns)
			// Every moved field keeps the SQL type, nullability and default of its
			// users.students_guardians source column.
			for _, column := range tc.columns {
				if column == "id" || column == "relationship_id" || column == "account_id" {
					continue
				}
				var same bool
				require.NoError(t, db.NewRaw(`SELECT old.data_type = target.data_type
					AND old.is_nullable = target.is_nullable
					AND old.column_default IS NOT DISTINCT FROM target.column_default
					FROM information_schema.columns old JOIN information_schema.columns target USING (column_name)
					WHERE old.table_schema = 'users' AND old.table_name = 'students_guardians'
					AND target.table_schema = ? AND target.table_name = ? AND old.column_name = ?`,
					schema, table, column).Scan(t.Context(), &same))
				require.True(t, same, "mapping of %s", column)
			}
		})
	}
	// Every old column has exactly one owner among the targets.
	var unowned []string
	require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name = 'students_guardians'
		AND column_name NOT IN ('id', 'tenant_id', 'created_at', 'updated_at')
		AND column_name NOT IN (SELECT column_name FROM information_schema.columns
			WHERE (table_schema, table_name) IN (('users', 'student_guardian_relationships'),
				('users', 'student_guardian_pickup_permissions'), ('auth', 'guardian_student_access')))
		ORDER BY column_name`).Scan(t.Context(), &unowned))
	require.Empty(t, unowned)
	// ...and no moved column is duplicated across two targets.
	var duplicated []string
	require.NoError(t, db.NewRaw(`SELECT column_name FROM information_schema.columns
		WHERE (table_schema, table_name) IN (('users', 'student_guardian_relationships'),
			('users', 'student_guardian_pickup_permissions'), ('auth', 'guardian_student_access'))
		AND column_name NOT IN ('id', 'tenant_id', 'relationship_id', 'created_at', 'updated_at')
		GROUP BY column_name HAVING count(*) > 1 ORDER BY column_name`).Scan(t.Context(), &duplicated))
	require.Empty(t, duplicated)
	// The link column is a NOT NULL bigint on both dependent targets.
	var links int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
		WHERE column_name = 'relationship_id' AND data_type = 'bigint' AND is_nullable = 'NO'
		AND (table_schema, table_name) IN (('users', 'student_guardian_pickup_permissions'), ('auth', 'guardian_student_access'))`).Scan(t.Context(), &links))
	require.Equal(t, 2, links)
}

type guardianStorageExpandFixture struct {
	tenant, student, guardian, account int64
}

func createGuardianStorageExpandFixture(t *testing.T, db *testpkg.DB) guardianStorageExpandFixture {
	t.Helper()
	f := guardianStorageExpandFixture{tenant: testpkg.UniqueTestTenantID(t)}
	testpkg.EnsureTestTenant(t, db, f.tenant)
	f.student = testpkg.CreateTestStudentForTenant(t, db, f.tenant, "Guardian", "Expand", "2a").ID
	f.guardian = testpkg.CreateTestGuardianProfileForTenant(t, db, f.tenant, "Guardian", "Expand", "guardian-expand").ID
	f.account = testpkg.CreateTestAccount(t, db, "guardian-expand").ID
	return f
}

// insertRelationship creates the People row the two dependent targets hang off.
func (f guardianStorageExpandFixture) insertRelationship(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_guardian_relationships
		(tenant_id, student_id, guardian_profile_id, relationship_type) VALUES (?, ?, ?, 'parent') RETURNING id`,
		f.tenant, f.student, f.guardian).Scan(t.Context(), &id))
	return id
}

func TestGuardianStorageExpandTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a, b := createGuardianStorageExpandFixture(t, db), createGuardianStorageExpandFixture(t, db)
	fixtures := []guardianStorageExpandFixture{a, b}
	// The dependent targets need one relationship per tenant. Create those as
	// superuser; the relationship subtest runs first against the empty table.
	var relationships []int64
	for _, tc := range []struct {
		table, insert, retarget string
		// args builds the insert arguments for fixture i stamped with tenant.
		args func(i int, tenant int64) []any
		// parent identifies fixture i's row for the tenant-retarget update.
		parent func(i int) int64
	}{
		{
			table:    "users.student_guardian_relationships",
			insert:   "INSERT INTO users.student_guardian_relationships (tenant_id, student_id, guardian_profile_id, relationship_type) VALUES (?, ?, ?, 'parent')",
			retarget: "UPDATE users.student_guardian_relationships SET tenant_id = ? WHERE student_id = ?",
			args:     func(i int, tenant int64) []any { return []any{tenant, fixtures[i].student, fixtures[i].guardian} },
			parent:   func(i int) int64 { return fixtures[i].student },
		},
		{
			table:    "users.student_guardian_pickup_permissions",
			insert:   "INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id) VALUES (?, ?)",
			retarget: "UPDATE users.student_guardian_pickup_permissions SET tenant_id = ? WHERE relationship_id = ?",
			args:     func(i int, tenant int64) []any { return []any{tenant, relationships[i]} },
			parent:   func(i int) int64 { return relationships[i] },
		},
		{
			table:    "auth.guardian_student_access",
			insert:   "INSERT INTO auth.guardian_student_access (tenant_id, relationship_id) VALUES (?, ?)",
			retarget: "UPDATE auth.guardian_student_access SET tenant_id = ? WHERE relationship_id = ?",
			args:     func(i int, tenant int64) []any { return []any{tenant, relationships[i]} },
			parent:   func(i int) int64 { return relationships[i] },
		},
	} {
		t.Run(tc.table, func(t *testing.T) {
			for i, own := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, tc.insert, tc.args(i, own.tenant)...)
					return err
				}))
			}
			for i, own := range fixtures {
				other := fixtures[1-i]
				var tenants []int64
				require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					return tx.NewRaw("SELECT tenant_id FROM "+tc.table).Scan(ctx, &tenants)
				}))
				require.Equal(t, []int64{own.tenant}, tenants, "unfiltered reads must remain tenant scoped")
				for _, query := range []string{
					"UPDATE " + tc.table + " SET updated_at = NOW() WHERE tenant_id = ?",
					"DELETE FROM " + tc.table + " WHERE tenant_id = ?",
				} {
					var affected int64
					require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
						result, err := tx.ExecContext(ctx, query, other.tenant)
						if err != nil {
							return err
						}
						affected, err = result.RowsAffected()
						return err
					}))
					require.Zero(t, affected)
				}
				err := testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, tc.insert, tc.args(i, other.tenant)...)
					return err
				})
				requirePresenceSQLState(t, err, "42501")
				err = testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
					_, err := tx.ExecContext(ctx, tc.retarget, other.tenant, tc.parent(i))
					return err
				})
				requirePresenceSQLState(t, err, "42501")
			}
			// An absent school context is deny-by-default, not a global read.
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_tenant; SELECT set_config('app.current_tenant_id', '', true)`)
			require.NoError(t, err)
			var count int
			require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+tc.table).Scan(t.Context(), &count))
			require.Zero(t, count)
			_, err = tx.ExecContext(t.Context(), tc.insert, tc.args(0, a.tenant)...)
			requirePresenceSQLState(t, err, "42501")
			require.NoError(t, tx.Rollback())
			if tc.table == "users.student_guardian_relationships" {
				// Keep the tenant-inserted relationships as parents for the
				// dependent subtests instead of removing them.
				for _, own := range fixtures {
					var id int64
					require.NoError(t, db.NewRaw("SELECT id FROM users.student_guardian_relationships WHERE tenant_id = ?", own.tenant).Scan(t.Context(), &id))
					relationships = append(relationships, id)
				}
				return
			}
			_, err = db.ExecContext(t.Context(), "DELETE FROM "+tc.table)
			require.NoError(t, err)
		})
	}
}

func TestGuardianStorageExpandConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a, b := createGuardianStorageExpandFixture(t, db), createGuardianStorageExpandFixture(t, db)
	relationship := a.insertRelationship(t, db)
	_, err := db.ExecContext(t.Context(), `INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id) VALUES (?, ?)`, a.tenant, relationship)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id) VALUES (?, ?, ?)`, a.tenant, relationship, a.account)
	require.NoError(t, err)

	// Cross-tenant parents are rejected by the composite FKs.
	for _, tc := range []struct {
		query string
		id    int64
	}{
		{"UPDATE users.student_guardian_relationships SET student_id = ? WHERE tenant_id = ?", b.student},
		{"UPDATE users.student_guardian_relationships SET guardian_profile_id = ? WHERE tenant_id = ?", b.guardian},
		{"UPDATE users.student_guardian_pickup_permissions SET tenant_id = ? WHERE tenant_id = ?", b.tenant},
		{"UPDATE auth.guardian_student_access SET tenant_id = ? WHERE tenant_id = ?", b.tenant},
	} {
		_, err := db.ExecContext(t.Context(), tc.query, tc.id, a.tenant)
		requirePresenceSQLState(t, err, "23503")
	}
	// One row per relationship in each dependent target, one relationship per pair.
	for _, query := range []string{
		`INSERT INTO users.student_guardian_relationships (tenant_id, student_id, guardian_profile_id, relationship_type) VALUES (?, ?, ?, 'other')`,
		`INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id) VALUES (?, ?)`,
		`INSERT INTO auth.guardian_student_access (tenant_id, relationship_id) VALUES (?, ?)`,
	} {
		args := []any{a.tenant, relationship}
		if strings.Contains(query, "student_id") {
			args = []any{a.tenant, a.student, a.guardian}
		}
		_, err := db.ExecContext(t.Context(), query, args...)
		requirePresenceSQLState(t, err, "23505")
	}
	// At most one primary and one payer per child; a second guardian may be
	// neither, and the same guardian may hold both flags.
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, a.tenant, "Second", "Guardian", "second-guardian").ID
	_, err = db.ExecContext(t.Context(), `UPDATE users.student_guardian_relationships SET is_primary = TRUE, is_payer = TRUE WHERE id = ?`, relationship)
	require.NoError(t, err)
	for _, flag := range []string{"is_primary", "is_payer"} {
		_, err := db.ExecContext(t.Context(), fmt.Sprintf(`INSERT INTO users.student_guardian_relationships
			(tenant_id, student_id, guardian_profile_id, relationship_type, %s) VALUES (?, ?, ?, 'parent', TRUE)`, flag), a.tenant, a.student, second)
		requirePresenceSQLState(t, err, "23505")
	}
	_, err = db.ExecContext(t.Context(), `INSERT INTO users.student_guardian_relationships
		(tenant_id, student_id, guardian_profile_id, relationship_type) VALUES (?, ?, ?, 'parent')`, a.tenant, a.student, second)
	require.NoError(t, err)
	// Permissions stay a JSON object.
	for _, value := range []string{`[]`, `"x"`, `1`, `null`} {
		_, err := db.ExecContext(t.Context(), `UPDATE auth.guardian_student_access SET permissions = ?::jsonb`, value)
		requirePresenceSQLState(t, err, "23514")
	}
	_, err = db.ExecContext(t.Context(), `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.access": true}'::jsonb`)
	require.NoError(t, err)
}

func TestGuardianStorageExpandReferenceDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createGuardianStorageExpandFixture(t, db)
	relationship := f.insertRelationship(t, db)
	_, err := db.ExecContext(t.Context(), `INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id, can_pickup) VALUES (?, ?, TRUE)`, f.tenant, relationship)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id) VALUES (?, ?, ?)`, f.tenant, relationship, f.account)
	require.NoError(t, err)
	// A guardian with links cannot be deleted blindly (#819 RESTRICT contract).
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.guardian_profiles WHERE id = ?`, f.guardian)
	requirePresenceSQLState(t, err, "23503")
	// Losing the portal account clears only the binding.
	_, err = db.ExecContext(t.Context(), `DELETE FROM auth.accounts WHERE id = ?`, f.account)
	require.NoError(t, err)
	var preserved bool
	require.NoError(t, db.NewRaw(`SELECT tenant_id = ? AND account_id IS NULL AND permissions = '{}'::jsonb
		FROM auth.guardian_student_access WHERE relationship_id = ?`, f.tenant, relationship).Scan(t.Context(), &preserved))
	require.True(t, preserved)
	// Deleting the relationship cascades to Care Plan and Identity rows.
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.student_guardian_relationships WHERE id = ?`, relationship)
	require.NoError(t, err)
	assertGuardianStorageTargetsEmpty(t, db)
	// Deleting the child cascades through the relationship.
	relationship = f.insertRelationship(t, db)
	_, err = db.ExecContext(t.Context(), `INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id) VALUES (?, ?)`, f.tenant, relationship)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO auth.guardian_student_access (tenant_id, relationship_id) VALUES (?, ?)`, f.tenant, relationship)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.students WHERE id = ?`, f.student)
	require.NoError(t, err)
	assertGuardianStorageTargetsEmpty(t, db)
	_, err = db.ExecContext(t.Context(), `DELETE FROM users.guardian_profiles WHERE id = ?`, f.guardian)
	require.NoError(t, err)
}

func guardianOldStorageSnapshot(t *testing.T, db *testpkg.DB) []string {
	t.Helper()
	var result []string
	for _, query := range []string{
		"SELECT COALESCE(jsonb_agg(to_jsonb(row) ORDER BY id), '[]')::text FROM users.students_guardians row",
		`SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY ordinal_position), '[]')::text FROM information_schema.columns c WHERE table_schema = 'users' AND table_name = 'students_guardians'`,
		`SELECT COALESCE(jsonb_agg(pg_get_constraintdef(oid) ORDER BY conname), '[]')::text FROM pg_constraint WHERE conrelid = 'users.students_guardians'::regclass`,
		`SELECT COALESCE(jsonb_agg(indexdef ORDER BY indexname), '[]')::text FROM pg_indexes WHERE schemaname = 'users' AND tablename = 'students_guardians'`,
		`SELECT COALESCE(jsonb_agg(pg_get_triggerdef(oid) ORDER BY tgname), '[]')::text FROM pg_trigger WHERE tgrelid = 'users.students_guardians'::regclass AND NOT tgisinternal`,
		`SELECT COALESCE(jsonb_agg(jsonb_build_object('name', policyname, 'qual', qual, 'check', with_check) ORDER BY policyname), '[]')::text FROM pg_policies WHERE schemaname = 'users' AND tablename = 'students_guardians'`,
	} {
		var snapshot string
		require.NoError(t, db.NewRaw(query).Scan(t.Context(), &snapshot))
		result = append(result, snapshot)
	}
	return result
}

func assertGuardianStorageTargetsEmpty(t *testing.T, db *testpkg.DB) {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.student_guardian_relationships)
		+ (SELECT count(*) FROM users.student_guardian_pickup_permissions)
		+ (SELECT count(*) FROM auth.guardian_student_access)`).Scan(t.Context(), &count))
	require.Zero(t, count, "Expand must never copy or dual-write old rows")
}

func TestGuardianStorageExpandUpDownPreservesOldAuthority(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createGuardianStorageExpandFixture(t, db)
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, f.tenant, f.student, f.guardian, "primary_guardian")
	assertGuardianStorageTargetsEmpty(t, db)
	before := guardianOldStorageSnapshot(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, false))
	require.Equal(t, before, guardianOldStorageSnapshot(t, db))
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, true))
	require.Equal(t, before, guardianOldStorageSnapshot(t, db))
	assertGuardianStorageTargetsEmpty(t, db)
	// The previous application's SQL shape still reads and writes the old
	// table, including the demotion trigger and permission updates. No target
	// row is required or produced.
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, f.tenant, "Second", "Guardian", "second-guardian").ID
	require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, f.tenant, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users.students_guardians
			(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role, is_primary, can_pickup, pickup_notes, is_payer, permissions)
			VALUES (?, ?, ?, 'relative', 'pickup_only', TRUE, TRUE, 'Oma', TRUE, '{"parent_portal.access": false}')`, f.tenant, f.student, second); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE users.students_guardians SET permissions = permissions || '{"parent_portal.notes.write": true}',
			is_emergency_contact = TRUE, emergency_priority = 2 WHERE id = ?`, link.ID)
		return err
	}))
	var demoted bool
	require.NoError(t, db.NewRaw(`SELECT NOT is_primary AND (permissions->>'parent_portal.notes.write')::boolean
		FROM users.students_guardians WHERE id = ?`, link.ID).Scan(t.Context(), &demoted))
	require.True(t, demoted, "old demotion trigger and permission writes keep working")
	assertGuardianStorageTargetsEmpty(t, db)
	beforeDown := guardianOldStorageSnapshot(t, db)
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, false))
	require.Equal(t, beforeDown, guardianOldStorageSnapshot(t, db))
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE (n.nspname = 'users' AND (c.relname LIKE '%student_guardian_relationships%' OR c.relname LIKE '%student_guardian_pickup_permissions%'))
		OR (n.nspname = 'auth' AND c.relname LIKE '%guardian_student_access%')`).Scan(t.Context(), &remaining))
	require.Zero(t, remaining, "tables, indexes and owned sequences must all be gone")
	var policies int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_policies WHERE tablename IN
		('student_guardian_relationships', 'student_guardian_pickup_permissions', 'guardian_student_access')`).Scan(t.Context(), &policies))
	require.Zero(t, policies)
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, true))
	assertGuardianStorageTargetsEmpty(t, db)
}

func TestGuardianStorageExpandRollbackRefusesPopulatedTargets(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createGuardianStorageExpandFixture(t, db)
	relationship := f.insertRelationship(t, db)
	for _, table := range []string{"users.student_guardian_pickup_permissions", "auth.guardian_student_access"} {
		_, err := db.ExecContext(t.Context(), fmt.Sprintf("INSERT INTO %s (tenant_id, relationship_id) VALUES (?, ?)", table), f.tenant, relationship)
		require.NoError(t, err)
	}
	// Drain from the leaves inward so each table is the only populated one.
	for _, table := range []string{"auth.guardian_student_access", "users.student_guardian_pickup_permissions", "users.student_guardian_relationships"} {
		require.ErrorContains(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, false), "requires empty target tables")
		var count int
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+table).Scan(t.Context(), &count))
		require.Equal(t, 1, count)
		_, err := db.ExecContext(t.Context(), "DELETE FROM "+table)
		require.NoError(t, err)
	}
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, false))
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, true))
	assertGuardianStorageTargetsEmpty(t, db)
}

func TestGuardianStorageExpandCatalogAndDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, table := range guardianStorageTargets {
		schema, name, _ := strings.Cut(table, ".")
		var forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid = ?::regclass`, table).Scan(t.Context(), &forced))
		require.True(t, forced, table)
		var policies []string
		require.NoError(t, db.NewRaw(`SELECT policyname FROM pg_policies WHERE schemaname = ? AND tablename = ?`, schema, name).Scan(t.Context(), &policies))
		require.Equal(t, []string{"tenant_isolation_" + schema + "_" + name}, policies)
		var invalid int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_index WHERE indrelid = ?::regclass AND NOT indisvalid`, table).Scan(t.Context(), &invalid))
		require.Zero(t, invalid)
		var customTriggers int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_trigger WHERE tgrelid = ?::regclass AND NOT tgisinternal`, table).Scan(t.Context(), &customTriggers))
		require.Zero(t, customTriggers, "Expand adds no routing or compatibility triggers")
		for _, role := range []string{"phoenix_tenant", "phoenix_admin"} {
			var granted bool
			require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, ?::regclass, 'SELECT, INSERT, UPDATE, DELETE')`, role, table).Scan(t.Context(), &granted))
			require.True(t, granted, "%s on %s", role, table)
		}
	}
	for index, columns := range map[string]string{
		"users.uq_student_guardian_relationships_pair":              "tenant_id, student_id, guardian_profile_id",
		"users.idx_student_guardian_relationships_guardian":         "tenant_id, guardian_profile_id",
		"users.uq_student_guardian_relationships_primary":           "tenant_id, student_id",
		"users.uq_student_guardian_relationships_payer":             "tenant_id, student_id",
		"users.idx_student_guardian_relationships_emergency":        "tenant_id, student_id, emergency_priority",
		"users.uq_student_guardian_pickup_permissions_relationship": "tenant_id, relationship_id",
		"auth.uq_guardian_student_access_relationship":              "tenant_id, relationship_id",
		"auth.idx_guardian_student_access_account":                  "tenant_id, account_id",
		"auth.idx_guardian_student_access_account_global":           "account_id",
	} {
		var actual string
		require.NoError(t, db.NewRaw(`SELECT string_agg(pg_get_indexdef(indexrelid, n, true), ', ' ORDER BY n)
			FROM pg_index CROSS JOIN LATERAL generate_series(1, indnkeyatts) n
			WHERE indexrelid = ?::regclass`, index).Scan(t.Context(), &actual))
		require.Equal(t, columns, actual, index)
	}
	f := createGuardianStorageExpandFixture(t, db)
	var defaults bool
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_guardian_relationships (tenant_id, student_id, guardian_profile_id, relationship_type)
		VALUES (?, ?, ?, 'parent')
		RETURNING guardian_role = 'custom' AND NOT is_primary AND NOT is_emergency_contact AND emergency_priority = 1
		AND NOT is_payer AND created_at IS NOT NULL AND updated_at IS NOT NULL`, f.tenant, f.student, f.guardian).Scan(t.Context(), &defaults))
	require.True(t, defaults)
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id)
		SELECT tenant_id, id FROM users.student_guardian_relationships
		RETURNING NOT can_pickup AND pickup_notes IS NULL AND created_at IS NOT NULL AND updated_at IS NOT NULL`).Scan(t.Context(), &defaults))
	require.True(t, defaults)
	require.NoError(t, db.NewRaw(`INSERT INTO auth.guardian_student_access (tenant_id, relationship_id)
		SELECT tenant_id, id FROM users.student_guardian_relationships
		RETURNING account_id IS NULL AND permissions = '{}'::jsonb AND created_at IS NOT NULL AND updated_at IS NOT NULL`).Scan(t.Context(), &defaults))
	require.True(t, defaults)
}

func TestGuardianStorageExpandFailureIsAtomic(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, false))
	// Fail after the first two tables exist, before grants and RLS provisioning.
	_, err := db.ExecContext(t.Context(), `CREATE TABLE auth.guardian_student_access (probe BOOLEAN)`)
	require.NoError(t, err)
	require.Error(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, true))
	for _, table := range guardianStorageTargets[:2] {
		var absent bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NULL`, table).Scan(t.Context(), &absent))
		require.True(t, absent, "a failed Expand must leave no partially provisioned %s", table)
	}
	_, err = db.ExecContext(t.Context(), `DROP TABLE auth.guardian_student_access`)
	require.NoError(t, err)
	require.NoError(t, runPresenceMigration(t.Context(), db, guardianStorageExpandName, true))
}
