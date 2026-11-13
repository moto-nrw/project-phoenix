package behaviortest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	activeSvc "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Check-in straight into a session (#6953): the app path an OGS uses when a
// terminal is offline. Attendance and room stay are written in one unit, so a
// child never shows as "Unterwegs" in between, and a rejected placement
// leaves no attendance behind.

// checkInPresence is the surface the HTTP adapter and the open-room workflow
// bind to.
type checkInPresence interface {
	CheckInStudentsToSessionAuthorized(ctx context.Context, studentIDs []int64, activeGroupID, staffID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
	CheckInStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID, staffID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
}

type checkInFixture struct {
	svc       activeSvc.Service
	checkIn   checkInPresence
	staff     int64
	colleague int64
	device    int64
	source    int64
	target    int64
}

func newCheckInFixture(t *testing.T, db *testpkg.DB) checkInFixture {
	t.Helper()
	testpkg.EnsureWebManualDevice(t, db)
	svc := setupActiveService(t, db)
	checkIn, ok := svc.(checkInPresence)
	require.True(t, ok, "the presence service must provide the check-in-to-session command")
	return checkInFixture{
		svc:       svc,
		checkIn:   checkIn,
		staff:     testpkg.CreateTestStaff(t, db, "Anmelden", "Team").ID,
		colleague: testpkg.CreateTestStaff(t, db, "Anmelden", "Kollegin").ID,
		device:    testpkg.CreateTestDevice(t, db, "checkin-to-session-device").ID,
		source: testpkg.CreateTestActiveGroup(t, db,
			testpkg.CreateTestActivityGroup(t, db, "checkin-to-session-source").ID,
			testpkg.CreateTestRoom(t, db, "Anmelden Quelle").ID).ID,
		target: testpkg.CreateTestActiveGroup(t, db,
			testpkg.CreateTestActivityGroup(t, db, "checkin-to-session-target").ID,
			testpkg.CreateTestRoom(t, db, "Anmelden Ziel").ID).ID,
	}
}

func (f checkInFixture) staffAuth() activeSvc.StudentMoveAuthorization {
	return activeSvc.StudentMoveAuthorization{StaffID: f.staff}
}

func openAttendanceCount(t *testing.T, db *testpkg.DB, studentIDs []int64) int {
	t.Helper()
	count, err := db.NewSelect().
		Table("active.attendance").
		Where("student_id IN (?) AND check_out_time IS NULL", bun.In(studentIDs)).
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}

func TestCheckInToSessionPlacesArrivalsInASupervisedRoom(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newCheckInFixture(t, db)
	ctx := testpkg.Ctx(t)
	// A colleague supervises the target; the caller supervises nothing. The
	// check-in right is school-wide (#2329), so arriving children may go to
	// any room with a running supervision, like a scan at its terminal.
	testpkg.CreateTestGroupSupervisor(t, db, f.colleague, f.target, "supervisor")
	first := testpkg.CreateTestStudent(t, db, "Mia", "Anmelden", "2a").ID
	second := testpkg.CreateTestStudent(t, db, "Ben", "Anmelden", "2a").ID

	result, err := f.checkIn.CheckInStudentsToSessionAuthorized(ctx, []int64{first, second, first}, f.target, f.staff, f.staffAuth())

	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{first, second}, result.CheckedIn)
	assert.ElementsMatch(t, []int64{first, second}, result.Moved)
	assert.Empty(t, result.Skipped)
	require.NotNil(t, result.ActiveGroupID)
	assert.Equal(t, f.target, *result.ActiveGroupID)
	assert.Equal(t, 2, openAttendanceCount(t, db, []int64{first, second}))
	sessions := openVisitSessions(t, testSchoolPresence(t, db), []int64{first, second})
	assert.Equal(t, map[int64]int64{first: f.target, second: f.target}, sessions,
		"the children must be in the room right away, not in transit")
}

func TestCheckInToSessionRejectsAnUnsupervisedRoomAndKeepsChildrenAbsent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newCheckInFixture(t, db)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Lea", "Ohne Aufsicht", "2b").ID

	_, err := f.checkIn.CheckInStudentsToSessionAuthorized(ctx, []int64{child}, f.target, f.staff, f.staffAuth())

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden,
		"a room without a responsible adult is no target; the app starts no supervision")
	assert.Zero(t, openAttendanceCount(t, db, []int64{child}),
		"a rejected placement must not leave the check-in behind")
}

func TestCheckInToSessionKeepsTheMoveRulesForPresentChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newCheckInFixture(t, db)
	ctx := testpkg.Ctx(t)
	testpkg.CreateTestGroupSupervisor(t, db, f.colleague, f.target, "supervisor")
	arriving := testpkg.CreateTestStudent(t, db, "Noah", "Neu", "3a").ID
	// Already present in a room the caller does not supervise: pulling it
	// out needs the ordinary move rights, which the check-in right does not
	// grant.
	present := presentChild(t, db, "Fremder Raum", f.staff, f.device, f.source)

	_, err := f.checkIn.CheckInStudentsToSessionAuthorized(ctx, []int64{arriving, present}, f.target, f.staff, f.staffAuth())

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Zero(t, openAttendanceCount(t, db, []int64{arriving}), "the whole unit rolls back")
	sessions := openVisitSessions(t, testSchoolPresence(t, db), []int64{present})
	assert.Equal(t, f.source, sessions[present], "the present child stays where it was")
}

func TestCheckInToSessionPullsPresentChildrenWithTheirSourceRights(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newCheckInFixture(t, db)
	ctx := testpkg.Ctx(t)
	// The caller supervises the target: the pull rule covers every child.
	testpkg.CreateTestGroupSupervisor(t, db, f.staff, f.target, "supervisor")
	arriving := testpkg.CreateTestStudent(t, db, "Ida", "Neu", "3b").ID
	present := presentChild(t, db, "Schon da", f.staff, f.device, f.source)

	result, err := f.checkIn.CheckInStudentsToSessionAuthorized(ctx, []int64{arriving, present}, f.target, f.staff, f.staffAuth())

	require.NoError(t, err)
	assert.Equal(t, []int64{arriving}, result.CheckedIn, "only the absent child is a new check-in")
	assert.ElementsMatch(t, []int64{arriving, present}, result.Moved)
	sessions := openVisitSessions(t, testSchoolPresence(t, db), []int64{arriving, present})
	assert.Equal(t, map[int64]int64{arriving: f.target, present: f.target}, sessions)
}

func TestCheckInToSessionSkipsChildrenItCannotCheckIn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newCheckInFixture(t, db)
	ctx := testpkg.Ctx(t)
	testpkg.CreateTestGroupSupervisor(t, db, f.colleague, f.target, "supervisor")
	child := testpkg.CreateTestStudent(t, db, "Emil", "Aktiv", "4a").ID
	graduate := testpkg.CreateTestStudent(t, db, "Ella", "Ehemalig", "4a").ID
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("status = ?", "alumnus").
		Where("student_profile_id = ? AND deleted_at IS NULL", graduate).
		Exec(ctx)
	require.NoError(t, err)

	result, err := f.checkIn.CheckInStudentsToSessionAuthorized(ctx, []int64{graduate, child}, f.target, f.staff, f.staffAuth())

	require.NoError(t, err)
	assert.Equal(t, []int64{child}, result.CheckedIn)
	assert.Equal(t, []int64{child}, result.Moved)
	assert.Equal(t, []activeSvc.StudentMoveSkipped{{StudentID: graduate, Reason: studentpresence.StudentMoveSkipNotFound}}, result.Skipped)
	assert.Zero(t, openAttendanceCount(t, db, []int64{graduate}))
}

func TestCheckInToOpenRoomSessionNeedsNoSupervision(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newCheckInFixture(t, db)
	ctx := testpkg.Ctx(t)
	_, presence := openRoomService(t, db)
	yard := testpkg.CreateTestRoom(t, db, "Anmelden Offener Raum")
	stay := testpkg.CreateTestActivityGroup(t, db, "Anmelden Raumaufenthalt")
	markActivitySystem(t, db, stay.ID)
	session, err := presence.EnsureOpenRoomSession(ctx, yard.ID, stay.ID)
	require.NoError(t, err)
	child := testpkg.CreateTestStudent(t, db, "Paul", "Hof", "1a").ID

	result, err := f.checkIn.CheckInStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, session.ID, f.staff, f.staffAuth())

	require.NoError(t, err)
	assert.Equal(t, []int64{child}, result.CheckedIn)
	assert.Equal(t, []int64{child}, result.Moved)
	sessions := openVisitSessions(t, testSchoolPresence(t, db), []int64{child})
	assert.Equal(t, session.ID, sessions[child])
}
