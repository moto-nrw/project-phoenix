package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// isSchoolPortalRole reports whether the role grants access to the school
// portal (#2207). Today that is exactly the lehrkraft system role; a future
// Schulleitung role widens this predicate without touching the login flow,
// the refresh path, or the error codes — they are all named after the
// portal, not the role.
func isSchoolPortalRole(role *authModels.Role) bool {
	return IsLehrkraftSystemRole(role)
}

// IsSchoolPortalOnlyForTenant reports whether EVERY role the account holds at
// this school is a school-portal role. Such an account has no reachable
// surface in the OGS tenant portal since the cutover (#2207 PR 3) removed the
// tenant-side class-day mount, so the tenant login refuses it and points at
// moto schule.
//
// Mirrors IsGuardianOnlyForTenant, with one deliberate difference: it works on
// the loaded role objects rather than on names, because isSchoolPortalRole
// requires the SYSTEM lehrkraft role. A tenant-scoped custom role that merely
// happens to be called "Lehrkraft" carries arbitrary permissions and must keep
// its tenant-portal access — a name match would lock such an account out of
// both portals at once (the school login requires the system role too).
//
// An empty role set is NOT school-portal-only: an account with no roles at all
// is a different problem and stays on the existing path.
func IsSchoolPortalOnlyForTenant(roles []*authModels.Role) bool {
	if len(roles) == 0 {
		return false
	}
	for _, role := range roles {
		if !isSchoolPortalRole(role) {
			return false
		}
	}
	return true
}

// LoginSchoolWithMFAGate authenticates a school-portal user (Lehrkraft) and
// issues a school-scope JWT bound to the first school where the account
// holds a school-portal role. Refuses accounts without such a role on any
// active tenant mapping (ErrAccountNoSchoolPortalRole → 403).
//
// Unlike the parents portal login, school tokens ARE tenant-bound: the
// class-day surface runs under RLS, so the resolved school is pinned into
// tenant_id exactly like a tenant login. Accounts mapped to several schools
// switch via SwitchSchool.
//
// The MFA gate mirrors LoginWithMFAGate — MFA requirement is a per-tenant
// setting and applies to the account, not the portal, so moving the login
// surface must not silently drop the second factor:
//   - required + enrolled → email-code challenge with the school challenge
//     scope; redeemable only at the school verify endpoint.
//   - required + not enrolled → school-scope enrollment token for the
//     school enrollment surface (/school/auth/mfa/enroll/*). Confirming
//     there mints school tokens, so the enrollment detour can never turn
//     a school login into a tenant session.
func (s *Service) LoginSchoolWithMFAGate(
	ctx context.Context,
	email, password, ipAddress, userAgent, trustedDeviceCookie string,
) (*LoginResult, error) {
	return s.loginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, "")
}

// LoginSchoolAtTenantWithMFAGate pins a school-portal login to the selected
// tenant. It is used only by a tenant-to-school handoff; direct school logins
// keep the established first-eligible-school behavior above.
func (s *Service) LoginSchoolAtTenantWithMFAGate(
	ctx context.Context,
	email, password, ipAddress, userAgent, trustedDeviceCookie, targetTenantSlug string,
) (*LoginResult, error) {
	return s.loginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, targetTenantSlug)
}

func (s *Service) loginSchoolWithMFAGate(
	ctx context.Context,
	email, password, ipAddress, userAgent, trustedDeviceCookie, targetTenantSlug string,
) (*LoginResult, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	portalTenantID := int64(0)
	hasPortalRole := false
	if targetTenantSlug != "" {
		metadata, metadataErr := s.loadAccountMetadata(ctx, account, targetTenantSlug)
		if metadataErr != nil {
			return nil, metadataErr
		}
		portalTenantID = metadata.tenantID
		for _, role := range account.Roles {
			if isSchoolPortalRole(role) {
				hasPortalRole = true
				break
			}
		}
		if !hasPortalRole {
			s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at requested school")
			return nil, &AuthError{Op: "school login", Err: ErrAccountNoSchoolPortalRole}
		}
	} else {
		var resolvedTenantID int64
		var findErr error
		hasPortalRole, resolvedTenantID, findErr = s.findSchoolPortalTenantForAccount(ctx, account.ID)
		if findErr != nil {
			return nil, &AuthError{Op: "school login: enumerate tenants", Err: findErr}
		}
		if hasPortalRole {
			portalTenantID = resolvedTenantID
		}
	}
	if !hasPortalRole || portalTenantID == 0 {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at any school")
		return nil, &AuthError{Op: "school login", Err: ErrAccountNoSchoolPortalRole}
	}

	// Runs the liveness gates that decide the RESPONSE and hydrates
	// account.Roles — the MFA IsRequired check below reads them. What the JWT
	// is built from is deliberately NOT taken from here: the payload this
	// returns is a pre-transaction snapshot, and the mint guard below reloads
	// it under its own lock.
	if _, err := s.loadSchoolMetadataForTenant(ctx, account, portalTenantID); err != nil {
		return nil, err
	}

	// MFA gate — same fail-closed semantics as LoginWithMFAGate: on infra
	// errors we refuse THIS login with 503 instead of silently dropping to
	// "not required".
	//
	// This verdict decides which BRANCH the login takes (challenge, enrollment,
	// or straight to the token). It is deliberately NOT carried into the mint:
	// the policy resolved here is a snapshot of two independently moving inputs
	// (the account's roles and the school's mfa_mode), so the guard re-resolves
	// it inside its own transaction instead — see schoolMintGuard.
	mfaRequired := false
	if s.mfaService != nil {
		mfaPolicy, policyErr := s.mfaService.ResolvePolicy(ctx, account.ID, portalTenantID)
		if policyErr != nil {
			return nil, &AuthError{Op: "check mfa required", Err: ErrMFAStatusUnavailable}
		}
		mfaRequired = mfaPolicy.RequiredFor(account)
	}

	enrolled := false
	if s.mfaService != nil {
		enrolled, err = s.mfaService.HasEnrollment(ctx, account.ID)
		if err != nil {
			return nil, &AuthError{Op: "check mfa enrollment", Err: ErrMFAStatusUnavailable}
		}
	}

	if mfaRequired && !enrolled {
		return s.schoolMFAEnrollmentResult(account, portalTenantID)
	}

	trustedDeviceVerified := false
	if mfaRequired && enrolled && trustedDeviceCookie != "" && s.mfaService != nil {
		ok, _ := s.mfaService.VerifyTrustedDevice(ctx, account.ID, portalTenantID, trustedDeviceCookie)
		trustedDeviceVerified = ok
	}

	if mfaRequired && enrolled && !trustedDeviceVerified {
		return s.schoolMFAChallengeResult(ctx, account, portalTenantID, ipAddress)
	}

	// The portal-role lookup above ran minutes ago in wall-clock terms once MFA
	// or a trusted-device check sat in between — and in its own, already
	// committed transaction either way. The guard re-checks school liveness,
	// membership and portal role INSIDE the transaction that writes the token,
	// so a revocation can no longer land in the gap, and it assembles the
	// claims there too — see schoolMintGuard.
	//
	// It also re-decides the MFA gate, but only where the gate concluded "no
	// second factor needed": both inputs of that verdict can move in the gap —
	// a role granted (`required_admins` flipping to true) or the school's
	// mfa_mode switched on — and either would otherwise hand out a session that
	// never saw a challenge. When a trusted device or an email code satisfied
	// an MFA requirement, the factor IS proven and nothing is re-decided.
	guardOpts := []schoolMintOption{}
	if !mfaRequired && s.mfaService != nil {
		guardOpts = append(guardOpts, withMFAGateRecheck(s.freshSchoolMFAPolicy(account.ID, portalTenantID)))
	}
	var metadata *accountMetadata
	token, err := s.createRefreshTokenWithRetryGuarded(ctx, account, portalTenantID, tenant.ScopeSchool, s.schoolMintGuard(account.ID, portalTenantID, &metadata, guardOpts...))
	if errors.Is(err, errSchoolMFARequiredAtMint) {
		// Nothing was written — the guard aborted its own transaction. Send
		// the login down the branch it would have taken had the role been
		// there from the start.
		if !enrolled {
			return s.schoolMFAEnrollmentResult(account, portalTenantID)
		}
		return s.schoolMFAChallengeResult(ctx, account, portalTenantID, ipAddress)
	}
	if err != nil {
		return nil, wrapSchoolMintError("school login", err)
	}
	if metadata == nil {
		return nil, &AuthError{Op: "school login", Err: errMissingSchoolClaimsPayload}
	}
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, email)
	// Attribute the audit event to the school the portal-role lookup actually
	// resolved. Without the explicit tenant, logAuthEvent falls back to the
	// account's FIRST active mapping — for a Lehrkraft mapped to several
	// schools that is routinely a different school than the one just logged
	// into, which silently files the login under the wrong tenant.
	accessToken, refreshToken, err := s.generateAndLogTokens(
		tenant.WithTenantID(ctx, portalTenantID),
		account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin,
	)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Status:       LoginStatusAuthenticated,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// IssueSchoolTokensForAuthenticatedAccount mints a school-scope token pair
// for an account whose identity was proven via a non-password channel — the
// school MFA verify endpoint after a successful email-code exchange. It
// re-validates that the account still holds a school-portal role at the
// tenant carried in the challenge, so a role revoked mid-challenge can not
// be laundered into a school session.
func (s *Service) IssueSchoolTokensForAuthenticatedAccount(
	ctx context.Context,
	accountID, tenantID int64,
	ipAddress, userAgent string,
) (string, string, error) {
	account, err := s.loadActiveAccountForSchoolMint(ctx, "issue school tokens", accountID)
	if err != nil {
		return "", "", err
	}

	// Liveness gates first — they decide which sentinel (and hence which
	// status code) the caller sees. The claims themselves come from the guard.
	if _, err := s.loadSchoolMetadataForTenant(ctx, account, tenantID); err != nil {
		return "", "", err
	}
	hasRole, err := s.hasSchoolPortalRoleAtTenant(ctx, accountID, tenantID)
	if err != nil {
		return "", "", err
	}
	if !hasRole {
		s.logFailedLogin(tenant.WithTenantID(ctx, tenantID), account.ID, ipAddress, userAgent, "No school portal role at school token issue")
		return "", "", &AuthError{Op: "issue school tokens", Err: ErrAccountNoSchoolPortalRole}
	}

	var metadata *accountMetadata
	token, err := s.createRefreshTokenWithRetryGuarded(ctx, account, tenantID, tenant.ScopeSchool, s.schoolMintGuard(account.ID, tenantID, &metadata))
	if err != nil {
		return "", "", wrapSchoolMintError("issue school tokens", err)
	}
	if metadata == nil {
		return "", "", &AuthError{Op: "issue school tokens", Err: errMissingSchoolClaimsPayload}
	}

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)
	// Same explicit attribution as the password login — the challenge carries
	// the school, so the audit event must not fall back to the first mapping.
	return s.generateAndLogTokens(
		tenant.WithTenantID(ctx, metadata.tenantID),
		account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin,
	)
}

// SwitchSchool re-authenticates a school-portal session to a different
// school — the school-scope sibling of SwitchTenant for Lehrkraft accounts
// mapped to several schools. The target is resolved by slug, must be an
// active mapping, and the account must hold a school-portal role THERE:
// being a Lehrkraft at school A and a caregiver at school B does not open
// school B's portal view.
//
// ipAddress and userAgent are threaded through from the request because
// generateAndLogTokens skips the audit write entirely when the IP is empty —
// passing "" here silently means "no tenant_switch event was ever recorded",
// which is the one thing this event exists for.
//
// No MFA gate, and therefore no MFA re-check in the guard: the caller already
// holds a school session whose second factor was settled at login, exactly as
// SwitchTenant treats a tenant session. Making the target school's mfa_mode
// re-challenge on every switch is a policy decision for the portal, not
// something to smuggle in through the mint guard.
func (s *Service) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	account, err := s.loadActiveAccountForSchoolMint(ctx, "switch school", accountID)
	if err != nil {
		return "", "", err
	}

	// Slug → school resolution incl. active-mapping check, inside an admin
	// tx (same RLS constraints as the login flows — there is no tenant
	// transaction yet).
	var targetTenantID int64
	if err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		tenantID, _, resolveErr := s.resolveAccountTenantBySlug(adminCtx, accountID, tenantSlug)
		if resolveErr != nil {
			return resolveErr
		}
		targetTenantID = tenantID
		return nil
	}); err != nil {
		return "", "", err
	}

	// Same split as the other two mint paths: gates here, claims in the guard.
	if _, err := s.loadSchoolMetadataForTenant(ctx, account, targetTenantID); err != nil {
		return "", "", err
	}
	hasRole, err := s.hasSchoolPortalRoleAtTenant(ctx, accountID, targetTenantID)
	if err != nil {
		return "", "", err
	}
	if !hasRole {
		return "", "", &AuthError{Op: "switch school", Err: ErrAccountNoSchoolPortalRole}
	}

	var metadata *accountMetadata
	token, err := s.createRefreshTokenWithRetryGuarded(ctx, account, targetTenantID, tenant.ScopeSchool, s.schoolMintGuard(accountID, targetTenantID, &metadata))
	if err != nil {
		return "", "", wrapSchoolMintError("switch school", err)
	}
	if metadata == nil {
		return "", "", &AuthError{Op: "switch school", Err: errMissingSchoolClaimsPayload}
	}

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)
	// The switch event belongs to the TARGET school, not to whatever mapping
	// happens to be oldest.
	return s.generateAndLogTokens(
		tenant.WithTenantID(ctx, metadata.tenantID),
		account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeTenantSwitch,
	)
}

// loadActiveAccountForSchoolMint fetches the account a non-password school
// mint path (MFA exchange, school switch) is about to issue a token for.
//
// Only a genuine "no such row" becomes ErrAccountNotFound — the handlers map
// that sentinel to 401 invalid_credentials. A dropped connection or a query
// timeout is NOT an authentication failure: collapsing the two told a Lehrkraft
// who had just entered a correct email code that their credentials were wrong,
// and hid a database outage behind a 401 nobody investigates. Everything else
// propagates and surfaces as a retryable 500.
func (s *Service) loadActiveAccountForSchoolMint(ctx context.Context, op string, accountID int64) (*authModels.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: op, Err: ErrAccountNotFound}
		}
		return nil, &AuthError{Op: op, Err: fmt.Errorf("look up account %d: %w", accountID, err)}
	}
	if account == nil {
		return nil, &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	if !account.Active {
		return nil, &AuthError{Op: op, Err: ErrAccountInactive}
	}
	return account, nil
}

// schoolMFAEnrollmentResult is the "MFA required but no factor enrolled"
// response: a school-scope enrollment token for /school/auth/mfa/enroll/*.
// Confirming there mints school tokens, so the enrollment detour can never
// turn a school login into a tenant session.
func (s *Service) schoolMFAEnrollmentResult(account *authModels.Account, tenantID int64) (*LoginResult, error) {
	enrollmentToken, err := s.tokenAuth.CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
		AccountID: account.ID,
		Scope:     jwt.MFAEnrollmentScopeSchool,
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

// schoolMFAChallengeResult is the "MFA required, factor enrolled" response: an
// email code plus a school-scope challenge token, redeemable only at the school
// verify endpoint.
func (s *Service) schoolMFAChallengeResult(ctx context.Context, account *authModels.Account, tenantID int64, ipAddress string) (*LoginResult, error) {
	challenge, err := s.mfaService.StartChallenge(ctx, account.ID, tenantID, jwt.MFAChallengeScopeSchool, ParseClientIP(ipAddress))
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

// errMissingSchoolClaimsPayload guards against a guard returning nil without
// filling its payload — impossible today, and an internal error rather than a
// token minted from an empty claims struct.
var errMissingSchoolClaimsPayload = errors.New("school claims payload missing after mint")

// errSchoolMFARequiredAtMint aborts a mint whose MFA gate has gone stale: the
// login concluded that no second factor was needed, and by the time the guard
// held the account lock the policy said otherwise. Never leaves the service —
// LoginSchoolWithMFAGate catches it and answers with the challenge (or the
// enrollment token) the login would have returned had the role been there from
// the start.
var errSchoolMFARequiredAtMint = errors.New("mfa became required before the school token was minted")

// schoolMintOption configures the extra checks a school mint guard settles
// inside the token transaction, on top of the four liveness facts every school
// mint shares.
type schoolMintOption func(*schoolMintChecks)

type schoolMintChecks struct {
	// resolveMFAPolicy, when set, re-reads the MFA policy inside the mint
	// transaction; the guard applies the result to the role set it read under
	// its lock. nil means the second factor is already settled for this mint —
	// proven by an email code or a trusted device, or (on refresh and school
	// switch) never gated here in the first place.
	resolveMFAPolicy mfaPolicyResolver
}

// mfaPolicyResolver re-reads the MFA policy from inside the mint transaction.
// A function rather than a resolved MFAPolicy: a policy resolved before the
// transaction is precisely the stale input this re-check exists to replace.
type mfaPolicyResolver func(ctx context.Context) (MFAPolicy, error)

// withMFAGateRecheck re-decides the MFA requirement inside the mint
// transaction. Pass it exactly where the pre-transaction gate concluded "no
// second factor needed" — that verdict is the one a concurrent role grant or
// mfa_mode change can invalidate.
func withMFAGateRecheck(resolve mfaPolicyResolver) schoolMintOption {
	return func(c *schoolMintChecks) { c.resolveMFAPolicy = resolve }
}

// freshSchoolMFAPolicy is the resolver the school login hands to the guard: it
// re-reads overrides and the school's mfa_mode on the mint transaction, past
// the request-scoped settings cache that would otherwise replay the value the
// login gate already read.
func (s *Service) freshSchoolMFAPolicy(accountID, tenantID int64) mfaPolicyResolver {
	return func(ctx context.Context) (MFAPolicy, error) {
		return s.mfaService.ResolvePolicyInTx(ctx, accountID, tenantID)
	}
}

// schoolMintGuard re-verifies the four facts a school session rests on and
// assembles the claims the JWT is built from, inside the transaction that
// persists the refresh token.
//
// The four facts:
//
//  1. the account itself is still active,
//  2. the school exists, is not soft-deleted, and is active,
//  3. the account's auth.account_tenants mapping is still `active`,
//  4. the account still holds a school-portal role at that school.
//
// Every school entry point checks the same facts up front — that is where the
// precise 401/403/404 responses come from — but those checks run in their own
// transactions, which have committed by the time the token is written. On the
// MFA path minutes can pass in between; even on the direct password path the
// checks and the write were two separate transactions. This guard closes that
// window: the rows are locked, so a concurrent deactivation or role
// revocation blocks until the mint commits and can never interleave between
// "may they?" and "here is the token".
//
// LOCK ORDER — auth.accounts FIRST, and deliberately FOR UPDATE rather than
// FOR SHARE. RevokeAccountTenantAccess (and the operator deactivation flows
// that follow it) take auth.accounts FOR UPDATE before they delete roles and
// deactivate the mapping. A guard that started at the school or mapping row
// and only reached the account later would be walking that chain backwards
// and could deadlock against a concurrent revocation. Taking the same row,
// in the same mode, first makes the two paths serialize instead: whoever
// wins the account row runs to completion. It also costs nothing — the very
// next statement of the mint (UpdateLastLogin in persistTokenInTransaction)
// takes that exclusive lock anyway; this just takes it a few statements
// earlier, before the decision instead of after it.
//
// CLAIMS — the guard also loads the payload the JWT is built from, into
// *claims, under the very same lock. Assembling them BEFORE the transaction
// (as the login, MFA-exchange and switch paths did) meant the roles and
// permissions in the access token came from a snapshot the guard's own checks
// had already superseded: a permission revoked while the login was in flight
// stayed in the minted token until it expired, even though the guard had
// re-read membership and portal role microseconds later. Anything a revocation
// can touch has to be read on the authorized side of the lock, not just the
// two facts the guard originally checked.
//
// PERMISSIONS — reading them under the account lock is not by itself enough.
// The lock serializes the paths that revoke a whole school access
// (RevokeAccountTenantAccess, staff offboarding), because those take the
// account row first; a bare permission revocation
// (RemovePermissionFromAccount, RemovePermissionFromRole) touches neither the
// account nor the role rows and would happily commit between this read and the
// token write, leaving the revoked permission in a JWT valid for the next
// AUTH_JWT_EXPIRY. LockAccountPermissionSourcesForTenant therefore takes FOR
// SHARE locks on the rows the permission set is derived from, in the same
// account → roles → permissions order every revocation path walks.
//
// MFA — with checks.resolveMFAPolicy set, the gate itself is re-decided here:
// the policy is re-READ inside this transaction (past the request-scoped
// settings cache) and applied to the role set read under the lock. Both of its
// inputs move independently. Re-applying the policy the login resolved would
// only have caught the role half; a school switching security.mfa_mode from off
// to required while the login is in flight would still have produced a session
// that never saw a challenge. An unreadable policy fails the mint closed.
//
// MFA LOCK ORDER — the re-read alone is still only a read: it orders nothing
// against a concurrent policy write, which can commit right after it. So the
// mode is pinned for the rest of the transaction (lockMFAPolicyForMint,
// shared), and the settings writer takes the exclusive side of that lock. That
// lock is taken as the FIRST statement of the mint, BEFORE auth.accounts:
// writing config.setting_values takes a foreign-key lock on the writing admin's
// own auth.accounts row, so a guard that grabbed the account first and the
// policy lock afterwards could deadlock against an admin flipping mfa_mode
// during their own school login. Whoever needs both takes the policy lock
// first. The account-scoped half of the policy (auth.mfa_overrides) rides on
// the account row lock instead — see lockMFAPolicyForMint.
//
// Errors are returned as bare sentinels; createRefreshTokenWithRetryGuarded
// hands them back untouched and the caller wraps them with its own op.
// The account the guard is handed is deliberately ignored: the checks below
// need the row as it is NOW and under this transaction's lock, which is what
// checkSchoolMintPreconditions re-reads FOR UPDATE — and it is that locked row
// the claims are then loaded for.
func (s *Service) schoolMintGuard(accountID, tenantID int64, claims **accountMetadata, opts ...schoolMintOption) mintGuard {
	var checks schoolMintChecks
	for _, opt := range opts {
		opt(&checks)
	}
	return func(ctx context.Context, _ *authModels.Account) error {
		// FIRST statement of the mint transaction, before any row lock — see
		// MFA LOCK ORDER above.
		if checks.resolveMFAPolicy != nil {
			if err := s.lockMFAPolicyForMint(ctx, accountID, tenantID); err != nil {
				return err
			}
		}
		account, err := s.checkSchoolMintPreconditions(ctx, accountID, tenantID)
		if err != nil {
			return err
		}
		// Pin the permission sources before they are read into the claims —
		// see PERMISSIONS above.
		if err := s.repos.Permission.LockAccountPermissionSourcesForTenant(ctx, accountID, tenantID); err != nil {
			return fmt.Errorf("lock permission sources of account %d at school %d: %w", accountID, tenantID, err)
		}
		// Same transaction, same locked account row: schoolClaimsPayloadInTx
		// deliberately re-runs no liveness gate — the checks above just did,
		// atomically with the write they authorize. It also hydrates
		// account.Roles from inside the lock, which is what the MFA re-check
		// below evaluates.
		payload, payloadErr := s.schoolClaimsPayloadInTx(ctx, account, tenantID)
		if payloadErr != nil {
			return payloadErr
		}
		if checks.resolveMFAPolicy != nil {
			policy, policyErr := checks.resolveMFAPolicy(ctx)
			if policyErr != nil {
				// Fail closed, exactly like the pre-transaction gate: an
				// unreadable policy must refuse THIS mint (503) rather than
				// fall through to "no second factor needed".
				s.getLogger().Warn("mfa policy re-read failed at school token mint; refusing to mint",
					slog.Int64("account_id", accountID),
					slog.Int64("tenant_id", tenantID),
					slog.String("error", policyErr.Error()),
				)
				return ErrMFAStatusUnavailable
			}
			if policy.RequiredFor(account) {
				s.getLogger().Info("mfa requirement appeared during school login; refusing to mint",
					slog.Int64("account_id", accountID),
					slog.Int64("tenant_id", tenantID),
				)
				return errSchoolMFARequiredAtMint
			}
		}
		*claims = payload
		return nil
	}
}

// lockMFAPolicyForMint pins the tenant-wide half of the MFA policy —
// security.mfa_mode — for the rest of the mint transaction, in SHARED mode so
// concurrent logins never block one another.
//
// Re-READING the policy inside the mint transaction (round 10) narrowed the
// window but could not close it: under READ COMMITTED the re-read is just an
// earlier statement, and an admin enabling MFA can still commit between it and
// the token insert. The setting's writer takes the exclusive side of this same
// lock around its whole read-modify-write, so the two orders instead: either
// the admin waits for this mint to commit — and the NEXT login is challenged —
// or the write lands first and the re-read below observes it and refuses.
//
// The per-account half of the policy (auth.mfa_overrides) needs no lock of its
// own: the override write paths take auth.accounts FOR UPDATE before touching
// the rows, and this transaction holds exactly that row from
// checkSchoolMintPreconditions until commit.
//
// An unavailable lock fails the mint closed with the same sentinel an
// unreadable policy produces (503) — proceeding would mean deciding the gate on
// an unordered read, which is the defect this closes.
func (s *Service) lockMFAPolicyForMint(ctx context.Context, accountID, tenantID int64) error {
	if s.settings == nil {
		return nil
	}
	if err := s.settings.LockMFAPolicySharedForTenant(ctx, tenantID); err != nil {
		s.getLogger().Warn("mfa policy lock unavailable at school token mint; refusing to mint",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return ErrMFAStatusUnavailable
	}
	return nil
}

// checkSchoolMintPreconditions runs the four checks described above and hands
// back the LOCKED account row, so a caller that needs to read more state under
// the same lock (the refresh guard, which assembles the token's claims there)
// does not fetch it a second time from a fresher snapshot.
func (s *Service) checkSchoolMintPreconditions(ctx context.Context, accountID, tenantID int64) (*authModels.Account, error) {
	// (1) Account liveness, re-read under the lock. The entry-point check
	// read a snapshot from an already-committed transaction; an operator
	// deactivating the account in the meantime must cut the mint, not
	// merely the next login.
	account, err := s.repos.Account.FindByIDForUpdate(ctx, accountID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("re-check account %d at school token mint: %w", accountID, err)
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}
	if !account.Active {
		return nil, ErrAccountInactive
	}

	school, err := s.repos.School.FindByIDForShare(ctx, tenantID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("re-check school %d at school token mint: %w", tenantID, err)
	}
	if school == nil || school.IsDeleted() || !school.Active {
		return nil, ErrTenantNotFound
	}

	activeMapping, err := s.repos.AccountTenant.ExistsActiveByAccountAndTenantForShare(ctx, accountID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("re-check membership of account %d at school %d: %w", accountID, tenantID, err)
	}
	if !activeMapping {
		return nil, ErrTenantAccessDenied
	}

	accountRoles, err := s.repos.AccountRole.FindByAccountIDForTenantForShare(ctx, accountID, tenantID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrAccountNoSchoolPortalRole
		}
		return nil, fmt.Errorf("re-check school portal role of account %d at school %d: %w", accountID, tenantID, err)
	}
	for _, ar := range accountRoles {
		if ar.Role != nil && isSchoolPortalRole(ar.Role) {
			return account, nil
		}
	}
	return nil, ErrAccountNoSchoolPortalRole
}

// schoolRefreshMintGuard is schoolMintGuard as the refresh path needs it: the
// same four checks and the same in-transaction claims load, wrapped in the two
// differences the refresh path needs — both about the ANSWER the caller gets,
// not about what is checked.
//
// A revoked school-portal role is reported to a refreshing client as
// ErrTenantAccessDenied, not ErrAccountNoSchoolPortalRole: from the session's
// point of view the access is simply gone, and that is the sentinel
// /auth/refresh already maps. And the refusal is logged through
// logRefreshDecision so a cut session shows up in the same stream as every
// other refresh rejection instead of vanishing into the mint path.
//
// Loading the claims inside the transaction matters twice as much here as on
// the mint paths: assembling them afterwards left a window where a school
// soft-deleted right after the guard failed the metadata lookup while
// SoftDeleteSchool had already deleted every token of that school — the freshly
// written successor and its recovery record included. The caller got an error
// and nothing to retry with. In here the two outcomes collapse into one
// transaction: claims and rotation commit together, or neither does and the
// presented refresh token survives untouched for the next attempt.
//
// DB errors keep propagating verbatim (the guard wraps them with %w, so they
// stay distinguishable from the sentinels): a transient blip must surface as
// a retryable 500, never as a revocation — masking it would log the user out
// for good instead of for one request.
func (s *Service) schoolRefreshMintGuard(accountID, tenantID int64, metadata **accountMetadata) mintGuard {
	guard := s.schoolMintGuard(accountID, tenantID, metadata)
	return func(ctx context.Context, account *authModels.Account) error {
		err := guard(ctx, account)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrAccountNoSchoolPortalRole) {
			s.logRefreshDecision("refresh_session_rejected", "school_portal_role_revoked", int(accountID), tenantID)
			return ErrTenantAccessDenied
		}
		if errors.Is(err, ErrTenantAccessDenied) {
			s.logRefreshDecision("refresh_session_rejected", "tenant_access_revoked", int(accountID), tenantID)
		}
		return err
	}
}

// wrapSchoolMintError gives a guard sentinel the AuthError envelope every
// school handler switches on, while leaving errors that already carry one
// (the generic token-persistence failures) alone.
func wrapSchoolMintError(op string, err error) error {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return err
	}
	return &AuthError{Op: op, Err: err}
}

// loadSchoolMetadataForTenant loads the tenant-scoped metadata (roles,
// permissions, person info, org id) and stamps the school scope on it. The
// permissions travel into the JWT exactly like a tenant login — that is
// what lets authorize.RequiresPermission(class_day:read) work unchanged
// behind /school/*.
//
// This is the ONE choke point every school token mint runs through BEFORE it
// writes anything — login, switch, MFA verify, MFA enrollment confirm — so the
// liveness gates live here rather than being repeated per entry point. (The
// refresh path assembles its claims through schoolClaimsPayloadInTx instead,
// from inside its rotation transaction; see there.)
//
// The gates themselves:
//
//   - The school must still be alive AND active. The shared metadata load
//     already rejects soft-deleted schools, but it reads a snapshot taken
//     before this call; re-checking BOTH deleted_at and active on the row we
//     fetch here closes the window where a soft-delete lands between the two
//     lookups and leaves active=true behind.
//   - The account's auth.account_tenants mapping must still be `active`.
//     Membership is revoked by flipping that status, and every path except the
//     password login used to trust the mapping resolved minutes earlier: an
//     in-flight MFA challenge, a school switch, or a refresh would keep minting
//     school tokens for an account that was already removed from the school.
//
// These checks decide the RESPONSE (which sentinel, hence which status code)
// and run before any token work. They are not the last word on authorization:
// they commit in their own transaction, so schoolMintGuard repeats them —
// plus the portal-role check — inside the transaction that actually writes
// the token. Fixing one of the two without the other is not enough.
func (s *Service) loadSchoolMetadataForTenant(ctx context.Context, account *authModels.Account, tenantID int64) (*accountMetadata, error) {
	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.scope = tenant.ScopeSchool

	// Both lookups run as phoenix_admin: auth.account_tenants is RLS-guarded
	// and the school auth flows have no tenant transaction yet.
	var (
		school        *platformModels.School
		activeMapping bool
	)
	if txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err = s.repos.School.FindByID(adminCtx, tenantID)
		if err != nil {
			return fmt.Errorf("lookup school %d for school metadata: %w", tenantID, err)
		}
		activeMapping, err = s.repos.AccountTenant.ExistsByAccountAndTenant(adminCtx, account.ID, tenantID)
		if err != nil {
			return fmt.Errorf("verify account %d membership at school %d: %w", account.ID, tenantID, err)
		}
		return nil
	}); txErr != nil {
		return nil, txErr
	}

	if school == nil || school.IsDeleted() || !school.Active {
		return nil, &AuthError{Op: "load school metadata", Err: ErrTenantNotFound}
	}
	if !activeMapping {
		return nil, &AuthError{Op: "load school metadata", Err: ErrTenantAccessDenied}
	}

	return metadata, nil
}

// schoolClaimsPayloadInTx loads the claims payload of a school token — roles,
// permissions, person, org — and stamps the school scope. No authorization,
// no liveness: just what the JWT is built from.
//
// This is what the REFRESH path uses, and the split exists for that one caller.
// Refresh settles authorization inside the rotation transaction
// (schoolRefreshMintGuard, under the account lock) and calls this from THERE,
// on the very same transaction — hence the InTx suffix and the phoenix_admin
// requirement inherited from loadAccountMetadataForTenantInTx. Re-running the
// entry-point gates in here would be pointless duplication (the guard checked
// the same four facts microseconds earlier, atomically with this write) and
// would push a failure past the rotation, where nothing can repair it.
//
// Every OTHER caller wants loadSchoolMetadataForTenant, which pairs the payload
// with the liveness gates that decide the response BEFORE any token work.
func (s *Service) schoolClaimsPayloadInTx(ctx context.Context, account *authModels.Account, tenantID int64) (*accountMetadata, error) {
	metadata, err := s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.scope = tenant.ScopeSchool
	return metadata, nil
}

// hasSchoolPortalRoleAtTenant reports whether the account holds a
// school-portal role at the tenant. Runs in an admin tx (auth.account_roles
// is RLS-guarded and the auth flows have no tenant transaction) and
// PROPAGATES query errors: a transient DB blip must surface as a retryable
// error, never masquerade as "role revoked" (which on the refresh path would
// log the user out for good).
func (s *Service) hasSchoolPortalRoleAtTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	var hasRole bool
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		accountRoles, rolesErr := s.repos.AccountRole.FindByAccountIDForTenant(adminCtx, accountID, tenantID)
		if rolesErr != nil {
			if isNotFoundError(rolesErr) {
				return nil
			}
			return rolesErr
		}
		for _, ar := range accountRoles {
			if ar.Role != nil && isSchoolPortalRole(ar.Role) {
				hasRole = true
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("load school portal role for account %d at tenant %d: %w", accountID, tenantID, err)
	}
	return hasRole, nil
}

// findSchoolPortalTenantForAccount checks every active tenant mapping for a
// school-portal role and returns the first matching tenant whose school is
// alive. Mirrors findGuardianTenantForAccount (admin tx because
// account_tenants + account_roles have RLS that requires
// app.current_tenant_id, which the public school login does not have yet)
// and resolveAccountTenantDefault's school filter: deactivated or
// soft-deleted schools are skipped, so an operator turning a school off
// cuts its Lehrkräfte's logins, and a dead school in the oldest mapping
// does not shadow a valid second school.
func (s *Service) findSchoolPortalTenantForAccount(ctx context.Context, accountID int64) (bool, int64, error) {
	hasPortalRole := false
	var firstPortalTenantID int64

	if err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		mappings, listErr := s.repos.AccountTenant.FindActiveByAccountID(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		var lastSchoolLookupErr error
		for _, m := range mappings {
			school, schoolErr := s.repos.School.FindByID(adminCtx, m.TenantID)
			if schoolErr != nil {
				// Mirror resolveAccountTenantDefault: keep scanning the
				// remaining mappings, but remember the error so a full
				// sweep of DB failures is not masked as "no portal role".
				if !isNotFoundError(schoolErr) {
					lastSchoolLookupErr = schoolErr
				}
				continue
			}
			if school == nil || school.IsDeleted() || !school.Active {
				continue
			}
			// FindByAccountIDForTenant hydrates ar.Role via its join —
			// same mechanism the refresh/switch role checks rely on, no
			// per-role lookup needed.
			accountRoles, roleErr := s.repos.AccountRole.FindByAccountIDForTenant(adminCtx, accountID, m.TenantID)
			if roleErr != nil {
				// A genuine "no roles" result comes back as an empty
				// slice, not an error. Any error here is a real
				// RLS/config/transient DB failure — propagate it so the
				// caller fails loudly instead of silently reporting "no
				// portal role".
				if isNotFoundError(roleErr) {
					continue
				}
				return roleErr
			}
			for _, ar := range accountRoles {
				if ar.Role != nil && isSchoolPortalRole(ar.Role) {
					hasPortalRole = true
					firstPortalTenantID = m.TenantID
					return nil
				}
			}
		}
		if !hasPortalRole && lastSchoolLookupErr != nil {
			return lastSchoolLookupErr
		}
		return nil
	}); err != nil {
		return false, 0, err
	}

	return hasPortalRole, firstPortalTenantID, nil
}

// HasSchoolPortalAccess reports whether the account may STILL hold a school
// session at this school. It exists for surfaces that authenticate once and
// then stay open for the whole token lifetime — today the school SSE stream
// (#2208), which would otherwise keep waking a Lehrkraft whose access was
// revoked minutes ago until her access token expires.
//
// It re-checks the same four facts every school token mint gates on
// (loadActiveAccountForSchoolMint + loadSchoolMetadataForTenant +
// hasSchoolPortalRoleAtTenant), because any one of them being revoked ends the
// session just as surely as the portal role does:
//
//   - the account is still active,
//   - the school is still alive and active,
//   - the auth.account_tenants mapping is still `active`,
//   - the account still holds a school-portal role there.
//
// It deliberately does NOT go through loadSchoolMetadataForTenant itself: that
// path also assembles roles, permissions and person data for a JWT, and this
// runs once a minute per open stream where none of it is needed.
//
// A revoked fact answers (false, nil) — that is the caller's signal to close
// the stream. Errors propagate for the same reason they do in
// hasSchoolPortalRoleAtTenant: a database blip is not a revocation, and the
// caller decides how long it may keep serving on the last successful answer.
func (s *Service) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("look up account %d for school portal access: %w", accountID, err)
	}
	if account == nil || !account.Active {
		return false, nil
	}

	// Both reads are RLS-guarded and this runs outside any tenant transaction
	// (the SSE stream holds none) — same admin-tx requirement as the login
	// flows.
	var (
		school        *platformModels.School
		activeMapping bool
	)
	if txErr := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err = s.repos.School.FindByID(adminCtx, tenantID)
		if err != nil {
			if isNotFoundError(err) {
				school = nil
				return nil
			}
			return fmt.Errorf("look up school %d for school portal access: %w", tenantID, err)
		}
		activeMapping, err = s.repos.AccountTenant.ExistsByAccountAndTenant(adminCtx, accountID, tenantID)
		if err != nil {
			return fmt.Errorf("verify account %d membership at school %d: %w", accountID, tenantID, err)
		}
		return nil
	}); txErr != nil {
		return false, txErr
	}

	if school == nil || school.IsDeleted() || !school.Active || !activeMapping {
		return false, nil
	}

	return s.hasSchoolPortalRoleAtTenant(ctx, accountID, tenantID)
}
