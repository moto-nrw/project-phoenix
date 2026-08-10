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

	hasGuardianRole, firstGuardianTenantID, err := s.findGuardianTenantForAccount(ctx, account.ID)
	if err != nil {
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

// findGuardianTenantForAccount checks every active tenant mapping for the
// guardian role. Done inside an admin tx because account_tenants +
// account_roles have RLS that requires app.current_tenant_id, which public
// parent auth flows do not have yet.
func (s *Service) findGuardianTenantForAccount(ctx context.Context, accountID int64) (bool, int64, error) {
	hasGuardianRole := false
	var firstGuardianTenantID int64

	if err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, tx bun.Tx) error {
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
				// caller fails loudly instead of silently reporting "not a
				// guardian" (which would skip token + email on a recovery).
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
				if strings.EqualFold(role.Name, guardianRoleName) {
					hasGuardianRole = true
					firstGuardianTenantID = m.TenantID
					return nil
				}
			}
		}
		return nil
	}); err != nil {
		return false, 0, err
	}

	return hasGuardianRole, firstGuardianTenantID, nil
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

// isGuardianOnlyAccountInTx is the refresh-path backward-compat check for
// parent-scope detection on refresh tokens that predate
// RefreshClaims.Scope. Returns true when the account has only the
// guardian role at the refresh's pinned tenant. New tokens carry an
// explicit Scope claim and don't need this fallback.
//
// InTx: it is called from refreshClaimsGuard, which already holds the
// phoenix_admin rotation transaction — hence the direct repository call
// instead of a nested WithAdminTx (bun does not nest, and a second connection
// would read a different snapshot than the rotation it is guarding).
//
// A failed role load is an ERROR, not a "no": it used to be swallowed into
// "not guardian-only", which was tolerable only because the caller
// immediately reloaded the same roles and failed there instead. From inside
// the guard the two outcomes are distinguishable and must stay that way — a
// DB blip must not mint a tenant-scope JWT for a guardian-only account.
func (s *Service) isGuardianOnlyAccountInTx(ctx context.Context, account *authModels.Account, tenantID int64) (bool, error) {
	if account == nil || tenantID <= 0 {
		return false, nil
	}
	if err := s.ensureAccountRolesLoadedForTenant(ctx, account, tenantID); err != nil {
		return false, err
	}
	return IsGuardianOnlyForTenant(s.extractRoleNames(account.Roles)), nil
}
