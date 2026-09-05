// Hermetic integration tests for the WP-B9 instance lifecycle service.
//
// Covers:
//   - State machine: start/complete/cancel per illegal prior status → 409.
//   - Start happy-path: bridge active.group created, supervisors copied, flags set.
//   - Complete happy-path: bridge ended, CompletedAt stamped.
//   - Cancel from planned: status flip only, no active.group side effects.
//   - Cancel from active: bridge ended (visits + supervisors close).
//   - Conflict detection (#2139): staff (different room) and student
//     sub-checks produce warnings; shared rooms are sanctioned.
//   - Re-plan-week: deletes only planned non-spontaneous; all other kinds survive.
//
// All fixtures use testpkg.CreateTest* and package-clone ownership — no hardcoded IDs.
package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	repos    *repositories.Factory
	db       *bun.DB
	ctx      context.Context
	roomID   int64
	staffID  int64
	student1 int64
	student2 int64
	tmplID   int64
	period   *scheduleModels.CalendarPeriod
}

// buildLifecycle prepares the minimum scaffold for instance-lifecycle tests:
// one tenant tx, one room, one staff, two students, one template (is_template).
// Returns the setup + a runnable cleanup that removes everything we created.
func buildLifecycle(t *testing.T) *lifecycleSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	// Register DB close FIRST so it runs LAST (LIFO) — after every other
	// t.Cleanup has released its row references.

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err)
	require.NoError(t, serviceFactory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))

	// Own tenant per caller, subtests included: every subtest builds its own
	// lifecycle and then asserts tenant-wide (loadLifecycleExceptions counts
	// every exception of the tenant), so sharing the parent's tenant would
	// make each subtest see its predecessors' rows (#2419).
	ctx := testpkg.OwnCtx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("LC-Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "LC", fmt.Sprintf("Super-%d", suffix))
	student1 := testpkg.CreateTestStudent(t, db, "LC-Alice", fmt.Sprintf("One-%d", suffix), "3a")
	student2 := testpkg.CreateTestStudent(t, db, "LC-Bob", fmt.Sprintf("Two-%d", suffix), "3a")

	// A template-ish activity group. IsTemplate flag is immaterial for the
	// lifecycle path; we only need a non-nil FK target for the spontaneous=
	// false branch to fire.
	templateRow := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("LC-Tmpl-%d", suffix))
	period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("LC-Period-%d", suffix),
		timezone.NewDate(2000, 1, 1), timezone.NewDate(2100, 1, 1))
	testpkg.SetCalendarPeriodActive(t, db, period, true)

	// Parent-fixture cleanup registered BEFORE any per-test child cleanups,
	// so LIFO orders children → parents → db.Close.

	setup := &lifecycleSetup{
		factory:  serviceFactory,
		repos:    repoFactory,
		db:       db,
		ctx:      ctx,
		roomID:   room.ID,
		staffID:  staff.ID,
		student1: student1.ID,
		student2: student2.ID,
		tmplID:   templateRow.ID,
		period:   period,
	}
	// These state-machine fixtures use fixed wall-clock windows. Time-policy
	// boundaries have dedicated clock-injected tests; keep this suite focused on
	// lifecycle persistence and bridge behavior.
	setup.svc = instanceServiceWithBroadcaster(setup, nil)
	return setup
}

func instanceServiceWithBroadcaster(s *lifecycleSetup, broadcaster realtime.Broadcaster) scheduleSvc.InstanceService {
	return scheduleSvc.NewInstanceService(scheduleSvc.InstanceServiceDependencies{
		InstanceRepo:       s.repos.ActivityInstance,
		IdempotencyRepo:    s.repos.InstanceIdempotency,
		InstanceStaffRepo:  s.repos.InstanceStaff,
		InstanceStudents:   s.repos.InstanceStudent,
		ExceptionRepo:      s.repos.ActivityException,
		ActiveGroupRepo:    s.repos.ActiveGroup,
		SupervisorRepo:     s.repos.GroupSupervisor,
		VisitRepo:          s.repos.ActiveVisit,
		RoomRepo:           s.repos.Room,
		ActivityGroupRepo:  s.repos.ActivityGroup,
		StaffRepo:          s.repos.Staff,
		StudentRepo:        s.repos.Student,
		CalendarPeriodRepo: s.repos.CalendarPeriod,
		ActiveService:      s.factory.Active,
		Materialization:    s.factory.Materialization,
		CareDayService:     s.factory.CareDay,
		DeviationEventRepo: s.repos.DeviationEvent,
		Broadcaster:        broadcaster,
		DB:                 s.db,
		Logger:             slog.Default(),
		RecoveryRepo:       scheduleRepo.NewActivityRecoveryRepository(s.db),
	})
}

// seedInstance inserts one planned activity_instance plus optional staff
// and student rows. Returns the instance for the caller to act on.
func seedInstance(t *testing.T, s *lifecycleSetup, withStaff bool, withStudents bool) *scheduleModels.ActivityInstance {
	t.Helper()
	ai := &scheduleModels.ActivityInstance{
		Date:            scheduleModels.NewDate(2026, 4, 20),
		ActivityGroupID: &s.tmplID,
		Title:           "Lifecycle-Test",
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          scheduleModels.InstanceStatusPlanned,
		IsSpontaneous:   false,
	}
	ai.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	if withStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true}
		row.SetTenantID(testpkg.Tenant(t))
		_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
	if withStudents {
		for _, sid := range []int64{s.student1, s.student2} {
			row := &scheduleModels.InstanceStudent{InstanceID: ai.ID, StudentID: sid, Status: scheduleModels.AttendanceStatusExpected}
			row.SetTenantID(testpkg.Tenant(t))
			_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_students`).Exec(s.ctx)
			require.NoError(t, err)
		}
	}
	return ai
}

func seedSpontaneousInstance(t *testing.T, s *lifecycleSetup, withStaff bool) *scheduleModels.ActivityInstance {
	t.Helper()
	ai := &scheduleModels.ActivityInstance{
		Date:          scheduleModels.NewDate(2026, 4, 20),
		Title:         "Lifecycle-Test-Spontaneous",
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		Status:        scheduleModels.InstanceStatusPlanned,
		IsSpontaneous: true,
	}
	ai.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	if withStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true}
		row.SetTenantID(testpkg.Tenant(t))
		_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
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
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(s.ctx)
	require.NoError(t, err)
}

// --- #1840 understaffed acknowledgement -------------------------------------

func TestInstance_SetUnderstaffedAck_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)
	ai := seedInstance(t, s, false, false)

	note := "keine Vertretung verfügbar"
	got, err := svc.SetUnderstaffedAck(s.ctx, ai.ID, true, &note, nil)
	require.NoError(t, err)
	assert.True(t, got.UnderstaffedAck)
	require.NotNil(t, got.UnderstaffedNote)
	assert.Equal(t, note, *got.UnderstaffedNote)

	reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.UnderstaffedAck)
	require.NotNil(t, reloaded.UnderstaffedNote)
	assert.Equal(t, note, *reloaded.UnderstaffedNote)

	// Clearing the flag also clears the note so a stale reason cannot linger.
	cleared, err := svc.SetUnderstaffedAck(s.ctx, ai.ID, false, nil, nil)
	require.NoError(t, err)
	assert.False(t, cleared.UnderstaffedAck)
	assert.Nil(t, cleared.UnderstaffedNote)

	reloaded2, err := s.repos.ActivityInstance.FindByID(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.False(t, reloaded2.UnderstaffedAck)
	assert.Nil(t, reloaded2.UnderstaffedNote)
}

func TestInstance_SetUnderstaffedAck_CompletedRejected(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)
	ai := seedInstance(t, s, false, false)
	forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCompleted)

	_, err := svc.SetUnderstaffedAck(s.ctx, ai.ID, true, nil, nil)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

// #1840: the deliberately-unstaffed flag may not be set while non-absent staff
// remain — that would persist contradictory state (a block that reads as
// unstaffed yet never appears in /gaps because it is staffed).
func TestInstance_SetUnderstaffedAck_RejectedWhileStaffed(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)
	ai := seedInstance(t, s, true, false) // one non-absent primary staff row

	_, err := svc.SetUnderstaffedAck(s.ctx, ai.ID, true, nil, nil)
	assert.ErrorIs(t, err, scheduleSvc.ErrUnderstaffedAckStillStaffed)

	// The flag never flipped in the DB.
	reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.False(t, reloaded.UnderstaffedAck)

	// Clearing the flag stays allowed even while staffed.
	cleared, err := svc.SetUnderstaffedAck(s.ctx, ai.ID, false, nil, nil)
	require.NoError(t, err)
	assert.False(t, cleared.UnderstaffedAck)
}

// #1840: a single planned position may be deliberately left unfilled while
// other staff remain. The acknowledgement is therefore allowed on a partially
// staffed block (present < planned) — it is rejected only when the block is
// fully staffed (see TestInstance_SetUnderstaffedAck_RejectedWhileStaffed).
func TestInstance_SetUnderstaffedAck_AllowedWhilePartiallyStaffed(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)
	ai := seedInstance(t, s, false, false) // no staff seeded; we add two below

	absentStaff := testpkg.CreateTestStaff(t, s.db, "LC-Absent", fmt.Sprintf("P-%d", time.Now().UnixNano()))

	// Two planned positions: one present, one absent with no replacement.
	testpkg.CreateTestInstanceStaff(t, s.db, ai.ID, s.staffID, testpkg.InstanceStaffOpts{IsPrimary: true})
	testpkg.CreateTestInstanceStaff(t, s.db, ai.ID, absentStaff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	note := "keine Vertretung für die zweite Position"
	got, err := svc.SetUnderstaffedAck(s.ctx, ai.ID, true, &note, nil)
	require.NoError(t, err)
	assert.True(t, got.UnderstaffedAck)
	require.NotNil(t, got.UnderstaffedNote)
	assert.Equal(t, note, *got.UnderstaffedNote)
}

func TestInstance_SetUnderstaffedAck_NotFound(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)

	_, err := svc.SetUnderstaffedAck(s.ctx, 99999999, true, nil, nil)
	assert.ErrorIs(t, err, scheduleSvc.ErrInstanceNotFound)
}

// #1840: Cancel persists the optional reason.
func TestInstance_Cancel_WithReason_Persists(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	svc := instanceServiceWithBroadcaster(s, nil)
	ai := seedInstance(t, s, false, false)

	reason := "Ausflug"
	cancelled, err := svc.Cancel(s.ctx, ai.ID, &reason, nil)
	require.NoError(t, err)
	require.NotNil(t, cancelled.CancelReason)
	assert.Equal(t, "Ausflug", *cancelled.CancelReason)

	reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, ai.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.CancelReason)
	assert.Equal(t, "Ausflug", *reloaded.CancelReason)
}

// --- State machine: happy paths ---------------------------------------------

func TestInstance_Start_HappyPath(t *testing.T) {
	t.Parallel()

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
}

func TestInstance_Start_BroadcastsActiveSupervisionChanged(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := instanceServiceWithBroadcaster(s, broadcaster)

	ai := seedInstance(t, s, true, true)

	result, err := svc.Start(s.ctx, ai.ID, s.staffID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Instance.ActiveGroupID)

	tenantEvents := broadcaster.CallsByMethod("tenant")
	var instanceStarted, activeSupervisionChanged *realtime.Event
	for i := range tenantEvents {
		event := &tenantEvents[i].Event
		switch event.Type {
		case realtime.EventInstanceStarted:
			instanceStarted = event
		case realtime.EventActiveSupervisionChanged:
			activeSupervisionChanged = event
		}
	}

	require.NotNil(t, instanceStarted, "expected instance_started tenant broadcast")
	require.NotNil(t, activeSupervisionChanged, "expected active_supervision_changed tenant broadcast")
	assert.Equal(t, instanceStarted.ActiveGroupID, activeSupervisionChanged.ActiveGroupID)
	require.NotNil(t, activeSupervisionChanged.Data.InstanceID)
	assert.Equal(t, fmt.Sprintf("%d", ai.ID), *activeSupervisionChanged.Data.InstanceID)
	require.NotNil(t, activeSupervisionChanged.Data.Reason)
	assert.Equal(t, "instance_started", *activeSupervisionChanged.Data.Reason)

	t.Cleanup(func() {
		_, _ = s.factory.Active.FindSupervisorsByActiveGroupID(s.ctx, result.ActiveGroupID)
	})
}

func TestInstance_Start_BroadcastsGroupAndTenantTimetableEvent(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := instanceServiceWithBroadcaster(s, broadcaster)

	ai := seedInstance(t, s, true, false)
	result, err := svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = s.factory.Active.FindSupervisorsByActiveGroupID(s.ctx, result.ActiveGroupID)
	})

	groupCalls := broadcaster.CallsByMethod("group")
	tenantCalls := broadcaster.CallsByMethod("tenant")
	require.Len(t, groupCalls, 1)
	require.Len(t, tenantCalls, 2)
	assert.Equal(t, tenant.FromContext(s.ctx), tenantCalls[0].TenantID)
	assert.Equal(t, realtime.EventInstanceStarted, groupCalls[0].Event.Type)
	assert.Equal(t, realtime.EventInstanceStarted, tenantCalls[0].Event.Type)
	assert.Equal(t, realtime.EventActiveSupervisionChanged, tenantCalls[1].Event.Type)
	assert.Equal(t, groupCalls[0].Event.ActiveGroupID, tenantCalls[0].Event.ActiveGroupID)
	assert.Equal(t, groupCalls[0].Event.ActiveGroupID, tenantCalls[1].Event.ActiveGroupID)
}

func TestInstance_CreateRejectsDateOutsideActiveCalendarPeriod(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	_, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:      timezone.NewDate(2101, 4, 21),
		StartTime: time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:     "Außerhalb des Zeitraums",
		RoomID:    s.roomID,
	})

	require.ErrorIs(t, err, scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod)
}

func TestInstance_CreateSpontaneousSkipsActiveCalendarPeriodValidation(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	isSpontaneous := true
	created, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:          timezone.NewDate(2101, 4, 21),
		StartTime:     time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:         "Spontan außerhalb des Zeitraums",
		RoomID:        s.roomID,
		IsSpontaneous: &isSpontaneous,
	})

	require.NoError(t, err)
	assert.True(t, created.IsSpontaneous)
}

func TestInstance_UpdatePlannedRejectsDateOutsideActiveCalendarPeriod(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedInstance(t, s, false, false)
	_, err := s.svc.UpdatePlanned(s.ctx, instance.ID, scheduleSvc.UpdateInstanceInput{
		Date:      timezone.NewDate(2101, 4, 21),
		StartTime: instance.StartTime,
		EndTime:   instance.EndTime,
		Title:     "Außerhalb des Zeitraums",
		RoomID:    s.roomID,
	}, nil)

	require.ErrorIs(t, err, scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod)
	stored, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, instance.Date, stored.Date)
}

func TestInstance_UpdatePlannedKeepsDateAfterPeriodDeactivation(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedInstance(t, s, false, false)
	testpkg.SetCalendarPeriodActive(t, s.db, s.period, false)

	updated, err := s.svc.UpdatePlanned(s.ctx, instance.ID, scheduleSvc.UpdateInstanceInput{
		Date:      timezone.Date(instance.Date),
		StartTime: instance.StartTime,
		EndTime:   instance.EndTime,
		Title:     "Bearbeitet nach Deaktivierung",
		RoomID:    s.roomID,
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, instance.Date, updated.Date)
}

func TestInstance_UpdatePlannedMovesSpontaneousOutsideActiveCalendarPeriod(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	targetDate := timezone.NewDate(2101, 4, 21)

	updated, err := s.svc.UpdatePlanned(s.ctx, instance.ID, scheduleSvc.UpdateInstanceInput{
		Date:      targetDate,
		StartTime: instance.StartTime,
		EndTime:   instance.EndTime,
		Title:     "Spontan verschoben",
		RoomID:    s.roomID,
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, scheduleModels.Date(targetDate), updated.Date)
	assert.True(t, updated.IsSpontaneous)
}

func TestInstance_UpdatePlannedConvertsSpontaneousRejectsOutsideActiveCalendarPeriod(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	periodID := s.period.ID
	_, err := s.svc.UpdatePlanned(s.ctx, instance.ID, scheduleSvc.UpdateInstanceInput{
		Date:             timezone.NewDate(2101, 4, 21),
		StartTime:        instance.StartTime,
		EndTime:          instance.EndTime,
		Title:            "Als geplant verschieben",
		RoomID:           s.roomID,
		ActivityGroupID:  &s.tmplID,
		CalendarPeriodID: &periodID,
	}, nil)

	require.ErrorIs(t, err, scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod)
}

func TestInstance_UpdatePlannedConvertsSpontaneousSameDateRejectsInactiveCalendarPeriod(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	periodID := s.period.ID
	testpkg.SetCalendarPeriodActive(t, s.db, s.period, false)

	_, err := s.svc.UpdatePlanned(s.ctx, instance.ID, scheduleSvc.UpdateInstanceInput{
		Date:             timezone.Date(instance.Date),
		StartTime:        instance.StartTime,
		EndTime:          instance.EndTime,
		Title:            "Als geplant behalten",
		RoomID:           s.roomID,
		ActivityGroupID:  &s.tmplID,
		CalendarPeriodID: &periodID,
	}, nil)

	require.ErrorIs(t, err, scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod)
}

func TestInstance_CreateIdempotencyRetryReturnsStoredResultAfterPeriodDeactivation(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	key := "retry-after-period-deactivation"
	req := scheduleSvc.CreateInstanceInput{
		Date:           timezone.NewDate(2026, 4, 21),
		StartTime:      time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:        time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:          "Idempotent außerhalb nach Deaktivierung",
		RoomID:         s.roomID,
		IdempotencyKey: &key,
	}
	created, err := s.svc.Create(s.ctx, req)
	require.NoError(t, err)

	testpkg.SetCalendarPeriodActive(t, s.db, s.period, false)

	replayed, err := s.svc.Create(s.ctx, req)
	require.NoError(t, err)
	assert.Equal(t, created.ID, replayed.ID)
}

// TestInstance_PlannedCRUD_BroadcastsStaffingDeviationChanged pins the SSE
// invalidation for ordinary planned-instance CRUD (#1844): create, edit and
// delete trigger no lifecycle transition, so no instance_* event fires — the
// tenant-wide staffing_deviation_changed signal is the only thing keeping an
// open "Heute geplant" card (focus revalidation disabled) or planner from
// staying stale until reload.
func TestInstance_PlannedCRUD_BroadcastsStaffingDeviationChanged(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := instanceServiceWithBroadcaster(s, broadcaster)

	deviationSources := func() []string {
		sources := []string{}
		for _, call := range broadcaster.CallsByMethod("tenant") {
			if call.Event.Type == realtime.EventStaffingDeviationChanged {
				require.NotNil(t, call.Event.Data.Source)
				sources = append(sources, *call.Event.Data.Source)
			}
		}
		return sources
	}

	inst, err := svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:      timezone.NewDate(2026, 4, 21),
		StartTime: time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:     "CRUD-Broadcast-Test",
		RoomID:    s.roomID,
		StaffIDs:  []int64{s.staffID},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"instance_create"}, deviationSources())

	_, err = svc.UpdatePlanned(s.ctx, inst.ID, scheduleSvc.UpdateInstanceInput{
		Date:      timezone.Date(inst.Date),
		StartTime: inst.StartTime,
		EndTime:   inst.EndTime,
		Title:     "CRUD-Broadcast-Test (edited)",
		RoomID:    s.roomID,
		StaffIDs:  []int64{s.staffID},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"instance_create", "instance_update"}, deviationSources())

	require.NoError(t, svc.DeleteCancelled(s.ctx, inst.ID))
	assert.Equal(t,
		[]string{"instance_create", "instance_update", "instance_delete"},
		deviationSources())
}

func TestInstance_Complete_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	completed, err := s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletedAt)

	// Bridge active.group must be closed.
	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime, "active.group should have been ended")
}

func TestInstance_Complete_ConfirmationMustMatchOpenVisits(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	now := time.Now()
	visit := &activeModels.Visit{StudentID: s.student1, ActiveGroupID: started.ActiveGroupID, EntryTime: now}
	visit.SetTenantID(testpkg.Tenant(t))
	_, err = s.db.NewInsert().Model(visit).ModelTableExpr(`active.visits`).Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.Complete(scheduleSvc.WithCompletionConfirmation(s.ctx, []int64{s.student2}), ai.ID)
	require.ErrorIs(t, err, scheduleSvc.ErrCompletionConfirmationStale)

	completed, err := s.svc.Complete(scheduleSvc.WithCompletionConfirmation(s.ctx, []int64{s.student1}), ai.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, completed.Status)
}

func TestInstance_Reopen_RestoresAbsenceProvenance(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)

	instanceWeekday := int(ai.Date.Weekday())
	if instanceWeekday == 0 {
		instanceWeekday = 7
	}
	for weekday := scheduleModels.WeekdayMonday; weekday <= scheduleModels.WeekdayFriday; weekday++ {
		if weekday == instanceWeekday {
			continue
		}
		for _, studentID := range []int64{s.student1, s.student2} {
			testpkg.CreateTestArrivalSchedule(t, s.db, studentID, weekday, s.staffID, "08:00")
		}
	}

	sick := &activeModels.StudentStatusDay{
		StudentID:  s.student1,
		Date:       timezone.Date(ai.Date),
		Status:     activeModels.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     activeModels.StudentStatusSourcePlanned,
	}
	require.NoError(t, s.repos.StudentStatusDay.UpsertReported(s.ctx, sick))

	partial := testpkg.CreateTestPickupException(t, s.db, s.student2, ai.Date.AddDays(1), s.staffID, "13:00", "Termin")
	_, err := s.db.NewUpdate().
		Table("schedule.instance_students").
		Set("status = ?", scheduleModels.AttendanceStatusAbsent).
		Set("substatus = ?", scheduleModels.AttendanceSubstatusExcused).
		Set("pickup_exception_id = ?", partial.ID).
		Where("instance_id = ? AND student_id = ?", ai.ID, s.student2).
		Exec(s.ctx)
	require.NoError(t, err)

	beforeSick := fetchAttendance(t, s, ai.ID, s.student1)
	require.Equal(t, scheduleModels.AttendanceStatusAbsent, beforeSick.Status)
	require.NotNil(t, beforeSick.StudentStatusDayID)
	assert.Equal(t, sick.ID, *beforeSick.StudentStatusDayID)
	require.NotNil(t, beforeSick.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusSick, *beforeSick.Substatus)

	beforePickup := fetchAttendance(t, s, ai.ID, s.student2)
	require.Equal(t, scheduleModels.AttendanceStatusAbsent, beforePickup.Status)
	require.NotNil(t, beforePickup.PickupExceptionID)
	assert.Equal(t, partial.ID, *beforePickup.PickupExceptionID)

	_, err = s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	afterCompleteSick := fetchAttendance(t, s, ai.ID, s.student1)
	assert.True(t, afterCompleteSick.NotScheduled)
	assert.Nil(t, afterCompleteSick.StudentStatusDayID)
	afterCompletePickup := fetchAttendance(t, s, ai.ID, s.student2)
	assert.True(t, afterCompletePickup.NotScheduled)
	assert.Nil(t, afterCompletePickup.PickupExceptionID)

	reopened, err := s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusActive, reopened.Instance.Status)

	restoredSick := fetchAttendance(t, s, ai.ID, s.student1)
	assert.False(t, restoredSick.NotScheduled)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, restoredSick.Status)
	require.NotNil(t, restoredSick.StudentStatusDayID)
	assert.Equal(t, sick.ID, *restoredSick.StudentStatusDayID)
	require.NotNil(t, restoredSick.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusSick, *restoredSick.Substatus)

	restoredPickup := fetchAttendance(t, s, ai.ID, s.student2)
	assert.False(t, restoredPickup.NotScheduled)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, restoredPickup.Status)
	require.NotNil(t, restoredPickup.PickupExceptionID)
	assert.Equal(t, partial.ID, *restoredPickup.PickupExceptionID)
}

func TestInstance_Reopen_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	// completed_by is an auth.accounts FK. These fixtures do not seed an
	// account, so Complete writes a nil actor and Reopen uses the admin path.
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	reopened, err := s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusActive, reopened.Instance.Status)
	assert.Equal(t, started.ActiveGroupID, reopened.ActiveGroupID)

	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	assert.Nil(t, group.EndTime)
}

func TestInstance_Reopen_RejectsOccupiedRoom(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	first := seedInstance(t, s, true, false)
	_, err := s.svc.Start(s.ctx, first.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, first.ID)
	require.NoError(t, err)

	// A second planned row on the same template+date violates
	// idx_activity_instances_template_unique. Occupancy cares about the
	// room, so a spontaneous instance in the same room is enough.
	second := seedSpontaneousInstance(t, s, true)
	_, err = s.svc.Start(s.ctx, second.ID, 0)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, first.ID, 0, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, activeSvc.ErrRoomConflict)
}

func TestInstance_Reopen_RejectsRoomCapacity(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	now := time.Now()
	for _, studentID := range []int64{s.student1, s.student2} {
		visit := &activeModels.Visit{StudentID: studentID, ActiveGroupID: started.ActiveGroupID, EntryTime: now}
		visit.SetTenantID(testpkg.Tenant(t))
		_, insertErr := s.db.NewInsert().Model(visit).ModelTableExpr(`active.visits`).Exec(s.ctx)
		require.NoError(t, insertErr)
	}

	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	_, err = s.db.NewUpdate().
		Table("facilities.rooms").
		Set("capacity = ?", 1).
		Where("id = ?", s.roomID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, activeSvc.ErrRoomCapacityExceeded)
}

func TestInstance_Reopen_RejectsNonActor(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, false)
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, ai.ID, 42, false)
	require.ErrorIs(t, err, scheduleSvc.ErrTimetableOperationForbidden)
}

func TestInstance_Reopen_RejectsExpiredWindow(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, false)
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	_, err = s.db.NewUpdate().
		Table("schedule.activity_instances").
		Set("reopen_until = ?", time.Now().Add(-time.Minute)).
		Where("id = ?", ai.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

func TestInstance_Reopen_RejectsAttendanceChangedAfterComplete(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	_, err = s.db.NewUpdate().
		Table("schedule.instance_students").
		Set("updated_at = ?", time.Now().Add(time.Minute)).
		Where("instance_id = ?", ai.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.ErrorIs(t, err, scheduleSvc.ErrTimetableOperationConflict)
}

func TestInstance_Reopen_RejectsSupervisorChangedAfterComplete(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	// active.group_supervisors carries a BEFORE UPDATE trigger
	// (update_modified_column) that overwrites updated_at with the DB clock's
	// now(), so an explicit future timestamp cannot be written. Bump the row
	// (the trigger stamps it "now") and move the recorded completion instant
	// two minutes into the past instead: the conflict predicate
	// updated_at > completed_at then holds regardless of Go-vs-Postgres clock
	// skew. Comparing the trigger stamp against the Go-clock completed_at
	// directly is a sub-millisecond race that made this test flaky.
	_, err = s.db.NewUpdate().
		Table("active.group_supervisors").
		Set("updated_at = now()").
		Where("group_id = ?", started.ActiveGroupID).
		Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.db.NewUpdate().
		Table("schedule.activity_instances").
		Set("completed_at = completed_at - interval '2 minutes'").
		Where("id = ?", ai.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.ErrorIs(t, err, scheduleSvc.ErrTimetableOperationConflict)
}

func TestInstance_Reopen_RejectsStaffNowSupervisingElsewhere(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, false)
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-ReopenRoom-%d", time.Now().UnixNano()))
	otherGroup := testpkg.CreateTestActiveGroup(t, s.db, s.tmplID, otherRoom.ID)
	testpkg.CreateTestGroupSupervisor(t, s.db, s.staffID, otherGroup.ID, "supervisor")

	_, err = s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.ErrorIs(t, err, scheduleSvc.ErrTimetableOperationConflict)
}

func TestInstance_Reopen_RejectsMissingSnapshot(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, false)
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)
	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	_, err = s.db.NewUpdate().
		Table("schedule.activity_instances").
		Set("completion_snapshot = NULL").
		Where("id = ?", ai.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.svc.Reopen(s.ctx, ai.ID, 0, true)
	require.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

func TestInstance_Cancel_FromPlanned_LeavesNoActiveGroup(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, false)

	cancelled, err := s.svc.Cancel(s.ctx, ai.ID, nil, nil)
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
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, false)
	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	cancelled, err := s.svc.Cancel(s.ctx, ai.ID, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCancelled, cancelled.Status)

	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime, "cancel-from-active must end active.group")
}

// --- State machine: 409 branches --------------------------------------------

func TestInstance_Start_Rejects_NonPlanned(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, false, false)
	forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCompleted)

	_, err := s.svc.Cancel(s.ctx, ai.ID, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

func TestInstance_Cancel_Rejects_AlreadyCancelled(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, false, false)
	forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCancelled)

	_, err := s.svc.Cancel(s.ctx, ai.ID, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
}

func TestInstance_Start_NotFound(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, false, false)

	// Primary staff (present) + a second staff flagged absent.
	primary := &scheduleModels.InstanceStaff{
		InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true, IsAbsent: false,
	}
	primary.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(primary).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	absent := testpkg.CreateTestStaff(t, s.db, "Absent", fmt.Sprintf("Staff-%d", time.Now().UnixNano()))

	absentRow := &scheduleModels.InstanceStaff{
		InstanceID: ai.ID, StaffID: absent.ID, IsPrimary: false, IsAbsent: true,
	}
	absentRow.SetTenantID(testpkg.Tenant(t))
	_, err = s.db.NewInsert().Model(absentRow).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	sups, err := s.factory.Active.FindSupervisorsByActiveGroupID(s.ctx, result.ActiveGroupID)
	require.NoError(t, err)
	require.Len(t, sups, 1, "only the non-absent staff should become a supervisor")
	assert.Equal(t, s.staffID, sups[0].StaffID)

	// And the SupervisorIDs on the SSE envelope (visible via start result)
	// must not include the absent staff — handled through activeStaffRows,
	// which the integration test can't introspect directly, but the
	// supervisor count above is the authoritative proof.
}

// --- Conflict detection -----------------------------------------------------

func TestInstance_Start_OccupiedRoomIsNotAConflict(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// Pre-seed a live active.group in the same room. Since #2139 a shared
	// room is sanctioned (parallel groups may use one room), so starting in
	// an occupied room must produce NO warning at all.
	now := time.Now()
	preGroup := &activeModels.Group{
		StartTime: now, LastActivity: now, TimeoutMinutes: 30,
		GroupID: &s.tmplID, RoomID: s.roomID,
	}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	assert.Empty(t, result.Warnings, "a pure room overlap must not warn (#2139); got %+v", result.Warnings)
	assert.Equal(t, scheduleModels.InstanceStatusActive, result.Instance.Status, "warnings never block the transition")
}

func TestInstance_Start_StaffSameRoomIsNotAConflict(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// Our staff member already supervises a live group in the SAME room the
	// instance starts in — sanctioned parallel supervision (#2139).
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: s.roomID}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	sup := &activeModels.GroupSupervisor{StaffID: s.staffID, GroupID: preGroup.ID, Role: "supervisor", StartDate: timezone.DateFromTime(now)}
	sup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateGroupSupervisor(s.ctx, sup))

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	for _, w := range result.Warnings {
		assert.NotEqual(t, scheduleSvc.ConflictKindStaff, w.Kind,
			"same-room supervision must not warn (#2139); got %+v", result.Warnings)
	}
}

// seedBridgedActiveInstance inserts an ACTIVE, spontaneous activity_instance
// bridged to the given running group (active_group_id set), optionally with
// one instance_staff row carrying a multi-room override. This is the shape a
// started multi-room block leaves behind: the active.group stores only the
// primary room, the per-staff room lives on the bridged instance's roster
// (#2151 review).
func seedBridgedActiveInstance(t *testing.T, s *lifecycleSetup, group *activeModels.Group, staffID int64, overrideRoomID *int64, withStaffRow bool) {
	t.Helper()
	ai := &scheduleModels.ActivityInstance{
		Date:          scheduleModels.NewDate(2026, 4, 20),
		Title:         "Lifecycle-Bridged",
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:        group.RoomID,
		Status:        scheduleModels.InstanceStatusActive,
		IsSpontaneous: true,
		ActiveGroupID: &group.ID,
	}
	ai.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	if withStaffRow {
		row := &scheduleModels.InstanceStaff{InstanceID: ai.ID, StaffID: staffID, IsPrimary: true, RoomID: overrideRoomID}
		row.SetTenantID(testpkg.Tenant(t))
		_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
}

func TestInstance_Start_StaffBridgedOverrideSameRoom_NoConflict(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// The running group's PRIMARY room differs from the new instance's room,
	// but the staff member's multi-room override on the bridged instance puts
	// them in exactly the new instance's room — sanctioned, no warning.
	// Comparing against the group's primary room alone would warn here.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-BridgeRoomA-%d", time.Now().UnixNano()))

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: otherRoom.ID}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	sup := &activeModels.GroupSupervisor{StaffID: s.staffID, GroupID: preGroup.ID, Role: "supervisor", StartDate: timezone.DateFromTime(now)}
	sup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateGroupSupervisor(s.ctx, sup))

	seedBridgedActiveInstance(t, s, preGroup, s.staffID, &s.roomID, true)

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	for _, w := range result.Warnings {
		assert.NotEqual(t, scheduleSvc.ConflictKindStaff, w.Kind,
			"bridged same-room override must not warn (#2151 review); got %+v", result.Warnings)
	}
}

func TestInstance_Start_StaffBridgedOverrideDifferentRoom_Conflict(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// The running group's PRIMARY room equals the new instance's room, but the
	// staff member's multi-room override on the bridged instance puts them in
	// ANOTHER room — a real double-booking that the primary-room comparison
	// would have suppressed.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-BridgeRoomB-%d", time.Now().UnixNano()))

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: s.roomID}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	sup := &activeModels.GroupSupervisor{StaffID: s.staffID, GroupID: preGroup.ID, Role: "supervisor", StartDate: timezone.DateFromTime(now)}
	sup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateGroupSupervisor(s.ctx, sup))

	seedBridgedActiveInstance(t, s, preGroup, s.staffID, &otherRoom.ID, true)

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	hasStaffWarning := false
	for _, w := range result.Warnings {
		if w.Kind == scheduleSvc.ConflictKindStaff && w.ResourceID == s.staffID {
			hasStaffWarning = true
			assert.Contains(t, w.Message, "anderen Raum")
		}
	}
	assert.True(t, hasStaffWarning,
		"bridged different-room override is a real double-booking and must warn (#2151 review); got %+v", result.Warnings)
}

func TestInstance_Start_StaffBridgedWithoutRosterRow_Conflict(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// The supervision points at a group bridged to an instance whose roster
	// does not contain the staff member — the effective room is undetermined,
	// so the warning must stay ("not certainly the same room").
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: s.roomID}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	sup := &activeModels.GroupSupervisor{StaffID: s.staffID, GroupID: preGroup.ID, Role: "supervisor", StartDate: timezone.DateFromTime(now)}
	sup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateGroupSupervisor(s.ctx, sup))

	seedBridgedActiveInstance(t, s, preGroup, s.staffID, nil, false)

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	hasStaffWarning := false
	for _, w := range result.Warnings {
		if w.Kind == scheduleSvc.ConflictKindStaff && w.ResourceID == s.staffID {
			hasStaffWarning = true
			assert.Contains(t, w.Message, "nicht eindeutig bestimmbar")
		}
	}
	assert.True(t, hasStaffWarning,
		"undetermined effective room must keep the warning (#2151 review); got %+v", result.Warnings)
}

func TestInstance_Start_ConflictWarning_Staff(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// Pre-seed an active.group + a live supervision by our staff member on it.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-OtherRoom-%d", time.Now().UnixNano()))

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: otherRoom.ID}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	sup := &activeModels.GroupSupervisor{StaffID: s.staffID, GroupID: preGroup.ID, Role: "supervisor", StartDate: timezone.DateFromTime(now)}
	sup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateGroupSupervisor(s.ctx, sup))

	ai := seedInstance(t, s, true, false)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	hasStaffWarning := false
	for _, w := range result.Warnings {
		if w.Kind == scheduleSvc.ConflictKindStaff && w.ResourceID == s.staffID {
			hasStaffWarning = true
		}
	}
	assert.True(t, hasStaffWarning, "staff supervising elsewhere must produce a staff warning; got %+v", result.Warnings)
}

func TestInstance_Start_ConflictWarning_Student(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	// Pre-seed an active.group in a different room + an open visit by student1.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("LC-OtherRoom2-%d", time.Now().UnixNano()))

	now := time.Now()
	preGroup := &activeModels.Group{StartTime: now, LastActivity: now, TimeoutMinutes: 30, GroupID: &s.tmplID, RoomID: otherRoom.ID}
	preGroup.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.factory.Active.CreateActiveGroup(s.ctx, preGroup))

	// Raw INSERT — factory.Active.CreateVisit triggers attendance-side-effect
	// writes (checked_in_by on active.attendance) that need a full staff/
	// account chain. We only need the visit row for the conflict check.
	visit := &activeModels.Visit{StudentID: s.student1, ActiveGroupID: preGroup.ID, EntryTime: now}
	visit.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(visit).ModelTableExpr(`active.visits`).Exec(s.ctx)
	require.NoError(t, err)

	ai := seedInstance(t, s, true, true)
	result, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

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
	t.Parallel()

	s := buildLifecycle(t)

	from := timezone.NewDate(2026, 4, 20)
	to := from.AddDays(6)

	// Six seeded rows inside the window — one of each protected kind.
	plannedNormal := insertInstance(t, s, from, scheduleModels.InstanceStatusPlanned, false)
	plannedSpont := insertInstance(t, s, from.AddDays(1), scheduleModels.InstanceStatusPlanned, true)
	active := insertInstance(t, s, from.AddDays(2), scheduleModels.InstanceStatusActive, false)
	completed := insertInstance(t, s, from.AddDays(3), scheduleModels.InstanceStatusCompleted, false)
	cancelled := insertInstance(t, s, from.AddDays(4), scheduleModels.InstanceStatusCancelled, false)
	// #2299: a manual planning-module block without an offering is planned
	// (is_spontaneous=false) but has no template — materialization could never
	// recreate it, so a whole-grid re-plan must not delete it.
	plannedManual := &scheduleModels.ActivityInstance{
		Date:      scheduleModels.Date(from.AddDays(1)),
		Title:     fmt.Sprintf("Row-manual-%d", time.Now().UnixNano()),
		StartTime: time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		RoomID:    s.roomID,
		Status:    scheduleModels.InstanceStatusPlanned,
	}
	plannedManual.SetTenantID(tenant.FromContext(s.ctx))
	_, err := s.db.NewInsert().Model(plannedManual).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	// Manual cleanup for the survivors (the planned-normal row may be deleted
	// by ReplanWeek; if so, the cleanup becomes a no-op).

	_, err = s.svc.ReplanWeek(s.ctx, from, to, nil, nil)
	require.NoError(t, err)

	assert.False(t, instanceExists(t, s, plannedNormal), "planned non-spontaneous must be deleted")
	assert.True(t, instanceExists(t, s, plannedSpont), "spontaneous planned must survive")
	assert.True(t, instanceExists(t, s, plannedManual.ID), "manual no-offering planned block must survive a whole-grid re-plan")
	assert.True(t, instanceExists(t, s, active), "active must survive")
	assert.True(t, instanceExists(t, s, completed), "completed must survive")
	assert.True(t, instanceExists(t, s, cancelled), "cancelled must survive")
}

func TestInstance_ReplanWeek_RemovesFutureLegacyWeekendInstances(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	from := timezone.NewDate(2026, 8, 24)
	for from.Weekday() != time.Monday {
		from = from.AddDays(1)
	}
	to := from.AddDays(6)
	legacyWeekend := insertInstance(t, s, from.AddDays(5), scheduleModels.InstanceStatusPlanned, false)

	result, err := s.svc.ReplanWeek(s.ctx, from, to, &s.tmplID, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, result.DeletedInstances)
	assert.False(t, instanceExists(t, s, legacyWeekend), "legacy weekend rows must not survive a series re-plan with stale template data")
}

// WP-B3: a re-plan scoped to one template deletes only that template's
// planned non-spontaneous instances — other templates' rows survive.
func TestInstance_ReplanWeek_ScopedToActivityGroup(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	from := timezone.NewDate(2026, 7, 20)
	to := from.AddDays(6)

	// Second template in the same tenant with its own planned instance.
	otherGroup := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("LC-Other-%d", time.Now().UnixNano()))

	mine := insertInstance(t, s, from, scheduleModels.InstanceStatusPlanned, false)
	other := &scheduleModels.ActivityInstance{
		Date:            scheduleModels.Date(from.AddDays(1)),
		Title:           fmt.Sprintf("Row-other-%d", time.Now().UnixNano()),
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          scheduleModels.InstanceStatusPlanned,
		IsSpontaneous:   false,
		ActivityGroupID: &otherGroup.ID,
	}
	other.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(other).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	result, err := s.svc.ReplanWeek(s.ctx, from, to, &s.tmplID, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, result.DeletedInstances, "only the scoped template's planned instance is deleted")
	assert.False(t, instanceExists(t, s, mine), "scoped template's planned instance must be deleted")
	assert.True(t, instanceExists(t, s, other.ID), "other template's planned instance survives a scoped re-plan")
}

func TestInstance_ReplanWeek_RejectsInvalidWindowAndMissingTenant(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	from := timezone.NewDate(2026, 4, 27)
	to := timezone.NewDate(2026, 4, 20)

	_, err := s.svc.ReplanWeek(s.ctx, from, to, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "to_date")

	_, err = s.svc.ReplanWeek(context.Background(), to, from, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant")
}

func TestInstance_Create_AssignsUniqueStaffAndStudents(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	createdBy := s.staffID

	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:             timezone.NewDate(2026, 5, 4),
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

func TestInstance_CreateAndUpdatePlanned_ReapplyActiveStatusDays(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	createdDate := timezone.NewDate(2026, 5, 12)
	updatedDate := timezone.NewDate(2026, 5, 13)

	sick := &activeModels.StudentStatusDay{
		StudentID: s.student1, Date: createdDate, Status: activeModels.StudentStatusDaySick,
		ReportedAt: time.Now(), Source: activeModels.StudentStatusSourcePlanned,
	}
	excused := &activeModels.StudentStatusDay{
		StudentID: s.student2, Date: updatedDate, Status: activeModels.StudentStatusDayExcused,
		ReportedAt: time.Now().Add(time.Minute), Source: activeModels.StudentStatusSourcePlanned,
	}
	require.NoError(t, s.repos.StudentStatusDay.UpsertReported(s.ctx, sick))
	require.NoError(t, s.repos.StudentStatusDay.UpsertReported(s.ctx, excused))

	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date: createdDate, StartTime: time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime: time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC), Title: "Status provenance create",
		RoomID: s.roomID, ActivityGroupID: &s.tmplID, StudentIDs: []int64{s.student1},
	})
	require.NoError(t, err)

	createdRow := fetchAttendance(t, s, inst.ID, s.student1)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, createdRow.Status)
	require.NotNil(t, createdRow.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusSick, *createdRow.Substatus)
	require.NotNil(t, createdRow.StudentStatusDayID)
	assert.Equal(t, sick.ID, *createdRow.StudentStatusDayID)

	updated, err := s.svc.UpdatePlanned(s.ctx, inst.ID, scheduleSvc.UpdateInstanceInput{
		Date: updatedDate, StartTime: time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime: time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC), Title: "Status provenance update",
		RoomID: s.roomID, ActivityGroupID: &s.tmplID, StudentIDs: []int64{s.student2},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.Date(updatedDate), updated.Date)

	updatedRow := fetchAttendance(t, s, inst.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, updatedRow.Status)
	require.NotNil(t, updatedRow.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *updatedRow.Substatus)
	require.NotNil(t, updatedRow.StudentStatusDayID)
	assert.Equal(t, excused.ID, *updatedRow.StudentStatusDayID)
}

func TestInstance_Create_RejectsCrossTenantReferences(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, s.db, foreignTenantID)

	foreignRoom := testpkg.CreateTestRoomForTenant(t, s.db, foreignTenantID, "LC-Foreign-Room")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, s.db, foreignTenantID, "LC", "ForeignStaff")
	foreignStudent := testpkg.CreateTestStudentForTenant(t, s.db, foreignTenantID, "LC", "ForeignStudent", "3b")
	foreignTemplate := testpkg.CreateTestActivityGroupForTenant(t, s.db, foreignTenantID, "LC-Foreign-Template")

	valid := scheduleSvc.CreateInstanceInput{
		Date:            timezone.NewDate(2026, 5, 9),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Cross tenant should fail",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student1},
	}

	cases := []struct {
		name   string
		mutate func(*scheduleSvc.CreateInstanceInput)
	}{
		{name: "room", mutate: func(in *scheduleSvc.CreateInstanceInput) { in.RoomID = foreignRoom.ID }},
		{name: "activity group", mutate: func(in *scheduleSvc.CreateInstanceInput) { in.ActivityGroupID = &foreignTemplate.ID }},
		{name: "staff", mutate: func(in *scheduleSvc.CreateInstanceInput) { in.StaffIDs = []int64{foreignStaff.ID} }},
		{name: "student", mutate: func(in *scheduleSvc.CreateInstanceInput) { in.StudentIDs = []int64{foreignStudent.ID} }},
		{name: "created by", mutate: func(in *scheduleSvc.CreateInstanceInput) { in.CreatedByStaffID = &foreignStaff.ID }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			before := countRowsWhere(t, s, "schedule.activity_instances", "room_id", req.RoomID)

			inst, err := s.svc.Create(s.ctx, req)

			require.Error(t, err)
			assert.Nil(t, inst)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceReference)
			assert.Equal(t, before, countRowsWhere(t, s, "schedule.activity_instances", "room_id", req.RoomID))
		})
	}
}

func TestInstance_Create_SpontaneousAndMissingTenant(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	inst, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:      timezone.NewDate(2026, 5, 5),
		StartTime: time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		Title:     "Spontaneous service test",
		RoomID:    s.roomID,
	})
	require.NoError(t, err)
	// #2299: is_spontaneous records the creation origin. A planning-module
	// create without an explicit flag is a planned block even without an
	// offering link — the lifecycle time guards must apply.
	assert.False(t, inst.IsSpontaneous)
	assert.Nil(t, inst.ActivityGroupID)

	isSpontaneous := true
	linked, err := s.svc.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:            timezone.NewDate(2026, 5, 7),
		StartTime:       time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		Title:           "Linked spontaneous service test",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		IsSpontaneous:   &isSpontaneous,
	})
	require.NoError(t, err)
	assert.True(t, linked.IsSpontaneous)
	require.NotNil(t, linked.ActivityGroupID)
	assert.Equal(t, s.tmplID, *linked.ActivityGroupID)

	_, err = s.svc.Create(context.Background(), scheduleSvc.CreateInstanceInput{
		Date:      timezone.NewDate(2026, 5, 6),
		StartTime: time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		Title:     "No tenant",
		RoomID:    s.roomID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant")
}

func TestInstance_UpdatePlanned_ReplacesAssignmentsAndFields(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)

	updated, err := s.svc.UpdatePlanned(s.ctx, ai.ID, scheduleSvc.UpdateInstanceInput{
		Date:       timezone.NewDate(2026, 5, 7),
		StartTime:  time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:    time.Date(1, 1, 1, 17, 30, 0, 0, time.UTC),
		Title:      "Updated planned instance",
		RoomID:     s.roomID,
		StaffIDs:   []int64{s.staffID, s.staffID},
		StudentIDs: []int64{s.student2},
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, "Updated planned instance", updated.Title)
	assert.Equal(t, "16:00", updated.StartTime.Format("15:04"))
	// #2299: unlinking the offering must NOT flip the block to spontaneous —
	// is_spontaneous keeps recording the creation origin.
	assert.False(t, updated.IsSpontaneous, "removing the offering link must not toggle is_spontaneous")
	assert.Nil(t, updated.ActivityGroupID)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", ai.ID))
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_students", "instance_id", ai.ID))

	row := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, row.Status)
}

// guardedLifecycleSettings satisfies scheduleSvc.LifecycleSettings for the
// time-policy pass-through test below.
type guardedLifecycleSettings struct {
	leadMinutes       int
	enforcePlannedEnd bool
}

func (s guardedLifecycleSettings) ResolveInt(context.Context, string) (int, error) {
	return s.leadMinutes, nil
}

func (s guardedLifecycleSettings) ResolveBool(context.Context, string) (bool, error) {
	return s.enforcePlannedEnd, nil
}

// #2299 guard pass-through: a block created in the planning module WITHOUT an
// offering link is subject to the lead-time guard, while an ad-hoc block
// (explicit IsSpontaneous=true) stays exempt at the same clock.
func TestInstance_Start_TimePolicyAppliesToNoOfferingPlannedBlock(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	date := timezone.NewDate(2026, 4, 22)
	// 08:00 Berlin — far before the 14:00 start minus the 15-minute lead.
	now := time.Date(2026, 4, 22, 8, 0, 0, 0, timezone.Berlin)
	guarded := scheduleSvc.NewInstanceService(scheduleSvc.InstanceServiceDependencies{
		InstanceRepo:       s.repos.ActivityInstance,
		IdempotencyRepo:    s.repos.InstanceIdempotency,
		InstanceStaffRepo:  s.repos.InstanceStaff,
		InstanceStudents:   s.repos.InstanceStudent,
		ExceptionRepo:      s.repos.ActivityException,
		ActiveGroupRepo:    s.repos.ActiveGroup,
		SupervisorRepo:     s.repos.GroupSupervisor,
		VisitRepo:          s.repos.ActiveVisit,
		RoomRepo:           s.repos.Room,
		ActivityGroupRepo:  s.repos.ActivityGroup,
		StaffRepo:          s.repos.Staff,
		StudentRepo:        s.repos.Student,
		CalendarPeriodRepo: s.repos.CalendarPeriod,
		ActiveService:      s.factory.Active,
		Materialization:    s.factory.Materialization,
		CareDayService:     s.factory.CareDay,
		DeviationEventRepo: s.repos.DeviationEvent,
		DB:                 s.db,
		Logger:             slog.Default(),
		RecoveryRepo:       scheduleRepo.NewActivityRecoveryRepository(s.db),
		Settings:           guardedLifecycleSettings{leadMinutes: 15, enforcePlannedEnd: true},
		Now:                func() time.Time { return now },
		EnforceTimePolicy:  true,
	})

	planned, err := guarded.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:      date,
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:     "Guarded no-offering block",
		RoomID:    s.roomID,
	})
	require.NoError(t, err)

	_, err = guarded.Start(s.ctx, planned.ID, s.staffID)
	require.ErrorIs(t, err, scheduleSvc.ErrInstanceStartTooEarly,
		"planning-module block without offering must hit the lead-time guard")

	isSpontaneous := true
	adhoc, err := guarded.Create(s.ctx, scheduleSvc.CreateInstanceInput{
		Date:          date,
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:         "Ad-hoc block",
		RoomID:        s.roomID,
		IsSpontaneous: &isSpontaneous,
	})
	require.NoError(t, err)

	_, err = guarded.Start(s.ctx, adhoc.ID, s.staffID)
	require.NoError(t, err, "ad-hoc block stays exempt from the time policy")
}

// #2299: is_spontaneous records the creation origin, so an edit that links an
// offering to a spontaneous block must not reclassify it as planned either.
func TestInstance_UpdatePlanned_KeepsSpontaneousOriginWhenLinkingOffering(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	ai := seedSpontaneousInstance(t, s, false)

	updated, err := s.svc.UpdatePlanned(s.ctx, ai.ID, scheduleSvc.UpdateInstanceInput{
		Date:            timezone.Date(ai.Date),
		StartTime:       ai.StartTime,
		EndTime:         ai.EndTime,
		Title:           ai.Title,
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
	}, nil)
	require.NoError(t, err)

	assert.True(t, updated.IsSpontaneous, "linking an offering must not toggle is_spontaneous")
	require.NotNil(t, updated.ActivityGroupID)
	assert.Equal(t, s.tmplID, *updated.ActivityGroupID)
}

func TestInstance_UpdatePlanned_RejectsCrossTenantReferencesBeforeMutation(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, s.db, foreignTenantID)

	foreignRoom := testpkg.CreateTestRoomForTenant(t, s.db, foreignTenantID, "LC-Update-Foreign-Room")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, s.db, foreignTenantID, "LC", "UpdateForeignStaff")
	foreignStudent := testpkg.CreateTestStudentForTenant(t, s.db, foreignTenantID, "LC", "UpdateForeignStudent", "4b")
	foreignTemplate := testpkg.CreateTestActivityGroupForTenant(t, s.db, foreignTenantID, "LC-Update-Foreign-Template")

	ai := seedInstance(t, s, true, true)
	valid := scheduleSvc.UpdateInstanceInput{
		Date:            timezone.NewDate(2026, 5, 11),
		StartTime:       time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		Title:           "Cross tenant update should fail",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{s.staffID},
		StudentIDs:      []int64{s.student1},
	}

	cases := []struct {
		name   string
		mutate func(*scheduleSvc.UpdateInstanceInput)
	}{
		{name: "room", mutate: func(in *scheduleSvc.UpdateInstanceInput) { in.RoomID = foreignRoom.ID }},
		{name: "activity group", mutate: func(in *scheduleSvc.UpdateInstanceInput) { in.ActivityGroupID = &foreignTemplate.ID }},
		{name: "staff", mutate: func(in *scheduleSvc.UpdateInstanceInput) { in.StaffIDs = []int64{foreignStaff.ID} }},
		{name: "student", mutate: func(in *scheduleSvc.UpdateInstanceInput) { in.StudentIDs = []int64{foreignStudent.ID} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)

			updated, err := s.svc.UpdatePlanned(s.ctx, ai.ID, req, nil)

			require.Error(t, err)
			assert.Nil(t, updated)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceReference)
			assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", ai.ID))
			assert.Equal(t, 2, countRowsWhere(t, s, "schedule.instance_students", "instance_id", ai.ID))
		})
	}
}

func TestInstance_UpdatePlanned_RejectsNonPlanned(t *testing.T) {
	t.Parallel()

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
				Date:      timezone.NewDate(2026, 5, 8),
				StartTime: time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
				Title:     "Should fail",
				RoomID:    s.roomID,
			}, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, scheduleSvc.ErrInvalidInstanceTransition)
		})
	}
}

func TestInstance_DeleteCancelled_PlannedOrCancelled(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
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

	t.Run("deletes planned template occurrence and writes cancellation exception", func(t *testing.T) {
		s := buildLifecycle(t)
		ai := seedInstance(t, s, false, false)

		require.NoError(t, s.svc.DeleteCancelled(s.ctx, ai.ID))
		assert.False(t, instanceExists(t, s, ai.ID))

		exceptions := loadLifecycleExceptions(t, s)
		require.Len(t, exceptions, 1)
		assert.Equal(t, s.tmplID, exceptions[0].ActivityGroupID)
		assert.Equal(t, ai.Date, exceptions[0].ExceptionDate)
		assert.Equal(t, scheduleModels.ActivityExceptionCancelled, exceptions[0].ExceptionType)
	})

	t.Run("deletes cancelled template occurrence", func(t *testing.T) {
		s := buildLifecycle(t)
		ai := seedInstance(t, s, false, false)
		forceSetInstanceStatus(t, s, ai.ID, scheduleModels.InstanceStatusCancelled)

		require.NoError(t, s.svc.DeleteCancelled(s.ctx, ai.ID))
		assert.False(t, instanceExists(t, s, ai.ID))
		assert.Len(t, loadLifecycleExceptions(t, s), 1)
	})

	t.Run("deletes spontaneous planned occurrence without exception", func(t *testing.T) {
		s := buildLifecycle(t)
		id := insertInstance(t, s, timezone.NewDate(2026, 4, 20), scheduleModels.InstanceStatusPlanned, true)

		require.NoError(t, s.svc.DeleteCancelled(s.ctx, id))
		assert.False(t, instanceExists(t, s, id))
		assert.Empty(t, loadLifecycleExceptions(t, s))
	})

	t.Run("rejects single delete when template has multiple same-day slots", func(t *testing.T) {
		s := buildLifecycle(t)
		date := timezone.NewDate(2026, 4, 20)
		firstID := insertInstanceAt(t, s, date, scheduleModels.InstanceStatusPlanned, false, 14)
		secondID := insertInstanceAt(t, s, date, scheduleModels.InstanceStatusPlanned, false, 15)

		err := s.svc.DeleteCancelled(s.ctx, firstID)
		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleSvc.ErrAmbiguousTemplateInstanceDelete)
		assert.True(t, instanceExists(t, s, firstID))
		assert.True(t, instanceExists(t, s, secondID))
		assert.Empty(t, loadLifecycleExceptions(t, s))
	})
}

func insertInstance(t *testing.T, s *lifecycleSetup, date timezone.Date, status string, spontaneous bool) int64 {
	t.Helper()
	return insertInstanceAt(t, s, date, status, spontaneous, 14)
}

func insertInstanceAt(t *testing.T, s *lifecycleSetup, date timezone.Date, status string, spontaneous bool, startHour int) int64 {
	t.Helper()
	endHour := startHour + 1
	row := &scheduleModels.ActivityInstance{
		Date:          scheduleModels.Date(date),
		Title:         fmt.Sprintf("Row-%s-%d", status, time.Now().UnixNano()),
		StartTime:     time.Date(1, 1, 1, startHour, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, endHour, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		Status:        status,
		IsSpontaneous: spontaneous,
	}
	if !spontaneous {
		row.ActivityGroupID = &s.tmplID
	}
	row.SetTenantID(testpkg.Tenant(t))
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
		Where("tenant_id = ?", testpkg.Tenant(t)).
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
		Where(`"ai".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	return count > 0
}

func loadLifecycleExceptions(t *testing.T, s *lifecycleSetup) []*scheduleModels.ActivityException {
	t.Helper()
	var rows []*scheduleModels.ActivityException
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		Where(`"activity_exception".tenant_id = ?`, testpkg.Tenant(t)).
		Order("exception_date ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
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
		Where(`"instance_student".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(s.ctx)
	require.NoError(t, err)
	return &row
}

// Complete must flip every remaining 'expected' row to 'absent' inside the
// same tenant tx. Present rows stay untouched.
func TestInstance_Complete_MarksRemainingExpectedAsAbsent(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true) // both students expected
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	// Simulate a live check-in for student1 via the monotonic mirror path.
	repoFactory := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db))
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

// Complete must leave children the care plan does not place here today alone
// (#1747). Assigning a whole group or year to an activity puts every member on
// every occurrence; stamping the ones who are not booked that weekday "absent"
// claims they failed to show up to care they never had, and that claim then
// travels into attendance statistics and exports.
func TestInstance_Complete_SkipsChildrenNotInCareThatDay(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true) // both students expected

	// student2 has a care plan, but only for weekdays other than the
	// instance's. student1 keeps no plan at all and must stay unaffected.
	instanceWeekday := int(ai.Date.Weekday())
	if instanceWeekday == 0 {
		instanceWeekday = 7 // ISO: the schedule tables key Sunday as 7
	}
	for weekday := scheduleModels.WeekdayMonday; weekday <= scheduleModels.WeekdayFriday; weekday++ {
		if weekday == instanceWeekday {
			continue
		}
		testpkg.CreateTestArrivalSchedule(t, s.db, s.student2, weekday, s.staffID, "08:00")
	}

	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	got1 := fetchAttendance(t, s, ai.ID, s.student1)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got1.Status,
		"a child without any care plan is still expected, so Complete marks them absent")
	assert.False(t, got1.NotScheduled,
		"a real absence must never carry the non-booking marker")

	got2 := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got2.Status,
		"a child not booked for care on this weekday must not be recorded absent")
	assert.True(t, got2.NotScheduled,
		"the spared row carries the reason it was spared, so no later writer of "+
			"status can create or destroy the fact")
}

// The mirror image of the test above: a child who IS booked today and whose day
// somebody cancelled ("Kommt heute nicht") must still be stamped absent (#1747
// review). The care plan booked the day, so the absence is real — and a
// surviving 'expected' row on a completed instance is filtered out of the
// attendance history and the exports, so skipping the stamp would erase the
// absence instead of recording it.
func TestInstance_Complete_RecordsAbsenceForCancelledDay(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true) // both students expected

	instanceWeekday := int(ai.Date.Weekday())
	if instanceWeekday == 0 {
		instanceWeekday = 7 // ISO: the schedule tables key Sunday as 7
	}

	// student2 is booked for care on the instance's weekday …
	testpkg.CreateTestArrivalSchedule(t, s.db, s.student2, instanceWeekday, s.staffID, "08:00")
	// … and the day was cancelled: an arrival exception with no time.
	testpkg.CreateTestArrivalException(t, s.db, s.student2, ai.Date, s.staffID, "", "krank gemeldet")

	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	_, err = s.svc.Complete(s.ctx, ai.ID)
	require.NoError(t, err)

	got := fetchAttendance(t, s, ai.ID, s.student2)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status,
		"a cancelled care day is an absence and must be recorded as one")
}

// Cancel must NOT flip expected → absent. A cancelled instance never ran,
// so "absent" would falsely imply the student failed to show up to an event
// that happened. The cancelled status on the instance itself is the signal.
func TestInstance_Cancel_FromPlanned_DoesNotTouchAttendance(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, false, true)

	cancelled, err := s.svc.Cancel(s.ctx, ai.ID, nil, nil)
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
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, true, true)
	_, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err)

	repoFactory := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db))
	_, err = repoFactory.InstanceStudent.UpdateAttendanceFromCheckin(
		s.ctx, ai.ID, s.student1, time.Now(),
	)
	require.NoError(t, err)

	_, err = s.svc.Cancel(s.ctx, ai.ID, nil, nil)
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
	t.Parallel()

	s := buildLifecycle(t)

	ai := seedInstance(t, s, false, false)
	repoFactory := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db))
	warnings := scheduleSvc.DetectStartConflicts(s.ctx, scheduleSvc.ConflictDependencies{
		GroupRepo:         repoFactory.ActiveGroup,
		SupervisorRepo:    repoFactory.GroupSupervisor,
		VisitRepo:         repoFactory.ActiveVisit,
		InstanceRepo:      repoFactory.ActivityInstance,
		InstanceStaffRepo: repoFactory.InstanceStaff,
		InstanceStudents:  repoFactory.InstanceStudent,
	}, ai, slog.Default())
	assert.Empty(t, warnings, "clean-room, no staff, no students → no warnings")
}
