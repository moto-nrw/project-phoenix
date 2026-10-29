package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestCollectOfferingWeekdayBookingsServiceWindow(t *testing.T) {
	t.Parallel()
	start := calendar.NewDate(2026, 11, 1)
	end := calendar.NewDate(2026, 11, 30)
	before := start.AddDays(-1)
	after := end.AddDays(1)
	insideStart := start.AddDays(5)
	insideEnd := end.AddDays(-5)
	phase := careplan.OfferingPhase{ServiceStart: start, ServiceEnd: end}
	offering := &careplan.CareOffering{DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	for _, tc := range []struct {
		name                string
		from, until         *calendar.Date
		wantFrom, wantUntil calendar.Date
		empty               bool
	}{
		{name: "open link", wantFrom: start, wantUntil: after},
		{name: "wider link", from: &before, until: &after, wantFrom: start, wantUntil: after},
		{name: "narrower link", from: &insideStart, until: &insideEnd, wantFrom: insideStart, wantUntil: insideEnd},
		{name: "ends at service start", until: &start, empty: true},
		{name: "starts after service end", from: &after, empty: true},
		{name: "last service day", from: &end, wantFrom: end, wantUntil: after},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bookings := collectOfferingWeekdayBookings(
				map[int64]*careplan.CareOffering{offering.ID: offering},
				map[int64]careplan.OfferingPhase{phase.ID: phase},
				[]ports.ApprovedOfferingChild{{Link: careplan.BookedOffering{CareOfferingID: offering.ID, ValidFrom: tc.from, ValidUntil: tc.until}}},
				before, nil,
			)
			if tc.empty {
				require.Empty(t, bookings)
				return
			}
			require.Len(t, bookings, 1)
			require.Len(t, bookings[0].Windows, 1)
			assert.Equal(t, &tc.wantFrom, bookings[0].Windows[0].ValidFrom)
			assert.Equal(t, &tc.wantUntil, bookings[0].Windows[0].ValidUntil)
			assert.Equal(t, []int{1}, bookings[0].Windows[0].Weekdays)
		})
	}
}

// TestCollectOfferingWeekdayBookingsClassDay: the Klassentag applies to an
// offering of a school-year phase without holiday care only (#5343); the
// pickup times travel keyed by ISO weekday.
func TestCollectOfferingWeekdayBookingsClassDay(t *testing.T) {
	t.Parallel()
	start := calendar.NewDate(2026, 11, 2)
	end := calendar.NewDate(2027, 1, 29)
	for _, tc := range []struct {
		name          string
		schoolYear    bool
		holidayCare   bool
		wantClassDays bool
	}{
		{name: "school year", schoolYear: true, wantClassDays: true},
		{name: "school year with holiday care", schoolYear: true, holidayCare: true},
		{name: "holiday or custom phase", schoolYear: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			phase := careplan.OfferingPhase{ServiceStart: start, ServiceEnd: end, SchoolYear: tc.schoolYear}
			offering := &careplan.CareOffering{
				DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon", "tue"}, IncludesHolidayCare: tc.holidayCare,
				PickupTimes: map[string]string{"mon": "12:45", " TUE ": " 13:30 ", "wed": "", "xyz": "14:00"},
			}
			bookings := collectOfferingWeekdayBookings(
				map[int64]*careplan.CareOffering{offering.ID: offering},
				map[int64]careplan.OfferingPhase{phase.ID: phase},
				[]ports.ApprovedOfferingChild{{Link: careplan.BookedOffering{CareOfferingID: offering.ID}}},
				start, nil,
			)
			require.Len(t, bookings, 1)
			assert.Equal(t, tc.wantClassDays, bookings[0].ClassDayApplies)
			assert.Equal(t, map[int]string{1: "12:45", 2: "13:30"}, bookings[0].PickupTimes)
		})
	}
}
