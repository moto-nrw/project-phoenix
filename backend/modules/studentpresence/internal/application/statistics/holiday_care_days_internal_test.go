package statistics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A confirmed holiday booking decides the Soll of the days it covers
// (#5994): one Soll and one comparison per child and day, never the
// contract's and the holiday booking's together.

type holidayStub struct {
	days map[int64]map[calendar.Date]ports.HolidayCareDay
	err  error
}

func (h holidayStub) HolidayCareDays(context.Context, []int64, calendar.Date, calendar.Date) (map[int64]map[calendar.Date]ports.HolidayCareDay, error) {
	return h.days, h.err
}

func holidayBooked(startHour, endHour int) ports.HolidayCareDay {
	return ports.HolidayCareDay{Arrival: *wallClockAt(startHour, 0), Pickup: *wallClockAt(endHour, 0)}
}

func contractDay(end *calendar.WallClock) ports.PlannedCareDay {
	return ports.PlannedCareDay{Expected: true, Arrival: wallClockAt(8, 0), Pickup: end, Source: studentpresence.PlannedCareSourceContract}
}

// holidayWeek is a child whose contract books Monday and Tuesday until
// 13:00 and whose holiday booking covers Monday from 08:00 to 16:00.
func holidayWeek(holidays ports.HolidayCareDays, rows ...*ports.Attendance) (Config, calendar.Date) {
	const child = int64(11)
	mon := calendar.NewDate(2026, 7, 20)
	cfg := Config{
		Students: careTimeStudents{{ID: child, FirstName: "Ida", LastName: "Ferien", EnrolledOn: enrolledEveryDay}},
		PlannedCareDays: WithHolidayCareDays(plannedStub{child: {
			mon: contractDay(wallClockAt(13, 0)), mon.AddDays(1): contractDay(wallClockAt(13, 0)),
		}}, holidays),
		StatusDays:          statusDayList{},
		AttendanceIntervals: &attendanceRowsStub{rows: rows},
		Retention:           reportRetention{},
		AccessLog:           &reportAudit{},
		Now:                 func() time.Time { return berlin(mon.AddDays(2), 12, 0) },
	}
	return cfg, mon
}

func TestHolidayBookingReplacesTheContractSollOfItsDay(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {mon: holidayBooked(8, 16)}}}
	cfg, from := holidayWeek(holidays, span(11, mon, 8, 0, 15, 45))

	result, err := NewService(cfg).CareTimeOverruns(context.Background(), studentpresence.CareTimeFilters{From: from, To: from.AddDays(1)}, careTimeActor)
	require.NoError(t, err)
	assert.Empty(t, result.Students, "the holiday booking until 16:00 covers a check-out at 15:45")
	assert.Equal(t, 1, result.ComparedDays, "one child-day, compared once")
	assert.Zero(t, result.UncomparedDays)

	child, err := NewService(cfg).StudentCareTimes(context.Background(), 11, from, from.AddDays(1), careTimeActor)
	require.NoError(t, err)
	require.Len(t, child.Days, 2, "Monday attended, Tuesday booked by the contract and missed")
	monday := child.Days[0]
	assert.Equal(t, studentpresence.CareTimeDayWithinPlan, monday.Kind)
	assert.Equal(t, studentpresence.PlannedCareSourceHoliday, monday.PlannedSource)
	assert.Equal(t, "16:00", monday.PlannedEnd.Time().Format("15:04"), "the contract's 13:00 does not override the holiday booking")
	assert.Equal(t, studentpresence.PlannedCareSourceContract, child.Days[1].PlannedSource, "outside the holiday booking the contract stays")
}

func TestHolidayBookingWithoutContractIsBooked(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	wed := mon.AddDays(2)
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {wed: holidayBooked(9, 14)}}}
	cfg, _ := holidayWeek(holidays, span(11, wed, 9, 0, 14, 40))
	cfg.Now = func() time.Time { return berlin(mon.AddDays(3), 12, 0) }

	child, err := NewService(cfg).StudentCareTimes(context.Background(), 11, wed, wed, careTimeActor)
	require.NoError(t, err)
	require.Len(t, child.Days, 1)
	assert.Equal(t, studentpresence.CareTimeDayOverrun, child.Days[0].Kind, "40 minutes past the booked 14:00")
	assert.Equal(t, 40, child.Days[0].LateMinutes)
	assert.Equal(t, studentpresence.PlannedCareSourceHoliday, child.Days[0].PlannedSource)
}

// A child present on a day without a holiday booking keeps the Soll beneath
// it: without contract or plan for the day it is "ohne Buchung".
func TestHolidayDayWithoutBookingIsUnbooked(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	wed := mon.AddDays(2)
	cfg, _ := holidayWeek(holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{}}, span(11, wed, 9, 0, 12, 0))
	cfg.Now = func() time.Time { return berlin(mon.AddDays(3), 12, 0) }

	child, err := NewService(cfg).StudentCareTimes(context.Background(), 11, wed, wed, careTimeActor)
	require.NoError(t, err)
	require.Len(t, child.Days, 1)
	assert.Equal(t, studentpresence.CareTimeDayUnbooked, child.Days[0].Kind)
}

// Missing data is never an all-clear: an open check-in on a holiday day
// stays incomplete, and an unreadable holiday booking fails the report
// instead of falling back to the contract.
func TestHolidayBookingNeverTurnsMissingDataIntoAnAllClear(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {mon: holidayBooked(8, 16)}}}
	open := &ports.Attendance{StudentID: 11, Date: mon, CheckInTime: berlin(mon, 8, 0)}
	cfg, from := holidayWeek(holidays, open)

	result, err := NewService(cfg).CareTimeOverruns(context.Background(), studentpresence.CareTimeFilters{From: from, To: from.AddDays(1)}, careTimeActor)
	require.NoError(t, err)
	assert.Zero(t, result.ComparedDays)
	assert.Equal(t, 1, result.UncomparedDays, "a day without check-out is not compared")

	failing, _ := holidayWeek(holidayStub{err: errors.New("holiday bookings unreadable")}, span(11, mon, 8, 0, 12, 0))
	_, err = NewService(failing).CareTimeOverruns(context.Background(), studentpresence.CareTimeFilters{From: from, To: from.AddDays(1)}, careTimeActor)
	require.Error(t, err, "a failed holiday read is no comparison against the contract")
}

func TestHolidayBookingKeepsACancelledDay(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	cancelled := plannedStub{11: {mon: {Cancelled: true, Source: studentpresence.PlannedCareSourceCarePlan}}}
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {mon: holidayBooked(8, 16)}}}
	planned, err := WithHolidayCareDays(cancelled, holidays).PlannedCareDays(context.Background(), []int64{11}, mon, mon)
	require.NoError(t, err)
	day := planned[11][mon]
	assert.True(t, day.Cancelled, "„Kommt heute nicht“ stays")
	assert.False(t, day.Expected)
	assert.Nil(t, day.Pickup, "a day the child does not come carries no times")
	assert.Equal(t, studentpresence.PlannedCareSourceHoliday, day.Source)
}

type contractStub map[int64]map[calendar.Date]studentpresence.ContractCareDay

func (c contractStub) ContractCareDays(context.Context, []int64, calendar.Date, calendar.Date) (map[int64]map[calendar.Date]studentpresence.ContractCareDay, error) {
	return c, nil
}

// A „Kommt heute nicht“ on a day the contract books no module survives both
// overlays: the holiday booking does not make the day expected again.
func TestCancellationSurvivesContractAndHolidayBooking(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	cancelled := plannedStub{11: {mon: {Cancelled: true, Source: studentpresence.PlannedCareSourceCarePlan}}}
	contracts := contractStub{11: {mon: {Booked: false}}}
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {mon: holidayBooked(8, 16)}}}
	planned, err := WithHolidayCareDays(WithContractCareDays(cancelled, contracts), holidays).
		PlannedCareDays(context.Background(), []int64{11}, mon, mon)
	require.NoError(t, err)
	day := planned[11][mon]
	assert.True(t, day.Cancelled, "the cancellation of the day stays")
	assert.False(t, day.Expected, "the holiday booking does not undo it")
	assert.Equal(t, studentpresence.PlannedCareSourceHoliday, day.Source)
}

// Holiday care runs in the school holidays: a missed day of a holiday booking
// in a holiday period is absent, not closed. A public holiday or a closing day
// stays closed, and a contract day in the holidays stays closed too (#5994).
func TestMissedHolidayBookingInTheSchoolHolidaysIsAbsent(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	tue, wed, thu := mon.AddDays(1), mon.AddDays(2), mon.AddDays(3)
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {
		mon: holidayBooked(8, 16), tue: holidayBooked(8, 16), wed: holidayBooked(8, 16),
	}}}
	cfg := Config{
		Students: careTimeStudents{{ID: 11, FirstName: "Ida", LastName: "Ferien", EnrolledOn: enrolledEveryDay}},
		PlannedCareDays: WithHolidayCareDays(plannedStub{11: {
			thu: contractDay(wallClockAt(13, 0)),
		}}, holidays),
		StatusDays:          statusDayList{},
		AttendanceIntervals: &attendanceRowsStub{},
		Holidays:            dateSet{tue: true},
		ClosingDays:         dateSet{wed: true},
		Periods:             periodList{{StartDate: mon.AddDays(-7), EndDate: mon.AddDays(14)}},
		Retention:           reportRetention{},
		AccessLog:           &reportAudit{},
		Now:                 func() time.Time { return berlin(mon.AddDays(7), 12, 0) },
	}
	child, err := NewService(cfg).StudentCareTimes(context.Background(), 11, mon, thu, careTimeActor)
	require.NoError(t, err)
	kinds := map[calendar.Date]studentpresence.CareTimeDayKind{}
	for _, day := range child.Days {
		kinds[day.Date] = day.Kind
	}
	assert.Equal(t, map[calendar.Date]studentpresence.CareTimeDayKind{
		mon: studentpresence.CareTimeDayAbsent,
		tue: studentpresence.CareTimeDayClosed,
		wed: studentpresence.CareTimeDayClosed,
		thu: studentpresence.CareTimeDayClosed,
	}, kinds)
	assert.Equal(t, 1, child.Summary.AbsentDays)
}

// A booked day that fits two or more children of the school has no Soll:
// it is not compared, neither against the booking nor against the
// contract beneath it, so it is never an all-clear (#5994).
func TestAmbiguousHolidayBookingDayIsNotCompared(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 20)
	ambiguous := ports.HolidayCareDay{Arrival: *wallClockAt(8, 0), Pickup: *wallClockAt(16, 0), Ambiguous: true}
	holidays := holidayStub{days: map[int64]map[calendar.Date]ports.HolidayCareDay{11: {mon: ambiguous}}}
	cfg, from := holidayWeek(holidays, span(11, mon, 8, 0, 12, 30))

	result, err := NewService(cfg).CareTimeOverruns(context.Background(), studentpresence.CareTimeFilters{From: from, To: from}, careTimeActor)
	require.NoError(t, err)
	assert.Zero(t, result.ComparedDays, "the contract's 13:00 would have made 12:30 an all-clear")
	assert.Equal(t, 1, result.UncomparedDays)

	child, err := NewService(cfg).StudentCareTimes(context.Background(), 11, from, from, careTimeActor)
	require.NoError(t, err)
	require.Len(t, child.Days, 1)
	assert.Equal(t, studentpresence.CareTimeDayNoPlan, child.Days[0].Kind)
}
