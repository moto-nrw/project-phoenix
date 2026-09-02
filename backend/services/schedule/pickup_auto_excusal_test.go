package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// autoExcusalHarness bundles everything the pickup auto-excusal tests need:
// a pickup service with the syncer wired, the raw repos for assertions, and
// a future Monday fixture day with three care blocks.
type autoExcusalHarness struct {
	db      *bun.DB
	ctx     context.Context
	svc     scheduleService.PickupScheduleService
	partial scheduleService.PartialAbsenceService

	student *fixtureStudent
	staffID int64

	date timezone.Date
	// before 13:00-14:00, overlap 14:00-15:00, after 15:00-16:00
	beforeRow, overlapRow, afterRow int64
}

type fixtureStudent struct {
	ID int64
}

func wallClockAt(h, m int) *time.Time {
	t := time.Date(2000, 1, 1, h, m, 0, 0, time.UTC)
	return &t
}

func setupAutoExcusalHarness(t *testing.T, withBaseline bool) *autoExcusalHarness {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	lock, notFound, err := repositories.NewCareStudentLock(db)
	require.NoError(t, err)
	scheduleService.BindCareStudentLockForDB(db, lock, notFound)
	repos := repositories.NewFactory(db)

	syncer := scheduleService.NewPickupAutoExcusalSyncer(
		repos.StudentPickupException,
		scheduletest.NewPickupBaselineService(repos.StudentPickupSchedule, repos.RequestChildOffering, repos.CareOffering),
		repos.InstanceStudent,
		db,
	)
	svc := scheduleService.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		repos.Student,
		repos.Person,
		syncer,
		scheduletest.NewPickupBaselineService(repos.StudentPickupSchedule, repos.RequestChildOffering, repos.CareOffering),
		db,
		nil,
	)
	partial := scheduleService.NewPartialAbsenceService(
		repos.StudentPickupException,
		repos.StudentStatusDay,
		repos.ExcusedAbsenceRequest,
		repos.InstanceStudent,
		syncer,
		db,
	)

	student := testpkg.CreateTestStudent(t, db, "Auto", "Excusal", "AE1")
	staff := testpkg.CreateTestStaff(t, db, "Auto", "Staff")
	room := testpkg.CreateTestRoom(t, db, "Auto excusal room")

	// A fixed far-future Monday keeps the exception inside
	// FindUpcomingByStudentID (today onwards) without consulting the live clock.
	date := timezone.NewDate(2099, 1, 1)
	for date.Weekday() != time.Monday {
		date = date.AddDays(1)
	}
	require.Equal(t, time.Monday, date.Weekday(), "fixture date must be a Monday")

	if withBaseline {
		testpkg.CreateTestPickupSchedule(t, db, student.ID, 1, staff.ID, "16:00")
	}

	before := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "Lernzeit",
	})
	overlap := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Freispiel",
	})
	after := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "AG",
	})
	beforeRow := testpkg.CreateTestInstanceStudent(t, db, before.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	overlapRow := testpkg.CreateTestInstanceStudent(t, db, overlap.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	afterRow := testpkg.CreateTestInstanceStudent(t, db, after.ID, student.ID, scheduleModel.AttendanceStatusExpected)

	return &autoExcusalHarness{
		db:         db,
		ctx:        testpkg.Ctx(t),
		svc:        svc,
		partial:    partial,
		student:    &fixtureStudent{ID: student.ID},
		staffID:    staff.ID,
		date:       date,
		beforeRow:  beforeRow.ID,
		overlapRow: overlapRow.ID,
		afterRow:   afterRow.ID,
	}
}

func (h *autoExcusalHarness) attendance(t *testing.T, rowID int64) *scheduleModel.InstanceStudent {
	t.Helper()
	row := new(scheduleModel.InstanceStudent)
	err := h.db.NewSelect().
		Model(row).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".id = ?`, rowID).
		Scan(context.Background())
	require.NoError(t, err)
	return row
}

func (h *autoExcusalHarness) resolveStaff() (int64, error) { return h.staffID, nil }

func TestAutoExcusal_PulledForwardPickupExcusesLaterBlocks(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.True(t, row.ExcusedAuto, "pull-forward must derive an auto excusal")
	require.NotNil(t, row.ExcusedFrom)
	assert.Equal(t, "14:45", timezone.NormalizeWallClock(*row.ExcusedFrom).Format("15:04"))
	assert.Nil(t, row.ExcusedCreatedBy, "auto excusals carry no staff author")

	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.beforeRow).Status)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.overlapRow).Status,
		"a block the pickup time falls INTO stays expected (the child attends its beginning)")
	after := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, after.Status)
	require.NotNil(t, after.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, *after.Substatus)
	require.NotNil(t, after.PickupExceptionID)
	assert.Equal(t, row.ID, *after.PickupExceptionID)
}

func TestAutoExcusal_MovingPickupBackReleasesBlocks(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	updated, err := h.svc.UpdateException(h.ctx, row.ID, h.student.ID, h.date, nil, wallClockAt(16, 30), false, h.resolveStaff)
	require.NoError(t, err)
	assert.False(t, updated.ExcusedAuto, "a later-than-baseline time is no pull-forward")
	assert.Nil(t, updated.ExcusedFrom)

	after := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, after.Status, "released block returns to expected")
	assert.Nil(t, after.PickupExceptionID)
	assert.Nil(t, after.Substatus)
}

func TestAutoExcusal_MovingPickupEarlierWidensTheExcusal(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)

	updated, err := h.svc.UpdateException(h.ctx, row.ID, h.student.ID, h.date, nil, wallClockAt(14, 0), false, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, updated.ExcusedAuto)
	assert.Equal(t, "14:00", timezone.NormalizeWallClock(*updated.ExcusedFrom).Format("15:04"))

	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.beforeRow).Status)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.overlapRow).Status,
		"the 14:00 block now starts AT the cutoff and is excused")
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.afterRow).Status)
}

func TestAutoExcusal_DeletingTheExceptionRestoresBlocks(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.afterRow).Status)

	require.NoError(t, h.svc.DeleteStudentPickupException(h.ctx, row.ID))

	after := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, after.Status)
	assert.Nil(t, after.PickupExceptionID)

	repos := repositories.NewFactory(h.db)
	gone, err := repos.StudentPickupException.FindByStudentIDAndDate(h.ctx, h.student.ID, h.date)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestAutoExcusal_NoWeeklyBaselineMeansNoCoupling(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, false)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	assert.False(t, row.ExcusedAuto, "without a weekly baseline there is no Vorverlegung")
	assert.Nil(t, row.ExcusedFrom)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.afterRow).Status)
}

func TestAutoExcusal_LaterThanBaselineMeansNoCoupling(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(17, 0), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	assert.False(t, row.ExcusedAuto)
	assert.Nil(t, row.ExcusedFrom)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.afterRow).Status)
}

func TestAutoExcusal_ManualPartialAbsenceIsNeverTouched(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	// A manual partial absence from 13:30 (staff decision, own dialog).
	manual, err := h.partial.Create(h.ctx, scheduleService.PartialAbsenceInput{
		StudentID: h.student.ID,
		Date:      h.date,
		FromTime:  *wallClockAt(13, 30),
		Reason:    "Arzttermin",
		StaffID:   h.staffID,
	})
	require.NoError(t, err)
	require.False(t, manual.ExcusedAuto)

	// Editing the day's pickup time must keep the manual cutoff untouched.
	updated, err := h.svc.UpdateException(h.ctx, manual.ID, h.student.ID, h.date, nil, wallClockAt(15, 30), false, h.resolveStaff)
	require.NoError(t, err)
	require.NotNil(t, updated.ExcusedFrom)
	assert.Equal(t, "13:30", timezone.NormalizeWallClock(*updated.ExcusedFrom).Format("15:04"))
	assert.False(t, updated.ExcusedAuto)

	// Blocks follow the manual 13:30 cutoff, not the 15:30 pickup time.
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.overlapRow).Status)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.afterRow).Status)
}

func TestAutoExcusal_ManualCreateConvertsAutoToManual(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	manual, err := h.partial.Create(h.ctx, scheduleService.PartialAbsenceInput{
		StudentID: h.student.ID,
		Date:      h.date,
		FromTime:  *wallClockAt(13, 30),
		Reason:    "Früher weg",
		StaffID:   h.staffID,
	})
	require.NoError(t, err)
	assert.False(t, manual.ExcusedAuto)
	require.NotNil(t, manual.ExcusedCreatedBy)
	assert.Equal(t, h.staffID, *manual.ExcusedCreatedBy)
	assert.Equal(t, "13:30", timezone.NormalizeWallClock(*manual.ExcusedFrom).Format("15:04"))

	// 14:00 block starts after the manual 13:30 cutoff → excused now.
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.overlapRow).Status)
}

func TestAutoExcusal_ManualDeleteRefusesAutoRows(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	err = h.partial.Delete(h.ctx, row.ID, h.student.ID)
	require.ErrorIs(t, err, scheduleService.ErrPartialAbsenceAutoManaged)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.afterRow).Status,
		"refused delete must not release the blocks")
}

func TestAutoExcusal_ManualDeleteOfConvertedRowRederivesAuto(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	// 14:45 pickup pulls forward against the 16:00 baseline → auto excusal.
	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	// Staff overrides with an earlier manual cutoff — converts auto to manual.
	manual, err := h.partial.Create(h.ctx, scheduleService.PartialAbsenceInput{
		StudentID: h.student.ID,
		Date:      h.date,
		FromTime:  *wallClockAt(13, 30),
		Reason:    "Früher weg",
		StaffID:   h.staffID,
	})
	require.NoError(t, err)
	require.False(t, manual.ExcusedAuto)
	require.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.overlapRow).Status)

	// Deleting the manual override leaves the 14:45 pickup time in place — it
	// still qualifies as a pull-forward, so the auto excusal must come back.
	require.NoError(t, h.partial.Delete(h.ctx, manual.ID, h.student.ID))

	fresh := h.exception(t)
	require.NotNil(t, fresh)
	assert.True(t, fresh.ExcusedAuto, "unchanged early pickup time must re-derive the auto excusal")
	require.NotNil(t, fresh.ExcusedFrom)
	assert.Equal(t, "14:45", timezone.NormalizeWallClock(*fresh.ExcusedFrom).Format("15:04"))
	assert.Nil(t, fresh.ExcusedCreatedBy)

	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.overlapRow).Status,
		"the manual 13:30 cutoff is gone — the 14:00 block returns to expected")
	after := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, after.Status)
	require.NotNil(t, after.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, *after.Substatus)
	require.NotNil(t, after.PickupExceptionID)
	assert.Equal(t, fresh.ID, *after.PickupExceptionID)
}

func (h *autoExcusalHarness) exception(t *testing.T) *scheduleModel.StudentPickupException {
	t.Helper()
	row, err := repositories.NewFactory(h.db).StudentPickupException.FindByStudentIDAndDate(h.ctx, h.student.ID, h.date)
	require.NoError(t, err)
	return row
}

func TestAutoExcusal_WeeklyBaselineMovedEarlierReleasesCoupling(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)
	require.Equal(t, scheduleModel.AttendanceStatusAbsent, h.attendance(t, h.afterRow).Status)

	// Monday baseline moves to 14:00 — a 14:45 pickup is no pull-forward
	// anymore, so the weekly write must release the coupling.
	err = h.svc.UpsertStudentPickupSchedule(h.ctx, &scheduleModel.StudentPickupSchedule{
		StudentID:  h.student.ID,
		Weekday:    scheduleModel.WeekdayMonday,
		PickupTime: *wallClockAt(14, 0),
		CreatedBy:  h.staffID,
	})
	require.NoError(t, err)

	fresh := h.exception(t)
	require.NotNil(t, fresh)
	assert.False(t, fresh.ExcusedAuto)
	assert.Nil(t, fresh.ExcusedFrom)
	after := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, after.Status)
	assert.Nil(t, after.PickupExceptionID)
}

func TestAutoExcusal_WeeklyBaselineDeletedReleasesCoupling(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	// Without a weekly baseline there is no Vorverlegung — deleting the plan
	// must release the derived absences.
	require.NoError(t, h.svc.DeleteAllStudentPickupSchedules(h.ctx, h.student.ID))

	fresh := h.exception(t)
	require.NotNil(t, fresh)
	assert.False(t, fresh.ExcusedAuto)
	assert.Nil(t, fresh.ExcusedFrom)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.afterRow).Status)
}

func TestAutoExcusal_WeeklyBaselineAddedCouplesExistingException(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, false)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.False(t, row.ExcusedAuto, "without a baseline the exception starts uncoupled")

	// A baseline appearing AFTER the exception makes 14:45 a pull-forward —
	// the weekly write must derive the coupling for the stored exception.
	err = h.svc.UpsertStudentPickupSchedule(h.ctx, &scheduleModel.StudentPickupSchedule{
		StudentID:  h.student.ID,
		Weekday:    scheduleModel.WeekdayMonday,
		PickupTime: *wallClockAt(16, 0),
		CreatedBy:  h.staffID,
	})
	require.NoError(t, err)

	fresh := h.exception(t)
	require.NotNil(t, fresh)
	assert.True(t, fresh.ExcusedAuto)
	require.NotNil(t, fresh.ExcusedFrom)
	assert.Equal(t, "14:45", timezone.NormalizeWallClock(*fresh.ExcusedFrom).Format("15:04"))
	after := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, after.Status)
	require.NotNil(t, after.PickupExceptionID)
	assert.Equal(t, fresh.ID, *after.PickupExceptionID)
}

func TestAutoExcusal_BulkWeeklyUpsertResyncsExceptions(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	// The multi-student bulk editor moves Monday to 13:30 — the stored 14:45
	// exception is later than the new baseline and must decouple.
	_, err = h.svc.BulkUpsertPickupSchedules(
		h.ctx,
		scheduleService.ArrivalScheduleBulkFilter{StudentIDs: []int64{h.student.ID}},
		[]scheduleService.PickupScheduleInput{{Weekday: scheduleModel.WeekdayMonday, PickupTime: "13:30"}},
		h.staffID,
	)
	require.NoError(t, err)

	fresh := h.exception(t)
	require.NotNil(t, fresh)
	assert.False(t, fresh.ExcusedAuto)
	assert.Nil(t, fresh.ExcusedFrom)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.afterRow).Status)
}

func TestAutoExcusal_FullDayStatusCoexistsAndReleaseReplays(t *testing.T) {
	t.Parallel()

	h := setupAutoExcusalHarness(t, true)
	repos := repositories.NewFactory(h.db)

	row, err := h.svc.CreateOrReclaimException(h.ctx, h.student.ID, h.date, wallClockAt(14, 45), nil, h.staffID, h.resolveStaff)
	require.NoError(t, err)
	require.True(t, row.ExcusedAuto)

	statusDay := testpkg.CreateTestStudentStatusDay(t, h.db, h.student.ID, h.date, "sick")
	// Project the sick day onto the slots the way the production repo does.
	_, err = repos.InstanceStudent.ApplyStatusDay(h.ctx, h.student.ID, h.date, statusDay.ID, scheduleModel.AttendanceSubstatusSick)
	require.NoError(t, err)

	before := h.attendance(t, h.beforeRow)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, before.Status)
	require.NotNil(t, before.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusSick, *before.Substatus, "sick owns the pre-pickup blocks")
	after := h.attendance(t, h.afterRow)
	require.NotNil(t, after.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, *after.Substatus, "the auto excusal keeps its blocks")

	// Clearing the sick day (production path: MarkClearedByID sets cleared_at,
	// then releases the slots) restores its rows — and must replay the still
	// active pickup cutoff instead of leaving the late blocks expected.
	err = repos.StudentStatusDay.MarkClearedByID(h.ctx, statusDay.ID, time.Now(), activeModel.StudentStatusSourceManual)
	require.NoError(t, err)

	assert.Equal(t, scheduleModel.AttendanceStatusExpected, h.attendance(t, h.beforeRow).Status)
	restored := h.attendance(t, h.afterRow)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, restored.Status)
	require.NotNil(t, restored.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, *restored.Substatus)
	require.NotNil(t, restored.PickupExceptionID)
	assert.Equal(t, row.ID, *restored.PickupExceptionID)
}
