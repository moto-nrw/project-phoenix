package application

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return calendar.NormalizeWallClock(parsed)
}

func testShift(t *testing.T, staffID int64, date calendar.Date, start, end string) *scheduleModel.StaffShift {
	t.Helper()
	return &scheduleModel.StaffShift{
		StaffID:   staffID,
		Date:      calendar.Date(date),
		StartTime: testClock(t, start),
		EndTime:   testClock(t, end),
	}
}

func TestEffectiveCoverageStaffIDs_MirrorsConcreteEditSemantics(t *testing.T) {
	t.Parallel()

	prior := []*scheduleModel.InstanceStaff{
		{StaffID: 1, IsAbsent: true},
		{StaffID: 2, IsSubstitute: true},
		{StaffID: 3, IsSubstitute: true, IsAbsent: true},
	}

	// Unchanged membership preserves deviations. Absent always wins, even on
	// a substitute row; only the active substitute remains effective.
	assert.Equal(t, []int64{2}, effectiveCoverageStaffIDs([]int64{1, 2, 3}, prior))
	// Changed membership is a base-roster re-plan; all submitted IDs become
	// regular/effective, including someone who was previously absent.
	assert.Equal(t, []int64{1, 4}, effectiveCoverageStaffIDs([]int64{1, 4}, prior))
}

func TestEffectiveSeriesCoverageStaffIDs_MirrorsReplanSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		proposed []int64
		prior    []*scheduleModel.InstanceStaff
		want     []int64
	}{
		{
			name: "removed absence drops orphan substitute", proposed: []int64{3},
			prior: []*scheduleModel.InstanceStaff{{StaffID: 1, IsAbsent: true}, {StaffID: 2, IsSubstitute: true}},
			want:  []int64{3},
		},
		{
			name: "surviving absence caps active substitutes", proposed: []int64{1},
			prior: []*scheduleModel.InstanceStaff{
				{StaffID: 1, IsAbsent: true}, {StaffID: 4, IsAbsent: true},
				{StaffID: 2, IsSubstitute: true}, {StaffID: 3, IsSubstitute: true},
			},
			want: []int64{2},
		},
		{
			name: "promoted substitute is normal base staff", proposed: []int64{2},
			prior: []*scheduleModel.InstanceStaff{{StaffID: 1, IsAbsent: true}, {StaffID: 2, IsSubstitute: true}},
			want:  []int64{2},
		},
		{
			name: "absent substitute history does not consume budget", proposed: []int64{1},
			prior: []*scheduleModel.InstanceStaff{
				{StaffID: 1, IsAbsent: true},
				{StaffID: 2, IsSubstitute: true, IsAbsent: true},
				{StaffID: 3, IsSubstitute: true},
			},
			want: []int64{3},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, effectiveSeriesCoverageStaffIDs(test.proposed, test.prior))
		})
	}
}

func TestSeriesDeviationSourceAndSurvivorMatching(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.July, 6)
	groupID := int64(17)
	instance := func(id int64, start, status string, spontaneous bool) *scheduleModel.ActivityInstance {
		row := &scheduleModel.ActivityInstance{
			Date: calendar.Date(date), ActivityGroupID: &groupID, StartTime: testClock(t, start),
			Status: status, IsSpontaneous: spontaneous,
		}
		row.ID = id
		return row
	}
	early := instance(1, "08:00", scheduleModel.InstanceStatusPlanned, false)
	late := instance(2, "09:00", scheduleModel.InstanceStatusPlanned, false)
	assert.Same(t, early, selectPreservedDeviationSource([]*scheduleModel.ActivityInstance{early}, testClock(t, "10:00")), "a sole slot carries its deviation across a time move")
	assert.Same(t, late, selectPreservedDeviationSource([]*scheduleModel.ActivityInstance{early, late}, testClock(t, "09:00")))
	assert.Nil(t, selectPreservedDeviationSource([]*scheduleModel.ActivityInstance{early, late}, testClock(t, "10:00")), "multi-slot dates require an exact start match")

	cancelled := instance(3, "10:00", scheduleModel.InstanceStatusCancelled, false)
	assert.True(t, survivingInstanceBlocksSeriesCandidate([]*scheduleModel.ActivityInstance{cancelled}, groupID, date, testClock(t, "10:00")))
	assert.False(t, survivingInstanceBlocksSeriesCandidate([]*scheduleModel.ActivityInstance{cancelled}, groupID, date, testClock(t, "11:00")), "a different proposed key still materializes")
	assert.False(t, survivingInstanceBlocksSeriesCandidate([]*scheduleModel.ActivityInstance{late}, groupID, date, testClock(t, "09:00")), "planned template rows are deleted before materialization")
	spontaneous := instance(4, "09:00", scheduleModel.InstanceStatusPlanned, true)
	assert.True(t, survivingInstanceBlocksSeriesCandidate([]*scheduleModel.ActivityInstance{spontaneous}, groupID, date, testClock(t, "09:00")))
}

type fakeShiftReader struct {
	rows      []*scheduleModel.StaffShift
	err       error
	calls     int
	from      calendar.Date
	to        calendar.Date
	staffIDs  []int64
	dates     []calendar.Date
	usedWeeks []calendar.Date
}

func (f *fakeShiftReader) FindByStaffIDsAndDates(_ context.Context, staffIDs []int64, dates []calendar.Date) ([]*scheduleModel.StaffShift, error) {
	f.calls++
	f.staffIDs = append([]int64(nil), staffIDs...)
	f.dates = make([]calendar.Date, len(dates))
	for index, date := range dates {
		f.dates[index] = calendar.Date(date)
	}
	return f.rows, f.err
}

func (f *fakeShiftReader) FindUsedCalendarWeeks(_ context.Context, from, to calendar.Date) ([]calendar.Date, error) {
	f.calls++
	f.from, f.to = calendar.Date(from), calendar.Date(to)
	if f.err != nil {
		return nil, f.err
	}
	if f.usedWeeks != nil {
		weeks := make([]calendar.Date, len(f.usedWeeks))
		for index, week := range f.usedWeeks {
			weeks[index] = calendar.Date(week)
		}
		return weeks, nil
	}
	seen := make(map[calendar.Date]bool)
	weeks := make([]calendar.Date, 0)
	for _, shift := range f.rows {
		if shift == nil {
			continue
		}
		week, _ := timetable.ContainingCalendarWeek(calendar.Date(shift.Date))
		if !seen[week] {
			seen[week] = true
			weeks = append(weeks, week)
		}
	}
	converted := make([]calendar.Date, len(weeks))
	for index, week := range weeks {
		converted[index] = calendar.Date(week)
	}
	return converted, nil
}

func (f *fakeShiftReader) FindByDateRange(_ context.Context, from, to calendar.Date) ([]*scheduleModel.StaffShift, error) {
	f.calls++
	f.from, f.to = calendar.Date(from), calendar.Date(to)
	return f.rows, f.err
}

type fakeInstanceReader struct {
	rows       []*scheduleModel.ActivityInstance
	err        error
	calls      int
	groupCalls int
	groupID    int64
	from       calendar.Date
	to         calendar.Date
}

func (f *fakeInstanceReader) FindByID(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
	return nil, errors.New("not used by the shift-coverage probe")
}

func (f *fakeInstanceReader) FindByTenantAndDateRange(_ context.Context, _, _ calendar.Date) ([]*scheduleModel.ActivityInstance, error) {
	f.calls++
	return f.rows, f.err
}

func (f *fakeInstanceReader) FindByActivityGroupAndDateRange(_ context.Context, groupID int64, from, to calendar.Date) ([]*scheduleModel.ActivityInstance, error) {
	f.groupCalls++
	f.groupID, f.from, f.to = groupID, calendar.Date(from), calendar.Date(to)
	return f.rows, f.err
}

type fakeInstanceStaffReader struct {
	timetable.InstanceStaffQuery
	rows  []*scheduleModel.InstanceStaff
	err   error
	calls int
	ids   []int64
}

// ListInstanceStaff serves the probe's batched read of the instances' staff.
func (f *fakeInstanceStaffReader) ListInstanceStaff(_ context.Context, filter timetable.InstanceStaffFilter) ([]timetable.InstanceStaff, error) {
	f.calls++
	f.ids = append([]int64(nil), filter.InstanceIDs...)
	if f.err != nil {
		return nil, f.err
	}
	return fakeInstanceStaff(f.rows), nil
}

type fakeStaffReader struct {
	rows      []*timetable.StaffMember
	byID      map[int64]*timetable.StaffMember
	err       error
	listCalls int
	findCalls int
}

type fakeCalendarPeriodReader struct {
	period *scheduleModel.CalendarPeriod
	err    error
	calls  int
	id     any
}

type fakeActivityExceptionReader struct {
	timetable.ActivityExceptionQuery
	rows    []*scheduleModel.ActivityException
	err     error
	calls   int
	groupID int64
	from    calendar.Date
	to      calendar.Date
}

type fakeActivityScheduleReader struct {
	timetable.ScheduleQuery
	rows    []*activityContract.Schedule
	err     error
	calls   int
	groupID int64
}

func (f *fakeActivityScheduleReader) FindTemplateStartTimes(context.Context, []int64) ([]timetable.TemplateStartTime, error) {
	return nil, errors.New("not used by the shift-coverage probe")
}

// ListSchedules serves the probe's one read, the schedules of one template.
func (f *fakeActivityScheduleReader) ListSchedules(_ context.Context, filter timetable.ScheduleFilter) ([]timetable.Schedule, error) {
	f.calls++
	if len(filter.GroupIDs) == 1 {
		f.groupID = filter.GroupIDs[0]
	}
	if f.err != nil {
		return nil, f.err
	}
	return publicTestSchedules(f.rows), nil
}

// ListActivityExceptions serves the probe's one series read, which always
// narrows to a template and a date window.
func (f *fakeActivityExceptionReader) ListActivityExceptions(_ context.Context, filter timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error) {
	if filter.ActivityGroupID == nil || filter.FromDate == nil || filter.ToDate == nil {
		return nil, errors.New("not used by the shift-coverage probe")
	}
	f.calls++
	from, err := calendar.ParseDate(*filter.FromDate)
	if err != nil {
		return nil, err
	}
	to, err := calendar.ParseDate(*filter.ToDate)
	if err != nil {
		return nil, err
	}
	f.groupID, f.from, f.to = *filter.ActivityGroupID, from, to
	if f.err != nil {
		return nil, f.err
	}
	result := make([]timetable.ActivityException, 0, len(f.rows))
	for _, row := range f.rows {
		input := publicActivityExceptionInput(row)
		result = append(result, timetable.ActivityException{
			ID: row.ID, ActivityGroupID: input.ActivityGroupID, ExceptionDate: input.ExceptionDate, ExceptionType: input.ExceptionType,
			StartTime: input.StartTime, EndTime: input.EndTime, RoomID: input.RoomID, Reason: input.Reason, CreatedBy: input.CreatedBy,
		})
	}
	return result, nil
}

func (f *fakeCalendarPeriodReader) FindByID(_ context.Context, nativeRecordID int64) (*scheduleModel.CalendarPeriod, error) {
	var id any = nativeRecordID
	f.calls++
	f.id = id
	return f.period, f.err
}

func (f *fakeStaffReader) ListAllWithPerson(_ context.Context) ([]*timetable.StaffMember, error) {
	f.listCalls++
	return f.rows, f.err
}

func (f *fakeStaffReader) FindWithPersonByIDs(_ context.Context, _ []int64) (map[int64]*timetable.StaffMember, error) {
	f.findCalls++
	return f.byID, f.err
}

func fakeStaff(id int64, first, last string) *timetable.StaffMember {
	return &timetable.StaffMember{Person: &timetable.Person{FirstName: first, LastName: last}, PersonID: id + 100}
}

func TestDetectShiftCoverage_BoundsReturnedWarningDetails(t *testing.T) {
	t.Parallel()

	start := calendar.NewDate(2026, time.January, 5)
	dates := make([]calendar.Date, 130)
	weeks := make([]calendar.Date, 0, 20)
	seenWeeks := make(map[calendar.Date]bool)
	for i := range dates {
		dates[i] = start.AddDays(i)
		week, _ := timetable.ContainingCalendarWeek(dates[i])
		if !seenWeeks[week] {
			seenWeeks[week] = true
			weeks = append(weeks, week)
		}
	}
	staff := fakeStaff(1, "Ada", "Lovelace")
	staff.ID = 1
	result, err := detectShiftCoverage(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: &fakeShiftReader{rows: shiftsMarkingWeeks(weeks)},
		Staff:  &fakeStaffReader{byID: map[int64]*timetable.StaffMember{1: staff}},
	}, timetable.ShiftCoverageProbe{
		Dates: dates, StartTime: testClock(t, "12:00"), EndTime: testClock(t, "13:00"), StaffIDs: []int64{1},
	})
	require.NoError(t, err)
	assert.Equal(t, 130, result.TotalWarningCount)
	assert.Len(t, result.Warnings, maxReturnedShiftCoverageWarnings)
}

func TestDetectShiftCoverageWarnings_EffectiveRosterAndContainingWeek(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.July, 8) // Wednesday
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// A shift anywhere in the tenant activates warnings for the week.
		testShift(t, 9, date, "07:00", "08:00"),
	}}
	instanceStaff := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 1, IsAbsent: true},
		{StaffID: 2, IsSubstitute: true},
		{StaffID: 3, IsSubstitute: true, IsAbsent: true},
	}}
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{2: fakeStaff(2, "Ersatz", "Person")}}
	excludeID := int64(42)
	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shiftReader, InstanceStaff: instanceStaff, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
	}, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "11:00"),
		StaffIDs: []int64{1, 2, 3}, ExcludeInstanceID: &excludeID,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1, "only the active substitute is effective")
	assert.Equal(t, int64(2), warnings[0].StaffID)
	assert.Equal(t, "Ersatz Person", warnings[0].StaffName)
	assert.Equal(t, "09:00", warnings[0].UncoveredStartTime)
	assert.Equal(t, "11:00", warnings[0].UncoveredEndTime)
	assert.Equal(t, calendar.NewDate(2026, time.July, 6), shiftReader.from)
	assert.Equal(t, calendar.NewDate(2026, time.July, 12), shiftReader.to)
	assert.Equal(t, 1, instanceStaff.calls)
	assert.Equal(t, 1, shiftReader.calls)
	assert.Equal(t, 1, staff.findCalls)
}

func TestDetectShiftCoverageWarnings_ChangedRosterDropsPriorAbsence(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.July, 6)
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{testShift(t, 9, date, "07:00", "08:00")}}
	instanceStaff := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 1, IsAbsent: true}, {StaffID: 2, IsSubstitute: true},
	}}
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{
		1: fakeStaff(1, "Wieder", "Eingeplant"), 4: fakeStaff(4, "Neu", "Dabei"),
	}}
	excludeID := int64(42)
	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shiftReader, InstanceStaff: instanceStaff, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
	}, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"),
		StaffIDs: []int64{1, 4}, ExcludeInstanceID: &excludeID,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 2, "changed membership makes both submitted IDs effective")
	assert.Equal(t, []int64{1, 4}, []int64{warnings[0].StaffID, warnings[1].StaffID})
	assert.Equal(t, 1, instanceStaff.calls, "concrete roster must be loaded in one batch")
	assert.Equal(t, 1, shiftReader.calls, "shifts and tenant week usage share one full-week snapshot")
	assert.Equal(t, 1, staff.findCalls, "all warning names must be loaded in one batch")
}

func TestDetectShiftCoverageWarnings_ConvertedInstanceSurvivesRecurrenceFiltering(t *testing.T) {
	t.Parallel()

	weekA := calendar.NewDate(2026, time.July, 6)
	concreteWeekB := weekA.AddDays(7)
	anchor := weekA
	scheduleAnchor := calendar.Date(anchor)
	period := &scheduleModel.CalendarPeriod{
		StartDate: calendar.Date(weekA), EndDate: calendar.Date(concreteWeekB.AddDays(6)),
		WeekCycleLength: 2, WeekCycleAnchor: &scheduleAnchor, IsActive: true,
	}
	period.ID = 61
	pattern := 1
	instanceID := int64(62)
	instanceStaff := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{InstanceID: instanceID, StaffID: 1, IsAbsent: true},
		{InstanceID: instanceID, StaffID: 2, IsSubstitute: true},
	}}
	shifts := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// The normal A-week occurrence's proposed base roster is covered.
		testShift(t, 1, weekA, "09:00", "10:00"),
		testShift(t, 2, weekA, "09:00", "10:00"),
		// An unrelated shift activates the concrete B week. Its active
		// substitute deliberately has no shift and must warn.
		testShift(t, 9, concreteWeekB, "07:00", "08:00"),
	}}
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{2: fakeStaff(2, "Ersatz", "Person")}}

	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shifts, InstanceStaff: instanceStaff, Staff: staff,
		CalendarPeriods: &fakeCalendarPeriodReader{period: period},
	}, timetable.ShiftCoverageProbe{
		Dates:     []calendar.Date{weekA, concreteWeekB},
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1, 2},
		ExcludeInstanceID: &instanceID, ConcreteInstanceDate: &concreteWeekB,
		CalendarPeriodID: &period.ID, WeekPattern: &pattern,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, concreteWeekB.String(), warnings[0].Date)
	assert.Equal(t, int64(2), warnings[0].StaffID)
	assert.Equal(t, 1, instanceStaff.calls)
}

// An external caregiver never has a Dienstplan shift (#4573: externals stay
// out of the Dienstplan), so a block they run warns about nobody but staff.
func TestDetectShiftCoverage_SkipsExternalCaregivers(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.July, 6)
	external := fakeStaff(2, "Erik", "Extern")
	external.IsGuest = true
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{1: fakeStaff(1, "Max", "Mustermann"), 2: external}}
	deps := ports.ConflictDetectionDependencies{
		Shifts:        &fakeShiftReader{rows: []*scheduleModel.StaffShift{testShift(t, 9, date, "07:00", "08:00")}},
		InstanceStaff: &fakeInstanceStaffReader{}, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
	}

	result, err := detectShiftCoverage(context.Background(), deps, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{date}, StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), StaffIDs: []int64{1, 2},
	})
	require.NoError(t, err)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, int64(1), result.Warnings[0].StaffID)
	assert.Equal(t, 1, result.TotalWarningCount)
	assert.Equal(t, 1, staff.findCalls)

	onlyExternal, err := detectShiftCoverage(context.Background(), deps, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{date}, StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), StaffIDs: []int64{2},
	})
	require.NoError(t, err)
	assert.Empty(t, onlyExternal.Warnings)
	assert.Zero(t, onlyExternal.TotalWarningCount)
}

func TestDetectShiftCoverageWarnings_GapAndTenantUnusedCases(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.July, 6)
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{1: fakeStaff(1, "Max", "Mustermann")}}

	t.Run("two shifts with gap", func(t *testing.T) {
		warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
			Shifts: &fakeShiftReader{rows: []*scheduleModel.StaffShift{
				testShift(t, 1, date, "08:00", "09:00"), testShift(t, 1, date, "10:00", "12:00"),
			}},
			InstanceStaff: &fakeInstanceStaffReader{}, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
		}, timetable.ShiftCoverageProbe{Dates: []calendar.Date{date}, StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), StaffIDs: []int64{1}})
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		assert.Equal(t, "09:00", warnings[0].UncoveredStartTime)
		assert.Equal(t, "10:00", warnings[0].UncoveredEndTime)
		assert.Contains(t, warnings[0].Message, "Max Mustermann")
		assert.Contains(t, warnings[0].Message, "08:00–12:00")
		assert.Contains(t, warnings[0].Message, "09:00–10:00")
	})

	t.Run("tenant does not use Dienstplan", func(t *testing.T) {
		nameReader := &fakeStaffReader{byID: staff.byID}
		warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
			Shifts: &fakeShiftReader{}, InstanceStaff: &fakeInstanceStaffReader{}, Staff: nameReader, CalendarPeriods: &fakeCalendarPeriodReader{},
		}, timetable.ShiftCoverageProbe{Dates: []calendar.Date{date}, StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), StaffIDs: []int64{1}})
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.Zero(t, nameReader.findCalls, "unused-week suppression must not perform name lookups")
	})
}

func TestDetectShiftCoverageWarnings_RecurringDatesReusePeriodAndABEngineWithFixedReads(t *testing.T) {
	t.Parallel()

	weekA := calendar.NewDate(2026, time.July, 6)
	weekB := weekA.AddDays(7)
	nextWeekA := weekA.AddDays(14)
	anchor := weekA
	scheduleAnchor := calendar.Date(anchor)
	period := &scheduleModel.CalendarPeriod{
		StartDate: calendar.Date(weekA), EndDate: calendar.Date(nextWeekA.AddDays(6)),
		WeekCycleLength: 2, WeekCycleAnchor: &scheduleAnchor, IsActive: true,
	}
	period.ID = 51
	periodReader := &fakeCalendarPeriodReader{period: period}
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		testShift(t, 9, weekA, "07:00", "08:00"),
		testShift(t, 9, weekB, "07:00", "08:00"),
		testShift(t, 9, nextWeekA, "07:00", "08:00"),
	}}
	instanceStaff := &fakeInstanceStaffReader{}
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{1: fakeStaff(1, "Serie", "Person")}}
	weekPattern := 1
	periodID := period.ID

	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shiftReader, InstanceStaff: instanceStaff, Staff: staff, CalendarPeriods: periodReader,
	}, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{
			nextWeekA, weekA, weekA.AddDays(2), weekA,
			weekB, weekB.AddDays(2), nextWeekA.AddDays(2), nextWeekA.AddDays(14),
		},
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
		CalendarPeriodID: &periodID, WeekPattern: &weekPattern,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 4)
	assert.Equal(t, []string{
		weekA.String(), weekA.AddDays(2).String(), nextWeekA.String(), nextWeekA.AddDays(2).String(),
	}, []string{warnings[0].Date, warnings[1].Date, warnings[2].Date, warnings[3].Date})
	assert.Equal(t, 1, periodReader.calls, "period must be loaded once for the entire series")
	assert.Equal(t, 1, shiftReader.calls, "all relevant dates share one full-week shift snapshot")
	assert.Equal(t, 1, staff.findCalls, "all names must share one batch read")
	assert.Zero(t, instanceStaff.calls, "series probes have no concrete deviation read")
	assert.Equal(t, weekA, shiftReader.from)
	assert.Equal(t, nextWeekA.AddDays(6), shiftReader.to)
}

func TestDetectShiftCoverageWarnings_SeriesReplanUsesEffectiveExceptionsAndDeviations(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.July, 6)
	wednesday := monday.AddDays(2)
	friday := monday.AddDays(4)
	groupID := int64(71)
	periodID := int64(72)
	weekPattern := 0
	period := &scheduleModel.CalendarPeriod{StartDate: calendar.Date(monday), EndDate: calendar.Date(friday), WeekCycleLength: 1, IsActive: true}
	period.ID = periodID

	mondayInstance := &scheduleModel.ActivityInstance{
		Date: calendar.Date(monday), ActivityGroupID: &groupID, Status: scheduleModel.InstanceStatusPlanned,
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "11:00"),
	}
	mondayInstance.ID = 81
	instances := &fakeInstanceReader{rows: []*scheduleModel.ActivityInstance{mondayInstance}}
	instanceStaff := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{InstanceID: mondayInstance.ID, StaffID: 1, IsAbsent: true},
		{InstanceID: mondayInstance.ID, StaffID: 2, IsSubstitute: true},
	}}
	modifiedStart := testClock(t, "10:00")
	exceptions := &fakeActivityExceptionReader{rows: []*scheduleModel.ActivityException{
		{ActivityGroupID: groupID, ExceptionDate: calendar.Date(wednesday), ExceptionType: scheduleModel.ActivityExceptionCancelled},
		{ActivityGroupID: groupID, ExceptionDate: calendar.Date(friday), ExceptionType: scheduleModel.ActivityExceptionModified, StartTime: &modifiedStart},
	}}
	shifts := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// Monday: the active substitute is fully covered by their own shift.
		testShift(t, 2, monday, "09:00", "11:00"),
		// Friday: the start-only exception overlays 10:00 onto the proposed
		// 09:00-11:00 window, leaving exactly 10:30-11:00 uncovered.
		testShift(t, 1, friday, "09:00", "10:30"),
	}}
	staff := &fakeStaffReader{byID: map[int64]*timetable.StaffMember{
		1: fakeStaff(1, "Plan", "Person"),
		2: fakeStaff(2, "Ersatz", "Person"),
	}}

	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shifts, Instances: legacyInstanceReads{fake: instances}, Exceptions: exceptions,
		Schedules:     &fakeActivityScheduleReader{rows: []*activityContract.Schedule{{ActivityGroupID: groupID}}},
		InstanceStaff: instanceStaff, Staff: staff,
		CalendarPeriods: &fakeCalendarPeriodReader{period: period},
	}, timetable.ShiftCoverageProbe{
		Dates:     []calendar.Date{monday, wednesday, friday},
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "11:00"), StaffIDs: []int64{1},
		ReplanActivityGroupID: &groupID, CalendarPeriodID: &periodID, WeekPattern: &weekPattern,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1, "covered substitute and cancelled occurrence must not warn")
	assert.Equal(t, int64(1), warnings[0].StaffID)
	assert.Equal(t, friday.String(), warnings[0].Date)
	assert.Equal(t, "10:00", warnings[0].StartTime)
	assert.Equal(t, "11:00", warnings[0].EndTime)
	assert.Equal(t, "10:30", warnings[0].UncoveredStartTime)
	assert.Equal(t, "11:00", warnings[0].UncoveredEndTime)
	assert.Equal(t, 1, instances.groupCalls)
	assert.Equal(t, groupID, instances.groupID)
	assert.Equal(t, monday, instances.from)
	assert.Equal(t, friday, instances.to)
	assert.Equal(t, 1, exceptions.calls)
	assert.Equal(t, 1, instanceStaff.calls)
	assert.Equal(t, 1, shifts.calls)
	assert.Equal(t, 1, staff.findCalls)
}

func TestDetectShiftCoverageWarnings_UsesCanonicalSeriesBoundsAndActivePeriod(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.July, 6)
	wednesday := monday.AddDays(2)
	nextMonday := monday.AddDays(7)
	groupID, periodID, pattern := int64(91), int64(92), 0
	validFrom, validUntil := activityContract.Date(wednesday), activityContract.Date(nextMonday)
	schedules := &fakeActivityScheduleReader{rows: []*activityContract.Schedule{{
		ActivityGroupID: groupID,
		ValidFrom:       &validFrom,
		ValidUntil:      &validUntil,
	}}}
	period := &scheduleModel.CalendarPeriod{
		StartDate: calendar.Date(monday), EndDate: calendar.Date(nextMonday), WeekCycleLength: 1, IsActive: true,
	}
	period.ID = periodID
	shifts := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// An unrelated shift activates the week; the assigned person has none.
		testShift(t, 9, monday, "07:00", "08:00"),
	}}

	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shifts, Instances: legacyInstanceReads{fake: &fakeInstanceReader{}}, Exceptions: &fakeActivityExceptionReader{},
		Schedules: schedules, InstanceStaff: &fakeInstanceStaffReader{},
		Staff:           &fakeStaffReader{byID: map[int64]*timetable.StaffMember{1: fakeStaff(1, "Grenze", "Serie")}},
		CalendarPeriods: &fakeCalendarPeriodReader{period: period},
	}, timetable.ShiftCoverageProbe{
		Dates:     []calendar.Date{monday, wednesday, nextMonday},
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
		ReplanActivityGroupID: &groupID, CalendarPeriodID: &periodID, WeekPattern: &pattern,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1, "valid_from is inclusive and valid_until is exclusive")
	assert.Equal(t, wednesday.String(), warnings[0].Date)
	assert.Equal(t, 1, schedules.calls)
	assert.Equal(t, groupID, schedules.groupID)

	inactive := *period
	inactive.IsActive = false
	shiftReader := &fakeShiftReader{rows: shifts.rows}
	warnings, err = detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shiftReader, Instances: legacyInstanceReads{fake: &fakeInstanceReader{}}, Exceptions: &fakeActivityExceptionReader{},
		Schedules: schedules, InstanceStaff: &fakeInstanceStaffReader{}, Staff: &fakeStaffReader{},
		CalendarPeriods: &fakeCalendarPeriodReader{period: &inactive},
	}, timetable.ShiftCoverageProbe{
		Dates:     []calendar.Date{wednesday},
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
		ReplanActivityGroupID: &groupID, CalendarPeriodID: &periodID, WeekPattern: &pattern,
	})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Zero(t, shiftReader.calls, "inactive periods cannot materialize and need no shift read")
}

func TestDetectShiftCoverageWarnings_IncludesWeekendShifts(t *testing.T) {
	t.Parallel()

	saturday := calendar.NewDate(2026, time.July, 11)
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// A weekend-only shift activates the tenant's calendar week.
		testShift(t, 9, saturday, "07:00", "08:00"),
	}}
	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: shiftReader, InstanceStaff: &fakeInstanceStaffReader{},
		Staff:           &fakeStaffReader{byID: map[int64]*timetable.StaffMember{1: fakeStaff(1, "Samstag", "Dienst")}},
		CalendarPeriods: &fakeCalendarPeriodReader{},
	}, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{saturday}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, saturday.String(), warnings[0].Date)
	assert.Equal(t, calendar.NewDate(2026, time.July, 6), shiftReader.from)
	assert.Equal(t, calendar.NewDate(2026, time.July, 12), shiftReader.to)
}

func TestNormalizeShiftCoverageQuery_ValidationBounds(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.January, 1)
	valid := timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
	}
	tests := []struct {
		name   string
		mutate func(*timetable.ShiftCoverageProbe)
	}{
		{name: "dates required", mutate: func(query *timetable.ShiftCoverageProbe) { query.Dates = nil }},
		{name: "calendar date required", mutate: func(query *timetable.ShiftCoverageProbe) { query.Dates = []calendar.Date{""} }},
		{name: "date count bounded", mutate: func(query *timetable.ShiftCoverageProbe) {
			query.Dates = make([]calendar.Date, 367)
			for day := range query.Dates {
				query.Dates[day] = date.AddDays(day)
			}
		}},
		{name: "span bounded", mutate: func(query *timetable.ShiftCoverageProbe) { query.Dates = append(query.Dates, date.AddDays(367)) }},
		{name: "end after start", mutate: func(query *timetable.ShiftCoverageProbe) { query.EndTime = query.StartTime }},
		{name: "staff required", mutate: func(query *timetable.ShiftCoverageProbe) { query.StaffIDs = nil }},
		{name: "positive staff", mutate: func(query *timetable.ShiftCoverageProbe) { query.StaffIDs = []int64{0} }},
		{name: "staff count bounded", mutate: func(query *timetable.ShiftCoverageProbe) {
			query.StaffIDs = make([]int64, maxShiftCoverageStaffIDs+1)
			for index := range query.StaffIDs {
				query.StaffIDs[index] = int64(index + 1)
			}
		}},
		{name: "date staff product bounded", mutate: func(query *timetable.ShiftCoverageProbe) {
			query.Dates = make([]calendar.Date, 21)
			for day := range query.Dates {
				query.Dates[day] = date.AddDays(day)
			}
			query.StaffIDs = make([]int64, 500)
			for index := range query.StaffIDs {
				query.StaffIDs[index] = int64(index + 1)
			}
		}},
		{name: "exclude positive", mutate: func(query *timetable.ShiftCoverageProbe) { zero := int64(0); query.ExcludeInstanceID = &zero }},
		{name: "multi-date exclude requires concrete date", mutate: func(query *timetable.ShiftCoverageProbe) {
			id := int64(2)
			query.ExcludeInstanceID = &id
			query.Dates = append(query.Dates, date.AddDays(1))
		}},
		{name: "concrete date requires instance", mutate: func(query *timetable.ShiftCoverageProbe) { query.ConcreteInstanceDate = &date }},
		{name: "concrete date must be a candidate", mutate: func(query *timetable.ShiftCoverageProbe) {
			id, other := int64(2), date.AddDays(1)
			query.ExcludeInstanceID, query.ConcreteInstanceDate = &id, &other
		}},
		{name: "replan group requires recurrence", mutate: func(query *timetable.ShiftCoverageProbe) { id := int64(2); query.ReplanActivityGroupID = &id }},
		{name: "replan group and instance are exclusive", mutate: func(query *timetable.ShiftCoverageProbe) {
			instanceID, groupID, periodID, pattern := int64(2), int64(3), int64(4), 0
			query.ExcludeInstanceID, query.ReplanActivityGroupID = &instanceID, &groupID
			query.CalendarPeriodID, query.WeekPattern = &periodID, &pattern
		}},
		{name: "recurrence pair", mutate: func(query *timetable.ShiftCoverageProbe) { id := int64(2); query.CalendarPeriodID = &id }},
		{name: "period positive", mutate: func(query *timetable.ShiftCoverageProbe) {
			zero, pattern := int64(0), 1
			query.CalendarPeriodID, query.WeekPattern = &zero, &pattern
		}},
		{name: "week pattern range", mutate: func(query *timetable.ShiftCoverageProbe) {
			id, pattern := int64(2), 3
			query.CalendarPeriodID, query.WeekPattern = &id, &pattern
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := valid
			query.Dates = append([]calendar.Date(nil), valid.Dates...)
			query.StaffIDs = append([]int64(nil), valid.StaffIDs...)
			test.mutate(&query)
			_, _, err := normalizeShiftCoverageProbe(query)
			require.ErrorIs(t, err, timetable.ErrInvalidShiftCoverageQuery)
		})
	}

	t.Run("dedupes and admits full school year", func(t *testing.T) {
		dates := make([]calendar.Date, 0, 367)
		for day := 0; day < 366; day++ {
			dates = append(dates, date.AddDays(day))
		}
		dates = append(dates, date)
		query := valid
		query.Dates = dates
		normalized, staffIDs, err := normalizeShiftCoverageProbe(query)
		require.NoError(t, err)
		assert.Len(t, normalized, 366)
		assert.Equal(t, []int64{1}, staffIDs)
	})
}

func TestDetectShiftCoverageWarnings_MissingTenantPeriodIsValidationError(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, time.July, 6)
	periodID, weekPattern := int64(999), 1
	periodReader := &fakeCalendarPeriodReader{err: sql.ErrNoRows}
	warnings, err := detectShiftCoverageWarnings(context.Background(), ports.ConflictDetectionDependencies{
		Shifts: &fakeShiftReader{}, InstanceStaff: &fakeInstanceStaffReader{}, Staff: &fakeStaffReader{}, CalendarPeriods: periodReader,
	}, timetable.ShiftCoverageProbe{
		Dates: []calendar.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
		CalendarPeriodID: &periodID, WeekPattern: &weekPattern,
	})
	require.ErrorIs(t, err, timetable.ErrInvalidShiftCoverageQuery)
	assert.Nil(t, warnings)
	assert.Equal(t, 1, periodReader.calls)
}

// detectShiftCoverage runs the probe over the given readers only; the
// start-check and planning readers stay unset because the probe never
// touches them.
func detectShiftCoverage(ctx context.Context, deps ports.ConflictDetectionDependencies, probe timetable.ShiftCoverageProbe) (timetable.ShiftCoverageResult, error) {
	deps.RunInTx = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	detection := &conflictDetection{deps: deps, logger: slog.Default()}
	return detection.DetectShiftCoverage(ctx, probe)
}

func detectShiftCoverageWarnings(ctx context.Context, deps ports.ConflictDetectionDependencies, probe timetable.ShiftCoverageProbe) ([]timetable.ShiftCoverageWarning, error) {
	result, err := detectShiftCoverage(ctx, deps, probe)
	return result.Warnings, err
}

func shiftsMarkingWeeks(weeks []calendar.Date) []*scheduleModel.StaffShift {
	rows := make([]*scheduleModel.StaffShift, 0, len(weeks))
	for _, week := range weeks {
		rows = append(rows, &scheduleModel.StaffShift{StaffID: 999, Date: week.AddDays(5)})
	}
	return rows
}
