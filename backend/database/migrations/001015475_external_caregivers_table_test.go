package migrations

// Migration 1.15.475 renames users.guests to users.external_caregivers
// (ADR 0090). A rename must keep everything PostgreSQL ties to the table:
// rows, row-level security, the least-privilege grants, the tenant and
// composite foreign keys and the unique index the store classifies
// conflicts by. Down restores the names the previous image reads.

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// externalCaregiverObjectNames lists the names of the table's sequence,
// indexes, constraints, policy and trigger, in the order the queries below
// return them.
func externalCaregiverObjectNames(t *testing.T, db *testpkg.DB, table string) (relations, constraints, policies, triggers []string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, db.NewRaw(`
		SELECT c.relname FROM pg_class c
		WHERE c.oid IN (
			SELECT indexrelid FROM pg_index WHERE indrelid = to_regclass('users.' || ?)
			UNION SELECT pg_get_serial_sequence('users.' || ?, 'id')::regclass
		)
		ORDER BY c.relname`, table, table).Scan(ctx, &relations))
	require.NoError(t, db.NewRaw(`SELECT conname FROM pg_constraint WHERE conrelid = to_regclass('users.' || ?) ORDER BY conname`,
		table).Scan(ctx, &constraints))
	require.NoError(t, db.NewRaw(`SELECT policyname FROM pg_policies WHERE schemaname = 'users' AND tablename = ? ORDER BY policyname`,
		table).Scan(ctx, &policies))
	require.NoError(t, db.NewRaw(`SELECT tgname FROM pg_trigger WHERE tgrelid = to_regclass('users.' || ?) AND NOT tgisinternal ORDER BY tgname`,
		table).Scan(ctx, &triggers))
	return relations, constraints, policies, triggers
}

func externalCaregiverTableExists(t *testing.T, db *testpkg.DB, table string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.' || ?) IS NOT NULL`, table).Scan(context.Background(), &exists))
	return exists
}

func TestExternalCaregiversTableRename(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	assertRenamed := func(t *testing.T) {
		t.Helper()
		assert.False(t, externalCaregiverTableExists(t, db, "guests"))
		require.True(t, externalCaregiverTableExists(t, db, "external_caregivers"))
		relations, constraints, policies, triggers := externalCaregiverObjectNames(t, db, "external_caregivers")
		assert.Equal(t, []string{
			"external_caregivers_id_seq",
			"external_caregivers_pkey",
			"idx_external_caregivers_organization",
			"idx_external_caregivers_staff_id",
			"idx_external_caregivers_tenant",
			"idx_external_caregivers_tenant_id",
			"idx_external_caregivers_tenant_staff",
		}, relations)
		assert.Equal(t, []string{
			"chk_external_caregivers_tenant_id_not_null",
			"external_caregivers_pkey",
			"external_caregivers_tenant_id_fkey",
			"fk_external_caregivers_staff_tenant",
		}, constraints)
		assert.Equal(t, []string{"tenant_isolation_users_external_caregivers"}, policies)
		assert.Equal(t, []string{"update_external_caregivers_updated_at"}, triggers)
	}
	assertRenamed(t)

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class
		WHERE oid = 'users.external_caregivers'::regclass`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled, "row-level security stays enabled")
	assert.True(t, forced, "row-level security stays forced")
	for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		assert.True(t, tenantHasPrivilege(t, db, "users.external_caregivers", privilege), privilege)
	}
	assert.False(t, tenantHasPrivilege(t, db, "users.external_caregivers", "TRUNCATE"))
	var sequenceUsable bool
	require.NoError(t, db.NewRaw(`SELECT has_sequence_privilege('phoenix_tenant', 'users.external_caregivers_id_seq', 'USAGE')`).
		Scan(ctx, &sequenceUsable))
	assert.True(t, sequenceUsable, "the tenant role can still draw IDs")

	var foreignKeys []string
	require.NoError(t, db.NewRaw(`
		SELECT format('%s (%s) -> %s.%s on delete %s', k.conname,
			(SELECT string_agg(a.attname, ', ' ORDER BY u.ord) FROM unnest(k.conkey) WITH ORDINALITY AS u(attnum, ord)
			 JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = u.attnum),
			n.nspname, c.relname, k.confdeltype)
		FROM pg_constraint k
		JOIN pg_class c ON c.oid = k.confrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE k.conrelid = 'users.external_caregivers'::regclass AND k.contype = 'f'
		ORDER BY k.conname`).Scan(ctx, &foreignKeys))
	assert.Equal(t, []string{
		"external_caregivers_tenant_id_fkey (tenant_id) -> platform.schools on delete a",
		"fk_external_caregivers_staff_tenant (tenant_id, staff_id) -> users.staff_school_memberships on delete c",
	}, foreignKeys, "the tenant key and the composite staff key survive the rename")

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	staffA := testpkg.CreateTestStaffForTenant(t, db, tenantA, "Extern", "Alpha")
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantB, "Extern", "Beta")
	insert := `INSERT INTO users.external_caregivers (tenant_id, staff_id, organization, activity_expertise)
		VALUES (?, ?, 'Musikschule', 'Musik')`
	_, err := db.ExecContext(ctx, insert, tenantA, staffA.ID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, insert, tenantA, staffA.ID)
	require.ErrorContains(t, err, "idx_external_caregivers_tenant_staff",
		"the store classifies a second record of the same staff member by this index name")
	_, err = db.ExecContext(ctx, insert, tenantB, staffA.ID)
	require.ErrorContains(t, err, "fk_external_caregivers_staff_tenant",
		"the composite key refuses a staff member of another school")

	assertTenantRowsIsolated(t, db, "users.external_caregivers", tenantA, insert, tenantB, staffB.ID)

	require.NoError(t, externalCaregiversTableDown(ctx, db))
	assert.False(t, externalCaregiverTableExists(t, db, "external_caregivers"))
	require.True(t, externalCaregiverTableExists(t, db, "guests"))
	relations, constraints, policies, triggers := externalCaregiverObjectNames(t, db, "guests")
	assert.Equal(t, []string{
		"guests_id_seq",
		"guests_pkey",
		"idx_guests_organization",
		"idx_guests_staff_id",
		"idx_guests_tenant",
		"idx_guests_tenant_id",
		"idx_guests_tenant_staff",
	}, relations)
	assert.Equal(t, []string{"chk_guests_tenant_id_not_null", "fk_guests_staff_tenant", "guests_pkey", "guests_tenant_id_fkey"}, constraints)
	assert.Equal(t, []string{"tenant_isolation_users_guests"}, policies)
	assert.Equal(t, []string{"update_guests_updated_at"}, triggers)
	var kept int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.guests WHERE staff_id = ?`, staffA.ID).Scan(ctx, &kept))
	assert.Equal(t, 1, kept, "the rollback keeps the rows")
	require.NoError(t, externalCaregiversTableDown(ctx, db), "the rollback is idempotent")

	require.NoError(t, externalCaregiversTableUp(ctx, db))
	assertRenamed(t)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.external_caregivers WHERE staff_id = ?`, staffA.ID).Scan(ctx, &kept))
	assert.Equal(t, 1, kept, "the rename keeps the rows")
	require.NoError(t, externalCaregiversTableUp(ctx, db), "a renamed table is left alone")
	assertRenamed(t)
}
