package appointments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReminderOccurrences(t *testing.T) {
	t.Parallel()

	window := func(from, to Date) (Date, Date) { return from, to }

	t.Run("a one-off inside the window yields its single date", func(t *testing.T) {
		from, to := window(NewDate(2026, 1, 5), NewDate(2026, 1, 5))
		occurrences := reminderOccurrences(recurrenceFixture(), nil, from, to)

		require.Len(t, occurrences, 1)
		assert.Equal(t, NewDate(2026, 1, 5), occurrences[0])
	})

	t.Run("a one-off outside the window yields nothing", func(t *testing.T) {
		from, to := window(NewDate(2026, 2, 1), NewDate(2026, 2, 2))
		assert.Empty(t, reminderOccurrences(recurrenceFixture(), nil, from, to))
	})

	t.Run("a weekly series yields every matching day in the window", func(t *testing.T) {
		appointment := recurrenceFixture()
		rule := &RecurrenceRule{
			ID:            1,
			AppointmentID: appointment.ID,
			Frequency:     RecurrenceFrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"monday"},
		}

		from, to := window(NewDate(2026, 1, 5), NewDate(2026, 1, 26))
		occurrences := reminderOccurrences(appointment, rule, from, to)

		assert.Equal(t, []Date{
			NewDate(2026, 1, 5),
			NewDate(2026, 1, 12),
			NewDate(2026, 1, 19),
			NewDate(2026, 1, 26),
		}, occurrences)
	})
}
