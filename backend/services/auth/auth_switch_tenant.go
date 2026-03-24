package auth

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SwitchTenant authenticates a user to a different tenant and returns new tokens.
// The account must be active and have an active mapping to the requested tenant.
//
// Invalidates all existing refresh tokens for the account before issuing a new one.
// This prevents stale tokens from a previous tenant (or a previous user session on
// the same browser) from being refreshed after a tenant switch or logout.
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

	// 3. Invalidate all existing refresh tokens for this account.
	// Uses WithAdminTx (BYPASSRLS) because auth.tokens has RLS enabled —
	// without bypassing, the tenant isolation policy silently filters out
	// tokens from other tenants, leaving them orphaned.
	if adminErr := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, _ bun.Tx) error {
		return s.repos.Token.DeleteByAccountID(ctx, account.ID)
	}); adminErr != nil {
		s.getLogger().Warn("switch-tenant: failed to delete old tokens",
			slog.Int64("account_id", account.ID),
			slog.Any("error", adminErr),
		)
		// Non-fatal: proceed with creating the new token even if cleanup fails.
		// The old tokens will eventually expire.
	}

	// 4. Create refresh token with resolved tenant ID
	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID)
	if err != nil {
		return "", "", err
	}

	// 5. Build JWT claims with the new tenant
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, account.Email)

	// 6. Generate token pair and log success
	s.getLogger().Info("tenant switch successful",
		slog.Int64("account_id", accountID),
		slog.Int64("new_tenant_id", metadata.tenantID),
	)

	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, "", "", audit.EventTypeTenantSwitch)
}
