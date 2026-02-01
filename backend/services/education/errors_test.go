package education

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEducationErrorVariables tests that error variables have correct messages
func TestEducationErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrGroupNotFound", ErrGroupNotFound, "education group not found"},
		{"ErrTeacherNotFound", ErrTeacherNotFound, "teacher not found"},
		{"ErrGroupTeacherNotFound", ErrGroupTeacherNotFound, "group-teacher relationship not found"},
		{"ErrSubstitutionNotFound", ErrSubstitutionNotFound, "substitution not found"},
		{"ErrRoomNotFound", ErrRoomNotFound, "room not found"},
		{"ErrDuplicateGroup", ErrDuplicateGroup, "a group with this name already exists"},
		{"ErrDuplicateTeacherInGroup", ErrDuplicateTeacherInGroup, "this teacher is already assigned to the group"},
		{"ErrSubstitutionConflict", ErrSubstitutionConflict, "substitution conflicts with an existing one"},
		{"ErrSameTeacherSubstitution", ErrSameTeacherSubstitution, "regular staff and substitute staff cannot be the same"},
		{"ErrInvalidDateRange", ErrInvalidDateRange, "invalid date range"},
		{"ErrDatabaseOperation", ErrDatabaseOperation, "database operation failed"},
		{"ErrInvalidData", ErrInvalidData, "invalid data provided"},
		{"ErrSubstitutionBackdated", ErrSubstitutionBackdated, "substitutions cannot be created or updated for past dates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestEducationErrorsAreDistinct ensures each error can be identified uniquely
func TestEducationErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrGroupNotFound,
		ErrTeacherNotFound,
		ErrGroupTeacherNotFound,
		ErrSubstitutionNotFound,
		ErrRoomNotFound,
		ErrDuplicateGroup,
		ErrDuplicateTeacherInGroup,
		ErrSubstitutionConflict,
		ErrSameTeacherSubstitution,
		ErrInvalidDateRange,
		ErrDatabaseOperation,
		ErrInvalidData,
		ErrSubstitutionBackdated,
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

// TestEducationError tests the EducationError custom error type
func TestEducationError(t *testing.T) {
	t.Run("Error with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("database constraint violation")
		educationErr := &EducationError{
			Op:  "CreateGroup",
			Err: underlyingErr,
		}

		expected := "education: CreateGroup: database constraint violation"
		assert.Equal(t, expected, educationErr.Error())
	})

	t.Run("Error without underlying error", func(t *testing.T) {
		educationErr := &EducationError{
			Op:  "UpdateGroup",
			Err: nil,
		}

		expected := "education: UpdateGroup: unknown error"
		assert.Equal(t, expected, educationErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrGroupNotFound
		educationErr := &EducationError{
			Op:  "GetGroup",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, educationErr.Unwrap())
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		educationErr := &EducationError{
			Op:  "Test",
			Err: nil,
		}

		assert.Nil(t, educationErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped EducationError", func(t *testing.T) {
		educationErr := &EducationError{
			Op:  "CreateGroup",
			Err: ErrDuplicateGroup,
		}

		assert.True(t, errors.Is(educationErr, ErrDuplicateGroup))
	})

	t.Run("errors.As works with EducationError", func(t *testing.T) {
		educationErr := &EducationError{
			Op:  "CreateGroup",
			Err: ErrDuplicateGroup,
		}

		var targetErr *EducationError
		assert.True(t, errors.As(educationErr, &targetErr))
		assert.Equal(t, "CreateGroup", targetErr.Op)
	})
}
