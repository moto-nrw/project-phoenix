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

func TestStaffRetentionAllowsOneDayNewerBerlinCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Retention", "Boundary")

	_, err := db.ExecContext(context.Background(), `
		UPDATE users.staff_school_memberships
		SET deleted_at = (((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 730)::timestamp
			AT TIME ZONE 'Europe/Berlin')
		WHERE id = ?`, staff.ID)
	require.NoError(t, err)

	err = withPhoenixTenantTx(t, authDB, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `
			SELECT users.erase_retired_staff_records(
				?,
				?,
				(((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 729)::timestamp
					AT TIME ZONE 'Europe/Berlin')
			)`, tenantID, staff.ID)
		return err
	})
	require.NoError(t, err, "the guard accepts the cutoff from an application already on the next Berlin day")
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
