package appointments

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMatchesRuleWeeklyIntervalAnchorsToCalendarWeeks(t *testing.T) {
	t.Parallel()

	// Wednesday start, biweekly, on Mondays and Wednesdays. Intervals must be
	// counted in calendar weeks (Mon–Sun), not 7-day blocks from the Wed start.
	start := NewDate(2026, 1, 7) // Wednesday (Jan 5 is the Monday)
	rule := &RecurrenceRule{
		Frequency:     RecurrenceFrequencyWeekly,
		IntervalCount: 2,
		Weekdays:      []string{"monday", "wednesday"},
	}

	// Week 0 (the start week): the Wednesday start is included.
	assert.True(t, matchesRule(start, NewDate(2026, 1, 7), rule))
	// Week 1 (odd) is skipped entirely — including the Monday only 5 days after
	// the start, which the old day-count logic wrongly treated as "week 0".
	assert.False(t, matchesRule(start, NewDate(2026, 1, 12), rule))
	assert.False(t, matchesRule(start, NewDate(2026, 1, 14), rule))
	// Week 2 (even) includes both weekdays — the Monday the old logic dropped.
	assert.True(t, matchesRule(start, NewDate(2026, 1, 19), rule))
	assert.True(t, matchesRule(start, NewDate(2026, 1, 21), rule))
}

func TestCalendarRecurrenceExpansion(t *testing.T) {
	t.Parallel()

	appointment := recurrenceFixture()
	endsOn := NewDate(2026, 1, 31)
	count := 3

	weekly := &RecurrenceRule{
		Frequency:     RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{" monday ", "WEDNESDAY"},
		EndsOn:        &endsOn,
	}
	assert.Equal(t, []Date{
		NewDate(2026, 1, 5),
		NewDate(2026, 1, 7),
		NewDate(2026, 1, 12),
	}, ExpandOccurrences(appointment, weekly, NewDate(2026, 1, 5), NewDate(2026, 1, 12)))

	monthly := &RecurrenceRule{
		Frequency:       RecurrenceFrequencyMonthly,
		IntervalCount:   1,
		MonthDays:       []int{5, 20},
		OccurrenceCount: &count,
	}
	assert.Equal(t, []Date{
		NewDate(2026, 1, 5),
		NewDate(2026, 1, 20),
		NewDate(2026, 2, 5),
	}, ExpandOccurrences(appointment, monthly, NewDate(2026, 1, 1), NewDate(2026, 3, 31)))

	yearly := &RecurrenceRule{Frequency: RecurrenceFrequencyYearly, IntervalCount: 1}
	daily := &RecurrenceRule{Frequency: RecurrenceFrequencyDaily, IntervalCount: 2}
	weeklyDefault := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1}
	monthlyDefault := &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 1}

	assert.True(t, matchesRule(Date(appointment.StartDate), NewDate(2026, 1, 7), daily))
	assert.False(t, matchesRule(Date(appointment.StartDate), NewDate(2026, 1, 6), daily))
	assert.True(t, matchesRule(Date(appointment.StartDate), NewDate(2026, 1, 12), weeklyDefault))
	assert.True(t, matchesRule(Date(appointment.StartDate), NewDate(2026, 2, 5), monthlyDefault))
	assert.True(t, matchesRule(Date(appointment.StartDate), NewDate(2027, 1, 5), yearly))
	assert.False(t, matchesRule(Date(appointment.StartDate), NewDate(2027, 1, 6), yearly))
	assert.False(t, matchesRule(Date(appointment.StartDate), NewDate(2026, 1, 4), yearly))
	assert.False(t, matchesRule(Date(appointment.StartDate), NewDate(2026, 1, 5), &RecurrenceRule{Frequency: "never", IntervalCount: 1}))
}

func TestFirstRecurrenceOccurrence(t *testing.T) {
	t.Parallel()

	appt := func(d Date) *Appointment {
		return &Appointment{StartDate: Date(d), EndDate: Date(d)}
	}
	ptr := func(d Date) *Date {
		converted := Date(d)
		return &converted
	}

	tuesday := NewDate(2026, 1, 6) // 2026-01-05 is a Monday
	weeklyMondayInterval := func(n int) *RecurrenceRule {
		return &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: n, Weekdays: []string{"monday"}}
	}

	tests := []struct {
		name  string
		appt  *Appointment
		rule  *RecurrenceRule
		want  Date
		w4dok bool
	}{
		{
			name: "daily matches start",
			appt: appt(NewDate(2026, 3, 4)),
			rule: &RecurrenceRule{Frequency: RecurrenceFrequencyDaily, IntervalCount: 5},
			want: NewDate(2026, 3, 4), w4dok: true,
		},
		{
			name: "yearly matches start",
			appt: appt(NewDate(2026, 3, 4)),
			rule: &RecurrenceRule{Frequency: RecurrenceFrequencyYearly, IntervalCount: 3},
			want: NewDate(2026, 3, 4), w4dok: true,
		},
		{
			name: "weekly without weekdays matches start",
			appt: appt(tuesday),
			rule: &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1},
			want: tuesday, w4dok: true,
		},
		{
			name: "weekly weekday excludes start weekday -> next matching weekday",
			appt: appt(tuesday),
			rule: weeklyMondayInterval(1),
			want: NewDate(2026, 1, 12), w4dok: true,
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
			appt: appt(NewDate(2026, 1, 31)),
			rule: &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 24, MonthDays: []int{30}},
			want: NewDate(2028, 1, 30), w4dok: true,
		},
		{
			name:  "monthly February-only day 31 is impossible",
			appt:  appt(NewDate(2026, 2, 15)),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 12, MonthDays: []int{31}},
			w4dok: false,
		},
		{
			name:  "monthly February-only day 30 is impossible",
			appt:  appt(NewDate(2026, 2, 15)),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 12, MonthDays: []int{30}},
			w4dok: false,
		},
		{
			name: "monthly February day 29 resolves to the next leap year",
			appt: appt(NewDate(2025, 2, 1)),
			rule: &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 12, MonthDays: []int{29}},
			want: NewDate(2028, 2, 29), w4dok: true,
		},
		{
			name:  "first occurrence after EndsOn is rejected",
			appt:  appt(tuesday),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: ptr(NewDate(2026, 1, 8))},
			w4dok: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FirstRecurrenceOccurrence(tc.appt, tc.rule)
			assert.Equal(t, tc.w4dok, ok)
			if tc.w4dok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestOccurrenceExists(t *testing.T) {
	t.Parallel()

	start := NewDate(2026, 1, 5) // Monday
	appt := &Appointment{StartDate: Date(start), EndDate: Date(start)}
	weeklyMonday := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}}

	assert.True(t, OccurrenceExists(appt, weeklyMonday, NewDate(2026, 1, 12)))  // valid Monday
	assert.False(t, OccurrenceExists(appt, weeklyMonday, NewDate(2026, 1, 13))) // Tuesday
	assert.False(t, OccurrenceExists(appt, weeklyMonday, NewDate(2026, 1, 4)))  // before start

	// A far-future valid occurrence is accepted arithmetically — no day-by-day
	// expansion from the series start.
	daily := &RecurrenceRule{Frequency: RecurrenceFrequencyDaily, IntervalCount: 1}
	assert.True(t, OccurrenceExists(appt, daily, start.AddDays(100000)))

	// EndsOn bound.
	endsOn := NewDate(2026, 1, 19)
	bounded := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: &endsOn}
	assert.True(t, OccurrenceExists(appt, bounded, NewDate(2026, 1, 19)))
	assert.False(t, OccurrenceExists(appt, bounded, NewDate(2026, 1, 26))) // after EndsOn

	// Count bound: only the first two occurrences exist.
	count := 2
	counted := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, OccurrenceCount: &count}
	assert.True(t, OccurrenceExists(appt, counted, NewDate(2026, 1, 5)))   // 1st
	assert.True(t, OccurrenceExists(appt, counted, NewDate(2026, 1, 12)))  // 2nd
	assert.False(t, OccurrenceExists(appt, counted, NewDate(2026, 1, 19))) // 3rd — beyond count
}

func TestHasOccurrenceInWindow(t *testing.T) {
	t.Parallel()

	oldStart := NewDate(2020, 1, 6) // an old Monday
	appt := &Appointment{StartDate: Date(oldStart), EndDate: Date(oldStart)}
	weeklyMonday := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}}

	from := NewDate(2026, 7, 1)
	to := NewDate(2026, 8, 1)

	// An old open-ended series still overlaps a current window.
	assert.True(t, HasOccurrenceInWindow(appt, weeklyMonday, from, to))

	// EndsOn in the past → no overlap now.
	past := NewDate(2020, 3, 1)
	ended := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: &past}
	assert.False(t, HasOccurrenceInWindow(appt, ended, from, to))

	// A count-bounded old series whose occurrences are all in the past → no overlap.
	count := 3
	counted := &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, OccurrenceCount: &count}
	assert.False(t, HasOccurrenceInWindow(appt, counted, from, to))

	// A multi-day occurrence starting just before the window but spanning into it
	// is detected (window scan starts early enough to catch it).
	multiDay := &Appointment{StartDate: NewDate(2020, 1, 1), EndDate: NewDate(2020, 1, 3)}
	daily := &RecurrenceRule{Frequency: RecurrenceFrequencyDaily, IntervalCount: 1}
	assert.True(t, HasOccurrenceInWindow(multiDay, daily, NewDate(2026, 7, 1), NewDate(2026, 7, 1)))
}

func TestBoundedRecurrenceDates(t *testing.T) {
	t.Parallel()

	appt := func(start, end Date) *Appointment {
		return &Appointment{StartDate: Date(start), EndDate: Date(end)}
	}
	same := func(d Date) *Appointment { return appt(d, d) }
	ptr := func(d Date) *Date {
		converted := Date(d)
		return &converted
	}

	tests := []struct {
		name  string
		appt  *Appointment
		rule  *RecurrenceRule
		limit int
		want  []Date
	}{
		{
			name:  "daily interval steps by period",
			appt:  same(NewDate(2026, 1, 5)),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyDaily, IntervalCount: 3},
			limit: 4,
			want: []Date{
				NewDate(2026, 1, 5), NewDate(2026, 1, 8),
				NewDate(2026, 1, 11), NewDate(2026, 1, 14),
			},
		},
		{
			name:  "weekly multiple weekdays in order",
			appt:  same(NewDate(2026, 1, 5)), // Monday
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday", "wednesday"}},
			limit: 5,
			want: []Date{
				NewDate(2026, 1, 5), NewDate(2026, 1, 7),
				NewDate(2026, 1, 12), NewDate(2026, 1, 14),
				NewDate(2026, 1, 19),
			},
		},
		{
			name:  "weekly interval skips whole calendar weeks",
			appt:  same(NewDate(2026, 1, 6)), // Tuesday, Monday is before start
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyWeekly, IntervalCount: 2, Weekdays: []string{"monday"}},
			limit: 3,
			want: []Date{
				NewDate(2026, 1, 19), NewDate(2026, 2, 2),
				NewDate(2026, 2, 16),
			},
		},
		{
			name:  "monthly day 31 skips short months without day-walking",
			appt:  same(NewDate(2026, 1, 31)),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{31}},
			limit: 5,
			want: []Date{
				NewDate(2026, 1, 31), NewDate(2026, 3, 31),
				NewDate(2026, 5, 31), NewDate(2026, 7, 31),
				NewDate(2026, 8, 31),
			},
		},
		{
			name:  "yearly Feb 29 lands only on leap years",
			appt:  same(NewDate(2024, 2, 29)),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyYearly, IntervalCount: 1},
			limit: 3,
			want: []Date{
				NewDate(2024, 2, 29), NewDate(2028, 2, 29),
				NewDate(2032, 2, 29),
			},
		},
		{
			name:  "EndsOn cuts the series short",
			appt:  same(NewDate(2026, 1, 5)),
			rule:  &RecurrenceRule{Frequency: RecurrenceFrequencyDaily, IntervalCount: 2, EndsOn: ptr(NewDate(2026, 1, 10))},
			limit: 10,
			want: []Date{
				NewDate(2026, 1, 5), NewDate(2026, 1, 7),
				NewDate(2026, 1, 9),
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
	t.Parallel()

	start := NewDate(2026, 1, 5) // Monday
	appt := &Appointment{StartDate: Date(start), EndDate: Date(start)}

	// A repeated weekday must not emit the same date twice (which would consume
	// two of the occurrence_count slots for one real occurrence).
	weeklyDup := &RecurrenceRule{
		Frequency:     RecurrenceFrequencyWeekly,
		IntervalCount: 1,
		Weekdays:      []string{"monday", "monday"},
	}
	assert.Equal(t, []Date{
		NewDate(2026, 1, 5), NewDate(2026, 1, 12),
		NewDate(2026, 1, 19),
	}, boundedRecurrenceDates(appt, weeklyDup, 3))

	// A repeated month day likewise collapses to one occurrence.
	monthlyDup := &RecurrenceRule{
		Frequency:     RecurrenceFrequencyMonthly,
		IntervalCount: 1,
		MonthDays:     []int{5, 5, 20},
	}
	assert.Equal(t, []Date{
		NewDate(2026, 1, 5), NewDate(2026, 1, 20),
		NewDate(2026, 2, 5),
	}, boundedRecurrenceDates(appt, monthlyDup, 3))
}

func TestOccurrenceExistsSparseCountBounded(t *testing.T) {
	t.Parallel()

	// A count-bounded monthly "day 31" series: the reachable occurrences are the
	// first N months that actually have a 31st. Membership must be exact without
	// scanning every day since the (possibly old) start.
	start := NewDate(2026, 1, 31)
	appt := &Appointment{StartDate: Date(start), EndDate: Date(start)}
	count := 5
	rule := &RecurrenceRule{
		Frequency:       RecurrenceFrequencyMonthly,
		IntervalCount:   1,
		MonthDays:       []int{31},
		OccurrenceCount: &count,
	}

	assert.True(t, OccurrenceExists(appt, rule, NewDate(2026, 8, 31)))   // 5th occurrence
	assert.False(t, OccurrenceExists(appt, rule, NewDate(2026, 10, 31))) // 6th — beyond count
	assert.False(t, OccurrenceExists(appt, rule, NewDate(2026, 2, 28)))  // not a rule date
}

func TestHasOccurrenceInWindowSparseCountBounded(t *testing.T) {
	t.Parallel()

	// A very old count-bounded yearly Feb-29 series: HasOccurrenceInWindow must
	// decide overlap by enumerating the (few) occurrences per period, never by
	// walking the ~decades of days between the start and the feed window.
	start := NewDate(2000, 2, 29)
	appt := &Appointment{StartDate: Date(start), EndDate: Date(start)}
	count := 10 // 2000, 2004, 2008, ... 2036
	rule := &RecurrenceRule{
		Frequency:       RecurrenceFrequencyYearly,
		IntervalCount:   1,
		OccurrenceCount: &count,
	}

	// 2028-02-29 is the 8th occurrence — inside the count, so a window over it hits.
	assert.True(t, HasOccurrenceInWindow(appt, rule, NewDate(2028, 2, 1), NewDate(2028, 3, 1)))
	// 2040 is past the 10th occurrence (2036) → no overlap.
	assert.False(t, HasOccurrenceInWindow(appt, rule, NewDate(2040, 1, 1), NewDate(2040, 12, 31)))
}

func recurrenceFixture() *Appointment {
	return &Appointment{ID: 42, OrganizerStaffID: 7, Title: "Planning", StartDate: NewDate(2026, 1, 5), EndDate: NewDate(2026, 1, 5), StartTime: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), DeliveryMode: DeliveryModeRSVPRequired, OverviewVisibility: OverviewVisibilityOrganizer}
}
