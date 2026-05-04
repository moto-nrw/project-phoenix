package api

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

func TestMostRecentWeekday(t *testing.T) {
	t.Parallel()
	loc := time.UTC

	tests := []struct {
		name   string
		from   time.Time
		target time.Weekday
		want   time.Time
	}{
		{
			name:   "from Monday targeting Monday returns same day",
			from:   time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
			target: time.Monday,
			want:   time.Date(2026, 5, 4, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Wednesday targeting Tuesday returns yesterday",
			from:   time.Date(2026, 5, 6, 0, 0, 0, 0, loc),
			target: time.Tuesday,
			want:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Tuesday targeting Wednesday wraps to previous week",
			from:   time.Date(2026, 5, 5, 0, 0, 0, 0, loc),
			target: time.Wednesday,
			want:   time.Date(2026, 4, 29, 0, 0, 0, 0, loc),
		},
		{
			name:   "from Sunday targeting Monday returns previous Monday",
			from:   time.Date(2026, 5, 3, 0, 0, 0, 0, loc),
			target: time.Monday,
			want:   time.Date(2026, 4, 27, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mostRecentWeekday(tt.from, tt.target)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.target, got.Weekday(),
				"resolved date must land on the requested weekday")
		})
	}
}

func TestBuildSession_RealisticShape(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	day := time.Date(2026, 5, 4, 0, 0, 0, 0, loc)

	tenantID := int64(42)
	staffID := int64(9999)

	// Seed RNG deterministically so the test does not flake.
	rng := rand.New(rand.NewPCG(1, 2))

	session, breaks, err := buildSession(rng, tenantID, staffID, day, loc)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Len(t, breaks, 1)

	assert.Equal(t, tenantID, session.TenantID)
	assert.Equal(t, staffID, session.StaffID)
	assert.Equal(t, staffID, session.CreatedBy)
	assert.Equal(t, day, session.Date)
	assert.Contains(t, []string{
		activeModels.WorkSessionStatusPresent,
		activeModels.WorkSessionStatusHomeOffice,
	}, session.Status)
	assert.Equal(t, day.Year(), session.CheckInTime.Year())
	assert.Equal(t, day.Month(), session.CheckInTime.Month())
	assert.Equal(t, day.Day(), session.CheckInTime.Day())
	assert.InDelta(t, 8, session.CheckInTime.Hour(), 1,
		"check-in should land near 08:00")

	if session.CheckOutTime != nil {
		assert.False(t, session.AutoCheckedOut, "explicit checkout means not auto-closed")
		assert.True(t, session.CheckOutTime.After(session.CheckInTime),
			"check-out must be after check-in")
	} else {
		assert.True(t, session.AutoCheckedOut, "missing checkout means auto-closed flag set")
	}

	br := breaks[0]
	assert.Equal(t, tenantID, br.TenantID)
	require.NotNil(t, br.EndedAt)
	assert.True(t, br.EndedAt.After(br.StartedAt))
	assert.Equal(t, br.DurationMinutes, session.BreakMinutes,
		"session should aggregate the same break minutes as the break row")
	assert.GreaterOrEqual(t, br.DurationMinutes, 25)
	assert.LessOrEqual(t, br.DurationMinutes, 35)
}
