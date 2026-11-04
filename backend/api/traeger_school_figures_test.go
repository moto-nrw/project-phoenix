package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	studentPresenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// figuresDay is a fixed Berlin day; the owners' reads take the instant.
var figuresDay = calendar.NewDate(2031, time.March, 12)

func figuresNoon() time.Time { return figuresDay.BerlinMidnight().Add(12 * time.Hour) }

func insertFigureWorkSession(t *testing.T, db *bun.DB, school, staffID int64, day calendar.Date, status string, checkIn time.Time, checkOut *time.Time) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO active.work_sessions
		(tenant_id, staff_id, date, status, check_in_time, check_out_time, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		school, staffID, day, status, checkIn, checkOut, staffID)
	require.NoError(t, err)
}

// insertOpenVisit records an open room visit in a group of its own.
func insertOpenVisit(t *testing.T, db *bun.DB, school, studentID int64, entry time.Time) *testpkg.ActiveGroupRow {
	t.Helper()
	group := testpkg.CreateTestActiveGroupForTenant(t, db, school)
	testpkg.CreateTestVisitForTenant(t, db, school, studentID, group.ID, entry, nil)
	return group
}

func TestPresentStudentsListOpenStaysOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	other, _ := testpkg.CreateTestTenant(t, db)
	noon, earlier := figuresNoon(), figuresNoon().Add(-2*time.Hour)
	staff := testpkg.CreateTestStaffForTenant(t, db, school, "Bea", "Betreuung")
	device := testpkg.CreateTestDeviceForTenant(t, db, school, "figures")

	both := testpkg.CreateTestStudentForTenant(t, db, school, "Anna", "Anwesend", "1a")
	testpkg.CreateTestAttendanceForTenant(t, db, school, both.ID, staff.ID, device.ID, figuresDay, earlier, nil)
	insertOpenVisit(t, db, school, both.ID, earlier)
	inRoom := testpkg.CreateTestStudentForTenant(t, db, school, "Ria", "Raum", "1a")
	insertOpenVisit(t, db, school, inRoom.ID, earlier)
	endedSession := testpkg.CreateTestStudentForTenant(t, db, school, "Eva", "Beendet", "1a")
	endedGroup := insertOpenVisit(t, db, school, endedSession.ID, earlier)
	// A historical interrupted close can leave a visit without an exit time.
	_, err := db.ExecContext(context.Background(), `UPDATE active.groups SET end_time = ? WHERE id = ?`, noon.Add(-time.Hour), endedGroup.ID)
	require.NoError(t, err)
	home := testpkg.CreateTestStudentForTenant(t, db, school, "Hans", "Heim", "1a")
	testpkg.CreateTestAttendanceForTenant(t, db, school, home.ID, staff.ID, device.ID, figuresDay, earlier, &noon)
	yesterday := testpkg.CreateTestStudentForTenant(t, db, school, "Yara", "Gestern", "1a")
	testpkg.CreateTestAttendanceForTenant(t, db, school, yesterday.ID, staff.ID, device.ID, figuresDay.AddDays(-1), earlier.AddDate(0, 0, -1), nil)

	otherStaff := testpkg.CreateTestStaffForTenant(t, db, other, "Fritz", "Fremd")
	otherDevice := testpkg.CreateTestDeviceForTenant(t, db, other, "figures-other")
	foreign := testpkg.CreateTestStudentForTenant(t, db, other, "Fiona", "Fremd", "1a")
	testpkg.CreateTestAttendanceForTenant(t, db, other, foreign.ID, otherStaff.ID, otherDevice.ID, figuresDay, earlier, nil)
	insertOpenVisit(t, db, other, foreign.ID, earlier)

	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		present, err := studentPresenceCompose.NewPresentStudents().PresentStudentIDsByTenant(adminCtx, []int64{school}, noon)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{both.ID, inRoom.ID}, present[school], "attendance and visit of one child appear once; only open stays of the day; only the given school")
		return nil
	}))
	_, err = studentPresenceCompose.NewPresentStudents().PresentStudentIDsByTenant(testpkg.TenantContext(school), []int64{school}, noon)
	assert.Error(t, err, "outside the administrative transaction it refuses")
}

func TestSickStudentsReadBothMarks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)

	day := testpkg.CreateTestStudent(t, db, "Sina", "Krank", "1a")
	testpkg.CreateTestStudentStatusDay(t, db, day.ID, figuresDay, "sick")
	cleared := testpkg.CreateTestStudent(t, db, "Clara", "Gesund", "1a")
	clearedDay := testpkg.CreateTestStudentStatusDay(t, db, cleared.ID, figuresDay, "sick")
	_, err := db.ExecContext(context.Background(), `UPDATE active.student_status_days SET cleared_at = NOW() WHERE id = ?`, clearedDay.ID)
	require.NoError(t, err)
	other, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestStudentForTenant(t, db, other, "Fiona", "Fremd", "1a")
	_, err = db.ExecContext(context.Background(), `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source) VALUES (?, ?, ?, 'sick', NOW(), 'manual')`, other, foreign.ID, figuresDay)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE users.student_care_profiles SET sick = true WHERE tenant_id = ?`, other)
	require.NoError(t, err)
	excused := testpkg.CreateTestStudent(t, db, "Emil", "Entschuldigt", "1a")
	testpkg.CreateTestStudentStatusDay(t, db, excused.ID, figuresDay, "excused")
	flag := testpkg.CreateTestStudent(t, db, "Finn", "Krank", "1a")
	var membership int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ?`, school, flag.ID).Scan(context.Background(), &membership))
	_, err = db.ExecContext(context.Background(), `UPDATE users.student_care_profiles SET sick = true WHERE tenant_id = ? AND membership_id = ?`, school, membership)
	require.NoError(t, err)

	sick := carePlanCompose.NewSickStudents()
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		days, err := sick.SickDayStudentIDsByTenant(adminCtx, []int64{school}, figuresNoon())
		require.NoError(t, err)
		assert.Equal(t, map[int64][]int64{school: {day.ID}}, days, "no cleared sick day, no other status")
		flags, err := sick.SickFlagMembershipIDsByTenant(adminCtx, []int64{school})
		require.NoError(t, err)
		assert.Equal(t, map[int64][]int64{school: {membership}}, flags)
		return nil
	}))
	_, err = sick.SickFlagMembershipIDsByTenant(testpkg.TenantContext(school), []int64{school})
	assert.Error(t, err, "outside the administrative transaction it refuses")
}

func TestOnSiteStaffKeepsOpenSessionsOnSite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	other, _ := testpkg.CreateTestTenant(t, db)
	noon, earlier := figuresNoon(), figuresNoon().Add(-2*time.Hour)

	today := testpkg.CreateTestStaffForTenant(t, db, school, "Bea", "Betreuung")
	insertFigureWorkSession(t, db, school, today.ID, figuresDay, "present", earlier, nil)
	overnight := testpkg.CreateTestStaffForTenant(t, db, school, "Olga", "Nacht")
	insertFigureWorkSession(t, db, school, overnight.ID, figuresDay.AddDays(-1), "present", noon.Add(-11*time.Hour), nil)
	homeOffice := testpkg.CreateTestStaffForTenant(t, db, school, "Hilde", "Zuhause")
	insertFigureWorkSession(t, db, school, homeOffice.ID, figuresDay, "home_office", earlier, nil)
	gone := testpkg.CreateTestStaffForTenant(t, db, school, "Gert", "Gegangen")
	insertFigureWorkSession(t, db, school, gone.ID, figuresDay, "present", earlier, &noon)
	stale := testpkg.CreateTestStaffForTenant(t, db, school, "Stefan", "Vergessen")
	insertFigureWorkSession(t, db, school, stale.ID, figuresDay.AddDays(-3), "present", noon.AddDate(0, 0, -3), nil)
	foreign := testpkg.CreateTestStaffForTenant(t, db, other, "Fritz", "Fremd")
	insertFigureWorkSession(t, db, other, foreign.ID, figuresDay, "present", earlier, nil)

	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		staff, err := workforceCompose.NewOnSiteStaff().OnSiteStaffIDsByTenant(adminCtx, []int64{school}, noon)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{today.ID, overnight.ID}, staff[school], "on site, also across midnight; no home office, closed or stale session")
		assert.NotContains(t, staff, other)
		return nil
	}))
}

func TestSchoolRostersFollowTheStichtagRuleAndLeaveOutExternalCaregivers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	other, _ := testpkg.CreateTestTenant(t, db)

	counted := testpkg.CreateTestStudentForTenant(t, db, school, "Anna", "Aktiv", "1a")
	ended := testpkg.CreateTestStudentForTenant(t, db, school, "Emil", "Beendet", "1a")
	_, err := db.ExecContext(context.Background(), `UPDATE users.student_school_memberships SET enrolled_until = ? WHERE tenant_id = ? AND student_profile_id = ?`, figuresDay.AddDays(-1), school, ended.ID)
	require.NoError(t, err)
	testpkg.CreateTestStudentForTenant(t, db, other, "Fiona", "Fremd", "1a")

	own := testpkg.CreateTestStaffForTenant(t, db, school, "Bea", "Betreuung")
	external := testpkg.CreateTestStaffForTenant(t, db, school, "Erik", "Extern")
	_, err = db.ExecContext(context.Background(), `INSERT INTO users.external_caregivers (tenant_id, staff_id, organization, activity_expertise) VALUES (?, ?, 'Musikschule', 'Musik')`, school, external.ID)
	require.NoError(t, err)
	testpkg.CreateTestStaffForTenant(t, db, other, "Fritz", "Fremd")

	rosters := schoolMembershipCompose.NewSchoolRosters()
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		students, err := rosters.ActiveStudentMembershipsByTenant(adminCtx, []int64{school}, figuresNoon())
		require.NoError(t, err)
		assert.Len(t, students, 1, "only the given school")
		assert.Contains(t, students[school], counted.ID)
		assert.NotContains(t, students[school], ended.ID, "care that ended before the day does not count")

		byTenant, err := schoolMembershipCompose.NewActiveStudentCounts().CountActiveStudentsByTenant(adminCtx, figuresNoon())
		require.NoError(t, err)
		assert.Len(t, students[school], byTenant[school], "the same number as the operator billing report")

		staff, err := rosters.ActiveStaffIDsByTenant(adminCtx, []int64{school}, figuresNoon())
		require.NoError(t, err)
		assert.Equal(t, map[int64][]int64{school: {own.ID}}, staff, "no external caregiver, no other school")
		return nil
	}))
}
