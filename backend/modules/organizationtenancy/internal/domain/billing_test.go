package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func local(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
}

func TestValidBillingKeyDay(t *testing.T) {
	t.Parallel()
	for day, valid := range map[int]bool{0: false, 1: true, 15: true, 28: true, 29: false, 31: false, -1: false} {
		assert.Equal(t, valid, ValidBillingKeyDay(day), "day %d", day)
	}
}

func TestDueBillingKeyDate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		local   time.Time
		keyDay  int
		keyDate string
		due     bool
	}{
		{"before the key date", local(2026, time.October, 14, 23, 59), 15, "2026-10-15", false},
		{"key date before the capture hour", local(2026, time.October, 15, 5, 59), 15, "2026-10-15", false},
		{"key date at the capture hour", local(2026, time.October, 15, 6, 0), 15, "2026-10-15", true},
		{"after the key date, same month", local(2026, time.October, 31, 23, 0), 15, "2026-10-15", true},
		{"next month before its key date", local(2026, time.November, 1, 12, 0), 15, "2026-11-15", false},
		{"first day as key day", local(2026, time.March, 1, 6, 30), 1, "2026-03-01", true},
		{"28th in February", local(2027, time.February, 28, 7, 0), 28, "2027-02-28", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			keyDate, due := DueBillingKeyDate(tc.local, tc.keyDay)
			assert.Equal(t, tc.keyDate, keyDate)
			assert.Equal(t, tc.due, due)
		})
	}
}

func TestNextBillingKeyDate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "2026-09-25", NextBillingKeyDate(local(2026, time.September, 22, 9, 0), 25))
	assert.Equal(t, "2026-09-22", NextBillingKeyDate(local(2026, time.September, 22, 23, 0), 22))
	assert.Equal(t, "2026-10-15", NextBillingKeyDate(local(2026, time.September, 22, 9, 0), 15))
	assert.Equal(t, "2027-01-15", NextBillingKeyDate(local(2026, time.December, 31, 9, 0), 15))
}

func TestBillingPeriod(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "2026-09-01", BillingPeriod("2026-09-15"))
	assert.Equal(t, "2026-12-01", BillingPeriod("2026-12-28"))
	assert.Empty(t, BillingPeriod("not a date"))
}
