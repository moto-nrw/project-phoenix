package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

var errAccountAuthenticationUnavailable = identityaccess.ErrAccountAuthenticationUnavailable

// attach hands every session flow the unit of work it was composed with so
// the reads outside a transaction resolve the tenant runtime too.
func (e engine) attach(ctx context.Context) context.Context {
	if e.runtime == nil {
		return ctx
	}
	return e.runtime(ctx)
}

func (e engine) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.LoginWithAudit(e.attach(ctx), email, password, ipAddress, userAgent, tenantSlug)
	return access, refresh, authenticationError(err)
}

func (e engine) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*identityaccess.LoginResult, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := e.auth.LoginWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie)
	return loginResult(result), authenticationError(err)
}

func (e engine) ConfirmAccountPassword(ctx context.Context, accountID int64, password string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.ConfirmAccountPassword(e.attach(ctx), accountID, password))
}

func (e engine) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.LoginParentWithAudit(e.attach(ctx), email, password, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.LoginResult, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := e.auth.LoginSchoolWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, trustedDeviceCookie)
	return loginResult(result), authenticationError(err)
}

func (e engine) LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*identityaccess.LoginResult, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := e.auth.LoginSchoolAtTenantWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
	return loginResult(result), authenticationError(err)
}

func (e engine) IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.IssueTokensForAuthenticatedAccount(e.attach(ctx), accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.IssueSchoolTokensForAuthenticatedAccount(e.attach(ctx), accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

// RefreshTokenWithAudit binds the recovery proof to the request context,
// where the rotation policy reads its hash.
func (e engine) RefreshTokenWithAudit(ctx context.Context, refreshToken, recoveryProof, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	ctx = e.withRecoveryProof(ctx, recoveryProof)
	access, refresh, err := e.auth.RefreshTokenWithAudit(e.attach(ctx), refreshToken, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.LogoutWithAudit(e.attach(ctx), refreshToken, ipAddress, userAgent))
}

func (e engine) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.SwitchTenant(e.attach(ctx), accountID, tenantSlug, presentedFamilyID)
	return access, refresh, authenticationError(err)
}

func (e engine) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	if e.auth == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := e.auth.SwitchSchool(e.attach(ctx), accountID, tenantSlug, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (e engine) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if e.auth == nil {
		return false, errAccountAuthenticationUnavailable
	}
	allowed, err := e.auth.HasSchoolPortalAccess(e.attach(ctx), accountID, tenantID)
	return allowed, authenticationError(err)
}

func (e engine) ValidateSessionTokens(ctx context.Context, accessToken, refreshToken, portal string) (identityaccess.SessionClaims, error) {
	if e.auth == nil {
		return identityaccess.SessionClaims{}, errAccountAuthenticationUnavailable
	}
	claims, err := e.auth.ValidateSessionTokens(e.attach(ctx), accessToken, refreshToken, portal)
	return identityaccess.SessionClaims(claims), authenticationError(err)
}

func (e engine) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if e.auth == nil {
		return false, errAccountAuthenticationUnavailable
	}
	return e.auth.VerifyAccountTenantMembership(e.attach(ctx), accountID, tenantID)
}

func (e engine) CountExpiredTokens(ctx context.Context) (int, error) {
	if e.auth == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := e.auth.CountExpiredTokens(e.attach(ctx))
	return count, authenticationError(err)
}

func (e engine) CleanupExpiredTokens(ctx context.Context) (int, error) {
	if e.auth == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := e.auth.CleanupExpiredTokens(e.attach(ctx))
	return count, authenticationError(err)
}

func (e engine) ListActiveSessions(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	sessions, err := e.auth.ListActiveSessions(e.attach(ctx), accountID)
	return accountSessions(sessions), authenticationError(err)
}

func (e engine) ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	ids, err := e.auth.ListSessionIDs(e.attach(ctx), accountID)
	return ids, authenticationError(err)
}

func (e engine) RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.RevokeAllTokensWithReason(e.attach(ctx), accountID, reason))
}

func (e engine) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	if e.auth == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := e.auth.RevokeTokensByTenantID(e.attach(ctx), tenantID)
	return count, authenticationError(err)
}

func (e engine) DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]identityaccess.AccountSession, error) {
	if e.auth == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	sessions, err := e.auth.DeleteAccountSessionsWithAudit(e.attach(ctx), accountID, reason, ipAddress, userAgent)
	return accountSessions(sessions), authenticationError(err)
}

func (e engine) QueuePushCleanup(ctx context.Context, accountID int64, sessions []identityaccess.AccountSession, reason string) {
	if e.auth == nil {
		return
	}
	values := make([]domain.AccountSession, 0, len(sessions))
	for _, session := range sessions {
		values = append(values, domain.AccountSession(session))
	}
	e.auth.QueuePushCleanup(e.attach(ctx), accountID, values, reason)
}

func (e engine) ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.ScheduleAccountWideRevoke(e.attach(ctx), accountID, reason, ipAddress, userAgent))
}

func (e engine) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	if e.auth == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(e.auth.MarkAccountWideWipeCompleted(e.attach(ctx), accountID))
}

func (e engine) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountClaims, error) {
	if e.auth == nil {
		return identityaccess.AccountClaims{}, errAccountAuthenticationUnavailable
	}
	payload, err := e.auth.LoadAccountClaims(e.attach(ctx), accountID, tenantID)
	if err != nil {
		return identityaccess.AccountClaims{}, authenticationError(err)
	}
	roles := make([]identityaccess.AccountRole, 0, len(payload.Roles))
	for _, role := range payload.Roles {
		roles = append(roles, identityaccess.AccountRole{ID: role.RoleID, Name: role.Name, IsSystem: role.IsSystem, TenantID: role.TenantID})
	}
	return identityaccess.AccountClaims{
		RoleNames: payload.RoleNames, Roles: roles, Permissions: payload.Permissions, Username: payload.Username,
		FirstName: payload.FirstName, LastName: payload.LastName, IsAdmin: payload.IsAdmin,
		TenantID: payload.TenantID, OrgID: payload.OrgID, Scope: payload.Scope,
	}, nil
}

func (e engine) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if e.auth == nil {
		return false, 0, errAccountAuthenticationUnavailable
	}
	found, tenantID, err := e.auth.FindGuardianTenant(e.attach(ctx), accountID)
	return found, tenantID, authenticationError(err)
}

func (e engine) FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if e.auth == nil {
		return false, 0, errAccountAuthenticationUnavailable
	}
	found, tenantID, err := e.auth.FindSchoolPortalTenant(e.attach(ctx), accountID)
	return found, tenantID, authenticationError(err)
}

func loginResult(value *domain.LoginResult) *identityaccess.LoginResult {
	if value == nil {
		return nil
	}
	result := identityaccess.LoginResult{
		Status: identityaccess.LoginStatus(value.Status), AccessToken: value.AccessToken, RefreshToken: value.RefreshToken,
		ChallengeToken: value.ChallengeToken, MaskedEmail: value.MaskedEmail, MFAEnrollmentRequired: value.MFAEnrollmentRequired,
		TrustedDeviceEnabled: value.TrustedDeviceEnabled, TrustedDeviceDays: value.TrustedDeviceDays,
	}
	return &result
}

var authenticationSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrInvalidCredentials, identityaccess.ErrInvalidCredentials},
	{domain.ErrAccountInactive, identityaccess.ErrAccountInactive},
	{domain.ErrAccountNotFound, identityaccess.ErrAccountNotFound},
	{domain.ErrTenantNotFound, identityaccess.ErrTenantNotFound},
	{domain.ErrTenantAccessDenied, identityaccess.ErrTenantAccessDenied},
	{domain.ErrParentMustUseParentPortal, identityaccess.ErrParentMustUseParentPortal},
	{domain.ErrAccountNoGuardianRole, identityaccess.ErrAccountNoGuardianRole},
	{domain.ErrAccountNoSchoolPortalRole, identityaccess.ErrAccountNoSchoolPortalRole},
	{domain.ErrMustUseSchoolPortal, identityaccess.ErrMustUseSchoolPortal},
	{domain.ErrDemoSessionTenantLocked, identityaccess.ErrDemoSessionTenantLocked},
	{domain.ErrInvalidToken, identityaccess.ErrInvalidToken},
	{domain.ErrTokenExpired, identityaccess.ErrTokenExpired},
	{domain.ErrTokenNotFound, identityaccess.ErrTokenNotFound},
	{domain.ErrMFAStatusUnavailable, identityaccess.ErrMFAStatusUnavailable},
	{domain.ErrTenantRequired, identityaccess.ErrTenantRequired},
	{domain.ErrAccountSessionNotFound, identityaccess.ErrAccountSessionNotFound},
	{domain.ErrAccountSessionRotated, identityaccess.ErrAccountSessionRotated},
	{domain.ErrMFAChallengeTokenInvalid, identityaccess.ErrMFAChallengeTokenInvalid},
	{domain.ErrMFACodeInvalid, identityaccess.ErrMFACodeInvalid},
	{domain.ErrMFALocked, identityaccess.ErrMFALocked},
	{domain.ErrMFARateLimited, identityaccess.ErrMFARateLimited},
	{domain.ErrMFANotEnrolled, identityaccess.ErrMFANotEnrolled},
	{domain.ErrMFAAlreadyEnrolled, identityaccess.ErrMFAAlreadyEnrolled},
	{domain.ErrMFAPermissionDenied, identityaccess.ErrMFAPermissionDenied},
	{domain.ErrMFAInvalidOverride, identityaccess.ErrMFAInvalidOverride},
	{domain.ErrMFAUnsupportedScope, identityaccess.ErrMFAUnsupportedScope},
	{domain.ErrPasskeyOriginInvalid, identityaccess.ErrPasskeyOriginInvalid},
	{domain.ErrPasskeySessionInvalid, identityaccess.ErrPasskeySessionInvalid},
	{domain.ErrPasskeyNotFound, identityaccess.ErrPasskeyNotFound},
}

// authenticationError translates a flow error to the public contract: the
// operation envelope keeps its text, and a cause that carries an internal
// sentinel gains the public one while keeping its message.
func authenticationError(err error) error {
	if err == nil {
		return nil
	}
	var operation *OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: authenticationError(operation.Err)}
	}
	for _, sentinel := range authenticationSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return MapError(err)
}

// translatedError keeps a wrapped cause's text while exposing the public
// sentinel to errors.Is.
type translatedError struct {
	text   string
	public error
	cause  error
}

func (e *translatedError) Error() string { return e.text }

func (e *translatedError) Unwrap() []error { return []error{e.public, e.cause} }
