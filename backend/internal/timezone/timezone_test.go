package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBerlinTimezoneLoaded(t *testing.T) {
	require.NotNil(t, Berlin, "Berlin timezone should be loaded")
	assert.Equal(t, "Europe/Berlin", Berlin.String())
}

func TestToday(t *testing.T) {
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
	// Get current time
	now := Now()

	// Should be in Berlin timezone
	assert.Equal(t, Berlin, now.Location())

	// Should be close to actual current time (within 1 second)
	actualNow := time.Now()
	timeDiff := actualNow.Sub(now)
	assert.Less(t, timeDiff.Abs(), time.Second, "Now() should return current time")
}

func TestDateOfUTC(t *testing.T) {
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
			name:      "UTC time that crosses day boundary in Berlin",
			input:     time.Date(2026, 1, 17, 23, 30, 0, 0, time.UTC), // 23:30 UTC = 00:30 CET next day
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18, // Should be 18th (Berlin date, but UTC timezone)
		},
		{
			name:      "Berlin time afternoon",
			input:     time.Date(2026, 1, 18, 14, 30, 0, 0, Berlin),
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18,
		},
		{
			name:      "Berlin time early morning",
			input:     time.Date(2026, 1, 18, 1, 0, 0, 0, Berlin),
			wantYear:  2026,
			wantMonth: time.January,
			wantDay:   18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DateOfUTC(tt.input)

			// Check date components (Berlin date)
			assert.Equal(t, tt.wantYear, result.Year())
			assert.Equal(t, tt.wantMonth, result.Month())
			assert.Equal(t, tt.wantDay, result.Day())

			// Should be midnight
			assert.Equal(t, 0, result.Hour())
			assert.Equal(t, 0, result.Minute())
			assert.Equal(t, 0, result.Second())
			assert.Equal(t, 0, result.Nanosecond())

			// Should be in UTC timezone (not Berlin!)
			assert.Equal(t, time.UTC, result.Location())
		})
	}
}

func TestDateOfUTC_DifferentFromDateOf(t *testing.T) {
	// Create a time that would have the same date in both UTC and Berlin
	testTime := time.Date(2026, 1, 18, 12, 0, 0, 0, time.UTC)

	dateOf := DateOf(testTime)
	dateOfUTC := DateOfUTC(testTime)

	// Both should have same year/month/day
	assert.Equal(t, dateOf.Year(), dateOfUTC.Year())
	assert.Equal(t, dateOf.Month(), dateOfUTC.Month())
	assert.Equal(t, dateOf.Day(), dateOfUTC.Day())

	// But different timezones
	assert.Equal(t, Berlin, dateOf.Location())
	assert.Equal(t, time.UTC, dateOfUTC.Location())

	// They represent the same date but different instants in time
	assert.NotEqual(t, dateOf.Unix(), dateOfUTC.Unix())
}
