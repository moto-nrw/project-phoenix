package activities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestActivitiesErrorVariables tests that error variables have correct messages
func TestActivitiesErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrCategoryNotFound", ErrCategoryNotFound, "activity category not found"},
		{"ErrGroupNotFound", ErrGroupNotFound, "activity group not found"},
		{"ErrScheduleNotFound", ErrScheduleNotFound, "schedule not found"},
		{"ErrSupervisorNotFound", ErrSupervisorNotFound, "supervisor not found"},
		{"ErrEnrollmentNotFound", ErrEnrollmentNotFound, "enrollment not found"},
		{"ErrGroupFull", ErrGroupFull, "activity group is at maximum capacity"},
		{"ErrAlreadyEnrolled", ErrAlreadyEnrolled, "student is already enrolled in this activity group"},
		{"ErrStudentAlreadyEnrolled", ErrStudentAlreadyEnrolled, "student is already enrolled in this activity group"},
		{"ErrNotEnrolled", ErrNotEnrolled, "student is not enrolled in this activity group"},
		{"ErrInvalidAttendanceStatus", ErrInvalidAttendanceStatus, "invalid attendance status"},
		{"ErrGroupClosed", ErrGroupClosed, "activity group is not open for enrollment"},
		{"ErrCannotDeletePrimary", ErrCannotDeletePrimary, "cannot delete primary supervisor"},
		{"ErrStaffNotFound", ErrStaffNotFound, "staff not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestActivitiesErrorsAreDistinct ensures errors are uniquely identifiable
func TestActivitiesErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrCategoryNotFound,
		ErrGroupNotFound,
		ErrScheduleNotFound,
		ErrSupervisorNotFound,
		ErrEnrollmentNotFound,
		ErrGroupFull,
		ErrAlreadyEnrolled,
		ErrNotEnrolled,
		ErrInvalidAttendanceStatus,
		ErrGroupClosed,
		ErrCannotDeletePrimary,
		ErrStaffNotFound,
	}

	for i, err1 := range errorVars {
		for j, err2 := range errorVars {
			if i == j {
				assert.True(t, errors.Is(err1, err2), "error should equal itself")
			} else {
				// ErrStudentAlreadyEnrolled is an alias for ErrAlreadyEnrolled
				if (err1 == ErrStudentAlreadyEnrolled && err2 == ErrAlreadyEnrolled) ||
					(err1 == ErrAlreadyEnrolled && err2 == ErrStudentAlreadyEnrolled) {
					assert.True(t, errors.Is(err1, err2), "alias errors should be equal")
				} else {
					assert.False(t, errors.Is(err1, err2), "different errors should not be equal")
				}
			}
		}
	}
}

// TestAlreadyEnrolledAlias verifies the alias works correctly
func TestAlreadyEnrolledAlias(t *testing.T) {
	assert.True(t, errors.Is(ErrStudentAlreadyEnrolled, ErrAlreadyEnrolled))
	assert.True(t, errors.Is(ErrAlreadyEnrolled, ErrStudentAlreadyEnrolled))
}

// TestActivityError tests the ActivityError custom error type
func TestActivityError(t *testing.T) {
	t.Run("Error with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("database constraint violation")
		activityErr := &ActivityError{
			Op:  "EnrollStudent",
			Err: underlyingErr,
		}

		expected := "activity error during EnrollStudent: database constraint violation"
		assert.Equal(t, expected, activityErr.Error())
	})

	t.Run("Error without underlying error", func(t *testing.T) {
		activityErr := &ActivityError{
			Op:  "CreateGroup",
			Err: nil,
		}

		expected := "activity error during CreateGroup"
		assert.Equal(t, expected, activityErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrGroupFull
		activityErr := &ActivityError{
			Op:  "EnrollStudent",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, activityErr.Unwrap())
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		activityErr := &ActivityError{
			Op:  "Test",
			Err: nil,
		}

		assert.Nil(t, activityErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped ActivityError", func(t *testing.T) {
		activityErr := &ActivityError{
			Op:  "EnrollStudent",
			Err: ErrAlreadyEnrolled,
		}

		assert.True(t, errors.Is(activityErr, ErrAlreadyEnrolled))
	})

	t.Run("errors.As works with ActivityError", func(t *testing.T) {
		activityErr := &ActivityError{
			Op:  "EnrollStudent",
			Err: ErrGroupFull,
		}

		var targetErr *ActivityError
		assert.True(t, errors.As(activityErr, &targetErr))
		assert.Equal(t, "EnrollStudent", targetErr.Op)
	})
}
