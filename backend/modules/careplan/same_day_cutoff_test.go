package careplan_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func berlinAt(year int, month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(year, month, day, hour, minute, second, 0, calendar.Berlin)
}

func sameDayCutoff(t *testing.T, clock string, now time.Time) careplan.SameDayCutoff {
	t.Helper()
	cutoff, err := careplan.NewSameDayCutoff(clock, now)
	require.NoError(t, err)
	return cutoff
}

// #3163: with a cutoff of 11:00, today is open at 10:59 and at exactly 11:00,
// and closed from the first moment after it. Later days never close.
func TestSameDayCutoffBoundaries(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, 8, 24)
	for _, tc := range []struct {
		name   string
		now    time.Time
		date   calendar.Date
		closed bool
	}{
		{"heute 10:59", berlinAt(2026, 8, 24, 10, 59, 0), today, false},
		{"heute genau 11:00", berlinAt(2026, 8, 24, 11, 0, 0), today, false},
		{"heute 11:00:01", berlinAt(2026, 8, 24, 11, 0, 1), today, true},
		{"heute 11:01", berlinAt(2026, 8, 24, 11, 1, 0), today, true},
		{"heute 23:59", berlinAt(2026, 8, 24, 23, 59, 0), today, true},
		{"morgen um 11:01", berlinAt(2026, 8, 24, 11, 1, 0), today.AddDays(1), false},
		{"in einer Woche um 11:01", berlinAt(2026, 8, 24, 11, 1, 0), today.AddDays(7), false},
		{"nach Mitternacht ist der neue Tag offen", berlinAt(2026, 8, 25, 0, 1, 0), today.AddDays(1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.closed, sameDayCutoff(t, "11:00", tc.now).Closed(tc.date))
		})
	}
}

// Without a cutoff nothing is ever closed, which keeps existing schools as
// they are.
func TestSameDayCutoffEmptyNeverCloses(t *testing.T) {
	t.Parallel()

	now := berlinAt(2026, 8, 24, 23, 59, 59)
	assert.False(t, sameDayCutoff(t, "", now).Closed(calendar.NewDate(2026, 8, 24)))
	assert.False(t, careplan.SameDayCutoff{}.Closed(calendar.NewDate(2026, 8, 24)))
}

func TestSameDayCutoffTrimsResolvedClock(t *testing.T) {
	t.Parallel()

	cutoff := sameDayCutoff(t, " \t11:00\n", berlinAt(2026, 8, 24, 11, 1, 0))
	assert.Equal(t, "11:00", cutoff.Clock)
	assert.True(t, cutoff.Closed(calendar.NewDate(2026, 8, 24)))
}

func TestSameDayCutoffAtUsesTimeAtValidation(t *testing.T) {
	t.Parallel()

	now := berlinAt(2026, 8, 24, 10, 59, 0)
	cutoff, err := careplan.NewSameDayCutoffAt("11:00", func() time.Time { return now })
	require.NoError(t, err)

	assert.False(t, cutoff.Closed(calendar.NewDate(2026, 8, 24)))
	now = berlinAt(2026, 8, 24, 11, 1, 0)
	assert.True(t, cutoff.Closed(calendar.NewDate(2026, 8, 24)))
}

// The cutoff is a Berlin wall-clock time. The same instant in UTC sits one
// hour apart from it in winter and two hours in summer, and on both
// changeover days the check still reads the Berlin clock.
func TestSameDayCutoffUsesBerlinTimeAcrossDaylightSaving(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		nowUTC time.Time
		date   calendar.Date
		closed bool
	}{
		// Winter (CET, UTC+1): 11:00 Berlin is 10:00 UTC.
		{"Winter 10:59 Berlin", time.Date(2026, 1, 15, 9, 59, 0, 0, time.UTC), calendar.NewDate(2026, 1, 15), false},
		{"Winter 11:01 Berlin", time.Date(2026, 1, 15, 10, 1, 0, 0, time.UTC), calendar.NewDate(2026, 1, 15), true},
		// Summer (CEST, UTC+2): 11:00 Berlin is 09:00 UTC.
		{"Sommer 10:59 Berlin", time.Date(2026, 7, 1, 8, 59, 0, 0, time.UTC), calendar.NewDate(2026, 7, 1), false},
		{"Sommer 11:01 Berlin", time.Date(2026, 7, 1, 9, 1, 0, 0, time.UTC), calendar.NewDate(2026, 7, 1), true},
		// 10:01 UTC in summer is already 12:01 Berlin — a UTC reading would say open.
		{"Sommer 10:01 UTC ist 12:01 Berlin", time.Date(2026, 7, 1, 10, 1, 0, 0, time.UTC), calendar.NewDate(2026, 7, 1), true},
		// Changeover days: 29 March 2026 (clocks go forward), 25 October 2026 (back).
		{"Umstellung März 10:59 Berlin", time.Date(2026, 3, 29, 8, 59, 0, 0, time.UTC), calendar.NewDate(2026, 3, 29), false},
		{"Umstellung März 11:01 Berlin", time.Date(2026, 3, 29, 9, 1, 0, 0, time.UTC), calendar.NewDate(2026, 3, 29), true},
		{"Umstellung Oktober 10:59 Berlin", time.Date(2026, 10, 25, 9, 59, 0, 0, time.UTC), calendar.NewDate(2026, 10, 25), false},
		{"Umstellung Oktober 11:01 Berlin", time.Date(2026, 10, 25, 10, 1, 0, 0, time.UTC), calendar.NewDate(2026, 10, 25), true},
		// 23:30 UTC on 30 June is already 1 July in Berlin: 30 June is no
		// longer today and never closes, even though UTC still shows it.
		{"UTC-Tag ist in Berlin schon gestern", time.Date(2026, 6, 30, 23, 30, 0, 0, time.UTC), calendar.NewDate(2026, 6, 30), false},
		{"Berliner Tag nach UTC-Mitternacht", time.Date(2026, 6, 30, 23, 30, 0, 0, time.UTC), calendar.NewDate(2026, 7, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.closed, sameDayCutoff(t, "11:00", tc.nowUTC).Closed(tc.date))
		})
	}

	// Early cutoff just after Berlin midnight while UTC still shows the day
	// before: the Berlin day is the one that closes.
	early := sameDayCutoff(t, "00:15", time.Date(2026, 6, 30, 22, 30, 0, 0, time.UTC)) // 00:30 Berlin, 1 July
	assert.True(t, early.Closed(calendar.NewDate(2026, 7, 1)))
	assert.False(t, early.Closed(calendar.NewDate(2026, 7, 2)))
}

func TestNewSameDayCutoffRejectsMalformedClock(t *testing.T) {
	t.Parallel()

	for _, clock := range []string{"11", "25:00", "11:60", "elf"} {
		_, err := careplan.NewSameDayCutoff(clock, berlinAt(2026, 8, 24, 10, 0, 0))
		require.Error(t, err, clock)
	}
	// A hand-built value that bypassed validation fails closed for today only.
	broken := careplan.SameDayCutoff{Clock: "elf", Now: berlinAt(2026, 8, 24, 10, 0, 0)}
	assert.True(t, broken.Closed(calendar.NewDate(2026, 8, 24)))
	assert.False(t, broken.Closed(calendar.NewDate(2026, 8, 25)))
}
