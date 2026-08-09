package auth

import (
	"context"

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
//   - required + not enrolled → tenant-scope enrollment token for the
//     existing /auth/mfa/enroll/* surface. Completing enrollment there
//     mints tenant tokens (still legitimate for lehrkraft accounts until
//     the tenant-login cutover); the user then logs in again at the school
//     portal, now enrolled, and takes the challenge branch.
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
			Scope:     jwt.MFAEnrollmentScopeTenant,
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
	if !accountHasSchoolPortalRole(account) {
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
	if !accountHasSchoolPortalRole(account) {
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
func (s *Service) loadSchoolMetadataForTenant(ctx context.Context, account *authModels.Account, tenantID int64) (*accountMetadata, error) {
	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.scope = tenant.ScopeSchool
	return metadata, nil
}

// accountHasSchoolPortalRole checks the hydrated account.Roles (loaded by
// loadSchoolMetadataForTenant for one specific tenant) for a school-portal
// role.
func accountHasSchoolPortalRole(account *authModels.Account) bool {
	for _, role := range account.Roles {
		if isSchoolPortalRole(role) {
			return true
		}
	}
	return false
}

// findSchoolPortalTenantForAccount checks every active tenant mapping for a
// school-portal role and returns the first matching tenant. Mirrors
// findGuardianTenantForAccount: admin tx because account_tenants +
// account_roles have RLS that requires app.current_tenant_id, which the
// public school login does not have yet.
func (s *Service) findSchoolPortalTenantForAccount(ctx context.Context, accountID int64) (bool, int64, error) {
	hasPortalRole := false
	var firstPortalTenantID int64

	if err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		mappings, listErr := s.repos.AccountTenant.FindActiveByAccountID(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		for _, m := range mappings {
			roles, roleErr := s.repos.AccountRole.FindByAccountIDForTenant(adminCtx, accountID, m.TenantID)
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
			for _, ar := range roles {
				role, lookupErr := s.repos.Role.FindByID(adminCtx, ar.RoleID)
				if lookupErr != nil {
					// A missing role row for an existing account_role is a
					// data inconsistency — treat it as "not this role".
					// Real DB errors must propagate.
					if isNotFoundError(lookupErr) {
						continue
					}
					return lookupErr
				}
				if role == nil {
					continue
				}
				if isSchoolPortalRole(role) {
					hasPortalRole = true
					firstPortalTenantID = m.TenantID
					return nil
				}
			}
		}
		return nil
	}); err != nil {
		return false, 0, err
	}

	return hasPortalRole, firstPortalTenantID, nil
}
