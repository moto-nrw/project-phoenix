// Hermetic integration tests for the WP-B9 instance lifecycle service.
//
// Covers:
//   - State machine: start/complete/cancel per illegal prior status → 409.
//   - Start happy-path: bridge active.group created, supervisors copied, flags set.
//   - Complete happy-path: bridge ended, CompletedAt stamped.
//   - Cancel from planned: status flip only, no active.group side effects.
//   - Cancel from active: bridge ended (visits + supervisors close).
//   - Conflict detection: room, staff, student sub-checks each produce a warning.
//   - Re-plan-week: deletes only planned non-spontaneous; all other kinds survive.
//
// All fixtures via testpkg.CreateTest* + CleanupTableRecords — no hardcoded IDs.
package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// lifecycleSetup bundles the wired service + common fixtures. Cleanup is
// registered via t.Cleanup on construction; tests should NOT call cleanup()
// explicitly. LIFO ordering of t.Cleanup ensures child rows (instances,
// active.groups, supervisors) are dropped before the parent fixtures
// (students, staff, template) so FK constraints pass.
type lifecycleSetup struct {
	svc      scheduleSvc.InstanceService
	factory  *services.Factory
	db       *bun.DB
	ctx      context.Context
	roomID   int64
	staffID  int64
	student1 int64
	student2 int64
	tmplID   int64
}

// buildLifecycle prepares the minimum scaffold for instance-lifecycle tests:
// one tenant tx, one room, one staff, two students, one template (is_template).
// Returns the setup + a runnable cleanup that removes everything we created.
func buildLifecycle(t *testing.T) *lifecycleSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	// Register DB close FIRST so it runs LAST (LIFO) — after every other
	// t.Cleanup has released its row references.
	t.Cleanup(func() { _ = db.Close() })

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("LC-Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "LC", fmt.Sprintf("Super-%d", suffix))
	student1 := testpkg.CreateTestStudent(t, db, "LC-Alice", fmt.Sprintf("One-%d", suffix), "3a")
	student2 := testpkg.CreateTestStudent(t, db, "LC-Bob", fmt.Sprintf("Two-%d", suffix), "3a")

	// A template-ish activity group. IsTemplate flag is immaterial for the
	// lifecycle path; we only need a non-nil FK target for the spontaneous=
	// false branch to fire.
	templateRow := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("LC-Tmpl-%d", suffix))

	// Parent-fixture cleanup registered BEFORE any per-test child cleanups,
	// so LIFO orders children → parents → db.Close.
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, student1.ID, staff.ID, 0, templateRow.ID, room.ID)
		testpkg.CleanupTableRecords(t, db, "users.students", student2.ID)
	})

	return &lifecycleSetup{
		svc:      serviceFactory.Instance,
		factory:  serviceFactory,
		db:       db,
		ctx:      ctx,
		roomID:   room.ID,
		staffID:  staff.ID,
		student1: student1.ID,
		student2: student2.ID,
		tmplID:   templateRow.ID,
	}
}

// seedInstance inserts one planned activity_instance plus optional staff
// and student rows. Returns the instance for the caller to act on.
func seedInstance(t *testing.T, s *lifecycleSetup, withStaff bool, withStudents bool) *scheduleModels.ActivityInstance {
	t.Helper()
	ai := &scheduleModels.ActivityInstance{
		Date:            time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		ActivityGroupID: &s.tmplID,
		Title:           "Lifecycle-Test",
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          scheduleModels.InstanceStatusPlanned,
		IsSpontaneous:   false,
	}
	ai.SetTenantID(1)
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	if withStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true}
		row.SetTenantID(1)
		_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
	if withStudents {
		for _, sid := range []int64{s.student1, s.student2} {
			row := &scheduleModels.InstanceStudent{InstanceID: ai.ID, StudentID: sid, Status: scheduleModels.AttendanceStatusExpected}
			row.SetTenantID(1)
			_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_students`).Exec(s.ctx)
			require.NoError(t, err)
		}
	}
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", ai.ID)
	})
	return ai
}

// forceSetInstanceStatus rewrites the status column directly. Used by 409-path
// tests to move an instance into a state that can't be reached via the public
// API (e.g. cancelled-from-scratch).
func forceSetInstanceStatus(t *testing.T, s *lifecycleSetup, id int64, status string) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", status).
		Where(`"activity_instance".id = ?`, id).
		Where("tenant_id = ?", 1).
		Exec(s.ctx)
	require.NoError(t, err)
}

// --- State machine: happy paths ---------------------------------------------

func TestInstance_Start_HappyPath(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true)

	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Instance)

	assert.Equal(t, scheduleModels.InstanceStatusActive, result.Instance.Status)
	require.NotNil(t, result.Instance.ActiveGroupID)
	assert.Equal(t, result.ActiveGroupID, *result.Instance.ActiveGroupID)
	assert.NotNil(t, result.Instance.StartedAt)

	// active.group row exists and is in the same room.
	group, err := s.factory.Active.GetActiveGroup(s.ctx, result.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.Equal(t, s.roomID, group.RoomID)
	assert.True(t, group.IsActive(), "bridge active.group should still be open")

	// Supervisor row copied from instance_staff.
	sups, err := s.factory.Active.FindSupervisorsByActiveGroupID(s.ctx, result.ActiveGroupID)
	require.NoError(t, err)
	assert.Len(t, sups, 1)
	assert.Equal(t, s.staffID, sups[0].StaffID)

	// Warnings should be empty in a clean-tenant scenario.
	assert.Empty(t, result.Warnings)

	// Cleanup — supervisors + active.group survive the instance cleanup.
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.group_supervisors", sups[0].ID)
		testpkg.CleanupTableRecords(t, s.db, "active.groups", result.ActiveGroupID)
	})
}

func TestInstance_Complete_HappyPath(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", started.ActiveGroupID)
	})

	completed, err := s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletedAt)

	// Bridge active.group must be closed.
	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime, "active.group should have been ended")
}

func TestInstance_Cancel_FromPlanned_LeavesNoActiveGroup(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, false)

	cancelled, err := s.svc.Cancel(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCancelled, cancelled.Status)
	require.NotNil(t, cancelled.CompletedAt)
	assert.Nil(t, cancelled.ActiveGroupID, "cancel-from-planned must not create a bridge")

	// Sanity: no active.group rows for this room.
	groups, err := s.factory.Active.FindActiveGroupsByRoomID(s.ctx, s.roomID)
	require.NoError(t, err)
	assert.Empty(t, groups, "planned→cancelled must not create active.group")
}

func TestInstance_Cancel_FromActive_EndsBridge(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", started.ActiveGroupID)
	})

	cancelled, err := s.svc.Cancel(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCancelled, cancelled.Status)

	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime, "cancel-from-active must end active.group")
}

// --- State machine: 409 branches --------------------------------------------

func TestInstance_Start_Rejects_NonPlanned(t *testing.T) {
	for _, status := range []string{
		scheduleModels.InstanceStatusActive,
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		t.Run(status, func(t *testing.T) {
			s := buildLifecycle(t)
			ai := seedInstance(t, s, false, false)
			forceSetInstanceStatus(t, s, ai.ID, status)

			_, err := s.svc.Start(s.ctx, ai.ID, 0)
			require.Error(t, err)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition,
				"start on %q must return ErrInvalidInstanceTransition", status)
		})
	}
}

func TestInstance_Complete_Rejects_NonActive(t *testing.T) {
	for _, status := range []string{
		scheduleModels.InstanceStatusPlanned,
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		t.Run(status, func(t *testing.T) {
			s := buildLifecycle(t)
			ai := seedInstance(t, s, false, false)
			forceSetInstanceStatus(t, s, ai.ID, status)

			_, err := s.svc.Complete(s.ctx, ai.ID)
			require.Error(t, err)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
		})
	}
}

func TestInstance_Cancel_Rejects_Completed(t *testing.T) {
	s := buildLifecycle(t)
	ai := seedInstance(t, s, false, false)
	forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCompleted)

	_, err := s.svc.Cancel(s.ctx, ai.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

func TestInstance_Cancel_Rejects_AlreadyCancelled(t *testing.T) {
	s := buildLifecycle(t)
	ai := seedInstance(t, s, false, false)
	forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCancelled)

	_, err := s.svc.Cancel(s.ctx, ai.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

func TestInstance_Start_NotFound(t *testing.T) {
	s := buildLifecycle(t)

	_, err := s.svc.Start(s.ctx, 99999999, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrInstanceNotFound)
}

// Absent staff rows on instance_staff must NOT be copied into
// active.group_supervisors on start. They represent staff marked out for
// this instance (sick, substitution flow); copying them would make them
// appear as actively supervising and contradict the absence flag.
func TestInstance_Start_SkipsAbsentStaff(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, false, false)

	// Primary staff (present) + a second staff flagged absent.
	primary := &scheduleModels.InstanceStaff{
		InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true, IsAbsent: false,
	}
	primary.SetTenantID(1)
	_, err := s.db.NewInsert().Model(primary).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	absent := testpkg.CreateTestStaff(t, s.db, "Absent", fmt.Sprintf("Staff-%d", time.Now().UnixNano()))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "users.staff", absent.ID) })

	absentRow := &scheduleModels.InstanceStaff{
		InstanceID: ai.ID, StaffID: absent.ID, IsPrimary: false, IsAbsent: true,
	}
	absentRow.SetTenantID(1)
	_, err = s.db.NewInsert().Model(absentRow).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", result.ActiveGroupID)
	})

	sups, err := s.factory.Active.FindSupervisorsByActiveGroupID(s.ctx, result.ActiveGroupID)
	require.NoError(t, err)
	require.Len(t, sups, 1, "only the non-absent staff should become a supervisor")
	assert.Equal(t, s.staffID, sups[0].StaffID)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.group_supervisors", sups[0].ID) })

	// And the SupervisorIDs on the SSE envelope (visible via start result)
	// must not include the absent staff — handled through activeStaffRows,
	// which the integration test can't introspect directly, but the
	// supervisor count above is the authoritative proof.
}

// --- Conflict detection -----------------------------------------------------

func TestInstance_Start_ConflictWarning_Room(t *testing.T) {
	s := buildLifecycle(t)

	// Pre-seed a live active.group in the same room. We do this through the
	// repo so it mirrors production state exactly (tenant scoped, open-ended).
	now := time.Now()
	preGroup := &activeModels.Group{
		StartTime: now, LastActivity: now, TimeoutMinutes: 30,
		GroupID: &s.tmplID, RoomID: s.roomID,
	}
	preGroup.SetTenantID(1)
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", preGroup.ID)
	})

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", result.ActiveGroupID)
	})

	hasRoomWarning := false
	for _, w := range result.Warnings {
		if w.Kind == scheduleSvc.ConflictKindRoom && w.ResourceID == s.roomID && w.CanOverride {
			hasRoomWarning = true
		}
	}
	assert.True(t, hasRoomWarning, "start in an occupied room must produce a room warning; got %+v", result.Warnings)
	assert.Equal(t, scheduleModels.InstanceStatusActive, result.Instance.Status, "warnings never block the transition")
}

func TestInstance_Start_ConflictWarning_Staff(t *testing.T) {
	s := buildLifecycle(t)

	// Pre-seed an active.group + a live supervision by our staff member on it.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-OtherRoom-%d", time.Now().UnixNano()))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", otherRoom.ID) })

	now := time.Now()
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: otherRoom.ID}
	preGroup.SetTenantID(1)
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", preGroup.ID) })

	sup := &activeModels.GroupSupervisor{StaffID: s.staffID, GroupID: preGroup.ID, Role: "supervisor", StartDate: now}
	sup.SetTenantID(1)
	require.NoError(t, s.factory.Active.CreateGroupSupervisor(s.ctx, sup))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.group_supervisors", sup.ID) })

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", result.ActiveGroupID) })

	hasStaffWarning := false
	for _, w := range result.Warnings {
		if w.Kind == scheduleSvc.ConflictKindStaff && w.ResourceID == s.staffID {
			hasStaffWarning = true
		}
	}
	assert.True(t, hasStaffWarning, "staff supervising elsewhere must produce a staff warning; got %+v", result.Warnings)
}

func TestInstance_Start_ConflictWarning_Student(t *testing.T) {
	s := buildLifecycle(t)

	// Pre-seed an active.group in a different room + an open visit by student1.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-OtherRoom2-%d", time.Now().UnixNano()))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", otherRoom.ID) })

	now := time.Now()
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: otherRoom.ID}
	preGroup.SetTenantID(1)
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", preGroup.ID) })

	// Raw INSERT — factory.Active.CreateVisit triggers attendance-side-effect
	// writes (checked_in_by on active.attendance) that need a full staff/
	// account chain. We only need the visit row for the conflict check.
	visit := &activeModels.Visit{StudentID: s.student1, ActiveGroupID: preGroup.ID, EntryTime: now}
	visit.SetTenantID(1)
	_, err := s.db.NewInsert().Model(visit).ModelTableExpr(`active.visits`).Exec(s.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.visits", visit.ID) })

	ai := seedInstance(t, s, true, true)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", result.ActiveGroupID) })

	hasStudentWarning := false
	for _, w := range result.Warnings {
		if w.Kind == scheduleSvc.ConflictKindStudent && w.ResourceID == s.student1 {
			hasStudentWarning = true
		}
	}
	assert.True(t, hasStudentWarning, "student with open visit elsewhere must produce a student warning; got %+v", result.Warnings)
}

// --- Re-plan-week protection ------------------------------------------------

func TestInstance_ReplanWeek_OnlyDeletesPlannedNonSpontaneous(t *testing.T) {
	s := buildLifecycle(t)

	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 6)

	// Five seeded rows inside the window — one of each protected kind.
	plannedNormal := insertInstance(t, s, from, scheduleModels.InstanceStatusPlanned, false)
	plannedSpont := insertInstance(t, s, from.AddDate(0, 0, 1), scheduleModels.InstanceStatusPlanned, true)
	active := insertInstance(t, s, from.AddDate(0, 0, 2), scheduleModels.InstanceStatusActive, false)
	completed := insertInstance(t, s, from.AddDate(0, 0, 3), scheduleModels.InstanceStatusCompleted, false)
	cancelled := insertInstance(t, s, from.AddDate(0, 0, 4), scheduleModels.InstanceStatusCancelled, false)

	// Manual cleanup for the survivors (the planned-normal row may be deleted
	// by ReplanWeek; if so, the cleanup becomes a no-op).
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances",
			plannedNormal, plannedSpont, active, completed, cancelled)
	})

	_, err := s.svc.ReplanWeek(s.ctx, from, to)
	require.NoError(t, err)

	assert.False(t, instanceExists(t, s, plannedNormal), "planned non-spontaneous must be deleted")
	assert.True(t, instanceExists(t, s, plannedSpont), "spontaneous planned must survive")
	assert.True(t, instanceExists(t, s, active), "active must survive")
	assert.True(t, instanceExists(t, s, completed), "completed must survive")
	assert.True(t, instanceExists(t, s, cancelled), "cancelled must survive")
}

func TestInstance_ReplanWeek_RejectsInvalidWindowAndMissingTenant(t *testing.T) {
	s := buildLifecycle(t)
	from := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)

	_, err := s.svc.ReplanWeek(s.ctx, from, to)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "to_date")

	_, err = s.svc.ReplanWeek(context.Background(), to, from)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant")
}

func TestInstance_Create_AssignsUniqueStaffAndStudents(t *testing.T) {
	s := buildLifecycle(t)
	createdBy := s.staffID

	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:             time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		StartTime:        time.Date(1, 1, 1, 9, 30, 0, 0, time.UTC),
		EndTime:          time.Date(1, 1, 1, 10, 15, 0, 0, time.UTC),
		Title:            "Create service test",
		RoomID:           s.roomID,
		ActivityGroupID:  &s.tmplID,
		StaffIDs:         []int64{s.staffID, s.staffID, 0, -40},
		StudentIDs:       []int64{s.student1, s.student1, s.student2, -20},
		CreatedByStaffID: &createdBy,
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	assert.Equal(t, scheduleModels.InstanceStatusPlanned, inst.Status)
	assert.False(t, inst.IsSpontaneous)
	require.NotNil(t, inst.CreatedBy)
	assert.Equal(t, createdBy, *inst.CreatedBy)
	assert.Equal(t, "09:30", inst.StartTime.Format("15:04"))

	staffRows := countRowsWhere(t, s, "schedule.instance_staff", "instance_id", inst.ID)
	studentRows := countRowsWhere(t, s, "schedule.instance_students", "instance_id", inst.ID)
	assert.Equal(t, 1, staffRows, "duplicate and non-positive staff ids must be ignored")
	assert.Equal(t, 2, studentRows, "duplicate and non-positive student ids must be ignored")
}

func TestInstance_Create_SpontaneousAndMissingTenant(t *testing.T) {
	s := buildLifecycle(t)

	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:      time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC),
		StartTime: time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		Title:     "Spontaneous service test",
		RoomID:    s.roomID,
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })
	assert.True(t, inst.IsSpontaneous)
	assert.Nil(t, inst.ActivityGroupID)

	_, err = s.svc.Create(context.Background(), scheduleSvc.CreateInstanceInput{
		Date:      time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC),
		StartTime: time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		Title:     "No tenant",
		RoomID:    s.roomID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant")
}

func TestInstance_UpdatePlanned_ReplacesAssignmentsAndFields(t *testing.T) {
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)

	updated, err := s.svc.UpdatePlanned(s.ctx, ai.ID, scheduleSvc.UpdateInstanceInput{
		Date:       time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		StartTime:  time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:    time.Date(1, 1, 1, 17, 30, 0, 0, time.UTC),
		Title:      "Updated planned instance",
		RoomID:     s.roomID,
		StaffIDs:   []int64{s.staffID, s.staffID},
		StudentIDs: []int64{s.student2},
	})
	require.NoError(t, err)

	assert.Equal(t, "Updated planned instance", updated.Title)
	assert.Equal(t, "16:00", updated.StartTime.Format("15:04"))
	assert.True(t, updated.IsSpontaneous, "nil ActivityGroupID should toggle to spontaneous")
	assert.Nil(t, updated.ActivityGroupID)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", ai.ID))
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_students", "instance_id", ai.ID))

	row := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, row.Status)
}

func TestInstance_UpdatePlanned_RejectsNonPlanned(t *testing.T) {
	for _, status := range []string{
		scheduleModels.InstanceStatusActive,
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		t.Run(status, func(t *testing.T) {
			s := buildLifecycle(t)
			ai := seedInstance(t, s, false, false)
			forceSetInstanceStatus(t, s, ai.ID, status)

			_, err := s.svc.UpdatePlanned(s.ctx, ai.ID, scheduleSvc.UpdateInstanceInput{
				Date:      time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
				StartTime: time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
				Title:     "Should fail",
				RoomID:    s.roomID,
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
		})
	}
}

func TestInstance_DeleteCancelled_OnlyCancelled(t *testing.T) {
	for _, status := range []string{
		scheduleModels.InstanceStatusPlanned,
		scheduleModels.InstanceStatusActive,
		scheduleModels.InstanceStatusCompleted,
	} {
		t.Run("rejects "+status, func(t *testing.T) {
			s := buildLifecycle(t)
			ai := seedInstance(t, s, false, false)
			forceSetInstanceStatus(t, s, ai.ID, status)

			err := s.svc.DeleteCancelled(s.ctx, ai.ID)
			require.Error(t, err)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
			assert.True(t, instanceExists(t, s, ai.ID))
		})
	}

	t.Run("deletes cancelled", func(t *testing.T) {
		s := buildLifecycle(t)
		ai := seedInstance(t, s, false, false)
		forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCancelled)

		require.NoError(t, s.svc.DeleteCancelled(s.ctx, ai.ID))
		assert.False(t, instanceExists(t, s, ai.ID))
	})
}

func insertInstance(t *testing.T, s *lifecycleSetup, date time.Time, status string, spontaneous bool) int64 {
	t.Helper()
	row := &scheduleModels.ActivityInstance{
		Date:          date,
		Title:         fmt.Sprintf("Row-%s-%d", status, time.Now().UnixNano()),
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		Status:        status,
		IsSpontaneous: spontaneous,
	}
	if !spontaneous {
		row.ActivityGroupID = &s.tmplID
	}
	row.SetTenantID(1)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	return row.ID
}

func countRowsWhere(t *testing.T, s *lifecycleSetup, table, column string, value int64) int {
	t.Helper()
	var count int
	err := s.db.NewSelect().
		TableExpr(table).
		ColumnExpr("COUNT(*)").
		Where(column+" = ?", value).
		Where("tenant_id = ?", 1).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	return count
}

func instanceExists(t *testing.T, s *lifecycleSetup, id int64) bool {
	t.Helper()
	var count int
	err := s.db.NewSelect().
		TableExpr(`schedule.activity_instances AS "ai"`).
		ColumnExpr("COUNT(*)").
		Where(`"ai".id = ?`, id).
		Where(`"ai".tenant_id = ?`, 1).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	return count > 0
}

// --- B10 follow-up: Complete marks remaining expected as absent -------------

// fetchAttendance loads an instance_student row by (instance_id, student_id).
// The tests below use it to assert status after lifecycle transitions.
func fetchAttendance(t *testing.T, s *lifecycleSetup, instanceID, studentID int64) *scheduleModels.InstanceStudent {
	t.Helper()
	var row scheduleModels.InstanceStudent
	err := s.db.NewSelect().
		Model(&row).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"instance_student".tenant_id = ?`, 1).
		Scan(s.ctx)
	require.NoError(t, err)
	return &row
}

// Complete must flip every remaining 'expected' row to 'absent' inside the
// same tenant tx. Present rows stay untouched.
func TestInstance_Complete_MarksRemainingExpectedAsAbsent(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true) // both students expected
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", started.ActiveGroupID)
	})

	// Simulate a live check-in for student1 via the monotonic mirror path.
	repoFactory := repositories.NewFactory(s.db)
	updated, err := repoFactory.InstanceStudent.UpdateAttendanceFromCheckin(
		s.ctx, ai.ID, s.student1, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, updated)

	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	got1 := fetchAttendance(t, s, ai.ID, s.student1)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, got1.Status,
		"checked-in student must stay present after Complete")

	got2 := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got2.Status,
		"expected student must be flipped to absent at Complete")
}

// Cancel must NOT flip expected → absent. A cancelled instance never ran,
// so "absent" would falsely imply the student failed to show up to an event
// that happened. The cancelled status on the instance itself is the signal.
func TestInstance_Cancel_FromPlanned_DoesNotTouchAttendance(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, false, true)

	cancelled, err := s.svc.Cancel(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCancelled, cancelled.Status)

	got1 := fetchAttendance(t, s, ai.ID, s.student1)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got1.Status,
		"Cancel(planned) must not flip attendance to absent")
	got2 := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got2.Status)
}

// Same rule for Cancel from active. Even the fire-alarm scenario: students
// who were present when the instance was aborted keep their present status,
// and students who hadn't arrived stay 'expected' (no event → no absence).
func TestInstance_Cancel_FromActive_DoesNotTouchAttendance(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "active.groups", started.ActiveGroupID)
	})

	repoFactory := repositories.NewFactory(s.db)
	_, err = repoFactory.InstanceStudent.UpdateAttendanceFromCheckin(
		s.ctx, ai.ID, s.student1, time.Now(),
	)
	require.NoError(t, err)

	_, err = s.svc.Cancel(s.ctx, ai.ID)
	require.NoError(t, err)

	got1 := fetchAttendance(t, s, ai.ID, s.student1)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, got1.Status,
		"Cancel(active) must preserve present rows — they were actually there")
	got2 := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got2.Status,
		"Cancel(active) must not flip expected → absent")
}

// --- Conflict-detection pure unit (nil-safe + empty happy path) ------------

func TestDetectStartConflicts_EmptyInstance_NoWarnings(t *testing.T) {
	s := buildLifecycle(t)

	ai := seedInstance(t, s, false, false)
	repoFactory := repositories.NewFactory(s.db)
	warnings := scheduleSvc.DetectStartConflicts(s.ctx, scheduleSvc.ConflictDependencies{
		GroupRepo:         repoFactory.ActiveGroup,
		SupervisorRepo:    repoFactory.GroupSupervisor,
		VisitRepo:         repoFactory.ActiveVisit,
		InstanceStaffRepo: repoFactory.InstanceStaff,
		InstanceStudents:  repoFactory.InstanceStudent,
	}, ai, slog.Default())
	assert.Empty(t, warnings, "clean-room, no staff, no students → no warnings")
}
