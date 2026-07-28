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

func TestPersonNotFoundError(t *testing.T) {
	err := &platform.PersonNotFoundError{PersonID: 42}
	assert.Equal(t, "person with ID 42 not found", err.Error())
}

func TestPersonHasActiveSupervisionsError(t *testing.T) {
	err := &platform.PersonHasActiveSupervisionsError{PersonID: 42, Count: 3}
	assert.Equal(t, "person with ID 42 has 3 active supervision(s) and cannot be deleted", err.Error())
}

func TestEmailAlreadyInUseError(t *testing.T) {
	err := &platform.EmailAlreadyInUseError{}
	assert.Equal(t, "email address is already in use", err.Error())
}

func TestEmailChangeRateLimitError(t *testing.T) {
	err := &platform.EmailChangeRateLimitError{}
	assert.Contains(t, err.Error(), "too many")
}

func TestEmailChangeSameEmailError(t *testing.T) {
	err := &platform.EmailChangeSameEmailError{}
	assert.Contains(t, err.Error(), "same as current")
}

func TestEmailChangeTokenInvalidError(t *testing.T) {
	err := &platform.EmailChangeTokenInvalidError{}
	assert.Contains(t, err.Error(), "invalid")
}

func TestSchoolInactiveError(t *testing.T) {
	err := &platform.SchoolInactiveError{SchoolID: 99}
	assert.Contains(t, err.Error(), "99")
	assert.Contains(t, err.Error(), "inactive")
}

func TestSchoolAlreadyDeletedError(t *testing.T) {
	err := &platform.SchoolAlreadyDeletedError{SchoolID: 88}
	assert.Contains(t, err.Error(), "88")
	assert.Contains(t, err.Error(), "already soft-deleted")
}

func TestOrganizationAlreadyDeletedError(t *testing.T) {
	err := &platform.OrganizationAlreadyDeletedError{OrganizationID: 101}
	assert.Contains(t, err.Error(), "101")
	assert.Contains(t, err.Error(), "already soft-deleted")
}

func TestOrganizationNotDeletedError(t *testing.T) {
	err := &platform.OrganizationNotDeletedError{OrganizationID: 202}
	assert.Contains(t, err.Error(), "202")
	assert.Contains(t, err.Error(), "not soft-deleted")
}

func TestOrganizationHasSchoolsError(t *testing.T) {
	err := &platform.OrganizationHasSchoolsError{OrganizationID: 303, SchoolCount: 5}
	assert.Equal(t, "organization with ID 303 has 5 existing school(s) and cannot be deleted", err.Error())
}

func TestOrganizationDeletedError(t *testing.T) {
	err := &platform.OrganizationDeletedError{OrganizationID: 404}
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "soft-deleted")
	assert.Contains(t, err.Error(), "cannot host schools")
}

func TestSchoolNotDeletedError(t *testing.T) {
	err := &platform.SchoolNotDeletedError{SchoolID: 77}
	assert.Contains(t, err.Error(), "77")
	assert.Contains(t, err.Error(), "not soft-deleted")
}

func TestDeviceInUseError(t *testing.T) {
	err := &platform.DeviceInUseError{DeviceID: 66}
	assert.Contains(t, err.Error(), "66")
	assert.Contains(t, err.Error(), "still in use")
}

func TestDeviceProtectedError(t *testing.T) {
	err := &platform.DeviceProtectedError{DeviceID: 55, Reason: "web-manual device"}
	assert.Contains(t, err.Error(), "55")
	assert.Contains(t, err.Error(), "protected")
	assert.Contains(t, err.Error(), "web-manual device")
}

func TestAccountNotFoundError(t *testing.T) {
	err := &platform.AccountNotFoundError{AccountID: 42}
	assert.Contains(t, err.Error(), "42")
	assert.Contains(t, err.Error(), "not found")
}

func TestAccountTenantAccessNotFoundError(t *testing.T) {
	err := &platform.AccountTenantAccessNotFoundError{AccountID: 42, SchoolID: 7}
	assert.Contains(t, err.Error(), "42")
	assert.Contains(t, err.Error(), "7")
	assert.Contains(t, err.Error(), "no active access")
}
