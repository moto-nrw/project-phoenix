package schedule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
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
// child, and the children's own times give way to it.
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

	t.Run("both children now inherit it", func(t *testing.T) {
		for _, studentID := range []int64{first.ID, second.ID} {
			stored, storeErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, studentID)
			require.NoError(t, storeErr)
			require.Len(t, stored, 1)
			assert.True(t, stored[0].InheritsClassTime())
		}
	})

	t.Run("the child whose own time was replaced is named", func(t *testing.T) {
		require.Len(t, result.OverwrittenStudents, 1)
		assert.Equal(t, first.ID, result.OverwrittenStudents[0].StudentID)
		assert.Equal(t, "12:15", result.OverwrittenStudents[0].PreviousTime)
		assert.Equal(t, "11:45", result.OverwrittenStudents[0].NewTime)
	})

	t.Run("the effective time both children read is the class time", func(t *testing.T) {
		rows, readErr := svc.GetStudentArrivalSchedules(ctx, second.ID)
		require.NoError(t, readErr)
		require.Len(t, rows, 1)
		assert.Equal(t, "11:45", rows[0].ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassSchedule, rows[0].Source)
		assert.Equal(t, "7c", rows[0].SourceClass)
	})
}
