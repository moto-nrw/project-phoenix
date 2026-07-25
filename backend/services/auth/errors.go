// Package auth provides authentication and user management services
package auth

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidCredentials returned when username/password combo is invalid
	ErrInvalidCredentials = errors.New("invalid username or password")

	// ErrAccountNotFound returned when account doesn't exist
	ErrAccountNotFound = errors.New("account not found")

	// ErrAccountInactive returned when account is deactivated
	ErrAccountInactive = errors.New("account is inactive")

	// ErrEmailAlreadyExists returned when email is already registered
	ErrEmailAlreadyExists = errors.New("Diese E-Mail-Adresse ist bereits registriert") //nolint:staticcheck // ST1005: user-facing German message

	// ErrUsernameAlreadyExists returned when username is already taken
	ErrUsernameAlreadyExists = errors.New("Dieser Benutzername ist bereits vergeben") //nolint:staticcheck // ST1005: user-facing German message

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

	// ErrRateLimitExceeded returned when password reset attempts exceed rate limit
	ErrRateLimitExceeded = errors.New("too many password reset requests")

	// ErrPermissionNotFound returned when permission doesn't exist
	ErrPermissionNotFound = errors.New("permission not found")

	// ErrRoleNotFound returned when role doesn't exist
	ErrRoleNotFound = errors.New("role not found")

	// ErrSystemRoleImmutable returned when attempting to modify a system role
	ErrSystemRoleImmutable = errors.New("system roles cannot be modified")

	// ErrTenantRequiredForRoleAssignment returned when tenant-scoped role setup is requested without a tenant context
	ErrTenantRequiredForRoleAssignment = errors.New("tenant context is required when assigning a role during registration")

	// ErrParentAccountNotFound returned when parent account doesn't exist
	ErrParentAccountNotFound = errors.New("parent account not found")

	// Tenant errors
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTenantAccessDenied = errors.New("account does not have access to this tenant")

	// Parent / staff portal split. Returned when an account tries to log
	// in at the wrong portal:
	//   - guardian-only account hitting the tenant login → ErrParentMustUseParentPortal
	//   - account with no guardian role hitting the parents login → ErrAccountNoGuardianRole
	// Frontend turns these into a clear redirect message ("please log in
	// at https://parents.{TENANT_DOMAIN}/" or vice versa).
	ErrParentMustUseParentPortal = errors.New("guardian accounts must log in at the parents portal")
	ErrAccountNoGuardianRole     = errors.New("account is not a guardian at any school")

	// Invitation errors
	ErrInvitationNotFound            = errors.New("invitation not found")
	ErrInvitationExpired             = errors.New("invitation has expired")
	ErrInvitationUsed                = errors.New("invitation has already been used")
	ErrGuardianInvitationPending     = errors.New("guardian already has an active invitation")
	ErrInvitationTenantDeleted       = errors.New("the school for this invitation has been deleted")
	ErrInvitationNameRequired        = errors.New("first name and last name are required")
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")

	// Related-accounts errors
	ErrCannotRemovePrimaryGuardian      = errors.New("the primary guardian cannot be removed by a parent")
	ErrCannotRemoveStaffManagedGuardian = errors.New("staff-managed guardian contacts cannot be removed by a parent")
	ErrCannotRemoveOwnAccess            = errors.New("a parent cannot remove their own access to a child")
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

// RateLimitError provides additional context for rate-limit responses.
type RateLimitError struct {
	Err      error
	Attempts int
	RetryAt  time.Time
}

// Error returns the error message for the rate limit error.
func (e *RateLimitError) Error() string {
	if e.Err == nil {
		return "rate limit exceeded"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// RetryAfterSeconds returns the positive number of seconds until retry, or zero if already allowed.
func (e *RateLimitError) RetryAfterSeconds(now time.Time) int {
	if e == nil || e.RetryAt.IsZero() {
		return 0
	}
	if !e.RetryAt.After(now) {
		return 0
	}
	return int(e.RetryAt.Sub(now).Seconds())
}
