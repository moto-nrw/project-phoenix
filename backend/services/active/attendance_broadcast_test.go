package active_test

import (
	"context"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Regression suite for #2113: attendance changes stopped reaching the SSE bus.
// On the checkout side 976b32f moved the visit-ending from
// ActiveService.EndVisit (which broadcasts) to the repository call inside
// performCheckOut (which does not); on the check-in side performCheckIn never
// broadcast at all, so every attendance write without a room visit was silent.
// Colleagues' open tabs kept showing the stale presence, and none of the
// affected views revalidate on focus or on an interval, so nothing corrected it.

// attendanceFixtures is one child in one OGS group plus the room session and
// device the attendance paths read. Depending on the builder used, the child is
// already checked in (open attendance row, optionally an open visit) or still
// absent.
type attendanceFixtures struct {
	studentID     int64
	staffID       int64
	activeGroupID int64
	eduGroupID    int64
	deviceID      int64
}

func (f attendanceFixtures) eduGroupIDStr() string {
	return strconv.FormatInt(f.eduGroupID, 10)
}

func (f attendanceFixtures) activeGroupTopic() string {
	return strconv.FormatInt(f.activeGroupID, 10)
}

func (f attendanceFixtures) eduTopic() string {
	return "edu:" + f.eduGroupIDStr()
}

// setupAbsentStudent builds the base fixtures without any attendance row and
// returns the cleanup to defer.
func setupAbsentStudent(t *testing.T, db *bun.DB, label string) (attendanceFixtures, func()) {
	t.Helper()

	activity := testpkg.CreateTestActivityGroup(t, db, "attendance-"+label)
	room := testpkg.CreateTestRoom(t, db, "Attendance Room "+label)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-Attendance-"+label)
	student := testpkg.CreateTestStudent(t, db, "Attendance", label, "2b")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)
	staff := testpkg.CreateTestStaff(t, db, "Attendance", "Staff"+label)
	iotDevice := testpkg.CreateTestDevice(t, db, "attendance-device-"+label)

	f := attendanceFixtures{
		studentID:     student.ID,
		staffID:       staff.ID,
		activeGroupID: activeGroup.ID,
		eduGroupID:    eduGroup.ID,
		deviceID:      iotDevice.ID,
	}
	// Attendance and visit rows are removed via their student_id, so they need
	// no ids of their own here. Students before their education group: the FK is
	// ON DELETE SET NULL and would null students.tenant_id otherwise.
	cleanup := func() {
	}
	return f, cleanup
}

// setupCheckedInStudent is setupAbsentStudent plus an open attendance row.
// withVisit=false leaves the child checked in without a room visit — the
// binary-mode shape, where the tenant keeps no visit rows.
func setupCheckedInStudent(t *testing.T, db *bun.DB, label string, withVisit bool) (attendanceFixtures, func()) {
	t.Helper()

	f, cleanup := setupAbsentStudent(t, db, label)
	testpkg.CreateTestAttendance(t, db, f.studentID, f.staffID, f.deviceID, time.Now(), nil)
	if withVisit {
		testpkg.CreateTestVisit(t, db, f.studentID, f.activeGroupID, time.Now(), nil)
	}
	return f, cleanup
}

// checkinEventsOnTopic returns the student_checkin events routed to one topic.
func checkinEventsOnTopic(b *testpkg.RecordingBroadcaster, topic string) []realtime.Event {
	out := make([]realtime.Event, 0)
	for _, c := range b.GroupCallsForTopic(topic) {
		if c.Event.Type == realtime.EventStudentCheckIn {
			out = append(out, c.Event)
		}
	}
	return out
}

// newDailyCheckoutService is setupServiceWithBroadcaster plus the PersonService
// the daily-checkout entry point needs: ConfirmDailyCheckout reads the
// attendance status first, and resolving its staff names goes through
// UsersService.
func newDailyCheckoutService(t *testing.T, db *bun.DB) (active.Service, *testpkg.RecordingBroadcaster) {
	t.Helper()

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
		UsersService: usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
			PersonRepo:  repos.Person,
			RFIDRepo:    repos.RFIDCard,
			AccountRepo: repos.Account,
			StudentRepo: repos.Student,
			StaffRepo:   repos.Staff,
			TeacherRepo: repos.Teacher,
			DB:          db,
		}),
		DB:          db,
		Broadcaster: broadcaster,
		Logger:      slog.Default(),
	})

	return svc, broadcaster
}

// checkoutEventsOnTopic returns the student_checkout events routed to one topic.
func checkoutEventsOnTopic(b *testpkg.RecordingBroadcaster, topic string) []realtime.Event {
	out := make([]realtime.Event, 0)
	for _, c := range b.GroupCallsForTopic(topic) {
		if c.Event.Type == realtime.EventStudentCheckOut {
			out = append(out, c.Event)
		}
	}
	return out
}

// TestCheckout_WebCheckoutWithOpenVisitBroadcasts is the direct regression test
// for #2113: POST /api/active/visits/student/{id}/checkout lands on
// CheckOutStudent, which must produce the same event set the removed
// EndVisit call used to produce.
func TestCheckout_WebCheckoutWithOpenVisitBroadcasts(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	f, cleanup := setupCheckedInStudent(t, db, "WebVisit", true)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	broadcaster.Reset()

	result, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)
	require.NotZero(t, result.AttendanceID, "the open attendance row must have been closed")

	studentIDStr := strconv.FormatInt(f.studentID, 10)

	// The room roster topic: this is what the active-supervisions and room
	// views subscribe to.
	roomEvents := checkoutEventsOnTopic(broadcaster, f.activeGroupTopic())
	require.Len(t, roomEvents, 1, "expected one student_checkout on the active-group topic")
	require.NotNil(t, roomEvents[0].Data.StudentID)
	assert.Equal(t, studentIDStr, *roomEvents[0].Data.StudentID)
	assert.Equal(t, f.activeGroupTopic(), roomEvents[0].ActiveGroupID)

	// The OGS group topic: "Meine Gruppe" and the group-scoped student lists.
	eduEvents := checkoutEventsOnTopic(broadcaster, f.eduTopic())
	require.Len(t, eduEvents, 1, "expected one student_checkout on the edu:{id} topic")
	require.NotNil(t, eduEvents[0].Data.GroupIDs, "the checkout must carry its edu group scope (#2057)")
	assert.Equal(t, []string{f.eduGroupIDStr()}, *eduEvents[0].Data.GroupIDs)

	// Tenant-wide invalidation, group-scoped so clients skip every other group.
	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1, "expected exactly one tenant-scoped dashboard_counts_changed")
	require.NotNil(t, counts[0].Event.Data.GroupIDs,
		"group_ids must be present and non-empty — an empty array reads as 'scope to nothing' (#2057)")
	assert.Equal(t, []string{f.eduGroupIDStr()}, *counts[0].Event.Data.GroupIDs)
	assert.Empty(t, broadcaster.CallsByMethod("all"), "must not fan out across tenants")

	assert.True(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"a room roster changed, so active_supervision_changed must fire")

	// #2085: the child id rides the group-scoped topics only.
	testpkg.AssertNoTenantWideStudentIdentity(t, broadcaster)
}

// TestCheckout_IdempotentCheckoutIsSilent pins the other half of the contract:
// a repeated checkout closes no row, so no client has anything to refetch.
func TestCheckout_IdempotentCheckoutIsSilent(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	f, cleanup := setupCheckedInStudent(t, db, "Idempotent", true)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)

	broadcaster.Reset()
	result, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)
	assert.Zero(t, result.AttendanceID, "second checkout must be the idempotent no-op path")

	assert.Empty(t, broadcaster.Calls(), "an idempotent checkout must broadcast nothing")
}

// TestCheckout_OrphanedVisitWithoutAttendanceBroadcasts covers the #895 heal
// path: even when attendance is already closed or missing, ending the stale
// room visit changes the room roster and must wake subscribed clients.
func TestCheckout_OrphanedVisitWithoutAttendanceBroadcasts(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	f, cleanup := setupAbsentStudent(t, db, "OrphanedVisit")
	defer cleanup()
	testpkg.CreateTestVisit(t, db, f.studentID, f.activeGroupID, time.Now(), nil)
	ctx := testpkg.Ctx(t)
	broadcaster.Reset()

	result, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)
	assert.Zero(t, result.AttendanceID, "there was no open attendance row to close")

	roomEvents := checkoutEventsOnTopic(broadcaster, f.activeGroupTopic())
	require.Len(t, roomEvents, 1, "the healed visit must emit a room-scoped checkout")
	assert.Equal(t, f.activeGroupTopic(), roomEvents[0].ActiveGroupID)
	assert.Len(t, checkoutEventsOnTopic(broadcaster, f.eduTopic()), 1,
		"the healed visit must emit an educational-group checkout")
	assert.Len(t, dashboardCountsTenantCalls(broadcaster), 1,
		"the healed visit must refresh dashboard counts")
	assert.True(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"ending the orphaned visit changed the room roster")

	testpkg.AssertNoTenantWideStudentIdentity(t, broadcaster)
}

// TestCheckout_RoomlessCheckoutBroadcastsEduTopic covers the shape used by
// POST /api/students/{id}/school-checkin with action "out" (the binary-mode
// "Kinder an- und abmelden" flow) and by the kiosk daily checkout: attendance
// closes, but no visit ends, so there is no active group to scope to.
func TestCheckout_RoomlessCheckoutBroadcastsEduTopic(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	f, cleanup := setupCheckedInStudent(t, db, "Roomless", false)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	broadcaster.Reset()

	_, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)

	eduEvents := checkoutEventsOnTopic(broadcaster, f.eduTopic())
	require.Len(t, eduEvents, 1, "expected one student_checkout on the edu:{id} topic")
	assert.Empty(t, eduEvents[0].ActiveGroupID, "no visit ended, so there is no active group")
	require.NotNil(t, eduEvents[0].Data.GroupIDs)
	assert.Equal(t, []string{f.eduGroupIDStr()}, *eduEvents[0].Data.GroupIDs)

	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1, "expected exactly one tenant-scoped dashboard_counts_changed")
	require.NotNil(t, counts[0].Event.Data.GroupIDs)
	assert.Equal(t, []string{f.eduGroupIDStr()}, *counts[0].Event.Data.GroupIDs)

	assert.False(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"no room roster changed, so active_supervision_changed must stay out of this shape")

	testpkg.AssertNoTenantWideStudentIdentity(t, broadcaster)
}

// TestCheckout_BroadcastRunsAfterCommit pins the ordering half of #2113 and,
// with it, the trap the implementation has to avoid: the student display data
// must be read INSIDE the transaction, because the tx carried in ctx is closed
// by the time the after-commit hook runs. A hook that did its own repository
// read would emit an event without a name or a group scope — or fail outright.
func TestCheckout_BroadcastRunsAfterCommit(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)

	f, cleanup := setupCheckedInStudent(t, db, "AfterCommit", true)
	defer cleanup()
	broadcaster.Reset()

	err := tenant.WithTenantTx(context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if _, err := svc.CheckOutStudent(txCtx, f.studentID, f.staffID, true); err != nil {
			return err
		}
		assert.Empty(t, broadcaster.Calls(),
			"nothing may reach the SSE bus before the transaction commits — a client woken "+
				"early refetches the pre-checkout state and nothing corrects it afterwards")
		return nil
	})
	require.NoError(t, err)

	eduEvents := checkoutEventsOnTopic(broadcaster, f.eduTopic())
	require.Len(t, eduEvents, 1, "the checkout must reach the SSE bus once the tx committed")
	require.NotNil(t, eduEvents[0].Data.StudentName)
	assert.NotEmpty(t, *eduEvents[0].Data.StudentName,
		"the student name proves the repository read happened inside the transaction")
	require.NotNil(t, eduEvents[0].Data.GroupIDs)
	assert.Equal(t, []string{f.eduGroupIDStr()}, *eduEvents[0].Data.GroupIDs)
}

// TestCheckout_DailyCheckoutBroadcastsExactlyOnce guards the seam the fix
// touched on the kiosk side. ConfirmDailyCheckout used to call
// BroadcastDailyCheckout explicitly, before the commit; performCheckOut now
// owns that emission, so the explicit call was removed. If it ever comes back,
// every "nach Hause" scan wakes every client of the school twice.
func TestCheckout_DailyCheckoutBroadcastsExactlyOnce(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// Own service instance rather than setupServiceWithBroadcaster: the daily
	// checkout reads the attendance status first, which resolves staff names
	// through UsersService — the shared helper leaves that dependency nil.
	svc, broadcaster := newDailyCheckoutService(t, db)

	// No visit: by the time the kiosk asks "nach Hause oder unterwegs?", the
	// child already left the room and the check-in handler closed the visit.
	f, cleanup := setupCheckedInStudent(t, db, "DailyOnce", false)
	defer cleanup()

	ctx := testpkg.Ctx(t)
	// The device resolves its auditable principal through the active session's
	// supervisor, so the session has to point at the device and have one.
	_, err := db.NewUpdate().
		Table("active.groups").
		Set("device_id = ?", f.deviceID).
		Where("id = ?", f.activeGroupID).
		Exec(ctx)
	require.NoError(t, err)
	testpkg.CreateTestGroupSupervisor(t, db, f.staffID, f.activeGroupID, "supervisor")

	broadcaster.Reset()
	result, err := svc.ConfirmDailyCheckout(ctx, f.studentID, f.deviceID, "zuhause")
	require.NoError(t, err)
	assert.Equal(t, "checked_out_daily", result.Action)

	checkouts := broadcaster.EventsOfType(realtime.EventStudentCheckOut)
	require.Len(t, checkouts, 1, "a daily checkout must emit exactly one student_checkout")
	require.NotNil(t, checkouts[0].Data.Source)
	assert.Equal(t, "daily_checkout", *checkouts[0].Data.Source,
		"the historical source label of the kiosk flow must survive the move")

	assert.Len(t, dashboardCountsTenantCalls(broadcaster), 1,
		"a daily checkout must emit exactly one dashboard_counts_changed")
}

// TestCheckout_DailyCheckoutWithOpenVisitPreservesSource covers the #895 heal
// path during kiosk day-end confirmation. The room-scoped event replaces the
// historical roomless event but must retain its stable source wire value.
func TestCheckout_DailyCheckoutWithOpenVisitPreservesSource(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc, broadcaster := newDailyCheckoutService(t, db)

	f, cleanup := setupCheckedInStudent(t, db, "DailyOpenVisit", true)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	_, err := db.NewUpdate().
		Table("active.groups").
		Set("device_id = ?", f.deviceID).
		Where("id = ?", f.activeGroupID).
		Exec(ctx)
	require.NoError(t, err)
	testpkg.CreateTestGroupSupervisor(t, db, f.staffID, f.activeGroupID, "supervisor")

	broadcaster.Reset()
	result, err := svc.ConfirmDailyCheckout(ctx, f.studentID, f.deviceID, "zuhause")
	require.NoError(t, err)
	assert.Equal(t, "checked_out_daily", result.Action)

	roomEvents := checkoutEventsOnTopic(broadcaster, f.activeGroupTopic())
	require.Len(t, roomEvents, 1, "the open visit must emit one room-scoped checkout")
	require.NotNil(t, roomEvents[0].Data.Source)
	assert.Equal(t, "daily_checkout", *roomEvents[0].Data.Source)

	eduEvents := checkoutEventsOnTopic(broadcaster, f.eduTopic())
	require.Len(t, eduEvents, 1, "the open visit must emit one educational-group checkout")
	require.NotNil(t, eduEvents[0].Data.Source)
	assert.Equal(t, "daily_checkout", *eduEvents[0].Data.Source)

	assert.Len(t, dashboardCountsTenantCalls(broadcaster), 1,
		"the daily checkout must emit one dashboard_counts_changed")
}

// =============================================================================
// Check-in — the mirror gap (#2113 follow-up)
// =============================================================================

// TestCheckin_WebCheckinBroadcastsEduTopic is the regression test for the
// check-in half: POST /api/students/{id}/school-checkin with action "in" lands
// on CheckInStudent, which wrote attendance and told nobody.
func TestCheckin_WebCheckinBroadcastsEduTopic(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	// A web check-in books attendance against the virtual WEB-MANUAL-001
	// device every real school is provisioned with.
	testpkg.EnsureWebManualDevice(t, db)

	f, cleanup := setupAbsentStudent(t, db, "WebCheckin")
	defer cleanup()
	ctx := testpkg.Ctx(t)
	broadcaster.Reset()

	result, err := svc.CheckInStudent(ctx, f.studentID, f.staffID, 0, true)
	require.NoError(t, err)
	require.NotZero(t, result.AttendanceID, "an attendance row must have been opened")

	studentIDStr := strconv.FormatInt(f.studentID, 10)

	eduEvents := checkinEventsOnTopic(broadcaster, f.eduTopic())
	require.Len(t, eduEvents, 1, "expected one student_checkin on the edu:{id} topic")
	assert.Empty(t, eduEvents[0].ActiveGroupID, "no room was entered, so there is no active group")
	require.NotNil(t, eduEvents[0].Data.StudentID)
	assert.Equal(t, studentIDStr, *eduEvents[0].Data.StudentID)
	require.NotNil(t, eduEvents[0].Data.GroupIDs, "the check-in must carry its edu group scope (#2057)")
	assert.Equal(t, []string{f.eduGroupIDStr()}, *eduEvents[0].Data.GroupIDs)

	assert.Empty(t, checkinEventsOnTopic(broadcaster, f.activeGroupTopic()),
		"a roomless check-in must not address an active-group topic")

	counts := dashboardCountsTenantCalls(broadcaster)
	require.Len(t, counts, 1, "expected exactly one tenant-scoped dashboard_counts_changed")
	require.NotNil(t, counts[0].Event.Data.GroupIDs,
		"group_ids must be present and non-empty — an empty array reads as 'scope to nothing' (#2057)")
	assert.Equal(t, []string{f.eduGroupIDStr()}, *counts[0].Event.Data.GroupIDs)
	assert.Empty(t, broadcaster.CallsByMethod("all"), "must not fan out across tenants")

	assert.False(t, broadcaster.HasEventType(realtime.EventActiveSupervisionChanged),
		"no room roster changed, so active_supervision_changed must stay out of this shape")

	// #2085: the child id rides the group-scoped topics only.
	testpkg.AssertNoTenantWideStudentIdentity(t, broadcaster)
}

// TestCheckin_RepeatedCheckinIsSilent pins the mirror of the idempotent
// checkout: the second "in" is absorbed by the partial unique index, no row
// moves, so no client has anything to refetch.
func TestCheckin_RepeatedCheckinIsSilent(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	// A web check-in books attendance against the virtual WEB-MANUAL-001
	// device every real school is provisioned with.
	testpkg.EnsureWebManualDevice(t, db)

	f, cleanup := setupAbsentStudent(t, db, "RepeatCheckin")
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := svc.CheckInStudent(ctx, f.studentID, f.staffID, 0, true)
	require.NoError(t, err)

	broadcaster.Reset()
	_, err = svc.CheckInStudent(ctx, f.studentID, f.staffID, 0, true)
	require.NoError(t, err)

	assert.Empty(t, broadcaster.Calls(), "an absorbed check-in must broadcast nothing")
}

// TestCheckin_BroadcastRunsAfterCommit is the check-in twin of
// TestCheckout_BroadcastRunsAfterCommit — same ordering requirement, same trap:
// the student read has to happen inside the transaction, because the tx carried
// in ctx is closed by the time the after-commit hook runs.
func TestCheckin_BroadcastRunsAfterCommit(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	// A web check-in books attendance against the virtual WEB-MANUAL-001
	// device every real school is provisioned with.
	testpkg.EnsureWebManualDevice(t, db)

	f, cleanup := setupAbsentStudent(t, db, "CheckinCommit")
	defer cleanup()
	broadcaster.Reset()

	err := tenant.WithTenantTx(context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if _, err := svc.CheckInStudent(txCtx, f.studentID, f.staffID, 0, true); err != nil {
			return err
		}
		assert.Empty(t, broadcaster.Calls(),
			"nothing may reach the SSE bus before the transaction commits")
		return nil
	})
	require.NoError(t, err)

	eduEvents := checkinEventsOnTopic(broadcaster, f.eduTopic())
	require.Len(t, eduEvents, 1, "the check-in must reach the SSE bus once the tx committed")
	require.NotNil(t, eduEvents[0].Data.StudentName)
	assert.NotEmpty(t, *eduEvents[0].Data.StudentName,
		"the student name proves the repository read happened inside the transaction")
	require.NotNil(t, eduEvents[0].Data.GroupIDs)
	assert.Equal(t, []string{f.eduGroupIDStr()}, *eduEvents[0].Data.GroupIDs)
}

// TestCheckin_RoomCheckinBroadcastsOnce guards the disjointness the check-in
// fix rests on. The detailed-mode room check-in writes its attendance row
// through CreateVisit's own path (ensureOrUpdateAttendance), never through
// performCheckIn, so it must keep emitting exactly one student_checkin per
// topic. If someone later routes CreateVisit through performCheckIn, every room
// entry would wake each client twice and this test fails.
func TestCheckin_RoomCheckinBroadcastsOnce(t *testing.T) {
	t.Parallel()

	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	// A web check-in books attendance against the virtual WEB-MANUAL-001
	// device every real school is provisioned with.
	testpkg.EnsureWebManualDevice(t, db)

	f, cleanup := setupAbsentStudent(t, db, "RoomCheckin")
	defer cleanup()

	staff := &usersModels.Staff{}
	staff.ID = f.staffID
	ctx := context.WithValue(testpkg.Ctx(t), device.CtxStaff, staff)
	broadcaster.Reset()

	visit := &activeModels.Visit{
		StudentID:     f.studentID,
		ActiveGroupID: f.activeGroupID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, svc.CreateVisit(ctx, visit))

	assert.Len(t, checkinEventsOnTopic(broadcaster, f.activeGroupTopic()), 1,
		"exactly one student_checkin on the active-group topic")
	assert.Len(t, checkinEventsOnTopic(broadcaster, f.eduTopic()), 1,
		"exactly one student_checkin on the edu:{id} topic")
	assert.Len(t, dashboardCountsTenantCalls(broadcaster), 1,
		"exactly one dashboard_counts_changed")
}
