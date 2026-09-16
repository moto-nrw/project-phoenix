package shiftplanning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services/schedule"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

func formattedGaps(gaps []schedule.ShiftCoverageInterval) [][2]string {
	out := make([][2]string, 0, len(gaps))
	for _, gap := range gaps {
		out = append(out, [2]string{
			timezone.NormalizeWallClock(gap.StartTime).Format("15:04"),
			timezone.NormalizeWallClock(gap.EndTime).Format("15:04"),
		})
	}
	return out
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

func (f *fakeOverviewHolidayService) HolidaysInRange(_ context.Context, _, _ timezone.Date) ([]schedule.Holiday, error) {
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

	assert.Equal(t, 0, targets[staffDateKey{StaffID: 11, Date: monday}], "both scheduled workdays are public holidays")
	assert.Equal(t, 60, targets[staffDateKey{StaffID: 12, Date: monday}], "the model's Monday target is removed")
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
