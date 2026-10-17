package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffRetentionMarksOnlyRetiredTombstones(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	live := testpkg.CreateTestStaff(t, db, "Erasure", "Live")

	_, err := db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET personal_data_erased_at = NOW() WHERE id = ?`, live.ID)
	assert.ErrorContains(t, err, "chk_staff_school_memberships_erased_retired", "a staff member on the staff cannot be marked erased")

	_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships
		SET deleted_at = NOW(), personal_data_erased_at = NOW() WHERE id = ?`, live.ID)
	assert.NoError(t, err, "a retired tombstone can be marked erased")
}

func TestStaffRetentionFunctionIsOnlyForTheTenantRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var grantees []string
	require.NoError(t, db.NewRaw(`
		SELECT grantee::regrole::text
		FROM pg_proc AS p, aclexplode(p.proacl)
		WHERE p.oid = 'users.erase_retired_staff_records(bigint, bigint, timestamptz)'::regprocedure
		  AND privilege_type = 'EXECUTE' AND grantee <> p.proowner
		ORDER BY 1`).Scan(context.Background(), &grantees))
	assert.Equal(t, []string{"phoenix_tenant"}, grantees, "only the tenant role may run the erasure")
}

func TestStaffRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	functionExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regprocedure('users.erase_retired_staff_records(bigint, bigint, timestamptz)') IS NOT NULL`).
			Scan(ctx, &exists))
		return exists
	}
	columnExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'users' AND table_name = 'staff_school_memberships' AND column_name = 'personal_data_erased_at')`).
			Scan(ctx, &exists))
		return exists
	}

	require.NoError(t, staffRetentionDown(ctx, db))
	assert.False(t, functionExists())
	assert.False(t, columnExists())

	require.NoError(t, staffRetentionUp(ctx, db))
	assert.True(t, functionExists())
	assert.True(t, columnExists())
}
