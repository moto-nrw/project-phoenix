package auth

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// ErrAccountSessionsUnavailable reports a service composed without the
// Identity & Access account-authentication port.
var ErrAccountSessionsUnavailable = errors.New("account sessions are not composed")

// LoginStatus discriminates the two shapes a /auth/login response can take.
type LoginStatus string

const (
	// LoginStatusAuthenticated means credential check + (if applicable) MFA
	// passed and the response carries a usable token pair.
	LoginStatusAuthenticated LoginStatus = "authenticated"
	// LoginStatusMFARequired means credentials were valid but the account
	// must present a second factor before tokens are issued. The response
	// carries a short-lived challenge token and (optional) UX hints.
	LoginStatusMFARequired LoginStatus = "mfa_required"
	// LoginStatusMFAEnrollmentRequired means credentials were valid but the
	// tenant requires MFA and the account has no credential yet. The
	// response carries an enrollment-scoped access token (no refresh) that
	// authorizes only /auth/mfa/enroll/*. The full session is minted after
	// successful enrollment.
	LoginStatusMFAEnrollmentRequired LoginStatus = "mfa_enrollment_required"
)

// MFAEnrollmentTokenTTL is the lifetime of an enrollment-scoped JWT. Long
// enough to fetch the emailed code and confirm enrollment in one sitting,
// short enough that an abandoned enrollment session does not linger.
const MFAEnrollmentTokenTTL = 15 * time.Minute

// LoginResult is the discriminated response shape for LoginWithMFAGate.
// Exactly one of (AccessToken+RefreshToken) or ChallengeToken is populated.
// MFAEnrollmentRequired flags accounts that have a token pair *and* a
// pending forced enrollment — the frontend uses it to redirect to the
// enrollment screen before showing the dashboard.
type LoginResult struct {
	Status                LoginStatus
	AccessToken           string
	RefreshToken          string
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	// TrustedDeviceEnabled is populated on the MFA-required branch only.
	// It mirrors security.mfa_trusted_device_enabled for the tenant so the
	// frontend can hide the "remember this device" checkbox when the admin
	// has disabled the feature.
	TrustedDeviceEnabled bool
	// TrustedDeviceDays is populated on the MFA-required branch only. It
	// mirrors security.mfa_trusted_device_days so the frontend can render
	// the exact label ("Auf diesem Gerät N Tage merken") that matches the
	// cookie lifetime the backend will actually issue.
	TrustedDeviceDays int
}

// RevokedSession identifies one refresh session a revocation removed.
type RevokedSession struct {
	ID          int64
	AccountID   int64
	TenantID    int64
	FamilyID    string
	PortalScope string
}

// ActiveSession is one live refresh session of an account.
type ActiveSession struct {
	ID         int64
	Token      string
	Expiry     time.Time
	Mobile     bool
	Identifier string
	CreatedAt  time.Time
}

// AccountRoleClaim is one role an account holds at a school.
type AccountRoleClaim struct {
	ID       int64
	Name     string
	IsSystem bool
	TenantID *int64
}

// AccountClaims is the tenant-scoped claims material of an account: roles,
// permissions, person names, admin flag and organization.
type AccountClaims struct {
	RoleNames   []string
	Roles       []AccountRoleClaim
	Permissions []string
	Username    string
	FirstName   string
	LastName    string
	IsAdmin     bool
	TenantID    int64
	OrgID       int64
	Scope       string
}

// AccountSessions is the consumer-owned port over the Identity & Access
// account-authentication capability (#3251). The composition root binds it;
// every error already carries the AuthError envelope and the sentinels of
// this package.
type AccountSessions interface {
	LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (accessToken, refreshToken string, err error)
	LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*LoginResult, error)
	LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*LoginResult, error)
	LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*LoginResult, error)
	IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (accessToken, newRefreshToken string, err error)
	LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error
	SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (accessToken, refreshToken string, err error)
	SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error)
	ValidateSessionTokens(ctx context.Context, accessToken, refreshToken, portal string) (*jwt.AppClaims, error)
	VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error)
	CountExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredTokens(ctx context.Context) (int, error)
	ListActiveSessions(ctx context.Context, accountID int64) ([]ActiveSession, error)
	ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error)
	RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error
	RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error)
	// DeleteAccountSessionsWithAudit revokes the account's sessions at the
	// caller's school on the caller's transaction (audit evidence included),
	// or schedules the account-wide wipe an account-wide reason asks for.
	DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]RevokedSession, error)
	// QueuePushCleanup removes the push subscriptions the revoked sessions
	// orphaned once the caller's transaction commits.
	QueuePushCleanup(ctx context.Context, accountID int64, revoked []RevokedSession, reason string)
	ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error
	MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error
	// LoadAccountClaims runs on the caller's transaction.
	LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (*AccountClaims, error)
	FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error)
	FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error)
}

func (s *Service) accountSessions(op string) (AccountSessions, error) {
	if s.sessions == nil {
		return nil, &AuthError{Op: op, Err: ErrAccountSessionsUnavailable}
	}
	return s.sessions, nil
}

// The AuthService session methods below delegate to the Identity & Access
// port so the retained consumers (school portal, parents portal, SSE,
// passkeys, cleanup CLI) keep their contract while the flows live in the
// owner module.

// Login authenticates a user and returns access and refresh tokens
func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginWithAudit(ctx, email, password, "", "", "")
}

func (s *Service) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	sessions, err := s.accountSessions("login")
	if err != nil {
		return "", "", err
	}
	return sessions.LoginWithAudit(ctx, email, password, ipAddress, userAgent, tenantSlug)
}

func (s *Service) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*LoginResult, error) {
	sessions, err := s.accountSessions("login")
	if err != nil {
		return nil, err
	}
	return sessions.LoginWithMFAGate(ctx, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie)
}

// LoginParent authenticates a parent and issues a parent-scope JWT.
func (s *Service) LoginParent(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginParentWithAudit(ctx, email, password, "", "")
}

func (s *Service) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	sessions, err := s.accountSessions("parent login")
	if err != nil {
		return "", "", err
	}
	return sessions.LoginParentWithAudit(ctx, email, password, ipAddress, userAgent)
}

func (s *Service) LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*LoginResult, error) {
	sessions, err := s.accountSessions("school login")
	if err != nil {
		return nil, err
	}
	return sessions.LoginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
}

func (s *Service) LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*LoginResult, error) {
	sessions, err := s.accountSessions("school login")
	if err != nil {
		return nil, err
	}
	return sessions.LoginSchoolAtTenantWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
}

func (s *Service) IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	sessions, err := s.accountSessions("issue tokens")
	if err != nil {
		return "", "", err
	}
	return sessions.IssueTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
}

func (s *Service) IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	sessions, err := s.accountSessions("issue school tokens")
	if err != nil {
		return "", "", err
	}
	return sessions.IssueSchoolTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
}

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	return s.RefreshTokenWithAudit(ctx, refreshToken, "", "")
}

func (s *Service) RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (string, string, error) {
	sessions, err := s.accountSessions("refresh session")
	if err != nil {
		return "", "", err
	}
	return sessions.RefreshTokenWithAudit(ctx, refreshToken, ipAddress, userAgent)
}

func (s *Service) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error {
	sessions, err := s.accountSessions("logout")
	if err != nil {
		return err
	}
	return sessions.LogoutWithAudit(ctx, refreshToken, ipAddress, userAgent)
}

func (s *Service) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	sessions, err := s.accountSessions("switch tenant")
	if err != nil {
		return "", "", err
	}
	return sessions.SwitchTenant(ctx, accountID, tenantSlug, presentedFamilyID)
}

func (s *Service) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	sessions, err := s.accountSessions("switch school")
	if err != nil {
		return "", "", err
	}
	return sessions.SwitchSchool(ctx, accountID, tenantSlug, ipAddress, userAgent)
}

func (s *Service) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error) {
	sessions, err := s.accountSessions("check school portal access")
	if err != nil {
		return false, err
	}
	return sessions.HasSchoolPortalAccess(ctx, accountID, tenantID)
}

// SessionTokenValidator verifies frontend handoffs without consuming refresh recovery state.
type SessionTokenValidator interface {
	ValidateSessionTokens(context.Context, string, string, string) (*jwt.AppClaims, error)
}

func (s *Service) ValidateSessionTokens(ctx context.Context, access, refresh, portal string) (*jwt.AppClaims, error) {
	sessions, err := s.accountSessions(opValidateToken)
	if err != nil {
		return nil, err
	}
	return sessions.ValidateSessionTokens(ctx, access, refresh, portal)
}

// VerifyAccountTenantMembership reports whether the account has a tenant
// mapping for the given school (issue #584; owner result verbatim).
func (s *Service) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	sessions, err := s.accountSessions("verify tenant membership")
	if err != nil {
		return false, err
	}
	return sessions.VerifyAccountTenantMembership(ctx, accountID, tenantID)
}

// CountExpiredTokens reports how many refresh tokens a cleanup would remove.
func (s *Service) CountExpiredTokens(ctx context.Context) (int, error) {
	sessions, err := s.accountSessions("count expired tokens")
	if err != nil {
		return 0, err
	}
	return sessions.CountExpiredTokens(ctx)
}

// CleanupExpiredTokens removes expired authentication tokens
func (s *Service) CleanupExpiredTokens(ctx context.Context) (int, error) {
	sessions, err := s.accountSessions("cleanup expired tokens")
	if err != nil {
		return 0, err
	}
	return sessions.CleanupExpiredTokens(ctx)
}

// RevokeAllTokens revokes all tokens for an account
func (s *Service) RevokeAllTokens(ctx context.Context, accountID int) error {
	return s.RevokeAllTokensWithReason(ctx, accountID, "administrative_revoke")
}

func (s *Service) RevokeAllTokensWithReason(ctx context.Context, accountID int, reason string) error {
	sessions, err := s.accountSessions("revoke all tokens")
	if err != nil {
		return err
	}
	return sessions.RevokeAllTokensWithReason(ctx, int64(accountID), reason)
}

// RevokeTokensByTenantID deletes all refresh tokens for a given tenant.
// Used during soft-delete to immediately cut off session refresh for all users of the school.
func (s *Service) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	sessions, err := s.accountSessions("revoke tokens by tenant")
	if err != nil {
		return 0, err
	}
	return sessions.RevokeTokensByTenantID(ctx, tenantID)
}

// GetActiveTokens retrieves all active tokens for an account
func (s *Service) GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error) {
	sessions, err := s.accountSessions("get active tokens")
	if err != nil {
		return nil, err
	}
	active, err := sessions.ListActiveSessions(ctx, int64(accountID))
	if err != nil {
		return nil, err
	}
	tokens := make([]*auth.Token, 0, len(active))
	for _, session := range active {
		token := &auth.Token{
			Model:     modelBase.Model{ID: session.ID, CreatedAt: session.CreatedAt},
			AccountID: int64(accountID),
			Token:     session.Token,
			Expiry:    session.Expiry,
			Mobile:    session.Mobile,
		}
		if session.Identifier != "" {
			identifier := session.Identifier
			token.Identifier = &identifier
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}
