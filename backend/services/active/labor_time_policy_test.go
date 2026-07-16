package active

import (
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
)

func TestNetMinutes(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("with checkout 8 hours no breaks", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.Equal(t, 480, netMinutes(ws, fixedNow))
	})

	t.Run("with checkout 8 hours 30min break", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		assert.Equal(t, 450, netMinutes(ws, fixedNow))
	})

	t.Run("net cannot be negative", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 8, 10, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 60,
		}
		assert.Equal(t, 0, netMinutes(ws, fixedNow))
	})

	t.Run("active session measures against now", func(t *testing.T) {
		checkIn := fixedNow.Add(-2 * time.Hour)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: nil,
			BreakMinutes: 0,
		}
		assert.Equal(t, 120, netMinutes(ws, fixedNow))
	})
}

// runningBreak is a break started `minutesAgo` before `now` and not yet ended.
func runningBreak(now time.Time, minutesAgo int) *activeModels.WorkSessionBreak {
	return &activeModels.WorkSessionBreak{
		StartedAt: now.Add(-time.Duration(minutesAgo) * time.Minute),
		EndedAt:   nil,
	}
}

func TestNetMinutesWithBreaks(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("deducts a running break from an open session", func(t *testing.T) {
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-2 * time.Hour),
			CheckOutTime: nil,
			BreakMinutes: 0, // cache holds ENDED breaks only
		}
		breaks := []*activeModels.WorkSessionBreak{runningBreak(fixedNow, 30)}
		assert.Equal(t, 90, netMinutesWithBreaks(ws, breaks, fixedNow))
	})

	t.Run("adds the running break on top of the ended-break cache", func(t *testing.T) {
		ended := fixedNow.Add(-time.Hour)
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-4 * time.Hour),
			CheckOutTime: nil,
			BreakMinutes: 30,
		}
		breaks := []*activeModels.WorkSessionBreak{
			{StartedAt: fixedNow.Add(-90 * time.Minute), EndedAt: &ended},
			runningBreak(fixedNow, 15),
		}
		// 240 gross - 30 cached - 15 running
		assert.Equal(t, 195, netMinutesWithBreaks(ws, breaks, fixedNow))
	})

	t.Run("checked-out session is untouched", func(t *testing.T) {
		checkOut := fixedNow.Add(-time.Hour)
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-9 * time.Hour),
			CheckOutTime: &checkOut,
			BreakMinutes: 60,
		}
		assert.Equal(t, 420, netMinutesWithBreaks(ws, nil, fixedNow))
	})

	t.Run("floors at zero", func(t *testing.T) {
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-10 * time.Minute),
			CheckOutTime: nil,
			BreakMinutes: 0,
		}
		breaks := []*activeModels.WorkSessionBreak{runningBreak(fixedNow, 30)}
		assert.Equal(t, 0, netMinutesWithBreaks(ws, breaks, fixedNow))
	})
}

func TestIsOvertime(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC)

	t.Run("not overtime under 10 hours", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.False(t, isOvertime(ws, nil, fixedNow))
	})

	t.Run("overtime over 10 hours", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.True(t, isOvertime(ws, nil, fixedNow)) // 660 min > 600
	})

	t.Run("exactly 10 hours not overtime", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.False(t, isOvertime(ws, nil, fixedNow)) // 600 min == 600, not >600
	})

	// The flag must judge the same net time that is reported to the reader,
	// otherwise a row shows "9:50" and an overtime marker at once (#1842).
	t.Run("running break keeps net under the threshold", func(t *testing.T) {
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-620 * time.Minute),
			CheckOutTime: nil,
			BreakMinutes: 0,
		}
		breaks := []*activeModels.WorkSessionBreak{runningBreak(fixedNow, 30)}
		// 620 gross would be overtime; 590 net is not.
		assert.Equal(t, 590, netMinutesWithBreaks(ws, breaks, fixedNow))
		assert.False(t, isOvertime(ws, breaks, fixedNow))
	})
}

func TestIsBreakCompliant(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC)

	makeSession := func(hours, breakMin int) *activeModels.WorkSession {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := checkIn.Add(time.Duration(hours)*time.Hour + time.Duration(breakMin)*time.Minute)
		return &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: breakMin,
		}
	}

	t.Run("under 6h no break needed", func(t *testing.T) {
		ws := makeSession(5, 0)
		assert.True(t, isBreakCompliant(ws, nil, fixedNow))
	})

	t.Run("exactly 6h no break needed", func(t *testing.T) {
		ws := makeSession(6, 0)
		assert.True(t, isBreakCompliant(ws, nil, fixedNow))
	})

	t.Run("over 6h needs 30min break - not compliant", func(t *testing.T) {
		// 7h gross with 15min break = 405min net > 360
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 15,
		}
		assert.False(t, isBreakCompliant(ws, nil, fixedNow))
	})

	t.Run("over 6h with 30min break - compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		assert.True(t, isBreakCompliant(ws, nil, fixedNow))
	})

	t.Run("over 9h needs 45min break - not compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		// 600min gross - 30min break = 570min net > 540
		assert.False(t, isBreakCompliant(ws, nil, fixedNow))
	})

	t.Run("over 9h with 45min break - compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 45,
		}
		// 600min gross - 45min break = 555min net > 540 → needs 45min break
		assert.True(t, isBreakCompliant(ws, nil, fixedNow))
	})

	// While a break is running it is absent from the BreakMinutes cache, so
	// judging the requirement against the cache alone would flag someone as
	// non-compliant *during* the very break that makes them compliant, right
	// next to a net time that already deducts it (#1842).
	t.Run("running break counts toward the requirement", func(t *testing.T) {
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-7 * time.Hour),
			CheckOutTime: nil,
			BreakMinutes: 0,
		}
		breaks := []*activeModels.WorkSessionBreak{runningBreak(fixedNow, 35)}
		// 420 gross - 35 running = 385 net > 360 → needs 30 min, 35 taken.
		assert.Equal(t, 385, netMinutesWithBreaks(ws, breaks, fixedNow))
		assert.True(t, isBreakCompliant(ws, breaks, fixedNow))
	})

	t.Run("a too-short running break stays non-compliant", func(t *testing.T) {
		ws := &activeModels.WorkSession{
			CheckInTime:  fixedNow.Add(-7 * time.Hour),
			CheckOutTime: nil,
			BreakMinutes: 0,
		}
		breaks := []*activeModels.WorkSessionBreak{runningBreak(fixedNow, 10)}
		// 410 net > 360 → needs 30 min, only 10 taken so far.
		assert.False(t, isBreakCompliant(ws, breaks, fixedNow))
	})
}
