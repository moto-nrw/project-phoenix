package appointments

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetentionCandidateLastDay(t *testing.T) {
	t.Parallel()

	twoDays := &Appointment{StartDate: NewDate(2026, 1, 5), EndDate: NewDate(2026, 1, 6)}
	count := func(n int) *int { return &n }
	date := func(d Date) *Date { return &d }

	tests := []struct {
		name      string
		candidate RetentionCandidate
		want      Date
		ends      bool
	}{
		{name: "single appointment ends on its end date",
			candidate: RetentionCandidate{Appointment: twoDays}, want: NewDate(2026, 1, 6), ends: true},
		{name: "a series bounded by a date ends with the span of the last possible start",
			candidate: RetentionCandidate{Appointment: twoDays, Rule: &RecurrenceRule{
				Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, EndsOn: date(NewDate(2026, 3, 2)),
			}}, want: NewDate(2026, 3, 3), ends: true},
		{name: "a counted series ends with its last occurrence",
			candidate: RetentionCandidate{Appointment: twoDays, Rule: &RecurrenceRule{
				Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, OccurrenceCount: count(3),
			}}, want: NewDate(2026, 1, 20), ends: true},
		{name: "a monthly series on the 31st skips the short months",
			candidate: RetentionCandidate{Appointment: &Appointment{StartDate: NewDate(2026, 1, 31), EndDate: NewDate(2026, 1, 31)},
				Rule: &RecurrenceRule{Frequency: RecurrenceFrequencyMonthly, IntervalCount: 1, MonthDays: []int{31}, OccurrenceCount: count(3)}},
			want: NewDate(2026, 5, 31), ends: true},
		{name: "an override that moves an occurrence later extends the series",
			candidate: RetentionCandidate{Appointment: twoDays, Rule: &RecurrenceRule{
				Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"}, OccurrenceCount: count(3),
			}, LatestOverride: NewDate(2026, 2, 10)}, want: NewDate(2026, 2, 10), ends: true},
		{name: "an endless series never ends",
			candidate: RetentionCandidate{Appointment: twoDays, Rule: &RecurrenceRule{
				Frequency: RecurrenceFrequencyWeekly, IntervalCount: 1, Weekdays: []string{"monday"},
			}}, ends: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ends := tt.candidate.LastDay()
			assert.Equal(t, tt.ends, ends)
			assert.Equal(t, tt.want, got)
		})
	}
}
