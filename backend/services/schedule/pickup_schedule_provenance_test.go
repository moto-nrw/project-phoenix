package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func wallClock(t *testing.T, hhmm string) time.Time {
	if t != nil {
		t.Helper()
	}
	parsed, err := time.Parse("15:04", hhmm)
	if t != nil {
		require.NoError(t, err)
	} else if err != nil {
		panic(err)
	}
	return timezone.WallClock(parsed)
}

func newProjectionWriteService(
	t *testing.T,
	db *bun.DB,
	studentID int64,
	hhmm string,
) (scheduleService.PickupScheduleService, *repositories.Factory) {
	t.Helper()
	repos := repositories.NewFactory(db)
	service := scheduleService.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		repos.Student,
		repos.Person,
		nil,
		scheduletest.FixedPickupBaseline{StudentID: studentID, Weekday: scheduleModels.WeekdayMonday, HHMM: hhmm},
		db,
		nil,
	)
	return service, repos
}

func TestUpsertBulkPickupSchedules_DoesNotMaterializeUnchangedProjection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Prova", "Nienz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Prove", "Nienz")
	repos := repositories.NewFactory(db)
	service := scheduleService.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		repos.Student,
		repos.Person,
		nil,
		scheduletest.FixedPickupBaseline{StudentID: student.ID, Weekday: scheduleModels.WeekdayMonday, HHMM: "14:30"},
		db,
		nil,
	)
	ctx := testpkg.Ctx(t)

	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Ganztag bis 14:30")
	legacy := &scheduleModels.StudentPickupSchedule{
		StudentID:      student.ID,
		Weekday:        scheduleModels.WeekdayMonday,
		PickupTime:     wallClock(t, "14:30"),
		CreatedBy:      staff.ID,
		Source:         scheduleModels.PickupScheduleSourceCareOffering,
		CareOfferingID: &offering.ID,
	}
	require.NoError(t, repos.StudentPickupSchedule.UpsertSchedule(ctx, legacy))

	replacement := []*scheduleModels.StudentPickupSchedule{
		{Weekday: scheduleModels.WeekdayMonday, PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID},
		{Weekday: scheduleModels.WeekdayTuesday, PickupTime: wallClock(t, "13:00"), CreatedBy: staff.ID},
	}
	require.NoError(t, service.UpsertBulkStudentPickupSchedules(ctx, student.ID, replacement))

	rows, err := repos.StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, scheduleModels.WeekdayTuesday, rows[0].Weekday)
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, rows[0].Source)
}

func TestUpsertBulkPickupSchedules_PreservesExplicitStaffOverride(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Manu", "Ell", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Manu", "Staff")
	service, repos := newProjectionWriteService(t, db, student.ID, "14:30")
	ctx := testpkg.Ctx(t)
	require.NoError(t, repos.StudentPickupSchedule.UpsertSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID: student.ID, Weekday: scheduleModels.WeekdayMonday,
		PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID,
		Source: scheduleModels.PickupScheduleSourceStaff,
	}))

	require.NoError(t, service.UpsertBulkStudentPickupSchedules(ctx, student.ID, []*scheduleModels.StudentPickupSchedule{
		{Weekday: scheduleModels.WeekdayMonday, PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID},
	}))

	rows, err := repos.StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, rows[0].Source)
}

func TestUpsertBulkPickupSchedules_ChangedProjectionBecomesStaffOverride(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Flipa", "Nienz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Flipe", "Nienz")
	service, repos := newProjectionWriteService(t, db, student.ID, "14:30")
	ctx := testpkg.Ctx(t)

	require.NoError(t, service.UpsertBulkStudentPickupSchedules(ctx, student.ID, []*scheduleModels.StudentPickupSchedule{
		{Weekday: scheduleModels.WeekdayMonday, PickupTime: wallClock(t, "15:15"), CreatedBy: staff.ID},
	}))

	rows, err := repos.StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, rows[0].Source)
	assert.Nil(t, rows[0].CareOfferingID)
}
