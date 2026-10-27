package timetracking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// laborDay is Monday 2026-10-05, a fixed Berlin workday.
var laborDay = calendar.NewDate(2026, time.October, 5)

func laborAt(day calendar.Date, hour, minute int) time.Time {
	return day.BerlinMidnight().Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

func closedBlock(id int64, start, end time.Time) *WorkSession {
	return &WorkSession{ID: id, CheckInTime: start, CheckOutTime: &end}
}

func endedBreak(start time.Time, minutes int) *WorkSessionBreak {
	end := start.Add(time.Duration(minutes) * time.Minute)
	return &WorkSessionBreak{StartedAt: start, EndedAt: &end}
}

func singleLaborHint(t *testing.T, sessions []*WorkSession, breaks map[int64][]*WorkSessionBreak, now time.Time) WorkDayLaborHint {
	t.Helper()
	hints := WorkDayLaborHints(sessions, breaks, now, laborDay, laborDay)
	require.Len(t, hints, 1)
	return hints[0]
}

func TestWorkDayLaborHints(t *testing.T) {
	t.Parallel()
	afterDay := laborAt(laborDay, 23, 0)

	t.Run("day with sufficient break has no hint", func(t *testing.T) {
		t.Parallel()
		block := closedBlock(1, laborAt(laborDay, 8, 0), laborAt(laborDay, 15, 0))
		block.BreakMinutes = 30
		breaks := map[int64][]*WorkSessionBreak{1: {endedBreak(laborAt(laborDay, 12, 0), 30)}}

		hint := singleLaborHint(t, []*WorkSession{block}, breaks, afterDay)

		assert.Equal(t, 390, hint.NetMinutes)
		assert.Equal(t, 30, hint.CountedBreakMinutes)
		assert.False(t, hint.IsBreakTooShort)
		assert.False(t, hint.IsOverDailyMax)
		assert.False(t, hint.HasHint())
	})

	t.Run("6 h 10 min without break is marked", func(t *testing.T) {
		t.Parallel()
		block := closedBlock(1, laborAt(laborDay, 8, 0), laborAt(laborDay, 14, 10))

		hint := singleLaborHint(t, []*WorkSession{block}, nil, afterDay)

		assert.Equal(t, 370, hint.NetMinutes)
		assert.Equal(t, 30, hint.RequiredBreakMinutes)
		assert.Equal(t, 0, hint.CountedBreakMinutes)
		assert.True(t, hint.IsBreakTooShort)
		assert.True(t, hint.HasHint())
	})

	t.Run("a break below 15 minutes does not count", func(t *testing.T) {
		t.Parallel()
		block := closedBlock(1, laborAt(laborDay, 8, 0), laborAt(laborDay, 14, 30))
		block.BreakMinutes = 20
		breaks := map[int64][]*WorkSessionBreak{1: {
			endedBreak(laborAt(laborDay, 10, 0), 10),
			endedBreak(laborAt(laborDay, 12, 0), 10),
		}}

		hint := singleLaborHint(t, []*WorkSession{block}, breaks, afterDay)

		assert.Equal(t, 20, hint.BreakMinutes)
		assert.Equal(t, 0, hint.CountedBreakMinutes)
		assert.True(t, hint.IsBreakTooShort)
	})

	t.Run("a running break is not marked as too short", func(t *testing.T) {
		t.Parallel()
		now := laborAt(laborDay, 14, 20)
		block := &WorkSession{ID: 1, CheckInTime: laborAt(laborDay, 8, 0)}
		breaks := map[int64][]*WorkSessionBreak{1: {runningBreak(now, 10)}}

		hint := singleLaborHint(t, []*WorkSession{block}, breaks, now)

		assert.Equal(t, 370, hint.NetMinutes)
		assert.True(t, hint.IsRunning)
		assert.False(t, hint.IsBreakTooShort)
		assert.Nil(t, hint.OverDailyMaxAt, "no live instants while a break runs")
		assert.Nil(t, hint.BreakTooShortAt)
	})

	t.Run("more than 10 hours is marked", func(t *testing.T) {
		t.Parallel()
		block := closedBlock(1, laborAt(laborDay, 7, 0), laborAt(laborDay, 18, 0))
		block.BreakMinutes = 45
		breaks := map[int64][]*WorkSessionBreak{1: {endedBreak(laborAt(laborDay, 12, 0), 45)}}

		hint := singleLaborHint(t, []*WorkSession{block}, breaks, afterDay)

		assert.Equal(t, 615, hint.NetMinutes)
		assert.True(t, hint.IsOverDailyMax)
		assert.False(t, hint.IsBreakTooShort)
	})

	t.Run("blocks of one day are evaluated together", func(t *testing.T) {
		t.Parallel()
		morning := closedBlock(1, laborAt(laborDay, 8, 0), laborAt(laborDay, 12, 0))
		afternoon := closedBlock(2, laborAt(laborDay, 12, 10), laborAt(laborDay, 15, 0))

		hint := singleLaborHint(t, []*WorkSession{afternoon, morning}, nil, afterDay)

		assert.Equal(t, 410, hint.NetMinutes)
		assert.Equal(t, 0, hint.CountedBreakMinutes, "a 10-minute gap is no break")
		assert.True(t, hint.IsBreakTooShort)
	})

	t.Run("a running day names when the hints would start", func(t *testing.T) {
		t.Parallel()
		now := laborAt(laborDay, 12, 0)
		block := &WorkSession{ID: 1, CheckInTime: laborAt(laborDay, 8, 0)}

		hint := singleLaborHint(t, []*WorkSession{block}, nil, now)

		require.NotNil(t, hint.OverDailyMaxAt)
		assert.Equal(t, now.Add(361*time.Minute), *hint.OverDailyMaxAt)
		require.NotNil(t, hint.BreakTooShortAt)
		assert.Equal(t, now.Add(121*time.Minute), *hint.BreakTooShortAt)
		assert.Equal(t, 30, hint.BreakTooShortRequiredMinutes)
	})

	t.Run("days outside the range are left out", func(t *testing.T) {
		t.Parallel()
		next := laborDay.AddDays(1)
		sessions := []*WorkSession{
			closedBlock(1, laborAt(laborDay, 8, 0), laborAt(laborDay, 15, 0)),
			closedBlock(2, laborAt(next, 8, 0), laborAt(next, 15, 0)),
		}

		hints := WorkDayLaborHints(sessions, nil, laborAt(next, 23, 0), next, next)

		require.Len(t, hints, 1)
		assert.Equal(t, next, hints[0].Date)
		assert.Equal(t, 1, CountLaborHintDays(sessions, nil, laborAt(next, 23, 0), laborDay, laborDay))
	})
}

func TestWeeklySummaryMarksWeekOver48Hours(t *testing.T) {
	t.Parallel()
	monday := laborDay
	sessions := make([]*SessionResponse, 0, 5)
	// Five days of 9 h 48 min net: 49 hours in the week.
	for offset := range 5 {
		day := monday.AddDays(offset)
		block := closedBlock(int64(offset+1), laborAt(day, 7, 0), laborAt(day, 17, 33))
		block.BreakMinutes = 45
		sessions = append(sessions, &SessionResponse{WorkSession: block})
	}
	svc := &workSessionService{}

	summaries := svc.buildWeeklySummaries(sessions, nil, monday, monday.AddDays(6))

	require.Len(t, summaries, 1)
	assert.Equal(t, 49*60, summaries[0].TotalNetMinutes)
	assert.True(t, summaries[0].IsOverWeeklyMax)

	notOver := svc.buildWeeklySummaries(sessions[:4], nil, monday, monday.AddDays(6))
	require.Len(t, notOver, 1)
	assert.False(t, notOver[0].IsOverWeeklyMax)
}
