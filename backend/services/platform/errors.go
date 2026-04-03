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

// OperatorInvitationNotFoundError is returned when an invitation token is not found
type OperatorInvitationNotFoundError struct {
	InvitationID int64
}

func (e *OperatorInvitationNotFoundError) Error() string {
	return fmt.Sprintf("operator invitation with ID %d not found", e.InvitationID)
}

// OperatorInvitationExpiredOrUsedError is returned when an invitation token is expired or already used
type OperatorInvitationExpiredOrUsedError struct{}

func (e *OperatorInvitationExpiredOrUsedError) Error() string {
	return "operator invitation is invalid, expired, or already used"
}

// OperatorInvitationAlreadyAcceptedError is returned when trying to resend/revoke an already used invitation
type OperatorInvitationAlreadyAcceptedError struct {
	InvitationID int64
}

func (e *OperatorInvitationAlreadyAcceptedError) Error() string {
	return fmt.Sprintf("operator invitation %d has already been accepted", e.InvitationID)
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
