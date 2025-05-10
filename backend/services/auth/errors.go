// Package auth provides authentication and user management services
package auth

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidCredentials returned when username/password combo is invalid
	ErrInvalidCredentials = errors.New("invalid username or password")

	// ErrAccountNotFound returned when account doesn't exist
	ErrAccountNotFound = errors.New("account not found")

	// ErrAccountInactive returned when account is deactivated
	ErrAccountInactive = errors.New("account is inactive")

	// ErrEmailAlreadyExists returned when email is already registered
	ErrEmailAlreadyExists = errors.New("email already exists")

	// ErrUsernameAlreadyExists returned when username is already taken
	ErrUsernameAlreadyExists = errors.New("username already exists")

	// ErrInvalidToken returned when token format is invalid
	ErrInvalidToken = errors.New("invalid token format")

	// ErrTokenExpired returned when token has expired
	ErrTokenExpired = errors.New("token has expired")

	// ErrTokenNotFound returned when token is not found in the database
	ErrTokenNotFound = errors.New("token not found")

	// ErrPasswordTooWeak returned when password doesn't meet complexity requirements
	ErrPasswordTooWeak = errors.New("password doesn't meet complexity requirements")

	// ErrPasswordMismatch returned when passwords don't match
	ErrPasswordMismatch = errors.New("passwords don't match")
)

// AuthError represents an authentication-related error
type AuthError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

// Error returns the error message
func (e *AuthError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("auth error during %s", e.Op)
	}
	return fmt.Sprintf("auth error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *AuthError) Unwrap() error {
	return e.Err
}
