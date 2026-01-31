package active

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestActiveServiceErrorVariables tests that error variables have correct messages
func TestActiveServiceErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrActiveGroupNotFound", ErrActiveGroupNotFound, "active group not found"},
		{"ErrVisitNotFound", ErrVisitNotFound, "visit not found"},
		{"ErrGroupSupervisorNotFound", ErrGroupSupervisorNotFound, "group supervisor not found"},
		{"ErrCombinedGroupNotFound", ErrCombinedGroupNotFound, "combined group not found"},
		{"ErrGroupMappingNotFound", ErrGroupMappingNotFound, "group mapping not found"},
		{"ErrStaffNotFound", ErrStaffNotFound, "staff member not found"},
		{"ErrStudentNotFound", ErrStudentNotFound, "student not found"},
		{"ErrActiveGroupAlreadyEnded", ErrActiveGroupAlreadyEnded, "active group session already ended"},
		{"ErrVisitAlreadyEnded", ErrVisitAlreadyEnded, "visit already ended"},
		{"ErrSupervisionAlreadyEnded", ErrSupervisionAlreadyEnded, "supervision already ended"},
		{"ErrCombinedGroupAlreadyEnded", ErrCombinedGroupAlreadyEnded, "combined group already ended"},
		{"ErrStudentAlreadyInGroup", ErrStudentAlreadyInGroup, "student already present in this group"},
		{"ErrGroupAlreadyInCombination", ErrGroupAlreadyInCombination, "group already part of this combination"},
		{"ErrInvalidTimeRange", ErrInvalidTimeRange, "invalid time range"},
		{"ErrCannotDeleteActiveGroup", ErrCannotDeleteActiveGroup, "cannot delete active group with active visits"},
		{"ErrStudentAlreadyActive", ErrStudentAlreadyActive, "student already has an active visit"},
		{"ErrStaffAlreadySupervising", ErrStaffAlreadySupervising, "staff member already supervising this group"},
		{"ErrInvalidData", ErrInvalidData, "invalid data provided"},
		{"ErrDatabaseOperation", ErrDatabaseOperation, "database operation failed"},
		{"ErrDeviceAlreadyActive", ErrDeviceAlreadyActive, "device is already running an activity session"},
		{"ErrNoActiveSession", ErrNoActiveSession, "no active session found"},
		{"ErrSessionConflict", ErrSessionConflict, "session conflict detected"},
		{"ErrInvalidActivitySession", ErrInvalidActivitySession, "invalid activity session parameters"},
		{"ErrRoomConflict", ErrRoomConflict, "room is already occupied by another active group"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestActiveServiceErrorsAreDistinct ensures each error can be identified uniquely
func TestActiveServiceErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrActiveGroupNotFound,
		ErrVisitNotFound,
		ErrGroupSupervisorNotFound,
		ErrCombinedGroupNotFound,
		ErrGroupMappingNotFound,
		ErrStaffNotFound,
		ErrStudentNotFound,
		ErrActiveGroupAlreadyEnded,
		ErrVisitAlreadyEnded,
		ErrSupervisionAlreadyEnded,
		ErrCombinedGroupAlreadyEnded,
		ErrStudentAlreadyInGroup,
		ErrGroupAlreadyInCombination,
		ErrInvalidTimeRange,
		ErrCannotDeleteActiveGroup,
		ErrStudentAlreadyActive,
		ErrStaffAlreadySupervising,
		ErrInvalidData,
		ErrDatabaseOperation,
		ErrDeviceAlreadyActive,
		ErrNoActiveSession,
		ErrSessionConflict,
		ErrInvalidActivitySession,
		ErrRoomConflict,
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

// TestActiveError tests the ActiveError custom error type
func TestActiveError(t *testing.T) {
	t.Run("Error with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("database connection lost")
		activeErr := &ActiveError{
			Op:  "CreateVisit",
			Err: underlyingErr,
		}

		expected := "active: CreateVisit: database connection lost"
		assert.Equal(t, expected, activeErr.Error())
	})

	t.Run("Error without underlying error", func(t *testing.T) {
		activeErr := &ActiveError{
			Op:  "EndVisit",
			Err: nil,
		}

		expected := "active: EndVisit: unknown error"
		assert.Equal(t, expected, activeErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrVisitNotFound
		activeErr := &ActiveError{
			Op:  "GetVisit",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, activeErr.Unwrap())
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		activeErr := &ActiveError{
			Op:  "Test",
			Err: nil,
		}

		assert.Nil(t, activeErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped ActiveError", func(t *testing.T) {
		activeErr := &ActiveError{
			Op:  "CreateVisit",
			Err: ErrStudentAlreadyActive,
		}

		assert.True(t, errors.Is(activeErr, ErrStudentAlreadyActive))
	})

	t.Run("errors.As works with ActiveError", func(t *testing.T) {
		activeErr := &ActiveError{
			Op:  "CreateVisit",
			Err: ErrStudentAlreadyActive,
		}

		var targetErr *ActiveError
		assert.True(t, errors.As(activeErr, &targetErr))
		assert.Equal(t, "CreateVisit", targetErr.Op)
	})
}
