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

// TestCollectOfferingWeekdayBookingsHolidayPhase (#5139): a booking carries
// whether its offering's phase is of the kind Ferienbetreuung. The
// offering's own holiday-care flag does not count: Ganztag including holiday
// care in a school-year phase stays a school-year booking.
func TestCollectOfferingWeekdayBookingsHolidayPhase(t *testing.T) {
	t.Parallel()
	start, end := calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 7, 31)
	schoolYear := careplan.OfferingPhase{ID: 1, ServiceStart: start, ServiceEnd: end}
	holiday := careplan.OfferingPhase{ID: 2, ServiceStart: calendar.NewDate(2026, 10, 12), ServiceEnd: calendar.NewDate(2026, 10, 23), Holiday: true}
	ganztag := &careplan.CareOffering{ID: 11, PhaseID: schoolYear.ID, DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}, IncludesHolidayCare: true}
	ferien := &careplan.CareOffering{ID: 12, PhaseID: holiday.ID, DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}, IncludesHolidayCare: true}

	bookings := collectOfferingWeekdayBookings(
		map[int64]*careplan.CareOffering{ganztag.ID: ganztag, ferien.ID: ferien},
		map[int64]careplan.OfferingPhase{schoolYear.ID: schoolYear, holiday.ID: holiday},
		[]ports.ApprovedOfferingChild{
			{StudentID: 21, Link: careplan.BookedOffering{CareOfferingID: ganztag.ID}},
			{StudentID: 21, Link: careplan.BookedOffering{CareOfferingID: ferien.ID}},
		},
		calendar.NewDate(2026, 9, 1), nil,
	)

	require.Len(t, bookings, 2)
	byOffering := map[int64]careplan.OfferingWeekdayBooking{}
	for _, booking := range bookings {
		byOffering[booking.OfferingID] = booking
	}
	assert.False(t, byOffering[ganztag.ID].HolidayPhase, "Ganztag including holiday care stays suggestible")
	assert.True(t, byOffering[ferien.ID].HolidayPhase)
}

// TestCollectOfferingWeekdayBookingsEnded (#5102): a window counts as ended
// only when its link ends before the phase's last service day ends. A link
// stored with the phase end, or wider, ends with the phase, not by a change.
func TestCollectOfferingWeekdayBookingsEnded(t *testing.T) {
	t.Parallel()
	start := calendar.NewDate(2026, 8, 1)
	end := calendar.NewDate(2027, 7, 31)
	phase := careplan.OfferingPhase{ServiceStart: start, ServiceEnd: end}
	offering := &careplan.CareOffering{DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	changed, withPhase, wider := calendar.NewDate(2026, 11, 2), end.AddDays(1), end.AddDays(30)
	for _, tc := range []struct {
		name  string
		until *calendar.Date
		ended bool
	}{
		{name: "open link"},
		{name: "ends with the phase", until: &withPhase},
		{name: "ends after the phase", until: &wider},
		{name: "ended by a change", until: &changed, ended: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bookings := collectOfferingWeekdayBookings(
				map[int64]*careplan.CareOffering{offering.ID: offering},
				map[int64]careplan.OfferingPhase{phase.ID: phase},
				[]ports.ApprovedOfferingChild{{Link: careplan.BookedOffering{CareOfferingID: offering.ID, ValidUntil: tc.until}}},
				start, nil,
			)
			require.Len(t, bookings, 1)
			require.Len(t, bookings[0].Windows, 1)
			assert.Equal(t, tc.ended, bookings[0].Windows[0].Ended)
		})
	}
}
