package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

func (s *AccountAuthentication) loginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, targetTenantSlug string) (*domain.LoginResult, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	portalTenantID, err := s.schoolLoginTenant(ctx, account, targetTenantSlug, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	// The liveness gates decide the response; the roles they hydrate feed
	// the MFA verdict below. The JWT is deliberately not built from this
	// pre-transaction snapshot: the mint guard reloads it under its lock.
	metadata, err := s.loadSchoolMetadataForTenant(ctx, account, portalTenantID)
	if err != nil {
		return nil, err
	}

	mfaRequired, enrolled, err := s.schoolLoginMFAVerdict(ctx, account.ID, portalTenantID, metadata.RoleNames)
	if err != nil {
		return nil, err
	}
	if mfaRequired && !enrolled {
		return s.mfaEnrollmentResult(account, portalTenantID, domain.ScopeSchool)
	}
	trustedDeviceVerified := false
	if mfaRequired && enrolled && trustedDeviceCookie != "" && s.mfa.Configured() {
		ok, _ := s.mfa.VerifyTrustedDevice(ctx, account.ID, portalTenantID, trustedDeviceCookie)
		trustedDeviceVerified = ok
	}
	if mfaRequired && enrolled && !trustedDeviceVerified {
		return s.mfaChallengeResult(ctx, account, portalTenantID, domain.ScopeSchool, ipAddress)
	}

	return s.mintSchoolLogin(ctx, account, portalTenantID, mfaRequired, enrolled, email, ipAddress, userAgent)
}

// IssueSchoolTokensForAuthenticatedAccount mints a school-scope pair for an
// account whose identity was proven through the school MFA exchange. It
// re-validates the school-portal role at the tenant carried in the
// challenge, so a role revoked mid-challenge cannot be laundered into a
// school session.
func (s *AccountAuthentication) issueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	account, err := s.loadActiveAccountForSchoolMint(ctx, "issue school tokens", accountID)
	if err != nil {
		return "", "", err
	}
	if _, err := s.loadSchoolMetadataForTenant(ctx, account, tenantID); err != nil {
		return "", "", err
	}
	hasRole, err := s.hasSchoolPortalRoleAtTenant(ctx, accountID, tenantID)
	if err != nil {
		return "", "", err
	}
	if !hasRole {
		s.logFailedLogin(s.runtime.WithTenantID(ctx, tenantID), account.ID, ipAddress, userAgent, "No school portal role at school token issue")
		return "", "", failed("issue school tokens", identityaccess.ErrAccountNoSchoolPortalRole)
	}
	var claims *domain.AccountClaimsPayload
	session, err := s.createRefreshSessionGuarded(ctx, account, tenantID, domain.ScopeSchool, s.schoolMintGuard(account.ID, tenantID, &claims), "")
	if err != nil {
		return "", "", wrapSchoolMintError("issue school tokens", err)
	}
	if claims == nil {
		return "", "", failed("issue school tokens", errMissingSchoolClaimsPayload)
	}
	access, refresh := buildClaims(account, session, claims, account.Email)
	return s.generateAndLogTokens(s.runtime.WithTenantID(ctx, claims.TenantID), account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
}

// SwitchSchool re-authenticates a school-portal session to another school
// where the account holds a school-portal role. No MFA gate: the caller
// already holds a school session whose second factor was settled at login.
func (s *AccountAuthentication) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (_ string, _ string, err error) {
	if s == nil {
		return "", "", errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	account, err := s.loadActiveAccountForSchoolMint(ctx, "switch school", accountID)
	if err != nil {
		return "", "", err
	}
	var targetTenantID int64
	if err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		tenantID, _, resolveErr := s.resolveAccountTenantBySlug(adminCtx, accountID, tenantSlug)
		if resolveErr != nil {
			return resolveErr
		}
		targetTenantID = tenantID
		return nil
	}); err != nil {
		return "", "", err
	}
	if _, err := s.loadSchoolMetadataForTenant(ctx, account, targetTenantID); err != nil {
		return "", "", err
	}
	hasRole, err := s.hasSchoolPortalRoleAtTenant(ctx, accountID, targetTenantID)
	if err != nil {
		return "", "", err
	}
	if !hasRole {
		return "", "", failed("switch school", identityaccess.ErrAccountNoSchoolPortalRole)
	}
	var claims *domain.AccountClaimsPayload
	session, err := s.createRefreshSessionGuarded(ctx, account, targetTenantID, domain.ScopeSchool, s.schoolMintGuard(accountID, targetTenantID, &claims), "")
	if err != nil {
		return "", "", wrapSchoolMintError("switch school", err)
	}
	if claims == nil {
		return "", "", failed("switch school", errMissingSchoolClaimsPayload)
	}
	access, refresh := buildClaims(account, session, claims, account.Email)
	return s.generateAndLogTokens(s.runtime.WithTenantID(ctx, claims.TenantID), account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventTenantSwitch)
}

// loadActiveAccountForSchoolMint fetches the account a non-password school
// mint is about to issue a token for. Only a genuine miss is
// ErrAccountNotFound; a database failure propagates as a retryable error
// instead of telling a Lehrkraft their credentials were wrong.
func (s *AccountAuthentication) loadActiveAccountForSchoolMint(ctx context.Context, op string, accountID int64) (domain.LoginAccount, error) {
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, false)
	if err != nil {
		return domain.LoginAccount{}, failed(op, fmt.Errorf("look up account %d: %w", accountID, err))
	}
	if !found {
		return domain.LoginAccount{}, failed(op, domain.ErrAccountNotFound)
	}
	if !account.Active {
		return domain.LoginAccount{}, failed(op, identityaccess.ErrAccountInactive)
	}
	return account, nil
}

// errMissingSchoolClaimsPayload guards against a guard returning nil without
// filling its payload: an internal error rather than a token minted from an
// empty claims struct.
var errMissingSchoolClaimsPayload = errors.New("school claims payload missing after mint")

// errSchoolMFARequiredAtMint aborts a mint whose MFA gate has gone stale. It
// never leaves the flow: the school login answers with the challenge or
// enrollment token the login would have returned.
var errSchoolMFARequiredAtMint = errors.New("mfa became required before the school token was minted")

type schoolMintOption func(*schoolMintChecks)

type schoolMintChecks struct {
	// resolveMFAPolicy, when set, re-reads the MFA policy inside the mint
	// transaction; nil means the second factor is already settled.
	resolveMFAPolicy mfaPolicyResolver
}

type mfaPolicyResolver func(ctx context.Context) (ports.MFAPolicy, error)

func withMFAGateRecheck(resolve mfaPolicyResolver) schoolMintOption {
	return func(c *schoolMintChecks) { c.resolveMFAPolicy = resolve }
}

// freshSchoolMFAPolicy re-reads overrides and the school's mfa_mode on the
// mint transaction, past the request-scoped settings cache.
func (s *AccountAuthentication) freshSchoolMFAPolicy(accountID, tenantID int64) mfaPolicyResolver {
	return func(ctx context.Context) (ports.MFAPolicy, error) {
		return s.mfa.ResolvePolicyInTx(ctx, accountID, tenantID)
	}
}

// schoolMintGuard re-verifies the four facts a school session rests on and
// assembles the claims the JWT is built from, inside the transaction that
// persists the session: the account is active, the school exists and is
// active, the mapping is active, and the account holds a school-portal role
// there. Lock order: the MFA policy lock first (the settings writer takes an
// account foreign-key lock), then auth.accounts FOR UPDATE, then the mapping
// and role rows FOR SHARE, then the permission sources; the same order every
// revocation path walks. With a resolver set, the MFA gate is re-decided on
// the role set read under the lock; an unreadable policy fails closed.
func (s *AccountAuthentication) schoolMintGuard(accountID, tenantID int64, claims **domain.AccountClaimsPayload, opts ...schoolMintOption) mintGuard {
	var checks schoolMintChecks
	for _, opt := range opts {
		opt(&checks)
	}
	return func(ctx context.Context, _ domain.LoginAccount) error {
		if checks.resolveMFAPolicy != nil {
			if err := s.lockMFAPolicyForMint(ctx, accountID, tenantID); err != nil {
				return err
			}
		}
		account, err := s.checkSchoolMintPreconditions(ctx, accountID, tenantID)
		if err != nil {
			return err
		}
		if _, err := s.store.LockAccountPermissionSources(ctx, accountID, tenantID); err != nil {
			return fmt.Errorf("lock permission sources of account %d at school %d: %w", accountID, tenantID, err)
		}
		payload, payloadErr := s.schoolClaimsPayloadInTx(ctx, account, tenantID)
		if payloadErr != nil {
			return payloadErr
		}
		if err := s.requireSchoolMintMFAPolicy(ctx, accountID, tenantID, payload, checks); err != nil {
			return err
		}
		*claims = payload
		return nil
	}
}

// lockMFAPolicyForMint pins the school's mfa_mode for the rest of the mint
// transaction in shared mode; an unavailable lock fails the mint closed.
func (s *AccountAuthentication) lockMFAPolicyForMint(ctx context.Context, accountID, tenantID int64) error {
	if err := s.mfaLock.LockMFAPolicySharedForTenant(ctx, tenantID); err != nil {
		s.logger.Warn("mfa policy lock unavailable at school token mint; refusing to mint",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return identityaccess.ErrMFAStatusUnavailable
	}
	return nil
}

// checkSchoolMintPreconditions runs the four checks under their locks and
// hands back the locked account row.
func (s *AccountAuthentication) checkSchoolMintPreconditions(ctx context.Context, accountID, tenantID int64) (domain.LoginAccount, error) {
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, true)
	if err != nil {
		return domain.LoginAccount{}, fmt.Errorf("re-check account %d at school token mint: %w", accountID, err)
	}
	if !found {
		return domain.LoginAccount{}, domain.ErrAccountNotFound
	}
	if !account.Active {
		return domain.LoginAccount{}, identityaccess.ErrAccountInactive
	}
	school, found, err := s.schools.LockSchoolShared(ctx, tenantID)
	if err != nil {
		return domain.LoginAccount{}, fmt.Errorf("re-check school %d at school token mint: %w", tenantID, err)
	}
	if !found || !school.Live() {
		return domain.LoginAccount{}, identityaccess.ErrTenantNotFound
	}
	activeMapping, _, err := s.store.LockActiveTenantMappingShared(ctx, accountID, tenantID)
	if err != nil {
		return domain.LoginAccount{}, fmt.Errorf("re-check membership of account %d at school %d: %w", accountID, tenantID, err)
	}
	if !activeMapping {
		return domain.LoginAccount{}, identityaccess.ErrTenantAccessDenied
	}
	roles, _, err := s.store.ListAccountRolesAtTenant(ctx, accountID, tenantID, true)
	if err != nil {
		return domain.LoginAccount{}, fmt.Errorf("re-check school portal role of account %d at school %d: %w", accountID, tenantID, err)
	}
	if !domain.HasSchoolPortalRole(roles) {
		return domain.LoginAccount{}, identityaccess.ErrAccountNoSchoolPortalRole
	}
	return account, nil
}

// schoolRefreshMintGuard is schoolMintGuard as the refresh path needs it: a
// revoked portal role is reported as ErrTenantAccessDenied, the sentinel the
// refresh route already maps, and every refusal is logged with the other
// refresh rejections.
func (s *AccountAuthentication) schoolRefreshMintGuard(accountID, tenantID int64, claims **domain.AccountClaimsPayload) mintGuard {
	guard := s.schoolMintGuard(accountID, tenantID, claims)
	return func(ctx context.Context, account domain.LoginAccount) error {
		err := guard(ctx, account)
		if err == nil {
			return nil
		}
		if errors.Is(err, identityaccess.ErrAccountNoSchoolPortalRole) {
			s.logRefreshDecision("refresh_session_rejected", "school_portal_role_revoked", accountID, tenantID)
			return identityaccess.ErrTenantAccessDenied
		}
		if errors.Is(err, identityaccess.ErrTenantAccessDenied) {
			s.logRefreshDecision("refresh_session_rejected", "tenant_access_revoked", accountID, tenantID)
		}
		return err
	}
}

// wrapSchoolMintError gives a guard sentinel the operation envelope every
// school handler switches on, leaving errors that already carry one alone.
func wrapSchoolMintError(op string, err error) error {
	var wrapped *OperationError
	if errors.As(err, &wrapped) {
		return err
	}
	return failed(op, err)
}

// loadSchoolMetadataForTenant loads the tenant-scoped claims material,
// stamps the school scope and runs the liveness gates every school mint
// passes before it writes anything: the school must be alive and active and
// the mapping must be active. The gates decide the response; the guard
// repeats them inside the mint transaction.
func (s *AccountAuthentication) loadSchoolMetadataForTenant(ctx context.Context, account domain.LoginAccount, tenantID int64) (*domain.AccountClaimsPayload, error) {
	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.Scope = domain.ScopeSchool
	var (
		school        domain.School
		schoolFound   bool
		activeMapping bool
	)
	if txErr := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var lookupErr error
		school, schoolFound, lookupErr = s.schools.FindSchool(adminCtx, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("lookup school %d for school metadata: %w", tenantID, lookupErr)
		}
		activeMapping, _, lookupErr = s.store.HasActiveAccountTenant(adminCtx, account.ID, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("verify account %d membership at school %d: %w", account.ID, tenantID, lookupErr)
		}
		return nil
	}); txErr != nil {
		return nil, txErr
	}
	if !schoolFound || !school.Live() {
		return nil, failed("load school metadata", identityaccess.ErrTenantNotFound)
	}
	if !activeMapping {
		return nil, failed("load school metadata", identityaccess.ErrTenantAccessDenied)
	}
	return metadata, nil
}

// schoolClaimsPayloadInTx loads the claims payload of a school token on the
// caller's transaction and stamps the school scope; no liveness gate runs
// here because the guard settled them microseconds earlier under its locks.
func (s *AccountAuthentication) schoolClaimsPayloadInTx(ctx context.Context, account domain.LoginAccount, tenantID int64) (*domain.AccountClaimsPayload, error) {
	metadata, err := s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.Scope = domain.ScopeSchool
	return metadata, nil
}

// hasSchoolPortalRoleAtTenant reports whether the account holds a
// school-portal role at the tenant; query errors propagate so a transient
// failure never masquerades as a revocation.
func (s *AccountAuthentication) hasSchoolPortalRoleAtTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	var hasRole bool
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		roles, _, rolesErr := s.store.ListAccountRolesAtTenant(adminCtx, accountID, tenantID, false)
		if rolesErr != nil {
			return rolesErr
		}
		hasRole = domain.HasSchoolPortalRole(roles)
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("load school portal role for account %d at tenant %d: %w", accountID, tenantID, err)
	}
	return hasRole, nil
}

// findSchoolPortalTenantForAccount checks every active mapping for a
// school-portal role and returns the first matching school that is alive
// and active; a full sweep of school lookup failures is not masked as "no
// portal role".
func (s *AccountAuthentication) findSchoolPortalTenantForAccount(ctx context.Context, accountID int64) (bool, int64, error) {
	hasPortalRole := false
	var firstPortalTenantID int64
	if err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var err error
		hasPortalRole, firstPortalTenantID, err = s.firstSchoolPortalTenant(adminCtx, accountID)
		return err
	}); err != nil {
		return false, 0, err
	}
	return hasPortalRole, firstPortalTenantID, nil
}

// HasSchoolPortalAccess reports whether the account may still hold a school
// session at this school: the account is active, the school is alive and
// active, the mapping is active and the account holds a school-portal role.
// A revoked fact answers (false, nil); lookup errors propagate so a database
// blip is not a revocation.
func (s *AccountAuthentication) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (_ bool, err error) {
	if s == nil {
		return false, errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, false)
	if err != nil {
		return false, fmt.Errorf("look up account %d for school portal access: %w", accountID, err)
	}
	if !found || !account.Active {
		return false, nil
	}
	var (
		school        domain.School
		schoolFound   bool
		activeMapping bool
	)
	if txErr := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var lookupErr error
		school, schoolFound, lookupErr = s.schools.FindSchool(adminCtx, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("look up school %d for school portal access: %w", tenantID, lookupErr)
		}
		activeMapping, _, lookupErr = s.store.HasActiveAccountTenant(adminCtx, accountID, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("verify account %d membership at school %d: %w", accountID, tenantID, lookupErr)
		}
		return nil
	}); txErr != nil {
		return false, txErr
	}
	if !schoolFound || !school.Live() || !activeMapping {
		return false, nil
	}
	return s.hasSchoolPortalRoleAtTenant(ctx, accountID, tenantID)
}

// FindSchoolPortalTenant is findSchoolPortalTenantForAccount for the
// retained password-reset flow, which routes school-portal accounts to the
// school reset link.
func (s *AccountAuthentication) findSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	return s.findSchoolPortalTenantForAccount(ctx, accountID)
}
