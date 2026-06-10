package active_test

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/realtime"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBroadcaster records all broadcast calls for assertion.
type mockBroadcaster struct {
	mu       sync.Mutex
	allCalls []realtime.Event
}

func (m *mockBroadcaster) BroadcastToGroup(_ int64, _ string, event realtime.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allCalls = append(m.allCalls, event)
	return nil
}

func (m *mockBroadcaster) BroadcastToAll(event realtime.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allCalls = append(m.allCalls, event)
	return nil
}

func (m *mockBroadcaster) BroadcastToTenant(_ int64, event realtime.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allCalls = append(m.allCalls, event)
	return nil
}

func (m *mockBroadcaster) getAllCalls() []realtime.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]realtime.Event, len(m.allCalls))
	copy(result, m.allCalls)
	return result
}

func (m *mockBroadcaster) hasEventType(t realtime.EventType) bool {
	for _, e := range m.getAllCalls() {
		if e.Type == t {
			return true
		}
	}
	return false
}

func (m *mockBroadcaster) eventsOfType(t realtime.EventType) []realtime.Event {
	events := make([]realtime.Event, 0)
	for _, e := range m.getAllCalls() {
		if e.Type == t {
			events = append(events, e)
		}
	}
	return events
}

func setupServiceWithBroadcaster(t *testing.T) (active.Service, *mockBroadcaster) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)
	broadcaster := &mockBroadcaster{}

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
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Student", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Broadcast", "Staff")
	iotDevice := testpkg.CreateTestDevice(t, db, "broadcast-device")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, iotDevice.ID)

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

	assert.True(t, broadcaster.hasEventType(realtime.EventDashboardCountsChanged),
		"expected dashboard_counts_changed after CreateVisit")
	assert.True(t, broadcaster.hasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after CreateVisit")
}

func TestBroadcast_EndVisitSendsDashboardCounts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, db, "broadcast-checkout")
	room := testpkg.CreateTestRoom(t, db, "Broadcast Checkout Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Broadcast", "Checkout", "1b")
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

	// Clear calls from visit creation
	broadcaster.mu.Lock()
	broadcaster.allCalls = nil
	broadcaster.mu.Unlock()

	err := svc.EndVisit(testpkg.TenantContext(1), visit.ID)
	require.NoError(t, err)

	assert.True(t, broadcaster.hasEventType(realtime.EventDashboardCountsChanged),
		"expected dashboard_counts_changed after EndVisit")
	assert.True(t, broadcaster.hasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after EndVisit")
}

func TestBroadcast_UpdateVisitMoveSendsMovementEvents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	broadcaster := &mockBroadcaster{}
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

	broadcaster.mu.Lock()
	broadcaster.allCalls = nil
	broadcaster.mu.Unlock()

	visit.ActiveGroupID = targetGroup.ID
	err := svc.UpdateVisit(testpkg.TenantContext(1), visit)
	require.NoError(t, err)

	checkouts := broadcaster.eventsOfType(realtime.EventStudentCheckOut)
	require.NotEmpty(t, checkouts, "expected student_checkout for source group")
	assert.Equal(t, strconv.FormatInt(sourceGroup.ID, 10), checkouts[0].ActiveGroupID)

	checkins := broadcaster.eventsOfType(realtime.EventStudentCheckIn)
	require.NotEmpty(t, checkins, "expected student_checkin for target group")
	assert.Equal(t, strconv.FormatInt(targetGroup.ID, 10), checkins[0].ActiveGroupID)

	assert.True(t, broadcaster.hasEventType(realtime.EventDashboardCountsChanged),
		"expected dashboard_counts_changed after visit move")
	assert.True(t, broadcaster.hasEventType(realtime.EventActiveSupervisionChanged),
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

	broadcaster.mu.Lock()
	broadcaster.allCalls = nil
	broadcaster.mu.Unlock()

	err := svc.EndActivitySession(testpkg.TenantContext(1), session.ID)
	require.NoError(t, err)

	// Should have dashboard_counts_changed from the batch student checkout
	calls := broadcaster.getAllCalls()
	found := false
	for _, call := range calls {
		if call.Type == realtime.EventDashboardCountsChanged {
			assert.Empty(t, call.ActiveGroupID, "global events should not leak group IDs")
			found = true
		}
	}
	assert.True(t, found, "expected dashboard_counts_changed after EndActivitySession with active visits")
	assert.True(t, broadcaster.hasEventType(realtime.EventActiveSupervisionChanged),
		"expected active_supervision_changed after EndActivitySession")
}

func TestBroadcast_DailyCheckoutSendsDashboardCounts(t *testing.T) {
	svc, broadcaster := setupServiceWithBroadcaster(t)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Broadcast", "DailyCheckout", "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	svc.BroadcastDailyCheckout(testpkg.TenantContext(1), student.ID)

	assert.True(t, broadcaster.hasEventType(realtime.EventDashboardCountsChanged),
		"expected dashboard_counts_changed after BroadcastDailyCheckout")
}
