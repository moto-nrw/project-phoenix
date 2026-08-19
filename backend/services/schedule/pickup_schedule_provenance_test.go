package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #2290: the weekly PUT replaces a student's pickup schedule wholesale. An
// untouched day must keep its Angebots-Gehzeit provenance — only a day whose
// time actually changed flips to staff ownership.

func wallClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return timezone.WallClock(parsed)
}

func TestUpsertBulkPickupSchedules_PreservesOfferingProvenance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	svc := scheduleService.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		repos.Student,
		repos.Person,
		nil,
		db,
		nil,
	)

	student := testpkg.CreateTestStudent(t, db, "Prova", "Nienz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Prove", "Nienz")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID)

	offeringID := int64(0)
	// A minimal care offering row to satisfy the FK on care_offering_id.
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Ganztag bis 14:30")
	offeringID = offering.ID

	sourced := &scheduleModels.StudentPickupSchedule{
		StudentID:      student.ID,
		Weekday:        scheduleModels.WeekdayMonday,
		PickupTime:     wallClock(t, "14:30"),
		CreatedBy:      staff.ID,
		Source:         scheduleModels.PickupScheduleSourceCareOffering,
		CareOfferingID: &offeringID,
	}
	require.NoError(t, repos.StudentPickupSchedule.UpsertSchedule(ctx, sourced))

	replacement := []*scheduleModels.StudentPickupSchedule{
		{StudentID: student.ID, Weekday: scheduleModels.WeekdayMonday, PickupTime: wallClock(t, "14:30"), CreatedBy: staff.ID},
		{StudentID: student.ID, Weekday: scheduleModels.WeekdayTuesday, PickupTime: wallClock(t, "13:00"), CreatedBy: staff.ID},
	}
	require.NoError(t, svc.UpsertBulkStudentPickupSchedules(ctx, student.ID, replacement))

	rows, err := repos.StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	byWeekday := map[int]*scheduleModels.StudentPickupSchedule{}
	for _, row := range rows {
		byWeekday[row.Weekday] = row
	}

	monday := byWeekday[scheduleModels.WeekdayMonday]
	require.NotNil(t, monday)
	assert.Equal(t, scheduleModels.PickupScheduleSourceCareOffering, monday.Source,
		"an unchanged day must keep its Angebots-Gehzeit provenance")
	require.NotNil(t, monday.CareOfferingID)
	assert.Equal(t, offeringID, *monday.CareOfferingID)

	tuesday := byWeekday[scheduleModels.WeekdayTuesday]
	require.NotNil(t, tuesday)
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, tuesday.Source)
}

func TestUpsertBulkPickupSchedules_ChangedTimeFlipsToStaff(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	svc := scheduleService.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		repos.Student,
		repos.Person,
		nil,
		db,
		nil,
	)

	student := testpkg.CreateTestStudent(t, db, "Flipa", "Nienz", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Flipe", "Nienz")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID)

	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Ganztag bis 14:30")
	offeringID := offering.ID

	sourced := &scheduleModels.StudentPickupSchedule{
		StudentID:      student.ID,
		Weekday:        scheduleModels.WeekdayMonday,
		PickupTime:     wallClock(t, "14:30"),
		CreatedBy:      staff.ID,
		Source:         scheduleModels.PickupScheduleSourceCareOffering,
		CareOfferingID: &offeringID,
	}
	require.NoError(t, repos.StudentPickupSchedule.UpsertSchedule(ctx, sourced))

	replacement := []*scheduleModels.StudentPickupSchedule{
		{StudentID: student.ID, Weekday: scheduleModels.WeekdayMonday, PickupTime: wallClock(t, "15:15"), CreatedBy: staff.ID},
	}
	require.NoError(t, svc.UpsertBulkStudentPickupSchedules(ctx, student.ID, replacement))

	rows, err := repos.StudentPickupSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, scheduleModels.PickupScheduleSourceStaff, rows[0].Source,
		"a manually changed time takes staff ownership")
	assert.Nil(t, rows[0].CareOfferingID)
}
