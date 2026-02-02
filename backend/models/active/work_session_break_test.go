package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkSessionBreak_Validate(t *testing.T) {
	now := time.Now()

	validBreak := func() *WorkSessionBreak {
		return &WorkSessionBreak{
			SessionID:       1,
			StartedAt:       now,
			DurationMinutes: 0,
		}
	}

	t.Run("valid active break", func(t *testing.T) {
		assert.NoError(t, validBreak().Validate())
	})

	t.Run("valid ended break", func(t *testing.T) {
		b := validBreak()
		later := now.Add(30 * time.Minute)
		b.EndedAt = &later
		b.DurationMinutes = 30
		assert.NoError(t, b.Validate())
	})

	t.Run("missing session ID", func(t *testing.T) {
		b := validBreak()
		b.SessionID = 0
		err := b.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session ID is required")
	})

	t.Run("missing started_at", func(t *testing.T) {
		b := validBreak()
		b.StartedAt = time.Time{}
		err := b.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "started_at is required")
	})

	t.Run("started_at after ended_at", func(t *testing.T) {
		b := validBreak()
		earlier := now.Add(-1 * time.Hour)
		b.EndedAt = &earlier
		err := b.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "started_at must be before ended_at")
	})

	t.Run("negative duration", func(t *testing.T) {
		b := validBreak()
		b.DurationMinutes = -5
		err := b.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duration_minutes cannot be negative")
	})
}

func TestWorkSessionBreak_IsActive(t *testing.T) {
	t.Run("active when no end time", func(t *testing.T) {
		b := &WorkSessionBreak{EndedAt: nil}
		assert.True(t, b.IsActive())
	})

	t.Run("inactive when ended", func(t *testing.T) {
		now := time.Now()
		b := &WorkSessionBreak{EndedAt: &now}
		assert.False(t, b.IsActive())
	})
}

func TestWorkSessionBreak_TableName(t *testing.T) {
	b := &WorkSessionBreak{}
	assert.Equal(t, "active.work_session_breaks", b.TableName())
}

func TestWorkSessionBreak_Getters(t *testing.T) {
	now := time.Now()
	b := &WorkSessionBreak{}
	b.ID = 7
	b.CreatedAt = now
	b.UpdatedAt = now

	assert.Equal(t, int64(7), b.GetID())
	assert.Equal(t, now, b.GetCreatedAt())
	assert.Equal(t, now, b.GetUpdatedAt())
}
