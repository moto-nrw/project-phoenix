package platform_test

import (
	"errors"
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
