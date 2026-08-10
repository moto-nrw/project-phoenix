package auth

import (
	"context"
	"errors"
	"fmt"

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
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	hasPortalRole, portalTenantID, err := s.findSchoolPortalTenantForAccount(ctx, account.ID)
	if err != nil {
		return nil, &AuthError{Op: "school login: enumerate tenants", Err: err}
	}
	if !hasPortalRole {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at any school")
		return nil, &AuthError{Op: "school login", Err: ErrAccountNoSchoolPortalRole}
	}

	// Loads roles + permissions for the resolved school and hydrates
	// account.Roles — the MFA IsRequired check below reads them.
	metadata, err := s.loadSchoolMetadataForTenant(ctx, account, portalTenantID)
	if err != nil {
		return nil, err
	}

	// MFA gate — same fail-closed semantics as LoginWithMFAGate: on infra
	// errors we refuse THIS login with 503 instead of silently dropping to
	// "not required".
	mfaRequired := false
	if s.mfaService != nil {
		mfaRequired, err = s.mfaService.IsRequired(ctx, account, portalTenantID)
		if err != nil {
			return nil, &AuthError{Op: "check mfa required", Err: ErrMFAStatusUnavailable}
		}
	}

	enrolled := false
	if s.mfaService != nil {
		enrolled, err = s.mfaService.HasEnrollment(ctx, account.ID)
		if err != nil {
			return nil, &AuthError{Op: "check mfa enrollment", Err: ErrMFAStatusUnavailable}
		}
	}

	if mfaRequired && !enrolled {
		enrollmentToken, tokenErr := s.tokenAuth.CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
			AccountID: account.ID,
			Scope:     jwt.MFAEnrollmentScopeSchool,
			TenantID:  portalTenantID,
		}, MFAEnrollmentTokenTTL)
		if tokenErr != nil {
			return nil, &AuthError{Op: "issue mfa enrollment token", Err: tokenErr}
		}
		return &LoginResult{
			Status:                LoginStatusMFAEnrollmentRequired,
			AccessToken:           enrollmentToken,
			MaskedEmail:           MaskEmailForUX(account.Email),
			MFAEnrollmentRequired: true,
		}, nil
	}

	trustedDeviceVerified := false
	if mfaRequired && enrolled && trustedDeviceCookie != "" && s.mfaService != nil {
		ok, _ := s.mfaService.VerifyTrustedDevice(ctx, account.ID, portalTenantID, trustedDeviceCookie)
		trustedDeviceVerified = ok
	}

	if mfaRequired && enrolled && !trustedDeviceVerified {
		challenge, chErr := s.mfaService.StartChallenge(ctx, account.ID, portalTenantID, jwt.MFAChallengeScopeSchool, ParseClientIP(ipAddress))
		if chErr != nil {
			return nil, &AuthError{Op: "start mfa challenge", Err: chErr}
		}
		return &LoginResult{
			Status:               LoginStatusMFARequired,
			ChallengeToken:       challenge,
			MaskedEmail:          MaskEmailForUX(account.Email),
			TrustedDeviceEnabled: s.mfaService.IsTrustedDeviceEnabled(ctx, portalTenantID),
			TrustedDeviceDays:    s.mfaService.TrustedDeviceDays(ctx, portalTenantID),
		}, nil
	}

	// The portal-role lookup above ran minutes ago in wall-clock terms once MFA
	// or a trusted-device check sat in between — and in its own, already
	// committed transaction either way. The guard re-checks school liveness,
	// membership and portal role INSIDE the transaction that writes the token,
	// so a revocation can no longer land in the gap.
	token, err := s.createRefreshTokenWithRetryGuarded(ctx, account, portalTenantID, s.schoolMintGuard(account.ID, portalTenantID))
	if err != nil {
		return nil, wrapSchoolMintError("school login", err)
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
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return "", "", &AuthError{Op: "issue school tokens", Err: ErrAccountNotFound}
	}
	if !account.Active {
		return "", "", &AuthError{Op: "issue school tokens", Err: ErrAccountInactive}
	}

	metadata, err := s.loadSchoolMetadataForTenant(ctx, account, tenantID)
	if err != nil {
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

	token, err := s.createRefreshTokenWithRetryGuarded(ctx, account, metadata.tenantID, s.schoolMintGuard(account.ID, metadata.tenantID))
	if err != nil {
		return "", "", wrapSchoolMintError("issue school tokens", err)
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
func (s *Service) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return "", "", &AuthError{Op: "switch school", Err: ErrAccountNotFound}
	}
	if !account.Active {
		return "", "", &AuthError{Op: "switch school", Err: ErrAccountInactive}
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

	metadata, err := s.loadSchoolMetadataForTenant(ctx, account, targetTenantID)
	if err != nil {
		return "", "", err
	}
	hasRole, err := s.hasSchoolPortalRoleAtTenant(ctx, accountID, targetTenantID)
	if err != nil {
		return "", "", err
	}
	if !hasRole {
		return "", "", &AuthError{Op: "switch school", Err: ErrAccountNoSchoolPortalRole}
	}

	token, err := s.createRefreshTokenWithRetryGuarded(ctx, account, metadata.tenantID, s.schoolMintGuard(accountID, metadata.tenantID))
	if err != nil {
		return "", "", wrapSchoolMintError("switch school", err)
	}

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)
	// The switch event belongs to the TARGET school, not to whatever mapping
	// happens to be oldest.
	return s.generateAndLogTokens(
		tenant.WithTenantID(ctx, metadata.tenantID),
		account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeTenantSwitch,
	)
}

// schoolMintGuard re-verifies the four facts a school session rests on,
// inside the transaction that persists the refresh token:
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
// Errors are returned as bare sentinels; createRefreshTokenWithRetryGuarded
// hands them back untouched and the caller wraps them with its own op.
func (s *Service) schoolMintGuard(accountID, tenantID int64) mintGuard {
	return func(ctx context.Context) error {
		// (1) Account liveness, re-read under the lock. The entry-point check
		// read a snapshot from an already-committed transaction; an operator
		// deactivating the account in the meantime must cut the mint, not
		// merely the next login.
		account, err := s.repos.Account.FindByIDForUpdate(ctx, accountID)
		if err != nil {
			if isNotFoundError(err) {
				return ErrAccountNotFound
			}
			return fmt.Errorf("re-check account %d at school token mint: %w", accountID, err)
		}
		if account == nil {
			return ErrAccountNotFound
		}
		if !account.Active {
			return ErrAccountInactive
		}

		school, err := s.repos.School.FindByIDForShare(ctx, tenantID)
		if err != nil {
			if isNotFoundError(err) {
				return ErrTenantNotFound
			}
			return fmt.Errorf("re-check school %d at school token mint: %w", tenantID, err)
		}
		if school == nil || school.IsDeleted() || !school.Active {
			return ErrTenantNotFound
		}

		activeMapping, err := s.repos.AccountTenant.ExistsActiveByAccountAndTenantForShare(ctx, accountID, tenantID)
		if err != nil {
			return fmt.Errorf("re-check membership of account %d at school %d: %w", accountID, tenantID, err)
		}
		if !activeMapping {
			return ErrTenantAccessDenied
		}

		accountRoles, err := s.repos.AccountRole.FindByAccountIDForTenantForShare(ctx, accountID, tenantID)
		if err != nil {
			if isNotFoundError(err) {
				return ErrAccountNoSchoolPortalRole
			}
			return fmt.Errorf("re-check school portal role of account %d at school %d: %w", accountID, tenantID, err)
		}
		for _, ar := range accountRoles {
			if ar.Role != nil && isSchoolPortalRole(ar.Role) {
				return nil
			}
		}
		return ErrAccountNoSchoolPortalRole
	}
}

// schoolRefreshMintGuard is schoolMintGuard as the refresh path needs it.
//
// Two differences, both about the answer the caller gets rather than about
// what is checked. A revoked school-portal role is reported to a refreshing
// client as ErrTenantAccessDenied, not ErrAccountNoSchoolPortalRole: from the
// session's point of view the access is simply gone, and that is the sentinel
// /auth/refresh already maps. And the refusal is logged through
// logRefreshDecision so a cut session shows up in the same stream as every
// other refresh rejection instead of vanishing into the mint path.
//
// DB errors keep propagating verbatim (the guard wraps them with %w, so they
// stay distinguishable from the sentinels): a transient blip must surface as
// a retryable 500, never as a revocation — masking it would log the user out
// for good instead of for one request.
func (s *Service) schoolRefreshMintGuard(accountID, tenantID int64) mintGuard {
	guard := s.schoolMintGuard(accountID, tenantID)
	return func(ctx context.Context) error {
		err := guard(ctx)
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
// This is the ONE choke point every school token mint runs through — login,
// switch, MFA verify, MFA enrollment confirm, and refresh — so the liveness
// gates live here rather than being repeated per entry point:
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
