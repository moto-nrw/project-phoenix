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

	// ErrInvalidStaffPINCredentials hides whether a requested staff/account
	// exists when kiosk staff-PIN authentication fails.
	ErrInvalidStaffPINCredentials = errors.New("invalid staff PIN credentials")

	// ErrStaffPINLocked reports an active staff-account PIN lockout.
	ErrStaffPINLocked = errors.New("staff PIN is temporarily locked")

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

	// School portal split (#2207). Returned when an account without a
	// school-portal role (today: the lehrkraft system role) tries to log
	// in at the school portal. Deliberately named after the portal, not
	// the role — a future Schulleitung role widens isSchoolPortalRole
	// without renaming this sentinel or its wire code.
	ErrAccountNoSchoolPortalRole = errors.New("account has no school portal role at any school")

	// ErrMustUseSchoolPortal is the symmetric half of the school portal
	// split: an account whose only role at this school is a school-portal
	// role (today: lehrkraft) has no reachable surface in the OGS tenant
	// portal, so the tenant login refuses it and points at moto schule.
	// Dual-role accounts (also caregiver, admin, or guest) pass through
	// unchanged — same rule as the guardian split above.
	ErrMustUseSchoolPortal = errors.New("school portal accounts must log in at the school portal")

	// Admin staff-view preview (#2893)
	// ErrPreviewSelf: previewing your own account is pointless and refused.
	ErrPreviewSelf = errors.New("cannot preview your own account")
	// ErrPreviewTargetNotStaff: the target has no tenant-portal surface at
	// this school (guardian-only, or no role at all).
	ErrPreviewTargetNotStaff = errors.New("account is not a staff member at this school")

	// Invitation errors
	ErrInvitationNotFound            = errors.New("invitation not found")
	ErrInvitationExpired             = errors.New("invitation has expired")
	ErrInvitationUsed                = errors.New("invitation has already been used")
	ErrInvitationTenantDeleted       = errors.New("the school for this invitation has been deleted")
	ErrInvitationNameRequired        = errors.New("first name and last name are required")
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")

	// Related-accounts errors
	ErrCannotRemovePrimaryGuardian      = errors.New("the primary guardian cannot be removed by a parent")
	ErrCannotRemoveStaffManagedGuardian = errors.New("staff-managed guardian contacts cannot be removed by a parent")
	ErrCannotRemoveOwnAccess            = errors.New("a parent cannot remove their own access to a child")
	// ErrCannotRemovePayerGuardian: the link carries the child's payer mark
	// (#2608). Clearing it is a financial decision that needs
	// guardians:financial; the parents portal never holds that, so the payer
	// stays until the school reassigns the payment account. The sentence is
	// German because the parents portal shows it verbatim.
	ErrCannotRemovePayerGuardian = errors.New("Diese Person ist als Zahler für das Kind eingetragen und kann nicht entfernt werden. Bitte wenden Sie sich an die Schule.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrInviteSocialWorkerManaged: the invited email belongs to a contact
	// linked to this child as a social worker. That is a school-managed
	// professional contact — the invite flow must never turn it into a legal
	// guardian, so the invite is refused entirely (mirrors
	// ErrGuardianSocialWorkerManaged on the parent edit paths).
	ErrInviteSocialWorkerManaged = errors.New("a social-worker contact is managed by the school and cannot be invited to the parents portal")
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
