package auth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	jwx "github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/internal/clientip"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Login authenticates a user and returns access and refresh tokens
func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginWithAudit(ctx, email, password, "", "", "")
}

// LoginWithAudit authenticates a user and returns access and refresh tokens with audit logging.
// tenantSlug is optional: when non-empty the account is resolved to the matching tenant;
// when empty the first active tenant mapping is used (Phase 3 fallback).
func (s *Service) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	// Validate credentials and get account
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}

	// Load account metadata first (includes tenant resolution from DB)
	// so the refresh token gets the correct tenant_id on creation.
	// Login is a public route — tenant.FromContext(ctx) would return 0.
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return "", "", err
	}

	// Tenant-portal policy: a guardian-only account at this tenant
	// must use the parents portal. Refuse with a sentinel the frontend
	// turns into "use https://parents.{TENANT_DOMAIN}/". Accounts that
	// are also staff/admin/teacher pass through unchanged.
	if IsGuardianOnlyForTenant(metadata.roleNames) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant login")
		return "", "", &AuthError{Op: "login", Err: ErrParentMustUseParentPortal}
	}

	// Create refresh token with resolved tenant ID
	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID, metadata.scope)
	if err != nil {
		return "", "", err
	}

	// Build JWT claims from account and metadata
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, email)

	// Generate token pair and log success
	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
}

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

// LoginWithMFAGate is the MFA-aware sibling of LoginWithAudit. The pure-
// password LoginWithAudit stays untouched so existing callers and tests
// don't shift; the new method is what the HTTP login handler now uses.
func (s *Service) LoginWithMFAGate(
	ctx context.Context,
	email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string,
) (*LoginResult, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return nil, err
	}

	if IsGuardianOnlyForTenant(metadata.roleNames) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant login")
		return nil, &AuthError{Op: "login", Err: ErrParentMustUseParentPortal}
	}

	// Hydrate roles on the account so MFAService.IsRequired can use
	// account.HasRole("admin") without re-querying. We're projecting just
	// names; the full Role objects aren't needed for this check.
	account.Roles = make([]*auth.Role, len(metadata.roleNames))
	for i, name := range metadata.roleNames {
		account.Roles[i] = &auth.Role{Name: name}
	}

	mfaRequired, enrolled, err := s.resolveTenantMFAGate(ctx, account, metadata.tenantID)
	if err != nil {
		return nil, err
	}

	if mfaRequired && !enrolled {
		return s.tenantMFAEnrollmentResult(account, metadata.tenantID)
	}

	// Decision: issue challenge ⇔ MFA required AND user is enrolled AND
	// no valid trusted-device cookie. Anything else falls through to the
	// existing token-pair pipeline.
	if mfaRequired && enrolled && !s.tenantTrustedDeviceVerified(ctx, account.ID, metadata.tenantID, trustedDeviceCookie) {
		return s.tenantMFAChallengeResult(ctx, account, metadata.tenantID, ipAddress)
	}

	// Token-pair issuance (regular login or MFA-skipped via trusted device).
	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID, metadata.scope)
	if err != nil {
		return nil, err
	}
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, email)
	accessToken, refreshToken, err := s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Status:       LoginStatusAuthenticated,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// resolveTenantMFAGate is the fail-closed MFA inquiry for tenant login.
// A missing MFA service is "not required / not enrolled". Infra errors from
// IsRequired or HasEnrollment refuse this login instead of dropping the
// second factor.
func (s *Service) resolveTenantMFAGate(ctx context.Context, account *auth.Account, tenantID int64) (required, enrolled bool, err error) {
	if s.mfaService == nil {
		return false, false, nil
	}
	required, err = s.mfaService.IsRequired(ctx, account, tenantID)
	if err != nil {
		return false, false, &AuthError{Op: "check mfa required", Err: ErrMFAStatusUnavailable}
	}
	enrolled, err = s.mfaService.HasEnrollment(ctx, account.ID)
	if err != nil {
		return false, false, &AuthError{Op: "check mfa enrollment", Err: ErrMFAStatusUnavailable}
	}
	return required, enrolled, nil
}

// tenantMFAEnrollmentResult issues a tenant-scope enrollment JWT that only
// the /auth/mfa/enroll/* surface accepts. The previous design returned a full
// session token plus an advisory MFAEnrollmentRequired flag that middleware
// did not enforce.
func (s *Service) tenantMFAEnrollmentResult(account *auth.Account, tenantID int64) (*LoginResult, error) {
	enrollmentToken, err := s.tokenAuth.CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
		AccountID: account.ID,
		Scope:     jwt.MFAEnrollmentScopeTenant,
		TenantID:  tenantID,
	}, MFAEnrollmentTokenTTL)
	if err != nil {
		return nil, &AuthError{Op: "issue mfa enrollment token", Err: err}
	}
	return &LoginResult{
		Status:                LoginStatusMFAEnrollmentRequired,
		AccessToken:           enrollmentToken,
		MaskedEmail:           MaskEmailForUX(account.Email),
		MFAEnrollmentRequired: true,
	}, nil
}

func (s *Service) tenantTrustedDeviceVerified(ctx context.Context, accountID, tenantID int64, cookie string) bool {
	if cookie == "" || s.mfaService == nil {
		return false
	}
	ok, _ := s.mfaService.VerifyTrustedDevice(ctx, accountID, tenantID, cookie)
	return ok
}

// tenantMFAChallengeResult is the "MFA required, factor enrolled" response:
// an email code plus a tenant-scope challenge token.
func (s *Service) tenantMFAChallengeResult(ctx context.Context, account *auth.Account, tenantID int64, ipAddress string) (*LoginResult, error) {
	challenge, err := s.mfaService.StartChallenge(ctx, account.ID, tenantID, jwt.MFAChallengeScopeTenant, ParseClientIP(ipAddress))
	if err != nil {
		return nil, &AuthError{Op: "start mfa challenge", Err: err}
	}
	return &LoginResult{
		Status:               LoginStatusMFARequired,
		ChallengeToken:       challenge,
		MaskedEmail:          MaskEmailForUX(account.Email),
		TrustedDeviceEnabled: s.mfaService.IsTrustedDeviceEnabled(ctx, tenantID),
		TrustedDeviceDays:    s.mfaService.TrustedDeviceDays(ctx, tenantID),
	}, nil
}

// MaskEmailForUX renders an email address as `j***@example.com` so the
// frontend can show the user *which* mailbox just received a code without
// leaking the full address (e.g. in shared-screen scenarios). Shared with
// the operator login flow in services/platform.
func MaskEmailForUX(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 1 {
		return local + "***" + domain
	}
	return string(local[0]) + "***" + domain
}

// ParseClientIP wraps net.ParseIP with the empty-string guard so audit rows
// don't get malformed inet values. Shared with the operator login flow in
// services/platform.
func ParseClientIP(ipAddress string) net.IP {
	return clientip.ParseIPString(ipAddress)
}

// IssueTokensForAuthenticatedAccount mints an access + refresh token pair for
// an account whose identity was proven via a non-password channel (typically
// MFA email-code or recovery-code verification). It skips password validation
// but otherwise reuses the same metadata-load → token-persist → claims-build
// → token-gen pipeline as a regular login, so the resulting session is
// indistinguishable from one obtained via /auth/login.
//
// tenantID is the tenant the user is authenticating into (carried in the MFA
// challenge JWT). Pass 0 to let loadAccountMetadataForTenant pick the user's
// only active tenant.
func (s *Service) IssueTokensForAuthenticatedAccount(
	ctx context.Context,
	accountID, tenantID int64,
	ipAddress, userAgent string,
) (string, string, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return "", "", &AuthError{Op: "issue tokens", Err: ErrAccountNotFound}
	}
	if !account.Active {
		return "", "", &AuthError{Op: "issue tokens", Err: ErrAccountInactive}
	}

	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return "", "", err
	}
	if IsGuardianOnlyForTenant(metadata.roleNames) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant token issue")
		return "", "", &AuthError{Op: "issue tokens", Err: ErrParentMustUseParentPortal}
	}

	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID, metadata.scope)
	if err != nil {
		return "", "", err
	}

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)
	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
}

// validateLoginCredentials validates email, password, and account status
func (s *Service) validateLoginCredentials(ctx context.Context, email, password, ipAddress, userAgent string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		s.logFailedLogin(ctx, 0, ipAddress, userAgent, "Account not found")
		return nil, &AuthError{Op: "login", Err: ErrAccountNotFound}
	}

	if !account.Active {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Account inactive")
		return nil, &AuthError{Op: "login", Err: ErrAccountInactive}
	}

	if err := s.verifyPassword(account, password); err != nil {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Invalid password")
		return nil, err
	}

	return account, nil
}

// verifyPassword checks if the provided password matches the account's hash
func (s *Service) verifyPassword(account *auth.Account, password string) error {
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(password, *account.PasswordHash)
	if err != nil || !valid {
		return &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	return nil
}

// mintGuard runs inside the token-persistence transaction, immediately before
// the token row is written. It is the only place where an authorization fact
// can be re-checked atomically with the write it authorizes: everything a
// login flow verified earlier lives in an already-committed transaction and
// may be stale by the time the token is minted.
//
// Implementations receive a context carrying the phoenix_admin transaction, so
// they MUST call repositories directly instead of opening a nested WithAdminTx
// (bun does not nest — that would take a second connection and defeat the
// point). Returning an error aborts the transaction and the mint; it is
// surfaced verbatim, never retried.
//
// The account is the one the CALLER holds. In refreshTokenInTransaction that is
// the row this very transaction locked FOR UPDATE, so a guard may read it as
// current; on the login path it is the pre-transaction read, and a guard that
// needs fresh account state must re-read (and lock) it itself.
//
// A guard is also where anything the JWT is built from belongs. Assembling
// claims AFTER the transaction committed means a failure at that point has
// already rotated the caller's refresh token and has no successor to hand
// back — see refreshClaimsGuard.
type mintGuard func(ctx context.Context, account *auth.Account) error

// mintGuardError marks an error as coming from a mintGuard so the retry loop
// can pass the caller's sentinel through untouched instead of burying it under
// the generic "login transaction" wrapper.
type mintGuardError struct{ err error }

func (e *mintGuardError) Error() string { return e.err.Error() }
func (e *mintGuardError) Unwrap() error { return e.err }

// createRefreshTokenWithRetry creates a refresh token with retry logic for concurrent logins
func (s *Service) createRefreshTokenWithRetry(ctx context.Context, account *auth.Account, tenantID int64, scope string) (*auth.Token, error) {
	return s.createRefreshTokenWithRetryGuarded(ctx, account, tenantID, scope, nil)
}

// createRefreshTokenWithRetryGuarded is createRefreshTokenWithRetry with an
// authorization re-check that runs inside the persistence transaction. A guard
// failure is terminal — the retry loop only exists for token-family
// collisions, and re-running a guard that just said "no" would be pointless.
func (s *Service) createRefreshTokenWithRetryGuarded(
	ctx context.Context,
	account *auth.Account,
	tenantID int64,
	scope string,
	guard mintGuard,
) (*auth.Token, error) {
	token := s.newRefreshToken(account.ID, scope)

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := s.persistTokenInTransaction(ctx, account, token, tenantID, guard)

		if err == nil {
			return token, nil
		}

		var guardErr *mintGuardError
		if errors.As(err, &guardErr) {
			return nil, guardErr.err
		}

		if !s.isTokenFamilyConflict(err) {
			return nil, &AuthError{Op: "login transaction", Err: err}
		}

		// Regenerate family ID and retry
		token.FamilyID = uuid.Must(uuid.NewV4()).String()
		s.getLogger().Warn("login race condition detected, retrying",
			slog.Int64("account_id", account.ID),
			slog.Int("attempt", attempt+1),
			slog.Int("max_retries", maxRetries))
	}

	return nil, &AuthError{Op: "login transaction", Err: fmt.Errorf("max retries exceeded")}
}

// newRefreshToken creates a new refresh token for the given account
func (s *Service) newRefreshToken(accountID int64, scope string) *auth.Token {
	identifier := "Service login"
	return &auth.Token{
		Token:       uuid.Must(uuid.NewV4()).String(),
		AccountID:   accountID,
		Expiry:      time.Now().Add(s.jwtRefreshExpiry),
		Mobile:      false,
		Identifier:  &identifier,
		FamilyID:    uuid.Must(uuid.NewV4()).String(),
		Generation:  0,
		PortalScope: persistedPortalScope(scope),
	}
}

// persistTokenInTransaction saves the token and updates last login in a transaction.
//
// Uses WithAdminTx (BYPASSRLS) because this is a public login route with no JWT/tenant
// context. The phoenix_auth connection role cannot pass RLS policies on auth.tokens,
// so we switch to phoenix_admin for the token write.
//
// guard (optional) re-validates the caller's authorization inside this
// transaction, before anything is written — see mintGuard.
func (s *Service) persistTokenInTransaction(ctx context.Context, account *auth.Account, token *auth.Token, tenantID int64, guard mintGuard) error {
	return tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		if err := s.applyMintGuard(ctx, account, guard); err != nil {
			return err
		}

		// Updating the account first acquires its row lock for the rest of this
		// transaction. Concurrent token issuers for the same account then enforce
		// the session cap serially instead of deleting from identical snapshots.
		if err := s.repos.Account.UpdateLastLogin(ctx, account.ID); err != nil {
			return fmt.Errorf("update last login before token issuance: %w", err)
		}
		loginTime := time.Now()
		account.LastLogin = &loginTime

		// Set tenant ID from DB resolution (not from context — login is a public route)
		token.SetTenantID(tenantID)

		// Create new token
		if err := s.repos.Token.Create(ctx, token); err != nil {
			if s.isTokenFamilyConflict(err) {
				return err // Will retry with new family ID
			}
			return err
		}

		now := time.Now()
		if err := s.repos.Token.DeleteExpiredRotatedForAccount(ctx, account.ID, now); err != nil {
			s.getLogger().Warn("failed to clean up refresh-token handoffs",
				slog.Int64("account_id", account.ID),
				slog.Any("error", err),
			)
		}

		return s.enforcePortalSessionCap(ctx, account.ID, token.PortalScope)
	})
}

func (s *Service) applyMintGuard(ctx context.Context, account *auth.Account, guard mintGuard) error {
	if guard == nil {
		return nil
	}
	if err := guard(ctx, account); err != nil {
		return &mintGuardError{err: err}
	}
	return nil
}

// enforcePortalSessionCap keeps at most five active sessions in this portal.
// Other portals keep their own sessions.
func (s *Service) enforcePortalSessionCap(ctx context.Context, accountID int64, portalScope string) error {
	const maxActiveSessionsPerPortal = 5
	deleted, err := s.repos.Token.CleanupOldTokensForAccountReturning(ctx, accountID, portalScope, maxActiveSessionsPerPortal)
	if err != nil {
		return fmt.Errorf("enforce active session cap: %w", err)
	}
	return s.auditRevokedTokens(ctx, deleted, "session_cap", "", "")
}

// isTokenFamilyConflict checks if error is due to token family conflict
func (s *Service) isTokenFamilyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uk_tokens_family_generation")
}

// accountMetadata holds account-related metadata for JWT claims
type accountMetadata struct {
	roleNames      []string
	permissionStrs []string
	username       string
	firstName      string
	lastName       string
	isAdmin        bool
	tenantID       int64
	orgID          int64
	scope          string
}

// loadAccountMetadata loads roles, permissions, person information, and tenant mapping.
// tenantSlug is optional: when non-empty the account is resolved to the matching tenant.
// Returns partial data with logged warnings if any lookups fail.
// Returns an error only when tenantSlug is provided but resolution fails (tenant not found
// or account not mapped to that tenant).
//
// D13 revision: tenant is resolved FIRST so that roles and permissions can be scoped to
// the resolved tenant, preventing cross-tenant privilege leakage in JWT tokens.
func (s *Service) loadAccountMetadata(ctx context.Context, account *auth.Account, tenantSlug string) (*accountMetadata, error) {
	// Use WithAdminTx (BYPASSRLS) because this runs during login/switch/refresh
	// where no tenant context exists yet. RLS policies on auth.account_roles and
	// auth.account_permissions require app.current_tenant_id to be set, which only
	// happens inside tenant-scoped transactions. Without BYPASSRLS the role/permission
	// queries return zero rows and the JWT gets empty permissions.
	var result *accountMetadata
	err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {

		// Step 1: Resolve tenant FIRST — roles/permissions depend on the target tenant.
		tenantID, orgID, err := s.resolveAccountTenant(ctx, account.ID, tenantSlug)
		if err != nil {
			return err
		}

		// Step 2: Load roles scoped to the resolved tenant (D13 §6.1 step 6).
		if err := s.ensureAccountRolesLoadedForTenant(ctx, account, tenantID); err != nil {
			return err
		}

		// Step 3: Load permissions scoped to the resolved tenant (D13 §6.1 step 7).
		permissions, err := s.loadAccountPermissionsForTenant(ctx, account.ID, tenantID)
		if err != nil {
			return err
		}
		roleNames := s.extractRoleNames(account.Roles)
		permissionStrs := s.extractPermissionNames(permissions)

		username := s.extractUsername(account)
		firstName, lastName, err := s.loadPersonNamesForTenant(ctx, account.ID, tenantID)
		if err != nil {
			return err
		}
		isAdmin := s.checkRoleFlags(roleNames)

		result = &accountMetadata{
			roleNames:      roleNames,
			permissionStrs: permissionStrs,
			username:       username,
			firstName:      firstName,
			lastName:       lastName,
			isAdmin:        isAdmin,
			tenantID:       tenantID,
			orgID:          orgID,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// loadAccountMetadataForTenant loads roles, permissions, and person information for a known tenant ID.
// Used by the refresh flow where the tenant is already validated and must be preserved exactly —
// re-resolving via slug or default fallback could silently switch to a different tenant.
func (s *Service) loadAccountMetadataForTenant(ctx context.Context, account *auth.Account, tenantID int64) (*accountMetadata, error) {
	var result *accountMetadata
	err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, _ bun.Tx) error {
		var err error
		result, err = s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// loadAccountMetadataForTenantInTx is loadAccountMetadataForTenant for callers
// that ALREADY hold a phoenix_admin transaction and need the load to happen
// inside it. bun does not nest transactions, so a caller inside one must not
// route through the WithAdminTx wrapper above: that would take a second
// connection with its own snapshot — which is precisely how the school refresh
// used to end up assembling claims from state its own rotation had already
// committed past.
//
// Only pass a context carrying a phoenix_admin transaction. Under the
// phoenix_tenant role the role/permission reads hit RLS and come back empty,
// which reads downstream as a legitimately unprivileged session.
func (s *Service) loadAccountMetadataForTenantInTx(ctx context.Context, account *auth.Account, tenantID int64) (*accountMetadata, error) {
	// Look up the school's organization ID for the JWT org_id claim.
	var orgID int64
	if tenantID > 0 {
		school, err := s.repos.School.FindByID(ctx, tenantID)
		if err != nil {
			// Distinguish "not found" from transient DB errors so the caller
			// returns 401 (re-login) instead of 500 (retry) when the school
			// was hard-deleted between token issuance and this refresh.
			if errors.Is(err, sql.ErrNoRows) {
				return nil, &AuthError{Op: "load metadata for tenant", Err: ErrTenantNotFound}
			}
			return nil, fmt.Errorf("lookup school for tenant %d: %w", tenantID, err)
		}
		if school == nil || school.IsDeleted() {
			return nil, &AuthError{Op: "load metadata for tenant", Err: ErrTenantNotFound}
		}
		orgID = school.OrganizationID
	}

	// Load roles and permissions scoped to the preserved tenant.
	if err := s.ensureAccountRolesLoadedForTenant(ctx, account, tenantID); err != nil {
		return nil, err
	}
	permissions, err := s.loadAccountPermissionsForTenant(ctx, account.ID, tenantID)
	if err != nil {
		return nil, err
	}
	roleNames := s.extractRoleNames(account.Roles)
	permissionStrs := s.extractPermissionNames(permissions)

	username := s.extractUsername(account)
	// Pin the person lookup to the tenant this token is being minted for —
	// the refresh/switch paths carry the previous school in the ambient
	// context (see loadPersonNamesForTenant).
	firstName, lastName, err := s.loadPersonNamesForTenant(ctx, account.ID, tenantID)
	if err != nil {
		return nil, err
	}
	isAdmin := s.checkRoleFlags(roleNames)

	return &accountMetadata{
		roleNames:      roleNames,
		permissionStrs: permissionStrs,
		username:       username,
		firstName:      firstName,
		lastName:       lastName,
		isAdmin:        isAdmin,
		tenantID:       tenantID,
		orgID:          orgID,
	}, nil
}

// ensureAccountRolesLoadedForTenant loads account roles scoped to a specific tenant.
// Used during login/switch flows where no tenant context exists yet (D13 §6.1 step 6).
//
// Query failures are PROPAGATED, never swallowed. An empty role set is not a
// harmless degradation: MFAService.IsRequired evaluates security.mfa_mode =
// required_admins against exactly these roles, so a transient DB error used to
// turn an admin into a role-less account and waved the login through without a
// second factor. A genuine "no roles" result arrives as an empty slice, so only
// real infra errors reach the caller — which maps them to a retryable 500.
func (s *Service) ensureAccountRolesLoadedForTenant(ctx context.Context, account *auth.Account, tenantID int64) error {
	// Clear any previously loaded roles to ensure fresh tenant-scoped loading
	account.Roles = nil

	accountRoles, err := s.repos.AccountRole.FindByAccountIDForTenant(ctx, account.ID, tenantID)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		s.getLogger().Warn("failed to load tenant-scoped roles; refusing login",
			slog.Int64("account_id", account.ID),
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return fmt.Errorf("load tenant-scoped roles for account %d at tenant %d: %w", account.ID, tenantID, err)
	}

	for _, ar := range accountRoles {
		if ar.Role != nil {
			account.Roles = append(account.Roles, ar.Role)
		}
	}
	return nil
}

// loadAccountPermissionsForTenant retrieves permissions scoped to a specific tenant.
// Used during login/switch flows where no tenant context exists yet (D13 §6.1 step 7).
//
// Same fail-closed contract as ensureAccountRolesLoadedForTenant: a DB error
// must not silently mint a token with an empty permission set, which reads to
// every downstream authorize check as a legitimately unprivileged session.
func (s *Service) loadAccountPermissionsForTenant(ctx context.Context, accountID int64, tenantID int64) ([]*auth.Permission, error) {
	permissions, err := s.repos.Permission.FindByAccountIDForTenant(ctx, accountID, tenantID)
	if err != nil {
		if isNotFoundError(err) {
			return []*auth.Permission{}, nil
		}
		s.getLogger().Warn("failed to load tenant-scoped permissions; refusing login",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("load tenant-scoped permissions for account %d at tenant %d: %w", accountID, tenantID, err)
	}
	return permissions, nil
}

// extractRoleNames converts roles to string slice
func (s *Service) extractRoleNames(roles []*auth.Role) []string {
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}
	return roleNames
}

// extractPermissionNames converts permissions to string slice
func (s *Service) extractPermissionNames(permissions []*auth.Permission) []string {
	permissionStrs := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionStrs = append(permissionStrs, perm.GetFullName())
	}
	return permissionStrs
}

// extractUsername safely extracts username from account
func (s *Service) extractUsername(account *auth.Account) string {
	if account.Username != nil {
		return *account.Username
	}
	return ""
}

// loadPersonNames retrieves first and last name from the person record visible
// in the context's tenant scope. A missing person row is a legitimate result
// ("" / ""); a failed LOOKUP is not, and is propagated — collapsing the two
// used to send a blank-named token on a DB blip and, worse, sent the caller
// into the cross-school fallback below on the strength of an error.
func (s *Service) loadPersonNames(ctx context.Context, accountID int64) (string, string, error) {
	person, err := s.repos.Person.FindByAccountID(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if person == nil {
		return "", "", nil
	}
	return person.FirstName, person.LastName, nil
}

// loadPersonNamesForTenant resolves the person names that belong in a token
// minted FOR tenantID, instead of whichever tenant happens to sit in the
// ambient context.
//
// users.persons is tenant-scoped and PersonRepository.FindByAccountID applies
// the tenant from the CONTEXT. Every mint path that switches schools — the
// tenant portal's SwitchTenant, the school portal's SwitchSchool — runs inside
// the request of the SOURCE school, so the ambient context named the school
// the user is leaving: an account with person rows at two schools got the old
// school's name stamped into the new JWT, and one with a person row only at
// the target got no name at all.
//
// The fallback keeps accounts without a person row at the target working: an
// org-scope Träger user reaches a school through organization membership, and
// their person row stays at their home school. It is deliberately NOT the
// unscoped "any person row with this account_id" query it used to be. That one
// dropped the tenant filter entirely and took whatever row the database handed
// back first — with no ORDER BY, so an account with person rows at several
// schools got an arbitrary school's name stamped into its JWT, and a *failed*
// target lookup fell into it as readily as a genuinely empty one.
//
// What replaces it is bounded and deterministic: only schools the account is
// ACTIVELY mapped to are consulted, in ascending tenant order, and the name is
// used only when those schools agree on it. Two different names across two
// schools is an ambiguity this function is not entitled to resolve — it yields
// no name (and says so in the log) rather than guessing one.
func (s *Service) loadPersonNamesForTenant(ctx context.Context, accountID, tenantID int64) (string, string, error) {
	if tenantID > 0 {
		firstName, lastName, err := s.loadPersonNames(tenant.WithTenantID(ctx, tenantID), accountID)
		if err != nil {
			return "", "", fmt.Errorf("load person names for account %d at tenant %d: %w", accountID, tenantID, err)
		}
		if firstName != "" || lastName != "" {
			return firstName, lastName, nil
		}
	}
	return s.loadPersonNamesFromMappedTenants(ctx, accountID, tenantID)
}

// loadPersonNamesFromMappedTenants resolves a person name from the OTHER
// schools the account is actively mapped to — the authorized, deterministic
// half of loadPersonNamesForTenant's fallback.
func (s *Service) loadPersonNamesFromMappedTenants(ctx context.Context, accountID, excludeTenantID int64) (string, string, error) {
	mappings, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return "", "", fmt.Errorf("list active tenants of account %d for person lookup: %w", accountID, err)
	}

	tenantIDs := mappedTenantIDsExcluding(mappings, excludeTenantID)

	var firstName, lastName string
	for _, mappedTenantID := range tenantIDs {
		candidateFirst, candidateLast, err := s.loadPersonNames(tenant.WithTenantID(ctx, mappedTenantID), accountID)
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
			s.getLogger().Warn("person name ambiguous across schools; minting token without a name",
				slog.Int64("account_id", accountID),
				slog.Int64("tenant_id", excludeTenantID),
			)
			return "", "", nil
		}
	}
	return firstName, lastName, nil
}

func mappedTenantIDsExcluding(mappings []auth.AccountTenant, excludeTenantID int64) []int64 {
	tenantIDs := make([]int64, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.TenantID != excludeTenantID && mapping.TenantID > 0 {
			tenantIDs = append(tenantIDs, mapping.TenantID)
		}
	}
	slices.Sort(tenantIDs)
	return tenantIDs
}

// checkRoleFlags determines if account has admin role
func (s *Service) checkRoleFlags(roleNames []string) bool {
	for _, roleName := range roleNames {
		if roleName == "admin" {
			return true
		}
	}
	return false
}

// resolveAccountTenant resolves the tenant ID and organization ID for an account.
// When tenantSlug is non-empty the school is looked up by subdomain and verified
// against the account's active tenant mappings. When empty, the first active
// mapping is used (Phase 3 fallback).
// Returns a non-nil error only when tenantSlug is provided but resolution fails.
func (s *Service) resolveAccountTenant(ctx context.Context, accountID int64, tenantSlug string) (int64, int64, error) {
	if tenantSlug != "" {
		return s.resolveAccountTenantBySlug(ctx, accountID, tenantSlug)
	}

	return s.resolveAccountTenantDefault(ctx, accountID)
}

// resolveAccountTenantBySlug resolves tenant by subdomain slug, then verifies
// the account has access to that tenant.
//
// Access check depends on the caller's scope (spec §6.3):
//   - Normal scope (""): check account_tenants for explicit mapping
//   - Org scope ("org"): check school.organization_id matches the caller's org_id
//     (Träger-Büro auto-access to all schools in their organization)
func (s *Service) resolveAccountTenantBySlug(ctx context.Context, accountID int64, tenantSlug string) (int64, int64, error) {
	// Look up the school by subdomain (includes soft-deleted schools so we can
	// distinguish "deleted" from "not found" and return appropriate errors).
	school, err := s.repos.School.FindBySubdomain(ctx, tenantSlug)
	if err != nil {
		s.getLogger().Warn("tenant slug lookup failed",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Any("error", err),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: err}
	}
	if school == nil {
		s.getLogger().Warn("tenant slug not found",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	if school.IsDeleted() {
		s.getLogger().Warn("tenant is soft-deleted",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Int64("tenant_id", school.ID),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	if !school.Active {
		s.getLogger().Warn("tenant is inactive",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Int64("tenant_id", school.ID),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	// Org-scope check (§6.3): Träger-Büro users access any school in their org
	// without needing an explicit account_tenants entry.
	callerScope := tenant.ScopeFromContext(ctx)
	callerOrgID := tenant.OrgFromContext(ctx)

	if callerScope == tenant.ScopeOrg && callerOrgID > 0 {
		if school.OrganizationID == callerOrgID {
			return school.ID, school.OrganizationID, nil
		}
		s.getLogger().Warn("org-scope account tried to access school outside their organization",
			slog.Int64("account_id", accountID),
			slog.Int64("school_org_id", school.OrganizationID),
			slog.Int64("caller_org_id", callerOrgID),
			slog.String("tenant_slug", tenantSlug),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantAccessDenied}
	}

	// Normal scope: verify the account has an explicit active mapping to this tenant
	exists, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, school.ID)
	if err != nil {
		s.getLogger().Warn("failed to verify account tenant mapping",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", school.ID),
			slog.Any("error", err),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantAccessDenied}
	}

	if !exists {
		s.getLogger().Warn("account does not have access to requested tenant",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", school.ID),
			slog.String("tenant_slug", tenantSlug),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantAccessDenied}
	}

	return school.ID, school.OrganizationID, nil
}

// resolveAccountTenantDefault resolves the tenant using the first active, non-deleted mapping
// (Phase 3 fallback). Iterates through all tenant mappings to handle cases where some schools
// are deleted or inactive. Returns ErrTenantNotFound if no valid mapping is found.
func (s *Service) resolveAccountTenantDefault(ctx context.Context, accountID int64) (int64, int64, error) {
	tenants, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		s.getLogger().Warn("failed to resolve account tenant",
			slog.Int64("account_id", accountID),
			slog.Any("error", err),
		)
		return 0, 0, fmt.Errorf("resolve account tenants: %w", err)
	}
	if len(tenants) == 0 {
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	// Iterate all mappings — skip deleted or inactive schools, use the first valid one.
	// Track lookup errors separately so we don't mask DB failures as "not found".
	var lastLookupErr error
	for _, t := range tenants {
		school, err := s.repos.School.FindByID(ctx, t.TenantID)
		if err != nil {
			s.getLogger().Warn("failed to resolve school for tenant",
				slog.Int64("tenant_id", t.TenantID),
				slog.Any("error", err),
			)
			lastLookupErr = err
			continue
		}
		if school == nil {
			continue
		}
		if school.IsDeleted() || !school.Active {
			s.getLogger().Debug("skipping deleted or inactive tenant during default resolution",
				slog.Int64("account_id", accountID),
				slog.Int64("tenant_id", t.TenantID),
				slog.Bool("deleted", school.IsDeleted()),
				slog.Bool("active", school.Active),
			)
			continue
		}
		return t.TenantID, school.OrganizationID, nil
	}

	// If every lookup failed with a DB error, propagate it instead of masking as not-found.
	if lastLookupErr != nil {
		return 0, 0, fmt.Errorf("resolve school for tenant: %w", lastLookupErr)
	}

	// All mappings point to deleted/inactive schools — no valid tenant available.
	return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
}

// buildJWTClaims constructs JWT claims from account and metadata
func (s *Service) buildJWTClaims(
	account *auth.Account,
	token *auth.Token,
	metadata *accountMetadata,
	email string,
) (jwt.AppClaims, jwt.RefreshClaims) {
	appClaims := jwt.AppClaims{
		ID:          int(account.ID),
		Sub:         email,
		Username:    metadata.username,
		FirstName:   metadata.firstName,
		LastName:    metadata.lastName,
		Roles:       metadata.roleNames,
		Permissions: metadata.permissionStrs,
		IsAdmin:     metadata.isAdmin,
		Scope:       metadata.scope,
		TenantID:    metadata.tenantID,
		OrgID:       metadata.orgID,
	}

	refreshClaims := jwt.RefreshClaims{
		ID:       int(account.ID),
		Token:    token.Token,
		TenantID: metadata.tenantID,
		Scope:    metadata.scope,
		CommonClaims: jwt.CommonClaims{
			ExpiresAt: token.Expiry.Unix(),
		},
	}

	return appClaims, refreshClaims
}

// generateAndLogTokens generates JWT token pair and logs the authentication event
func (s *Service) generateAndLogTokens(
	ctx context.Context,
	accountID int64,
	appClaims jwt.AppClaims,
	refreshClaims jwt.RefreshClaims,
	ipAddress, userAgent, eventType string,
) (string, string, error) {
	accessToken, refreshToken, err := s.tokenAuth.GenTokenPair(appClaims, refreshClaims)
	if err != nil {
		return "", "", &AuthError{Op: "generate tokens", Err: err}
	}

	if ipAddress != "" {
		s.logAuthEvent(ctx, accountID, eventType, true, ipAddress, userAgent, "")
	}

	return accessToken, refreshToken, nil
}

// logFailedLogin logs a failed login attempt if IP address is provided
func (s *Service) logFailedLogin(ctx context.Context, accountID int64, ipAddress, userAgent, reason string) {
	if ipAddress != "" {
		s.logAuthEvent(ctx, accountID, audit.EventTypeLogin, false, ipAddress, userAgent, reason)
	}
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*auth.Account, error) {
	// Validate and normalize registration inputs
	if err := s.validateRegistrationInputs(ctx, email, username, password); err != nil {
		return nil, err
	}

	if roleID != nil && *roleID > 0 && tenantID <= 0 {
		return nil, &AuthError{Op: "register", Err: ErrTenantRequiredForRoleAssignment}
	}
	if roleID != nil && *roleID > 0 {
		var roleErr error
		err := tenant.WithAdminTxOrDirect(ctx, s.db, func(adminCtx context.Context) error {
			// System roles have tenant_id NULL. Clear only the Go context tenant
			// for this lookup; the surrounding transaction and its RLS context stay
			// intact for the subsequent account creation.
			roleLookupCtx := tenant.WithTenantID(adminCtx, 0)
			_, roleErr = ValidateAssignableSchoolRole(roleLookupCtx, s.repos.Role, *roleID, tenantID)
			return roleErr
		})
		if err != nil {
			return nil, &AuthError{Op: "register", Err: err}
		}
	}

	// Create account object with hashed password
	account, err := s.createAccountObject(email, username, password)
	if err != nil {
		return nil, err
	}

	// Persist account and assign role in transaction
	if err := s.persistAccountWithRole(ctx, account, roleID, tenantID); err != nil {
		return nil, err
	}

	return account, nil
}

// validateRegistrationInputs validates registration data and checks for conflicts
func (s *Service) validateRegistrationInputs(ctx context.Context, email, username, password string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if err := ValidatePasswordStrength(password); err != nil {
		return &AuthError{Op: "register", Err: err}
	}

	// Check if email already exists
	if _, err := s.repos.Account.FindByEmail(ctx, email); err == nil {
		return &AuthError{Op: "register", Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	if _, err := s.repos.Account.FindByUsername(ctx, username); err == nil {
		return &AuthError{Op: "register", Err: ErrUsernameAlreadyExists}
	}

	return nil
}

// createAccountObject creates a new account with hashed password
func (s *Service) createAccountObject(email, username, password string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: opHashPassword, Err: err}
	}

	usernamePtr := &username
	now := time.Now()

	return &auth.Account{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
		LastLogin:    &now,
	}, nil
}

// persistAccountWithRole saves account, maps it to a tenant, and assigns a role.
// Uses WithTenantTx so that RLS on auth.account_roles enforces tenant isolation
// at the database level. phoenix_tenant has CRUD on all tables in the auth schema
// (including auth.accounts which has no RLS), so no admin escalation is needed.
// The WITH CHECK policy on auth.account_roles guarantees the inserted tenant_id
// matches the transaction's app.current_tenant_id — a code bug cannot silently
// create cross-tenant role assignments.
func (s *Service) persistAccountWithRole(ctx context.Context, account *auth.Account, roleID *int64, tenantID int64) error {
	if tenantID <= 0 {
		// No tenant context (e.g. tests) — fall back to admin tx for the account insert only.
		return tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
			return s.repos.Account.Create(ctx, account)
		})
	}

	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {

		// Create account (auth.accounts has no tenant_id, no RLS — plain INSERT)
		if err := s.repos.Account.Create(ctx, account); err != nil {
			return err
		}

		// Map account to tenant so the user can log into this school
		now := time.Now()
		mapping := &auth.AccountTenant{
			AccountID:   account.ID,
			TenantID:    tenantID,
			Status:      auth.AccountTenantStatusActive,
			ActivatedAt: &now,
		}
		if err := s.repos.AccountTenant.Create(ctx, mapping); err != nil {
			return fmt.Errorf("failed to create account-tenant mapping: %w", err)
		}

		// Assign role scoped to this tenant (RLS WITH CHECK enforces tenant_id match)
		if roleID != nil && *roleID > 0 {
			accountRole := &auth.AccountRole{
				AccountID: account.ID,
				RoleID:    *roleID,
			}
			accountRole.SetTenantID(tenantID)
			if err := s.repos.AccountRole.Create(ctx, accountRole); err != nil {
				return fmt.Errorf("failed to assign role to account: %w", err)
			}
		}

		return nil
	})
}

// LinkAccountToTenant links an existing account to a tenant with an optional role assignment.
// The password is NOT changed — the user keeps their current credentials.
// Returns ErrAccountNotFound if no account exists with the given email.
// Returns ErrAccountInactive if the account is deactivated.
func (s *Service) LinkAccountToTenant(ctx context.Context, email string, roleID *int64, tenantID int64) (*auth.Account, error) {
	const op = "link-to-tenant"
	email = strings.TrimSpace(strings.ToLower(email))

	if tenantID <= 0 {
		return nil, &AuthError{Op: op, Err: ErrTenantRequiredForRoleAssignment}
	}

	// Same role policy as operator-led school access: no guardian (that is the
	// guardian invitation flow), no retired teacher role, and no role belonging
	// to a different school (issue #1021).
	if roleID != nil && *roleID > 0 {
		var roleErr error
		// System roles have tenant_id NULL. Clear only the Go context tenant for
		// this lookup; the admin transaction and the target-school policy remain
		// in force.
		err := tenant.WithAdminTxOrDirect(ctx, s.db, func(adminCtx context.Context) error {
			roleLookupCtx := tenant.WithTenantID(adminCtx, 0)
			_, roleErr = ValidateAssignableSchoolRole(roleLookupCtx, s.repos.Role, *roleID, tenantID)
			return roleErr
		})
		if err != nil {
			return nil, &AuthError{Op: op, Err: err}
		}
	}

	// Find existing account
	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: op, Err: ErrAccountNotFound}
	}

	if !account.Active {
		return nil, &AuthError{Op: op, Err: ErrAccountInactive}
	}

	// Link to tenant (idempotent — handles already-linked case)
	if err := s.performAccountTenantLink(ctx, account, roleID, tenantID); err != nil {
		return nil, &AuthError{Op: op, Err: fmt.Errorf("link failed: %w", err)}
	}

	s.getLogger().Info("account linked to tenant",
		slog.Int64("account_id", account.ID),
		slog.Int64("tenant_id", tenantID))

	return account, nil
}

// performAccountTenantLink creates a tenant mapping and role assignment for an existing account.
func (s *Service) performAccountTenantLink(ctx context.Context, account *auth.Account, roleID *int64, tenantID int64) error {
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {

		if err := s.ensureTenantMapping(ctx, account.ID, tenantID); err != nil {
			return err
		}
		return s.ensureRoleAssignment(ctx, account.ID, roleID, tenantID)
	})
}

// ensureTenantMapping creates an account-tenant mapping if one does not already exist.
func (s *Service) ensureTenantMapping(ctx context.Context, accountID, tenantID int64) error {
	now := time.Now()
	mapping := &auth.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      auth.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.repos.AccountTenant.Create(ctx, mapping); err != nil {
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("failed to create account-tenant mapping: %w", err)
		}
	}
	return nil
}

// ensureRoleAssignment assigns a role to an account for a tenant, ignoring duplicates.
func (s *Service) ensureRoleAssignment(ctx context.Context, accountID int64, roleID *int64, tenantID int64) error {
	if roleID == nil || *roleID <= 0 {
		return nil
	}
	accountRole := &auth.AccountRole{
		AccountID: accountID,
		RoleID:    *roleID,
	}
	accountRole.SetTenantID(tenantID)
	if err := s.repos.AccountRole.Create(ctx, accountRole); err != nil {
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("failed to assign role to account: %w", err)
		}
	}
	return nil
}

// isDuplicateKeyError checks if a database error is a unique constraint violation (PG code 23505).
func isDuplicateKeyError(err error) bool {
	return modelBase.IsUniqueViolation(err)
}

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	return s.RefreshTokenWithAudit(ctx, refreshTokenStr, "", "")
}

// parseRefreshTokenClaims parses and validates JWT refresh token claims
func (s *Service) parseRefreshTokenClaims(refreshTokenStr string) (*jwt.RefreshClaims, error) {
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return nil, &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	claims := extractClaims(jwtToken)

	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return nil, &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	return &refreshClaims, nil
}

// refreshTokenInTransaction validates and refreshes token in a transaction.
//
// Uses WithAdminTx (BYPASSRLS) because token refresh is a pre-authentication flow
// with no JWT/tenant context yet. The phoenix_auth connection role cannot pass RLS
// policies on auth.tokens (same reason persistTokenInTransaction uses WithAdminTx).
// guard (optional) re-validates the caller's authorization inside this
// transaction, immediately after the account row is locked and before any
// token is written — see mintGuard. It is what keeps a scope-specific
// authorization decision (today: school-portal access) from being made
// against state the rotation has already committed past.
func (s *Service) refreshTokenInTransaction(ctx context.Context, refreshClaims *jwt.RefreshClaims, ipAddress, userAgent string, tenantID int64, guard mintGuard) (*auth.Account, *auth.Token, bool, error) {
	var dbToken *auth.Token
	var account *auth.Account
	var newToken *auth.Token
	var recovered bool
	var rejectAfterCommit error

	err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		var err error
		now := time.Now()

		// Resolve the owning account without a token lock, then lock the account
		// first. Login uses the same account -> token order when enforcing the
		// session cap, preventing a login/refresh deadlock.
		unlockedToken, err := s.repos.Token.FindByToken(ctx, refreshClaims.Token)
		if err != nil {
			if modelBase.IsNoRows(err) {
				s.logRefreshDecision("refresh_session_rejected", "token_not_found", refreshClaims.ID, refreshClaims.TenantID)
				return &AuthError{Op: "get token", Err: ErrTokenNotFound}
			}
			return fmt.Errorf("find refresh token owner: %w", err)
		}
		account, err = s.fetchAndValidateAccountForUpdate(ctx, unlockedToken.AccountID, ipAddress, userAgent)
		if err != nil {
			reason := "account_lookup_failed"
			if errors.Is(err, ErrAccountInactive) {
				reason = "account_inactive"
			} else if errors.Is(err, ErrAccountNotFound) {
				reason = "account_not_found"
			}
			s.logRefreshDecision("refresh_session_rejected", reason, refreshClaims.ID, refreshClaims.TenantID)
			return err
		}

		// Authorization runs HERE — after the account row is locked, before
		// the rotation writes anything. Doing it after the transaction
		// committed (where the school checks used to live) meant a
		// revocation could land in between and still be answered with a
		// freshly minted token; it also meant a legitimate refusal had
		// already consumed the caller's refresh token.
		if guard != nil {
			if err := guard(ctx, account); err != nil {
				return err
			}
		}

		dbToken, err = s.repos.Token.FindByTokenForUpdate(ctx, refreshClaims.Token)
		if err != nil {
			if modelBase.IsNoRows(err) {
				s.logRefreshDecision("refresh_session_rejected", "token_not_found", refreshClaims.ID, refreshClaims.TenantID)
				return &AuthError{Op: "get token", Err: ErrTokenNotFound}
			}
			return fmt.Errorf("find refresh token: %w", err)
		}

		if dbToken.AccountID != int64(refreshClaims.ID) || (refreshClaims.TenantID > 0 && dbToken.TenantID != refreshClaims.TenantID) {
			if err := s.deleteFamilyWithAudit(ctx, dbToken, "claim_mismatch", ipAddress, userAgent); err != nil {
				return fmt.Errorf("revoke mismatched refresh-token family: %w", err)
			}
			rejectAfterCommit = ErrInvalidToken
			s.logRefreshDecision("refresh_session_rejected", "claim_mismatch", refreshClaims.ID, refreshClaims.TenantID)
			return nil
		}

		if now.After(dbToken.Expiry) {
			if err := s.deleteFamilyWithAudit(ctx, dbToken, "token_expired", ipAddress, userAgent); err != nil {
				return fmt.Errorf("delete expired refresh-token family: %w", err)
			}
			rejectAfterCommit = ErrTokenExpired
			s.logRefreshDecision("refresh_session_rejected", "token_expired", refreshClaims.ID, refreshClaims.TenantID)
			return nil
		}

		dbToken, recovered, err = s.resolveRefreshHandoff(ctx, dbToken, now)
		if err != nil {
			if errors.Is(err, ErrInvalidToken) {
				if revokeErr := s.deleteFamilyWithAudit(ctx, dbToken, "replay_detected", ipAddress, userAgent); revokeErr != nil {
					return fmt.Errorf("revoke replayed refresh-token family: %w", revokeErr)
				}
				rejectAfterCommit = ErrInvalidToken
				s.logRefreshDecision("refresh_session_rejected", "replay_detected", refreshClaims.ID, refreshClaims.TenantID)
				return nil
			}
			return err
		}
		if dbToken.FamilyID != "" {
			latestToken, latestErr := s.repos.Token.GetLatestTokenInFamily(ctx, dbToken.FamilyID)
			if latestErr != nil {
				return fmt.Errorf("inspect refresh-token family: %w", latestErr)
			}
			if latestToken != nil && latestToken.Generation > dbToken.Generation {
				if revokeErr := s.deleteFamilyWithAudit(ctx, dbToken, "lineage_mismatch", ipAddress, userAgent); revokeErr != nil {
					return fmt.Errorf("revoke inconsistent refresh-token family: %w", revokeErr)
				}
				rejectAfterCommit = ErrInvalidToken
				s.logRefreshDecision("refresh_session_rejected", "lineage_mismatch", refreshClaims.ID, refreshClaims.TenantID)
				return nil
			}
		}

		// Backward compat: pre-migration tokens have tenantID=0 in their claims.
		// Resolve from account_tenants so the rotated token gets the correct value.
		effectiveTenantID := tenantID
		if effectiveTenantID == 0 {
			resolved, _, resolveErr := s.resolveAccountTenant(ctx, account.ID, "")
			if resolveErr != nil {
				return resolveErr
			}
			effectiveTenantID = resolved
		}

		if recovered {
			newToken = dbToken
		} else {
			// Create and persist new token with resolved tenant.
			newToken, err = s.createAndPersistNewToken(ctx, dbToken, account.ID, effectiveTenantID, refreshClaims.Scope, now)
			if err != nil {
				return err
			}
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return s.repos.Account.Update(ctx, account)
	})

	if err != nil {
		return nil, nil, false, &AuthError{Op: "refresh transaction", Err: err}
	}
	if rejectAfterCommit != nil {
		return nil, nil, false, &AuthError{Op: "refresh transaction", Err: rejectAfterCommit}
	}

	return account, newToken, recovered, nil
}

// fetchAndValidateAccountForUpdate locks the account before refresh locks its
// token row. This preserves the account-first order used by login.
func (s *Service) fetchAndValidateAccountForUpdate(ctx context.Context, accountID int64, ipAddress, userAgent string) (*auth.Account, error) {
	account, err := s.repos.Account.FindByIDForUpdate(ctx, accountID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
		}
		return nil, &AuthError{Op: opGetAccount, Err: fmt.Errorf("account lookup failed: %w", err)}
	}
	if !account.Active {
		if ipAddress != "" {
			s.logAuthEvent(ctx, account.ID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Account inactive")
		}
		return nil, &AuthError{Op: "check account status", Err: ErrAccountInactive}
	}
	return account, nil
}

// resolveRefreshHandoff follows a bounded, validated successor chain. A chain
// exists only when a previous rotation committed but its response cookie may
// not have reached the browser. Outside the grace period, reuse is replay.
func (s *Service) resolveRefreshHandoff(ctx context.Context, presented *auth.Token, now time.Time) (*auth.Token, bool, error) {
	current := presented
	// The request proves possession of the token it presented. Later hops are
	// trusted only after their persisted family/account/tenant/generation links
	// have been validated; they were rotated under different access tokens.
	proofValidated := false
	for hop := 0; hop < rotation.MaxRecoveryHops; hop++ {
		if current.RotatedAt == nil {
			return current, current.ID != presented.ID, nil
		}
		if current.ReplacementToken == nil || current.RotatedAt.After(now) || now.Sub(*current.RotatedAt) > rotation.RecoveryGrace {
			return current, false, ErrInvalidToken
		}
		if !proofValidated {
			if !rotation.MatchesRecoveryProof(ctx, current.RecoveryProofHash) {
				return current, false, ErrInvalidToken
			}
			proofValidated = true
		}

		next, err := s.repos.Token.FindByTokenForUpdate(ctx, *current.ReplacementToken)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return current, false, ErrInvalidToken
			}
			return current, false, fmt.Errorf("follow refresh-token handoff: %w", err)
		}
		// TenantID 0 is the documented pre-tenant-claim legacy state. Its
		// first successor is allowed to carry the resolved tenant; all modern
		// lineage must remain pinned to the same tenant.
		if next.FamilyID != current.FamilyID || next.AccountID != current.AccountID || (current.TenantID != 0 && next.TenantID != current.TenantID) || next.Generation != current.Generation+1 {
			return current, false, ErrInvalidToken
		}
		current = next
	}
	return current, false, ErrInvalidToken
}

// fetchAndValidateAccount retrieves account and checks if active
func (s *Service) fetchAndValidateAccount(ctx context.Context, accountID int64, ipAddress, userAgent string) (*auth.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
		}
		// A transient database failure is not evidence that the account was
		// deleted. Preserve the original error so the refresh endpoint returns
		// 5xx and the frontend retries instead of destroying a valid session.
		return nil, &AuthError{Op: opGetAccount, Err: fmt.Errorf("account lookup failed: %w", err)}
	}

	if !account.Active {
		if ipAddress != "" {
			s.logAuthEvent(ctx, account.ID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Account inactive")
		}
		return nil, &AuthError{Op: "check account status", Err: ErrAccountInactive}
	}

	return account, nil
}

// createAndPersistNewToken creates a successor and persists the bounded
// predecessor handoff atomically.
func (s *Service) createAndPersistNewToken(ctx context.Context, oldToken *auth.Token, accountID int64, tenantID int64, scope string, now time.Time) (*auth.Token, error) {
	newToken := &auth.Token{
		Token:       uuid.Must(uuid.NewV4()).String(),
		AccountID:   accountID,
		Expiry:      now.Add(s.jwtRefreshExpiry),
		Mobile:      oldToken.Mobile,
		Identifier:  oldToken.Identifier,
		FamilyID:    oldToken.FamilyID,
		Generation:  oldToken.Generation + 1,
		PortalScope: persistedPortalScope(scope),
	}

	// Set tenant ID from refresh claims (not from context — refresh is a public route)
	newToken.SetTenantID(tenantID)

	if err := s.repos.Token.Create(ctx, newToken); err != nil {
		return nil, err
	}
	if err := s.repos.Token.MarkRotated(ctx, oldToken.ID, newToken.Token, rotation.RecoveryProofHash(ctx), now); err != nil {
		return nil, err
	}
	if err := s.repos.Token.DeleteExpiredRotatedForAccount(ctx, accountID, now); err != nil {
		return nil, err
	}

	return newToken, nil
}

func (s *Service) logRefreshDecision(event, reason string, accountID int, tenantID int64) {
	s.getLogger().Warn(event,
		slog.String("reason", reason),
		slog.Int("account_id", accountID),
		slog.Int64("tenant_id", tenantID),
	)
}

// refreshResult carries token pair through singleflight
type refreshResult struct {
	accessToken  string
	refreshToken string
}

func refreshSingleflightKey(refreshToken string, proofHash []byte) string {
	return refreshToken + "\x00" + hex.EncodeToString(proofHash)
}

// RefreshTokenWithAudit generates new token pair from a refresh token with audit logging.
// Concurrent calls with the same refresh token are deduplicated via singleflight.
func (s *Service) RefreshTokenWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) (string, string, error) {
	// Use the caller's context so cancellation propagates to the DB transaction.
	// If the first caller disconnects (e.g. frontend 5s timeout), the transaction
	// rolls back and the old refresh token is preserved — callers retry safely.
	// NOT using WithoutCancel: a transient retry for shared callers is far better
	// than completing a token rotation the client never receives (permanent logout).
	sfCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// A refresh token alone is not enough to join another caller's in-flight
	// recovery. Bind singleflight to the independent proof hash as well, or a
	// wrong/missing-proof request could receive a legitimate caller's result
	// without reaching the constant-time handoff check.
	proofHash := rotation.RecoveryProofHash(ctx)
	singleflightKey := refreshSingleflightKey(refreshTokenStr, proofHash)
	v, err, shared := s.refreshSF.Do(singleflightKey, func() (any, error) {
		return s.doRefreshTokenWithAudit(sfCtx, refreshTokenStr, ipAddress, userAgent)
	})
	if shared {
		s.getLogger().Info("concurrent_refresh_deduplicated", "shared", true)
	}
	if err != nil {
		return "", "", err
	}
	r := v.(*refreshResult)
	return r.accessToken, r.refreshToken, nil
}

// doRefreshTokenWithAudit performs the actual token refresh (called via singleflight)
func (s *Service) doRefreshTokenWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) (*refreshResult, error) {
	// Parse and validate refresh token claims
	refreshClaims, err := s.parseRefreshTokenClaims(refreshTokenStr)
	if err != nil {
		s.logRefreshDecision("refresh_session_rejected", "invalid_format", 0, 0)
		return nil, err
	}

	// Re-validate tenant access if tenant_id was in the refresh token
	if refreshClaims.TenantID > 0 {
		if err := s.validateTenantAccess(ctx, refreshClaims); err != nil {
			return nil, err
		}
	}

	// A school token without a school is dead on arrival: SchoolMiddleware
	// refuses tenant_id=0 on every request, and nothing downstream can pin the
	// session to a school either — the rotation would fall back to the
	// account's default mapping and mint a successor for a school this session
	// never proved access to. Refusing BEFORE the rotation matters as much as
	// refusing at all: this used to be caught only after the transaction had
	// committed, so a token nothing accepts was answered by burning the
	// caller's refresh token.
	if refreshClaims.Scope == tenant.ScopeSchool && refreshClaims.TenantID <= 0 {
		s.logRefreshDecision("refresh_session_rejected", "school_token_without_tenant", refreshClaims.ID, refreshClaims.TenantID)
		return nil, &AuthError{Op: "refresh school session", Err: ErrTenantAccessDenied}
	}

	// Every refresh carries its authorization AND its claims INTO the rotation
	// transaction. For the school scope that is the whole portal's footing —
	// account liveness, school liveness, an active mapping, the school-portal
	// role — re-checked under the account lock, so a revocation that commits
	// while the refresh is in flight either loses the race outright or blocks
	// until the rotation is done and cuts the NEXT one. Checking after the
	// rotation (as this path used to) had both failure modes: a token minted
	// for an account whose access had just been revoked, and a rejected refresh
	// that had nonetheless burned the caller's refresh token.
	//
	// The claims payload is assembled in the same transaction for the second
	// half of that argument, and that half is not school-specific — hence a
	// guard on EVERY path, not just the school one. Loading claims afterwards
	// left a window no handoff could repair: roles, permissions or the person
	// lookup failing there (a soft-deleted school, a DB blip) returns an error
	// to a caller whose refresh token the rotation has already consumed, and
	// whose successor it never received. With no recovery proof in hand the
	// retry cannot reach the handoff and may be read as replay. Inside the
	// transaction that outcome cannot exist: either the rotation and the claims
	// commit together, or the transaction rolls back and the presented refresh
	// token is still the caller's.
	var (
		guard    mintGuard
		metadata *accountMetadata
	)
	if refreshClaims.Scope == tenant.ScopeSchool {
		guard = s.schoolRefreshMintGuard(int64(refreshClaims.ID), refreshClaims.TenantID, &metadata)
	} else {
		guard = s.refreshClaimsGuard(refreshClaims.Scope, refreshClaims.TenantID, &metadata)
	}

	// Validate and refresh token in transaction (pass tenant from old JWT for the new token)
	account, newToken, recovered, err := s.refreshTokenInTransaction(ctx, refreshClaims, ipAddress, userAgent, refreshClaims.TenantID, guard)
	if err != nil {
		return nil, err
	}
	if recovered {
		s.getLogger().Info("refresh_rotation_recovered",
			slog.Int("account_id", refreshClaims.ID),
			slog.Int64("tenant_id", refreshClaims.TenantID),
			slog.Int("generation", newToken.Generation),
		)
	}

	// The claims are already assembled: the guard built them inside the
	// rotation transaction (see above), so there is deliberately nothing left
	// to load here and no second chance to fail after the rotation committed.
	// A nil payload past a successful rotation would mean a guard returned nil
	// without filling it — impossible today, and an internal error rather than
	// a token minted from an empty claims struct.
	if metadata == nil {
		return nil, &AuthError{Op: "refresh session", Err: fmt.Errorf("refresh claims payload missing after rotation")}
	}

	// Build JWT claims from account and metadata
	appClaims, newRefreshClaims := s.buildJWTClaims(account, newToken, metadata, account.Email)

	// Generate token pair and log success as token refresh.
	//
	// The whole refresh validates against refreshClaims.TenantID, so the audit
	// event must be filed there too. The incoming context has no tenant (the
	// refresh route is pre-authentication), and logAuthEvent then falls back to
	// the account's FIRST active mapping — for anyone mapped to several schools
	// that routinely attributes the refresh to the wrong school.
	auditCtx := ctx
	if refreshClaims.TenantID > 0 {
		auditCtx = tenant.WithTenantID(ctx, refreshClaims.TenantID)
	}
	accessToken, refreshToken, err := s.generateAndLogTokens(auditCtx, account.ID, appClaims, newRefreshClaims, ipAddress, userAgent, audit.EventTypeTokenRefresh)
	if err != nil {
		return nil, err
	}
	return &refreshResult{accessToken: accessToken, refreshToken: refreshToken}, nil
}

// refreshClaimsGuard assembles the claims payload of a NON-school refresh from
// inside the rotation transaction — the tenant and parent scopes.
//
// It exists for atomicity, not for authorization: the tenant scope settles its
// access in validateTenantAccess before any of this runs. What the guard buys
// is that a claims load which FAILS cannot leave the caller stranded. Run after
// the transaction (where this used to live), a failing role, permission or
// person lookup returned an error on a rotation that had already committed:
// the presented refresh token was consumed, the successor was never handed
// back, and a retry without a recovery proof cannot reach the handoff — it
// looks like replay and can take the whole token family down. In here the two
// halves share one transaction, so an error rolls the rotation back and the
// caller still holds the token it presented.
//
// The scope decision itself is unchanged. Parent-scope refresh tokens must
// round-trip as parent tokens: loadAccountMetadataForTenantInTx returns
// tenant-scope metadata (scope="", tenant_id pinned), so a naive refresh would
// silently demote a parent JWT to a tenant JWT — that token then fails the
// parents-portal ParentMiddleware on the very next request and the parent
// dashboard gets stuck on the auth-guard loading state. It is detected via:
//   - the explicit scope claim (new tokens, see RefreshClaims.Scope), OR
//   - backward-compat: the account is guardian-only at the refresh tenant
//     (old in-flight refresh tokens issued before Scope was added).
//
// That backward-compat probe now PROPAGATES its errors instead of reading a
// failed role load as "not guardian-only" (isGuardianOnlyAccount's contract,
// which the post-rotation caller could afford because the very next step
// reloaded the same roles and failed there). Inside the guard the distinction
// is real: guessing here would mint a tenant JWT for a guardian.
func (s *Service) refreshClaimsGuard(scope string, tenantID int64, out **accountMetadata) mintGuard {
	return func(ctx context.Context, account *auth.Account) error {
		if scope == tenant.ScopeParent {
			*out = s.buildParentMetadata(account)
			return nil
		}

		guardianOnly, err := s.isGuardianOnlyAccountInTx(ctx, account, tenantID)
		if err != nil {
			return err
		}
		if guardianOnly {
			*out = s.buildParentMetadata(account)
			return nil
		}

		// Refresh preserves the tenant of the existing refresh token — never
		// re-resolve via the default fallback, which could silently switch a
		// multi-school account to a different school.
		metadata, err := s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
		if err != nil {
			return err
		}
		*out = metadata
		return nil
	}
}

// validateTenantAccess ensures the account still has active access to the tenant from the refresh token
// and that the tenant (school) has not been soft-deleted.
//
// The school.deleted_at check is critical: SoftDeleteSchool bulk-deletes auth.tokens by tenant_id,
// but a concurrent refreshTokenInTransaction can insert a new token after the DELETE runs
// (the DELETE only sees rows visible in its statement snapshot). This check catches that race —
// even if a new token slips through, the next refresh is rejected because the school is deleted.
func (s *Service) validateTenantAccess(ctx context.Context, claims *jwt.RefreshClaims) error {
	exists, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, int64(claims.ID), claims.TenantID)
	if err != nil {
		s.getLogger().Error("refresh_session_validation_failed",
			slog.String("reason", "tenant_lookup_error"),
			slog.Int("account_id", claims.ID),
			slog.Int64("tenant_id", claims.TenantID),
			slog.Any("error", err),
		)
		return &AuthError{Op: "validate tenant access", Err: fmt.Errorf("tenant access lookup failed: %w", err)}
	}
	if !exists {
		s.logRefreshDecision("refresh_session_rejected", "tenant_access_revoked", claims.ID, claims.TenantID)
		return &AuthError{Op: "validate tenant access", Err: ErrTenantAccessDenied}
	}

	// Check that the school itself has not been soft-deleted.
	school, err := s.repos.School.FindByID(ctx, claims.TenantID)
	if err != nil {
		// Distinguish "not found" from transient DB errors.
		// Not-found → tenant genuinely gone → terminal 401/404.
		// Anything else (timeout, connection reset, etc.) → 500 so the
		// frontend can retry instead of force-logging out the user.
		if errors.Is(err, sql.ErrNoRows) {
			s.logRefreshDecision("refresh_session_rejected", "tenant_not_found", claims.ID, claims.TenantID)
			return &AuthError{Op: "validate tenant access", Err: ErrTenantNotFound}
		}
		s.getLogger().Error("failed to look up school during refresh validation",
			slog.Int("account_id", claims.ID),
			slog.Int64("tenant_id", claims.TenantID),
			slog.Any("error", err),
		)
		return &AuthError{Op: "validate tenant access", Err: fmt.Errorf("school lookup failed: %w", err)}
	}
	if school == nil || school.IsDeleted() {
		s.logRefreshDecision("refresh_session_rejected", "tenant_deleted", claims.ID, claims.TenantID)
		return &AuthError{Op: "validate tenant access", Err: ErrTenantNotFound}
	}

	// An inactive school cannot be logged into on ANY portal — every resolver
	// (resolveAccountTenantBySlug, resolveAccountTenantDefault, the school
	// portal-tenant finder) skips it. Refusing it here too keeps a running
	// session from outliving the switch-off, and it has to happen HERE rather
	// than after rotation: the later liveness gate only ran once the refresh
	// token had already been consumed, so a school deactivated mid-session
	// answered its next refresh with an error AND destroyed the token that
	// would have let the client retry.
	if !school.Active {
		s.logRefreshDecision("refresh_session_rejected", "tenant_inactive", claims.ID, claims.TenantID)
		return &AuthError{Op: "validate tenant access", Err: ErrTenantNotFound}
	}

	return nil
}

// LogoutWithAudit invalidates the presented refresh-token family with audit
// logging. Other devices and other portals keep their sessions.
//
// Uses WithAdminTx (BYPASSRLS) because logout is a pre-deauthentication flow.
// auth.tokens has RLS enabled — without setting app.current_tenant_id, the
// tenant isolation policy silently filters out all rows, causing FindByToken
// to return "not found" and tokens to never actually be deleted.
func (s *Service) LogoutWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) error {
	// Parse JWT refresh token
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse refresh token claims
	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	// Use WithAdminTx to bypass RLS on auth.tokens (same pattern as refreshTokenInTransaction).
	err = tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		// Get token from database to find the account ID
		dbToken, err := s.repos.Token.FindByToken(ctx, refreshClaims.Token)
		if err != nil {
			// Token not found, consider logout successful
			return nil
		}

		if err := s.deleteFamilyWithAudit(ctx, dbToken, "logout", ipAddress, userAgent); err != nil {
			s.getLogger().Warn("failed to delete refresh-token family during logout",
				slog.Int64("account_id", dbToken.AccountID),
				slog.Any("error", err),
			)
			return &AuthError{Op: "delete token family with audit", Err: err}
		}

		// Log successful logout against the school the session actually
		// belonged to. /auth/logout is a pre-deauthentication route with no
		// tenant in context, and logAuthEvent then falls back to the account's
		// FIRST active mapping — for a Lehrkraft or a caregiver mapped to
		// several schools that files the logout under a school they were never
		// logged into. The token row carries the tenant the session was minted
		// for; the claims are the fallback for pre-tenant-claim legacy rows.
		auditCtx := ctx
		switch {
		case dbToken.TenantID > 0:
			auditCtx = tenant.WithTenantID(ctx, dbToken.TenantID)
		case refreshClaims.TenantID > 0:
			auditCtx = tenant.WithTenantID(ctx, refreshClaims.TenantID)
		}

		// Log successful logout
		if ipAddress != "" {
			s.logAuthEvent(auditCtx, dbToken.AccountID, audit.EventTypeLogout, true, ipAddress, userAgent, "")
		}

		return nil
	})
	// Family-scoped logout leaves other sessions signed in, so their staff
	// push subscriptions stay. The logging-out device unregisters itself
	// through the frontend before this call. Account-wide push cleanup lives
	// on full-session revocation (password reset, deactivation, role change).
	return err
}

// ChangePassword updates an account's password
func (s *Service) ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error {
	// Get account
	account, err := s.repos.Account.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	// Verify current password
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return &AuthError{Op: "verify password", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(currentPassword, *account.PasswordHash)
	if err != nil || !valid {
		return &AuthError{Op: "verify password", Err: ErrInvalidCredentials}
	}

	// Validate new password
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return &AuthError{Op: "validate password", Err: err}
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return &AuthError{Op: opHashPassword, Err: err}
	}

	// Update password
	account.PasswordHash = &passwordHash
	if err := s.repos.Account.Update(ctx, account); err != nil {
		return &AuthError{Op: opUpdateAccount, Err: err}
	}

	return nil
}

// GetAccountByID retrieves an account by ID
func (s *Service) GetAccountByID(ctx context.Context, id int) (*auth.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}
	return account, nil
}

// Helper functions

// extractClaims extracts all claims from a jwt token into a map
func extractClaims(token jwx.Token) map[string]interface{} {
	claims := make(map[string]interface{})

	// Extract all claims via Keys() + Get()
	for _, k := range token.Keys() {
		var v interface{}
		if err := token.Get(k, &v); err == nil {
			claims[k] = v
		}
	}

	return claims
}

// getAccountPermissions retrieves all permissions for an account (both direct and role-based)
// Uses the repository's FindByAccountID which combines direct and role-based permissions in a single query
func (s *Service) getAccountPermissions(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	// FindByAccountID already uses a CTE to combine direct and role-based permissions
	// in a single query, avoiding N+1 queries
	return s.repos.Permission.FindByAccountID(ctx, accountID)
}
