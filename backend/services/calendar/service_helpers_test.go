package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func helperClock(hour, minute int) time.Time {
	return timezone.WallClock(time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC))
}

func helperAppointment() *calModels.Appointment {
	return &calModels.Appointment{
		Model:              base.Model{ID: 42},
		OrganizerStaffID:   7,
		Title:              "Planning",
		StartDate:          timezone.NewDate(2026, 1, 5),
		EndDate:            timezone.NewDate(2026, 1, 5),
		StartTime:          helperClock(9, 0),
		EndTime:            helperClock(10, 0),
		DeliveryMode:       calModels.DeliveryModeRSVPRequired,
		OverviewVisibility: calModels.OverviewVisibilityOrganizer,
	}
}

func TestCalendarOverviewVisibilityHelpers(t *testing.T) {
	appointment := helperAppointment()

	assert.False(t, canStaffViewOverview(nil, 7, true))
	assert.True(t, canStaffViewOverview(appointment, 7, false))
	assert.False(t, canStaffViewOverview(appointment, 8, true))

	appointment.OverviewVisibility = calModels.OverviewVisibilityStaff
	assert.True(t, canStaffViewOverview(appointment, 8, true))
	assert.False(t, canParentViewOverview(appointment, true))

	appointment.OverviewVisibility = calModels.OverviewVisibilityAll
	assert.True(t, canStaffViewOverview(appointment, 8, true))
	assert.True(t, canParentViewOverview(appointment, true))
	assert.False(t, canParentViewOverview(appointment, false))
}

func TestAppointmentEventAndOverride(t *testing.T) {
	appointment := helperAppointment()
	appointment.OverviewVisibility = calModels.OverviewVisibilityAll
	status := calModels.ResponseStatusPending
	recipientID := int64(99)

	event := appointmentEvent(appointment, timezone.NewDate(2026, 1, 6), &status, &recipientID, 8)

	assert.Equal(t, "appointment:42:2026-01-06", event.ID)
	assert.Equal(t, EventSourceAppointment, event.Source)
	assert.Equal(t, "2026-01-06", event.StartDate)
	assert.Equal(t, "09:00", event.StartTime)
	assert.True(t, event.CanRespond)
	assert.True(t, event.CanViewOverview)
	assert.False(t, event.CanEdit)

	title := "Changed"
	description := "New details"
	location := "Room 2"
	startDate := timezone.NewDate(2026, 1, 7)
	endDate := timezone.NewDate(2026, 1, 8)
	startTime := helperClock(11, 30)
	endTime := helperClock(12, 45)
	allDay := true
	applyOverride(&event, &calModels.AppointmentOccurrenceOverride{
		Title:       &title,
		Description: &description,
		Location:    &location,
		StartDate:   &startDate,
		EndDate:     &endDate,
		StartTime:   &startTime,
		EndTime:     &endTime,
		AllDay:      &allDay,
	})

	assert.Equal(t, "Changed", event.Title)
	require.NotNil(t, event.Description)
	assert.Equal(t, "New details", *event.Description)
	require.NotNil(t, event.Location)
	assert.Equal(t, "Room 2", *event.Location)
	assert.Equal(t, "2026-01-07", event.StartDate)
	assert.Equal(t, "2026-01-08", event.EndDate)
	assert.Equal(t, "11:30", event.StartTime)
	assert.Equal(t, "12:45", event.EndTime)
	assert.True(t, event.AllDay)
}

func TestAppointmentEventCanRespond(t *testing.T) {
	appointment := helperAppointment()
	recipientID := int64(99)
	day := timezone.NewDate(2026, 1, 6)

	// A recipient can respond for any non-informational status, including an
	// already accepted/declined one — the respond endpoints allow changing it.
	for _, status := range []string{
		calModels.ResponseStatusPending,
		calModels.ResponseStatusAccepted,
		calModels.ResponseStatusDeclined,
	} {
		s := status
		event := appointmentEvent(appointment, day, &s, &recipientID, 8)
		assert.Truef(t, event.CanRespond, "status %q should be respondable", status)
	}

	// Informational recipients cannot respond.
	info := calModels.ResponseStatusInfo
	infoEvent := appointmentEvent(appointment, day, &info, &recipientID, 8)
	assert.False(t, infoEvent.CanRespond)

	// A non-recipient (nil recipient/status) cannot respond.
	nonRecipient := appointmentEvent(appointment, day, nil, nil, 8)
	assert.False(t, nonRecipient.CanRespond)
}

func TestMatchesRuleWeeklyIntervalAnchorsToCalendarWeeks(t *testing.T) {
	// Wednesday start, biweekly, on Mondays and Wednesdays. Intervals must be
	// counted in calendar weeks (Mon–Sun), not 7-day blocks from the Wed start.
	start := timezone.NewDate(2026, 1, 7) // Wednesday (Jan 5 is the Monday)
	rule := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 2,
		Weekdays:      []string{"monday", "wednesday"},
	}

	// Week 0 (the start week): the Wednesday start is included.
	assert.True(t, matchesRule(start, timezone.NewDate(2026, 1, 7), rule))
	// Week 1 (odd) is skipped entirely — including the Monday only 5 days after
	// the start, which the old day-count logic wrongly treated as "week 0".
	assert.False(t, matchesRule(start, timezone.NewDate(2026, 1, 12), rule))
	assert.False(t, matchesRule(start, timezone.NewDate(2026, 1, 14), rule))
	// Week 2 (even) includes both weekdays — the Monday the old logic dropped.
	assert.True(t, matchesRule(start, timezone.NewDate(2026, 1, 19), rule))
	assert.True(t, matchesRule(start, timezone.NewDate(2026, 1, 21), rule))
}

func TestCalendarRecipientStatusHelpers(t *testing.T) {
	staffID := int64(10)
	guardianID := int64(20)
	recipients := []*calModels.AppointmentRecipient{
		{Model: base.Model{ID: 101}, RecipientType: calModels.RecipientTypeStaff, StaffID: &staffID, Status: calModels.ResponseStatusAccepted},
		{Model: base.Model{ID: 102}, RecipientType: calModels.RecipientTypeGuardianProfile, GuardianProfileID: &guardianID, Status: calModels.ResponseStatusDeclined},
	}

	status, recipientID := staffRecipientStatus(recipients, staffID)
	require.NotNil(t, status)
	require.NotNil(t, recipientID)
	assert.Equal(t, calModels.ResponseStatusAccepted, *status)
	assert.Equal(t, int64(101), *recipientID)

	status, recipientID = guardianRecipientStatus(recipients, []int64{99, guardianID})
	require.NotNil(t, status)
	require.NotNil(t, recipientID)
	assert.Equal(t, calModels.ResponseStatusDeclined, *status)
	assert.Equal(t, int64(102), *recipientID)

	status, recipientID = staffRecipientStatus(recipients, 99)
	assert.Nil(t, status)
	assert.Nil(t, recipientID)
}

func TestCalendarRecurrenceExpansion(t *testing.T) {
	appointment := helperAppointment()
	endsOn := timezone.NewDate(2026, 1, 31)
	count := 3

	weekly := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{" monday ", "WEDNESDAY"},
		EndsOn:        &endsOn,
	}
	assert.Equal(t, []timezone.Date{
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 7),
		timezone.NewDate(2026, 1, 12),
	}, expandOccurrences(appointment, weekly, timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 12)))

	monthly := &calModels.RecurrenceRule{
		Frequency:       calModels.RecurrenceFrequencyMonthly,
		IntervalCount:   1,
		MonthDays:       []int{5, 20},
		OccurrenceCount: &count,
	}
	assert.Equal(t, []timezone.Date{
		timezone.NewDate(2026, 1, 5),
		timezone.NewDate(2026, 1, 20),
		timezone.NewDate(2026, 2, 5),
	}, expandOccurrences(appointment, monthly, timezone.NewDate(2026, 1, 1), timezone.NewDate(2026, 3, 31)))

	yearly := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyYearly, IntervalCount: 1}
	daily := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 2}
	weeklyDefault := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1}
	monthlyDefault := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 1}

	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 7), daily))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 6), daily))
	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 12), weeklyDefault))
	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 2, 5), monthlyDefault))
	assert.True(t, matchesRule(appointment.StartDate, timezone.NewDate(2027, 1, 5), yearly))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2027, 1, 6), yearly))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 4), yearly))
	assert.False(t, matchesRule(appointment.StartDate, timezone.NewDate(2026, 1, 5), &calModels.RecurrenceRule{Frequency: "never", IntervalCount: 1}))
}

func TestFirstRecurrenceOccurrence(t *testing.T) {
	appt := func(d timezone.Date) *calModels.Appointment {
		return &calModels.Appointment{StartDate: d, EndDate: d}
	}
	ptr := func(d timezone.Date) *timezone.Date { return &d }

	tuesday := timezone.NewDate(2026, 1, 6) // 2026-01-05 is a Monday
	weeklyMondayInterval := func(n int) *calModels.RecurrenceRule {
		return &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: n, Weekdays: []string{"monday"}}
	}

	tests := []struct {
		name  string
		appt  *calModels.Appointment
		rule  *calModels.RecurrenceRule
		want  timezone.Date
		w4dok bool
	}{
		{
			name: "daily matches start",
			appt: appt(timezone.NewDate(2026, 3, 4)),
			rule: &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 5},
			want: timezone.NewDate(2026, 3, 4), w4dok: true,
		},
		{
			name: "yearly matches start",
			appt: appt(timezone.NewDate(2026, 3, 4)),
			rule: &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyYearly, IntervalCount: 3},
			want: timezone.NewDate(2026, 3, 4), w4dok: true,
		},
		{
			name: "weekly without weekdays matches start",
			appt: appt(tuesday),
			rule: &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1},
			want: tuesday, w4dok: true,
		},
		{
			name: "weekly weekday excludes start weekday -> next matching weekday",
			appt: appt(tuesday),
			rule: weeklyMondayInterval(1),
			want: timezone.NewDate(2026, 1, 12), w4dok: true,
		},
		{
			name:  "weekly large interval resolves far out, no fixed cap",
			appt:  appt(tuesday),
			rule:  weeklyMondayInterval(60),
			want:  weekStartMonday(tuesday).AddDays(60 * 7),
			w4dok: true,
		},
		{
			name: "monthly sparse day beyond a year is accepted",
			appt: appt(timezone.NewDate(2026, 1, 31)),
			rule: &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 24, MonthDays: []int{30}},
			want: timezone.NewDate(2028, 1, 30), w4dok: true,
		},
		{
			name:  "monthly February-only day 31 is impossible",
			appt:  appt(timezone.NewDate(2026, 2, 15)),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 12, MonthDays: []int{31}},
			w4dok: false,
		},
		{
			name:  "monthly February-only day 30 is impossible",
			appt:  appt(timezone.NewDate(2026, 2, 15)),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 12, MonthDays: []int{30}},
			w4dok: false,
		},
		{
			name: "monthly February day 29 resolves to the next leap year",
			appt: appt(timezone.NewDate(2025, 2, 1)),
			rule: &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 12, MonthDays: []int{29}},
			want: timezone.NewDate(2028, 2, 29), w4dok: true,
		},
		{
			name:  "first occurrence after EndsOn is rejected",
			appt:  appt(tuesday),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: ptr(timezone.NewDate(2026, 1, 8))},
			w4dok: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := firstRecurrenceOccurrence(tc.appt, tc.rule)
			assert.Equal(t, tc.w4dok, ok)
			if tc.w4dok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestOccurrenceExists(t *testing.T) {
	start := timezone.NewDate(2026, 1, 5) // Monday
	appt := &calModels.Appointment{StartDate: start, EndDate: start}
	weeklyMonday := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}}

	assert.True(t, occurrenceExists(appt, weeklyMonday, timezone.NewDate(2026, 1, 12)))  // valid Monday
	assert.False(t, occurrenceExists(appt, weeklyMonday, timezone.NewDate(2026, 1, 13))) // Tuesday
	assert.False(t, occurrenceExists(appt, weeklyMonday, timezone.NewDate(2026, 1, 4)))  // before start

	// A far-future valid occurrence is accepted arithmetically — no day-by-day
	// expansion from the series start.
	daily := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 1}
	assert.True(t, occurrenceExists(appt, daily, start.AddDays(100000)))

	// EndsOn bound.
	endsOn := timezone.NewDate(2026, 1, 19)
	bounded := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: &endsOn}
	assert.True(t, occurrenceExists(appt, bounded, timezone.NewDate(2026, 1, 19)))
	assert.False(t, occurrenceExists(appt, bounded, timezone.NewDate(2026, 1, 26))) // after EndsOn

	// Count bound: only the first two occurrences exist.
	count := 2
	counted := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, OccurrenceCount: &count}
	assert.True(t, occurrenceExists(appt, counted, timezone.NewDate(2026, 1, 5)))   // 1st
	assert.True(t, occurrenceExists(appt, counted, timezone.NewDate(2026, 1, 12)))  // 2nd
	assert.False(t, occurrenceExists(appt, counted, timezone.NewDate(2026, 1, 19))) // 3rd — beyond count
}

func TestHasOccurrenceInWindow(t *testing.T) {
	oldStart := timezone.NewDate(2020, 1, 6) // an old Monday
	appt := &calModels.Appointment{StartDate: oldStart, EndDate: oldStart}
	weeklyMonday := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}}

	from := timezone.NewDate(2026, 7, 1)
	to := timezone.NewDate(2026, 8, 1)

	// An old open-ended series still overlaps a current window.
	assert.True(t, hasOccurrenceInWindow(appt, weeklyMonday, from, to))

	// EndsOn in the past → no overlap now.
	past := timezone.NewDate(2020, 3, 1)
	ended := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: &past}
	assert.False(t, hasOccurrenceInWindow(appt, ended, from, to))

	// A count-bounded old series whose occurrences are all in the past → no overlap.
	count := 3
	counted := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, OccurrenceCount: &count}
	assert.False(t, hasOccurrenceInWindow(appt, counted, from, to))

	// A multi-day occurrence starting just before the window but spanning into it
	// is detected (window scan starts early enough to catch it).
	multiDay := &calModels.Appointment{StartDate: timezone.NewDate(2020, 1, 1), EndDate: timezone.NewDate(2020, 1, 3)}
	daily := &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 1}
	assert.True(t, hasOccurrenceInWindow(multiDay, daily, timezone.NewDate(2026, 7, 1), timezone.NewDate(2026, 7, 1)))
}

func TestBoundedRecurrenceDates(t *testing.T) {
	appt := func(start, end timezone.Date) *calModels.Appointment {
		return &calModels.Appointment{StartDate: start, EndDate: end}
	}
	same := func(d timezone.Date) *calModels.Appointment { return appt(d, d) }
	ptr := func(d timezone.Date) *timezone.Date { return &d }

	tests := []struct {
		name  string
		appt  *calModels.Appointment
		rule  *calModels.RecurrenceRule
		limit int
		want  []timezone.Date
	}{
		{
			name:  "daily interval steps by period",
			appt:  same(timezone.NewDate(2026, 1, 5)),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 3},
			limit: 4,
			want: []timezone.Date{
				timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 8),
				timezone.NewDate(2026, 1, 11), timezone.NewDate(2026, 1, 14),
			},
		},
		{
			name:  "weekly multiple weekdays in order",
			appt:  same(timezone.NewDate(2026, 1, 5)), // Monday
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday", "wednesday"}},
			limit: 5,
			want: []timezone.Date{
				timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 7),
				timezone.NewDate(2026, 1, 12), timezone.NewDate(2026, 1, 14),
				timezone.NewDate(2026, 1, 19),
			},
		},
		{
			name:  "weekly interval skips whole calendar weeks",
			appt:  same(timezone.NewDate(2026, 1, 6)), // Tuesday, Monday is before start
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyWeekly, IntervalCount: 2, Weekdays: []string{"monday"}},
			limit: 3,
			want: []timezone.Date{
				timezone.NewDate(2026, 1, 19), timezone.NewDate(2026, 2, 2),
				timezone.NewDate(2026, 2, 16),
			},
		},
		{
			name:  "monthly day 31 skips short months without day-walking",
			appt:  same(timezone.NewDate(2026, 1, 31)),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{31}},
			limit: 5,
			want: []timezone.Date{
				timezone.NewDate(2026, 1, 31), timezone.NewDate(2026, 3, 31),
				timezone.NewDate(2026, 5, 31), timezone.NewDate(2026, 7, 31),
				timezone.NewDate(2026, 8, 31),
			},
		},
		{
			name:  "yearly Feb 29 lands only on leap years",
			appt:  same(timezone.NewDate(2024, 2, 29)),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyYearly, IntervalCount: 1},
			limit: 3,
			want: []timezone.Date{
				timezone.NewDate(2024, 2, 29), timezone.NewDate(2028, 2, 29),
				timezone.NewDate(2032, 2, 29),
			},
		},
		{
			name:  "EndsOn cuts the series short",
			appt:  same(timezone.NewDate(2026, 1, 5)),
			rule:  &calModels.RecurrenceRule{Frequency: calModels.RecurrenceFrequencyDaily, IntervalCount: 2, EndsOn: ptr(timezone.NewDate(2026, 1, 10))},
			limit: 10,
			want: []timezone.Date{
				timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 7),
				timezone.NewDate(2026, 1, 9),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, boundedRecurrenceDates(tc.appt, tc.rule, tc.limit))
		})
	}
}

func TestBoundedRecurrenceDatesDeduplicates(t *testing.T) {
	start := timezone.NewDate(2026, 1, 5) // Monday
	appt := &calModels.Appointment{StartDate: start, EndDate: start}

	// A repeated weekday must not emit the same date twice (which would consume
	// two of the occurrence_count slots for one real occurrence).
	weeklyDup := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday", "monday"},
	}
	assert.Equal(t, []timezone.Date{
		timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 12),
		timezone.NewDate(2026, 1, 19),
	}, boundedRecurrenceDates(appt, weeklyDup, 3))

	// A repeated month day likewise collapses to one occurrence.
	monthlyDup := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyMonthly,
		IntervalCount: 1,
		MonthDays:     []int{5, 5, 20},
	}
	assert.Equal(t, []timezone.Date{
		timezone.NewDate(2026, 1, 5), timezone.NewDate(2026, 1, 20),
		timezone.NewDate(2026, 2, 5),
	}, boundedRecurrenceDates(appt, monthlyDup, 3))
}

func TestOccurrenceExistsSparseCountBounded(t *testing.T) {
	// A count-bounded monthly "day 31" series: the reachable occurrences are the
	// first N months that actually have a 31st. Membership must be exact without
	// scanning every day since the (possibly old) start.
	start := timezone.NewDate(2026, 1, 31)
	appt := &calModels.Appointment{StartDate: start, EndDate: start}
	count := 5
	rule := &calModels.RecurrenceRule{
		Frequency:       calModels.RecurrenceFrequencyMonthly,
		IntervalCount:   1,
		MonthDays:       []int{31},
		OccurrenceCount: &count,
	}

	assert.True(t, occurrenceExists(appt, rule, timezone.NewDate(2026, 8, 31)))   // 5th occurrence
	assert.False(t, occurrenceExists(appt, rule, timezone.NewDate(2026, 10, 31))) // 6th — beyond count
	assert.False(t, occurrenceExists(appt, rule, timezone.NewDate(2026, 2, 28)))  // not a rule date
}

func TestHasOccurrenceInWindowSparseCountBounded(t *testing.T) {
	// A very old count-bounded yearly Feb-29 series: hasOccurrenceInWindow must
	// decide overlap by enumerating the (few) occurrences per period, never by
	// walking the ~decades of days between the start and the feed window.
	start := timezone.NewDate(2000, 2, 29)
	appt := &calModels.Appointment{StartDate: start, EndDate: start}
	count := 10 // 2000, 2004, 2008, ... 2036
	rule := &calModels.RecurrenceRule{
		Frequency:       calModels.RecurrenceFrequencyYearly,
		IntervalCount:   1,
		OccurrenceCount: &count,
	}

	// 2028-02-29 is the 8th occurrence — inside the count, so a window over it hits.
	assert.True(t, hasOccurrenceInWindow(appt, rule, timezone.NewDate(2028, 2, 1), timezone.NewDate(2028, 3, 1)))
	// 2040 is past the 10th occurrence (2036) → no overlap.
	assert.False(t, hasOccurrenceInWindow(appt, rule, timezone.NewDate(2040, 1, 1), timezone.NewDate(2040, 12, 31)))
}

func TestAppointmentICSEventUIDIncludesTenant(t *testing.T) {
	// The parent feed aggregates appointments across schools, so the UID must be
	// globally unique — appointment IDs repeat per tenant.
	appt := &calModels.Appointment{
		Model:       base.Model{ID: 5},
		TenantModel: base.TenantModel{TenantID: 42},
		Title:       "Termin",
		StartDate:   timezone.NewDate(2026, 1, 5),
		EndDate:     timezone.NewDate(2026, 1, 5),
	}
	event := appointmentICSEvent(appt, nil, nil)
	assert.Equal(t, "appointment-42-5@moto-app.de", event.UID)
}

func TestAppointmentICSEventExportsUnclampedRecurrence(t *testing.T) {
	// A subscription feed is stateless and re-rendered on every poll, so the
	// exported RRULE must carry the series' REAL horizon (open-ended here), never
	// a moving "today + N" UNTIL: advancing that window without bumping SEQUENCE
	// would leave clients frozen on the first horizon they saw, dropping later
	// occurrences. DTSTART is still anchored to the first real occurrence.
	oldMonday := timezone.NewDate(2020, 1, 6)
	appt := &calModels.Appointment{
		Model:       base.Model{ID: 9},
		TenantModel: base.TenantModel{TenantID: 1},
		Title:       "Wöchentlich",
		StartDate:   oldMonday,
		EndDate:     oldMonday,
		StartTime:   helperClock(9, 0),
		EndTime:     helperClock(10, 0),
	}
	rule := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday"},
	}

	event := appointmentICSEvent(appt, rule, nil)
	require.NotNil(t, event.Recurrence)
	assert.Equal(t, oldMonday, event.StartDate)
	assert.Nil(t, event.Recurrence.Until, "open-ended series must export no UNTIL")
	assert.Nil(t, event.Recurrence.Count)

	// A real EndsOn is preserved verbatim (a stable horizon, safe to export).
	endsOn := timezone.NewDate(2026, 3, 30)
	bounded := &calModels.RecurrenceRule{
		Frequency:     calModels.RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday"},
		EndsOn:        &endsOn,
	}
	boundedEvent := appointmentICSEvent(appt, bounded, nil)
	require.NotNil(t, boundedEvent.Recurrence)
	require.NotNil(t, boundedEvent.Recurrence.Until)
	assert.Equal(t, endsOn, *boundedEvent.Recurrence.Until)
}

func TestCalendarGroupingAndDisplayHelpers(t *testing.T) {
	children := []*parentModels.ChildSummary{
		{TenantID: 2, GuardianProfileID: 20},
		{TenantID: 1, GuardianProfileID: 10},
		{TenantID: 1, GuardianProfileID: 10},
	}

	grouped := groupChildrenByTenant(children)
	require.Len(t, grouped[1], 2)
	require.Len(t, grouped[2], 1)
	assert.ElementsMatch(t, []int64{20, 10}, distinctGuardianProfileIDs(children))
	assert.Equal(t, map[int64]struct{}{1: {}, 2: {}}, int64Set([]int64{1, 2, 1}))

	staff := &userModels.Staff{Person: &userModels.Person{FirstName: " Ada ", LastName: " Lovelace "}}
	assert.Equal(t, "Ada   Lovelace", staffName(staff))
	assert.Empty(t, staffName(nil))

	email := "parent@example.test"
	assert.Equal(t, "parent@example.test", guardianName(&userModels.GuardianProfile{Email: &email}))
	assert.Empty(t, guardianName(nil))

	assert.Equal(t, "Kind 44", studentDisplayName(&userModels.Student{Model: base.Model{ID: 44}}))
	assert.Equal(t, "Grace Hopper", studentDisplayName(&userModels.Student{Person: &userModels.Person{FirstName: "Grace", LastName: "Hopper"}}))

	staffID := int64(50)
	guardianID := int64(60)
	assert.Equal(t, "staff:50", recipientKey(calModels.RecipientTypeStaff, &staffID, nil))
	assert.Equal(t, "guardian:60", recipientKey(calModels.RecipientTypeGuardianProfile, nil, &guardianID))
	assert.Equal(t, "staff:0", recipientKey(calModels.RecipientTypeStaff, nil, nil))
}

func TestCalendarSortEvents(t *testing.T) {
	events := []Event{
		{ID: "c", StartDate: "2026-01-02", StartTime: "08:00"},
		{ID: "b", StartDate: "2026-01-01", StartTime: "10:00"},
		{ID: "a", StartDate: "2026-01-01", StartTime: "10:00"},
		{ID: "d", StartDate: "2026-01-01", StartTime: "09:00"},
	}

	sortEvents(events)

	assert.Equal(t, []string{"d", "a", "b", "c"}, []string{events[0].ID, events[1].ID, events[2].ID, events[3].ID})
}

func TestCalendarServiceValidatesInputsBeforeDependencies(t *testing.T) {
	svc := &service{}
	start := timezone.NewDate(2026, 1, 5)
	ctx := context.TODO()

	err := validateWindow(timezone.Date{}, start)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "from and to are required")

	err = validateWindow(start, start.AddDays(-1))
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "to must be on or after from")

	err = validateWindow(start, start.AddDays(maxCalendarWindowDays))
	assert.True(t, errors.Is(err, ErrInvalidRequest))
	assert.Contains(t, err.Error(), "range cannot exceed")

	assert.NoError(t, validateWindow(start, start.AddDays(maxCalendarWindowDays-1)))

	_, err = svc.ListMyStaffEvents(ctx, start, start.AddDays(-1))
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.ListMyParentEvents(ctx, 0, start, start)
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetStaffAppointmentOverview(ctx, 0)
	assert.True(t, errors.Is(err, ErrInvalidRequest))

	_, err = svc.GetParentAppointmentOverview(ctx, 0, 1)
	assert.True(t, errors.Is(err, ErrForbidden))

	_, err = svc.GetParentAppointmentOverview(ctx, 1, 0)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
}
