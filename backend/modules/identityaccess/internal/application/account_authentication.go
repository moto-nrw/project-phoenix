package application

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/singleflight"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// AccountAuthentication runs tenant, parent and school login, refresh, tenant
// and school switching, logout, session validation, session cleanup and
// session revocation over the account-session service and the consumer-owned
// ports (#3251). The session writes join the transactions the runtime port
// opens; foreign facts (schools, person names, audit, push subscriptions, the
// MFA gate, the token codec) arrive through ports the composition binds.
type AccountAuthentication struct {
	sessions  *Service
	store     ports.AccountLoginStore
	schools   ports.SchoolDirectory
	persons   ports.PersonDirectory
	passwords ports.PasswordVerifier
	codec     ports.TokenCodec
	mfa       ports.MFAGate
	mfaLock   ports.MFAPolicyLock
	audit     ports.AuthAudit
	push      ports.PushSubscriptionCleanup
	runtime   ports.Runtime
	attach    func(context.Context) context.Context
	// withRecoveryProof binds a refresh-token recovery proof to the request
	// context, where the rotation policy reads its hash.
	withRecoveryProof func(ctx context.Context, proof string) context.Context
	rotation          ports.Rotation
	// caterers lets a caterer session refresh only while its account
	// belongs to a caterer (ADR 0077).
	caterers ports.CatererStore
	logger   *slog.Logger
	// refreshSF deduplicates concurrent refresh calls for the same refresh
	// token and recovery proof.
	refreshSF singleflight.Group
}

// AccountAuthenticationDependencies are the ports the flows consume.
type AccountAuthenticationDependencies struct {
	Store     ports.AccountLoginStore
	Schools   ports.SchoolDirectory
	Persons   ports.PersonDirectory
	Passwords ports.PasswordVerifier
	Codec     ports.TokenCodec
	MFA       ports.MFAGate
	MFALock   ports.MFAPolicyLock
	Audit     ports.AuthAudit
	Push      ports.PushSubscriptionCleanup
	Runtime   ports.Runtime
	// Attach binds the composed unit of work ahead of every public flow.
	Attach func(context.Context) context.Context
	// WithRecoveryProof binds a refresh-token recovery proof to the request
	// context, where the rotation policy reads its hash.
	WithRecoveryProof func(ctx context.Context, proof string) context.Context
	Rotation          ports.Rotation
	// Caterers is optional: without it no caterer session refreshes.
	Caterers ports.CatererStore
	Logger   *slog.Logger
}

// NewAccountAuthentication composes the flows over the session service that
// owns auth.tokens.
func NewAccountAuthentication(sessions *Service, deps AccountAuthenticationDependencies) (*AccountAuthentication, error) {
	switch {
	case sessions == nil:
		return nil, fmt.Errorf("identity access account authentication: session service is required")
	case deps.Store == nil:
		return nil, fmt.Errorf("identity access account authentication: account store is required")
	case deps.Schools == nil:
		return nil, fmt.Errorf("identity access account authentication: school directory is required")
	case deps.Persons == nil:
		return nil, fmt.Errorf("identity access account authentication: person directory is required")
	case deps.Passwords == nil:
		return nil, fmt.Errorf("identity access account authentication: password verifier is required")
	case deps.Codec == nil:
		return nil, fmt.Errorf("identity access account authentication: token codec is required")
	case deps.MFA == nil:
		return nil, fmt.Errorf("identity access account authentication: mfa gate is required")
	case deps.MFALock == nil:
		return nil, fmt.Errorf("identity access account authentication: mfa policy lock is required")
	case deps.Audit == nil:
		return nil, fmt.Errorf("identity access account authentication: audit recorder is required")
	case deps.Push == nil:
		return nil, fmt.Errorf("identity access account authentication: push subscription cleanup is required")
	case deps.Runtime == nil:
		return nil, fmt.Errorf("identity access account authentication: tenant runtime is required")
	case deps.Rotation == nil:
		return nil, fmt.Errorf("identity access account authentication: rotation policy is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountAuthentication{
		sessions: sessions, store: deps.Store, schools: deps.Schools, persons: deps.Persons, passwords: deps.Passwords,
		codec: deps.Codec, mfa: deps.MFA, mfaLock: deps.MFALock, audit: deps.Audit, push: deps.Push,
		runtime: deps.Runtime, attach: deps.Attach, withRecoveryProof: deps.WithRecoveryProof, rotation: deps.Rotation, caterers: deps.Caterers, logger: logger,
	}, nil
}

// OperationError wraps a flow failure with the operation that failed, the way the
// retained auth service reported it; the sentinel stays reachable through
// errors.Is.
type OperationError struct {
	Op  string
	Err error
}

func (e *OperationError) Error() string {
	if e.Err == nil {
		return "auth error during " + e.Op
	}
	return "auth error during " + e.Op + ": " + e.Err.Error()
}

func (e *OperationError) Unwrap() error { return e.Err }

func failed(op string, err error) error { return &OperationError{Op: op, Err: err} }

// mintGuard runs inside the token-persistence transaction, immediately before
// the session row is written. It is the only place where an authorization
// fact can be re-checked atomically with the write it authorizes; the login
// flows verify everything else in already-committed transactions that may be
// stale by the time the token is minted. A guard receives the administrative
// transaction in ctx and must read through the store on it, never open a
// nested one. Returning an error aborts the mint; the error surfaces verbatim
// and is never retried. A guard is also where anything the JWT is built from
// belongs: assembling claims after the transaction committed means a failure
// there has already rotated the caller's refresh token with no successor to
// hand back.
type mintGuard func(ctx context.Context, account domain.LoginAccount) error

// mintGuardError marks an error as coming from a mintGuard so the retry loop
// passes the caller's sentinel through instead of burying it under the
// generic transaction wrapper.
type mintGuardError struct{ err error }

func (e *mintGuardError) Error() string { return e.err.Error() }
func (e *mintGuardError) Unwrap() error { return e.err }

type SchoolDirectory struct{ identityPorts.SchoolDirectory }

func (d SchoolDirectory) FindSchool(ctx context.Context, id int64) (domain.School, bool, error) {
	school, found, err := d.SchoolDirectory.FindSchool(ctx, id)
	return domain.School(school), found, err
}

func (d SchoolDirectory) FindSchoolBySubdomain(ctx context.Context, subdomain string) (domain.School, bool, error) {
	school, found, err := d.SchoolDirectory.FindSchoolBySubdomain(ctx, subdomain)
	return domain.School(school), found, err
}

func (d SchoolDirectory) LockSchoolShared(ctx context.Context, id int64) (domain.School, bool, error) {
	school, found, err := d.SchoolDirectory.LockSchoolShared(ctx, id)
	return domain.School(school), found, err
}

func (d SchoolDirectory) ListActiveSchoolsByID(ctx context.Context, schoolIDs []int64) ([]domain.School, error) {
	schools, err := d.SchoolDirectory.ListActiveSchoolsByID(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	result := make([]domain.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, domain.School(school))
	}
	return result, nil
}

type TokenCodec struct{ identityPorts.TokenCodec }

func (c TokenCodec) IssueTokenPair(access domain.SessionClaims, refresh domain.RefreshClaims) (string, string, error) {
	return c.TokenCodec.IssueTokenPair(identityaccess.SessionClaims(access), identityaccess.RefreshClaims(refresh))
}

func (c TokenCodec) ParseAccessToken(token string) (domain.SessionClaims, error) {
	claims, err := c.TokenCodec.ParseAccessToken(token)
	return domain.SessionClaims(claims), err
}

func (c TokenCodec) ParseRefreshToken(token string) (domain.RefreshClaims, error) {
	claims, err := c.TokenCodec.ParseRefreshToken(token)
	return domain.RefreshClaims(claims), err
}

type AuthAudit struct{ Source identityPorts.AuthAudit }

func (a AuthAudit) RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error {
	public := identityaccess.AuthEvent{
		AccountID: event.AccountID, TenantID: event.TenantID, Type: event.Type, Success: event.Success,
		IPAddress: event.IPAddress, UserAgent: event.UserAgent, ErrorMessage: event.ErrorMessage,
	}
	if event.TenantAccess != nil {
		evidence := identityaccess.TenantAccessEvidence(*event.TenantAccess)
		public.TenantAccess = &evidence
	}
	if event.RevokedSessions != nil {
		evidence := identityaccess.RevokedSessionsEvidence(*event.RevokedSessions)
		public.RevokedSessions = &evidence
	}
	if event.PendingWipe != nil {
		evidence := identityaccess.PendingWipeEvidence(*event.PendingWipe)
		public.PendingWipe = &evidence
	}
	if event.MFA != nil {
		evidence := identityaccess.MFAEvidence{
			ChallengeID: event.MFA.ChallengeID, DeviceID: event.MFA.DeviceID, LockedUntil: event.MFA.LockedUntil,
		}
		if event.MFA.Admin != nil {
			admin := identityaccess.MFAAdminEvidence(*event.MFA.Admin)
			evidence.Admin = &admin
		}
		public.MFA = &evidence
	}
	if event.CompletedWipe != nil {
		evidence := identityaccess.CompletedWipeEvidence(*event.CompletedWipe)
		public.CompletedWipe = &evidence
	}
	return a.Source.RecordAuthEvent(ctx, public)
}

func (a AuthAudit) ListPendingAccountWideWipes(ctx context.Context) ([]domain.PendingAccountWideWipe, error) {
	pending, err := a.Source.ListPendingAccountWideWipes(ctx)
	return pendingWipes(pending), err
}

func (a AuthAudit) ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]domain.PendingAccountWideWipe, error) {
	pending, err := a.Source.ClaimPendingAccountWideWipes(ctx, accountID)
	return pendingWipes(pending), err
}

func pendingWipes(values []identityaccess.PendingAccountWideWipe) []domain.PendingAccountWideWipe {
	if values == nil {
		return nil
	}
	result := make([]domain.PendingAccountWideWipe, 0, len(values))
	for _, value := range values {
		result = append(result, domain.PendingAccountWideWipe(value))
	}
	return result
}

func (s *AccountAuthentication) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	if s == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := s.loginWithAudit(bindRuntime(s.attach, ctx), email, password, ipAddress, userAgent, tenantSlug)
	return access, refresh, authenticationError(err)
}

func (s *AccountAuthentication) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*identityaccess.LoginResult, error) {
	if s == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := s.loginWithMFAGate(bindRuntime(s.attach, ctx), email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie)
	return loginResult(result), authenticationError(err)
}

func (s *AccountAuthentication) ConfirmAccountPassword(ctx context.Context, accountID int64, password string) error {
	if s == nil {
		return errAccountAuthenticationUnavailable
	}
	return authenticationError(s.confirmAccountPassword(bindRuntime(s.attach, ctx), accountID, password))
}

func (s *AccountAuthentication) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	if s == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := s.loginParentWithAudit(bindRuntime(s.attach, ctx), email, password, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (s *AccountAuthentication) IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	if s == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := s.issueSchoolTokensForAuthenticatedAccount(bindRuntime(s.attach, ctx), accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (s *AccountAuthentication) RefreshTokenWithAudit(ctx context.Context, refreshToken, recoveryProof, ipAddress, userAgent string) (string, string, error) {
	if s == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	if s.withRecoveryProof != nil {
		ctx = s.withRecoveryProof(ctx, recoveryProof)
	}
	access, refresh, err := s.refreshTokenWithAudit(bindRuntime(s.attach, ctx), refreshToken, ipAddress, userAgent)
	return access, refresh, authenticationError(err)
}

func (s *AccountAuthentication) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	if s == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	access, refresh, err := s.switchTenant(bindRuntime(s.attach, ctx), accountID, tenantSlug, presentedFamilyID)
	return access, refresh, authenticationError(err)
}

func (s *AccountAuthentication) CleanupExpiredTokens(ctx context.Context) (int, error) {
	if s == nil {
		return 0, errAccountAuthenticationUnavailable
	}
	count, err := s.cleanupExpiredTokens(bindRuntime(s.attach, ctx))
	return count, authenticationError(err)
}

func (s *AccountAuthentication) LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.LoginResult, error) {
	if s == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := s.loginSchoolWithMFAGate(bindRuntime(s.attach, ctx), email, password, ipAddress, userAgent, trustedDeviceCookie, "")
	return loginResult(result), authenticationError(err)
}

func (s *AccountAuthentication) LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*identityaccess.LoginResult, error) {
	if s == nil {
		return nil, errAccountAuthenticationUnavailable
	}
	result, err := s.loginSchoolWithMFAGate(bindRuntime(s.attach, ctx), email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
	return loginResult(result), authenticationError(err)
}

func (s *AccountAuthentication) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountClaims, error) {
	if s == nil {
		return identityaccess.AccountClaims{}, errAccountAuthenticationUnavailable
	}
	payload, err := s.loadAccountClaims(bindRuntime(s.attach, ctx), accountID, tenantID)
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

func (s *AccountAuthentication) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if s == nil {
		return false, 0, errAccountAuthenticationUnavailable
	}
	found, tenantID, err := s.findGuardianTenant(bindRuntime(s.attach, ctx), accountID)
	return found, tenantID, authenticationError(err)
}

func (s *AccountAuthentication) FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	if s == nil {
		return false, 0, errAccountAuthenticationUnavailable
	}
	found, tenantID, err := s.findSchoolPortalTenant(bindRuntime(s.attach, ctx), accountID)
	return found, tenantID, authenticationError(err)
}

// BindRuntime attaches the unit of work the session flows were composed with.
func (s *AccountAuthentication) BindRuntime(ctx context.Context) context.Context {
	return bindRuntime(s.attach, ctx)
}
