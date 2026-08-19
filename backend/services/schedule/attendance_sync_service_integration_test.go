// Hermetic integration tests for the WP-B10 attendance sync service.
//
// Covers every branch of the graceful-degradation tree declared in
// attendance_sync_service.go:
//
//	B1 visit.ActiveGroupID <= 0    → nil
//	B2 instance lookup error       → not hermetically reachable (covered by repo tests)
//	B3 no instance bridged         → nil (walk-in)
//	B4 instance_student lookup err → not hermetically reachable (covered by repo tests)
//	B5 no instance_student row     → durable unplanned slot row
//	B6 row already non-expected    → snapshot of current state, no write
//	B7 UPDATE error                → not hermetically reachable (covered by repo tests)
//	B8 rowsAffected=0 race         → snapshot, no persisted change
//	B9 happy path                  → snapshot with status=present, row flipped
//
// Check-out is covered via MirrorCheckOutForVisit — no mutation regardless
// of current state.
package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// attendanceSyncSetup bundles the syncer and the repos used to seed test
// state. All cleanup registered via t.Cleanup in LIFO order.
type attendanceSyncSetup struct {
	syncer      *scheduleSvc.AttendanceSyncService
	instRepo    scheduleModels.ActivityInstanceRepository
	isRepo      scheduleModels.InstanceStudentRepository
	groupRepo   activeModels.GroupRepository
	db          *bun.DB
	ctx         context.Context
	roomID      int64
	activityID  int64
	activeGroup *activeModels.Group
	instance    *scheduleModels.ActivityInstance
}

func buildAttendanceSyncSetup(t *testing.T) *attendanceSyncSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("AS-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AS-Activity-%d", suffix))

	// Bridge active.group — a running session linked to the instance.
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	// Create the instance and manually bridge to the active.group (simulating
	// the post-Start state — we don't exercise the lifecycle service here).
	instance := &scheduleModels.ActivityInstance{
		Date:            timezone.NewDate(2026, 4, 21),
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("AS-Instance-%d", suffix),
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          scheduleModels.InstanceStatusActive,
		ActiveGroupID:   &activeGroup.ID,
	}
	instance.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	// Register parent cleanup first because t.Cleanup executes LIFO: the
	// instance callback registered below must run before its room/group parents.
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity.ID, room.ID)
		testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		// active.group cleanup happens via CleanupActivityFixtures below
	})

	return &attendanceSyncSetup{
		syncer: scheduleSvc.NewAttendanceSyncService(
			scheduleRepo.NewActivityInstanceRepository(db),
			scheduleRepo.NewInstanceStudentRepository(db),
			slog.Default(),
		),
		instRepo:    scheduleRepo.NewActivityInstanceRepository(db),
		isRepo:      scheduleRepo.NewInstanceStudentRepository(db),
		groupRepo:   activeRepo.NewGroupRepository(db),
		db:          db,
		ctx:         ctx,
		roomID:      room.ID,
		activityID:  activity.ID,
		activeGroup: activeGroup,
		instance:    instance,
	}
}

// seedInstanceStudent inserts an instance_students row for s.instance + studentID
// with the given status. Registers cleanup automatically.
func seedInstanceStudent(t *testing.T, s *attendanceSyncSetup, studentID int64, status string) *scheduleModels.InstanceStudent {
	t.Helper()
	row := &scheduleModels.InstanceStudent{
		InstanceID: s.instance.ID,
		StudentID:  studentID,
		Status:     status,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.isRepo.Create(s.ctx, row))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID)
	})
	return row
}

// --- MirrorCheckInForVisit ---------------------------------------------------

func TestAttendanceSync_MirrorCheckIn_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Happy", fmt.Sprintf("A-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	row := seedInstanceStudent(t, s, student.ID, scheduleModels.AttendanceStatusExpected)

	entryTime := time.Date(2026, 4, 21, 13, 5, 0, 0, time.UTC)
	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     entryTime,
	}

	snap := s.syncer.MirrorCheckInForVisit(s.ctx, visit)
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, snap.Status)
	assert.Nil(t, snap.Substatus)
	assert.Nil(t, snap.Note)

	// DB state: row flipped to present + checked_in_at stamped.
	got, err := s.isRepo.FindByID(s.ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, got.Status)
	require.NotNil(t, got.CheckedInAt)
	assert.WithinDuration(t, entryTime, *got.CheckedInAt, time.Second)
}

func TestAttendanceSync_MirrorCheckIn_WalkIn_NoInstance(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Walk", fmt.Sprintf("W-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	// NEW active.group that is NOT bridged to any instance.
	orphanGroup := testpkg.CreateTestActiveGroup(t, s.db, s.activityID, s.roomID)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", orphanGroup.ID) })

	snap := s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: orphanGroup.ID,
		EntryTime:     time.Now(),
	})
	assert.Nil(t, snap, "no instance bridged — walk-in, no snapshot")
}

func TestAttendanceSync_MirrorCheckIn_WalkIn_NotEnrolled(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	// No instance_students row seeded for this student.
	student := testpkg.CreateTestStudent(t, s.db, "AS-Unsubbed", fmt.Sprintf("U-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	snap := s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     time.Now(),
	})
	require.NotNil(t, snap)
	assert.True(t, snap.IsUnplanned)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, snap.Status)
	row, err := s.isRepo.FindByInstanceAndStudent(s.ctx, s.instance.ID, student.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.True(t, row.IsUnplanned)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })
}

func TestAttendanceSync_MirrorCheckIn_CompletedWalkInPersistsClosedInterval(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Closed", fmt.Sprintf("U-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	entryTime := time.Date(2026, 4, 21, 12, 10, 0, 0, time.UTC)
	exitTime := entryTime.Add(45 * time.Minute)
	snap := s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
		StudentID: student.ID, ActiveGroupID: s.activeGroup.ID,
		EntryTime: entryTime, ExitTime: &exitTime,
	})
	require.NotNil(t, snap)
	assert.True(t, snap.IsUnplanned)

	row, err := s.isRepo.FindByInstanceAndStudent(s.ctx, s.instance.ID, student.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })
	require.NotNil(t, row.CheckedInAt)
	require.NotNil(t, row.CheckedOutAt)
	assert.WithinDuration(t, entryTime, *row.CheckedInAt, time.Second)
	assert.WithinDuration(t, exitTime, *row.CheckedOutAt, time.Second)
}

func TestAttendanceSync_BulkSessionEndPersistsSlotCheckout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout bool
	}{
		{name: "explicit session end"},
		{name: "session timeout", timeout: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := buildAttendanceSyncSetup(t)
			student := testpkg.CreateTestStudent(t, s.db, "AS-Bulk", fmt.Sprintf("B-%d", time.Now().UnixNano()), "3a")
			t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

			row := seedInstanceStudent(t, s, student.ID, scheduleModels.AttendanceStatusExpected)
			entryTime := time.Now().Add(-time.Hour)
			require.NotNil(t, s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
				StudentID: student.ID, ActiveGroupID: s.activeGroup.ID, EntryTime: entryTime,
			}))
			visit := testpkg.CreateTestVisit(t, s.db, student.ID, s.activeGroup.ID, entryTime, nil)
			t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.visits", visit.ID) })

			factory, err := services.NewFactory(repositories.NewFactory(s.db), s.db, slog.Default())
			require.NoError(t, err)
			if tt.timeout {
				device := testpkg.CreateTestDevice(t, s.db, fmt.Sprintf("AS-Bulk-%d", time.Now().UnixNano()))
				t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, device.ID) })
				_, err = s.db.NewUpdate().
					Table("active.groups").
					Set("device_id = ?", device.ID).
					Where("id = ?", s.activeGroup.ID).
					Exec(s.ctx)
				require.NoError(t, err)
				_, err = factory.Active.ProcessSessionTimeout(s.ctx, device.ID)
			} else {
				err = factory.Active.EndActivitySession(s.ctx, s.activeGroup.ID)
			}
			require.NoError(t, err)

			got, err := s.isRepo.FindByID(s.ctx, row.ID)
			require.NoError(t, err)
			require.NotNil(t, got.CheckedOutAt, "bulk-ended visit must close its slot attendance")
			assert.False(t, got.CheckedOutAt.Before(entryTime))
		})
	}
}

func TestAttendancePerCareSlot_MorningPresentAfternoonSickAndClearIndependent(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Slots", fmt.Sprintf("S-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	morning := seedInstanceStudent(t, s, student.ID, scheduleModels.AttendanceStatusExpected)
	morningCheckIn := time.Date(2026, 4, 21, 5, 5, 0, 0, time.UTC)
	morningCheckOut := time.Date(2026, 4, 21, 6, 0, 0, 0, time.UTC)
	require.NotNil(t, s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
		StudentID: student.ID, ActiveGroupID: s.activeGroup.ID, EntryTime: morningCheckIn,
	}))
	require.NotNil(t, s.syncer.MirrorCheckOutForVisit(s.ctx, &activeModels.Visit{
		StudentID: student.ID, ActiveGroupID: s.activeGroup.ID,
		EntryTime: morningCheckIn, ExitTime: &morningCheckOut,
	}))

	afternoonInstance := &scheduleModels.ActivityInstance{
		Date: timezone.NewDate(2026, 4, 21), Title: fmt.Sprintf("AS-Afternoon-%d", time.Now().UnixNano()),
		StartTime: time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:    s.roomID, Status: scheduleModels.InstanceStatusPlanned,
	}
	afternoonInstance.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.instRepo.Create(s.ctx, afternoonInstance))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", afternoonInstance.ID) })
	afternoon := &scheduleModels.InstanceStudent{
		InstanceID: afternoonInstance.ID, StudentID: student.ID,
		Status: scheduleModels.AttendanceStatusExpected,
	}
	afternoon.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.isRepo.Create(s.ctx, afternoon))

	statusRepo := activeRepo.NewStudentStatusDayRepository(s.db)
	statusDay := &activeModels.StudentStatusDay{
		StudentID: student.ID, Date: afternoonInstance.Date,
		Status: activeModels.StudentStatusDaySick, ReportedAt: morningCheckOut,
		Source: activeModels.StudentStatusSourceManual,
	}
	require.NoError(t, statusRepo.UpsertReported(s.ctx, statusDay))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.student_status_days", statusDay.ID) })

	gotMorning, err := s.isRepo.FindByID(s.ctx, morning.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, gotMorning.Status)
	require.NotNil(t, gotMorning.CheckedOutAt)
	assert.WithinDuration(t, morningCheckOut, *gotMorning.CheckedOutAt, time.Second)
	assert.Nil(t, gotMorning.StudentStatusDayID, "observed morning attendance must not be owned by the later sickness")

	gotAfternoon, err := s.isRepo.FindByID(s.ctx, afternoon.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotAfternoon.Status)
	require.NotNil(t, gotAfternoon.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusSick, *gotAfternoon.Substatus)
	require.NotNil(t, gotAfternoon.StudentStatusDayID)
	assert.Equal(t, statusDay.ID, *gotAfternoon.StudentStatusDayID)

	historyRows, err := s.isRepo.FindInstancesWithAttendanceByStudentAndDateRange(
		s.ctx, student.ID, afternoonInstance.Date, afternoonInstance.Date,
	)
	require.NoError(t, err)
	require.Len(t, historyRows, 2, "history and exports must retain one row per care slot")

	require.NoError(t, statusRepo.MarkClearedByID(s.ctx, statusDay.ID, morningCheckOut.Add(time.Minute), activeModels.StudentStatusSourceManual))
	gotMorning, err = s.isRepo.FindByID(s.ctx, morning.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, gotMorning.Status)
	gotAfternoon, err = s.isRepo.FindByID(s.ctx, afternoon.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, gotAfternoon.Status)
	assert.Nil(t, gotAfternoon.Substatus)
}

func TestAttendanceSync_MirrorCheckIn_AlreadyPresent_NoClobber(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Dbl", fmt.Sprintf("D-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	firstCheckin := time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC)

	// Seed row as already-present with a stamped checked_in_at and a substatus
	// — exactly what a post-hoc staff edit would leave.
	excused := scheduleModels.AttendanceSubstatusExcused
	row := &scheduleModels.InstanceStudent{
		InstanceID:  s.instance.ID,
		StudentID:   student.ID,
		Status:      scheduleModels.AttendanceStatusPresent,
		Substatus:   &excused,
		CheckedInAt: &firstCheckin,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.isRepo.Create(s.ctx, row))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })

	// Second check-in — later time. Mirror MUST NOT overwrite checked_in_at
	// or the substatus.
	snap := s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     time.Date(2026, 4, 21, 14, 30, 0, 0, time.UTC),
	})
	require.NotNil(t, snap, "snapshot should still reflect current state for SSE")
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *snap.Substatus)

	// DB: original first-checkin timestamp must survive.
	got, err := s.isRepo.FindByID(s.ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CheckedInAt)
	assert.WithinDuration(t, firstCheckin, *got.CheckedInAt, time.Second)
}

func TestAttendanceSync_MirrorCheckIn_NilVisitOrZeroActiveGroup(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	// StudentID is never read in B1 (the ActiveGroupID<=0 check short-circuits
	// above it), but we still use a real fixture to stay hermetic.
	student := testpkg.CreateTestStudent(t, s.db, "AS-B1", fmt.Sprintf("B1-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	// Nil visit — B1
	assert.Nil(t, s.syncer.MirrorCheckInForVisit(s.ctx, nil))

	// Zero ActiveGroupID — B1
	assert.Nil(t, s.syncer.MirrorCheckInForVisit(s.ctx, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: 0,
	}))
}

func TestAttendanceSync_MirrorCheckIn_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Ten", fmt.Sprintf("T-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })
	seedInstanceStudent(t, s, student.ID, scheduleModels.AttendanceStatusExpected)

	// Call with tenant 2 context — RLS should hide everything.
	ctxT2 := testpkg.TenantContext(2)
	snap := s.syncer.MirrorCheckInForVisit(ctxT2, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     time.Now(),
	})
	assert.Nil(t, snap, "wrong tenant — instance invisible under RLS")
}

// --- MirrorCheckOutForVisit --------------------------------------------------

func TestAttendanceSync_LoadAttendance_ReturnsSnapshot(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Load", fmt.Sprintf("L-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	note := "bus verspätet"
	late := scheduleModels.AttendanceSubstatusLate
	row := &scheduleModels.InstanceStudent{
		InstanceID: s.instance.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
		Substatus:  &late,
		Note:       &note,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.isRepo.Create(s.ctx, row))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })

	snap := s.syncer.MirrorCheckOutForVisit(s.ctx, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
	})
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusLate, *snap.Substatus)
	require.NotNil(t, snap.Note)
	assert.Equal(t, "bus verspätet", *snap.Note)

	// Read-only: row must be unchanged (including updated_at).
	got, err := s.isRepo.FindByID(s.ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, row.UpdatedAt.UnixNano(), got.UpdatedAt.UnixNano(), "LoadAttendance must not mutate")
}

func TestAttendanceSync_LoadAttendance_NoRow(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-NoRow", fmt.Sprintf("N-%d", time.Now().UnixNano()), "3a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, student.ID) })

	snap := s.syncer.MirrorCheckOutForVisit(s.ctx, &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
	})
	assert.Nil(t, snap)
}
