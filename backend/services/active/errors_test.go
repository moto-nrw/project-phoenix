package active

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActiveError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ActiveError
		expected string
	}{
		{
			name: "error with operation and underlying error",
			err: &ActiveError{
				Op:  "CreateVisit",
				Err: ErrStudentAlreadyActive,
			},
			expected: "active: CreateVisit: student already has an active visit",
		},
		{
			name: "error with operation and nil error",
			err: &ActiveError{
				Op:  "DeleteGroup",
				Err: nil,
			},
			expected: "active: DeleteGroup: unknown error",
		},
		{
			name: "error with wrapped database error",
			err: &ActiveError{
				Op:  "GetVisit",
				Err: ErrDatabaseOperation,
			},
			expected: "active: GetVisit: database operation failed",
		},
		{
			name: "error with custom error",
			err: &ActiveError{
				Op:  "UpdateGroup",
				Err: errors.New("custom error message"),
			},
			expected: "active: UpdateGroup: custom error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestActiveError_Unwrap(t *testing.T) {
	tests := []struct {
		name        string
		err         *ActiveError
		expectedErr error
	}{
		{
			name: "unwrap returns underlying error",
			err: &ActiveError{
				Op:  "CreateVisit",
				Err: ErrStudentAlreadyActive,
			},
			expectedErr: ErrStudentAlreadyActive,
		},
		{
			name: "unwrap returns nil for nil error",
			err: &ActiveError{
				Op:  "DeleteGroup",
				Err: nil,
			},
			expectedErr: nil,
		},
		{
			name: "unwrap returns custom error",
			err: &ActiveError{
				Op:  "UpdateGroup",
				Err: errors.New("custom error"),
			},
			expectedErr: errors.New("custom error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Unwrap()
			if tt.expectedErr == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expectedErr.Error(), result.Error())
			}
		})
	}
}

func TestActiveError_ErrorsIs(t *testing.T) {
	// Test that errors.Is works correctly with ActiveError
	wrappedErr := &ActiveError{
		Op:  "CreateVisit",
		Err: ErrStudentAlreadyActive,
	}

	assert.True(t, errors.Is(wrappedErr, ErrStudentAlreadyActive))
	assert.False(t, errors.Is(wrappedErr, ErrVisitNotFound))
}

func TestActiveError_ErrorsAs(t *testing.T) {
	// Test that errors.As works correctly with ActiveError
	wrappedErr := &ActiveError{
		Op:  "CreateVisit",
		Err: ErrStudentAlreadyActive,
	}

	var activeErr *ActiveError
	assert.True(t, errors.As(wrappedErr, &activeErr))
	assert.Equal(t, "CreateVisit", activeErr.Op)
}

func TestPredefinedErrors(t *testing.T) {
	// Verify all predefined errors are properly defined
	errors := []struct {
		name string
		err  error
	}{
		{"ErrActiveGroupNotFound", ErrActiveGroupNotFound},
		{"ErrVisitNotFound", ErrVisitNotFound},
		{"ErrGroupSupervisorNotFound", ErrGroupSupervisorNotFound},
		{"ErrCombinedGroupNotFound", ErrCombinedGroupNotFound},
		{"ErrGroupMappingNotFound", ErrGroupMappingNotFound},
		{"ErrStaffNotFound", ErrStaffNotFound},
		{"ErrActiveGroupAlreadyEnded", ErrActiveGroupAlreadyEnded},
		{"ErrVisitAlreadyEnded", ErrVisitAlreadyEnded},
		{"ErrSupervisionAlreadyEnded", ErrSupervisionAlreadyEnded},
		{"ErrCombinedGroupAlreadyEnded", ErrCombinedGroupAlreadyEnded},
		{"ErrStudentAlreadyInGroup", ErrStudentAlreadyInGroup},
		{"ErrGroupAlreadyInCombination", ErrGroupAlreadyInCombination},
		{"ErrInvalidTimeRange", ErrInvalidTimeRange},
		{"ErrCannotDeleteActiveGroup", ErrCannotDeleteActiveGroup},
		{"ErrStudentAlreadyActive", ErrStudentAlreadyActive},
		{"ErrStaffAlreadySupervising", ErrStaffAlreadySupervising},
		{"ErrInvalidData", ErrInvalidData},
		{"ErrDatabaseOperation", ErrDatabaseOperation},
		{"ErrDeviceAlreadyActive", ErrDeviceAlreadyActive},
		{"ErrNoActiveSession", ErrNoActiveSession},
		{"ErrSessionConflict", ErrSessionConflict},
		{"ErrInvalidActivitySession", ErrInvalidActivitySession},
		{"ErrRoomConflict", ErrRoomConflict},
	}

	for _, e := range errors {
		t.Run(e.name, func(t *testing.T) {
			assert.NotNil(t, e.err)
			assert.NotEmpty(t, e.err.Error())
		})
	}
}
