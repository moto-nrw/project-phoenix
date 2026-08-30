package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentEnrollmentSelectedWeekdaysStrictConstraint(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	require.NoError(t, studentEnrollmentSelectedWeekdaysStrictUp(context.Background(), db))

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Weekday", "Strict", "3a")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "WeekdayStrict")
	t.Cleanup(func() {
		testpkg.CleanupTenantTestData(t, db, tenantID)
		require.NoError(t, studentEnrollmentSelectedWeekdaysStrictUp(context.Background(), db))
	})

	require.NoError(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `[1,3,7]`))
	require.Error(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `[]`))
	require.Error(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `[0]`))
	require.Error(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `[8]`))
	require.Error(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `["1"]`))
	require.Error(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `[1.5]`))
	require.Error(t, insertSelectedWeekdaysForConstraintTest(db, tenantID, student.ID, group.ID, `[1,1]`))
}

func insertSelectedWeekdaysForConstraintTest(db *testpkg.DB, tenantID, studentID, groupID int64, selectedWeekdays string) error {
	_, err := db.NewRaw(`
		INSERT INTO activities.student_enrollments (
			tenant_id, student_id, activity_group_id, valid_from, valid_until, selected_weekdays
		)
		VALUES (?, ?, ?, '2026-01-01', '2026-01-02', ?::jsonb)
	`, tenantID, studentID, groupID, selectedWeekdays).Exec(context.Background())
	return err
}
