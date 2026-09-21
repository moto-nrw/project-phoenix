package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type arrivalClassRules struct {
	times    map[int]time.Time
	lockErr  error
	classErr error
}

func (r *arrivalClassRules) LockStudent(context.Context, int64) error { return r.lockErr }
func (r *arrivalClassRules) ClassTimesForStudent(context.Context, int64) (map[int]time.Time, error) {
	return r.times, r.classErr
}

func TestNativeArrivalWeekPreservesInheritanceAndRollsBackFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db)
	student := testpkg.CreateTestStudent(t, db, "Arrival", "Native", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Arrival", "Staff")
	clock := time.Date(1, 1, 1, 13, 30, 0, 0, time.UTC)
	rules := &arrivalClassRules{times: map[int]time.Time{1: clock}}
	service, err := compose.NewArrivalSchedules(db, owner, nil, rules, compose.ArrivalScheduleDependencies{})
	require.NoError(t, err)
	rows := []*careplan.ArrivalSchedule{{StudentID: student.ID, Weekday: 1, ExpectedArrival: clock, CreatedBy: staff.ID},
		{StudentID: student.ID, Weekday: 2, ExpectedArrival: clock, CreatedBy: staff.ID}}
	require.NoError(t, service.UpsertBulkStudentArrivalSchedules(ctx, student.ID, rows))
	require.True(t, rows[0].InheritsClassTime(), "matching the class time must not become a permanent deviation")
	stored, err := owner.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, stored, 2)
	byDay := make(map[int]careplan.ArrivalSchedule)
	for _, row := range stored {
		byDay[row.Weekday] = row
	}
	require.True(t, byDay[1].ExpectedArrival.IsZero())
	require.Equal(t, "13:30", byDay[2].ExpectedArrival.Format("15:04"))

	failure := errors.New("class times unavailable")
	rules.classErr = failure
	err = service.UpsertBulkStudentArrivalSchedules(ctx, student.ID, rows[:1])
	require.ErrorIs(t, err, failure)
	rules.classErr = nil
	rules.lockErr = failure
	require.ErrorIs(t, service.DeleteAllStudentArrivalSchedules(ctx, student.ID), failure)
	stored, err = owner.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, stored, 2, "failed writes must preserve the whole week")
}

func TestNativeArrivalExceptionAndNoteUseOwnerRecords(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db)
	student := testpkg.CreateTestStudent(t, db, "Arrival", "Exception", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Arrival", "Author")
	service, err := compose.NewArrivalSchedules(db, owner, nil, nil, compose.ArrivalScheduleDependencies{})
	require.NoError(t, err)
	date := calendar.TodayDate().AddDays(7)
	clock := time.Date(1, 1, 1, 12, 45, 0, 0, time.UTC)
	row, err := service.CreateOrReclaimException(ctx, student.ID, date, &clock, nil, staff.ID, nil)
	require.NoError(t, err)
	require.Positive(t, row.ID)
	require.Equal(t, testpkg.Tenant(t), row.TenantID)
	_, err = service.CreateOrReclaimException(ctx, student.ID, date, &clock, nil, staff.ID, nil)
	require.ErrorIs(t, err, careplan.ErrCareExceptionDayConflict)
	note := &careplan.ArrivalNote{StudentID: student.ID, NoteDate: careplan.Date(date), Content: "Read this note", CreatedBy: staff.ID}
	require.NoError(t, service.CreateStudentArrivalNote(ctx, note))
	data, err := service.GetStudentArrivalDataForDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.Len(t, data.Exceptions, 1)
	require.Len(t, data.Notes, 1)
	require.Equal(t, row.ID, data.Exceptions[0].ID)
	require.Equal(t, note.ID, data.Notes[0].ID)
	other := testpkg.CreateTestStudent(t, db, "Other", "Arrival", "1a")
	require.ErrorIs(t, service.DeleteStudentArrivalException(ctx, row.ID, other.ID), careplan.ErrCareExceptionWrongStudent)
	preserved, err := service.GetStudentArrivalExceptionByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, student.ID, preserved.StudentID)
	require.NoError(t, service.DeleteStudentArrivalException(ctx, row.ID, row.StudentID))
	require.NoError(t, service.DeleteStudentArrivalNote(ctx, note.ID))
}
