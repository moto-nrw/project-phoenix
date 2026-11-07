package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestStudentMembershipAwaitsFirstCareBackfillsOnlyProvableRows runs the
// migration of #6041 over memberships from before it: only a pending row with
// a start that never changed since its insert is marked as a child without
// care; an edited, active or second membership stays unknown. The trigger
// then keeps a later write from marking a known child.
func TestStudentMembershipAwaitsFirstCareBackfillsOnlyProvableRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, studentMembershipAwaitsFirstCareDown(ctx, db))

	tenantID := testpkg.Tenant(t)
	insert := func(name, status string, from any) int64 {
		t.Helper()
		person := testpkg.CreateTestPerson(t, db, "Backfill", name)
		var profileID int64
		require.NoError(t, db.NewRaw(`WITH profile AS (
			INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?) RETURNING id, tenant_id
		) INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class, status, enrolled_from)
			SELECT tenant_id, id, '1a', ?, ?::date FROM profile RETURNING student_profile_id`,
			tenantID, person.ID, status, from).Scan(ctx, &profileID))
		return profileID
	}
	untouched := insert("Unberuehrt", "pending", "2035-02-03")
	edited := insert("Bearbeitet", "pending", "2035-02-03")
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET school_class = '1b' WHERE student_profile_id = ?`, edited)
	require.NoError(t, err)
	active := insert("Aktiv", "active", "2035-02-03")
	undated := insert("Ohne", "pending", nil)
	second := insert("Zweite", "pending", "2035-02-03")
	_, err = db.ExecContext(ctx, `INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class, status, deleted_at)
		VALUES (?, ?, '1a', 'inactive', NOW() - INTERVAL '1 year')`, tenantID, second)
	require.NoError(t, err)

	require.NoError(t, studentMembershipAwaitsFirstCareUp(ctx, db))
	awaits := func(profileID int64) bool {
		t.Helper()
		var value bool
		require.NoError(t, db.NewRaw(`SELECT awaits_first_care FROM users.student_school_memberships
			WHERE student_profile_id = ? AND deleted_at IS NULL`, profileID).Scan(ctx, &value))
		return value
	}
	assert.True(t, awaits(untouched), "a pending row unchanged since its insert never had care")
	assert.False(t, awaits(edited), "an edited row may have been a known child")
	assert.False(t, awaits(active))
	assert.False(t, awaits(undated))
	assert.False(t, awaits(second), "an earlier membership may have had care")
	var bumped int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_school_memberships
		WHERE student_profile_id = ? AND updated_at <> created_at`, untouched).Scan(ctx, &bumped))
	assert.Zero(t, bumped, "the backfill keeps updated_at")

	_, err = db.ExecContext(ctx, `UPDATE users.student_school_memberships SET awaits_first_care = true WHERE student_profile_id = ?`, active)
	require.NoError(t, err)
	assert.False(t, awaits(active), "only the trigger decides the fact")
	_, err = db.ExecContext(ctx, `UPDATE users.student_school_memberships SET status = 'active' WHERE student_profile_id = ?`, untouched)
	require.NoError(t, err)
	assert.False(t, awaits(untouched), "activation ends the wait for good")

	require.NoError(t, studentMembershipAwaitsFirstCareDown(ctx, db))
	require.NoError(t, studentMembershipAwaitsFirstCareUp(ctx, db), "the migration runs again after its down path")
}
