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
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupServiceWithBroadcaster(t *testing.T) (active.Service, *testpkg.RecordingBroadcaster) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

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

	return svc, broadcaster
}

func TestBroadcast_CreateVisitSendsDashboardCounts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, eduGroup.ID, staff.ID, iotDevice.ID)

	staffCtx := context.WithValue(testpkg.TenantContext(1), device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, iotDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}

	err := svc.CreateVisit(deviceCtx, visit)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

	assert.True(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after CreateVisit")

	// #2057: the dashboard refresh is tenant-scoped (not broadcast to every
	// school on the deployment) and carries the student's educational group id
	// so clients can scope their ogs-students-{gid} revalidation.
	eduGroupIDStr := strconv.FormatInt(eduGroup.ID, 10)
	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1, "expected exactly one tenant-scoped dashboard_counts_changed after CreateVisit")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	assert.NotZero(t, counts[0].TenantID, "tenant id from the request context must reach the broadcast")
	require.NotNil(t, counts[0].Event.Data.GroupIDs, "dashboard_counts_changed must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *counts[0].Event.Data.GroupIDs)

	// The scoped student_checkin carries the edu group id too, so the client's
	// backpressure fallback path (checkin delivered, counts event dropped)
	// stays scoped.
	checkins := broadcaster.EventsOfType(realtime.EventStudentCheckIn)
	require.NotEmpty(t, checkins)
	require.NotNil(t, checkins[0].Data.GroupIDs, "student_checkin must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *checkins[0].Data.GroupIDs)
}

// dashboardCountsTenantCalls returns every tenant-scoped
// dashboard_counts_changed broadcast recorded so far.
func dashboardCountsTenantCalls(broadcaster *testpkg.RecordingBroadcaster) []testpkg.BroadcastCall {
	out := make([]testpkg.BroadcastCall, 0)
	for _, c := range broadcaster.CallsByMethod("tenant") {
		if c.Event.Type == realtime.EventDashboardCountsChanged {
			out = append(out, c)
		}
	}
	return out
}

func TestBroadcast_EndVisitSendsDashboardCounts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, db, "broadcast-checkout")
	room := testpkg.CreateTestRoom(t, db, "Broadcast Checkout Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-Broadcast-Checkout")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Checkout", "1b")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID, eduGroup.ID)

	// Clear calls from visit creation
	broadcaster.Reset()

	err := svc.EndVisit(testpkg.TenantContext(1), visit.ID)
	require.NoError(t, err)

	assert.True(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after EndVisit")

	// #2057: tenant-scoped, carrying the student's educational group id.
	eduGroupIDStr := strconv.FormatInt(eduGroup.ID, 10)
	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1, "expected exactly one tenant-scoped dashboard_counts_changed after EndVisit")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	require.NotNil(t, counts[0].Event.Data.GroupIDs, "dashboard_counts_changed must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *counts[0].Event.Data.GroupIDs)

	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts)
	require.NotNil(t, checkouts[0].Data.GroupIDs, "student_checkout must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *checkouts[0].Data.GroupIDs)
}

func TestBroadcast_UpdateVisitMoveSendsMovementEvents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	defer testpkg.CleanupActivityFixtures(t, db,
		sourceActivity.ID, targetActivity.ID,
		sourceRoom.ID, targetRoom.ID,
		sourceGroup.ID, targetGroup.ID,
		student.ID, visit.ID,
	)

	broadcaster.Reset()

	visit.ActiveGroupID = targetGroup.ID
	err := svc.UpdateVisit(testpkg.TenantContext(1), visit)
	require.NoError(t, err)

	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts, "expected student_checkout for source group")
	assert.Equal(t, strconv.FormatInt(sourceGroup.ID, 10), checkouts[0].ActiveGroupID)

	checkins := broadcaster.EventsOfType(realtime.EventStudentCheckIn)
	require.NotEmpty(t, checkins, "expected student_checkin for target group")
	assert.Equal(t, strconv.FormatInt(targetGroup.ID, 10), checkins[0].ActiveGroupID)

	assert.True(t, broadcaster.HasEventType(realtime.EventDashboardCountsChanged),
		"expected dashboard_counts_changed after visit move")
	assert.True(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after visit move")
}

func TestBroadcast_EndActivitySessionSendsDashboardCounts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	room := testpkg.CreateTestRoom(t, db, "Broadcast End Room")
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "Ender")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "broadcast-end")
	session := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "BatchEnd", "2a")
	visit := testpkg.CreateTestVisit(t, db, student.ID, session.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, room.ID, staff.ID, activityGroup.ID, session.ID, student.ID, visit.ID)

	broadcaster.Reset()

	err := svc.EndActivitySession(testpkg.TenantContext(1), session.ID)
	require.NoError(t, err)

	// Should have dashboard_counts_changed from the batch student checkout,
	// tenant-scoped (#2057). The student here has no educational group, so
	// group_ids must be ABSENT (nil, never an empty slice) — clients read the
	// absence as "scope unknown → refresh broadly".
	counts := dashboardCountsTenantCalls(broadcaster)
	require.NotEmpty(t, counts, "expected dashboard_counts_changed after EndActivitySession with active visits")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	for _, c := range counts {
		assert.Empty(t, c.Event.ActiveGroupID, "tenant-wide events should not leak group IDs in the topic")
		assert.Nil(t, c.Event.Data.GroupIDs, "no educational group -> group_ids must be omitted entirely")
	}
	assert.True(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after EndActivitySession")
}

// TestBroadcast_EndActivitySessionBatchesCheckouts verifies the issue #848 fix:
// ending a session with multiple active visits emits ONE bulk_student_checkout
// per topic carrying every student ID, instead of one student_checkout per
// student. This is what keeps a single client's SSE channel buffer from
// overflowing during a whole-session checkout.
func TestBroadcast_EndActivitySessionBatchesCheckouts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	room := testpkg.CreateTestRoom(t, db, "Broadcast Batch Room")
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "BatchEnder")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "broadcast-batch-end")
	session := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
	studentA := testpkg.CreateTestStudent(t, db, "Broadcast", "BatchA", "2a")
	studentB := testpkg.CreateTestStudent(t, db, "Broadcast", "BatchB", "2b")
	visitA := testpkg.CreateTestVisit(t, db, studentA.ID, session.ID, time.Now(), nil)
	visitB := testpkg.CreateTestVisit(t, db, studentB.ID, session.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db,
		room.ID, staff.ID, activityGroup.ID, session.ID,
		studentA.ID, studentB.ID, visitA.ID, visitB.ID,
	)

	broadcaster.Reset()

	err := svc.EndActivitySession(testpkg.TenantContext(1), session.ID)
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

	// Dashboard refreshes are constant per batch, not one-per-student: the
	// checkout batch fires one and broadcastActivityEndEvent fires one, so two
	// students still yield two — independent of how many were checked out.
	assert.Len(t, broadcaster.EventsOfType(realtime.EventDashboardCountsChanged), 2,
		"session end fires a fixed number of dashboard refreshes regardless of student count")
}

// TestBroadcast_EndActivitySessionBatchesPerEducationGroup verifies the edu-group
// fan-out of the issue #848 batch: when checked-out students belong to different
// OGS groups, each edu:{groupID} topic must receive its OWN bulk_student_checkout
// carrying only that group's students — so a client watching one OGS group
// invalidates exactly its students' caches, not the whole session's. The
// active-group topic still carries every student.
func TestBroadcast_EndActivitySessionBatchesPerEducationGroup(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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

	visitA := testpkg.CreateTestVisit(t, db, studentA.ID, session.ID, time.Now(), nil)
	visitB := testpkg.CreateTestVisit(t, db, studentB.ID, session.ID, time.Now(), nil)
	visitC := testpkg.CreateTestVisit(t, db, studentC.ID, session.ID, time.Now(), nil)
	// Students must be cleaned up before their education groups: the composite
	// students.group_id FK is ON DELETE SET NULL, and deleting a still-referenced
	// group nulls students.tenant_id (NOT NULL) and errors. Cleanup processes IDs
	// in order, so list students/visits ahead of the groups.
	defer testpkg.CleanupActivityFixtures(t, db,
		studentA.ID, studentB.ID, studentC.ID,
		visitA.ID, visitB.ID, visitC.ID,
		groupX.ID, groupY.ID,
		session.ID, activityGroup.ID, room.ID, staff.ID,
	)

	broadcaster.Reset()

	err := svc.EndActivitySession(testpkg.TenantContext(1), session.ID)
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

	// The batch's single dashboard refresh is tenant-scoped and lists every
	// affected edu group; the activity-end refresh right after carries none
	// (session end affects room occupancy across groups -> broad refresh).
	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 2, "batch + activity end fire one dashboard refresh each")
	require.NotNil(t, counts[0].Event.Data.GroupIDs, "batch dashboard refresh must carry the affected edu group ids")
	assert.ElementsMatch(t, []string{gidX, gidY}, *counts[0].Event.Data.GroupIDs)
	assert.Nil(t, counts[1].Event.Data.GroupIDs, "activity-end dashboard refresh is deliberately unscoped")
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

func TestBroadcast_DailyCheckoutSendsDashboardCounts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Broadcast", "DailyCheckout", "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	svc.BroadcastDailyCheckout(testpkg.TenantContext(1), student.ID)

	// #2057: tenant-scoped; the student has no educational group, so group_ids
	// must be absent (broad-refresh fallback on the client).
	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1, "expected dashboard_counts_changed after BroadcastDailyCheckout")
	assert.Empty(t, broadcaster.CallsByMethod("all"), "dashboard refresh must not use cross-tenant BroadcastToAll")
	assert.Nil(t, counts[0].Event.Data.GroupIDs, "no educational group -> group_ids must be omitted entirely")
}

func TestBroadcast_DailyCheckoutCarriesEducationGroupID(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-DailyCheckout")
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "DailyCheckoutGrp", "3b")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, eduGroup.ID)

	svc.BroadcastDailyCheckout(testpkg.TenantContext(1), student.ID)

	eduGroupIDStr := strconv.FormatInt(eduGroup.ID, 10)
	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1)
	require.NotNil(t, counts[0].Event.Data.GroupIDs, "dashboard_counts_changed must carry the educational group id")
	assert.Equal(t, []string{eduGroupIDStr}, *counts[0].Event.Data.GroupIDs)

	// The educational-group student_checkout carries the group id too.
	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts)
	require.NotNil(t, checkouts[0].Data.GroupIDs)
	assert.Equal(t, []string{eduGroupIDStr}, *checkouts[0].Data.GroupIDs)
}
