package application

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// LoginParentWithAudit authenticates a parent and issues a parent-scope JWT
// with no tenant binding: the token works against parent-scoped endpoints
// regardless of which school the children attend. Accounts without a
// guardian role on at least one active mapping are refused.
func (s *AccountAuthentication) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}
	hasGuardianRole, firstGuardianTenantID, err := s.findGuardianTenantForAccount(ctx, account.ID)
	if err != nil {
		return "", "", failed("parent login: enumerate tenants", err)
	}
	if !hasGuardianRole {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Not a guardian at any school")
		return "", "", failed("parent login", domain.ErrAccountNoGuardianRole)
	}
	// The refresh session row is pinned to the first guardian tenant for the
	// foreign key and RLS; the token itself is cross-tenant on read.
	session, err := s.createRefreshSession(ctx, account, firstGuardianTenantID, domain.ScopeParent)
	if err != nil {
		return "", "", err
	}
	metadata := buildParentMetadata(account)
	access, refresh := buildClaims(account, session, metadata, email)
	return s.generateAndLogTokens(ctx, account.ID, access, refresh, ipAddress, userAgent, domain.AuthEventLogin)
}

// buildParentMetadata is the claims material of a parent-scope token: the
// guardian role only, no tenant, no names. Handlers re-query per-tenant
// detail at request time.
func buildParentMetadata(account domain.LoginAccount) *domain.AccountClaimsPayload {
	return &domain.AccountClaimsPayload{
		RoleNames:   []string{domain.GuardianRoleName},
		Permissions: nil,
		Username:    account.Username,
		Scope:       domain.ScopeParent,
	}
}

// findGuardianTenantForAccount checks every active mapping for the guardian
// role inside an administrative transaction; the parent flows have no
// tenant transaction yet.
func (s *AccountAuthentication) findGuardianTenantForAccount(ctx context.Context, accountID int64) (bool, int64, error) {
	hasGuardianRole := false
	var firstGuardianTenantID int64
	if err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		tenantIDs, _, listErr := s.store.ListActiveTenantIDs(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		for _, tenantID := range tenantIDs {
			roles, _, roleErr := s.store.ListAccountRolesAtTenant(adminCtx, accountID, tenantID, false)
			if roleErr != nil {
				// A real database failure must not read as "not a guardian".
				return roleErr
			}
			if domain.HasGuardianRole(roles) {
				hasGuardianRole = true
				firstGuardianTenantID = tenantID
				return nil
			}
		}
		return nil
	}); err != nil {
		return false, 0, err
	}
	return hasGuardianRole, firstGuardianTenantID, nil
}

// FindGuardianTenant is findGuardianTenantForAccount for the retained
// password-reset flow, which routes guardian accounts to the parents reset
// link.
func (s *AccountAuthentication) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	return s.findGuardianTenantForAccount(ctx, accountID)
}

// isGuardianOnlyAccountInTx is the refresh-path backward-compat check for
// parent-scope detection on refresh tokens that predate the scope claim. It
// runs on the rotation transaction; a failed role load is an error, never a
// "no", because a database blip must not mint a tenant JWT for a guardian.
func (s *AccountAuthentication) isGuardianOnlyAccountInTx(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if accountID <= 0 || tenantID <= 0 {
		return false, nil
	}
	roles, err := s.loadAccountRolesForTenant(ctx, accountID, tenantID)
	if err != nil {
		return false, err
	}
	return domain.IsGuardianOnly(domain.RoleNames(roles)), nil
}

// SwitchTenant authenticates an account to a different school and returns
// new tokens. Refresh sessions of other families stay alive; the presented
// family is retired instead, with a short grace, because the browser
// replaces its session with the returned pair (#2952). An empty
// presentedFamilyID skips the retirement.
func (s *AccountAuthentication) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	account, found, _, err := s.store.FindLoginAccount(ctx, accountID, false)
	if err != nil || !found {
		s.logger.Warn("switch-tenant: account not found",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
		return "", "", failed("switch tenant", domain.ErrAccountNotFound)
	}
	if !account.Active {
		return "", "", failed("switch tenant", domain.ErrAccountInactive)
	}
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return "", "", err
	}
	// A school where the account is Lehrkraft-only has no reachable
	// tenant-portal surface, even for a still-valid tenant session.
	if domain.IsSchoolPortalOnly(metadata.Roles) {
		return "", "", failed("switch tenant", domain.ErrMustUseSchoolPortal)
	}
	session, err := s.createRefreshSessionGuarded(ctx, account, metadata.TenantID, metadata.Scope, nil, presentedFamilyID)
	if err != nil {
		return "", "", err
	}
	access, refresh := buildClaims(account, session, metadata, account.Email)
	s.logger.Info("tenant switch successful",
		slog.Int64("account_id", accountID),
		slog.Int64("new_tenant_id", metadata.TenantID))
	return s.generateAndLogTokens(ctx, account.ID, access, refresh, "", "", domain.AuthEventTenantSwitch)
}

// VerifyAccountTenantMembership reports whether the account has an active
// mapping for the school.
func (s *AccountAuthentication) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	exists, _, err := s.store.HasActiveAccountTenant(ctx, accountID, tenantID)
	return exists, err
}
