// End-to-end SSE enrichment test for WP-B10.
//
// The repo and service layers each have their own hermetic coverage, but
// nothing asserts the wiring glue: that CreateVisit actually threads the
// AttendanceSyncer result into broadcastVisitCreated and that the
// resulting student_checkin Event carries attendance_status/substatus/note.
// Without this test, a reviewer could delete applyAttendanceSnapshot from
// visit_helpers.go and every lower-level test would still pass — the SSE
// enrichment would silently disappear in production.
//
// The test wires the real AttendanceSyncService into active.NewService
// (same way services.NewFactory wires it) against a seeded instance +
// instance_students row and asserts the broadcast shape.
package active_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	active "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingBroadcaster captures both BroadcastToGroup and BroadcastToAll
// so the enrichment assertion can inspect the Event payload, not just the
// event type. The existing mockBroadcaster in broadcast_test.go ignores
// BroadcastToGroup, which is exactly where student_checkin goes.
type recordingBroadcaster struct {
	mu         sync.Mutex
	groupCalls []realtime.Event
	allCalls   []realtime.Event
}

func (r *recordingBroadcaster) BroadcastToGroup(_ int64, _ string, event realtime.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groupCalls = append(r.groupCalls, event)
	return nil
}

func (r *recordingBroadcaster) BroadcastToTenant(_ int64, _ realtime.Event) error { return nil }

func (r *recordingBroadcaster) BroadcastToAll(event realtime.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allCalls = append(r.allCalls, event)
	return nil
}

// firstOfType returns the first event of the given type seen across either
// broadcast channel, or nil if none matches.
func (r *recordingBroadcaster) firstOfType(t realtime.EventType) *realtime.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.groupCalls {
		if r.groupCalls[i].Type == t {
			e := r.groupCalls[i]
			return &e
		}
	}
	for i := range r.allCalls {
		if r.allCalls[i].Type == t {
			e := r.allCalls[i]
			return &e
		}
	}
	return nil
}

func TestCreateVisit_EnrichesCheckInEventWithAttendance(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	// Real attendance syncer, wired the same way services.NewFactory does.
	syncer := scheduleSvc.NewAttendanceSyncService(
		repos.ActivityInstance,
		repos.InstanceStudent,
		slog.Default(),
	)
	broadcaster := &recordingBroadcaster{}

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
		AttendanceSyncer:   syncer,
		Logger:             slog.Default(),
	})

	// Fixtures: activity + room + active.group + student + staff + device.
	// CreateVisit requires staff + device on ctx for attendance FK.
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("E2E-Act-%d", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("E2E-Room-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "E2E-Stu", fmt.Sprintf("A-%d", suffix), "3a")
	staff := testpkg.CreateTestStaff(t, db, "E2E-Staff", fmt.Sprintf("S-%d", suffix))
	iotDevice := testpkg.CreateTestDevice(t, db, fmt.Sprintf("e2e-%d", suffix))
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db,
			activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, iotDevice.ID)
	})

	// Create an instance bridged to the active.group and an instance_students
	// row in 'expected' — exactly what the syncer is designed to flip.
	instance := &scheduleModels.ActivityInstance{
		Date:            time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("E2E-Inst-%d", suffix),
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
		ActiveGroupID:   &activeGroup.ID,
	}
	instance.SetTenantID(1)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID) })

	isRepo := scheduleRepo.NewInstanceStudentRepository(db)
	row := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, isRepo.Create(ctx, row))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID) })

	// ACT: drive a check-in through the service.
	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, iotDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, svc.CreateVisit(deviceCtx, visit))
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, visit.ID) })

	// ASSERT: student_checkin event carries attendance_status=present.
	// This is the load-bearing assertion — it's what protects against
	// someone silently removing applyAttendanceSnapshot from visit_helpers.go.
	ev := broadcaster.firstOfType(realtime.EventStudentCheckIn)
	require.NotNil(t, ev, "expected student_checkin event to be broadcast")
	require.NotNil(t, ev.Data.AttendanceStatus,
		"attendance_status must be populated when the visit bridges to an instance_students row")
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, *ev.Data.AttendanceStatus)
	// Substatus/note are null for a first-time flip — the column defaults
	// are NULL and no PATCH has happened.
	assert.Nil(t, ev.Data.AttendanceSubstatus)
	assert.Nil(t, ev.Data.AttendanceNote)

	// DB post-condition: the mirrored row flipped to 'present'.
	updated, err := isRepo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, updated.Status)
	require.NotNil(t, updated.CheckedInAt, "checked_in_at must be stamped by the mirror write")
}

// TestCreateVisit_WalkInLeavesAttendanceFieldsUnset is the negative case
// for the same wiring: when the visit does NOT bridge to an instance
// (walk-in, non-planned active group), the syncer returns nil and the SSE
// event MUST omit the three attendance fields. This rules out a regression
// where we accidentally stamp status="present" for every check-in.
func TestCreateVisit_WalkInLeavesAttendanceFieldsUnset(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)
	suffix := time.Now().UnixNano()
	syncer := scheduleSvc.NewAttendanceSyncService(
		repos.ActivityInstance,
		repos.InstanceStudent,
		slog.Default(),
	)
	broadcaster := &recordingBroadcaster{}

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
		AttendanceSyncer:   syncer,
		Logger:             slog.Default(),
	})

	// NO instance bridges to this active.group — it's a walk-in session.
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("E2E-Walk-%d", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("E2E-Walk-Room-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "E2E-Walk", fmt.Sprintf("W-%d", suffix), "3a")
	staff := testpkg.CreateTestStaff(t, db, "E2E-Walk-Staff", fmt.Sprintf("W-%d", suffix))
	iotDevice := testpkg.CreateTestDevice(t, db, fmt.Sprintf("e2e-walk-%d", suffix))
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db,
			activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, iotDevice.ID)
	})

	ctx := testpkg.TenantContext(1)
	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, iotDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, svc.CreateVisit(deviceCtx, visit))
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, visit.ID) })

	ev := broadcaster.firstOfType(realtime.EventStudentCheckIn)
	require.NotNil(t, ev, "expected student_checkin event to be broadcast")
	assert.Nil(t, ev.Data.AttendanceStatus, "walk-in must not stamp attendance_status")
	assert.Nil(t, ev.Data.AttendanceSubstatus)
	assert.Nil(t, ev.Data.AttendanceNote)
}
