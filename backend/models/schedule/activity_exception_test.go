package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validActivityException() *ActivityException {
	return &ActivityException{
		ActivityGroupID: 17,
		ExceptionDate:   timezone.NewDate(2026, 9, 15),
		ExceptionType:   ActivityExceptionCancelled,
	}
}

func TestActivityException_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid cancelled exception", func(t *testing.T) {
		require.NoError(t, validActivityException().Validate())
	})

	t.Run("valid modified exception with time override", func(t *testing.T) {
		e := validActivityException()
		e.ExceptionType = ActivityExceptionModified
		start := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
		e.StartTime = &start
		e.EndTime = &end
		require.NoError(t, e.Validate())
	})

	t.Run("valid modified exception with room-only override", func(t *testing.T) {
		e := validActivityException()
		e.ExceptionType = ActivityExceptionModified
		roomID := int64(42)
		e.RoomID = &roomID
		require.NoError(t, e.Validate())
	})

	t.Run("missing activity_group_id", func(t *testing.T) {
		e := validActivityException()
		e.ActivityGroupID = 0
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "activity_group_id is required")
	})

	t.Run("missing exception_date", func(t *testing.T) {
		e := validActivityException()
		e.ExceptionDate = timezone.Date("")
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exception_date is required")
	})

	t.Run("invalid exception_type", func(t *testing.T) {
		e := validActivityException()
		e.ExceptionType = "unknown"
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid exception type")
	})

	t.Run("cancelled exception cannot carry overrides", func(t *testing.T) {
		e := validActivityException()
		roomID := int64(42)
		e.RoomID = &roomID
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled exception must not set")
	})

	t.Run("modified exception with no overrides is rejected", func(t *testing.T) {
		e := validActivityException()
		e.ExceptionType = ActivityExceptionModified
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must override at least one")
	})

	t.Run("modified exception with end before start is rejected", func(t *testing.T) {
		e := validActivityException()
		e.ExceptionType = ActivityExceptionModified
		start := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
		end := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
		e.StartTime = &start
		e.EndTime = &end
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "end_time must be after start_time")
	})

	t.Run("reason over 500 chars is rejected", func(t *testing.T) {
		e := validActivityException()
		r := strings.Repeat("a", ActivityExceptionReasonMaxLength+1)
		e.Reason = &r
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reason cannot exceed 500 characters")
	})
}

func TestIsValidExceptionType(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidExceptionType(ActivityExceptionCancelled))
	assert.True(t, IsValidExceptionType(ActivityExceptionModified))
	assert.False(t, IsValidExceptionType(""))
	assert.False(t, IsValidExceptionType("unknown"))
}

func TestActivityException_IsCancellation(t *testing.T) {
	t.Parallel()

	assert.True(t, validActivityException().IsCancellation())

	modified := validActivityException()
	modified.ExceptionType = ActivityExceptionModified
	assert.False(t, modified.IsCancellation())
}
