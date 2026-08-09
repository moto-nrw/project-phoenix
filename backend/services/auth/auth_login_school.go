package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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

	token, err := s.createRefreshTokenWithRetry(ctx, account, portalTenantID)
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
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at school token issue")
		return "", "", &AuthError{Op: "issue school tokens", Err: ErrAccountNoSchoolPortalRole}
	}

	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID)
	if err != nil {
		return "", "", err
	}

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)
	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
}

// SwitchSchool re-authenticates a school-portal session to a different
// school — the school-scope sibling of SwitchTenant for Lehrkraft accounts
// mapped to several schools. The target is resolved by slug, must be an
// active mapping, and the account must hold a school-portal role THERE:
// being a Lehrkraft at school A and a caregiver at school B does not open
// school B's portal view.
func (s *Service) SwitchSchool(ctx context.Context, accountID int64, tenantSlug string) (string, string, error) {
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

	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID)
	if err != nil {
		return "", "", err
	}

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)
	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, "", "", audit.EventTypeTenantSwitch)
}

// loadSchoolMetadataForTenant loads the tenant-scoped metadata (roles,
// permissions, person info, org id) and stamps the school scope on it. The
// permissions travel into the JWT exactly like a tenant login — that is
// what lets authorize.RequiresPermission(class_day:read) work unchanged
// behind /school/*.
//
// On top of the shared metadata load (which already rejects soft-deleted
// schools) this refuses INACTIVE schools: an operator deactivating a school
// must cut every school-portal token mint — login, switch, MFA verify, and
// refresh all funnel through here.
func (s *Service) loadSchoolMetadataForTenant(ctx context.Context, account *authModels.Account, tenantID int64) (*accountMetadata, error) {
	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	school, err := s.repos.School.FindByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("lookup school %d for school metadata: %w", tenantID, err)
	}
	if school == nil || !school.Active {
		return nil, &AuthError{Op: "load school metadata", Err: ErrTenantNotFound}
	}
	metadata.scope = tenant.ScopeSchool
	return metadata, nil
}

// hasSchoolPortalRoleAtTenant reports whether the account holds a
// school-portal role at the tenant. Runs in an admin tx (auth.account_roles
// is RLS-guarded and the auth flows have no tenant transaction) and — unlike
// the swallow-and-warn role hydration in ensureAccountRolesLoadedForTenant —
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
