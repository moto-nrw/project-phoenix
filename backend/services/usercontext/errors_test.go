package usercontext

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUserContextErrorVariables tests that error variables have correct messages
func TestUserContextErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrUserNotFound", ErrUserNotFound, "user not found"},
		{"ErrUserNotAuthenticated", ErrUserNotAuthenticated, "user not authenticated"},
		{"ErrUserNotAuthorized", ErrUserNotAuthorized, "user not authorized"},
		{"ErrUserNotLinkedToPerson", ErrUserNotLinkedToPerson, "user account not linked to a person"},
		{"ErrUserNotLinkedToStaff", ErrUserNotLinkedToStaff, "user not linked to a staff member"},
		{"ErrUserNotLinkedToTeacher", ErrUserNotLinkedToTeacher, "user not linked to a teacher"},
		{"ErrNoActiveGroups", ErrNoActiveGroups, "user has no active groups"},
		{"ErrGroupNotFound", ErrGroupNotFound, "group not found"},
		{"ErrInvalidOperation", ErrInvalidOperation, "invalid operation for current user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestUserContextErrorsAreDistinct ensures each error can be identified uniquely
func TestUserContextErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrUserNotFound,
		ErrUserNotAuthenticated,
		ErrUserNotAuthorized,
		ErrUserNotLinkedToPerson,
		ErrUserNotLinkedToStaff,
		ErrUserNotLinkedToTeacher,
		ErrNoActiveGroups,
		ErrGroupNotFound,
		ErrInvalidOperation,
	}

	for i, err1 := range errorVars {
		for j, err2 := range errorVars {
			if i == j {
				assert.True(t, errors.Is(err1, err2), "error should equal itself")
			} else {
				assert.False(t, errors.Is(err1, err2), "different errors should not be equal")
			}
		}
	}
}

// TestUserContextError tests the UserContextError custom error type
func TestUserContextError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		underlyingErr := errors.New("database connection lost")
		userContextErr := &UserContextError{
			Op:  "GetUserContext",
			Err: underlyingErr,
		}

		expected := "usercontext.GetUserContext: database connection lost"
		assert.Equal(t, expected, userContextErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrUserNotFound
		userContextErr := &UserContextError{
			Op:  "GetUserContext",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, userContextErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped UserContextError", func(t *testing.T) {
		userContextErr := &UserContextError{
			Op:  "GetUserContext",
			Err: ErrUserNotAuthenticated,
		}

		assert.True(t, errors.Is(userContextErr, ErrUserNotAuthenticated))
	})

	t.Run("errors.As works with UserContextError", func(t *testing.T) {
		userContextErr := &UserContextError{
			Op:  "GetUserContext",
			Err: ErrUserNotFound,
		}

		var targetErr *UserContextError
		assert.True(t, errors.As(userContextErr, &targetErr))
		assert.Equal(t, "GetUserContext", targetErr.Op)
	})
}

// TestPartialError tests the PartialError custom error type
func TestPartialError(t *testing.T) {
	t.Run("Error message includes counts", func(t *testing.T) {
		lastErr := errors.New("final operation failed")
		partialErr := &PartialError{
			Op:           "BulkUpdate",
			SuccessCount: 5,
			FailureCount: 2,
			FailedIDs:    []int64{10, 20},
			LastErr:      lastErr,
		}

		expected := "usercontext.BulkUpdate: partial failure - 5 succeeded, 2 failed (last error: final operation failed)"
		assert.Equal(t, expected, partialErr.Error())
	})

	t.Run("Unwrap returns last error", func(t *testing.T) {
		lastErr := errors.New("database error")
		partialErr := &PartialError{
			Op:           "BulkDelete",
			SuccessCount: 3,
			FailureCount: 1,
			FailedIDs:    []int64{5},
			LastErr:      lastErr,
		}

		assert.Equal(t, lastErr, partialErr.Unwrap())
	})

	t.Run("FailedIDs contains correct IDs", func(t *testing.T) {
		partialErr := &PartialError{
			Op:           "BulkUpdate",
			SuccessCount: 8,
			FailureCount: 3,
			FailedIDs:    []int64{11, 55, 99},
			LastErr:      errors.New("validation failed"),
		}

		assert.Len(t, partialErr.FailedIDs, 3)
		assert.Contains(t, partialErr.FailedIDs, int64(11))
		assert.Contains(t, partialErr.FailedIDs, int64(55))
		assert.Contains(t, partialErr.FailedIDs, int64(99))
	})

	t.Run("PartialError with zero successes", func(t *testing.T) {
		partialErr := &PartialError{
			Op:           "BulkOperation",
			SuccessCount: 0,
			FailureCount: 10,
			FailedIDs:    []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			LastErr:      errors.New("all operations failed"),
		}

		assert.Equal(t, 0, partialErr.SuccessCount)
		assert.Equal(t, 10, partialErr.FailureCount)
		assert.Len(t, partialErr.FailedIDs, 10)
	})

	t.Run("PartialError with zero failures", func(t *testing.T) {
		// Edge case - should probably not be used but test anyway
		partialErr := &PartialError{
			Op:           "BulkOperation",
			SuccessCount: 10,
			FailureCount: 0,
			FailedIDs:    []int64{},
			LastErr:      nil,
		}

		assert.Equal(t, 10, partialErr.SuccessCount)
		assert.Equal(t, 0, partialErr.FailureCount)
		assert.Empty(t, partialErr.FailedIDs)
	})
}
