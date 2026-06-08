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
		assert.False(t, isOvertime(ws, fixedNow))
	})

	t.Run("overtime over 10 hours", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.True(t, isOvertime(ws, fixedNow)) // 660 min > 600
	})

	t.Run("exactly 10 hours not overtime", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.False(t, isOvertime(ws, fixedNow)) // 600 min == 600, not >600
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
		assert.True(t, isBreakCompliant(ws, fixedNow))
	})

	t.Run("exactly 6h no break needed", func(t *testing.T) {
		ws := makeSession(6, 0)
		assert.True(t, isBreakCompliant(ws, fixedNow))
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
		assert.False(t, isBreakCompliant(ws, fixedNow))
	})

	t.Run("over 6h with 30min break - compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
		ws := &activeModels.WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		assert.True(t, isBreakCompliant(ws, fixedNow))
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
		assert.False(t, isBreakCompliant(ws, fixedNow))
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
		assert.True(t, isBreakCompliant(ws, fixedNow))
	})
}
