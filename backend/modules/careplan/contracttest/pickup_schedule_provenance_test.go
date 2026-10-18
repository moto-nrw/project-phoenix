package contracttest_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	return calendar.NormalizeWallClock(parsed)
}

func newProjectionWriteService(
	t *testing.T,
	db *bun.DB,
	studentID int64,
	hhmm string,
) (careplan.PickupScheduleService, *api.RepositoryTestFactory) {
	t.Helper()
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	pickupStudents, err := api.PickupPlanStudents(api.MustNewPeopleDirectory(testpkg.MustDatabase(db)))
	require.NoError(t, err)
	service, err := careplanCompose.NewPickupSchedules(db, repos.CarePlan(), fixedPickupBaseline{StudentID: studentID, Weekday: timetable.WeekdayMonday, HHMM: hhmm}, nil, pickupStudents, nil)
	require.NoError(t, err)
	return service, repos
}

func TestUpsertBulkPickupSchedules_DoesNotMaterializeUnchangedProjection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Prova", "Nienz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Prove", "Nienz")
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	pickupStudents, err := api.PickupPlanStudents(api.MustNewPeopleDirectory(testpkg.MustDatabase(db)))
	require.NoError(t, err)
	service, err := careplanCompose.NewPickupSchedules(db, repos.CarePlan(), fixedPickupBaseline{StudentID: student.ID, Weekday: timetable.WeekdayMonday, HHMM: "14:30"}, nil, pickupStudents, nil)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Ganztag bis 14:30")
	legacy := &careplan.PickupSchedule{
		StudentID:      student.ID,
		Weekday:        timetable.WeekdayMonday,
		PickupTime:     wallClock(t, "14:30"),
		CreatedBy:      staff.ID,
		Source:         careplan.ScheduleSourceCareOffering,
		CareOfferingID: &offering.ID,
	}
	require.NoError(t, careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.UpsertSchedule(ctx, legacy))

	replacement := []*careplan.PickupSchedule{
		{Weekday: timetable.WeekdayMonday, PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID},
		{Weekday: timetable.WeekdayTuesday, PickupTime: wallClock(t, "13:00"), CreatedBy: staff.ID},
	}
	require.NoError(t, service.UpsertBulkStudentPickupSchedules(ctx, student.ID, replacement))

	rows, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, timetable.WeekdayTuesday, rows[0].Weekday)
	assert.Equal(t, careplan.ScheduleSourceStaff, rows[0].Source)
}

func TestUpsertBulkPickupSchedules_PreservesExplicitStaffOverride(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Manu", "Ell", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Manu", "Staff")
	service, repos := newProjectionWriteService(t, db, student.ID, "14:30")
	ctx := testpkg.Ctx(t)
	require.NoError(t, careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.UpsertSchedule(ctx, &careplan.PickupSchedule{
		StudentID: student.ID, Weekday: timetable.WeekdayMonday,
		PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID,
		Source: careplan.ScheduleSourceStaff,
	}))

	require.NoError(t, service.UpsertBulkStudentPickupSchedules(ctx, student.ID, []*careplan.PickupSchedule{
		{Weekday: timetable.WeekdayMonday, PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID},
	}))

	rows, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, careplan.ScheduleSourceStaff, rows[0].Source)
}

func TestUpsertBulkPickupSchedules_ChangedProjectionBecomesStaffOverride(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Flipa", "Nienz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Flipe", "Nienz")
	service, repos := newProjectionWriteService(t, db, student.ID, "14:30")
	ctx := testpkg.Ctx(t)

	require.NoError(t, service.UpsertBulkStudentPickupSchedules(ctx, student.ID, []*careplan.PickupSchedule{
		{Weekday: timetable.WeekdayMonday, PickupTime: wallClock(t, "15:15"), CreatedBy: staff.ID},
	}))

	rows, err := careplantest.NewScheduleRecords(repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, careplan.ScheduleSourceStaff, rows[0].Source)
	assert.Nil(t, rows[0].CareOfferingID)
}
