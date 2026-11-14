package calendar_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func TestParseFileDateReadsGermanAndISODatesAndDropsTheTime(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]calendar.Date{
		"31.12.2024":       calendar.NewDate(2024, 12, 31),
		"1.2.2024":         calendar.NewDate(2024, 2, 1),
		"31/12/2024":       calendar.NewDate(2024, 12, 31),
		"2024-12-31":       calendar.NewDate(2024, 12, 31),
		"01.03.2024 00:00": calendar.NewDate(2024, 3, 1),
		"2024-03-01T14:05": calendar.NewDate(2024, 3, 1),
	} {
		got, ok := calendar.ParseFileDate(value)
		assert.True(t, ok, value)
		assert.Equal(t, want, got, value)
	}
	for _, value := range []string{"", "31.12.24", "31.02.2024", "heute", "12.2024", "2024-13-01"} {
		_, ok := calendar.ParseFileDate(value)
		assert.False(t, ok, value)
	}
}
