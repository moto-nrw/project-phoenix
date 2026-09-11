package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarDateRoundTrip(t *testing.T) {
	t.Parallel()

	date := NewCalendarDate(2026, time.March, 29)
	assert.Equal(t, "2026-03-29", string(date))
	assert.Equal(t, "2026-03-30", date.AddDays(1).String())
	assert.Equal(t, 1, date.DaysUntil(date.AddDays(1)))

	encoded, err := json.Marshal(date)
	require.NoError(t, err)
	assert.JSONEq(t, `"2026-03-29"`, string(encoded))

	var decoded CalendarDate
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, date, decoded)
}

func TestCalendarDateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := ParseCalendarDate("2026-02-30")
	require.Error(t, err)
}

func TestCalendarDateZeroMarshalsAsNull(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(CalendarDate(""))
	require.NoError(t, err)
	assert.Equal(t, "null", string(encoded))
}
