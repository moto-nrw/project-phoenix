package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestStudentMembershipFirstCareFromBackfillsOnlyProvableRows runs the
// migration of #6041 over memberships from before it: only a pending row with
// a start that never changed since its insert gets that start as its
// first-care bound; an edited, active, undated or second membership stays
// unknown, and updated_at stays as it was.
func TestStudentMembershipFirstCareFromBackfillsOnlyProvableRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, studentMembershipFirstCareFromDown(ctx, db))

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

	require.NoError(t, studentMembershipFirstCareFromUp(ctx, db))
	bound := func(profileID int64) *calendar.Date {
		t.Helper()
		var value *calendar.Date
		require.NoError(t, db.NewRaw(`SELECT first_care_from FROM users.student_school_memberships
			WHERE student_profile_id = ? AND deleted_at IS NULL`, profileID).Scan(ctx, &value))
		return value
	}
	require.NotNil(t, bound(untouched), "a pending row unchanged since its insert is a new child")
	assert.Equal(t, calendar.Date("2035-02-03"), *bound(untouched))
	assert.Nil(t, bound(edited), "an edited row may have been a known child")
	assert.Nil(t, bound(active))
	assert.Nil(t, bound(undated))
	assert.Nil(t, bound(second), "an earlier membership may have had care")
	var bumped int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_school_memberships
		WHERE student_profile_id = ? AND updated_at <> created_at`, untouched).Scan(ctx, &bumped))
	assert.Zero(t, bumped, "the backfill keeps updated_at")

	require.NoError(t, studentMembershipFirstCareFromDown(ctx, db))
	require.NoError(t, studentMembershipFirstCareFromUp(ctx, db), "the migration runs again after its down path")
}
