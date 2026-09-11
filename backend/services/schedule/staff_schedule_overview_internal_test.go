package schedule

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

func testShift(t *testing.T, staffID int64, date timezone.Date, start, end string) *scheduleModel.StaffShift {
	t.Helper()
	return &scheduleModel.StaffShift{
		StaffID:   staffID,
		Date:      scheduleModel.Date(date),
		StartTime: testClock(t, start),
		EndTime:   testClock(t, end),
	}
}

func formattedGaps(gaps []ShiftCoverageInterval) [][2]string {
	out := make([][2]string, 0, len(gaps))
	for _, gap := range gaps {
		out = append(out, [2]string{
			timezone.NormalizeWallClock(gap.StartTime).Format("15:04"),
			timezone.NormalizeWallClock(gap.EndTime).Format("15:04"),
		})
	}
	return out
}

func TestUncoveredShiftIntervals(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	start := testClock(t, "08:00")
	end := testClock(t, "12:00")

	tests := []struct {
		name   string
		shifts []*scheduleModel.StaffShift
		want   [][2]string
	}{
		{name: "one shift covers", shifts: []*scheduleModel.StaffShift{testShift(t, 1, date, "07:00", "13:00")}, want: [][2]string{}},
		{name: "no shift", want: [][2]string{{"08:00", "12:00"}}},
		{name: "starts too late", shifts: []*scheduleModel.StaffShift{testShift(t, 1, date, "09:00", "13:00")}, want: [][2]string{{"08:00", "09:00"}}},
		{name: "ends too early", shifts: []*scheduleModel.StaffShift{testShift(t, 1, date, "07:00", "11:00")}, want: [][2]string{{"11:00", "12:00"}}},
		{name: "touching shifts cover", shifts: []*scheduleModel.StaffShift{
			testShift(t, 1, date, "08:00", "10:00"),
			testShift(t, 1, date, "10:00", "12:00"),
		}, want: [][2]string{}},
		{name: "gap is exact", shifts: []*scheduleModel.StaffShift{
			testShift(t, 1, date, "10:00", "12:00"),
			testShift(t, 1, date, "07:00", "09:00"),
		}, want: [][2]string{{"09:00", "10:00"}}},
		{name: "overlaps merge", shifts: []*scheduleModel.StaffShift{
			testShift(t, 1, date, "07:00", "10:30"),
			testShift(t, 1, date, "09:00", "13:00"),
		}, want: [][2]string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formattedGaps(uncoveredShiftIntervals(start, end, tc.shifts)))
		})
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

	date := timezone.NewDate(2026, time.July, 6)
	groupID := int64(17)
	instance := func(id int64, start, status string, spontaneous bool) *scheduleModel.ActivityInstance {
		row := &scheduleModel.ActivityInstance{
			Date: scheduleModel.Date(date), ActivityGroupID: &groupID, StartTime: testClock(t, start),
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
	from      timezone.Date
	to        timezone.Date
	staffIDs  []int64
	dates     []timezone.Date
	usedWeeks []timezone.Date
}

func (f *fakeShiftReader) FindByStaffIDsAndDates(_ context.Context, staffIDs []int64, dates []scheduleModel.Date) ([]*scheduleModel.StaffShift, error) {
	f.calls++
	f.staffIDs = append([]int64(nil), staffIDs...)
	f.dates = make([]timezone.Date, len(dates))
	for index, date := range dates {
		f.dates[index] = timezone.Date(date)
	}
	return f.rows, f.err
}

func (f *fakeShiftReader) FindUsedCalendarWeeks(_ context.Context, from, to scheduleModel.Date) ([]scheduleModel.Date, error) {
	f.calls++
	f.from, f.to = timezone.Date(from), timezone.Date(to)
	if f.err != nil {
		return nil, f.err
	}
	if f.usedWeeks != nil {
		weeks := make([]scheduleModel.Date, len(f.usedWeeks))
		for index, week := range f.usedWeeks {
			weeks[index] = scheduleModel.Date(week)
		}
		return weeks, nil
	}
	seen := make(map[timezone.Date]bool)
	weeks := make([]timezone.Date, 0)
	for _, shift := range f.rows {
		if shift == nil {
			continue
		}
		week, _ := containingCalendarWeek(timezone.Date(shift.Date))
		if !seen[week] {
			seen[week] = true
			weeks = append(weeks, week)
		}
	}
	converted := make([]scheduleModel.Date, len(weeks))
	for index, week := range weeks {
		converted[index] = scheduleModel.Date(week)
	}
	return converted, nil
}

func (f *fakeShiftReader) FindByDateRange(_ context.Context, from, to scheduleModel.Date) ([]*scheduleModel.StaffShift, error) {
	f.calls++
	f.from, f.to = timezone.Date(from), timezone.Date(to)
	return f.rows, f.err
}

type fakeInstanceReader struct {
	rows       []*scheduleModel.ActivityInstance
	err        error
	calls      int
	groupCalls int
	groupID    int64
	from       timezone.Date
	to         timezone.Date
}

func (f *fakeInstanceReader) FindByTenantAndDateRange(_ context.Context, _, _ scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error) {
	f.calls++
	return f.rows, f.err
}

func (f *fakeInstanceReader) FindByActivityGroupAndDateRange(_ context.Context, groupID int64, from, to scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error) {
	f.groupCalls++
	f.groupID, f.from, f.to = groupID, timezone.Date(from), timezone.Date(to)
	return f.rows, f.err
}

type fakeInstanceStaffReader struct {
	rows  []*scheduleModel.InstanceStaff
	err   error
	calls int
	ids   []int64
}

func (f *fakeInstanceStaffReader) FindByInstanceIDs(_ context.Context, ids []int64) ([]*scheduleModel.InstanceStaff, error) {
	f.calls++
	f.ids = append([]int64(nil), ids...)
	return f.rows, f.err
}

type fakeRoomReader struct {
	rows  []*facilities.Room
	err   error
	calls int
	ids   []int64
}

func (f *fakeRoomReader) FindByIDs(_ context.Context, ids []int64) ([]*facilities.Room, error) {
	f.calls++
	f.ids = append([]int64(nil), ids...)
	return f.rows, f.err
}

type fakeStaffReader struct {
	rows      []*users.Staff
	byID      map[int64]*users.Staff
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
	rows    []*scheduleModel.ActivityException
	err     error
	calls   int
	groupID int64
	from    timezone.Date
	to      timezone.Date
}

type fakeActivityScheduleReader struct {
	rows    []*activitiesModel.Schedule
	err     error
	calls   int
	groupID int64
}

func (f *fakeActivityScheduleReader) FindByGroupID(_ context.Context, groupID int64) ([]*activitiesModel.Schedule, error) {
	f.calls++
	f.groupID = groupID
	return f.rows, f.err
}

func (f *fakeActivityExceptionReader) FindByActivityGroupAndDateRange(
	_ context.Context,
	groupID int64,
	from, to scheduleModel.Date,
) ([]*scheduleModel.ActivityException, error) {
	f.calls++
	f.groupID, f.from, f.to = groupID, timezone.Date(from), timezone.Date(to)
	return f.rows, f.err
}

func (f *fakeCalendarPeriodReader) FindByID(_ context.Context, id any) (*scheduleModel.CalendarPeriod, error) {
	f.calls++
	f.id = id
	return f.period, f.err
}

func (f *fakeStaffReader) ListAllWithPerson(_ context.Context) ([]*users.Staff, error) {
	f.listCalls++
	return f.rows, f.err
}

func (f *fakeStaffReader) FindWithPersonByIDs(_ context.Context, _ []int64) (map[int64]*users.Staff, error) {
	f.findCalls++
	return f.byID, f.err
}

func fakeStaff(id int64, first, last string) *users.Staff {
	return &users.Staff{Person: &users.Person{FirstName: first, LastName: last}, PersonID: id + 100}
}

type fakeOverviewHolidayService struct {
	dates map[timezone.Date]bool
	err   error
	calls int
	from  timezone.Date
	to    timezone.Date
}

func (f *fakeOverviewHolidayService) HolidaysInRange(_ context.Context, _, _ timezone.Date) ([]Holiday, error) {
	return nil, f.err
}

func (f *fakeOverviewHolidayService) HolidayDates(_ context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	f.calls++
	f.from, f.to = from, to
	return f.dates, f.err
}

type fakeOverviewWorkModelReader struct {
	models []*configModel.WorkTimeModel
	err    error
	ids    []int64
}

func (f *fakeOverviewWorkModelReader) FindByIDs(_ context.Context, ids []int64) ([]*configModel.WorkTimeModel, error) {
	f.ids = append([]int64(nil), ids...)
	return f.models, f.err
}

func TestStaffScheduleOverview_ReducesScheduleAndModelTargetsOnPublicHolidays(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.December, 21)
	friday := monday.AddDays(4)
	validFrom := monday.AddDays(-7)
	modelID := int64(22)
	holidayReader := &fakeOverviewHolidayService{dates: map[timezone.Date]bool{
		monday: true,
		friday: true,
	}}
	modelReader := &fakeOverviewWorkModelReader{models: []*configModel.WorkTimeModel{{
		ID:                 modelID,
		RotationLength:     1,
		RotationAnchorDate: workforceDate(monday),
		Entries: []*configModel.WorkTimeModelEntry{
			{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 240},
			{WeekIndex: 0, DayOfWeek: configModel.DayTuesday, TargetMinutes: 60},
		},
	}}}
	service := &staffScheduleOverviewService{deps: StaffScheduleOverviewDependencies{
		Holidays:   holidayReader,
		WorkModels: modelReader,
	}}
	scheduledStaff := &users.Staff{}
	scheduledStaff.ID = 11
	modelStaff := &users.Staff{WorkTimeModelID: &modelID}
	modelStaff.ID = 12

	targets, err := service.resolveWeeklyTargets(
		context.Background(),
		[]*users.Staff{scheduledStaff, modelStaff},
		[]*configModel.StaffWorkSchedule{
			{StaffID: 11, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayMonday, TargetMinutes: 120, ValidFrom: workforceDate(validFrom)},
			{StaffID: 11, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayFriday, TargetMinutes: 180, ValidFrom: workforceDate(validFrom)},
		},
		[]timezone.Date{monday},
	)
	require.NoError(t, err)

	assert.Equal(t, 0, targets[staffDateKey{staffID: 11, date: monday}], "both scheduled workdays are public holidays")
	assert.Equal(t, 60, targets[staffDateKey{staffID: 12, date: monday}], "the model's Monday target is removed")
	assert.Equal(t, []int64{modelID}, modelReader.ids)
	assert.Equal(t, monday, holidayReader.from)
	assert.Equal(t, monday.AddDays(6), holidayReader.to)
}

func TestStaffScheduleOverview_HolidayRangeUsesAllWeeksAndPropagatesErrors(t *testing.T) {
	t.Parallel()

	middle := timezone.NewDate(2026, time.December, 21)
	earlier := middle.AddDays(-7)
	later := middle.AddDays(7)

	t.Run("uses earliest Monday through latest Sunday", func(t *testing.T) {
		reader := &fakeOverviewHolidayService{dates: map[timezone.Date]bool{middle: true}}
		service := &staffScheduleOverviewService{deps: StaffScheduleOverviewDependencies{Holidays: reader}}

		set, err := service.holidayDatesForWeeks(context.Background(), []timezone.Date{middle, earlier, later})
		require.NoError(t, err)
		assert.True(t, set[middle])
		assert.Equal(t, earlier, reader.from)
		assert.Equal(t, later.AddDays(6), reader.to)
	})

	t.Run("wraps resolver errors", func(t *testing.T) {
		reader := &fakeOverviewHolidayService{err: errors.New("boom")}
		service := &staffScheduleOverviewService{deps: StaffScheduleOverviewDependencies{Holidays: reader}}

		_, err := service.holidayDatesForWeeks(context.Background(), []timezone.Date{middle})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load public holidays")

		_, err = service.resolveWeeklyTargets(context.Background(), nil, nil, []timezone.Date{middle})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load public holidays")
	})

	t.Run("skips lookup without weeks", func(t *testing.T) {
		reader := &fakeOverviewHolidayService{}
		service := &staffScheduleOverviewService{deps: StaffScheduleOverviewDependencies{Holidays: reader}}

		set, err := service.holidayDatesForWeeks(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, set)
		assert.Zero(t, reader.calls)
	})
}

func TestStaffScheduleOverview_BatchesAndProjectsEffectiveDailyAssignments(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	mainRoomID, overrideRoomID := int64(21), int64(22)
	instances := &fakeInstanceReader{rows: []*scheduleModel.ActivityInstance{
		{Date: scheduleModel.Date(date), Title: "Lernzeit", StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), RoomID: mainRoomID, Status: scheduleModel.InstanceStatusPlanned},
		{Date: scheduleModel.Date(date), Title: "Abgesagt", StartTime: testClock(t, "13:00"), EndTime: testClock(t, "14:00"), RoomID: mainRoomID, Status: scheduleModel.InstanceStatusCancelled},
	}}
	instances.rows[0].ID = 31
	instances.rows[1].ID = 32
	reason := "krank"
	assignmentRows := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{InstanceID: 31, StaffID: 1, RoomID: &overrideRoomID, IsAbsent: true, AbsenceReason: &reason},
		{InstanceID: 31, StaffID: 2, IsSubstitute: true},
		{InstanceID: 31, StaffID: 3},
		{InstanceID: 32, StaffID: 1},
	}}
	shifts := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		testShift(t, 2, date, "08:00", "12:00"),
		testShift(t, 3, date, "08:00", "10:00"),
		testShift(t, 3, date, "10:00", "12:00"),
	}}
	rooms := &fakeRoomReader{rows: []*facilities.Room{
		{Name: "Hauptraum"},
		{Name: "Nebenraum"},
	}}
	rooms.rows[0].ID = mainRoomID
	rooms.rows[1].ID = overrideRoomID
	staff := &fakeStaffReader{rows: []*users.Staff{
		fakeStaff(3, "Clara", "Zulu"),
		fakeStaff(1, "Anna", "Alpha"),
		fakeStaff(2, "Berta", "Mitte"),
	}}
	staff.rows[0].ID = 3
	staff.rows[1].ID = 1
	staff.rows[2].ID = 2

	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts: shifts, Instances: instances, InstanceStaff: assignmentRows, Rooms: rooms, Staff: staff,
	})
	got, err := service.GetOverview(context.Background(), date, date.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Assignments, 3, "cancelled instance assignments must be omitted")
	assert.True(t, got.DienstplanInUse)
	assert.Equal(t, []int64{31}, assignmentRows.ids)
	assert.ElementsMatch(t, []int64{mainRoomID, overrideRoomID}, rooms.ids)
	assert.Equal(t, []int64{1, 2, 3}, []int64{got.Staff[0].ID, got.Staff[1].ID, got.Staff[2].ID})

	absent := got.Assignments[0]
	assert.Equal(t, overrideRoomID, absent.RoomID)
	assert.Equal(t, "Nebenraum", absent.RoomName)
	assert.Equal(t, CoverageStatusNotApplicable, absent.CoverageStatus)
	require.NotNil(t, absent.CoverageReason)
	assert.Equal(t, CoverageReasonAbsent, *absent.CoverageReason)

	substitute := got.Assignments[1]
	assert.True(t, substitute.IsSubstitute)
	assert.Equal(t, CoverageStatusCovered, substitute.CoverageStatus, "an active substitute uses their own covering shift")
	assert.Empty(t, substitute.UncoveredIntervals)
	assert.Equal(t, CoverageStatusCovered, got.Assignments[2].CoverageStatus)

	assert.Equal(t, 2, shifts.calls)
	assert.Equal(t, 1, instances.calls)
	assert.Equal(t, 1, assignmentRows.calls)
	assert.Equal(t, 1, rooms.calls)
	assert.Equal(t, 1, staff.listCalls)
}

func TestStaffScheduleOverview_UsesTenantShiftWeeksPerISOWeek(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.July, 6)
	nextMonday := monday.AddDays(7)
	instances := &fakeInstanceReader{rows: []*scheduleModel.ActivityInstance{
		{Date: scheduleModel.Date(monday), Title: "Mensa", StartTime: testClock(t, "12:00"), EndTime: testClock(t, "13:00"), Status: scheduleModel.InstanceStatusPlanned},
		{Date: scheduleModel.Date(nextMonday), Title: "Lernzeit", StartTime: testClock(t, "12:00"), EndTime: testClock(t, "13:00"), Status: scheduleModel.InstanceStatusPlanned},
	}}
	instances.rows[0].ID = 41
	instances.rows[1].ID = 42
	shifts := &fakeShiftReader{usedWeeks: []timezone.Date{monday}}
	staff := fakeStaff(1, "Ada", "Lovelace")
	staff.ID = 1
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:    shifts,
		Instances: instances,
		InstanceStaff: &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
			{InstanceID: 41, StaffID: 1}, {InstanceID: 42, StaffID: 1},
		}},
		Rooms: &fakeRoomReader{},
		Staff: &fakeStaffReader{rows: []*users.Staff{staff}},
	})

	got, err := service.GetOverview(context.Background(), monday, nextMonday)
	require.NoError(t, err)
	require.Len(t, got.Assignments, 2)
	assert.Equal(t, CoverageStatusUncovered, got.Assignments[0].CoverageStatus, "a weekend-only tenant shift activates the Monday ISO week")
	assert.Equal(t, CoverageStatusNotApplicable, got.Assignments[1].CoverageStatus, "week-one usage must not leak into week two")
	assert.Equal(t, CoverageReasonDienstplanNotUsed, *got.Assignments[1].CoverageReason)
	assert.Equal(t, monday, shifts.from)
	assert.Equal(t, nextMonday.AddDays(6), shifts.to)
}

func TestDetectShiftCoverage_BoundsReturnedWarningDetails(t *testing.T) {
	t.Parallel()

	start := timezone.NewDate(2026, time.January, 5)
	dates := make([]timezone.Date, 130)
	weeks := make([]timezone.Date, 0, 20)
	seenWeeks := make(map[timezone.Date]bool)
	for i := range dates {
		dates[i] = start.AddDays(i)
		week, _ := containingCalendarWeek(dates[i])
		if !seenWeeks[week] {
			seenWeeks[week] = true
			weeks = append(weeks, week)
		}
	}
	staff := fakeStaff(1, "Ada", "Lovelace")
	staff.ID = 1
	result, err := DetectShiftCoverage(context.Background(), ShiftCoverageDependencies{
		Shifts: &fakeShiftReader{usedWeeks: weeks},
		Staff:  &fakeStaffReader{byID: map[int64]*users.Staff{1: staff}},
	}, ShiftCoverageQuery{
		Dates: dates, StartTime: testClock(t, "12:00"), EndTime: testClock(t, "13:00"), StaffIDs: []int64{1},
	})
	require.NoError(t, err)
	assert.Equal(t, 130, result.TotalWarningCount)
	assert.Len(t, result.Warnings, maxReturnedShiftCoverageWarnings)
}

func TestStaffScheduleOverview_TenantWeekWithoutAnyShiftSuppressesWarnings(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	instance := &scheduleModel.ActivityInstance{
		Date: scheduleModel.Date(date), Title: "AG", StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), RoomID: 2, Status: scheduleModel.InstanceStatusPlanned,
	}
	instance.ID = 8
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts:        &fakeShiftReader{},
		Instances:     &fakeInstanceReader{rows: []*scheduleModel.ActivityInstance{instance}},
		InstanceStaff: &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{{InstanceID: 8, StaffID: 4}}},
		Rooms:         &fakeRoomReader{rows: []*facilities.Room{{Name: "Raum"}}},
		Staff:         &fakeStaffReader{},
	})
	got, err := service.GetOverview(context.Background(), date, date.AddDays(4))
	require.NoError(t, err)
	require.Len(t, got.Assignments, 1)
	assert.False(t, got.DienstplanInUse)
	assert.Equal(t, CoverageStatusNotApplicable, got.Assignments[0].CoverageStatus)
	require.NotNil(t, got.Assignments[0].CoverageReason)
	assert.Equal(t, CoverageReasonDienstplanNotUsed, *got.Assignments[0].CoverageReason)
	assert.Empty(t, got.Assignments[0].UncoveredIntervals)
}

func TestStaffScheduleOverview_PropagatesReadErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("database unavailable")
	service := NewStaffScheduleOverviewService(StaffScheduleOverviewDependencies{
		Shifts: &fakeShiftReader{err: want},
	})
	_, err := service.GetOverview(context.Background(), timezone.NewDate(2026, time.July, 6), timezone.NewDate(2026, time.July, 10))
	require.ErrorIs(t, err, want)
}

func DetectShiftCoverageWarnings(ctx context.Context, deps ShiftCoverageDependencies, query ShiftCoverageQuery) ([]ShiftCoverageWarning, error) {
	result, err := DetectShiftCoverage(ctx, deps, query)
	return result.Warnings, err
}

func TestDetectShiftCoverageWarnings_EffectiveRosterAndContainingWeek(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 8) // Wednesday
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// A shift anywhere in the tenant activates warnings for the week.
		testShift(t, 9, date, "07:00", "08:00"),
	}}
	instanceStaff := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 1, IsAbsent: true},
		{StaffID: 2, IsSubstitute: true},
		{StaffID: 3, IsSubstitute: true, IsAbsent: true},
	}}
	staff := &fakeStaffReader{byID: map[int64]*users.Staff{2: fakeStaff(2, "Ersatz", "Person")}}
	excludeID := int64(42)
	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shiftReader, InstanceStaff: instanceStaff, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
	}, ShiftCoverageQuery{
		Dates: []timezone.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "11:00"),
		StaffIDs: []int64{1, 2, 3}, ExcludeInstanceID: &excludeID,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1, "only the active substitute is effective")
	assert.Equal(t, int64(2), warnings[0].StaffID)
	assert.Equal(t, "Ersatz Person", warnings[0].StaffName)
	assert.Equal(t, "09:00", warnings[0].UncoveredStartTime)
	assert.Equal(t, "11:00", warnings[0].UncoveredEndTime)
	assert.Equal(t, timezone.NewDate(2026, time.July, 6), shiftReader.from)
	assert.Equal(t, timezone.NewDate(2026, time.July, 12), shiftReader.to)
	assert.Equal(t, 1, instanceStaff.calls)
	assert.Equal(t, 2, shiftReader.calls)
	assert.Equal(t, 1, staff.findCalls)
}

func TestDetectShiftCoverageWarnings_ChangedRosterDropsPriorAbsence(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{testShift(t, 9, date, "07:00", "08:00")}}
	instanceStaff := &fakeInstanceStaffReader{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 1, IsAbsent: true}, {StaffID: 2, IsSubstitute: true},
	}}
	staff := &fakeStaffReader{byID: map[int64]*users.Staff{
		1: fakeStaff(1, "Wieder", "Eingeplant"), 4: fakeStaff(4, "Neu", "Dabei"),
	}}
	excludeID := int64(42)
	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shiftReader, InstanceStaff: instanceStaff, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
	}, ShiftCoverageQuery{
		Dates: []timezone.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"),
		StaffIDs: []int64{1, 4}, ExcludeInstanceID: &excludeID,
	})
	require.NoError(t, err)
	require.Len(t, warnings, 2, "changed membership makes both submitted IDs effective")
	assert.Equal(t, []int64{1, 4}, []int64{warnings[0].StaffID, warnings[1].StaffID})
	assert.Equal(t, 1, instanceStaff.calls, "concrete roster must be loaded in one batch")
	assert.Equal(t, 2, shiftReader.calls, "relevant shifts and tenant week usage each use one batched read")
	assert.Equal(t, 1, staff.findCalls, "all warning names must be loaded in one batch")
}

func TestDetectShiftCoverageWarnings_ConvertedInstanceSurvivesRecurrenceFiltering(t *testing.T) {
	t.Parallel()

	weekA := timezone.NewDate(2026, time.July, 6)
	concreteWeekB := weekA.AddDays(7)
	anchor := weekA
	scheduleAnchor := scheduleModel.Date(anchor)
	period := &scheduleModel.CalendarPeriod{
		StartDate: scheduleModel.Date(weekA), EndDate: scheduleModel.Date(concreteWeekB.AddDays(6)),
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
	staff := &fakeStaffReader{byID: map[int64]*users.Staff{2: fakeStaff(2, "Ersatz", "Person")}}

	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shifts, InstanceStaff: instanceStaff, Staff: staff,
		CalendarPeriods: &fakeCalendarPeriodReader{period: period},
	}, ShiftCoverageQuery{
		Dates:     []timezone.Date{weekA, concreteWeekB},
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

func TestDetectShiftCoverageWarnings_GapAndTenantUnusedCases(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	staff := &fakeStaffReader{byID: map[int64]*users.Staff{1: fakeStaff(1, "Max", "Mustermann")}}

	t.Run("two shifts with gap", func(t *testing.T) {
		warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
			Shifts: &fakeShiftReader{rows: []*scheduleModel.StaffShift{
				testShift(t, 1, date, "08:00", "09:00"), testShift(t, 1, date, "10:00", "12:00"),
			}},
			InstanceStaff: &fakeInstanceStaffReader{}, Staff: staff, CalendarPeriods: &fakeCalendarPeriodReader{},
		}, ShiftCoverageQuery{Dates: []timezone.Date{date}, StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), StaffIDs: []int64{1}})
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
		warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
			Shifts: &fakeShiftReader{}, InstanceStaff: &fakeInstanceStaffReader{}, Staff: nameReader, CalendarPeriods: &fakeCalendarPeriodReader{},
		}, ShiftCoverageQuery{Dates: []timezone.Date{date}, StartTime: testClock(t, "08:00"), EndTime: testClock(t, "12:00"), StaffIDs: []int64{1}})
		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.Zero(t, nameReader.findCalls, "unused-week suppression must not perform name lookups")
	})
}

func TestDetectShiftCoverageWarnings_RecurringDatesReusePeriodAndABEngineWithFixedReads(t *testing.T) {
	t.Parallel()

	weekA := timezone.NewDate(2026, time.July, 6)
	weekB := weekA.AddDays(7)
	nextWeekA := weekA.AddDays(14)
	anchor := weekA
	scheduleAnchor := scheduleModel.Date(anchor)
	period := &scheduleModel.CalendarPeriod{
		StartDate: scheduleModel.Date(weekA), EndDate: scheduleModel.Date(nextWeekA.AddDays(6)),
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
	staff := &fakeStaffReader{byID: map[int64]*users.Staff{1: fakeStaff(1, "Serie", "Person")}}
	weekPattern := 1
	periodID := period.ID

	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shiftReader, InstanceStaff: instanceStaff, Staff: staff, CalendarPeriods: periodReader,
	}, ShiftCoverageQuery{
		Dates: []timezone.Date{
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
	assert.Equal(t, 2, shiftReader.calls, "all relevant dates share one shift read plus one week-usage read")
	assert.Equal(t, 1, staff.findCalls, "all names must share one batch read")
	assert.Zero(t, instanceStaff.calls, "series probes have no concrete deviation read")
	assert.Equal(t, weekA, shiftReader.from)
	assert.Equal(t, nextWeekA.AddDays(6), shiftReader.to)
}

func TestDetectShiftCoverageWarnings_SeriesReplanUsesEffectiveExceptionsAndDeviations(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.July, 6)
	wednesday := monday.AddDays(2)
	friday := monday.AddDays(4)
	groupID := int64(71)
	periodID := int64(72)
	weekPattern := 0
	period := &scheduleModel.CalendarPeriod{StartDate: scheduleModel.Date(monday), EndDate: scheduleModel.Date(friday), WeekCycleLength: 1, IsActive: true}
	period.ID = periodID

	mondayInstance := &scheduleModel.ActivityInstance{
		Date: scheduleModel.Date(monday), ActivityGroupID: &groupID, Status: scheduleModel.InstanceStatusPlanned,
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
		{ActivityGroupID: groupID, ExceptionDate: scheduleModel.Date(wednesday), ExceptionType: scheduleModel.ActivityExceptionCancelled},
		{ActivityGroupID: groupID, ExceptionDate: scheduleModel.Date(friday), ExceptionType: scheduleModel.ActivityExceptionModified, StartTime: &modifiedStart},
	}}
	shifts := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// Monday: the active substitute is fully covered by their own shift.
		testShift(t, 2, monday, "09:00", "11:00"),
		// Friday: the start-only exception overlays 10:00 onto the proposed
		// 09:00-11:00 window, leaving exactly 10:30-11:00 uncovered.
		testShift(t, 1, friday, "09:00", "10:30"),
	}}
	staff := &fakeStaffReader{byID: map[int64]*users.Staff{
		1: fakeStaff(1, "Plan", "Person"),
		2: fakeStaff(2, "Ersatz", "Person"),
	}}

	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shifts, Instances: instances, Exceptions: exceptions,
		Schedules:     &fakeActivityScheduleReader{rows: []*activitiesModel.Schedule{{ActivityGroupID: groupID}}},
		InstanceStaff: instanceStaff, Staff: staff,
		CalendarPeriods: &fakeCalendarPeriodReader{period: period},
	}, ShiftCoverageQuery{
		Dates:     []timezone.Date{monday, wednesday, friday},
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
	assert.Equal(t, 2, shifts.calls)
	assert.Equal(t, 1, staff.findCalls)
}

func TestDetectShiftCoverageWarnings_UsesCanonicalSeriesBoundsAndActivePeriod(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.July, 6)
	wednesday := monday.AddDays(2)
	nextMonday := monday.AddDays(7)
	groupID, periodID, pattern := int64(91), int64(92), 0
	validFrom, validUntil := activitiesModel.Date(wednesday), activitiesModel.Date(nextMonday)
	schedules := &fakeActivityScheduleReader{rows: []*activitiesModel.Schedule{{
		ActivityGroupID: groupID,
		ValidFrom:       &validFrom,
		ValidUntil:      &validUntil,
	}}}
	period := &scheduleModel.CalendarPeriod{
		StartDate: scheduleModel.Date(monday), EndDate: scheduleModel.Date(nextMonday), WeekCycleLength: 1, IsActive: true,
	}
	period.ID = periodID
	shifts := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// An unrelated shift activates the week; the assigned person has none.
		testShift(t, 9, monday, "07:00", "08:00"),
	}}

	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shifts, Instances: &fakeInstanceReader{}, Exceptions: &fakeActivityExceptionReader{},
		Schedules: schedules, InstanceStaff: &fakeInstanceStaffReader{},
		Staff:           &fakeStaffReader{byID: map[int64]*users.Staff{1: fakeStaff(1, "Grenze", "Serie")}},
		CalendarPeriods: &fakeCalendarPeriodReader{period: period},
	}, ShiftCoverageQuery{
		Dates:     []timezone.Date{monday, wednesday, nextMonday},
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
	warnings, err = DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shiftReader, Instances: &fakeInstanceReader{}, Exceptions: &fakeActivityExceptionReader{},
		Schedules: schedules, InstanceStaff: &fakeInstanceStaffReader{}, Staff: &fakeStaffReader{},
		CalendarPeriods: &fakeCalendarPeriodReader{period: &inactive},
	}, ShiftCoverageQuery{
		Dates:     []timezone.Date{wednesday},
		StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
		ReplanActivityGroupID: &groupID, CalendarPeriodID: &periodID, WeekPattern: &pattern,
	})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Zero(t, shiftReader.calls, "inactive periods cannot materialize and need no shift read")
}

func TestDetectShiftCoverageWarnings_IncludesWeekendShifts(t *testing.T) {
	t.Parallel()

	saturday := timezone.NewDate(2026, time.July, 11)
	shiftReader := &fakeShiftReader{rows: []*scheduleModel.StaffShift{
		// A weekend-only shift activates the tenant's calendar week.
		testShift(t, 9, saturday, "07:00", "08:00"),
	}}
	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: shiftReader, InstanceStaff: &fakeInstanceStaffReader{},
		Staff:           &fakeStaffReader{byID: map[int64]*users.Staff{1: fakeStaff(1, "Samstag", "Dienst")}},
		CalendarPeriods: &fakeCalendarPeriodReader{},
	}, ShiftCoverageQuery{
		Dates: []timezone.Date{saturday}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
	})
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, saturday.String(), warnings[0].Date)
	assert.Equal(t, timezone.NewDate(2026, time.July, 6), shiftReader.from)
	assert.Equal(t, timezone.NewDate(2026, time.July, 12), shiftReader.to)
}

func TestNormalizeShiftCoverageQuery_ValidationBounds(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.January, 1)
	valid := ShiftCoverageQuery{
		Dates: []timezone.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
	}
	tests := []struct {
		name   string
		mutate func(*ShiftCoverageQuery)
	}{
		{name: "dates required", mutate: func(query *ShiftCoverageQuery) { query.Dates = nil }},
		{name: "calendar date required", mutate: func(query *ShiftCoverageQuery) { query.Dates = []timezone.Date{""} }},
		{name: "date count bounded", mutate: func(query *ShiftCoverageQuery) {
			query.Dates = make([]timezone.Date, 367)
			for day := range query.Dates {
				query.Dates[day] = date.AddDays(day)
			}
		}},
		{name: "span bounded", mutate: func(query *ShiftCoverageQuery) { query.Dates = append(query.Dates, date.AddDays(367)) }},
		{name: "end after start", mutate: func(query *ShiftCoverageQuery) { query.EndTime = query.StartTime }},
		{name: "staff required", mutate: func(query *ShiftCoverageQuery) { query.StaffIDs = nil }},
		{name: "positive staff", mutate: func(query *ShiftCoverageQuery) { query.StaffIDs = []int64{0} }},
		{name: "staff count bounded", mutate: func(query *ShiftCoverageQuery) {
			query.StaffIDs = make([]int64, maxShiftCoverageStaffIDs+1)
			for index := range query.StaffIDs {
				query.StaffIDs[index] = int64(index + 1)
			}
		}},
		{name: "date staff product bounded", mutate: func(query *ShiftCoverageQuery) {
			query.Dates = make([]timezone.Date, 21)
			for day := range query.Dates {
				query.Dates[day] = date.AddDays(day)
			}
			query.StaffIDs = make([]int64, 500)
			for index := range query.StaffIDs {
				query.StaffIDs[index] = int64(index + 1)
			}
		}},
		{name: "exclude positive", mutate: func(query *ShiftCoverageQuery) { zero := int64(0); query.ExcludeInstanceID = &zero }},
		{name: "multi-date exclude requires concrete date", mutate: func(query *ShiftCoverageQuery) {
			id := int64(2)
			query.ExcludeInstanceID = &id
			query.Dates = append(query.Dates, date.AddDays(1))
		}},
		{name: "concrete date requires instance", mutate: func(query *ShiftCoverageQuery) { query.ConcreteInstanceDate = &date }},
		{name: "concrete date must be a candidate", mutate: func(query *ShiftCoverageQuery) {
			id, other := int64(2), date.AddDays(1)
			query.ExcludeInstanceID, query.ConcreteInstanceDate = &id, &other
		}},
		{name: "replan group requires recurrence", mutate: func(query *ShiftCoverageQuery) { id := int64(2); query.ReplanActivityGroupID = &id }},
		{name: "replan group and instance are exclusive", mutate: func(query *ShiftCoverageQuery) {
			instanceID, groupID, periodID, pattern := int64(2), int64(3), int64(4), 0
			query.ExcludeInstanceID, query.ReplanActivityGroupID = &instanceID, &groupID
			query.CalendarPeriodID, query.WeekPattern = &periodID, &pattern
		}},
		{name: "recurrence pair", mutate: func(query *ShiftCoverageQuery) { id := int64(2); query.CalendarPeriodID = &id }},
		{name: "period positive", mutate: func(query *ShiftCoverageQuery) {
			zero, pattern := int64(0), 1
			query.CalendarPeriodID, query.WeekPattern = &zero, &pattern
		}},
		{name: "week pattern range", mutate: func(query *ShiftCoverageQuery) {
			id, pattern := int64(2), 3
			query.CalendarPeriodID, query.WeekPattern = &id, &pattern
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := valid
			query.Dates = append([]timezone.Date(nil), valid.Dates...)
			query.StaffIDs = append([]int64(nil), valid.StaffIDs...)
			test.mutate(&query)
			_, _, err := normalizeShiftCoverageQuery(query)
			require.ErrorIs(t, err, ErrInvalidShiftCoverageQuery)
		})
	}

	t.Run("dedupes and admits full school year", func(t *testing.T) {
		dates := make([]timezone.Date, 0, 367)
		for day := 0; day < 366; day++ {
			dates = append(dates, date.AddDays(day))
		}
		dates = append(dates, date)
		query := valid
		query.Dates = dates
		normalized, staffIDs, err := normalizeShiftCoverageQuery(query)
		require.NoError(t, err)
		assert.Len(t, normalized, 366)
		assert.Equal(t, []int64{1}, staffIDs)
	})
}

func TestDetectShiftCoverageWarnings_MissingTenantPeriodIsValidationError(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	periodID, weekPattern := int64(999), 1
	periodReader := &fakeCalendarPeriodReader{err: sql.ErrNoRows}
	warnings, err := DetectShiftCoverageWarnings(context.Background(), ShiftCoverageDependencies{
		Shifts: &fakeShiftReader{}, InstanceStaff: &fakeInstanceStaffReader{}, Staff: &fakeStaffReader{}, CalendarPeriods: periodReader,
	}, ShiftCoverageQuery{
		Dates: []timezone.Date{date}, StartTime: testClock(t, "09:00"), EndTime: testClock(t, "10:00"), StaffIDs: []int64{1},
		CalendarPeriodID: &periodID, WeekPattern: &weekPattern,
	})
	require.ErrorIs(t, err, ErrInvalidShiftCoverageQuery)
	assert.Nil(t, warnings)
	assert.Equal(t, 1, periodReader.calls)
}
