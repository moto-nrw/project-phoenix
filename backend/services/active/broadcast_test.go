package active_test

import (
	"context"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/realtime"
	active "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingGuardianWaker struct {
	studentIDs []int64
}

func (w *recordingGuardianWaker) BroadcastChildUpdateToGuardians(_ int64, studentID int64) {
	w.studentIDs = append(w.studentIDs, studentID)
}

func setupServiceWithBroadcaster(t *testing.T) (active.Service, *testpkg.RecordingBroadcaster) {
	t.Helper()
	return newServiceWithBroadcaster(t, testpkg.SetupTestDB(t))
}

func newServiceWithBroadcaster(
	t *testing.T,
	db *bun.DB,
) (active.Service, *testpkg.RecordingBroadcaster) {
	t.Helper()

	repos := repositories.NewFactory(db)
	broadcaster := testpkg.NewRecordingBroadcaster()

	deps := active.ServiceDependencies{
		GroupRepo:          repos.ActiveGroup,
		SessionStartLock:   repos.SessionStartLock,
		VisitRepo:          repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		CombinedGroupRepo:  repos.CombinedGroup,
		GroupMappingRepo:   repos.GroupMapping,
		AttendanceRepo:     repos.Attendance,
		StudentRepo:        repos.Student,
		PersonRepo:         repos.Person,
		TeacherRepo:        repos.Teacher,
		StaffRepo:          repos.Staff,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActivityCatRepo:    repos.ActivityCategory,
		EducationGroupRepo: repos.Group,
		DeviceRepo:         repos.Device,
		DB:                 db,
		Broadcaster:        broadcaster,
		Logger:             slog.Default(),
	}
	svc := active.NewService(deps)

	return svc, broadcaster
}

func TestBroadcast_CreateVisitSendsOnePreciseRefresh(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "broadcast-checkin")
	room := testpkg.CreateTestRoom(t, db, "Broadcast Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-Broadcast-Checkin")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Student", "1a")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "Staff")
	iotDevice := testpkg.CreateTestDevice(t, db, "broadcast-device")
	// Students before their education group (FK ON DELETE SET NULL nulls
	// students.tenant_id otherwise — see the edu-batch test below).

	staffCtx := context.WithValue(testpkg.Ctx(t), device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, iotDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}

	err := svc.CreateVisit(deviceCtx, visit)
	require.NoError(t, err)

	assert.True(t, broadcaster.HasEventType(realtime.EventDashboardCountsChanged),
		"expected the combined refresh after CreateVisit")

	// #2057: the supervision refresh is tenant-scoped (not broadcast to every
	// school on the deployment) and carries the student's educational group id
	// so clients can scope their ogs-students-{gid} revalidation.
	eduGroupIDStr := strconv.FormatInt(eduGroup.ID, 10)
	refreshes := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, refreshes, 1, "CreateVisit must emit one aggregate refresh, not a paired dashboard event")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	assert.NotZero(t, refreshes[0].TenantID, "tenant id from the request context must reach the broadcast")
	assert.Equal(t, strconv.FormatInt(activeGroup.ID, 10), refreshes[0].Event.ActiveGroupID)
	require.NotNil(t, refreshes[0].Event.Data.GroupIDs, "combined refresh must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *refreshes[0].Event.Data.GroupIDs)
	require.NotNil(t, refreshes[0].Event.Data.Reason, "combined refresh must carry supervision semantics")
	assert.Equal(t, "student_moved", *refreshes[0].Event.Data.Reason)
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged),
		"CreateVisit must fold supervision invalidation into the combined refresh")
	assert.Empty(t, broadcaster.EventsOfType(realtime.EventActiveSupervisionChanged),
		"CreateVisit must not emit legacy compatibility frames")

	// The scoped student_checkin carries the edu group id too, so the client's
	// backpressure fallback path (checkin delivered, counts event dropped)
	// stays scoped.
	checkins := broadcaster.EventsOfType(realtime.EventStudentCheckIn)
	require.NotEmpty(t, checkins)
	require.NotNil(t, checkins[0].Data.GroupIDs, "student_checkin must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *checkins[0].Data.GroupIDs)
}

func tenantCallsOfType(broadcaster *testpkg.RecordingBroadcaster, eventType realtime.EventType) []testpkg.BroadcastCall {
	out := make([]testpkg.BroadcastCall, 0)
	for _, c := range broadcaster.CallsByMethod("tenant") {
		if c.Event.Type == eventType {
			out = append(out, c)
		}
	}
	return out
}

func TestBroadcast_EndVisitSendsOnePreciseRefresh(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "broadcast-checkout")
	room := testpkg.CreateTestRoom(t, db, "Broadcast Checkout Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-Broadcast-Checkout")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Checkout", "1b")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)

	// Clear calls from visit creation
	broadcaster.Reset()
	waker := &recordingGuardianWaker{}
	svc.(interface{ SetGuardianWaker(active.GuardianWaker) }).SetGuardianWaker(waker)

	err := svc.EndVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{student.ID}, waker.studentIDs)

	assert.True(t, broadcaster.HasEventType(realtime.EventDashboardCountsChanged),
		"expected the combined refresh after EndVisit")

	// #2057: tenant-scoped, carrying the student's educational group id.
	eduGroupIDStr := strconv.FormatInt(eduGroup.ID, 10)
	refreshes := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, refreshes, 1, "EndVisit must emit one aggregate refresh, not a paired dashboard event")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	assert.Equal(t, strconv.FormatInt(activeGroup.ID, 10), refreshes[0].Event.ActiveGroupID)
	require.NotNil(t, refreshes[0].Event.Data.GroupIDs, "combined refresh must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *refreshes[0].Event.Data.GroupIDs)
	require.NotNil(t, refreshes[0].Event.Data.Reason, "combined refresh must carry supervision semantics")
	assert.Equal(t, "student_moved", *refreshes[0].Event.Data.Reason)
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged),
		"EndVisit must fold supervision invalidation into the combined refresh")

	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts)
	require.NotNil(t, checkouts[0].Data.GroupIDs, "student_checkout must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *checkouts[0].Data.GroupIDs)
}

func TestBroadcast_EndVisitRunsAfterCommit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc, broadcaster := newServiceWithBroadcaster(t, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "end-visit-after-commit")
	room := testpkg.CreateTestRoom(t, db, "End Visit After Commit Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "EndVisit", "AfterCommit", "1a")
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	broadcaster.Reset()

	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if err := svc.EndVisit(txCtx, visit.ID); err != nil {
			return err
		}
		assert.Empty(t, broadcaster.Calls(), "the checkout event must wait for the transaction commit")
		return nil
	})
	require.NoError(t, err)
	assert.NotEmpty(t, broadcaster.EventsOfType(realtime.EventStudentCheckOut))
}

func TestBroadcast_UpdateVisitMoveSendsMovementEvents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := active.NewService(active.ServiceDependencies{
		GroupRepo:          repos.ActiveGroup,
		VisitRepo:          repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		CombinedGroupRepo:  repos.CombinedGroup,
		GroupMappingRepo:   repos.GroupMapping,
		AttendanceRepo:     repos.Attendance,
		StudentRepo:        repos.Student,
		PersonRepo:         repos.Person,
		TeacherRepo:        repos.Teacher,
		StaffRepo:          repos.Staff,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActivityCatRepo:    repos.ActivityCategory,
		EducationGroupRepo: repos.Group,
		DeviceRepo:         repos.Device,
		DB:                 db,
		Broadcaster:        broadcaster,
		Logger:             slog.Default(),
	})

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "broadcast-move-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "broadcast-move-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Broadcast Move Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Broadcast Move Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Move", "4a")
	visit := testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, time.Now(), nil)

	broadcaster.Reset()

	visit.ActiveGroupID = targetGroup.ID
	err := svc.UpdateVisit(testpkg.Ctx(t), visit)
	require.NoError(t, err)

	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts, "expected student_checkout for source group")
	assert.Equal(t, strconv.FormatInt(sourceGroup.ID, 10), checkouts[0].ActiveGroupID)

	checkins := broadcaster.EventsOfType(realtime.EventStudentCheckIn)
	require.NotEmpty(t, checkins, "expected student_checkin for target group")
	assert.Equal(t, strconv.FormatInt(targetGroup.ID, 10), checkins[0].ActiveGroupID)

	assert.Len(t, tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged), 2,
		"source checkout and target checkin each need one precise aggregate refresh")
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged),
		"visit move must fold both supervision invalidations into the combined refreshes")
}

func TestBroadcast_StartActivitySessionSendsOneCombinedRefresh(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "broadcast-session-start")
	room := testpkg.CreateTestRoom(t, db, "Broadcast Session Start Room")
	device := testpkg.CreateTestDevice(t, db, "broadcast-session-start-device")
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "SessionStarter")

	session, err := svc.StartActivitySessionWithSupervisors(
		testpkg.Ctx(t), activity.ID, device.ID, []int64{staff.ID}, &room.ID,
	)
	require.NoError(t, err)

	refreshes := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, refreshes, 1, "session start must emit one combined tenant refresh")
	assert.Equal(t, strconv.FormatInt(session.ID, 10), refreshes[0].Event.ActiveGroupID)
	require.NotNil(t, refreshes[0].Event.Data.Reason)
	assert.Equal(t, "activity_started", *refreshes[0].Event.Data.Reason)
	assert.Nil(t, refreshes[0].Event.Data.GroupIDs)
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged),
		"session start must not emit a separate supervision refresh")
}

func TestBroadcast_EndActivitySessionSendsBoundedRefreshes(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	room := testpkg.CreateTestRoom(t, db, "Broadcast End Room")
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "Ender")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "broadcast-end")
	session := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "BatchEnd", "2a")
	testpkg.CreateTestVisit(t, db, student.ID, session.ID, time.Now(), nil)

	broadcaster.Reset()

	err := svc.EndActivitySession(testpkg.Ctx(t), session.ID)
	require.NoError(t, err)

	// The batch checkout and activity end each emit one tenant-scoped refresh
	// (#2057). The student here has no educational group, so
	// group_ids must be ABSENT (nil, never an empty slice) — clients read the
	// absence as "scope unknown → refresh broadly".
	refreshes := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, refreshes, 2, "batch checkout and activity end each emit one bounded refresh")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	for _, c := range refreshes {
		assert.Nil(t, c.Event.Data.GroupIDs, "no educational group -> group_ids must be omitted entirely")
		assert.Equal(t, strconv.FormatInt(session.ID, 10), c.Event.ActiveGroupID)
		require.NotNil(t, c.Event.Data.Reason)
	}
	assert.Equal(t, "student_moved", *refreshes[0].Event.Data.Reason)
	assert.Equal(t, "activity_ended", *refreshes[1].Event.Data.Reason)
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged),
		"batch checkout and activity end must not emit separate supervision refreshes")
}

// TestBroadcast_EndActivitySessionEmitsActivityEndOnServeRole reproduces
// #2951. The service runs on the least-privilege phoenix_auth pool, the same
// identity the HTTP server uses. That role has no privileges outside a
// SET ROLE transaction, so a repository lookup from inside the after-commit
// hook failed with "permission denied for schema active" and activity_end
// was never sent. Every lookup the event needs has to happen inside the
// ending transaction; only the emission may run after the commit.
func TestBroadcast_EndActivitySessionEmitsActivityEndOnServeRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	serveDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { assert.NoError(t, serveDB.Close()) })
	svc, broadcaster := newServiceWithBroadcaster(t, serveDB)

	room := testpkg.CreateTestRoom(t, db, "Serve Role End Room")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "serve-role-end")
	session := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Serve", "RoleEnd", "3a")
	testpkg.CreateTestVisit(t, db, student.ID, session.ID, time.Now(), nil)

	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, serveDB))
	ctx = tenant.WithTenantID(ctx, testpkg.Tenant(t))
	broadcaster.Reset()

	require.NoError(t, svc.EndActivitySession(ctx, session.ID))

	ends := broadcaster.EventsOfType(realtime.EventActivityEnd)
	require.Len(t, ends, 1, "activity_end must reach the SSE client after the commit")
	require.NotNil(t, ends[0].Data.ActivityName)
	require.NotNil(t, ends[0].Data.RoomName)
	assert.Equal(t, activityGroup.Name, *ends[0].Data.ActivityName)
	assert.Equal(t, room.Name, *ends[0].Data.RoomName)
	assert.Equal(t, strconv.FormatInt(session.ID, 10), ends[0].ActiveGroupID)

	refreshes := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, refreshes, 2, "batch checkout and activity end each emit one refresh")
	assert.Equal(t, "activity_ended", *refreshes[1].Event.Data.Reason)
}

// TestBroadcast_EndActivitySessionBatchesCheckouts verifies the issue #848 fix:
// ending a session with multiple active visits emits ONE bulk_student_checkout
// per topic carrying every student ID, instead of one student_checkout per
// student. This is what keeps a single client's SSE channel buffer from
// overflowing during a whole-session checkout.
func TestBroadcast_EndActivitySessionBatchesCheckouts(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	room := testpkg.CreateTestRoom(t, db, "Broadcast Batch Room")
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "BatchEnder")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "broadcast-batch-end")
	session := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
	studentA := testpkg.CreateTestStudent(t, db, "Broadcast", "BatchA", "2a")
	studentB := testpkg.CreateTestStudent(t, db, "Broadcast", "BatchB", "2b")
	testpkg.CreateTestVisit(t, db, studentA.ID, session.ID, time.Now(), nil)
	testpkg.CreateTestVisit(t, db, studentB.ID, session.ID, time.Now(), nil)

	broadcaster.Reset()

	err := svc.EndActivitySession(testpkg.Ctx(t), session.ID)
	require.NoError(t, err)

	// No per-student student_checkout events on the bulk path — those would be
	// the 2N burst that overflowed the channel buffer.
	assert.Empty(t, broadcaster.EventsOfType(realtime.EventStudentCheckOut),
		"bulk session end must not emit per-student student_checkout events")

	// Exactly one bulk_student_checkout on the active-group topic, carrying both
	// students. (Edu-group topics get their own events only when students have an
	// OGS group assigned; the active-group topic always fires.)
	sessionIDStr := strconv.FormatInt(session.ID, 10)
	var activeTopicBulk *realtime.Event
	for _, e := range broadcaster.EventsOfType(realtime.EventBulkStudentCheckOut) {
		if e.ActiveGroupID == sessionIDStr {
			evt := e
			activeTopicBulk = &evt
		}
	}
	require.NotNil(t, activeTopicBulk,
		"expected a bulk_student_checkout on the active-group topic")
	require.NotNil(t, activeTopicBulk.Data.StudentIDs,
		"bulk event must carry student_ids")
	assert.ElementsMatch(t,
		[]string{strconv.FormatInt(studentA.ID, 10), strconv.FormatInt(studentB.ID, 10)},
		*activeTopicBulk.Data.StudentIDs,
		"active-topic bulk event must list every checked-out student")

	// Aggregate refreshes are constant per batch, not one-per-student: the
	// checkout batch fires one and broadcastActivityEndEvent fires one.
	assert.Len(t, tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged), 2,
		"session end fires a fixed number of aggregate refreshes regardless of student count")
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged))
}

// TestBroadcast_EndActivitySessionBatchesPerEducationGroup verifies the edu-group
// fan-out of the issue #848 batch: when checked-out students belong to different
// OGS groups, each edu:{groupID} topic must receive its OWN bulk_student_checkout
// carrying only that group's students — so a client watching one OGS group
// invalidates exactly its students' caches, not the whole session's. The
// active-group topic still carries every student.
func TestBroadcast_EndActivitySessionBatchesPerEducationGroup(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	ctx := context.Background()

	room := testpkg.CreateTestRoom(t, db, "Edu Batch Room")
	staff := testpkg.CreateTestStaff(t, db, "Edu", "BatchEnder")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "edu-batch-end")
	session := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")

	// Two OGS groups: A+B in groupX, C in groupY.
	groupX := testpkg.CreateTestEducationGroup(t, db, "OGS-Batch-X")
	groupY := testpkg.CreateTestEducationGroup(t, db, "OGS-Batch-Y")
	studentA := testpkg.CreateTestStudent(t, db, "Edu", "BatchA", "2a")
	studentB := testpkg.CreateTestStudent(t, db, "Edu", "BatchB", "2b")
	studentC := testpkg.CreateTestStudent(t, db, "Edu", "BatchC", "2c")
	assignStudentToEducationGroup(t, db, ctx, studentA.ID, groupX.ID)
	assignStudentToEducationGroup(t, db, ctx, studentB.ID, groupX.ID)
	assignStudentToEducationGroup(t, db, ctx, studentC.ID, groupY.ID)

	testpkg.CreateTestVisit(t, db, studentA.ID, session.ID, time.Now(), nil)
	testpkg.CreateTestVisit(t, db, studentB.ID, session.ID, time.Now(), nil)
	testpkg.CreateTestVisit(t, db, studentC.ID, session.ID, time.Now(), nil)
	// Students must be cleaned up before their education groups: the composite
	// students.group_id FK is ON DELETE SET NULL, and deleting a still-referenced
	// group nulls students.tenant_id (NOT NULL) and errors. Cleanup processes IDs
	// in order, so list students/visits ahead of the groups.

	broadcaster.Reset()

	err := svc.EndActivitySession(testpkg.Ctx(t), session.ID)
	require.NoError(t, err)

	idA := strconv.FormatInt(studentA.ID, 10)
	idB := strconv.FormatInt(studentB.ID, 10)
	idC := strconv.FormatInt(studentC.ID, 10)

	// edu:{groupX} gets one bulk event carrying A and B only.
	groupXCalls := broadcaster.GroupCallsForTopic("edu:" + strconv.FormatInt(groupX.ID, 10))
	require.Len(t, groupXCalls, 1, "groupX edu topic must receive exactly one bulk event")
	require.Equal(t, realtime.EventBulkStudentCheckOut, groupXCalls[0].Event.Type)
	require.NotNil(t, groupXCalls[0].Event.Data.StudentIDs)
	assert.ElementsMatch(t, []string{idA, idB}, *groupXCalls[0].Event.Data.StudentIDs,
		"groupX event must carry only groupX's students")

	// edu:{groupY} gets one bulk event carrying C only — not A/B.
	groupYCalls := broadcaster.GroupCallsForTopic("edu:" + strconv.FormatInt(groupY.ID, 10))
	require.Len(t, groupYCalls, 1, "groupY edu topic must receive exactly one bulk event")
	require.NotNil(t, groupYCalls[0].Event.Data.StudentIDs)
	assert.ElementsMatch(t, []string{idC}, *groupYCalls[0].Event.Data.StudentIDs,
		"groupY event must carry only groupY's student")

	// The active-group topic still carries every student — and every affected
	// educational group id (#2057).
	sessionIDStr := strconv.FormatInt(session.ID, 10)
	var activeTopicBulk *realtime.Event
	for _, e := range broadcaster.EventsOfType(realtime.EventBulkStudentCheckOut) {
		if e.ActiveGroupID == sessionIDStr && e.Data.StudentIDs != nil && len(*e.Data.StudentIDs) == 3 {
			evt := e
			activeTopicBulk = &evt
		}
	}
	require.NotNil(t, activeTopicBulk, "active-group topic must carry a bulk event with all students")
	assert.ElementsMatch(t, []string{idA, idB, idC}, *activeTopicBulk.Data.StudentIDs)

	gidX := strconv.FormatInt(groupX.ID, 10)
	gidY := strconv.FormatInt(groupY.ID, 10)
	require.NotNil(t, activeTopicBulk.Data.GroupIDs, "active-topic bulk event must carry the affected edu group ids")
	assert.ElementsMatch(t, []string{gidX, gidY}, *activeTopicBulk.Data.GroupIDs)

	// Each edu-topic bulk event carries only its own group id.
	require.NotNil(t, groupXCalls[0].Event.Data.GroupIDs)
	assert.Equal(t, []string{gidX}, *groupXCalls[0].Event.Data.GroupIDs)
	require.NotNil(t, groupYCalls[0].Event.Data.GroupIDs)
	assert.Equal(t, []string{gidY}, *groupYCalls[0].Event.Data.GroupIDs)

	// The batch's single aggregate refresh is tenant-scoped and lists every
	// affected edu group; the activity-end refresh right after carries none
	// (session end affects room occupancy across groups -> broad refresh).
	refreshes := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, refreshes, 2, "batch + activity end fire one aggregate refresh each")
	assert.Equal(t, sessionIDStr, refreshes[0].Event.ActiveGroupID)
	require.NotNil(t, refreshes[0].Event.Data.Reason)
	assert.Equal(t, "student_moved", *refreshes[0].Event.Data.Reason)
	require.NotNil(t, refreshes[0].Event.Data.GroupIDs, "batch refresh must carry the affected edu group ids")
	assert.ElementsMatch(t, []string{gidX, gidY}, *refreshes[0].Event.Data.GroupIDs)
	assert.Equal(t, sessionIDStr, refreshes[1].Event.ActiveGroupID)
	require.NotNil(t, refreshes[1].Event.Data.Reason)
	assert.Equal(t, "activity_ended", *refreshes[1].Event.Data.Reason)
	assert.Nil(t, refreshes[1].Event.Data.GroupIDs, "activity-end refresh is deliberately unscoped")
	assert.Empty(t, tenantCallsOfType(broadcaster, realtime.EventActiveSupervisionChanged))
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
}

// assignStudentToEducationGroup sets a student's OGS group_id in the DB so the
// SSE collection step (which reads student.GroupID) routes the student to its
// edu:{groupID} topic. Mirrors the assignment used in attendance_service_test.go.
func assignStudentToEducationGroup(tb testing.TB, db *bun.DB, ctx context.Context, studentID, groupID int64) {
	tb.Helper()
	_, err := db.NewUpdate().
		Table("users.students").
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(ctx)
	require.NoError(tb, err, "failed to assign student to education group")
}

// checkOutFixturedStudent opens an attendance row for the student and closes it
// through the real checkout path, so the roomless broadcast under test is the
// one production emits. The package clone owns the fixture rows.
func checkOutFixturedStudent(t *testing.T, db *bun.DB, svc active.Service, studentID int64, label string) {
	t.Helper()

	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "Staff"+label)
	iotDevice := testpkg.CreateTestDevice(t, db, "roomless-checkout-"+label)

	testpkg.CreateTestAttendance(t, db, studentID, staff.ID, iotDevice.ID, time.Now(), nil)

	_, err := svc.CheckOutStudent(testpkg.Ctx(t), studentID, staff.ID, true)
	require.NoError(t, err)

}

// TestBroadcast_RoomlessCheckoutSendsDashboardCounts covers the scope fallback:
// a child without an OGS group produces no group ids, and the contract says the
// field must then be absent entirely rather than an empty array — clients read
// an empty array as "scope to nothing" and drop the invalidation (#2057).
func TestBroadcast_RoomlessCheckoutSendsDashboardCounts(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Broadcast", "RoomlessCheckout", "3a")

	checkOutFixturedStudent(t, db, svc, student.ID, "NoGroup")

	counts := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, counts, 1, "expected dashboard_counts_changed after the checkout")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	assert.Nil(t, counts[0].Event.Data.GroupIDs, "no educational group -> group_ids must be omitted entirely")
}

func TestBroadcast_RoomlessCheckoutCarriesEducationGroupID(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-RoomlessCheckout")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "RoomlessCheckoutGrp", "3b")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)

	checkOutFixturedStudent(t, db, svc, student.ID, "WithGroup")

	eduGroupIDStr := strconv.FormatInt(eduGroup.ID, 10)
	counts := tenantCallsOfType(broadcaster, realtime.EventDashboardCountsChanged)
	require.Len(t, counts, 1)
	require.NotNil(t, counts[0].Event.Data.GroupIDs, "dashboard_counts_changed must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *counts[0].Event.Data.GroupIDs)

	// The educational-group student_checkout carries the group id too.
	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts)
	require.NotNil(t, checkouts[0].Data.GroupIDs)
	assert.Equal(t, []string{eduGroupIDStr}, *checkouts[0].Data.GroupIDs)
}

// TestBroadcast_TenantWideEventsCarryNoStudentIdentity pins the two halves of
// the #2085 contract on the check-in, check-out and whole-session-end paths:
//
//  1. nothing broadcast tenant-wide names the child. active_supervision_changed
//     used to carry a student_id, which handed every staff client of the school
//     the movement fact for a child their own API responses redact under
//     gdpr.student_data_scope = group_supervisors_only.
//  2. the group-scoped student_checkin / student_checkout still do carry it, so
//     the supervisors entitled to that child's data keep their per-child
//     detail-cache invalidation and their views stay live.
func TestBroadcast_TenantWideEventsCarryNoStudentIdentity(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	// A web check-in books attendance against the virtual WEB-MANUAL-001
	// device every real school is provisioned with.
	testpkg.EnsureWebManualDevice(t, db)

	activity := testpkg.CreateTestActivityGroup(t, db, "broadcast-gdpr")
	room := testpkg.CreateTestRoom(t, db, "Broadcast GDPR Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-Broadcast-GDPR")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Gdpr", "1c")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)

	ctx := testpkg.Ctx(t)
	studentIDStr := strconv.FormatInt(student.ID, 10)

	// --- check-in -----------------------------------------------------------
	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, svc.CreateVisit(ctx, visit))

	assertBothHalvesOfTheContract(t, broadcaster, realtime.EventStudentCheckIn, studentIDStr)

	// --- check-out ----------------------------------------------------------
	broadcaster.Reset()
	require.NoError(t, svc.EndVisit(ctx, visit.ID))

	assertBothHalvesOfTheContract(t, broadcaster, realtime.EventStudentCheckOut, studentIDStr)

	// --- whole-session end (bulk path) --------------------------------------
	broadcaster.Reset()
	testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	require.NoError(t, svc.EndActivitySession(ctx, activeGroup.ID))

	assertBothHalvesOfTheContract(t, broadcaster, realtime.EventBulkStudentCheckOut, studentIDStr)
}

// assertBothHalvesOfTheContract checks one flow's broadcasts: nothing
// tenant-wide names the child, a refresh actually fired (carrying the reason
// clients branch on instead of an id), and the group-scoped movement event
// still names the child for the entitled supervisors.
func assertBothHalvesOfTheContract(
	tb testing.TB,
	broadcaster *testpkg.RecordingBroadcaster,
	groupScopedEvent realtime.EventType,
	studentIDStr string,
) {
	tb.Helper()

	testpkg.AssertNoTenantWideStudentIdentity(tb, broadcaster)

	refreshes := broadcaster.EventsOfType(realtime.EventDashboardCountsChanged)
	require.NotEmpty(tb, refreshes, "expected a combined dashboard/supervision refresh")
	for _, e := range refreshes {
		assert.NotNil(tb, e.Data.Reason, "the refresh reason is what clients branch on instead of an id")
	}

	assertGroupScopedEventNamesStudent(tb, broadcaster, groupScopedEvent, studentIDStr)
}

// assertGroupScopedEventNamesStudent checks the other half of the contract: the
// entitled supervisors' topic still receives the child's id, either as
// student_id (single events) or in student_ids (bulk checkout).
func assertGroupScopedEventNamesStudent(
	tb testing.TB,
	broadcaster *testpkg.RecordingBroadcaster,
	eventType realtime.EventType,
	studentIDStr string,
) {
	tb.Helper()
	for _, call := range broadcaster.CallsByMethod("group") {
		if call.Event.Type != eventType {
			continue
		}
		if call.Event.Data.StudentID != nil && *call.Event.Data.StudentID == studentIDStr {
			return
		}
		if call.Event.Data.StudentIDs != nil {
			assert.Contains(tb, *call.Event.Data.StudentIDs, studentIDStr)
			return
		}
	}
	tb.Errorf("expected a group-scoped %s naming student %s", eventType, studentIDStr)
}
