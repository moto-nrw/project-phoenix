package usercontext

import (
	"errors"
	"fmt"
)

// Common error variables for the usercontext service
var (
	// ErrUserNotFound indicates the user could not be found
	ErrUserNotFound = errors.New("user not found")

	// ErrUserNotAuthenticated indicates the user is not authenticated
	ErrUserNotAuthenticated = errors.New("user not authenticated")

	// ErrUserNotAuthorized indicates the user does not have the required permissions
	ErrUserNotAuthorized = errors.New("user not authorized")

	// ErrUserNotLinkedToPerson indicates the user account is not linked to a person
	ErrUserNotLinkedToPerson = errors.New("user account not linked to a person")

	// ErrUserNotLinkedToStaff indicates the user is not linked to a staff member
	ErrUserNotLinkedToStaff = errors.New("user not linked to a staff member")

	// ErrUserNotLinkedToTeacher indicates the user is not linked to a teacher
	ErrUserNotLinkedToTeacher = errors.New("user not linked to a teacher")

	// ErrNoActiveGroups indicates the user has no active groups
	ErrNoActiveGroups = errors.New("user has no active groups")

	// ErrGroupNotFound indicates the requested group could not be found
	ErrGroupNotFound = errors.New("group not found")

	// ErrInvalidOperation indicates the requested operation is invalid for the current user
	ErrInvalidOperation = errors.New("invalid operation for current user")
)

// UserContextError represents an error in the usercontext service
type UserContextError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

// Error returns the error message
func (e *UserContextError) Error() string {
	return fmt.Sprintf("usercontext.%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *UserContextError) Unwrap() error {
	return e.Err
}