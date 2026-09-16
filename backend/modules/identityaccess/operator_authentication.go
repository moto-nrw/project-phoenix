package identityaccess

import (
	"context"
	"errors"
	"fmt"
)

// Operator authentication (#3252): platform operator login, the MFA-proven
// token issue, refresh with rotation recovery, profile and password changes
// and the session revocation they entail. The HTTP layer switches on the
// sentinels.
var (
	ErrOperatorInvalidCredentials = errors.New("invalid credentials")
	ErrOperatorInactive           = errors.New("operator account is inactive")
	// ErrOperatorRefreshTokenInvalid refuses a refresh whose session
	// expired, was rotated already, was revoked, or never existed
	// server-side.
	ErrOperatorRefreshTokenInvalid = errors.New("operator refresh token is invalid")
	ErrOperatorPasswordMismatch    = errors.New("current password is incorrect")
	// ErrOperatorAuthenticationUnavailable reports a module composed without
	// the operator dependencies (repository fixtures, CLI roots that only
	// read).
	ErrOperatorAuthenticationUnavailable = errors.New("operator authentication is not composed")
)

// InvalidInputError rejects a request whose data cannot be applied: a blank
// display name, a weak password, a role that may not be handed out at the
// school, a missing name for a new staff identity. Err carries the message
// the caller may show.
type InvalidInputError struct{ Err error }

func (e *InvalidInputError) Error() string { return fmt.Sprintf("invalid data: %v", e.Err) }

func (e *InvalidInputError) Unwrap() error { return e.Err }

// OperatorLoginResult is the discriminated operator login response. Exactly
// one of (AccessToken+RefreshToken), the enrollment AccessToken or
// ChallengeToken is populated; Operator accompanies the authenticated and
// the enrollment branches.
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

// OperatorAuditEntry is one platform-scoped ledger entry the operator flows
// append through the Audit owner: who did what to which resource, with the
// request address and the typed change summary of the action.
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

// OperatorAuthentication is the capability operator login, the MFA and
// passkey token exchange, refresh, profile and password changes consume.
type OperatorAuthentication interface {
	// LoginOperatorWithMFAGate checks the credentials and consults the
	// mandatory operator MFA gate: an enrolled operator without a
	// verifiable trusted-device cookie receives a challenge, an operator
	// not yet enrolled an enrollment-scoped token.
	LoginOperatorWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*OperatorLoginResult, error)
	// IssueTokensForAuthenticatedOperator mints a token pair for an
	// operator whose identity was proven via a non-password channel.
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// RefreshOperatorToken rotates the persisted refresh session and issues
	// a new token pair; replays outside the recovery grace revoke the
	// family.
	RefreshOperatorToken(ctx context.Context, operatorID int64, refreshToken string) (accessToken, newRefreshToken string, err error)
	UpdateOperatorProfile(ctx context.Context, operatorID int64, displayName string) (Operator, error)
	// ChangeOperatorPassword rotates the password and revokes every refresh
	// session and pending e-mail change of the operator atomically.
	ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error
}

func (m *Module) LoginOperatorWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*OperatorLoginResult, error) {
	return m.engine.LoginOperatorWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
}

func (m *Module) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	return m.engine.IssueTokensForAuthenticatedOperator(ctx, operatorID, ipAddress, userAgent)
}

func (m *Module) RefreshOperatorToken(ctx context.Context, operatorID int64, refreshToken string) (string, string, error) {
	return m.engine.RefreshOperatorToken(ctx, operatorID, refreshToken)
}

func (m *Module) UpdateOperatorProfile(ctx context.Context, operatorID int64, displayName string) (Operator, error) {
	return m.engine.UpdateOperatorProfile(ctx, operatorID, displayName)
}

func (m *Module) ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	return m.engine.ChangeOperatorPassword(ctx, operatorID, currentPassword, newPassword)
}
