package presence_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeSvc "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// webExceedParticipantLimitKey is the #3632 setting key. The architecture
// policy keeps the settings packages out of this test, so it is spelled out;
// TestAllSettingsRegistered pins the same string on the registry side.
const webExceedParticipantLimitKey = "attendance.web_exceed_participant_limit_enabled"

// forbidWebOverbooking stores the tenant override that turns off exceeding
// the participant limit on the web (#3632), through the real settings key.
func forbidWebOverbooking(t *testing.T, db *testpkg.DB) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, ?, 'false', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t), webExceedParticipantLimitKey).Exec(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`,
			testpkg.Tenant(t), webExceedParticipantLimitKey).Exec(context.Background())
	})
}

// limitFixture is one running session of a limited activity with some
// children already in it, plus children checked in and waiting in transit.
type limitFixture struct {
	session   *testpkg.ActiveGroupRow
	room      int64
	staff     int64
	device    int64
	inTransit []int64
}

func newLimitFixture(t *testing.T, db *testpkg.DB, name string, maxParticipants, present, transit int) limitFixture {
	t.Helper()
	now := time.Now()
	activity := testpkg.CreateTestActivityGroup(t, db, name)
	// 0 means no limit, which the schema stores as NULL, never as 0.
	var limit any
	if maxParticipants > 0 {
		limit = maxParticipants
	}
	_, err := db.NewRaw(`UPDATE activities.groups SET max_participants = ? WHERE id = ?`, limit, activity.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, db, name+" Room")
	session := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Limit", name)
	device := testpkg.CreateTestDevice(t, db, name+"-device")
	for i := 0; i < present; i++ {
		student := testpkg.CreateTestStudent(t, db, "Present", name, "L1")
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-time.Hour), nil)
		testpkg.CreateTestVisit(t, db, student.ID, session.ID, now.Add(-30*time.Minute), nil)
	}
	fixture := limitFixture{session: session, room: room.ID, staff: staff.ID, device: device.ID}
	for i := 0; i < transit; i++ {
		student := testpkg.CreateTestStudent(t, db, "Transit", name, "L1")
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-time.Hour), nil)
		fixture.inTransit = append(fixture.inTransit, student.ID)
	}
	return fixture
}

func requireLimitRefusal(t *testing.T, err error, current, maxParticipants, incoming int) {
	t.Helper()
	require.ErrorIs(t, err, activeSvc.ErrActivityParticipantLimitExceeded)
	require.NotErrorIs(t, err, activeSvc.ErrRoomCapacityExceeded)
	var limitErr *activeSvc.ActivityParticipantLimitError
	require.True(t, errors.As(err, &limitErr))
	assert.Equal(t, current, limitErr.CurrentOccupancy)
	assert.Equal(t, maxParticipants, limitErr.MaxParticipants)
	assert.Equal(t, incoming, limitErr.Incoming)
	assert.NotEmpty(t, limitErr.ActivityName)
	assert.NotZero(t, limitErr.ActivityID)
}

func openVisitsIn(t *testing.T, service activeSvc.Service, activeGroupID int64) int {
	t.Helper()
	count, err := service.CountActiveVisitsByActiveGroupID(testpkg.Ctx(t), activeGroupID)
	require.NoError(t, err)
	return count
}

// Each case is its own top-level test: the test tenant, and with it the
// stored setting, is shared by all subtests of one top-level test.

func TestWebParticipantLimit_TransitDefaultAllowsExceeding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	fixture := newLimitFixture(t, db, "limit-transit-default", 2, 1, 3)

	result, err := service.AssignTransitStudentsToActiveGroup(testpkg.Ctx(t), fixture.inTransit, fixture.session.ID)

	require.NoError(t, err)
	assert.ElementsMatch(t, fixture.inTransit, result.Assigned)
	assert.Equal(t, 4, openVisitsIn(t, service, fixture.session.ID))
}

func TestWebParticipantLimit_TransitRefusesWholeBulkAssignment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	fixture := newLimitFixture(t, db, "limit-transit-refused", 3, 1, 3)

	_, err := service.AssignTransitStudentsToActiveGroup(testpkg.Ctx(t), fixture.inTransit, fixture.session.ID)

	requireLimitRefusal(t, err, 1, 3, 3)
	assert.Equal(t, 1, openVisitsIn(t, service, fixture.session.ID), "no child may be assigned when not all fit")
	transit, err := service.ListStudentsInTransit(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Subset(t, transit, fixture.inTransit)
}

func TestWebParticipantLimit_TransitFillsLastPlaces(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	fixture := newLimitFixture(t, db, "limit-transit-fits", 3, 1, 2)

	result, err := service.AssignTransitStudentsToActiveGroup(testpkg.Ctx(t), fixture.inTransit, fixture.session.ID)

	require.NoError(t, err)
	assert.ElementsMatch(t, fixture.inTransit, result.Assigned)
}

func TestWebParticipantLimit_TransitWithoutLimit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	fixture := newLimitFixture(t, db, "limit-transit-unlimited", 0, 1, 3)

	result, err := service.AssignTransitStudentsToActiveGroup(testpkg.Ctx(t), fixture.inTransit, fixture.session.ID)

	require.NoError(t, err)
	assert.Len(t, result.Assigned, 3)
}

func TestWebParticipantLimit_TransitChecksRoomCapacityFirst(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	fixture := newLimitFixture(t, db, "limit-transit-room-first", 2, 1, 3)
	testpkg.SetRoomCapacity(t, db, fixture.room, 2)

	_, err := service.AssignTransitStudentsToActiveGroup(testpkg.Ctx(t), fixture.inTransit, fixture.session.ID)

	require.ErrorIs(t, err, activeSvc.ErrRoomCapacityExceeded)
	require.NotErrorIs(t, err, activeSvc.ErrActivityParticipantLimitExceeded)
}

// TestWebParticipantLimit_ConcurrentAssignments pins the lock analysis: both
// assignments lock the target session row before counting, so they count
// one after the other and cannot fill the last place twice.
func TestWebParticipantLimit_ConcurrentAssignments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	fixture := newLimitFixture(t, db, "limit-transit-race", 2, 1, 2)

	errs := make([]error, len(fixture.inTransit))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, studentID := range fixture.inTransit {
		wg.Add(1)
		go func(i int, studentID int64) {
			defer wg.Done()
			<-start
			_, errs[i] = service.AssignTransitStudentsToActiveGroup(testpkg.Ctx(t), []int64{studentID}, fixture.session.ID)
		}(i, studentID)
	}
	close(start)
	wg.Wait()

	refused := 0
	for _, err := range errs {
		if err != nil {
			requireLimitRefusal(t, err, 2, 2, 1)
			refused++
		}
	}
	assert.Equal(t, 1, refused, "exactly one of the two assignments may take the last place")
	assert.Equal(t, 2, openVisitsIn(t, service, fixture.session.ID))
}

func TestWebParticipantLimit_MoveBetweenSessions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	target := newLimitFixture(t, db, "limit-move-target", 1, 1, 0)

	// The source session shares the target's room: the room count does not
	// change, the activity count does.
	source := testpkg.CreateTestActiveGroup(t, db, testpkg.CreateTestActivityGroup(t, db, "limit-move-source").ID, target.room)
	student := testpkg.CreateTestStudent(t, db, "Mover", "Limit", "L1")
	testpkg.CreateTestAttendance(t, db, student.ID, target.staff, target.device, time.Now().Add(-time.Hour), nil)
	testpkg.CreateTestVisit(t, db, student.ID, source.ID, time.Now().Add(-20*time.Minute), nil)

	_, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, target.session.ID, activeSvcBypassAuth)

	requireLimitRefusal(t, err, 1, 1, 1)
	visit, err := service.GetStudentCurrentVisit(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, source.ID, visit.ActiveGroupID, "the refused move keeps the child where it was")
}

func TestWebParticipantLimit_ManualCheckIn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fixture := newLimitFixture(t, db, "limit-manual", 1, 1, 1)
	staffCtx := services.WithAttendanceStaff(ctx, fixture.staff, testpkg.Tenant(t))

	visit := &studentpresence.Visit{StudentID: fixture.inTransit[0], ActiveGroupID: fixture.session.ID, EntryTime: time.Now()}
	err := service.CreateVisit(staffCtx, visit)

	requireLimitRefusal(t, err, 1, 1, 1)
	assert.Zero(t, visit.ID)
	assert.Equal(t, 1, openVisitsIn(t, service, fixture.session.ID))
}

// TestWebParticipantLimit_TerminalUnchanged pins that the web setting never
// reaches a kiosk request: the terminal enforces the limit in its own scan
// path before the visit gets here, with its own error contract.
func TestWebParticipantLimit_TerminalUnchanged(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fixture := newLimitFixture(t, db, "limit-terminal", 1, 1, 1)
	staffCtx := services.WithAttendanceStaff(ctx, fixture.staff, testpkg.Tenant(t))
	deviceCtx := services.WithAttendanceDevice(staffCtx, fixture.device, testpkg.Tenant(t))

	visit := &studentpresence.Visit{StudentID: fixture.inTransit[0], ActiveGroupID: fixture.session.ID, EntryTime: time.Now()}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))
	assert.NotZero(t, visit.ID)
}

func TestWebParticipantLimit_VisitTransfer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	target := newLimitFixture(t, db, "limit-transfer-target", 1, 1, 0)
	source := testpkg.CreateTestActiveGroup(t, db, testpkg.CreateTestActivityGroup(t, db, "limit-transfer-source").ID, target.room)
	student := testpkg.CreateTestStudent(t, db, "Transfer", "Limit", "L1")
	testpkg.CreateTestAttendance(t, db, student.ID, target.staff, target.device, time.Now().Add(-time.Hour), nil)
	stored := testpkg.CreateTestVisit(t, db, student.ID, source.ID, time.Now().Add(-20*time.Minute), nil)

	moved := &studentpresence.Visit{
		ID: stored.ID, TenantID: stored.TenantID, StudentID: student.ID,
		ActiveGroupID: target.session.ID, EntryTime: stored.EntryTime,
	}
	err := service.UpdateVisit(ctx, moved)

	requireLimitRefusal(t, err, 1, 1, 1)
	visit, err := service.GetStudentCurrentVisit(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, source.ID, visit.ActiveGroupID)
}

func TestWebParticipantLimit_BulkMoveMovesNobody(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	target := newLimitFixture(t, db, "limit-bulk-move-target", 2, 1, 0)
	source := testpkg.CreateTestActiveGroup(t, db, testpkg.CreateTestActivityGroup(t, db, "limit-bulk-move-source").ID,
		testpkg.CreateTestRoom(t, db, "limit-bulk-move-source Room").ID)
	first := presentChild(t, db, "BulkEins", target.staff, target.device, source.ID)
	second := presentChild(t, db, "BulkZwei", target.staff, target.device, source.ID)

	_, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{first, second}, target.session.ID, activeSvcBypassAuth)

	requireLimitRefusal(t, err, 1, 2, 2)
	assert.Equal(t, 2, openVisitsIn(t, service, source.ID), "no child may move when not all fit")
	assert.Equal(t, 1, openVisitsIn(t, service, target.session.ID))
}

func TestWebParticipantLimit_OpenRoomMove(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service, openRoom := openRoomService(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "OffenerRaum", "Limit")
	device := testpkg.CreateTestDevice(t, db, "limit-open-room")
	source := testpkg.CreateTestActiveGroup(t, db, testpkg.CreateTestActivityGroup(t, db, "limit-open-room-source").ID,
		testpkg.CreateTestRoom(t, db, "limit-open-room-source Room").ID)
	roomActivity := testpkg.CreateTestActivityGroupWithLimit(t, db, "limit-open-room-stay", 1)
	roomSession, err := openRoom.EnsureOpenRoomSession(ctx, testpkg.CreateTestRoom(t, db, "limit-open-room Werkraum").ID, roomActivity.ID)
	require.NoError(t, err)
	presentChild(t, db, "Drinnen", staff.ID, device.ID, roomSession.ID)
	child := presentChild(t, db, "Draußen", staff.ID, device.ID, source.ID)

	_, err = openRoom.MoveStudentsToOpenRoomSessionAuthorized(ctx, []int64{child}, roomSession.ID, activeSvcBypassAuth)

	requireLimitRefusal(t, err, 1, 1, 1)
	visit, err := service.GetStudentCurrentVisit(ctx, child)
	require.NoError(t, err)
	assert.Equal(t, source.ID, visit.ActiveGroupID)
}

func TestWebParticipantLimit_VisitReopen(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fixture := newLimitFixture(t, db, "limit-reopen", 1, 1, 0)
	student := testpkg.CreateTestStudent(t, db, "Reopen", "Limit", "L1")
	entry := time.Now().Add(-40 * time.Minute)
	exit := time.Now().Add(-10 * time.Minute)
	testpkg.CreateTestAttendance(t, db, student.ID, fixture.staff, fixture.device, time.Now().Add(-time.Hour), nil)
	closed := testpkg.CreateTestVisit(t, db, student.ID, fixture.session.ID, entry, &exit)

	reopened := &studentpresence.Visit{
		ID: closed.ID, TenantID: closed.TenantID, StudentID: student.ID,
		ActiveGroupID: fixture.session.ID, EntryTime: closed.EntryTime,
	}
	err := service.UpdateVisit(ctx, reopened)

	requireLimitRefusal(t, err, 1, 1, 1)
	assert.Equal(t, 1, openVisitsIn(t, service, fixture.session.ID))
}

func TestWebParticipantLimit_SessionActivitySwitch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	forbidWebOverbooking(t, db)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fixture := newLimitFixture(t, db, "limit-switch-source", 0, 3, 0)
	smaller := testpkg.CreateTestActivityGroupWithLimit(t, db, "limit-switch-target", 2)

	session, err := service.GetActiveGroup(ctx, fixture.session.ID)
	require.NoError(t, err)
	session.GroupID = &smaller.ID
	err = service.UpdateActiveGroup(ctx, session)

	requireLimitRefusal(t, err, 0, 2, 3)
	unchanged, err := service.GetActiveGroup(ctx, fixture.session.ID)
	require.NoError(t, err)
	assert.NotEqual(t, smaller.ID, *unchanged.GroupID)
}
