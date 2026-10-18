package contracttest_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// timetableDataWithArrivalBaseline composes the Timetable owner's planner
// reads (#3551) the way the composition root does, with the arrival
// projection in booking mode (#2414). Only the readers the student week
// touches are real; the rest only have to exist.
func timetableDataWithArrivalBaseline(
	t *testing.T,
	env *decisionTestEnv,
	authoritative bool,
) timetable.TimetableDataCapability {
	t.Helper()
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: env.db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	data, err := arrivalTimetable.NewTimetableData(timetablePorts.TimetableDataDependencies{
		Instances:         env.timetable.ActivityInstance,
		InstanceStaff:     env.timetable.InstanceStaff,
		Participants:      env.timetable.InstanceStudent,
		PickupExceptions:  env.reposSchedules.StudentPickupException,
		ArrivalExceptions: env.reposSchedules.StudentArrivalException,
		ArrivalBaselines:  arrivalBaselineProjector{reader: bookingModeArrivalBaseline(t, env, authoritative)},
		PickupBaselines:   pickupBaselineProjector{reader: newPickupBaselineService(env.repos.CarePlan(), approvedOfferingTestProjection(env.reposEnrollment, env.reposPeople))},
		Visits:            presence,
		Templates:         unusedStudentWeekTemplates{},
		Groups:            unusedStudentWeekReaders{},
		Categories:        unusedStudentWeekCategories{},
		Rooms:             unusedStudentWeekReaders{},
		RoomOccupancy:     unusedStudentWeekReaders{},
		DeviationEvents:   unusedStudentWeekReaders{},
		ConflictAcks:      unusedStudentWeekReaders{},
		Logger:            slog.Default(),
	})
	require.NoError(t, err)
	return data
}

// unusedStudentWeekReaders stands in for the planner readers the student
// week never touches.
type unusedStudentWeekReaders struct {
	timetablePorts.DataGroups
	timetablePorts.RoomNames
	timetablePorts.RoomOccupancy
	timetablePorts.DeviationEventReader
	timetable.ConflictAckCapability
}

// unusedStudentWeekTemplates and unusedStudentWeekCategories stand in for
// the template and category writes of a spontaneous start.
type unusedStudentWeekTemplates struct {
	timetablePorts.DataTemplates
}

type unusedStudentWeekCategories struct {
	timetablePorts.DataCategories
}

// pickupBaselineProjector serves the Timetable pickup port from Care Plan's
// baseline projection, the way the composition root does.
type pickupBaselineProjector struct {
	reader careplan.PickupBaselineReader
}

func (p pickupBaselineProjector) ProjectPickups(ctx context.Context, studentIDs []int64, from, to calendar.Date) (timetablePorts.PickupBaselines, error) {
	projection, err := p.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return pickupBaselineProjection{projection: projection}, nil
}

type pickupBaselineProjection struct {
	projection *careplan.PickupBaselineProjection
}

func (p pickupBaselineProjection) RegularPickup(studentID int64, date calendar.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.PickupTime, true
}

// exceptionConflictsWithArrivalBaseline composes the Timetable owner's
// conflict detection over the same retained rows and the arrival projection
// in booking mode (#2414, #3550). Only the readers the exception-conflict read
// touches are real; the rest only have to exist.
func exceptionConflictsWithArrivalBaseline(
	t *testing.T,
	env *decisionTestEnv,
	authoritative bool,
) timetable.ConflictDetectionCapability {
	t.Helper()
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: env.db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	detection, err := arrivalTimetable.NewConflictDetection(timetablePorts.ConflictDetectionDependencies{
		Instances:         env.timetable.ActivityInstance,
		InstanceStaff:     env.timetable.InstanceStaff,
		InstanceStudents:  env.timetable.InstanceStudent,
		Exceptions:        env.timetable.ActivityException,
		Schedules:         env.timetable.ActivitySchedule,
		Shifts:            unusedConflictShifts{},
		Staff:             api.NewTimetableStaff(env.reposPeople.Staff),
		CalendarPeriods:   env.reposSchedules.CalendarPeriod,
		ArrivalExceptions: env.reposSchedules.StudentArrivalException,
		ArrivalBaselines:  arrivalBaselineProjector{reader: bookingModeArrivalBaseline(t, env, authoritative)},
		Presence:          presence,
		Sessions:          env.reposPresence.ActiveGroup,
		// Exception conflicts carry no fingerprint; the digest is never used.
		ContentHash: func([]byte) string { return "" },
		Logger:      slog.Default(),
	})
	require.NoError(t, err)
	return detection
}

// unusedConflictShifts stands in for the Dienstplan rows the exception read
// never touches.
type unusedConflictShifts struct {
	timetablePorts.ConflictShifts
}

// arrivalBaselineProjector serves the conflict detection's arrival port from
// Care Plan's baseline projection, the way the composition root does.
type arrivalBaselineProjector struct {
	reader careplan.ArrivalBaselineReader
}

func (p arrivalBaselineProjector) ProjectArrivals(ctx context.Context, studentIDs []int64, from, to calendar.Date) (timetablePorts.ArrivalBaselines, error) {
	projection, err := p.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return arrivalBaselineProjection{projection: projection}, nil
}

type arrivalBaselineProjection struct {
	projection *careplan.ArrivalBaselineProjection
}

func (p arrivalBaselineProjection) ExpectedArrival(studentID int64, date calendar.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.ExpectedArrival, true
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
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, timetable.WeekdayThursday, staff.ID, "11:45")

	monday := nextWeekday(decisionTestToday, time.Monday)
	thursday := monday.AddDays(3)

	t.Run("with the booking mode on only the booked weekday plans", func(t *testing.T) {
		data := timetableDataWithArrivalBaseline(t, env, true)
		week, err := data.StudentWeek(ctx, studentID, monday, thursday)
		require.NoError(t, err)

		booked := week.ArrivalByDate[monday.String()]
		require.True(t, booked.HasSchedule, "the booked weekday keeps its arrival")
		assert.Equal(t, "11:45", booked.Time.Format("15:04"))
		// The week carries the time only; the source stays with Care Plan's
		// projection the week is built from.
		projection, err := bookingModeArrivalBaseline(t, env, true).Project(ctx, []int64{studentID}, monday, thursday)
		require.NoError(t, err)
		source := projection.ForDate(studentID, monday)
		require.NotNil(t, source)
		assert.Equal(t, careplan.ScheduleSourceClassSchedule, source.Source)

		assert.False(t, week.ArrivalByDate[thursday.String()].HasSchedule,
			"a stale row on an unbooked weekday must not plan anything")
	})

	t.Run("with the booking mode off the stored row still plans", func(t *testing.T) {
		data := timetableDataWithArrivalBaseline(t, env, false)
		week, err := data.StudentWeek(ctx, studentID, monday, thursday)
		require.NoError(t, err)

		require.True(t, week.ArrivalByDate[thursday.String()].HasSchedule,
			"the six schools without a Halbjahresanmeldung must not change")
		assert.False(t, week.ArrivalByDate[monday.String()].HasSchedule,
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
	week, err := data.StudentWeek(ctx, studentID, monday, monday)
	require.NoError(t, err)

	arrival := week.ArrivalByDate[monday.String()]
	require.True(t, arrival.HasSchedule, "the booked day must remain visible without a class time")
	assert.True(t, arrival.Time.IsZero(), "the visible care day has no arrival time to show")

	room := testpkg.CreateTestRoom(t, env.db, "Ohne-Zeit-Raum")
	activity := testpkg.CreateTestActivityGroup(t, env.db, "Ohne-Zeit-AG")
	instance := testpkg.CreateTestActivityInstance(t, env.db, monday, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &activity.ID,
	})
	testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, studentpresence.SessionAttendanceExpected)
	reason := "Fällt aus"
	exception := &timetable.ActivityException{
		ActivityGroupID: activity.ID,
		ExceptionDate:   string(careplan.Date(monday)),
		ExceptionType:   timetable.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exception.TenantID = testpkg.Tenant(t)
	require.NoError(t, exception.Validate())
	require.NoError(t, arrivalTimetable.NativeExceptionRecords(env.timetable.ActivityException).Create(ctx, exception))

	conflicts, err := exceptionConflictsWithArrivalBaseline(t, env, true).DetectExceptionConflicts(ctx, monday, monday)
	require.NoError(t, err)
	require.Len(t, conflicts, 1)
	assert.Equal(t, timetable.SlotSourceSchedule, conflicts[0].ArrivalSource,
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
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, timetable.WeekdayThursday, staff.ID, "11:45")

	monday := nextWeekday(decisionTestToday, time.Monday)
	thursday := monday.AddDays(3)

	// Both days: the activity is moved to 10:00, before the 11:45 arrival.
	room := testpkg.CreateTestRoom(t, env.db, "Konflikt-Raum")
	activity := testpkg.CreateTestActivityGroup(t, env.db, "Konflikt-AG")
	movedStart := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC))
	for _, date := range []calendar.Date{monday, thursday} {
		instance := testpkg.CreateTestActivityInstance(t, env.db, date, room.ID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activity.ID,
		})
		testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, studentpresence.SessionAttendanceExpected)
		createModifiedException(t, env, activity.ID, date, staff.ID, movedStart)
	}

	conflicts, err := exceptionConflictsWithArrivalBaseline(t, env, true).
		DetectExceptionConflicts(ctx, monday, thursday)
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
	date calendar.Date,
	staffID int64,
	startTime time.Time,
) {
	t.Helper()
	reason := "Verlegt"
	exception := &timetable.ActivityException{
		ActivityGroupID: activityGroupID,
		ExceptionDate:   string(careplan.Date(date)),
		ExceptionType:   timetable.ActivityExceptionModified,
		StartTime:       activityClockPtr(&startTime),
		Reason:          &reason,
		CreatedBy:       &staffID,
	}
	exception.TenantID = testpkg.Tenant(t)
	require.NoError(t, exception.Validate())
	require.NoError(t, arrivalTimetable.NativeExceptionRecords(env.timetable.ActivityException).Create(testpkg.Ctx(t), exception))
}
