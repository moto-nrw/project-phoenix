package platform

import "fmt"

// OperatorNotFoundError is returned when an operator is not found
type OperatorNotFoundError struct {
	OperatorID int64
	Email      string
}

func (e *OperatorNotFoundError) Error() string {
	if e.Email != "" {
		return fmt.Sprintf("operator with email '%s' not found", e.Email)
	}
	return fmt.Sprintf("operator with ID %d not found", e.OperatorID)
}

// InvalidCredentialsError is returned when credentials are invalid
type InvalidCredentialsError struct{}

func (e *InvalidCredentialsError) Error() string {
	return "invalid credentials"
}

// OperatorInactiveError is returned when an operator account is inactive
type OperatorInactiveError struct {
	OperatorID int64
}

func (e *OperatorInactiveError) Error() string {
	return fmt.Sprintf("operator account %d is inactive", e.OperatorID)
}

// AnnouncementNotFoundError is returned when an announcement is not found
type AnnouncementNotFoundError struct {
	AnnouncementID int64
}

func (e *AnnouncementNotFoundError) Error() string {
	return fmt.Sprintf("announcement with ID %d not found", e.AnnouncementID)
}

// InvalidDataError is returned when data validation fails
type InvalidDataError struct {
	Err error
}

func (e *InvalidDataError) Error() string {
	return fmt.Sprintf("invalid data: %v", e.Err)
}

func (e *InvalidDataError) Unwrap() error {
	return e.Err
}

// ConflictError is returned when a write conflicts with existing data.
type ConflictError struct {
	Err error
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %v", e.Err)
}

// PostNotFoundError is returned when a suggestion post is not found
type PostNotFoundError struct {
	PostID int64
}

func (e *PostNotFoundError) Error() string {
	return fmt.Sprintf("suggestion post with ID %d not found", e.PostID)
}

// CommentNotFoundError is returned when an operator comment is not found
type CommentNotFoundError struct {
	CommentID int64
}

func (e *CommentNotFoundError) Error() string {
	return fmt.Sprintf("operator comment with ID %d not found", e.CommentID)
}

// PasswordMismatchError is returned when the current password does not match
type PasswordMismatchError struct{}

func (e *PasswordMismatchError) Error() string {
	return "current password is incorrect"
}

// OperatorRefreshTokenInvalidError is returned when an operator refresh token
// has expired, was rotated already, was revoked, or never existed server-side.
type OperatorRefreshTokenInvalidError struct{}

func (e *OperatorRefreshTokenInvalidError) Error() string {
	return "operator refresh token is invalid"
}

// OrganizationNotFoundError is returned when an organization does not exist.
type OrganizationNotFoundError struct {
	OrganizationID int64
}

func (e *OrganizationNotFoundError) Error() string {
	return fmt.Sprintf("organization with ID %d not found", e.OrganizationID)
}

// SchoolNotFoundError is returned when a school does not exist.
type SchoolNotFoundError struct {
	SchoolID int64
}

func (e *SchoolNotFoundError) Error() string {
	return fmt.Sprintf("school with ID %d not found", e.SchoolID)
}

// AccountNotFoundError is returned when an account does not exist.
type AccountNotFoundError struct {
	AccountID int64
}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf("account with ID %d not found", e.AccountID)
}

// AccountTenantAccessNotFoundError is returned when an account has no active
// mapping to the school an operation targets.
type AccountTenantAccessNotFoundError struct {
	AccountID int64
	SchoolID  int64
}

func (e *AccountTenantAccessNotFoundError) Error() string {
	return fmt.Sprintf("account %d has no active access to school %d", e.AccountID, e.SchoolID)
}

// SchoolInactiveError is returned when an operation targets an inactive school.
type SchoolInactiveError struct {
	SchoolID int64
}

func (e *SchoolInactiveError) Error() string {
	return fmt.Sprintf("school with ID %d is inactive", e.SchoolID)
}

// OperatorDeviceNotFoundError is returned when a device does not exist.
// Named "Operator..." to distinguish from services/iot.DeviceNotFoundError
// which uses string DeviceID (human-readable). This uses int64 DB primary key.
type OperatorDeviceNotFoundError struct {
	DeviceID int64
}

func (e *OperatorDeviceNotFoundError) Error() string {
	return fmt.Sprintf("device with ID %d not found", e.DeviceID)
}

// OrganizationAlreadyDeletedError is returned when trying to soft-delete an already deleted organization.
type OrganizationAlreadyDeletedError struct {
	OrganizationID int64
}

func (e *OrganizationAlreadyDeletedError) Error() string {
	return fmt.Sprintf("organization with ID %d is already soft-deleted", e.OrganizationID)
}

// OrganizationNotDeletedError is returned when trying to restore an organization that is not soft-deleted.
type OrganizationNotDeletedError struct {
	OrganizationID int64
}

func (e *OrganizationNotDeletedError) Error() string {
	return fmt.Sprintf("organization with ID %d is not soft-deleted", e.OrganizationID)
}

// OrganizationHasSchoolsError is returned when trying to delete an organization that still has non-deleted schools.
type OrganizationHasSchoolsError struct {
	OrganizationID int64
	SchoolCount    int
}

func (e *OrganizationHasSchoolsError) Error() string {
	return fmt.Sprintf("organization with ID %d has %d existing school(s) and cannot be deleted", e.OrganizationID, e.SchoolCount)
}

// OrganizationDeletedError is returned when a school operation targets a parent
// organization that has been soft-deleted. Blocks create/update/restore paths
// from attaching active schools to a deleted organization.
type OrganizationDeletedError struct {
	OrganizationID int64
}

func (e *OrganizationDeletedError) Error() string {
	return fmt.Sprintf("organization with ID %d is soft-deleted and cannot host schools", e.OrganizationID)
}

// SchoolAlreadyDeletedError is returned when trying to soft-delete an already deleted school.
type SchoolAlreadyDeletedError struct {
	SchoolID int64
}

func (e *SchoolAlreadyDeletedError) Error() string {
	return fmt.Sprintf("school with ID %d is already soft-deleted", e.SchoolID)
}

// SchoolNotDeletedError is returned when trying to restore a school that is not soft-deleted.
type SchoolNotDeletedError struct {
	SchoolID int64
}

func (e *SchoolNotDeletedError) Error() string {
	return fmt.Sprintf("school with ID %d is not soft-deleted", e.SchoolID)
}

// DeviceInUseError is returned when a device cannot be deleted because it is
// still referenced by attendance or active-group records.
type DeviceInUseError struct {
	DeviceID int64
}

func (e *DeviceInUseError) Error() string {
	return fmt.Sprintf("device with ID %d is still in use and cannot be deleted", e.DeviceID)
}

// DeviceProtectedError is returned when attempting to delete a system-managed
// device that must not be removed (e.g. the web-manual virtual device).
type DeviceProtectedError struct {
	DeviceID int64
	Reason   string
}

// DeviceTransferProtectedError is returned when a system-managed device must
// remain assigned to its original school.
type DeviceTransferProtectedError struct {
	DeviceID int64
	Reason   string
}

func (e *DeviceTransferProtectedError) Error() string {
	return fmt.Sprintf("device with ID %d cannot be transferred: %s", e.DeviceID, e.Reason)
}

const (
	DeviceTransferBlockedOnline        = "device_online"
	DeviceTransferBlockedActiveSession = "active_session"
)

// DeviceTransferBlockedError reports live device state that must end before a transfer.
type DeviceTransferBlockedError struct {
	DeviceID int64
	Reason   string
}

func (e *DeviceTransferBlockedError) Error() string {
	return fmt.Sprintf("device with ID %d cannot be transferred: %s", e.DeviceID, e.Reason)
}

// DeviceTransferOrganizationMismatchError prevents moving devices across organization boundaries.
type DeviceTransferOrganizationMismatchError struct {
	SourceSchoolID int64
	TargetSchoolID int64
}

func (e *DeviceTransferOrganizationMismatchError) Error() string {
	return fmt.Sprintf("schools %d and %d belong to different organizations", e.SourceSchoolID, e.TargetSchoolID)
}

// DeviceTransferSameSchoolError reports a no-op transfer request.
type DeviceTransferSameSchoolError struct {
	SchoolID int64
}

func (e *DeviceTransferSameSchoolError) Error() string {
	return fmt.Sprintf("device already belongs to school %d", e.SchoolID)
}

func (e *DeviceProtectedError) Error() string {
	return fmt.Sprintf("device with ID %d is protected: %s", e.DeviceID, e.Reason)
}

// PersonNotFoundError is returned when a person does not exist or was already soft-deleted.
type PersonNotFoundError struct {
	PersonID int64
}

func (e *PersonNotFoundError) Error() string {
	return fmt.Sprintf("person with ID %d not found", e.PersonID)
}

// EmailAlreadyInUseError is returned when the requested email is already taken by another operator
type EmailAlreadyInUseError struct{}

func (e *EmailAlreadyInUseError) Error() string {
	return "email address is already in use"
}

// EmailChangeRateLimitError is returned when too many email change requests have been made
type EmailChangeRateLimitError struct{}

func (e *EmailChangeRateLimitError) Error() string {
	return "too many email change attempts, please wait"
}

// EmailChangeSameEmailError is returned when the new email matches the current email
type EmailChangeSameEmailError struct{}

func (e *EmailChangeSameEmailError) Error() string {
	return "new email is the same as current email"
}

// EmailChangeTokenInvalidError is returned when a confirmation token is not found, expired, or already used
type EmailChangeTokenInvalidError struct{}

func (e *EmailChangeTokenInvalidError) Error() string {
	return "email change token is invalid, expired, or already used"
}

// OperatorInvitationNotFoundError is returned when an invitation token is not found, expired, or already used
type OperatorInvitationNotFoundError struct{}

func (e *OperatorInvitationNotFoundError) Error() string {
	return "operator invitation not found, expired, or already used"
}

// OperatorInvitationEmailExistsError is returned when an operator with that email already exists
type OperatorInvitationEmailExistsError struct{}

func (e *OperatorInvitationEmailExistsError) Error() string {
	return "an operator with this email already exists"
}

// OperatorInvitationRateLimitError is returned when an inviter has created
// too many invitation tokens within the rate-limit window.
type OperatorInvitationRateLimitError struct{}

func (e *OperatorInvitationRateLimitError) Error() string {
	return "too many invitation attempts, please wait"
}

// PersonHasActiveSupervisionsError is returned when a person cannot be deleted
// because the associated staff member has active group supervisions.
type PersonHasActiveSupervisionsError struct {
	PersonID int64
	Count    int
}

func (e *PersonHasActiveSupervisionsError) Error() string {
	return fmt.Sprintf("person with ID %d has %d active supervision(s) and cannot be deleted", e.PersonID, e.Count)
}
