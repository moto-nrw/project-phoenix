// Hermetic integration tests for the WP-B10 attendance mirror, the Timetable
// owner's command Student Presence triggers through its syncer port (#3424
// slice S3).
//
// Covers every branch of the graceful-degradation tree declared in
// modules/timetable/compose/attendance_mirror.go:
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
package presence_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// attendanceSyncSetup bundles the syncer and the repos used to seed test
// state. All cleanup registered via t.Cleanup in LIFO order.
type attendanceSyncSetup struct {
	syncer studentpresence.AttendanceSyncer
	api.TimetableOwnerRows
	statusRepo  *api.StudentStatusDayRepository
	groupRepo   studentpresence.SessionRecords
	db          *bun.DB
	ctx         context.Context
	roomID      int64
	activityID  int64
	activeGroup *testpkg.ActiveGroupRow
	instance    *testpkg.ActivityInstance
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
	instance := &testpkg.ActivityInstance{
		Date:            testpkg.Date(2026, 4, 21),
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("AS-Instance-%d", suffix),
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          studentpresence.ActivitySessionActive,
		ActiveGroupID:   &activeGroup.ID,
	}
	instance.TenantID = testpkg.Tenant(t)
	testpkg.InsertActivityInstanceRow(t, ctx, db, instance)
	owners := api.NewUnobservedTimetableDependencies(testpkg.NewDatabase(t, db))
	repoFactory := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db), owners)
	timetableRepos := api.NewAttendanceSyncTestRepositories(testpkg.MustDatabase(db), owners.Capability)
	instanceStudentRepo := timetableRepos.Participants

	// Register parent cleanup first because t.Cleanup executes LIFO: the
	// instance callback registered below must run before its room/group parents.
	t.Cleanup(func() {
		// active.group is tenant-owned and dies with the package clone
	})

	syncer, err := api.NewAttendanceSyncTestMirror(api.TimetableOwnerRows{Instances: timetableRepos.Instances, Participants: instanceStudentRepo}, slog.Default())
	require.NoError(t, err)
	return &attendanceSyncSetup{
		syncer:             syncer,
		TimetableOwnerRows: timetableRepos,
		statusRepo:         repoFactory.StudentStatusDay,
		groupRepo:          api.NewPresenceSessionRecords(testpkg.MustDatabase(db)),
		db:                 db,
		ctx:                ctx,
		roomID:             room.ID,
		activityID:         activity.ID,
		activeGroup:        activeGroup,
		instance:           instance,
	}
}

// newAttendanceMirror binds the Timetable owner's attendance mirror to the
// Presence syncer port the way the composition root does (#3424 slice S3).
func newAttendanceMirror(t testing.TB, rows api.TimetableOwnerRows, logger *slog.Logger) studentpresence.AttendanceSyncer {
	t.Helper()
	syncer, err := api.NewAttendanceSyncTestMirror(rows, logger)
	require.NoError(t, err)
	return syncer
}

// --- MirrorCheckInForVisit ---------------------------------------------------

func TestAttendanceSync_MirrorCheckIn_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Happy", fmt.Sprintf("A-%d", time.Now().UnixNano()), "3a")

	row := api.SeedAttendanceMirrorParticipant(func(err error) { require.NoError(t, err) }, s.ctx, s.TimetableOwnerRows, s.instance.ID, student.ID, testpkg.Tenant(t), studentpresence.SessionAttendanceExpected)

	entryTime := time.Date(2026, 4, 21, 13, 5, 0, 0, time.UTC)
	visit := &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     entryTime,
	}

	snap, checkInErr1 := s.syncer.MirrorCheckInForVisit(s.ctx, visit)
	require.NoError(t, checkInErr1)
	require.NotNil(t, snap)
	assert.Equal(t, studentpresence.SessionAttendancePresent, snap.Status)
	assert.Nil(t, snap.Substatus)
	assert.Nil(t, snap.Note)

	// DB state: row flipped to present + checked_in_at stamped.
	got, err := s.Participants.FindByID(s.ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, studentpresence.SessionAttendancePresent, got.Status)
	require.NotNil(t, got.CheckedInAt)
	assert.WithinDuration(t, entryTime, *got.CheckedInAt, time.Second)
}

func TestAttendanceSync_MirrorCheckIn_WalkIn_NoInstance(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Walk", fmt.Sprintf("W-%d", time.Now().UnixNano()), "3a")

	// NEW active.group that is NOT bridged to any instance.
	orphanGroup := testpkg.CreateTestActiveGroup(t, s.db, s.activityID, s.roomID)

	snap, checkInErr2 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: orphanGroup.ID,
		EntryTime:     time.Now(),
	})
	require.NoError(t, checkInErr2)
	assert.Nil(t, snap, "no instance bridged — walk-in, no snapshot")
}

func TestAttendanceSync_MirrorCheckIn_WalkIn_NotEnrolled(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	// No instance_students row seeded for this student.
	student := testpkg.CreateTestStudent(t, s.db, "AS-Unsubbed", fmt.Sprintf("U-%d", time.Now().UnixNano()), "3a")

	snap, checkInErr3 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     time.Now(),
	})
	require.NoError(t, checkInErr3)
	require.NotNil(t, snap)
	assert.True(t, snap.IsUnplanned)
	assert.Equal(t, studentpresence.SessionAttendancePresent, snap.Status)
	row, err := s.Participants.FindByInstanceAndStudent(s.ctx, s.instance.ID, student.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.True(t, row.IsUnplanned)
}

func TestAttendanceSync_MirrorCheckIn_CompletedWalkInPersistsClosedInterval(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Closed", fmt.Sprintf("U-%d", time.Now().UnixNano()), "3a")

	entryTime := time.Date(2026, 4, 21, 12, 10, 0, 0, time.UTC)
	exitTime := entryTime.Add(45 * time.Minute)
	snap, checkInErr4 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
		StudentID: student.ID, ActiveGroupID: s.activeGroup.ID,
		EntryTime: entryTime, ExitTime: &exitTime,
	})
	require.NoError(t, checkInErr4)
	require.NotNil(t, snap)
	assert.True(t, snap.IsUnplanned)

	row, err := s.Participants.FindByInstanceAndStudent(s.ctx, s.instance.ID, student.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
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

			row := api.SeedAttendanceMirrorParticipant(func(err error) { require.NoError(t, err) }, s.ctx, s.TimetableOwnerRows, s.instance.ID, student.ID, testpkg.Tenant(t), studentpresence.SessionAttendanceExpected)
			entryTime := time.Now().Add(-time.Hour)
			checkInSnapshot5, checkInErr5 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
				StudentID: student.ID, ActiveGroupID: s.activeGroup.ID, EntryTime: entryTime,
			})
			require.NoError(t, checkInErr5)
			require.NotNil(t, checkInSnapshot5)
			testpkg.CreateTestVisit(t, s.db, student.ID, s.activeGroup.ID, entryTime, nil)

			factory, err := api.NewServiceGraphForTests(api.NewRepositoryTestFactory(testpkg.NewDatabase(t, s.db), api.NewUnobservedTimetableDependencies(testpkg.NewDatabase(t, s.db))), testpkg.NewDatabase(t, s.db), slog.Default())
			require.NoError(t, err)
			if tt.timeout {
				device := testpkg.CreateTestDevice(t, s.db, fmt.Sprintf("AS-Bulk-%d", time.Now().UnixNano()))
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

			got, err := s.Participants.FindByID(s.ctx, row.ID)
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

	morning := api.SeedAttendanceMirrorParticipant(func(err error) { require.NoError(t, err) }, s.ctx, s.TimetableOwnerRows, s.instance.ID, student.ID, testpkg.Tenant(t), studentpresence.SessionAttendanceExpected)
	morningCheckIn := time.Date(2026, 4, 21, 5, 5, 0, 0, time.UTC)
	morningCheckOut := time.Date(2026, 4, 21, 6, 0, 0, 0, time.UTC)
	checkInSnapshot6, checkInErr6 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
		StudentID: student.ID, ActiveGroupID: s.activeGroup.ID, EntryTime: morningCheckIn,
	})
	require.NoError(t, checkInErr6)
	require.NotNil(t, checkInSnapshot6)
	checkoutSnapshot, checkoutErr := s.syncer.MirrorCheckOutForVisit(s.ctx, &studentpresence.Visit{
		StudentID: student.ID, ActiveGroupID: s.activeGroup.ID,
		EntryTime: morningCheckIn, ExitTime: &morningCheckOut,
	})
	require.NoError(t, checkoutErr)
	require.NotNil(t, checkoutSnapshot)

	afternoonInstance := api.NewAttendanceMirrorInstance()
	afternoonInstance.Date = calendar.NewDate(2026, 4, 21)
	afternoonInstance.Title = fmt.Sprintf("AS-Afternoon-%d", time.Now().UnixNano())
	afternoonInstance.StartTime = time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC)
	afternoonInstance.EndTime = time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)
	afternoonInstance.RoomID = s.roomID
	afternoonInstance.TenantID = testpkg.Tenant(t)
	require.NoError(t, s.Instances.Create(s.ctx, afternoonInstance))
	afternoon := api.NewAttendanceMirrorParticipant()
	afternoon.InstanceID = afternoonInstance.ID
	afternoon.StudentID = student.ID
	afternoon.Status = studentpresence.SessionAttendanceExpected
	afternoon.TenantID = testpkg.Tenant(t)
	require.NoError(t, s.Participants.Create(s.ctx, afternoon))

	statusRepo := s.statusRepo
	statusDay := &absencerecords.StudentStatusDay{
		StudentID: student.ID, Date: calendar.Date(afternoonInstance.Date),
		Status: absencerecords.StudentStatusDaySick, ReportedAt: morningCheckOut,
		Source: absencerecords.StudentStatusSourceManual,
	}
	require.NoError(t, statusRepo.UpsertReported(s.ctx, statusDay))

	gotMorning, err := s.Participants.FindByID(s.ctx, morning.ID)
	require.NoError(t, err)
	assert.Equal(t, studentpresence.SessionAttendancePresent, gotMorning.Status)
	require.NotNil(t, gotMorning.CheckedOutAt)
	assert.WithinDuration(t, morningCheckOut, *gotMorning.CheckedOutAt, time.Second)
	assert.Nil(t, gotMorning.StudentStatusDayID, "observed morning attendance must not be owned by the later sickness")

	gotAfternoon, err := s.Participants.FindByID(s.ctx, afternoon.ID)
	require.NoError(t, err)
	assert.Equal(t, studentpresence.SessionAttendanceAbsent, gotAfternoon.Status)
	require.NotNil(t, gotAfternoon.Substatus)
	assert.Equal(t, studentpresence.SessionAttendanceSubstatusSick, *gotAfternoon.Substatus)
	require.NotNil(t, gotAfternoon.StudentStatusDayID)
	assert.Equal(t, statusDay.ID, *gotAfternoon.StudentStatusDayID)

	historyRows, err := s.Participants.FindInstancesWithAttendanceByStudentAndDateRange(
		s.ctx, student.ID, afternoonInstance.Date, afternoonInstance.Date,
	)
	require.NoError(t, err)
	require.Len(t, historyRows, 2, "history and exports must retain one row per care slot")

	require.NoError(t, statusRepo.MarkClearedByID(s.ctx, statusDay.ID, morningCheckOut.Add(time.Minute), absencerecords.StudentStatusSourceManual))
	gotMorning, err = s.Participants.FindByID(s.ctx, morning.ID)
	require.NoError(t, err)
	assert.Equal(t, studentpresence.SessionAttendancePresent, gotMorning.Status)
	gotAfternoon, err = s.Participants.FindByID(s.ctx, afternoon.ID)
	require.NoError(t, err)
	assert.Equal(t, studentpresence.SessionAttendanceExpected, gotAfternoon.Status)
	assert.Nil(t, gotAfternoon.Substatus)
}

func TestAttendanceSync_MirrorCheckIn_AlreadyPresent_NoClobber(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Dbl", fmt.Sprintf("D-%d", time.Now().UnixNano()), "3a")

	firstCheckin := time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC)

	// Seed row as already-present with a stamped checked_in_at and a substatus
	// — exactly what a post-hoc staff edit would leave.
	excused := studentpresence.SessionAttendanceSubstatusExcused
	row := api.NewAttendanceMirrorParticipant()
	row.InstanceID = s.instance.ID
	row.StudentID = student.ID
	row.Status = studentpresence.SessionAttendancePresent
	row.Substatus = &excused
	row.CheckedInAt = &firstCheckin
	row.TenantID = testpkg.Tenant(t)
	require.NoError(t, s.Participants.Create(s.ctx, row))

	// Second check-in — later time. Mirror MUST NOT overwrite checked_in_at
	// or the substatus.
	snap, checkInErr7 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     time.Date(2026, 4, 21, 14, 30, 0, 0, time.UTC),
	})
	require.NoError(t, checkInErr7)
	require.NotNil(t, snap, "snapshot should still reflect current state for SSE")
	assert.Equal(t, studentpresence.SessionAttendancePresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, studentpresence.SessionAttendanceSubstatusExcused, *snap.Substatus)

	// DB: original first-checkin timestamp must survive.
	got, err := s.Participants.FindByID(s.ctx, row.ID)
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

	// Nil visit — B1
	checkInSnapshot8, checkInErr8 := s.syncer.MirrorCheckInForVisit(s.ctx, nil)
	require.NoError(t, checkInErr8)
	assert.Nil(t, checkInSnapshot8)

	// Zero ActiveGroupID — B1
	checkInSnapshot9, checkInErr9 := s.syncer.MirrorCheckInForVisit(s.ctx, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: 0,
	})
	require.NoError(t, checkInErr9)
	assert.Nil(t, checkInSnapshot9)
}

func TestAttendanceSync_MirrorCheckIn_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Ten", fmt.Sprintf("T-%d", time.Now().UnixNano()), "3a")
	api.SeedAttendanceMirrorParticipant(func(err error) { require.NoError(t, err) }, s.ctx, s.TimetableOwnerRows, s.instance.ID, student.ID, testpkg.Tenant(t), studentpresence.SessionAttendanceExpected)

	// Call with tenant 2 context — RLS should hide everything.
	ctxT2 := testpkg.TenantContext(2)
	snap, checkInErr10 := s.syncer.MirrorCheckInForVisit(ctxT2, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
		EntryTime:     time.Now(),
	})
	require.NoError(t, checkInErr10)
	assert.Nil(t, snap, "wrong tenant — instance invisible under RLS")
}

// --- MirrorCheckOutForVisit --------------------------------------------------

func TestAttendanceSync_LoadAttendance_ReturnsSnapshot(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-Load", fmt.Sprintf("L-%d", time.Now().UnixNano()), "3a")

	note := "bus verspätet"
	late := studentpresence.SessionAttendanceSubstatusLate
	row := api.NewAttendanceMirrorParticipant()
	row.InstanceID = s.instance.ID
	row.StudentID = student.ID
	row.Status = studentpresence.SessionAttendancePresent
	row.Substatus = &late
	row.Note = &note
	row.TenantID = testpkg.Tenant(t)
	require.NoError(t, s.Participants.Create(s.ctx, row))

	snap, err := s.syncer.MirrorCheckOutForVisit(s.ctx, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.Equal(t, studentpresence.SessionAttendancePresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, studentpresence.SessionAttendanceSubstatusLate, *snap.Substatus)
	require.NotNil(t, snap.Note)
	assert.Equal(t, "bus verspätet", *snap.Note)

	// Read-only: row must be unchanged (including updated_at).
	got, err := s.Participants.FindByID(s.ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, row.UpdatedAt.UnixNano(), got.UpdatedAt.UnixNano(), "LoadAttendance must not mutate")
}

func TestAttendanceSync_LoadAttendance_NoRow(t *testing.T) {
	t.Parallel()

	s := buildAttendanceSyncSetup(t)
	student := testpkg.CreateTestStudent(t, s.db, "AS-NoRow", fmt.Sprintf("N-%d", time.Now().UnixNano()), "3a")

	snap, err := s.syncer.MirrorCheckOutForVisit(s.ctx, &studentpresence.Visit{
		StudentID:     student.ID,
		ActiveGroupID: s.activeGroup.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, snap)
}
