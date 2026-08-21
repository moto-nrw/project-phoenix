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

func TestAnnouncementNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.AnnouncementNotFoundError{AnnouncementID: 789}
	assert.Contains(t, err.Error(), "789")
	assert.Contains(t, err.Error(), "not found")
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

func TestPostNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.PostNotFoundError{PostID: 111}
	assert.Contains(t, err.Error(), "111")
	assert.Contains(t, err.Error(), "not found")
}

func TestCommentNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.CommentNotFoundError{CommentID: 222}
	assert.Contains(t, err.Error(), "222")
	assert.Contains(t, err.Error(), "not found")
}

func TestPasswordMismatchError(t *testing.T) {
	t.Parallel()

	err := &platform.PasswordMismatchError{}
	assert.Equal(t, "current password is incorrect", err.Error())
}

func TestOrganizationNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.OrganizationNotFoundError{OrganizationID: 333}
	assert.Contains(t, err.Error(), "333")
	assert.Contains(t, err.Error(), "not found")
}

func TestSchoolNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.SchoolNotFoundError{SchoolID: 444}
	assert.Contains(t, err.Error(), "444")
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorDeviceNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.OperatorDeviceNotFoundError{DeviceID: 555}
	assert.Contains(t, err.Error(), "555")
	assert.Contains(t, err.Error(), "not found")
}

func TestDeviceTransferErrors(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"device with ID 11 cannot be transferred: protected",
		(&platform.DeviceTransferProtectedError{DeviceID: 11, Reason: "protected"}).Error(),
	)
	assert.Equal(t,
		"device with ID 12 cannot be transferred: device_online",
		(&platform.DeviceTransferBlockedError{DeviceID: 12, Reason: platform.DeviceTransferBlockedOnline}).Error(),
	)
	assert.Equal(t,
		"schools 13 and 14 belong to different organizations",
		(&platform.DeviceTransferOrganizationMismatchError{SourceSchoolID: 13, TargetSchoolID: 14}).Error(),
	)
	assert.Equal(t,
		"device already belongs to school 15",
		(&platform.DeviceTransferSameSchoolError{SchoolID: 15}).Error(),
	)
}

func TestPersonNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.PersonNotFoundError{PersonID: 42}
	assert.Equal(t, "person with ID 42 not found", err.Error())
}

func TestPersonHasActiveSupervisionsError(t *testing.T) {
	t.Parallel()

	err := &platform.PersonHasActiveSupervisionsError{PersonID: 42, Count: 3}
	assert.Equal(t, "person with ID 42 has 3 active supervision(s) and cannot be deleted", err.Error())
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

func TestSchoolInactiveError(t *testing.T) {
	t.Parallel()

	err := &platform.SchoolInactiveError{SchoolID: 99}
	assert.Contains(t, err.Error(), "99")
	assert.Contains(t, err.Error(), "inactive")
}

func TestSchoolAlreadyDeletedError(t *testing.T) {
	t.Parallel()

	err := &platform.SchoolAlreadyDeletedError{SchoolID: 88}
	assert.Contains(t, err.Error(), "88")
	assert.Contains(t, err.Error(), "already soft-deleted")
}

func TestOrganizationAlreadyDeletedError(t *testing.T) {
	t.Parallel()

	err := &platform.OrganizationAlreadyDeletedError{OrganizationID: 101}
	assert.Contains(t, err.Error(), "101")
	assert.Contains(t, err.Error(), "already soft-deleted")
}

func TestOrganizationNotDeletedError(t *testing.T) {
	t.Parallel()

	err := &platform.OrganizationNotDeletedError{OrganizationID: 202}
	assert.Contains(t, err.Error(), "202")
	assert.Contains(t, err.Error(), "not soft-deleted")
}

func TestOrganizationHasSchoolsError(t *testing.T) {
	t.Parallel()

	err := &platform.OrganizationHasSchoolsError{OrganizationID: 303, SchoolCount: 5}
	assert.Equal(t, "organization with ID 303 has 5 existing school(s) and cannot be deleted", err.Error())
}

func TestOrganizationDeletedError(t *testing.T) {
	t.Parallel()

	err := &platform.OrganizationDeletedError{OrganizationID: 404}
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "soft-deleted")
	assert.Contains(t, err.Error(), "cannot host schools")
}

func TestSchoolNotDeletedError(t *testing.T) {
	t.Parallel()

	err := &platform.SchoolNotDeletedError{SchoolID: 77}
	assert.Contains(t, err.Error(), "77")
	assert.Contains(t, err.Error(), "not soft-deleted")
}

func TestDeviceInUseError(t *testing.T) {
	t.Parallel()

	err := &platform.DeviceInUseError{DeviceID: 66}
	assert.Contains(t, err.Error(), "66")
	assert.Contains(t, err.Error(), "still in use")
}

func TestDeviceProtectedError(t *testing.T) {
	t.Parallel()

	err := &platform.DeviceProtectedError{DeviceID: 55, Reason: "web-manual device"}
	assert.Contains(t, err.Error(), "55")
	assert.Contains(t, err.Error(), "protected")
	assert.Contains(t, err.Error(), "web-manual device")
}

func TestAccountNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.AccountNotFoundError{AccountID: 42}
	assert.Contains(t, err.Error(), "42")
	assert.Contains(t, err.Error(), "not found")
}

func TestAccountTenantAccessNotFoundError(t *testing.T) {
	t.Parallel()

	err := &platform.AccountTenantAccessNotFoundError{AccountID: 42, SchoolID: 7}
	assert.Contains(t, err.Error(), "42")
	assert.Contains(t, err.Error(), "7")
	assert.Contains(t, err.Error(), "no active access")
}
