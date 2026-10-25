package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *AccountAuthentication) schoolLoginTenant(ctx context.Context, account domain.LoginAccount, targetTenantSlug, ipAddress, userAgent string) (int64, error) {
	portalTenantID := int64(0)
	hasPortalRole := false
	if targetTenantSlug != "" {
		metadata, metadataErr := s.loadAccountMetadata(ctx, account, targetTenantSlug)
		if metadataErr != nil {
			return 0, metadataErr
		}
		portalTenantID = metadata.TenantID
		hasPortalRole = domain.HasSchoolPortalRole(metadata.Roles)
		if !hasPortalRole {
			s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at requested school")
			return 0, failed("school login", identityaccess.ErrAccountNoSchoolPortalRole)
		}
	} else {
		var resolvedTenantID int64
		var findErr error
		hasPortalRole, resolvedTenantID, findErr = s.findSchoolPortalTenantForAccount(ctx, account.ID)
		if findErr != nil {
			return 0, failed("school login: enumerate tenants", findErr)
		}
		if hasPortalRole {
			portalTenantID = resolvedTenantID
		}
	}
	if !hasPortalRole || portalTenantID == 0 {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at any school")
		return 0, failed("school login", identityaccess.ErrAccountNoSchoolPortalRole)
	}
	// Checked before any second factor is mailed: a school without
	// Unterricht am Vormittag has no school portal (#4895).
	if err := s.requireSchoolPortal(ctx, "school login", account.ID, portalTenantID); err != nil {
		s.logFailedLogin(s.runtime.WithTenantID(ctx, portalTenantID), account.ID, ipAddress, userAgent, "School portal not booked")
		return 0, err
	}

	return portalTenantID, nil
}

func (s *AccountAuthentication) schoolLoginMFAVerdict(ctx context.Context, accountID, portalTenantID int64, permissions []string) (mfaRequired, enrolled bool, err error) {
	// Same fail-closed MFA semantics as the tenant login. The verdict
	// resolved here decides the branch only; the guard re-resolves the
	// policy inside its own transaction (see schoolMintGuard).
	mfaRequired = false
	if s.mfa.Configured() {
		policy, policyErr := s.mfa.ResolvePolicy(ctx, accountID, portalTenantID)
		if policyErr != nil {
			return false, false, failed("check mfa required", identityaccess.ErrMFAStatusUnavailable)
		}
		mfaRequired = policy.RequiredFor(permissions)
	}
	enrolled = false
	if s.mfa.Configured() {
		enrolled, err = s.mfa.HasEnrollment(ctx, accountID)
		if err != nil {
			return false, false, failed("check mfa enrollment", identityaccess.ErrMFAStatusUnavailable)
		}
	}
	return mfaRequired, enrolled, nil
}

func (s *AccountAuthentication) mintSchoolLogin(ctx context.Context, account domain.LoginAccount, portalTenantID int64, mfaRequired, enrolled bool, email, ipAddress, userAgent, trustedDeviceCookie string) (*domain.LoginResult, error) {
	// The guard re-checks school liveness, membership and portal role inside
	// the transaction that writes the session and assembles the claims there.
	// Where the gate concluded "no second factor needed", it also re-decides
	// the MFA gate: a role granted or the school's mfa_mode switched on in
	// the gap would otherwise hand out a session that never saw a challenge.
	guardOpts := []schoolMintOption{}
	if !mfaRequired && s.mfa.Configured() {
		guardOpts = append(guardOpts, withMFAGateRecheck(s.freshSchoolMFAPolicy(account.ID, portalTenantID)))
	}
	var claims *domain.AccountClaimsPayload
	guard := s.schoolMintGuard(account.ID, portalTenantID, &claims, guardOpts...)
	if mfaRequired && enrolled {
		guard = s.trustedDeviceMintGuard(account.ID, portalTenantID, trustedDeviceCookie, guard)
	}
	session, err := s.createRefreshSessionGuarded(ctx, account, portalTenantID, domain.ScopeSchool, guard, "", mfaRequired && enrolled)
	if errors.Is(err, errSchoolMFARequiredAtMint) {
		// Nothing was written; send the login down the branch it would have
		// taken had the requirement been there from the start.
		if !enrolled {
			return s.mfaEnrollmentResult(account, portalTenantID, domain.ScopeSchool)
		}
		return s.mfaChallengeResult(ctx, account, portalTenantID, domain.ScopeSchool, ipAddress)
	}
	if err != nil {
		return nil, wrapSchoolMintError("school login", err)
	}
	if claims == nil {
		return nil, failed("school login", errMissingSchoolClaimsPayload)
	}
	access, refresh := buildClaims(account, session, claims, email)
	// The audit event belongs to the school the portal-role lookup resolved,
	// not to the account's first mapping.
	accessToken, refreshToken, err := s.generateAndLogTokens(s.runtime.WithTenantID(ctx, portalTenantID), account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
	if err != nil {
		return nil, err
	}
	return &domain.LoginResult{Status: domain.LoginStatusAuthenticated, AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *AccountAuthentication) requireSchoolMintMFAPolicy(ctx context.Context, accountID, tenantID int64, payload *domain.AccountClaimsPayload, checks schoolMintChecks) error {
	if checks.resolveMFAPolicy != nil {
		policy, policyErr := checks.resolveMFAPolicy(ctx)
		if policyErr != nil {
			s.logger.Warn("mfa policy re-read failed at school token mint; refusing to mint",
				slog.Int64("account_id", accountID),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", policyErr.Error()))
			return identityaccess.ErrMFAStatusUnavailable
		}
		if policy.RequiredFor(payload.Permissions) {
			s.logger.Info("mfa requirement appeared during school login; refusing to mint",
				slog.Int64("account_id", accountID),
				slog.Int64("tenant_id", tenantID))
			return errSchoolMFARequiredAtMint
		}
	}
	return nil
}

func (s *AccountAuthentication) firstSchoolPortalTenant(adminCtx context.Context, accountID int64) (hasPortalRole bool, firstPortalTenantID int64, err error) {
	tenantIDs, _, listErr := s.store.ListActiveTenantIDs(adminCtx, accountID)
	if listErr != nil {
		return hasPortalRole, firstPortalTenantID, listErr
	}
	var lastSchoolLookupErr error
	for _, tenantID := range tenantIDs {
		school, found, schoolErr := s.schools.FindSchool(adminCtx, tenantID)
		if schoolErr != nil {
			lastSchoolLookupErr = schoolErr
			continue
		}
		if !found || !school.Live() {
			continue
		}
		roles, _, roleErr := s.store.ListAccountRolesAtTenant(adminCtx, accountID, tenantID, false)
		if roleErr != nil {
			return hasPortalRole, firstPortalTenantID, roleErr
		}
		if domain.HasSchoolPortalRole(roles) {
			hasPortalRole = true
			firstPortalTenantID = tenantID
			return hasPortalRole, firstPortalTenantID, nil
		}
	}
	if !hasPortalRole && lastSchoolLookupErr != nil {
		return hasPortalRole, firstPortalTenantID, lastSchoolLookupErr
	}
	return hasPortalRole, firstPortalTenantID, nil
}
