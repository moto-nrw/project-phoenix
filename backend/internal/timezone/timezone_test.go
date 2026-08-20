package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBerlinTimezoneLoaded(t *testing.T) {
	t.Parallel()

	require.NotNil(t, Berlin, "Berlin timezone should be loaded")
	assert.Equal(t, "Europe/Berlin", Berlin.String())
}

func TestToday(t *testing.T) {
	t.Parallel()

	// Get today's date
	today := Today()

	// Should be midnight
	assert.Equal(t, 0, today.Hour())
	assert.Equal(t, 0, today.Minute())
	assert.Equal(t, 0, today.Second())
	assert.Equal(t, 0, today.Nanosecond())

	// Should be in Berlin timezone
	assert.Equal(t, Berlin, today.Location())

	// Should be today's date
	now := time.Now().In(Berlin)
	assert.Equal(t, now.Year(), today.Year())
	assert.Equal(t, now.Month(), today.Month())
	assert.Equal(t, now.Day(), today.Day())
}

func TestDateOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     time.Time
		wantYear  int
		wantMonth time.Month
		wantDay   int
	}{
		{
			name:      "UTC time in the middle of the day",
			input:     time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18,
		},
		{
			name:      "UTC midnight becomes same day in Berlin",
			input:     time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC),
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18,
		},
		{
			name:      "UTC time that crosses day boundary to Berlin",
			input:     time.Date(2026, 1, 17, 23, 30, 0, 0, time.UTC), // 23:30 UTC = 00:30 CET next day
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18, // Should be 18th in Berlin
		},
		{
			name:      "Berlin time stays same day",
			input:     time.Date(2026, 1, 18, 14, 30, 0, 0, Berlin),
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18,
		},
		{
			name:      "Early morning Berlin time",
			input:     time.Date(2026, 1, 18, 1, 0, 0, 0, Berlin),
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DateOf(tt.input)

			// Check date components
			assert.Equal(t, tt.wantYear, result.Year())
			assert.Equal(t, tt.wantMonth, result.Month())
			assert.Equal(t, tt.wantDay, result.Day())

			// Should be midnight
			assert.Equal(t, 0, result.Hour())
			assert.Equal(t, 0, result.Minute())
			assert.Equal(t, 0, result.Second())
			assert.Equal(t, 0, result.Nanosecond())

			// Should be in Berlin timezone
			assert.Equal(t, Berlin, result.Location())
		})
	}
}

func TestNow(t *testing.T) {
	t.Parallel()

	// Get current time
	now := Now()

	// Should be in Berlin timezone
	assert.Equal(t, Berlin, now.Location())

	// Should be close to actual current time (within 1 second)
	actualNow := time.Now()
	timeDiff := actualNow.Sub(now)
	assert.Less(t, timeDiff.Abs(), time.Second, "Now() should return current time")
}

func TestEndOfDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   time.Time
		wantDay int
		wantUTC time.Time // expected instant in UTC
	}{
		{
			name:    "UTC midnight in winter (CET) returns 23:59:59 Berlin same day",
			input:   time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), // pgx DATE value
			wantDay: 15,
			// 23:59:59 CET = 22:59:59 UTC
			wantUTC: time.Date(2026, 1, 15, 22, 59, 59, 0, time.UTC),
		},
		{
			name:    "UTC midnight in summer (CEST) returns 23:59:59 Berlin same day",
			input:   time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), // pgx DATE value
			wantDay: 10,
			// 23:59:59 CEST = 21:59:59 UTC
			wantUTC: time.Date(2026, 7, 10, 21, 59, 59, 0, time.UTC),
		},
		{
			name:    "Berlin input preserves calendar date",
			input:   time.Date(2026, 3, 20, 14, 30, 0, 0, Berlin),
			wantDay: 20,
			wantUTC: time.Date(2026, 3, 20, 22, 59, 59, 0, time.UTC), // March 20 is CET
		},
		{
			name:    "DST transition day — spring forward (March 29 2026)",
			input:   time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			wantDay: 29,
			// March 29 2026 clocks spring forward, end of day is CEST
			wantUTC: time.Date(2026, 3, 29, 21, 59, 59, 0, time.UTC),
		},
		{
			name:    "DST transition day — fall back (October 25 2026)",
			input:   time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC),
			wantDay: 25,
			// October 25 2026 clocks fall back, end of day is CET
			wantUTC: time.Date(2026, 10, 25, 22, 59, 59, 0, time.UTC),
		},
		{
			name:    "New Year's Eve",
			input:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantDay: 31,
			wantUTC: time.Date(2026, 12, 31, 22, 59, 59, 0, time.UTC), // CET
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EndOfDay(tt.input)

			// Must be in Berlin timezone
			assert.Equal(t, Berlin, result.Location())

			// Must be 23:59:59 Berlin time
			assert.Equal(t, 23, result.Hour())
			assert.Equal(t, 59, result.Minute())
			assert.Equal(t, 59, result.Second())
			assert.Equal(t, 0, result.Nanosecond())

			// Calendar date must match input
			assert.Equal(t, tt.wantDay, result.Day())

			// UTC instant must be correct (verifies CET vs CEST offset)
			assert.True(t, tt.wantUTC.Equal(result),
				"expected UTC %v but got %v", tt.wantUTC, result.UTC())
		})
	}
}

func TestFormatBerlinClock(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil pointer", func(t *testing.T) {
		assert.Nil(t, FormatBerlinClock(nil))
	})

	t.Run("UTC input is converted to Berlin wall clock (CEST)", func(t *testing.T) {
		// 2026-04-27 06:05 UTC = 08:05 CEST
		t1 := time.Date(2026, 4, 27, 6, 5, 0, 0, time.UTC)
		got := FormatBerlinClock(&t1)
		require.NotNil(t, got)
		assert.Equal(t, "08:05", *got)
	})

	t.Run("UTC input is converted to Berlin wall clock (CET)", func(t *testing.T) {
		// 2026-01-15 13:42 UTC = 14:42 CET
		t1 := time.Date(2026, 1, 15, 13, 42, 0, 0, time.UTC)
		got := FormatBerlinClock(&t1)
		require.NotNil(t, got)
		assert.Equal(t, "14:42", *got)
	})

	t.Run("input already in Berlin formats wall clock as-is", func(t *testing.T) {
		t1 := time.Date(2026, 1, 15, 7, 12, 0, 0, Berlin)
		got := FormatBerlinClock(&t1)
		require.NotNil(t, got)
		assert.Equal(t, "07:12", *got)
	})

	t.Run("zero pads single digit hours and minutes", func(t *testing.T) {
		t1 := time.Date(2026, 1, 15, 4, 3, 0, 0, Berlin)
		got := FormatBerlinClock(&t1)
		require.NotNil(t, got)
		assert.Equal(t, "04:03", *got)
	})
}
