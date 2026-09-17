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

// PasswordMismatchError is returned when the current password does not match
type PasswordMismatchError struct{}

func (e *PasswordMismatchError) Error() string {
	return "current password is incorrect"
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
