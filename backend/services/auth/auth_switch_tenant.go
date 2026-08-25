package auth

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

// SwitchTenant authenticates a user to a different tenant and returns new tokens.
// The account must be active and have an active mapping to the requested tenant.
//
// Old refresh tokens are intentionally kept alive — the account may have active
// sessions on other devices or tenants. Logout revokes only the presented
// token family. Login caps active sessions at five per portal.
func (s *Service) SwitchTenant(ctx context.Context, accountID int64, tenantSlug string) (string, string, error) {
	// 1. Look up the account by ID
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		s.getLogger().Warn("switch-tenant: account not found",
			slog.Int64("account_id", accountID),
			slog.Any("error", err),
		)
		return "", "", &AuthError{Op: "switch tenant", Err: ErrAccountNotFound}
	}

	if !account.Active {
		return "", "", &AuthError{Op: "switch tenant", Err: ErrAccountInactive}
	}

	// 2. Load account metadata for the new tenant (includes slug-based tenant resolution
	//    and access verification via resolveAccountTenantBySlug)
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return "", "", err
	}
	// The target tenant may be a school where this account is Lehrkraft-only.
	// Such an account has no reachable tenant-portal surface after the #2207
	// cutover, even if it arrived here through a still-valid tenant session.
	if IsSchoolPortalOnlyForTenant(account.Roles) {
		return "", "", &AuthError{Op: "switch tenant", Err: ErrMustUseSchoolPortal}
	}

	// 3. Create refresh token with resolved tenant ID
	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID, metadata.scope)
	if err != nil {
		return "", "", err
	}

	// 4. Build JWT claims with the new tenant
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)

	// 5. Generate token pair and log success
	s.getLogger().Info("tenant switch successful",
		slog.Int64("account_id", accountID),
		slog.Int64("new_tenant_id", metadata.tenantID),
	)

	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, "", "", audit.EventTypeTenantSwitch)
}
