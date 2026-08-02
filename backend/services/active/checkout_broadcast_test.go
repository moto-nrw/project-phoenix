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

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Regression suite for #2113: the staff-side checkout stopped emitting any SSE
// event when 976b32f moved the visit-ending from ActiveService.EndVisit (which
// broadcasts) to the repository call inside performCheckOut (which does not).
// Every colleague's open tab kept showing the child as present, and none of the
// affected views revalidate on focus or on an interval, so nothing corrected it.

// checkoutFixtures is one child checked into one room, plus everything the
// checkout path reads: an OGS group for the edu:{id} topic, an open attendance
// row for CloseOpenForToday, and an open visit for endOpenVisitForStudent.
type checkoutFixtures struct {
	studentID     int64
	staffID       int64
	activeGroupID int64
	eduGroupID    int64
	deviceID      int64
}

func (f checkoutFixtures) eduGroupIDStr() string {
	return strconv.FormatInt(f.eduGroupID, 10)
}

func (f checkoutFixtures) activeGroupTopic() string {
	return strconv.FormatInt(f.activeGroupID, 10)
}

func (f checkoutFixtures) eduTopic() string {
	return "edu:" + f.eduGroupIDStr()
}

// setupCheckedInStudent builds the fixtures above and returns the cleanup to
// defer (the caller's own `defer db.Close()` must outlive it, so cleanup cannot
// hide in t.Cleanup). withVisit=false leaves the child checked in without a
// room visit — the binary-mode shape, where the tenant keeps no visit rows.
func setupCheckedInStudent(t *testing.T, db *bun.DB, label string, withVisit bool) (checkoutFixtures, func()) {
	t.Helper()

	activity := testpkg.CreateTestActivityGroup(t, db, "checkout-"+label)
	room := testpkg.CreateTestRoom(t, db, "Checkout Room "+label)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "OGS-Checkout-"+label)
	student := testpkg.CreateTestStudent(t, db, "Checkout", label, "2b")
	assignStudentToEducationGroup(t, db, context.Background(), student.ID, eduGroup.ID)
	staff := testpkg.CreateTestStaff(t, db, "Checkout", "Staff"+label)
	iotDevice := testpkg.CreateTestDevice(t, db, "checkout-device-"+label)

	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, iotDevice.ID, time.Now(), nil)
	if withVisit {
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	}

	f := checkoutFixtures{
		studentID:     student.ID,
		staffID:       staff.ID,
		activeGroupID: activeGroup.ID,
		eduGroupID:    eduGroup.ID,
		deviceID:      iotDevice.ID,
	}
	// The attendance and visit rows are removed via their student_id, so they
	// need no ids of their own here. Students before their education group: the
	// FK is ON DELETE SET NULL and would null students.tenant_id otherwise.
	cleanup := func() {
		testpkg.CleanupActivityFixtures(t, db,
			activity.ID, room.ID, activeGroup.ID, student.ID, eduGroup.ID, staff.ID, iotDevice.ID)
	}
	return f, cleanup
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
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	f, cleanup := setupCheckedInStudent(t, db, "WebVisit", true)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
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
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	f, cleanup := setupCheckedInStudent(t, db, "Idempotent", true)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)

	broadcaster.Reset()
	result, err := svc.CheckOutStudent(ctx, f.studentID, f.staffID, true)
	require.NoError(t, err)
	assert.Zero(t, result.AttendanceID, "second checkout must be the idempotent no-op path")

	assert.Empty(t, broadcaster.Calls(), "an idempotent checkout must broadcast nothing")
}

// TestCheckout_RoomlessCheckoutBroadcastsEduTopic covers the shape used by
// POST /api/students/{id}/school-checkin with action "out" (the binary-mode
// "Kinder an- und abmelden" flow) and by the kiosk daily checkout: attendance
// closes, but no visit ends, so there is no active group to scope to.
func TestCheckout_RoomlessCheckoutBroadcastsEduTopic(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	f, cleanup := setupCheckedInStudent(t, db, "Roomless", false)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
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
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	f, cleanup := setupCheckedInStudent(t, db, "AfterCommit", true)
	defer cleanup()
	broadcaster.Reset()

	err := tenant.WithTenantTx(context.Background(), db, 1, func(txCtx context.Context, _ bun.Tx) error {
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	// Own service instance rather than setupServiceWithBroadcaster: the daily
	// checkout reads the attendance status first, which resolves staff names
	// through UsersService — the shared helper leaves that dependency nil.
	svc, broadcaster := newDailyCheckoutService(t, db)

	// No visit: by the time the kiosk asks "nach Hause oder unterwegs?", the
	// child already left the room and the check-in handler closed the visit.
	f, cleanup := setupCheckedInStudent(t, db, "DailyOnce", false)
	defer cleanup()

	ctx := testpkg.TenantContext(1)
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
