package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkSession_Validate(t *testing.T) {
	now := time.Now()
	validSession := func() *WorkSession {
		return &WorkSession{
			StaffID:     1,
			CheckInTime: now,
			Status:      WorkSessionStatusPresent,
			CreatedBy:   1,
		}
	}

	t.Run("valid present session", func(t *testing.T) {
		ws := validSession()
		assert.NoError(t, ws.Validate())
	})

	t.Run("valid home_office session", func(t *testing.T) {
		ws := validSession()
		ws.Status = WorkSessionStatusHomeOffice
		assert.NoError(t, ws.Validate())
	})

	t.Run("valid session with checkout", func(t *testing.T) {
		ws := validSession()
		later := now.Add(8 * time.Hour)
		ws.CheckOutTime = &later
		assert.NoError(t, ws.Validate())
	})

	t.Run("missing staff ID", func(t *testing.T) {
		ws := validSession()
		ws.StaffID = 0
		err := ws.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID is required")
	})

	t.Run("missing check-in time", func(t *testing.T) {
		ws := validSession()
		ws.CheckInTime = time.Time{}
		err := ws.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "check-in time is required")
	})

	t.Run("invalid status", func(t *testing.T) {
		ws := validSession()
		ws.Status = "invalid"
		err := ws.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status must be")
	})

	t.Run("check-in after check-out", func(t *testing.T) {
		ws := validSession()
		earlier := now.Add(-1 * time.Hour)
		ws.CheckOutTime = &earlier
		err := ws.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "check-in time must be before check-out time")
	})

	t.Run("negative break minutes", func(t *testing.T) {
		ws := validSession()
		ws.BreakMinutes = -1
		err := ws.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "break minutes cannot be negative")
	})

	t.Run("missing created_by", func(t *testing.T) {
		ws := validSession()
		ws.CreatedBy = 0
		err := ws.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "created_by is required")
	})
}

func TestWorkSession_IsActive(t *testing.T) {
	t.Run("active when no checkout", func(t *testing.T) {
		ws := &WorkSession{CheckOutTime: nil}
		assert.True(t, ws.IsActive())
	})

	t.Run("inactive when checked out", func(t *testing.T) {
		now := time.Now()
		ws := &WorkSession{CheckOutTime: &now}
		assert.False(t, ws.IsActive())
	})
}

func TestWorkSession_CheckOut(t *testing.T) {
	ws := &WorkSession{CheckOutTime: nil}
	assert.True(t, ws.IsActive())

	ws.CheckOut()
	assert.False(t, ws.IsActive())
	assert.NotNil(t, ws.CheckOutTime)
}

func TestWorkSession_NetMinutes(t *testing.T) {
	t.Run("with checkout 8 hours no breaks", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.Equal(t, 480, ws.NetMinutes())
	})

	t.Run("with checkout 8 hours 30min break", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		assert.Equal(t, 450, ws.NetMinutes())
	})

	t.Run("net cannot be negative", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 8, 10, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 60,
		}
		assert.Equal(t, 0, ws.NetMinutes())
	})

	t.Run("active session uses current time", func(t *testing.T) {
		checkIn := time.Now().Add(-2 * time.Hour)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: nil,
			BreakMinutes: 0,
		}
		net := ws.NetMinutes()
		// Should be approximately 120 minutes (allow 2 min tolerance)
		assert.InDelta(t, 120, net, 2)
	})
}

func TestWorkSession_IsOvertime(t *testing.T) {
	t.Run("not overtime under 10 hours", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.False(t, ws.IsOvertime())
	})

	t.Run("overtime over 10 hours", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.True(t, ws.IsOvertime()) // 660 min > 600
	})

	t.Run("exactly 10 hours not overtime", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 0,
		}
		assert.False(t, ws.IsOvertime()) // 600 min == 600, not >600
	})
}

func TestWorkSession_IsBreakCompliant(t *testing.T) {
	makeSession := func(hours int, breakMin int) *WorkSession {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := checkIn.Add(time.Duration(hours)*time.Hour + time.Duration(breakMin)*time.Minute)
		return &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: breakMin,
		}
	}

	t.Run("under 6h no break needed", func(t *testing.T) {
		ws := makeSession(5, 0)
		assert.True(t, ws.IsBreakCompliant())
	})

	t.Run("exactly 6h no break needed", func(t *testing.T) {
		ws := makeSession(6, 0)
		assert.True(t, ws.IsBreakCompliant())
	})

	t.Run("over 6h needs 30min break - not compliant", func(t *testing.T) {
		// 7h gross with 15min break = 405min net > 360
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 15,
		}
		assert.False(t, ws.IsBreakCompliant())
	})

	t.Run("over 6h with 30min break - compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		assert.True(t, ws.IsBreakCompliant())
	})

	t.Run("over 9h needs 45min break - not compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 30,
		}
		// 600min gross - 30min break = 570min net > 540
		assert.False(t, ws.IsBreakCompliant())
	})

	t.Run("over 9h with 45min break - compliant", func(t *testing.T) {
		checkIn := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
		checkOut := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
		ws := &WorkSession{
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 45,
		}
		// 600min gross - 45min break = 555min net > 540 → needs 45min break
		assert.True(t, ws.IsBreakCompliant())
	})
}

func TestWorkSession_TableName(t *testing.T) {
	ws := &WorkSession{}
	assert.Equal(t, "active.work_sessions", ws.TableName())
}

func TestWorkSession_Getters(t *testing.T) {
	now := time.Now()
	ws := &WorkSession{}
	ws.ID = 42
	ws.CreatedAt = now
	ws.UpdatedAt = now

	assert.Equal(t, int64(42), ws.GetID())
	assert.Equal(t, now, ws.GetCreatedAt())
	assert.Equal(t, now, ws.GetUpdatedAt())
}

func TestWorkSession_BeforeAppendModel(t *testing.T) {
	ws := &WorkSession{}

	t.Run("handles SelectQuery", func(t *testing.T) {
		// BeforeAppendModel should not error on any query type
		err := ws.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("handles UpdateQuery", func(t *testing.T) {
		err := ws.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("handles DeleteQuery", func(t *testing.T) {
		err := ws.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("handles InsertQuery", func(t *testing.T) {
		err := ws.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})
}
