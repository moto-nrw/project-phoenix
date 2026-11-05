package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// switchingPickup books 14:30 before the switch date and 16:00 from it on, and
// counts how often the baseline is projected.
type switchingPickup struct {
	ports.BookingPickup
	switchOn    calendar.Date
	rows        []ports.PickupWeekdayRow
	projections int
	longestDays int
}

func (p *switchingPickup) WeekdayRows(context.Context, int64) ([]ports.PickupWeekdayRow, error) {
	return p.rows, nil
}

func (p *switchingPickup) OfferingPickupsBetween(_ context.Context, studentID int64, from, to calendar.Date) (*careplan.PickupBaselineProjection, error) {
	p.projections++
	p.longestDays = max(p.longestDays, from.DaysUntil(to)+1)
	byDate := careplan.PickupPlanByDate{}
	for day := from; !day.After(to); day = day.AddDays(1) {
		clock := "14:30"
		if !day.Before(p.switchOn) {
			clock = "16:00"
		}
		byDate[day] = careplan.PickupWeek{
			domain.ISOWeekday(day): {StudentID: studentID, PickupTime: wallClock(clock), Source: careplan.ScheduleSourceCareOffering},
		}
	}
	return &careplan.PickupBaselineProjection{OfferingByStudentDate: careplan.PickupPlansByStudent{studentID: byDate}}, nil
}

// A booking change a year ahead checks every earlier week of each overridden
// weekday, but from one projection instead of one per week and weekday.
func TestPickupOverridesProjectTheRangeOnce(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 4)
	stichtag := calendar.NewDate(2027, 11, 5)
	pickup := &switchingPickup{switchOn: stichtag, rows: []ports.PickupWeekdayRow{
		{ID: 1, Weekday: 1, PickupTime: "14:30", Manual: true},
		{ID: 2, Weekday: 3, PickupTime: "14:30", Manual: true},
		{ID: 3, Weekday: 4, PickupTime: "15:15", Manual: true},
		{ID: 4, Weekday: 5, PickupTime: "14:30", Manual: false},
	}}
	m := &BookingMaterialization{deps: BookingMaterializationDependencies{
		Pickup: pickup, Today: func() calendar.Date { return today },
	}}

	overrides, err := m.pickupOverrides(context.Background(), 7, stichtag)

	require.NoError(t, err)
	assert.Equal(t, 1, pickup.projections)
	assert.Equal(t, []careplan.PickupOverride{
		{Weekday: 1, Date: calendar.NewDate(2027, 11, 8), StoredTime: "14:30", BookedTime: "16:00", ResetNow: true},
		{Weekday: 3, Date: calendar.NewDate(2027, 11, 10), StoredTime: "14:30", BookedTime: "16:00", ResetNow: true},
		{Weekday: 4, Date: calendar.NewDate(2027, 11, 11), StoredTime: "15:15", BookedTime: "16:00", ResetNow: false},
	}, overrides, "Friday's legacy offering row hides nothing")
}

// A change years ahead is projected in bounded windows: memory does not grow
// with the distance, and every row is still answered for every earlier day.
func TestPickupOverridesWalkLongRangesInWindows(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 4)
	stichtag := calendar.NewDate(2029, 11, 5)
	pickup := &switchingPickup{switchOn: stichtag, rows: []ports.PickupWeekdayRow{
		{ID: 1, Weekday: 1, PickupTime: "14:30", Manual: true},
		{ID: 3, Weekday: 4, PickupTime: "15:15", Manual: true},
	}}
	m := &BookingMaterialization{deps: BookingMaterializationDependencies{
		Pickup: pickup, Today: func() calendar.Date { return today },
	}}

	overrides, err := m.pickupOverrides(context.Background(), 7, stichtag)

	require.NoError(t, err)
	assert.Equal(t, 2, pickup.projections, "1,104 days take two windows")
	assert.LessOrEqual(t, pickup.longestDays, pickupProjectionWindowDays)
	assert.Equal(t, []careplan.PickupOverride{
		{Weekday: 1, Date: stichtag, StoredTime: "14:30", BookedTime: "16:00", ResetNow: true},
		{Weekday: 4, Date: calendar.NewDate(2029, 11, 8), StoredTime: "15:15", BookedTime: "16:00", ResetNow: false},
	}, overrides)
}

// The reset walks the earlier days latest first and stops in the window that
// holds the last day still needing the stored time.
func TestPickupResetCheckStopsAtTheLatestNeededDay(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 4)
	date := calendar.NewDate(2029, 11, 5)
	pickup := &switchingPickup{switchOn: date}
	m := &BookingMaterialization{deps: BookingMaterializationDependencies{
		Pickup: pickup, Today: func() calendar.Date { return today },
	}}
	row := ports.PickupWeekdayRow{ID: 1, Weekday: 1, PickupTime: "15:15", Manual: true}

	err := m.refuseResetWhileEarlierDaysNeedRow(context.Background(), 7, row, date)

	var stillNeeded *careplan.PickupResetStillNeededError
	require.ErrorAs(t, err, &stillNeeded)
	assert.Equal(t, date.AddDays(-7), stillNeeded.NeededUntil)
	assert.Equal(t, 1, pickup.projections, "the latest window already names the day")
	assert.LessOrEqual(t, pickup.longestDays, pickupProjectionWindowDays)
}
