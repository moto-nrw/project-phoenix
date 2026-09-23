package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// LoginWithAudit authenticates a tenant-portal user and returns an access and
// refresh token pair. tenantSlug is optional: when non-empty the account is
// resolved to the matching school; when empty the first active mapping is
// used.
func (s *AccountAuthentication) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}
	// The tenant is resolved from the database first so the refresh session
	// gets the correct tenant on creation; login is a public route and the
	// context names no tenant.
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return "", "", err
	}
	// Tenant-portal policy: a guardian-only account at this school must use
	// the parents portal, a school-portal-only account moto schule (#2207).
	// Dual-role accounts pass through unchanged.
	if domain.IsGuardianOnly(metadata.RoleNames) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant login")
		return "", "", failed("login", domain.ErrParentMustUseParentPortal)
	}
	if domain.IsSchoolPortalOnly(metadata.Roles) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "School-portal-only account at tenant login")
		return "", "", failed("login", domain.ErrMustUseSchoolPortal)
	}
	session, err := s.createRefreshSession(ctx, account, metadata.TenantID, metadata.Scope)
	if err != nil {
		return "", "", err
	}
	access, refresh := buildClaims(account, session, metadata, email)
	return s.generateAndLogTokens(ctx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
}

// LoginWithMFAGate is the MFA-aware sibling of LoginWithAudit: after a
// successful credential check it consults the MFA gate and returns either a
// token pair, a challenge token or an enrollment token. trustedDeviceCookie
// may be empty; when set and verifiable, MFA is skipped.
func (s *AccountAuthentication) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*domain.LoginResult, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return nil, err
	}
	if domain.IsGuardianOnly(metadata.RoleNames) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant login")
		return nil, failed("login", domain.ErrParentMustUseParentPortal)
	}
	if domain.IsSchoolPortalOnly(metadata.Roles) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "School-portal-only account at tenant login")
		return nil, failed("login", domain.ErrMustUseSchoolPortal)
	}

	mfaRequired, enrolled, err := s.resolveTenantMFAGate(ctx, account, metadata)
	if err != nil {
		return nil, err
	}
	if mfaRequired && !enrolled {
		return s.mfaEnrollmentResult(account, metadata.TenantID, domain.ScopeTenant)
	}
	// Issue a challenge exactly when MFA is required, the factor is enrolled
	// and no valid trusted-device cookie is presented.
	if mfaRequired && enrolled && !s.trustedDeviceVerified(ctx, account.ID, metadata.TenantID, trustedDeviceCookie) {
		return s.mfaChallengeResult(ctx, account, metadata.TenantID, domain.ScopeTenant, ipAddress)
	}

	session, err := s.createRefreshSession(ctx, account, metadata.TenantID, metadata.Scope)
	if err != nil {
		return nil, err
	}
	access, refresh := buildClaims(account, session, metadata, email)
	accessToken, refreshToken, err := s.generateAndLogTokens(ctx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
	if err != nil {
		return nil, err
	}
	return &domain.LoginResult{Status: domain.LoginStatusAuthenticated, AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// resolveTenantMFAGate is the fail-closed MFA inquiry for the tenant login:
// an unconfigured gate is "not required / not enrolled", an infrastructure
// error refuses the login instead of dropping the second factor.
func (s *AccountAuthentication) resolveTenantMFAGate(ctx context.Context, account domain.LoginAccount, metadata *domain.AccountClaimsPayload) (required, enrolled bool, err error) {
	if !s.mfa.Configured() {
		return false, false, nil
	}
	required, err = s.mfa.IsRequired(ctx, account.ID, account.Email, metadata.RoleNames, metadata.TenantID)
	if err != nil {
		return false, false, failed("check mfa required", domain.ErrMFAStatusUnavailable)
	}
	enrolled, err = s.mfa.HasEnrollment(ctx, account.ID)
	if err != nil {
		return false, false, failed("check mfa enrollment", domain.ErrMFAStatusUnavailable)
	}
	return required, enrolled, nil
}

// mfaEnrollmentResult issues an enrollment JWT only the portal's
// /mfa/enroll/* surface accepts.
func (s *AccountAuthentication) mfaEnrollmentResult(account domain.LoginAccount, tenantID int64, scope string) (*domain.LoginResult, error) {
	enrollmentScope := "tenant"
	if scope == domain.ScopeSchool {
		enrollmentScope = "school"
	}
	token, err := s.codec.IssueMFAEnrollmentToken(account.ID, tenantID, enrollmentScope, domain.MFAEnrollmentTokenTTL)
	if err != nil {
		return nil, failed("issue mfa enrollment token", err)
	}
	return &domain.LoginResult{
		Status:                domain.LoginStatusMFAEnrollmentRequired,
		AccessToken:           token,
		MaskedEmail:           domain.MaskEmail(account.Email),
		MFAEnrollmentRequired: true,
	}, nil
}

func (s *AccountAuthentication) trustedDeviceVerified(ctx context.Context, accountID, tenantID int64, cookie string) bool {
	if cookie == "" || !s.mfa.Configured() {
		return false
	}
	ok, _ := s.mfa.VerifyTrustedDevice(ctx, accountID, tenantID, cookie)
	return ok
}

// mfaChallengeResult is the "MFA required, factor enrolled" response: an
// email code plus a challenge token bound to the portal scope.
func (s *AccountAuthentication) mfaChallengeResult(ctx context.Context, account domain.LoginAccount, tenantID int64, scope, ipAddress string) (*domain.LoginResult, error) {
	challengeScope := "tenant"
	if scope == domain.ScopeSchool {
		challengeScope = "school"
	}
	challenge, err := s.mfa.StartChallenge(ctx, account.ID, tenantID, challengeScope, ipAddress)
	if err != nil {
		return nil, failed("start mfa challenge", err)
	}
	return &domain.LoginResult{
		Status:               domain.LoginStatusMFARequired,
		ChallengeToken:       challenge,
		MaskedEmail:          domain.MaskEmail(account.Email),
		TrustedDeviceEnabled: s.mfa.IsTrustedDeviceEnabled(ctx, tenantID),
		TrustedDeviceDays:    s.mfa.TrustedDeviceDays(ctx, tenantID),
	}, nil
}

// IssueTokensForAuthenticatedAccount mints a token pair for an account whose
// identity was proven via a non-password channel (MFA code, recovery code or
// passkey). It skips the credential check but otherwise reuses the login
// pipeline, so the session is indistinguishable from a password login.
// tenantID is the school carried in the MFA challenge.
func (s *AccountAuthentication) IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	account, err := s.authenticatedAccount(ctx, "issue tokens", accountID)
	if err != nil {
		return "", "", err
	}
	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return "", "", err
	}
	if domain.IsGuardianOnly(metadata.RoleNames) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant token issue")
		return "", "", failed("issue tokens", domain.ErrParentMustUseParentPortal)
	}
	if domain.IsSchoolPortalOnly(metadata.Roles) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "School-portal-only account at tenant token issue")
		return "", "", failed("issue tokens", domain.ErrMustUseSchoolPortal)
	}
	session, err := s.createRefreshSession(ctx, account, metadata.TenantID, metadata.Scope)
	if err != nil {
		return "", "", err
	}
	access, refresh := buildClaims(account, session, metadata, account.Email)
	return s.generateAndLogTokens(ctx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
}

// authenticatedAccount loads the active account whose identity a
// non-password channel proved. A missing account and a failed lookup both
// read as not found, as they always did at the token issue sites.
func (s *AccountAuthentication) authenticatedAccount(ctx context.Context, operation string, accountID int64) (domain.LoginAccount, error) {
	account, found, _, err := s.store.FindLoginAccount(ctx, accountID, false)
	if err != nil || !found {
		return domain.LoginAccount{}, failed(operation, domain.ErrAccountNotFound)
	}
	if !account.Active {
		return domain.LoginAccount{}, failed(operation, domain.ErrAccountInactive)
	}
	return account, nil
}

// validateLoginCredentials checks the address, the password and the account
// state. Failures are audited when the caller passed an IP address.
//
// The password is verified before the account state is judged, for the same
// reason the wrong-portal refusals sit behind it: "this account is
// deactivated" is a fact about a foreign account, and answering it to a
// caller who did not present the credential would let anyone probe which
// addresses exist and are disabled. Behind a correct password it leaks
// nothing and is the only way the owner learns why the portal refuses them
// (#3376).
func (s *AccountAuthentication) validateLoginCredentials(ctx context.Context, email, password, ipAddress, userAgent string) (domain.LoginAccount, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	account, found, _, err := s.store.FindLoginAccountByEmail(ctx, email)
	if err != nil || !found {
		s.logFailedLogin(ctx, 0, ipAddress, userAgent, "Account not found")
		return domain.LoginAccount{}, failed("login", domain.ErrAccountNotFound)
	}
	if err := s.verifyPassword(account, password); err != nil {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Invalid password")
		return domain.LoginAccount{}, err
	}
	if !account.Active {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Account inactive")
		return domain.LoginAccount{}, failed("login", domain.ErrAccountInactive)
	}
	return account, nil
}

func (s *AccountAuthentication) verifyPassword(account domain.LoginAccount, password string) error {
	if account.PasswordHash == "" {
		return failed("login", domain.ErrInvalidCredentials)
	}
	valid, err := s.passwords.VerifyPassword(password, account.PasswordHash)
	if err != nil || !valid {
		return failed("login", domain.ErrInvalidCredentials)
	}
	return nil
}

// createRefreshSession mints a refresh session with retry logic for
// concurrent logins that collide on the token family.
func (s *AccountAuthentication) createRefreshSession(ctx context.Context, account domain.LoginAccount, tenantID int64, scope string) (domain.AccountSession, error) {
	return s.createRefreshSessionGuarded(ctx, account, tenantID, scope, nil, "")
}

// createRefreshSessionGuarded is createRefreshSession with an authorization
// re-check inside the persistence transaction. A guard failure is terminal:
// the retry loop exists for token-family collisions only. retireFamilyID
// names the caller's current family, retired in the same transaction before
// the session cap runs so the new session replaces it (tenant switch, #2952).
func (s *AccountAuthentication) createRefreshSessionGuarded(ctx context.Context, account domain.LoginAccount, tenantID int64, scope string, guard mintGuard, retireFamilyID string) (domain.AccountSession, error) {
	session := s.newRefreshSession(account.ID, scope)
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		stored, err := s.persistSessionInTransaction(ctx, account, session, tenantID, guard, retireFamilyID)
		if err == nil {
			return stored, nil
		}
		var guardErr *mintGuardError
		if errors.As(err, &guardErr) {
			return domain.AccountSession{}, guardErr.err
		}
		if !isTokenFamilyConflict(err) {
			return domain.AccountSession{}, failed("login transaction", err)
		}
		session.FamilyID = uuid.Must(uuid.NewV4()).String()
		s.logger.Warn("login race condition detected, retrying",
			slog.Int64("account_id", account.ID),
			slog.Int("attempt", attempt+1),
			slog.Int("max_retries", maxRetries))
	}
	return domain.AccountSession{}, failed("login transaction", fmt.Errorf("max retries exceeded"))
}

func (s *AccountAuthentication) newRefreshSession(accountID int64, scope string) domain.AccountSession {
	identifier := domain.RefreshTokenIdentifier
	now := time.Now()
	return domain.AccountSession{
		Token:       uuid.Must(uuid.NewV4()).String(),
		AccountID:   accountID,
		Expiry:      now.Add(s.codec.RefreshExpiry()),
		Mobile:      false,
		Identifier:  &identifier,
		FamilyID:    uuid.Must(uuid.NewV4()).String(),
		Generation:  0,
		PortalScope: domain.PersistedPortalScope(scope),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// persistSessionInTransaction writes the session in one administrative
// transaction: the login route has no tenant context and auth.tokens is
// RLS-guarded. The guard re-validates the caller's authorization first;
// the last-login write then takes the account row lock so concurrent
// issuers enforce the session cap serially instead of from identical
// snapshots; the replaced family is retired before the cap so it is the cap's
// first candidate.
func (s *AccountAuthentication) persistSessionInTransaction(ctx context.Context, account domain.LoginAccount, session domain.AccountSession, tenantID int64, guard mintGuard, retireFamilyID string) (domain.AccountSession, error) {
	var stored domain.AccountSession
	err := s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if guard != nil {
			if err := guard(txCtx, account); err != nil {
				return &mintGuardError{err: err}
			}
		}
		loginTime := time.Now()
		if _, err := s.store.RecordAccountLogin(txCtx, account.ID, loginTime); err != nil {
			return fmt.Errorf("update last login before token issuance: %w", err)
		}
		if retireFamilyID != "" {
			if err := s.sessions.RetireAccountSessionFamily(txCtx, account.ID, retireFamilyID, loginTime.Add(s.rotation.RecoveryGrace())); err != nil {
				return fmt.Errorf("retire replaced refresh-token family: %w", err)
			}
		}
		// The tenant comes from the database resolution, never from the
		// context: login is a public route.
		session.TenantID = tenantID
		created, err := s.sessions.CreateAccountSession(txCtx, session)
		if err != nil {
			return err
		}
		stored = created
		if err := s.sessions.DeleteExpiredRotatedAccountSessions(txCtx, account.ID, time.Now()); err != nil {
			s.logger.Warn("failed to clean up refresh-token handoffs",
				slog.Int64("account_id", account.ID),
				slog.Any("error", err))
		}
		return s.capSessionsUnlessDemo(txCtx, account.ID, created.PortalScope)
	})
	if err != nil {
		return domain.AccountSession{}, err
	}
	return stored, nil
}

// capSessionsUnlessDemo enforces the session cap for every account except one
// a demo access signed in: all visitors of the public demo share that
// account, so the cap would end the demo of an earlier visitor (#3462).
func (s *AccountAuthentication) capSessionsUnlessDemo(ctx context.Context, accountID int64, portalScope string) error {
	demo, err := s.store.DemoAccountExists(ctx, accountID)
	if err != nil {
		return fmt.Errorf("check demo account before session cap: %w", err)
	}
	if demo {
		return nil
	}
	evicted, err := s.enforcePortalSessionCap(ctx, accountID, portalScope)
	if err != nil {
		return err
	}
	s.queuePushCleanup(ctx, accountID, evicted, "session_cap")
	return nil
}

// enforcePortalSessionCap keeps at most five active sessions in this portal
// group; other portals keep their own sessions.
func (s *AccountAuthentication) enforcePortalSessionCap(ctx context.Context, accountID int64, portalScope string) ([]domain.AccountSession, error) {
	evicted, err := s.sessions.EnforceAccountSessionCap(ctx, accountID, portalScope, domain.MaxActiveSessionsPerPortal)
	if err != nil {
		return nil, fmt.Errorf("enforce active session cap: %w", err)
	}
	if err := s.auditRevokedSessions(ctx, evicted, "session_cap", "", ""); err != nil {
		return nil, err
	}
	return evicted, nil
}

func isTokenFamilyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uk_tokens_family_generation")
}

// loadAccountMetadata resolves the tenant first, then the roles, permissions
// and person names scoped to it, inside one administrative transaction: the
// login, switch and refresh flows run before a tenant transaction exists and
// the role and permission tables are RLS-guarded.
func (s *AccountAuthentication) loadAccountMetadata(ctx context.Context, account domain.LoginAccount, tenantSlug string) (*domain.AccountClaimsPayload, error) {
	var result *domain.AccountClaimsPayload
	err := s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		tenantID, orgID, err := s.resolveAccountTenant(txCtx, account.ID, tenantSlug)
		if err != nil {
			return err
		}
		payload, err := s.loadClaimsPayloadInTx(txCtx, account, tenantID, orgID)
		if err != nil {
			return err
		}
		result = payload
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// loadAccountMetadataForTenant loads the claims material for a known
// tenant. The refresh and MFA exchange paths use it: the tenant is already
// validated and must be preserved exactly.
func (s *AccountAuthentication) loadAccountMetadataForTenant(ctx context.Context, account domain.LoginAccount, tenantID int64) (*domain.AccountClaimsPayload, error) {
	var result *domain.AccountClaimsPayload
	err := s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		payload, err := s.loadAccountMetadataForTenantInTx(txCtx, account, tenantID)
		if err != nil {
			return err
		}
		result = payload
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// loadAccountMetadataForTenantInTx is loadAccountMetadataForTenant for
// callers that already hold the administrative transaction: the school's
// organization is looked up on it, then the claims material.
func (s *AccountAuthentication) loadAccountMetadataForTenantInTx(ctx context.Context, account domain.LoginAccount, tenantID int64) (*domain.AccountClaimsPayload, error) {
	var orgID int64
	if tenantID > 0 {
		school, found, err := s.schools.FindSchool(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("lookup school for tenant %d: %w", tenantID, err)
		}
		// A school hard-deleted between issuance and this load answers with
		// re-login, not retry.
		if !found || school.Deleted {
			return nil, failed("load metadata for tenant", domain.ErrTenantNotFound)
		}
		orgID = school.OrganizationID
	}
	return s.loadClaimsPayloadInTx(ctx, account, tenantID, orgID)
}

// loadClaimsPayloadInTx assembles roles, permissions, names and the admin
// flag for the tenant on the caller's transaction. Query failures propagate:
// an empty role set is not a harmless degradation, because the MFA gate
// evaluates required_admins against exactly these roles.
func (s *AccountAuthentication) loadClaimsPayloadInTx(ctx context.Context, account domain.LoginAccount, tenantID, orgID int64) (*domain.AccountClaimsPayload, error) {
	roles, err := s.loadAccountRolesForTenant(ctx, account.ID, tenantID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.loadAccountPermissionsForTenant(ctx, account.ID, tenantID)
	if err != nil {
		return nil, err
	}
	roleNames := domain.RoleNames(roles)
	firstName, lastName, err := s.loadPersonNamesForTenant(ctx, account.ID, tenantID)
	if err != nil {
		return nil, err
	}
	return &domain.AccountClaimsPayload{
		RoleNames:   roleNames,
		Roles:       roles,
		Permissions: permissions,
		Username:    account.Username,
		FirstName:   firstName,
		LastName:    lastName,
		IsAdmin:     domain.IsAdmin(roleNames),
		TenantID:    tenantID,
		OrgID:       orgID,
	}, nil
}

func (s *AccountAuthentication) loadAccountRolesForTenant(ctx context.Context, accountID, tenantID int64) ([]domain.RoleAssignment, error) {
	roles, _, err := s.store.ListAccountRolesAtTenant(ctx, accountID, tenantID, false)
	if err != nil {
		s.logger.Warn("failed to load tenant-scoped roles; refusing login",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err))
		return nil, fmt.Errorf("load tenant-scoped roles for account %d at tenant %d: %w", accountID, tenantID, err)
	}
	return roles, nil
}

func (s *AccountAuthentication) loadAccountPermissionsForTenant(ctx context.Context, accountID, tenantID int64) ([]string, error) {
	permissions, _, err := s.store.ListAccountPermissionsAtTenant(ctx, accountID, tenantID)
	if err != nil {
		s.logger.Warn("failed to load tenant-scoped permissions; refusing login",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err))
		return nil, fmt.Errorf("load tenant-scoped permissions for account %d at tenant %d: %w", accountID, tenantID, err)
	}
	if permissions == nil {
		permissions = []string{}
	}
	return permissions, nil
}

// loadPersonNamesForTenant resolves the person names that belong in a token
// minted for tenantID, instead of whichever tenant sits in the ambient
// context (the switch paths run inside the request of the source school).
// Without a person row at the target, only the schools the account is
// actively mapped to are consulted, in ascending tenant order, and the name
// is used only when those schools agree on it.
func (s *AccountAuthentication) loadPersonNamesForTenant(ctx context.Context, accountID, tenantID int64) (string, string, error) {
	if tenantID > 0 {
		firstName, lastName, err := s.loadPersonNames(s.runtime.WithTenantID(ctx, tenantID), accountID)
		if err != nil {
			return "", "", fmt.Errorf("load person names for account %d at tenant %d: %w", accountID, tenantID, err)
		}
		if firstName != "" || lastName != "" {
			return firstName, lastName, nil
		}
	}
	return s.loadPersonNamesFromMappedTenants(ctx, accountID, tenantID)
}

func (s *AccountAuthentication) loadPersonNames(ctx context.Context, accountID int64) (string, string, error) {
	firstName, lastName, found, err := s.persons.FindPersonName(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", nil
	}
	return firstName, lastName, nil
}

func (s *AccountAuthentication) loadPersonNamesFromMappedTenants(ctx context.Context, accountID, excludeTenantID int64) (string, string, error) {
	tenantIDs, _, err := s.store.ListActiveTenantIDs(ctx, accountID)
	if err != nil {
		return "", "", fmt.Errorf("list active tenants of account %d for person lookup: %w", accountID, err)
	}
	candidates := make([]int64, 0, len(tenantIDs))
	for _, mappedTenantID := range tenantIDs {
		if mappedTenantID != excludeTenantID && mappedTenantID > 0 {
			candidates = append(candidates, mappedTenantID)
		}
	}
	slices.Sort(candidates)
	var firstName, lastName string
	for _, mappedTenantID := range candidates {
		candidateFirst, candidateLast, err := s.loadPersonNames(s.runtime.WithTenantID(ctx, mappedTenantID), accountID)
		if err != nil {
			return "", "", fmt.Errorf("load person names for account %d at tenant %d: %w", accountID, mappedTenantID, err)
		}
		if candidateFirst == "" && candidateLast == "" {
			continue
		}
		if firstName == "" && lastName == "" {
			firstName, lastName = candidateFirst, candidateLast
			continue
		}
		if candidateFirst != firstName || candidateLast != lastName {
			s.logger.Warn("person name ambiguous across schools; minting token without a name",
				slog.Int64("account_id", accountID),
				slog.Int64("tenant_id", excludeTenantID))
			return "", "", nil
		}
	}
	return firstName, lastName, nil
}

// resolveAccountTenant resolves the school and organization for an account:
// by subdomain when tenantSlug is set, else the first active mapping.
func (s *AccountAuthentication) resolveAccountTenant(ctx context.Context, accountID int64, tenantSlug string) (int64, int64, error) {
	if tenantSlug != "" {
		return s.resolveAccountTenantBySlug(ctx, accountID, tenantSlug)
	}
	return s.resolveAccountTenantDefault(ctx, accountID)
}

// resolveAccountTenantBySlug resolves the school by subdomain and verifies
// access: an org-scope caller reaches every school of its organization, a
// tenant-scope caller needs an active mapping.
func (s *AccountAuthentication) resolveAccountTenantBySlug(ctx context.Context, accountID int64, tenantSlug string) (int64, int64, error) {
	school, found, err := s.schools.FindSchoolBySubdomain(ctx, tenantSlug)
	if err != nil {
		s.logger.Warn("tenant slug lookup failed",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Any("error", err))
		return 0, 0, failed("resolve tenant", err)
	}
	if !found {
		s.logger.Warn("tenant slug not found",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug))
		return 0, 0, failed("resolve tenant", domain.ErrTenantNotFound)
	}
	if school.Deleted {
		s.logger.Warn("tenant is soft-deleted",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Int64("tenant_id", school.ID))
		return 0, 0, failed("resolve tenant", domain.ErrTenantNotFound)
	}
	if !school.Active {
		s.logger.Warn("tenant is inactive",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Int64("tenant_id", school.ID))
		return 0, 0, failed("resolve tenant", domain.ErrTenantNotFound)
	}
	callerScope := s.runtime.Scope(ctx)
	callerOrgID := s.runtime.OrgID(ctx)
	if callerScope == domain.ScopeOrg && callerOrgID > 0 {
		if school.OrganizationID == callerOrgID {
			return school.ID, school.OrganizationID, nil
		}
		s.logger.Warn("org-scope account tried to access school outside their organization",
			slog.Int64("account_id", accountID),
			slog.Int64("school_org_id", school.OrganizationID),
			slog.Int64("caller_org_id", callerOrgID),
			slog.String("tenant_slug", tenantSlug))
		return 0, 0, failed("resolve tenant", domain.ErrTenantAccessDenied)
	}
	exists, _, err := s.store.HasActiveAccountTenant(ctx, accountID, school.ID)
	if err != nil {
		s.logger.Warn("failed to verify account tenant mapping",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", school.ID),
			slog.Any("error", err))
		return 0, 0, failed("resolve tenant", domain.ErrTenantAccessDenied)
	}
	if !exists {
		s.logger.Warn("account does not have access to requested tenant",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", school.ID),
			slog.String("tenant_slug", tenantSlug))
		return 0, 0, failed("resolve tenant", domain.ErrTenantAccessDenied)
	}
	return school.ID, school.OrganizationID, nil
}

// resolveAccountTenantDefault picks the first active mapping whose school is
// alive and active, in mapping order.
func (s *AccountAuthentication) resolveAccountTenantDefault(ctx context.Context, accountID int64) (int64, int64, error) {
	tenantIDs, _, err := s.store.ListActiveTenantIDs(ctx, accountID)
	if err != nil {
		s.logger.Warn("failed to resolve account tenant",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
		return 0, 0, fmt.Errorf("resolve account tenants: %w", err)
	}
	if len(tenantIDs) == 0 {
		return 0, 0, failed("resolve tenant", domain.ErrTenantNotFound)
	}
	schools, err := s.schools.ListActiveSchoolsByID(ctx, tenantIDs)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve account schools: %w", err)
	}
	organizationBySchool := make(map[int64]int64, len(schools))
	for _, school := range schools {
		organizationBySchool[school.ID] = school.OrganizationID
	}
	for _, tenantID := range tenantIDs {
		if organizationID, ok := organizationBySchool[tenantID]; ok {
			return tenantID, organizationID, nil
		}
	}
	return 0, 0, failed("resolve tenant", domain.ErrTenantNotFound)
}

// buildClaims constructs the access and refresh claims from the account, the
// minted session and the claims material.
func buildClaims(account domain.LoginAccount, session domain.AccountSession, metadata *domain.AccountClaimsPayload, email string) (domain.SessionClaims, domain.RefreshClaims) {
	access := domain.SessionClaims{
		AccountID:   account.ID,
		Email:       email,
		Username:    metadata.Username,
		FirstName:   metadata.FirstName,
		LastName:    metadata.LastName,
		Roles:       metadata.RoleNames,
		Permissions: metadata.Permissions,
		IsAdmin:     metadata.IsAdmin,
		Scope:       metadata.Scope,
		TenantID:    metadata.TenantID,
		OrgID:       metadata.OrgID,
		FamilyID:    session.FamilyID,
	}
	refresh := domain.RefreshClaims{
		AccountID: account.ID,
		Token:     session.Token,
		TenantID:  metadata.TenantID,
		Scope:     metadata.Scope,
		ExpiresAt: session.Expiry.Unix(),
	}
	return access, refresh
}

// generateAndLogTokens signs the pair and records the event. Persistence has
// already committed, so an audit failure is logged rather than reported:
// reporting it would leave the caller with an issued refresh session but no
// response.
func (s *AccountAuthentication) generateAndLogTokens(ctx context.Context, accountID int64, access domain.SessionClaims, refresh domain.RefreshClaims, ipAddress, userAgent, eventType string) (string, string, error) {
	accessToken, refreshToken, err := s.codec.IssueTokenPair(access, refresh)
	if err != nil {
		return "", "", failed("generate tokens", err)
	}
	if ipAddress != "" {
		if err := s.logAuthEvent(ctx, accountID, eventType, true, ipAddress, userAgent, ""); err != nil {
			s.logger.Error("failed to audit authenticated session",
				slog.Int64("account_id", accountID),
				slog.String("event_type", eventType),
				slog.Any("error", err))
		}
	}
	return accessToken, refreshToken, nil
}

func (s *AccountAuthentication) logFailedLogin(ctx context.Context, accountID int64, ipAddress, userAgent, reason string) {
	if ipAddress == "" {
		return
	}
	if err := s.logAuthEvent(ctx, accountID, domain.AuthEventLogin, false, ipAddress, userAgent, reason); err != nil {
		s.logger.Error("failed to audit rejected login", slog.Any("error", err))
	}
}

// logAuthEvent records one authentication event. The pre-authentication
// routes carry no tenant, so it is resolved from the account's first active
// mapping; the event joins an ambient transaction or opens the tenant's.
func (s *AccountAuthentication) logAuthEvent(ctx context.Context, accountID int64, eventType string, success bool, ipAddress, userAgent, errorMessage string) error {
	tenantID := s.runtime.TenantID(ctx)
	if tenantID == 0 && accountID > 0 {
		tenantID, _, _ = s.resolveAccountTenant(ctx, accountID, "")
	}
	if tenantID <= 0 {
		return fmt.Errorf("audit auth event: tenant is required")
	}
	event := domain.AuthEvent{
		AccountID: accountID, TenantID: tenantID, Type: eventType, Success: success,
		IPAddress: ipAddress, UserAgent: userAgent, ErrorMessage: errorMessage,
	}
	if s.runtime.HasTransaction(ctx) {
		return s.audit.RecordAuthEvent(ctx, event)
	}
	return s.runtime.WithTenantTx(s.runtime.WithTenantID(ctx, tenantID), tenantID, func(txCtx context.Context) error {
		return s.audit.RecordAuthEvent(txCtx, event)
	})
}
