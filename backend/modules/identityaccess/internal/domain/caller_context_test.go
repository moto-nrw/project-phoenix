package domain

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The sentinel texts are part of the /api/me wire format.
func TestCallerErrorVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrCallerNotFound", ErrCallerNotFound, "user not found"},
		{"ErrCallerNotAuthenticated", ErrCallerNotAuthenticated, "user not authenticated"},
		{"ErrCallerNotAuthorized", ErrCallerNotAuthorized, "user not authorized"},
		{"ErrCallerNotLinkedToPerson", ErrCallerNotLinkedToPerson, "user account not linked to a person"},
		{"ErrCallerNotLinkedToStaff", ErrCallerNotLinkedToStaff, "user not linked to a staff member"},
		{"ErrCallerNotLinkedToTeacher", ErrCallerNotLinkedToTeacher, "user not linked to a teacher"},
		{"ErrCallerGroupNotFound", ErrCallerGroupNotFound, "group not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestCallerErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	errorVars := []error{
		ErrCallerNotFound,
		ErrCallerNotAuthenticated,
		ErrCallerNotAuthorized,
		ErrCallerNotLinkedToPerson,
		ErrCallerNotLinkedToStaff,
		ErrCallerNotLinkedToTeacher,
		ErrCallerGroupNotFound,
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

func TestCallerError(t *testing.T) {
	t.Parallel()

	t.Run("Error message format", func(t *testing.T) {
		callerErr := &CallerError{Op: "GetUserContext", Err: errors.New("database connection lost")}

		assert.Equal(t, "usercontext.GetUserContext: database connection lost", callerErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		callerErr := &CallerError{Op: "GetUserContext", Err: ErrCallerNotFound}

		assert.Equal(t, ErrCallerNotFound, callerErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped CallerError", func(t *testing.T) {
		callerErr := &CallerError{Op: "GetUserContext", Err: ErrCallerNotAuthenticated}

		assert.True(t, errors.Is(callerErr, ErrCallerNotAuthenticated))
	})

	t.Run("errors.As works with CallerError", func(t *testing.T) {
		callerErr := &CallerError{Op: "GetUserContext", Err: ErrCallerNotFound}

		var targetErr *CallerError
		assert.True(t, errors.As(callerErr, &targetErr))
		assert.Equal(t, "GetUserContext", targetErr.Op)
	})
}

func TestCallerGroupsPartialError(t *testing.T) {
	t.Parallel()

	t.Run("Error message includes counts", func(t *testing.T) {
		partialErr := &CallerGroupsPartialError{
			Op: "BulkUpdate", SuccessCount: 5, FailureCount: 2,
			FailedIDs: []int64{10, 20}, LastErr: errors.New("final operation failed"),
		}

		expected := "usercontext.BulkUpdate: partial failure - 5 succeeded, 2 failed (last error: final operation failed)"
		assert.Equal(t, expected, partialErr.Error())
	})

	t.Run("Unwrap returns last error", func(t *testing.T) {
		lastErr := errors.New("database error")
		partialErr := &CallerGroupsPartialError{
			Op: "BulkDelete", SuccessCount: 3, FailureCount: 1, FailedIDs: []int64{5}, LastErr: lastErr,
		}

		assert.Equal(t, lastErr, partialErr.Unwrap())
	})

	t.Run("FailedIDs contains correct IDs", func(t *testing.T) {
		partialErr := &CallerGroupsPartialError{
			Op: "BulkUpdate", SuccessCount: 8, FailureCount: 3,
			FailedIDs: []int64{11, 55, 99}, LastErr: errors.New("validation failed"),
		}

		assert.Len(t, partialErr.FailedIDs, 3)
		assert.Contains(t, partialErr.FailedIDs, int64(11))
		assert.Contains(t, partialErr.FailedIDs, int64(55))
		assert.Contains(t, partialErr.FailedIDs, int64(99))
	})

	t.Run("zero successes", func(t *testing.T) {
		partialErr := &CallerGroupsPartialError{
			Op: "BulkOperation", SuccessCount: 0, FailureCount: 10,
			FailedIDs: []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			LastErr:   errors.New("all operations failed"),
		}

		assert.Equal(t, 0, partialErr.SuccessCount)
		assert.Equal(t, 10, partialErr.FailureCount)
		assert.Len(t, partialErr.FailedIDs, 10)
	})

	t.Run("zero failures", func(t *testing.T) {
		partialErr := &CallerGroupsPartialError{
			Op: "BulkOperation", SuccessCount: 10, FailureCount: 0, FailedIDs: []int64{}, LastErr: nil,
		}

		assert.Equal(t, 10, partialErr.SuccessCount)
		assert.Equal(t, 0, partialErr.FailureCount)
		assert.Empty(t, partialErr.FailedIDs)
	})
}

func TestSSESetupError_ErrorMessage(t *testing.T) {
	t.Parallel()

	err := &SSESetupError{Message: "Account not found", Status: http.StatusUnauthorized}

	assert.Equal(t, "SSE setup: Account not found", err.Error())
	assert.Equal(t, http.StatusUnauthorized, err.Status)
}

func TestSSESetupError_AsMatchesTypedError(t *testing.T) {
	t.Parallel()

	var err error = &SSESetupError{Message: "forbidden", Status: http.StatusForbidden}

	var setupErr *SSESetupError
	require.True(t, errors.As(err, &setupErr), "Should match *SSESetupError via errors.As")
	assert.Equal(t, "forbidden", setupErr.Message)
	assert.Equal(t, http.StatusForbidden, setupErr.Status)
}

func TestSSESetupError_AsDistinguishesErrors(t *testing.T) {
	t.Parallel()

	var setupErr *SSESetupError
	assert.False(t, errors.As(assert.AnError, &setupErr), "Regular error should not match *SSESetupError")
}
