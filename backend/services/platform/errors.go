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
