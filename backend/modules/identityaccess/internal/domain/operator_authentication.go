package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Operator authentication errors (#3252). The HTTP layer switches on the
// sentinels; the texts are the retained operator auth service's.
var (
	ErrOperatorInvalidCredentials = errors.New("invalid credentials")
	ErrOperatorInactive           = errors.New("operator account is inactive")
	// ErrOperatorRefreshTokenInvalid refuses a refresh whose session expired,
	// was rotated already, was revoked, or never existed server-side.
	ErrOperatorRefreshTokenInvalid = errors.New("operator refresh token is invalid")
	ErrOperatorPasswordMismatch    = errors.New("current password is incorrect")
	// ErrOperatorAuthenticationUnavailable reports a module composed without
	// the operator dependencies.
	ErrOperatorAuthenticationUnavailable = errors.New("operator authentication is not composed")
)

// InvalidInputError rejects a request whose data cannot be applied: a blank
// display name, a weak password, a role that may not be handed out at the
// school. Err carries the message the caller may show.
type InvalidInputError struct{ Err error }

func (e *InvalidInputError) Error() string { return fmt.Sprintf("invalid data: %v", e.Err) }

func (e *InvalidInputError) Unwrap() error { return e.Err }

// InvalidInput builds the rejection for a message the caller may show.
func InvalidInput(message string) error { return &InvalidInputError{Err: errors.New(message)} }

// Operator claims and audit vocabulary as the retained flows wrote them.
const (
	// OperatorRoleName is the single role every operator access token carries.
	OperatorRoleName = "operator"
	// OperatorScope is the JWT scope of the platform portal.
	OperatorScope = ScopePlatform
	// OperatorMFAEnrollmentScope is the enrollment-token scope the codec maps
	// to the platform enrollment surface.
	OperatorMFAEnrollmentScope = "platform"

	OperatorAuditActionLogin        = "login"
	OperatorAuditActionTokenRevoked = "token_revoked"
	OperatorAuditActionCreate       = "create"
	OperatorAuditActionUpdate       = "update"
	OperatorAuditActionDelete       = "delete"
	OperatorAuditResourceOperator   = "operator"
	OperatorAuditResourceMapping    = "account_tenant"

	OperatorAuditActionMFAEmailSent          = "mfa_email_sent"
	OperatorAuditActionMFAVerified           = "mfa_verified"
	OperatorAuditActionMFAFailed             = "mfa_failed"
	OperatorAuditActionMFALocked             = "mfa_locked"
	OperatorAuditActionMFAEnrolled           = "mfa_enrolled"
	OperatorAuditActionMFADisabled           = "mfa_disabled"
	OperatorAuditActionMFATrustedDeviceAdded = "mfa_trusted_device_added"
	OperatorAuditActionMFAAdminOverride      = "mfa_admin_override"
	OperatorAuditResourceOperatorMFA         = "operator_mfa"
	OperatorAuditResourceAccount             = "account"
)

// OperatorAuditEntry is one platform-scoped ledger entry: who did what to
// which resource, with the request address and the typed change summary of
// the action (none for a login).
type OperatorAuditEntry struct {
	OperatorID   int64
	Action       string
	ResourceType string
	ResourceID   *int64
	IPAddress    string
	// RevokedSessions is set on token_revoked entries.
	RevokedSessions *RevokedSessionsEvidence
	// AccessChange is set on the create, update and delete entries of an
	// account's school access.
	AccessChange *OperatorAccessChange
	// MFA is set on the operator's own mfa_* entries and on the operator's
	// account-wide MFA override (#3331).
	MFA *OperatorMFAEvidence
}

// OperatorAccessChange summarizes one operator change of an account's
// school access. RoleID and RoleName describe the granted or new role,
// RemovedRoles the roles a role change dropped, AccountDeactivated whether
// a revocation removed the account's last school.
type OperatorAccessChange struct {
	SchoolID           int64
	Email              string
	RoleID             int64
	RoleName           string
	RemovedRoles       []string
	AccountDeactivated bool
}

// OperatorLoginResult is the discriminated operator login response.
type OperatorLoginResult struct {
	Status                LoginStatus
	AccessToken           string
	RefreshToken          string
	Operator              *Operator
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	// TrustedDeviceEnabled is populated on the MFA-required branch only.
	// Operator MFA has no per-tenant toggle; the feature is always on.
	TrustedDeviceEnabled bool
	TrustedDeviceDays    int
}

// OperatorSubject is the access-token subject of an operator.
func OperatorSubject(operatorID int64) string { return fmt.Sprintf("operator:%d", operatorID) }

// ValidateOperatorDisplayName applies the profile rule: a trimmed, non-empty
// name of at most 100 characters.
func ValidateOperatorDisplayName(displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	switch {
	case displayName == "":
		return "", InvalidInput("display name is required")
	case len(displayName) > maxOperatorDisplayNameLength:
		return "", InvalidInput("display name must not exceed 100 characters")
	}
	return displayName, nil
}
