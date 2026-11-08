package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *AccountAuthentication) schoolLoginTenant(ctx context.Context, account domain.LoginAccount, targetTenantSlug, ipAddress, userAgent string) (int64, error) {
	if targetTenantSlug != "" {
		return s.requestedSchoolLoginTenant(ctx, account, targetTenantSlug, ipAddress, userAgent)
	}
	return s.implicitSchoolLoginTenant(ctx, account, ipAddress, userAgent)
}

func (s *AccountAuthentication) requestedSchoolLoginTenant(ctx context.Context, account domain.LoginAccount, targetTenantSlug, ipAddress, userAgent string) (int64, error) {
	metadata, err := s.loadAccountMetadata(ctx, account, targetTenantSlug)
	if err != nil {
		return 0, err
	}
	if !domain.HasSchoolPortalRole(metadata.Roles) {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at requested school")
		return 0, failed("school login", identityaccess.ErrAccountNoSchoolPortalRole)
	}
	if err := s.requireSchoolPortal(ctx, "school login", account.ID, metadata.TenantID); err != nil {
		s.logFailedLogin(s.runtime.WithTenantID(ctx, metadata.TenantID), account.ID, ipAddress, userAgent, "School portal not booked")
		return 0, err
	}
	return metadata.TenantID, nil
}

func (s *AccountAuthentication) implicitSchoolLoginTenant(ctx context.Context, account domain.LoginAccount, ipAddress, userAgent string) (int64, error) {
	hasPortalRole, portalTenantID, err := s.findSchoolPortalTenantForAccount(ctx, account.ID, true)
	if err != nil {
		return 0, s.implicitSchoolLoginError(ctx, account.ID, portalTenantID, ipAddress, userAgent, err)
	}
	if !hasPortalRole || portalTenantID == 0 {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "No school portal role at any school")
		return 0, failed("school login", identityaccess.ErrAccountNoSchoolPortalRole)
	}
	return portalTenantID, nil
}

func (s *AccountAuthentication) implicitSchoolLoginError(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string, err error) error {
	if _, notBooked := identityaccess.PortalNotBooked(err); notBooked {
		s.logPortalRefusal("school login", accountID, tenantID, err)
		s.logFailedLogin(s.runtime.WithTenantID(ctx, tenantID), accountID, ipAddress, userAgent, "School portal not booked")
		return failed("school login", err)
	}
	return failed("school login: enumerate tenants", err)
}

func (s *AccountAuthentication) schoolLoginMFAVerdict(ctx context.Context, accountID, portalTenantID int64, permissions []string) (mfaRequired, enrolled bool, err error) {
	// Same fail-closed MFA semantics as the tenant login. The verdict
	// resolved here decides the branch only; the guard re-resolves the
	// policy inside the mint transaction (see schoolEligibility).
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
	var proof mintProof
	if mfaRequired && enrolled {
		proof = mintProof{kind: proofTrustedDevice, trustedDeviceCookie: trustedDeviceCookie}
	}
	minted, err := s.mintAccountSession(ctx, mintRequest{
		portal: domain.PortalSchool, account: account, email: email, tenantID: portalTenantID, proof: proof,
		// Where the gate concluded "no second factor needed", the mint decides
		// it again: a role granted or the school's mfa_mode switched on in the
		// gap would otherwise hand out a session that never saw a challenge.
		recheckMFAGate: !mfaRequired && s.mfa.Configured(),
		// The event belongs to the school the portal-role lookup resolved,
		// not to the account's first mapping.
		audit: &mintAudit{event: domain.AuthEventLogin, ipAddress: ipAddress, userAgent: userAgent, schoolID: portalTenantID},
	})
	if errors.Is(err, errSchoolMFARequiredAtMint) {
		// Nothing was written; send the login down the branch it would have
		// taken had the requirement been there from the start.
		if !enrolled {
			return s.mfaEnrollmentResult(account, portalTenantID, domain.PortalSchool)
		}
		return s.mfaChallengeResult(ctx, account, portalTenantID, domain.PortalSchool, ipAddress)
	}
	if err != nil {
		return nil, wrapSchoolMintError("school login", err)
	}
	return &domain.LoginResult{Status: domain.LoginStatusAuthenticated, AccessToken: minted.accessToken, RefreshToken: minted.refreshToken}, nil
}

func (s *AccountAuthentication) requireSchoolMintMFAPolicy(ctx context.Context, accountID, tenantID int64, payload *domain.AccountClaimsPayload) error {
	policy, err := s.mfa.ResolvePolicyInTx(ctx, accountID, tenantID)
	if err != nil {
		s.logger.Warn("mfa policy re-read failed at school token mint; refusing to mint",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return identityaccess.ErrMFAStatusUnavailable
	}
	if policy.RequiredFor(payload.Permissions) {
		s.logger.Info("mfa requirement appeared during school login; refusing to mint",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID))
		return errSchoolMFARequiredAtMint
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

// schoolPortalTenants lists the active, live schools where the account has a
// school-portal role. School lookup failures only surface when no school
// qualifies, so one unreadable mapping does not mask a later valid school.
func (s *AccountAuthentication) schoolPortalTenants(adminCtx context.Context, accountID int64) ([]int64, error) {
	tenantIDs, _, listErr := s.store.ListActiveTenantIDs(adminCtx, accountID)
	if listErr != nil {
		return nil, listErr
	}
	var lastSchoolLookupErr error
	portalTenantIDs := make([]int64, 0, len(tenantIDs))
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
			return nil, roleErr
		}
		if domain.HasSchoolPortalRole(roles) {
			portalTenantIDs = append(portalTenantIDs, tenantID)
		}
	}
	if len(portalTenantIDs) == 0 && lastSchoolLookupErr != nil {
		return nil, lastSchoolLookupErr
	}
	return portalTenantIDs, nil
}
