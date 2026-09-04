package enrollment_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// timetableDataWithArrivalBaseline wires the read facade behind api/timetable
// the way the factory does, with the arrival projection in booking mode
// (#2414). Only the repositories the arrival read paths touch are filled — the
// facade is a read boundary here, not a write one.
func timetableDataWithArrivalBaseline(
	t *testing.T,
	env *decisionTestEnv,
	authoritative bool,
) *scheduleService.TimetableDataService {
	t.Helper()
	return scheduleService.NewTimetableDataService(scheduleService.TimetableDataDependencies{
		InstanceStudentRepo:   scheduleRepo.NewInstanceStudentRepository(env.db),
		ActivityInstanceRepo:  scheduleRepo.NewActivityInstanceRepository(env.db),
		ActivityExceptionRepo: scheduleRepo.NewActivityExceptionRepository(env.db),
		ActivityScheduleRepo:  env.repos.ActivitySchedule,
		ArrivalScheduleRepo:   env.repos.StudentArrivalSchedule,
		ArrivalBaselines:      bookingModeArrivalBaseline(t, env, authoritative),
		ArrivalExceptionRepo:  env.repos.StudentArrivalException,
		PickupScheduleRepo:    env.repos.StudentPickupSchedule,
		PickupBaselines: scheduletest.NewPickupBaselineService(
			env.repos.StudentPickupSchedule,
			env.repos.RequestChildOffering,
			env.repos.CareOffering,
		),
		PickupExceptionRepo: env.repos.StudentPickupException,
		VisitRepo:           activeRepo.NewVisitRepository(env.db),
		EducationGroupRepo:  educationRepo.NewGroupRepository(env.db),
		Logger:              slog.Default(),
		DB:                  env.db,
	})
}

// TestTimetableRead_StudentWeekAppliesTheBookingMode closes the last read gap
// of the 19.08. incident on the per-student day/week surface: the stale row on
// an unbooked weekday must plan nothing there either, exactly like the
// care-day derivation.
func TestTimetableRead_StudentWeekAppliesTheBookingMode(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "wochenansicht", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "wochenansicht@example.com", "Wanda", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "thu": "11:45"})

	// The row on Thursday survived the Abmeldung, the booking covers Monday.
	staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Wochenansicht")
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayThursday, staff.ID, "11:45")

	monday := nextWeekday(decisionTestToday, time.Monday)
	thursday := monday.AddDays(3)

	t.Run("with the booking mode on only the booked weekday plans", func(t *testing.T) {
		data := timetableDataWithArrivalBaseline(t, env, true)
		pre, err := data.PreloadStudentWeek(ctx, studentID, monday, thursday)
		require.NoError(t, err)

		booked := pre.ArrivalSchedByDate[monday.String()]
		require.NotNil(t, booked, "the booked weekday keeps its arrival")
		assert.Equal(t, "11:45", booked.ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModels.ArrivalScheduleSourceClassSchedule, booked.Source)

		assert.Nil(t, pre.ArrivalSchedByDate[thursday.String()],
			"a stale row on an unbooked weekday must not plan anything")
	})

	t.Run("with the booking mode off the stored row still plans", func(t *testing.T) {
		data := timetableDataWithArrivalBaseline(t, env, false)
		pre, err := data.PreloadStudentWeek(ctx, studentID, monday, thursday)
		require.NoError(t, err)

		require.NotNil(t, pre.ArrivalSchedByDate[thursday.String()],
			"the six schools without a Halbjahresanmeldung must not change")
		assert.Nil(t, pre.ArrivalSchedByDate[monday.String()],
			"without the booking mode a weekday without a stored row is not a care day")
	})
}

// TestTimetableRead_StudentWeekCareDayWithoutClassTime pins that a care day
// whose class carries no time keeps the day and stays timeless. Copying the
// zero value into a time would render as 00:00 (#2414).
func TestTimetableRead_StudentWeekCareDayWithoutClassTime(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ohne-klassenzeit", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "ohne-klassenzeit@example.com", "Ohne", 2,
	)
	setStudentClass(t, env, studentID, "4c")
	setArrivalClassTimes(t, env, "4c", map[string]string{"tue": "12:45"})

	monday := nextWeekday(decisionTestToday, time.Monday)
	data := timetableDataWithArrivalBaseline(t, env, true)
	pre, err := data.PreloadStudentWeek(ctx, studentID, monday, monday)
	require.NoError(t, err)

	arrival := pre.ArrivalSchedByDate[monday.String()]
	require.NotNil(t, arrival, "the booked day must remain visible without a class time")
	assert.True(t, arrival.ExpectedArrival.IsZero(), "the visible care day has no arrival time to show")

	room := testpkg.CreateTestRoom(t, env.db, "Ohne-Zeit-Raum")
	activity := testpkg.CreateTestActivityGroup(t, env.db, "Ohne-Zeit-AG")
	instance := testpkg.CreateTestActivityInstance(t, env.db, monday, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &activity.ID,
	})
	testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, scheduleModels.AttendanceStatusExpected)
	reason := "Fällt aus"
	exception := &scheduleModels.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   monday,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, exception.Validate())
	require.NoError(t, scheduleRepo.NewActivityExceptionRepository(env.db).Create(ctx, exception))

	conflicts, err := data.DetectExceptionConflicts(ctx, monday, monday, slog.Default())
	require.NoError(t, err)
	require.Len(t, conflicts, 1)
	assert.Equal(t, scheduleService.SlotSourceSchedule, conflicts[0].ArrivalSource,
		"a timeless care day remains scheduled even without an arrival time")
	assert.Empty(t, conflicts[0].ExpectedArrival,
		"a timeless care day must not render as 00:00 in a cancellation warning")
}

// TestTimetableRead_ExceptionConflictsApplyTheBookingMode covers the second
// read surface: the planner warns when a moved activity now starts before a
// child arrives. On a weekday the booking no longer covers, that warning is
// about a child who is not coming at all.
func TestTimetableRead_ExceptionConflictsApplyTheBookingMode(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "konflikt", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "konflikt@example.com", "Konrad", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "thu": "11:45"})

	// The Thursday row survived the Abmeldung; the booking covers Monday only.
	staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Konflikt")
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayThursday, staff.ID, "11:45")

	monday := nextWeekday(decisionTestToday, time.Monday)
	thursday := monday.AddDays(3)

	// Both days: the activity is moved to 10:00, before the 11:45 arrival.
	room := testpkg.CreateTestRoom(t, env.db, "Konflikt-Raum")
	activity := testpkg.CreateTestActivityGroup(t, env.db, "Konflikt-AG")
	movedStart := timezone.NormalizeWallClock(time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC))
	for _, date := range []timezone.Date{monday, thursday} {
		instance := testpkg.CreateTestActivityInstance(t, env.db, date, room.ID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activity.ID,
		})
		testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, scheduleModels.AttendanceStatusExpected)
		createModifiedException(t, env, activity.ID, date, staff.ID, movedStart)
	}

	conflicts, err := timetableDataWithArrivalBaseline(t, env, true).
		DetectExceptionConflicts(ctx, monday, thursday, slog.Default())
	require.NoError(t, err)

	dates := make(map[string]bool, len(conflicts))
	for _, conflict := range conflicts {
		dates[conflict.Date] = true
	}
	assert.True(t, dates[monday.String()], "the booked day still warns about the moved start time")
	assert.False(t, dates[thursday.String()],
		"a stale arrival row must not warn about a child that is not in care that day")
}

func createModifiedException(
	t *testing.T,
	env *decisionTestEnv,
	activityGroupID int64,
	date timezone.Date,
	staffID int64,
	startTime time.Time,
) {
	t.Helper()
	reason := "Verlegt"
	exception := &scheduleModels.ActivityException{
		ActivityGroupID: activityGroupID,
		ExceptionDate:   date,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &startTime,
		Reason:          &reason,
		CreatedBy:       &staffID,
	}
	exception.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, exception.Validate())
	require.NoError(t, scheduleRepo.NewActivityExceptionRepository(env.db).Create(testpkg.Ctx(t), exception))
}
