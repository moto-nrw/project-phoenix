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
