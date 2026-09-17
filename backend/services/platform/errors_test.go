package platform_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
)

func TestOperatorNotFoundError_WithEmail(t *testing.T) {
	t.Parallel()

	err := &platform.OperatorNotFoundError{Email: "test@example.com"}
	assert.Contains(t, err.Error(), "test@example.com")
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorNotFoundError_WithOperatorID(t *testing.T) {
	t.Parallel()

	err := &platform.OperatorNotFoundError{OperatorID: 123}
	assert.Contains(t, err.Error(), "123")
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorNotFoundError_EmailTakesPrecedence(t *testing.T) {
	t.Parallel()

	err := &platform.OperatorNotFoundError{
		Email:      "test@example.com",
		OperatorID: 123,
	}
	// When email is present, it should be used in the error message
	assert.Contains(t, err.Error(), "test@example.com")
}

func TestInvalidCredentialsError(t *testing.T) {
	t.Parallel()

	err := &platform.InvalidCredentialsError{}
	assert.Equal(t, "invalid credentials", err.Error())
}

func TestOperatorInactiveError(t *testing.T) {
	t.Parallel()

	err := &platform.OperatorInactiveError{OperatorID: 456}
	assert.Contains(t, err.Error(), "456")
	assert.Contains(t, err.Error(), "inactive")
}

func TestInvalidDataError_WithError(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("validation failed")
	err := &platform.InvalidDataError{Err: innerErr}
	assert.Contains(t, err.Error(), "invalid data")
	assert.Contains(t, err.Error(), "validation failed")
}

func TestInvalidDataError_WithNilError(t *testing.T) {
	t.Parallel()

	err := &platform.InvalidDataError{Err: nil}
	assert.Contains(t, err.Error(), "invalid data")
}

func TestInvalidDataError_Unwrap(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("validation failed")
	err := &platform.InvalidDataError{Err: innerErr}

	assert.ErrorIs(t, err, innerErr)
	assert.Equal(t, innerErr, err.Unwrap())
}

func TestConflictError(t *testing.T) {
	t.Parallel()

	err := &platform.ConflictError{Err: errors.New("duplicate slug")}
	assert.Contains(t, err.Error(), "conflict")
	assert.Contains(t, err.Error(), "duplicate slug")
}

func TestPasswordMismatchError(t *testing.T) {
	t.Parallel()

	err := &platform.PasswordMismatchError{}
	assert.Equal(t, "current password is incorrect", err.Error())
}

func TestEmailAlreadyInUseError(t *testing.T) {
	t.Parallel()

	err := &platform.EmailAlreadyInUseError{}
	assert.Equal(t, "email address is already in use", err.Error())
}

func TestEmailChangeRateLimitError(t *testing.T) {
	t.Parallel()

	err := &platform.EmailChangeRateLimitError{}
	assert.Contains(t, err.Error(), "too many")
}

func TestEmailChangeSameEmailError(t *testing.T) {
	t.Parallel()

	err := &platform.EmailChangeSameEmailError{}
	assert.Contains(t, err.Error(), "same as current")
}

func TestEmailChangeTokenInvalidError(t *testing.T) {
	t.Parallel()

	err := &platform.EmailChangeTokenInvalidError{}
	assert.Contains(t, err.Error(), "invalid")
}
