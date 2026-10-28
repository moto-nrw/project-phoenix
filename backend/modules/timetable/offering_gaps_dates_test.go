package timetable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The school year 2030/31 from a Monday on; the Regeltermin and bookings
// below run on Mondays.
var (
	datesScope = OfferingGapScope{From: calendar.NewDate(2030, 9, 2), Until: calendar.NewDate(2031, 7, 18)}
	november   = weeklyRun{Weekday: WeekdayMonday, From: calendar.NewDate(2030, 11, 4), Until: calendar.NewDate(2030, 11, 25)}
)

func datePtr(d calendar.Date) *calendar.Date { return &d }

func mondayBooking(from, until *calendar.Date) []OfferingWeekdayBooking {
	bookings := gapBookings(gapRandstunde, "Randstunde", "3a", 900, 1, WeekdayMonday)
	bookings[0].Windows = []OfferingBookingWindow{{ValidFrom: from, ValidUntil: until, Weekdays: []int{WeekdayMonday}}}
	return bookings
}

// weeklyRun stands for a schedule materialized on every weekday from From to
// Until, both inclusive.
type weeklyRun struct {
	Weekday     int
	From, Until calendar.Date
}

func randstundeTemplate(runs ...weeklyRun) []OfferingGapTemplate {
	occurrences := map[calendar.Date]bool{}
	for _, run := range runs {
		for date := NextWeekdayOnOrAfter(run.From, run.Weekday); !date.After(run.Until); date = date.AddDays(7) {
			occurrences[date] = true
		}
	}
	return []OfferingGapTemplate{{Occurrences: occurrences, Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}}}}
}

// TestDetectOfferingGapsNeedsOverlappingDates pins the date check of #5140:
// a Regeltermin covers a booked date only when it has an occurrence on that
// date, whatever the weekday says.
func TestDetectOfferingGapsNeedsOverlappingDates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		booking []OfferingWeekdayBooking
		runs    []weeklyRun
		wantGap bool
	}{
		{
			name:    "November series, December booking",
			booking: mondayBooking(datePtr(calendar.NewDate(2030, 12, 2)), datePtr(calendar.NewDate(2031, 1, 1))),
			runs:    []weeklyRun{november}, wantGap: true,
		},
		{
			name:    "November series, booking for the whole year",
			booking: mondayBooking(nil, nil),
			runs:    []weeklyRun{november}, wantGap: true,
		},
		{
			name:    "November series, November booking",
			booking: mondayBooking(datePtr(calendar.NewDate(2030, 11, 4)), datePtr(calendar.NewDate(2030, 12, 1))),
			runs:    []weeklyRun{november},
		},
		{
			name:    "two runs meet without a hole",
			booking: mondayBooking(datePtr(calendar.NewDate(2030, 11, 1)), datePtr(calendar.NewDate(2030, 12, 10))),
			runs: []weeklyRun{november,
				{Weekday: WeekdayMonday, From: calendar.NewDate(2030, 12, 2), Until: calendar.NewDate(2031, 7, 14)}},
		},
		{
			name:    "a run on Wednesday covers no Monday",
			booking: mondayBooking(datePtr(calendar.NewDate(2030, 11, 4)), datePtr(calendar.NewDate(2030, 12, 1))),
			runs:    []weeklyRun{{Weekday: WeekdayWednesday, From: november.From, Until: november.Until}}, wantGap: true,
		},
		{
			name:    "a booking that ended before the scope counts no date",
			booking: mondayBooking(nil, datePtr(calendar.NewDate(2030, 9, 1))),
		},
		{
			name:    "a booking starting after the scope counts no date",
			booking: mondayBooking(datePtr(calendar.NewDate(2031, 8, 1)), nil),
		},
		{
			name:    "the exclusive end of a booking is not booked",
			booking: mondayBooking(datePtr(calendar.NewDate(2030, 11, 4)), datePtr(calendar.NewDate(2030, 12, 2))),
			runs:    []weeklyRun{november},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gaps := DetectOfferingGaps(datesScope, randstundeTemplate(tc.runs...), tc.booking)
			if !tc.wantGap {
				assert.Empty(t, gaps)
				return
			}
			require.Len(t, gaps, 1)
			assert.Equal(t, []int{WeekdayMonday}, gaps[0].Weekdays)
		})
	}
}

// TestDetectOfferingGapsPerLinkWindow: a child whose first link books Monday
// until November and whose second link books Wednesday from December on is
// checked link by link, not with the union of both weekdays over the year.
func TestDetectOfferingGapsPerLinkWindow(t *testing.T) {
	t.Parallel()
	booking := gapBookings(gapRandstunde, "Randstunde", "3a", 901, 1, WeekdayMonday, WeekdayWednesday)
	booking[0].Windows = []OfferingBookingWindow{
		{ValidUntil: datePtr(calendar.NewDate(2030, 12, 1)), Weekdays: []int{WeekdayMonday}},
		{ValidFrom: datePtr(calendar.NewDate(2030, 12, 1)), Weekdays: []int{WeekdayWednesday}},
	}
	runs := []weeklyRun{
		{Weekday: WeekdayMonday, From: datesScope.From, Until: november.Until},
		{Weekday: WeekdayWednesday, From: calendar.NewDate(2030, 12, 4), Until: datesScope.Until},
	}
	assert.Empty(t, DetectOfferingGaps(datesScope, randstundeTemplate(runs...), booking))

	runs[1].From = calendar.NewDate(2031, 1, 8)
	gaps := DetectOfferingGaps(datesScope, randstundeTemplate(runs...), booking)
	require.Len(t, gaps, 1, "the December Wednesdays have no Regeltermin")
	assert.Equal(t, []int{WeekdayWednesday}, gaps[0].Weekdays)
}

func TestWeekdayStepping(t *testing.T) {
	t.Parallel()
	wednesday := calendar.NewDate(2030, 11, 6)
	assert.Equal(t, calendar.NewDate(2030, 11, 11), NextWeekdayOnOrAfter(wednesday, WeekdayMonday))
	assert.Equal(t, wednesday, NextWeekdayOnOrAfter(wednesday, WeekdayWednesday))
	assert.Equal(t, calendar.NewDate(2030, 11, 10), NextWeekdayOnOrAfter(wednesday, 7), "Sunday is ISO 7")
}
