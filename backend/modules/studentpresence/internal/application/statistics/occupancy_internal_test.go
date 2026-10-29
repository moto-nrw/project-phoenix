package statistics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func clock(t *testing.T, hour, minute int) *calendar.WallClock {
	t.Helper()
	c, err := calendar.NewWallClock(hour, minute, 0, 0)
	require.NoError(t, err)
	return &c
}

func berlinAt(day calendar.Date, hour, minute int) time.Time {
	return day.BerlinMidnight().Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

func occupancyStudent(id int64, class string) *ports.StatisticsStudent {
	return &ports.StatisticsStudent{ID: id, SchoolClass: class, EnrolledOn: enrolledEveryDay}
}

// slotAt returns the cell starting at hour:minute.
func slotAt(t *testing.T, weekday studentpresence.StatisticsOccupancyWeekday, hour, minute int) studentpresence.StatisticsOccupancySlot {
	t.Helper()
	for _, slot := range weekday.Slots {
		if slot.StartMinute == hour*60+minute {
			return slot
		}
	}
	t.Fatalf("no slot at %02d:%02d", hour, minute)
	return studentpresence.StatisticsOccupancySlot{}
}

// A window [07:45, 16:00) touches the 07:30 slot but not the 16:00 slot;
// [08:00, 08:30) touches exactly one slot.
func TestBuildOccupancy_SlotBoundaries(t *testing.T) {
	t.Parallel()
	wed := calendar.NewDate(2026, 8, 19)
	out := buildOccupancy(occupancyInputs{
		students: []*ports.StatisticsStudent{occupancyStudent(1, "1a"), occupancyStudent(2, "1a")},
		careDays: map[calendar.Date]bool{wed: true},
		planned: map[int64]map[calendar.Date]ports.PlannedCareDay{
			1: {wed: {Expected: true, Arrival: clock(t, 7, 45), Pickup: clock(t, 16, 0)}},
			2: {wed: {Expected: true, Arrival: clock(t, 8, 0), Pickup: clock(t, 8, 30)}},
		},
		actualFrom: wed.AddDays(-30),
		today:      wed.AddDays(1),
	})

	require.Len(t, out.Weekdays, 1)
	day := out.Weekdays[0]
	assert.Equal(t, 3, day.Weekday)
	assert.Equal(t, 1, day.PlanDays)
	assert.Equal(t, 0, slotAt(t, day, 7, 0).PlannedMax)
	assert.Equal(t, 1, slotAt(t, day, 7, 30).PlannedMax, "07:45 arrival counts from the 07:30 slot on")
	assert.Equal(t, 2, slotAt(t, day, 8, 0).PlannedMax)
	assert.Equal(t, 1, slotAt(t, day, 8, 30).PlannedMax, "a pickup at 08:30 ends before the 08:30 slot")
	assert.Equal(t, 1, slotAt(t, day, 15, 30).PlannedMax)
	assert.Equal(t, 0, slotAt(t, day, 16, 0).PlannedMax, "a pickup at 16:00 does not count at 16:00")
	assert.Equal(t, 7*60, day.Slots[0].StartMinute, "default axis starts at 07:00")
	assert.Equal(t, 16*60+30, day.Slots[len(day.Slots)-1].StartMinute, "default axis ends with the 16:30 slot")
}

// Data before 07:00 or after 17:00 widens the axis instead of disappearing.
func TestBuildOccupancy_AxisWidensToData(t *testing.T) {
	t.Parallel()
	wed := calendar.NewDate(2026, 8, 19)
	out := buildOccupancy(occupancyInputs{
		students: []*ports.StatisticsStudent{occupancyStudent(1, "")},
		careDays: map[calendar.Date]bool{wed: true},
		planned: map[int64]map[calendar.Date]ports.PlannedCareDay{
			1: {wed: {Expected: true, Arrival: clock(t, 6, 30), Pickup: clock(t, 17, 15)}},
		},
		actualFrom: wed,
		today:      wed,
	})
	day := out.Weekdays[0]
	assert.Equal(t, 6*60+30, day.Slots[0].StartMinute)
	assert.Equal(t, 17*60, day.Slots[len(day.Slots)-1].StartMinute)
}

// Sick, excused and class-trip days and "Kommt heute nicht" are never
// planned; a planned day without a time pair is counted as untimed.
func TestBuildOccupancy_ExceptionsDoNotCountAsPlanned(t *testing.T) {
	t.Parallel()
	wed := calendar.NewDate(2026, 8, 19)
	window := func() ports.PlannedCareDay {
		return ports.PlannedCareDay{Expected: true, Arrival: clock(t, 8, 0), Pickup: clock(t, 15, 0)}
	}
	students := []*ports.StatisticsStudent{}
	for id := int64(1); id <= 7; id++ {
		students = append(students, occupancyStudent(id, "2b"))
	}
	out := buildOccupancy(occupancyInputs{
		students: students,
		careDays: map[calendar.Date]bool{wed: true},
		statusDays: []ports.StatusDay{
			{StudentID: 1, Date: wed, Status: statusSick},
			{StudentID: 2, Date: wed, Status: statusExcused},
			{StudentID: 3, Date: wed, Status: statusClassTrip},
		},
		planned: map[int64]map[calendar.Date]ports.PlannedCareDay{
			1: {wed: window()},
			2: {wed: window()},
			3: {wed: window()},
			4: {wed: {Expected: false}}, // Kommt heute nicht / not booked
			5: {wed: {Expected: true, Arrival: clock(t, 8, 0)}},
			6: {wed: window()},
			7: {wed: {Expected: true, Arrival: clock(t, 15, 0), Pickup: clock(t, 8, 0)}},
		},
		actualFrom: wed,
		today:      wed,
	})
	assert.Equal(t, 1, slotAt(t, out.Weekdays[0], 10, 0).PlannedMax, "only child 6 is planned")
	assert.Equal(t, 2, out.UntimedPlanDays, "missing pickup and pickup before arrival")
}

// Actual values: several rows of one child count once, an open row on today
// runs until now, an open row of an earlier day only fills its check-in slot,
// a row of a non-care day is ignored.
func TestBuildOccupancy_ActualWindows(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 8, 17)
	tue := calendar.NewDate(2026, 8, 18)
	sat := calendar.NewDate(2026, 8, 22)
	checkout := func(day calendar.Date, hour, minute int) *time.Time {
		at := berlinAt(day, hour, minute)
		return &at
	}
	out := buildOccupancy(occupancyInputs{
		students: []*ports.StatisticsStudent{occupancyStudent(1, ""), occupancyStudent(2, ""), occupancyStudent(3, "")},
		careDays: map[calendar.Date]bool{mon: true, tue: true},
		attendance: []*ports.Attendance{
			{StudentID: 1, Date: mon, CheckInTime: berlinAt(mon, 8, 0), CheckOutTime: checkout(mon, 9, 0)},
			{StudentID: 1, Date: mon, CheckInTime: berlinAt(mon, 8, 40), CheckOutTime: checkout(mon, 10, 0)},
			{StudentID: 2, Date: mon, CheckInTime: berlinAt(mon, 12, 10)},
			{StudentID: 3, Date: tue, CheckInTime: berlinAt(tue, 11, 0)},
			{StudentID: 3, Date: sat, CheckInTime: berlinAt(sat, 9, 0), CheckOutTime: checkout(sat, 12, 0)},
		},
		actualFrom: mon,
		today:      tue,
		now:        berlinAt(tue, 13, 5),
	})
	require.Len(t, out.Weekdays, 2)
	monday, tuesday := out.Weekdays[0], out.Weekdays[1]
	assert.Equal(t, 1, *slotAt(t, monday, 8, 30).ActualMax, "overlapping rows of one child count once")
	assert.Equal(t, 1, *slotAt(t, monday, 12, 0).ActualMax, "open row of an earlier day fills its check-in slot")
	assert.Equal(t, 0, *slotAt(t, monday, 12, 30).ActualMax)
	assert.Equal(t, 1, *slotAt(t, tuesday, 13, 0).ActualMax, "open row of today runs until now")
	assert.Equal(t, 0, *slotAt(t, tuesday, 13, 30).ActualMax)
}

// Days before the attendance retention cutoff carry no actual values: the
// average divides only by the days that still have data.
func TestBuildOccupancy_ActualStopsAtRetention(t *testing.T) {
	t.Parallel()
	mon1 := calendar.NewDate(2026, 8, 10)
	mon2 := calendar.NewDate(2026, 8, 17)
	tue2 := calendar.NewDate(2026, 8, 18)
	out := buildOccupancy(occupancyInputs{
		students: []*ports.StatisticsStudent{occupancyStudent(1, "")},
		careDays: map[calendar.Date]bool{mon1: true, mon2: true, tue2: true},
		attendance: []*ports.Attendance{
			{StudentID: 1, Date: mon2, CheckInTime: berlinAt(mon2, 9, 0), CheckOutTime: ptrTime(berlinAt(mon2, 10, 0))},
		},
		actualFrom: calendar.NewDate(2026, 8, 15),
		today:      calendar.NewDate(2026, 8, 25),
	})
	monday := out.Weekdays[0]
	assert.Equal(t, 2, monday.PlanDays)
	assert.Equal(t, 1, monday.ActualDays)
	require.NotNil(t, slotAt(t, monday, 9, 0).ActualAverage)
	assert.InDelta(t, 1.0, *slotAt(t, monday, 9, 0).ActualAverage, 0.001)
}

func ptrTime(t time.Time) *time.Time { return &t }

type plannedStub map[int64]map[calendar.Date]ports.PlannedCareDay

func (p plannedStub) PlannedCareDays(_ context.Context, ids []int64, _, _ calendar.Date) (map[int64]map[calendar.Date]ports.PlannedCareDay, error) {
	out := map[int64]map[calendar.Date]ports.PlannedCareDay{}
	for _, id := range ids {
		if days, ok := p[id]; ok {
			out[id] = days
		}
	}
	return out, nil
}

type attendanceRowsStub struct {
	rows    []*ports.Attendance
	filters []studentpresence.AttendanceFilter
}

func (a *attendanceRowsStub) ListAttendance(_ context.Context, filter studentpresence.AttendanceFilter) ([]*ports.Attendance, ports.Stats, error) {
	a.filters = append(a.filters, filter)
	return a.rows, ports.Stats{}, nil
}

func (a *attendanceRowsStub) ListAttendanceDays(context.Context, string, string) ([]studentpresence.AttendanceDay, error) {
	return nil, nil
}

type noStatusDays struct{}

func (noStatusDays) StatusDays(context.Context, calendar.Date, calendar.Date) ([]ports.StatusDay, error) {
	return nil, nil
}

type occupancyStudents []*ports.StatisticsStudent

func (s occupancyStudents) FindOverlappingWithGroups(context.Context, calendar.Date, calendar.Date, calendar.Date) ([]*ports.StatisticsStudent, error) {
	return s, nil
}

// A window across the autumn holidays: the holiday week is no care day, so
// the Monday average divides by the two school Mondays only. The class
// filter narrows plan and actual alike, and the attendance read starts at the
// retention cutoff.
func TestReport_OccupancyAcrossHolidays(t *testing.T) {
	t.Parallel()
	from := calendar.NewDate(2026, 8, 3) // Monday
	to := calendar.NewDate(2026, 8, 21)  // Friday, three weeks
	holidayMon := calendar.NewDate(2026, 8, 10)
	mon1, mon3 := from, calendar.NewDate(2026, 8, 17)
	plan := func(hour int) ports.PlannedCareDay {
		return ports.PlannedCareDay{Expected: true, Arrival: clock(t, hour, 0), Pickup: clock(t, 16, 0)}
	}
	planned := plannedStub{
		1: {mon1: plan(8), holidayMon: plan(8), mon3: plan(8)},
		2: {mon1: plan(8), mon3: plan(12)},
		3: {mon1: plan(8), mon3: plan(8)}, // class 4c, filtered out
	}
	attendance := &attendanceRowsStub{}
	svc := NewService(Config{
		Students:            occupancyStudents{occupancyStudent(1, "1a"), occupancyStudent(2, "1a"), occupancyStudent(3, "4c")},
		Periods:             periodList{{StartDate: holidayMon, EndDate: calendar.NewDate(2026, 8, 14)}},
		Attendance:          attendance,
		StatusDays:          noStatusDays{},
		PlannedCareDays:     planned,
		AttendanceIntervals: attendance,
		Retention:           reportRetention{},
		AccessLog:           &reportAudit{},
		Now:                 fixedNow, // 2026-08-25
	})
	report, err := svc.Report(context.Background(), studentpresence.StatisticsFilters{
		From: from, To: to, SchoolClasses: []string{"1a"},
		Sections: []studentpresence.StatisticsSection{studentpresence.StatisticsSectionOccupancy},
	}, studentpresence.StatisticsActor{AccountID: 1})
	require.NoError(t, err)
	require.NotNil(t, report.Occupancy)

	occupancy := report.Occupancy
	require.Len(t, occupancy.Weekdays, 5)
	monday := occupancy.Weekdays[0]
	assert.Equal(t, 1, monday.Weekday)
	assert.Equal(t, 2, monday.PlanDays, "the holiday Monday is no care day")
	assert.InDelta(t, 1.5, slotAt(t, monday, 8, 0).PlannedAverage, 0.001, "child 2 comes at 12:00 on the second Monday")
	assert.Equal(t, 2, slotAt(t, monday, 8, 0).PlannedMax)
	assert.InDelta(t, 1.5, slotAt(t, monday, 11, 30).PlannedAverage, 0.001)
	assert.InDelta(t, 2.0, slotAt(t, monday, 12, 0).PlannedAverage, 0.001)
	assert.Equal(t, 365, occupancy.ActualDataDays)
	assert.Equal(t, calendar.NewDate(2025, 8, 25), occupancy.ActualDataFrom)

	require.Len(t, attendance.filters, 1)
	assert.Equal(t, from.String(), attendance.filters[0].FromDate)
	assert.ElementsMatch(t, []int64{1, 2}, attendance.filters[0].StudentIDs, "the class filter narrows the attendance read")
	assert.Nil(t, report.Students, "the attendance table was not asked for")
}
