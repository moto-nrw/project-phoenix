package application

import (
	"context"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// LoginParentWithAudit authenticates a parent and issues a parent-scope JWT
// with no tenant binding: the token works against parent-scoped endpoints
// regardless of which school the children attend. Accounts without a
// guardian role on at least one active mapping are refused.
func (s *AccountAuthentication) loginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}
	return s.issueParentTokens(ctx, account, email, ipAddress, userAgent)
}

// IssueDemoParentTokens mints the parent-scope session the demo access
// redeems for the demo school's parent (#3468). It skips the credential
// check but otherwise is the parent login; a parent session carries no
// staff rights, so it records no proof (MFANone).
func (s *AccountAuthentication) IssueDemoParentTokens(ctx context.Context, accountID int64, ipAddress, userAgent string) (string, string, error) {
	account, err := s.authenticatedAccount(ctx, "issue parent tokens", accountID)
	if err != nil {
		return "", "", err
	}
	return s.issueParentTokens(ctx, account, account.Email, ipAddress, userAgent)
}

// issueParentTokens refuses an account without a guardian role on an active
// mapping at a live school and mints the parent-scope session.
func (s *AccountAuthentication) issueParentTokens(ctx context.Context, account domain.LoginAccount, email, ipAddress, userAgent string) (string, string, error) {
	guardianTenantIDs, err := s.findGuardianTenantForAccount(ctx, account.ID)
	if err != nil {
		return "", "", failed("parent login: enumerate tenants", err)
	}
	if len(guardianTenantIDs) == 0 {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Not a guardian at any live school")
		return "", "", failed("parent login", identityaccess.ErrAccountNoGuardianRole)
	}
	// The parents portal opens while at least one guardian school booked
	// Eltern-App (#4895).
	bookedTenantIDs, err := s.bookedGuardianSchools(ctx, "parent login", account.ID, guardianTenantIDs)
	if err != nil {
		if _, notBooked := identityaccess.PortalNotBooked(err); notBooked {
			s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Parents portal not booked at any guardian school")
		}
		return "", "", err
	}
	// The refresh session row is pinned to the first guardian tenant with
	// the portal for the foreign key and RLS; the token itself is
	// cross-tenant on read.
	minted, err := s.mintAccountSession(ctx, mintRequest{
		portal: domain.PortalParent, account: account, email: email, tenantID: bookedTenantIDs[0],
		op: "parent login", booked: bookedTenantIDs,
		audit: &mintAudit{event: domain.AuthEventLogin, ipAddress: ipAddress, userAgent: userAgent},
	})
	return minted.accessToken, minted.refreshToken, err
}

// buildParentMetadata is the claims material of a parent-scope token: the
// guardian role only, no tenant, no names. Handlers re-query per-tenant
// detail at request time.
func buildParentMetadata(account domain.LoginAccount) *domain.AccountClaimsPayload {
	return &domain.AccountClaimsPayload{
		RoleNames:   []string{domain.GuardianRoleName},
		Permissions: nil,
		Username:    account.Username,
		Scope:       domain.PortalParent.Claim,
	}
}

// findGuardianTenantForAccount returns every active mapping that carries the
// guardian role at a live school, in mapping order, read inside an
// administrative transaction; the parent flows have no tenant transaction
// yet. A school switched inactive or deleted opens no parents portal, as it
// opens no staff session; school liveness belongs to Organisation & Tenancy.
func (s *AccountAuthentication) findGuardianTenantForAccount(ctx context.Context, accountID int64) ([]int64, error) {
	var guardianTenantIDs []int64
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var err error
		guardianTenantIDs, err = s.guardianSchoolsInTx(adminCtx, accountID, false)
		return err
	})
	return guardianTenantIDs, err
}

// guardianSchoolsInTx is findGuardianTenantForAccount on the caller's
// transaction; lockRoles pins the role rows for a mint. A failed role load is
// an error, never "not a guardian".
func (s *AccountAuthentication) guardianSchoolsInTx(ctx context.Context, accountID int64, lockRoles bool) ([]int64, error) {
	tenantIDs, _, err := s.store.ListActiveTenantIDs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var guardianTenantIDs []int64
	for _, tenantID := range tenantIDs {
		roles, _, err := s.store.ListAccountRolesAtTenant(ctx, accountID, tenantID, lockRoles)
		if err != nil {
			return nil, err
		}
		if domain.HasGuardianRole(roles) {
			guardianTenantIDs = append(guardianTenantIDs, tenantID)
		}
	}
	return s.liveSchoolIDs(ctx, guardianTenantIDs)
}

// liveSchoolIDs keeps the schools that are switched active and not deleted,
// in the given order, inside the caller's transaction.
func (s *AccountAuthentication) liveSchoolIDs(ctx context.Context, tenantIDs []int64) ([]int64, error) {
	if len(tenantIDs) == 0 {
		return tenantIDs, nil
	}
	live, err := s.schools.ListActiveSchoolsByID(ctx, tenantIDs)
	if err != nil {
		return nil, err
	}
	operational := make(map[int64]bool, len(live))
	for _, school := range live {
		operational[school.ID] = true
	}
	return slices.DeleteFunc(tenantIDs, func(tenantID int64) bool { return !operational[tenantID] }), nil
}

// FindGuardianTenant is findGuardianTenantForAccount for the retained
// password-reset flow, which routes guardian accounts to the parents reset
// link.
func (s *AccountAuthentication) findGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	tenantIDs, err := s.findGuardianTenantForAccount(ctx, accountID)
	if err != nil || len(tenantIDs) == 0 {
		return false, 0, err
	}
	return true, tenantIDs[0], nil
}

// SwitchTenant authenticates an account to a different school and returns
// new tokens. Refresh sessions of other families stay alive; the presented
// family is retired instead, with a short grace, because the browser
// replaces its session with the returned pair (#2952). An empty
// presentedFamilyID skips the retirement.
func (s *AccountAuthentication) switchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID, ipAddress, userAgent string) (string, string, error) {
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, false)
	if err != nil || !found {
		s.logger.Warn("switch-tenant: account not found",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
		return "", "", failed("switch tenant", domain.ErrAccountNotFound)
	}
	if !account.Active {
		return "", "", failed("switch tenant", identityaccess.ErrAccountInactive)
	}
	// A Träger school session moves within its Träger without a mapping
	// (#4654); a school outside it takes the account's own mapping.
	if school, ok, err := s.traegerSwitchTarget(ctx, accountID, tenantSlug, presentedFamilyID); err != nil || ok {
		if err != nil {
			return "", "", err
		}
		return s.switchTraegerSchool(ctx, account, school, presentedFamilyID, ipAddress, userAgent)
	}
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return "", "", err
	}
	minted, err := s.mintAccountSession(ctx, mintRequest{
		portal: domain.PortalTenant, account: account, tenantID: metadata.TenantID,
		op: "switch tenant", precondition: s.demoTenantLock,
		sourceFamilyID: presentedFamilyID, retireFamilyID: presentedFamilyID,
		audit: &mintAudit{event: domain.AuthEventTenantSwitch, ipAddress: ipAddress, userAgent: userAgent, schoolID: metadata.TenantID},
	})
	if err != nil {
		return "", "", err
	}
	s.logger.Info("tenant switch successful",
		slog.Int64("account_id", accountID),
		slog.Int64("new_tenant_id", metadata.TenantID))
	return minted.accessToken, minted.refreshToken, nil
}

// VerifyAccountTenantMembership reports whether the account has an active
// mapping for the school.
func (s *AccountAuthentication) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (_ bool, err error) {
	if s == nil {
		return false, errAccountAuthenticationUnavailable
	}
	ctx = bindRuntime(s.attach, ctx)
	var exists bool
	err = globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		var err error
		exists, _, err = s.store.HasActiveAccountTenant(txCtx, accountID, tenantID)
		return err
	})
	return exists, err
}
