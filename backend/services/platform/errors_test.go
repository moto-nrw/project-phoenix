package platform_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
)

func TestOperatorNotFoundError_WithEmail(t *testing.T) {
	err := &platform.OperatorNotFoundError{Email: "test@example.com"}
	assert.Contains(t, err.Error(), "test@example.com")
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorNotFoundError_WithOperatorID(t *testing.T) {
	err := &platform.OperatorNotFoundError{OperatorID: 123}
	assert.Contains(t, err.Error(), "123")
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorNotFoundError_EmailTakesPrecedence(t *testing.T) {
	err := &platform.OperatorNotFoundError{
		Email:      "test@example.com",
		OperatorID: 123,
	}
	// When email is present, it should be used in the error message
	assert.Contains(t, err.Error(), "test@example.com")
}

func TestInvalidCredentialsError(t *testing.T) {
	err := &platform.InvalidCredentialsError{}
	assert.Equal(t, "invalid credentials", err.Error())
}

func TestOperatorInactiveError(t *testing.T) {
	err := &platform.OperatorInactiveError{OperatorID: 456}
	assert.Contains(t, err.Error(), "456")
	assert.Contains(t, err.Error(), "inactive")
}

func TestAnnouncementNotFoundError(t *testing.T) {
	err := &platform.AnnouncementNotFoundError{AnnouncementID: 789}
	assert.Contains(t, err.Error(), "789")
	assert.Contains(t, err.Error(), "not found")
}

func TestInvalidDataError_WithError(t *testing.T) {
	innerErr := errors.New("validation failed")
	err := &platform.InvalidDataError{Err: innerErr}
	assert.Contains(t, err.Error(), "invalid data")
	assert.Contains(t, err.Error(), "validation failed")
}

func TestInvalidDataError_WithNilError(t *testing.T) {
	err := &platform.InvalidDataError{Err: nil}
	assert.Contains(t, err.Error(), "invalid data")
}

func TestInvalidDataError_Unwrap(t *testing.T) {
	innerErr := errors.New("validation failed")
	err := &platform.InvalidDataError{Err: innerErr}

	assert.ErrorIs(t, err, innerErr)
	assert.Equal(t, innerErr, err.Unwrap())
}

func TestConflictError(t *testing.T) {
	err := &platform.ConflictError{Err: errors.New("duplicate slug")}
	assert.Contains(t, err.Error(), "conflict")
	assert.Contains(t, err.Error(), "duplicate slug")
}

func TestPostNotFoundError(t *testing.T) {
	err := &platform.PostNotFoundError{PostID: 111}
	assert.Contains(t, err.Error(), "111")
	assert.Contains(t, err.Error(), "not found")
}

func TestCommentNotFoundError(t *testing.T) {
	err := &platform.CommentNotFoundError{CommentID: 222}
	assert.Contains(t, err.Error(), "222")
	assert.Contains(t, err.Error(), "not found")
}

func TestPasswordMismatchError(t *testing.T) {
	err := &platform.PasswordMismatchError{}
	assert.Equal(t, "current password is incorrect", err.Error())
}

func TestOrganizationNotFoundError(t *testing.T) {
	err := &platform.OrganizationNotFoundError{OrganizationID: 333}
	assert.Contains(t, err.Error(), "333")
	assert.Contains(t, err.Error(), "not found")
}

func TestSchoolNotFoundError(t *testing.T) {
	err := &platform.SchoolNotFoundError{SchoolID: 444}
	assert.Contains(t, err.Error(), "444")
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorDeviceNotFoundError(t *testing.T) {
	err := &platform.OperatorDeviceNotFoundError{DeviceID: 555}
	assert.Contains(t, err.Error(), "555")
	assert.Contains(t, err.Error(), "not found")
}

func TestSchoolAlreadyDeletedError_Error(t *testing.T) {
	err := &platform.SchoolAlreadyDeletedError{SchoolID: 42}
	assert.Equal(t, "school with ID 42 is already soft-deleted", err.Error())
}

func TestSchoolNotDeletedError_Error(t *testing.T) {
	err := &platform.SchoolNotDeletedError{SchoolID: 99}
	assert.Equal(t, "school with ID 99 is not soft-deleted", err.Error())
}

func TestTenantPurgeError_Error(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	err := &platform.TenantPurgeError{SchoolID: 7, Err: inner}
	assert.Equal(t, "failed to purge tenant 7: connection refused", err.Error())
}

func TestTenantPurgeError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	err := &platform.TenantPurgeError{SchoolID: 7, Err: inner}
	assert.Equal(t, inner, err.Unwrap())
}

func TestErrorTypes_CanBeMatchedWithErrorsAs(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "SchoolAlreadyDeletedError",
			err:  fmt.Errorf("wrapped: %w", &platform.SchoolAlreadyDeletedError{SchoolID: 10}),
		},
		{
			name: "SchoolNotDeletedError",
			err:  fmt.Errorf("wrapped: %w", &platform.SchoolNotDeletedError{SchoolID: 20}),
		},
		{
			name: "TenantPurgeError",
			err:  fmt.Errorf("wrapped: %w", &platform.TenantPurgeError{SchoolID: 30, Err: errors.New("db down")}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "SchoolAlreadyDeletedError":
				var target *platform.SchoolAlreadyDeletedError
				assert.True(t, errors.As(tt.err, &target), "errors.As should match %s", tt.name)
				assert.Equal(t, int64(10), target.SchoolID)
			case "SchoolNotDeletedError":
				var target *platform.SchoolNotDeletedError
				assert.True(t, errors.As(tt.err, &target), "errors.As should match %s", tt.name)
				assert.Equal(t, int64(20), target.SchoolID)
			case "TenantPurgeError":
				var target *platform.TenantPurgeError
				assert.True(t, errors.As(tt.err, &target), "errors.As should match %s", tt.name)
				assert.Equal(t, int64(30), target.SchoolID)
			}
		})
	}
}
