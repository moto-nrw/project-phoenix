package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPeriodScopedRosterUniquenessAllowsSamePersonAcrossScopes(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)

	ctx := context.Background()
	require.NoError(t, periodScopedRosterUniquenessUp(ctx, db))

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Roster", "Student", "3a")
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Roster", "Staff")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "RosterPeriodScoped")

	periodA := insertRosterUniquenessPeriod(t, db, tenantID, "Roster A")
	periodB := insertRosterUniquenessPeriod(t, db, tenantID, "Roster B")

	insertRosterEnrollment(t, db, tenantID, student.ID, group.ID, &periodA)
	insertRosterEnrollment(t, db, tenantID, student.ID, group.ID, &periodB)
	insertRosterEnrollment(t, db, tenantID, student.ID, group.ID, nil)

	insertRosterSupervisor(t, db, tenantID, staff.ID, group.ID, &periodA)
	insertRosterSupervisor(t, db, tenantID, staff.ID, group.ID, &periodB)
	insertRosterSupervisor(t, db, tenantID, staff.ID, group.ID, nil)

	require.Error(t, insertRosterEnrollmentErr(t, db, tenantID, student.ID, group.ID, &periodA))
	require.Error(t, insertRosterSupervisorErr(t, db, tenantID, staff.ID, group.ID, &periodA))
}

func insertRosterUniquenessPeriod(t *testing.T, db *bun.DB, tenantID int64, name string) int64 {
	t.Helper()

	var id int64
	err := db.NewRaw(`
		INSERT INTO schedule.calendar_periods (
			tenant_id, name, period_type, start_date, end_date, week_cycle_length, is_active
		)
		VALUES (?, ?, 'custom', '2026-01-01', '2026-12-31', 1, true)
		RETURNING id
	`, tenantID, name).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}

func insertRosterEnrollment(t *testing.T, db *bun.DB, tenantID, studentID, groupID int64, periodID *int64) {
	t.Helper()
	require.NoError(t, insertRosterEnrollmentErr(t, db, tenantID, studentID, groupID, periodID))
}

func insertRosterEnrollmentErr(t *testing.T, db *bun.DB, tenantID, studentID, groupID int64, periodID *int64) error {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO activities.student_enrollments (
			tenant_id, student_id, activity_group_id, valid_from, calendar_period_id
		)
		VALUES (?, ?, ?, '2026-01-01', ?)
	`, tenantID, studentID, groupID, periodID).Exec(context.Background())
	return err
}

func insertRosterSupervisor(t *testing.T, db *bun.DB, tenantID, staffID, groupID int64, periodID *int64) {
	t.Helper()
	require.NoError(t, insertRosterSupervisorErr(t, db, tenantID, staffID, groupID, periodID))
}

func insertRosterSupervisorErr(t *testing.T, db *bun.DB, tenantID, staffID, groupID int64, periodID *int64) error {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO activities.supervisors (
			tenant_id, staff_id, group_id, valid_from, calendar_period_id
		)
		VALUES (?, ?, ?, '2026-01-01', ?)
	`, tenantID, staffID, groupID, periodID).Exec(context.Background())
	return err
}
