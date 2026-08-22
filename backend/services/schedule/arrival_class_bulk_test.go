package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func arrivalServiceWithClassTimes(t *testing.T, repos *repositories.Factory) scheduleService.ArrivalScheduleService {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		classArrivalBaseline(t, repos),
		repos.ClassArrivalTime,
		db,
		nil,
	)
}

// TestBulkUpsertBySchoolClassWritesTheClassTimetable pins the pflege win of
// #2414: setting the time for a class writes one class row, not one row per
// child. A child's own time remains the higher-priority deviation (ADR 0005).
func TestBulkUpsertBySchoolClassWritesTheClassTimetable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassTimes(t, repos)

	staff := testpkg.CreateTestStaff(t, db, "Sammel", "Betreuung")
	first := testpkg.CreateTestStudent(t, db, "Eins", "Kind", "7c")
	second := testpkg.CreateTestStudent(t, db, "Zwei", "Kind", "7c")
	testpkg.CreateTestArrivalSchedule(t, db, first.ID, scheduleModel.WeekdayMonday, staff.ID, "12:15")
	testpkg.CreateTestArrivalSchedule(t, db, second.ID, scheduleModel.WeekdayMonday, staff.ID, "")

	result, err := svc.BulkUpsertArrivalSchedules(ctx,
		scheduleService.ArrivalScheduleBulkFilter{SchoolClass: "7c"},
		[]scheduleService.ArrivalScheduleInput{{Weekday: scheduleModel.WeekdayMonday, ArrivalTime: "11:45"}},
		staff.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsAffected)

	t.Run("the class carries the time exactly once", func(t *testing.T) {
		rows, findErr := repos.ClassArrivalTime.FindByClasses(ctx, []string{"7c"})
		require.NoError(t, findErr)
		require.Len(t, rows, 1)
		assert.Equal(t, "11:45", rows[0].ArrivalTimes["mon"])
	})

	t.Run("an own time remains while an inherited row follows the class", func(t *testing.T) {
		firstRows, storeErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, first.ID)
		require.NoError(t, storeErr)
		require.Len(t, firstRows, 1)
		assert.Equal(t, "12:15", firstRows[0].ExpectedArrival.Format("15:04"))

		secondRows, storeErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, second.ID)
		require.NoError(t, storeErr)
		require.Len(t, secondRows, 1)
		assert.True(t, secondRows[0].InheritsClassTime())
	})

	t.Run("no child is reported as overwritten", func(t *testing.T) {
		assert.Empty(t, result.OverwrittenStudents)
	})

	t.Run("the effective times respect the priority order", func(t *testing.T) {
		firstWeek, readErr := svc.GetStudentArrivalSchedules(ctx, first.ID)
		require.NoError(t, readErr)
		require.Len(t, firstWeek, 1)
		assert.Equal(t, "12:15", firstWeek[0].ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceStaff, firstWeek[0].Source)

		rows, readErr := svc.GetStudentArrivalSchedules(ctx, second.ID)
		require.NoError(t, readErr)
		require.Len(t, rows, 1)
		assert.Equal(t, "11:45", rows[0].ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassSchedule, rows[0].Source)
		assert.Equal(t, "7c", rows[0].SourceClass)
	})
}

type failingClassArrivalTimeRepository struct {
	educationModel.ClassArrivalTimeRepository
	err error
}

type bookingMondayBaseline struct{}

func (bookingMondayBaseline) Project(
	_ context.Context,
	studentIDs []int64,
	from timezone.Date,
	_ timezone.Date,
) (*scheduleService.ArrivalBaselineProjection, error) {
	projection := &scheduleService.ArrivalBaselineProjection{
		BookingsAuthoritative: true,
		WeeklyByStudentDate:   make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
		DerivedByStudentDate:  make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
	}
	for _, studentID := range studentIDs {
		projection.WeeklyByStudentDate[studentID] = scheduleService.ArrivalPlanByDate{
			from: scheduleService.ArrivalWeek{
				scheduleModel.WeekdayMonday: {
					StudentID: studentID,
					Weekday:   scheduleModel.WeekdayMonday,
				},
			},
		}
	}
	return projection, nil
}

func (r failingClassArrivalTimeRepository) FindByClasses(context.Context, []string) ([]*educationModel.ClassArrivalTime, error) {
	return nil, r.err
}

func TestWeeklyWriteFailsWhenClassTimeCannotBeChecked(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("class-time lookup failed")
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		classArrivalBaseline(t, repos),
		failingClassArrivalTimeRepository{ClassArrivalTimeRepository: repos.ClassArrivalTime, err: wantErr},
		db,
		nil,
	)

	staff := testpkg.CreateTestStaff(t, db, "Fehler", "Pruefung")
	student := testpkg.CreateTestStudent(t, db, "Fehler", "Kind", "8e")
	err := svc.UpsertBulkStudentArrivalSchedules(ctx, student.ID, []*scheduleModel.StudentArrivalSchedule{
		{
			StudentID:       student.ID,
			Weekday:         scheduleModel.WeekdayMonday,
			ExpectedArrival: timezone.WallClock(mustParseHHMM(t, "11:45")),
			CreatedBy:       staff.ID,
		},
	})

	require.ErrorIs(t, err, wantErr)
	stored, findErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, findErr)
	assert.Empty(t, stored, "a failed class-time check must not persist a false deviation")
}

func TestBookingModeWeeklyWritePreservesInactiveRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		bookingMondayBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)

	staff := testpkg.CreateTestStaff(t, db, "Buchung", "Bewahren")
	student := testpkg.CreateTestStudent(t, db, "Buchung", "Kind", "8f")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "12:15")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayTuesday, staff.ID, "13:00")

	require.NoError(t, svc.UpsertBulkStudentArrivalSchedules(ctx, student.ID, nil))

	stored, err := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, scheduleModel.WeekdayTuesday, stored[0].Weekday)
	assert.Equal(t, "13:00", stored[0].ExpectedArrival.Format("15:04"))
}

// TestWeeklyWriteCollapsesIntoTheClassTime pins the ADR 0005 promise: a value
// identical to the class time is not stored as a deviation, so the child keeps
// following the class when the Unterrichtsschluss moves.
func TestWeeklyWriteCollapsesIntoTheClassTime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassTimes(t, repos)

	staff := testpkg.CreateTestStaff(t, db, "Deckungs", "Gleich")
	student := testpkg.CreateTestStudent(t, db, "Deckungs", "Kind", "8d")
	setClassArrivalTimes(t, repos, "8d", map[string]string{"mon": "11:45", "tue": "11:45"})

	sameAsClass := timezone.WallClock(mustParseHHMM(t, "11:45"))
	deviating := timezone.WallClock(mustParseHHMM(t, "12:15"))
	require.NoError(t, svc.UpsertBulkStudentArrivalSchedules(ctx, student.ID,
		[]*scheduleModel.StudentArrivalSchedule{
			{StudentID: student.ID, Weekday: scheduleModel.WeekdayMonday, ExpectedArrival: sameAsClass, CreatedBy: staff.ID},
			{StudentID: student.ID, Weekday: scheduleModel.WeekdayTuesday, ExpectedArrival: deviating, CreatedBy: staff.ID},
		}))

	stored, err := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	byWeekday := map[int]*scheduleModel.StudentArrivalSchedule{}
	for _, row := range stored {
		byWeekday[row.Weekday] = row
	}
	assert.True(t, byWeekday[scheduleModel.WeekdayMonday].InheritsClassTime(),
		"a value identical to the class time is no deviation")
	assert.False(t, byWeekday[scheduleModel.WeekdayTuesday].InheritsClassTime())
	assert.Equal(t, "12:15", byWeekday[scheduleModel.WeekdayTuesday].ExpectedArrival.Format("15:04"))
}

func mustParseHHMM(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return parsed
}
