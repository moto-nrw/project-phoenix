package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBerlinTimezoneLoaded(t *testing.T) {
	require.NotNil(t, Berlin)
	assert.Equal(t, "Europe/Berlin", Berlin.String())
}

func TestDateOf_MidnightBoundary(t *testing.T) {
	// This is the critical bug case: 00:30 CET should be date 2026-01-18
	// but UTC().Truncate(24h) would return 2026-01-17

	tests := []struct {
		name     string
		input    time.Time
		wantDate string // YYYY-MM-DD
	}{
		{
			name:     "00:30 CET should be same day",
			input:    time.Date(2026, 1, 18, 0, 30, 0, 0, Berlin),
			wantDate: "2026-01-18",
		},
		{
			name:     "23:30 UTC (= 00:30 CET next day) should be next day in Berlin",
			input:    time.Date(2026, 1, 17, 23, 30, 0, 0, time.UTC),
			wantDate: "2026-01-18", // 23:30 UTC = 00:30 CET on Jan 18
		},
		{
			name:     "22:30 UTC (= 23:30 CET) should be same day",
			input:    time.Date(2026, 1, 17, 22, 30, 0, 0, time.UTC),
			wantDate: "2026-01-17",
		},
		{
			name:     "midday is unambiguous",
			input:    time.Date(2026, 1, 18, 12, 0, 0, 0, Berlin),
			wantDate: "2026-01-18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DateOf(tt.input)
			assert.Equal(t, tt.wantDate, got.Format("2006-01-02"))
			assert.Equal(t, 0, got.Hour(), "should be midnight")
			assert.Equal(t, 0, got.Minute(), "should be midnight")
			assert.Equal(t, Berlin, got.Location(), "should be in Berlin timezone")
		})
	}
}

func TestDateOf_VsWrongTruncateMethod(t *testing.T) {
	// Demonstrate why UTC().Truncate(24h) is wrong

	// 00:30 CET on Jan 18 = 23:30 UTC on Jan 17
	checkinTime := time.Date(2026, 1, 17, 23, 30, 0, 0, time.UTC)

	// WRONG: Old method using UTC truncate
	wrongDate := checkinTime.UTC().Truncate(24 * time.Hour)
	assert.Equal(t, "2026-01-17", wrongDate.Format("2006-01-02"), "UTC truncate gives wrong date")

	// CORRECT: Our method using Berlin timezone
	correctDate := DateOf(checkinTime)
	assert.Equal(t, "2026-01-18", correctDate.Format("2006-01-02"), "DateOf gives correct date")
}

func TestToday(t *testing.T) {
	today := Today()

	// Should be midnight
	assert.Equal(t, 0, today.Hour())
	assert.Equal(t, 0, today.Minute())
	assert.Equal(t, 0, today.Second())

	// Should be in Berlin timezone
	assert.Equal(t, Berlin, today.Location())

	// Should be today's date
	now := time.Now().In(Berlin)
	assert.Equal(t, now.Year(), today.Year())
	assert.Equal(t, now.Month(), today.Month())
	assert.Equal(t, now.Day(), today.Day())
}

func TestNow(t *testing.T) {
	now := Now()
	assert.Equal(t, Berlin, now.Location())
}
