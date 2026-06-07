package auth

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// guardianRoleName must match auth.roles.name for the guardian base
// role (the one assigned by guardian_invitation_service.linkProfileToAccount
// and by decision_service.ensureGuardianRoleForTenant). Lowercase to
// match the DB seed.
const guardianRoleName = "guardian"

// LoginParent authenticates a parent and issues a parent-scope JWT
// with no tenant_id binding. The token works against parent-scoped
// endpoints regardless of which school the parent's children attend;
// per-action tenant context is derived from the picked child + the
// auth.account_tenants mapping table.
//
// Refuses accounts that don't have a guardian role on at least one
// linked tenant (ErrAccountNoGuardianRole). Frontend renders this as
// "this email isn't registered as a parent — please use the staff
// login at https://{tenant}.{TENANT_DOMAIN}/".
func (s *Service) LoginParent(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginParentWithAudit(ctx, email, password, "", "")
}

// LoginParentWithAudit is the audit-logged variant of LoginParent.
func (s *Service) LoginParentWithAudit(
	ctx context.Context,
	email, password, ipAddress, userAgent string,
) (string, string, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}

	// Walk every active tenant mapping and check for the guardian role.
	// Done inside an admin tx because account_tenants + account_roles
	// have RLS that requires app.current_tenant_id, which we don't have
	// during a public login.
	//
	// We also capture the first tenant where the role hits — that
	// tenant becomes the housekeeping tenant_id for the refresh token
	// row (auth.tokens has a NOT NULL FK to platform.schools, so we
	// can't store tenant_id=0). The JWT itself carries tenant_id=0 +
	// scope=parent; the row's tenant is purely for the FK constraint
	// and RLS bookkeeping. Refresh path keys off scope, not tenant_id.
	hasGuardianRole := false
	var firstGuardianTenantID int64
	if err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)
		mappings, listErr := txService.repos.AccountTenant.FindActiveByAccountID(adminCtx, account.ID)
		if listErr != nil {
			return listErr
		}
		for _, m := range mappings {
			roles, roleErr := txService.repos.AccountRole.FindByAccountIDForTenant(adminCtx, account.ID, m.TenantID)
			if roleErr != nil {
				continue
			}
			for _, ar := range roles {
				role, lookupErr := txService.repos.Role.FindByID(adminCtx, ar.RoleID)
				if lookupErr != nil || role == nil {
					continue
				}
				if strings.EqualFold(role.Name, guardianRoleName) {
					hasGuardianRole = true
					firstGuardianTenantID = m.TenantID
					return nil
				}
			}
		}
		return nil
	}); err != nil {
		return "", "", &AuthError{Op: "parent login: enumerate tenants", Err: err}
	}

	if !hasGuardianRole {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Not a guardian at any school")
		return "", "", &AuthError{Op: "parent login", Err: ErrAccountNoGuardianRole}
	}

	// Pin the refresh token row to the first guardian tenant for the
	// FK + RLS. The token's purpose is parent-scope (cross-tenant on
	// read), but the row needs a real tenant_id to satisfy the schema.
	token, err := s.createRefreshTokenWithRetry(ctx, account, firstGuardianTenantID)
	if err != nil {
		return "", "", err
	}

	// Build minimal metadata for parent claims. Roles list carries
	// only "guardian" — handlers that need per-tenant role detail
	// re-query at request time inside the admin tx.
	metadata := s.buildParentMetadata(account)

	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, email)

	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
}

// buildParentMetadata constructs the accountMetadata struct for a
// parent-scope token. tenantID and orgID are zero on purpose. First/
// last name lookups are skipped here — the parent might have
// per-tenant person rows with the same display name, and the JWT
// claim is only used for greetings; the frontend renders the
// preferred name from the cross-tenant /me/children response or
// from the email address.
func (s *Service) buildParentMetadata(account *authModels.Account) *accountMetadata {
	username := s.extractUsername(account)

	return &accountMetadata{
		roleNames:      []string{guardianRoleName},
		permissionStrs: nil,
		username:       username,
		firstName:      "",
		lastName:       "",
		isAdmin:        false,
		tenantID:       0,
		orgID:          0,
		scope:          tenant.ScopeParent,
	}
}

// IsGuardianOnlyForTenant reports whether the account has the guardian
// role and no other (admin/staff/teacher/etc.) for the given tenant.
// Used by the standard tenant LoginWithAudit to refuse guardians who
// try to log in at a tenant subdomain — they should be using the
// parents portal.
//
// "Only guardian" is a strict check: the role list must be exactly
// ["guardian"] case-insensitive after de-duplication. An account that
// is also a teacher at the same school is allowed through.
func IsGuardianOnlyForTenant(roleNames []string) bool {
	if len(roleNames) == 0 {
		return false
	}
	for _, r := range roleNames {
		if !strings.EqualFold(r, guardianRoleName) {
			return false
		}
	}
	return true
}

// isGuardianOnlyAccount is the refresh-path backward-compat check for
// parent-scope detection on refresh tokens that predate
// RefreshClaims.Scope. Returns true when the account has only the
// guardian role at the refresh's pinned tenant. New tokens carry an
// explicit Scope claim and don't need this fallback.
//
// Errors from role loading are treated as "not guardian-only" — the
// caller falls back to the regular tenant refresh path, which is the
// safe default for ambiguous cases.
func (s *Service) isGuardianOnlyAccount(ctx context.Context, account *authModels.Account, tenantID int64) bool {
	if account == nil || tenantID <= 0 {
		return false
	}
	err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)
		txService.ensureAccountRolesLoadedForTenant(ctx, account, tenantID)
		return nil
	})
	if err != nil {
		return false
	}
	return IsGuardianOnlyForTenant(s.extractRoleNames(account.Roles))
}
