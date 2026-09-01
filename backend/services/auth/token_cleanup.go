package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Token Management

// CleanupExpiredTokens removes expired authentication tokens
func (s *Service) CleanupExpiredTokens(ctx context.Context) (int, error) {
	count, err := s.repos.Token.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired tokens", Err: err}
	}
	if recErr := s.reconcileRevokedSessions(ctx); recErr != nil {
		s.getLogger().Warn("revocation follow-up reconciliation failed",
			slog.Any("error", recErr),
		)
	}
	return count, nil
}

func (s *Service) reconcileRevokedSessions(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return tenant.WithAdminTx(s.withTenantRuntime(tenant.ContextWithoutTransaction(ctx)), s.db, func(adminCtx context.Context, _ bun.Tx) error {
		seen := map[int64]struct{}{}
		queue := func(ids []int64) {
			for _, id := range ids {
				if id > 0 {
					seen[id] = struct{}{}
				}
			}
		}
		if s.repos.Token != nil {
			inactive, err := s.repos.Token.ListInactiveAccountIDsWithLiveTokens(adminCtx)
			if err != nil {
				return err
			}
			queue(inactive)
		}
		for accountID := range seen {
			if err := s.wipeAccountWideIndependently(adminCtx, accountID, "account_deactivated", "", ""); err != nil {
				return err
			}
		}
		if s.repos.AuthEvent == nil {
			if s.repos.PushSubscription == nil {
				return nil
			}
			return s.repos.PushSubscription.DeleteOrphanedSubscriptions(adminCtx)
		}
		pending, err := s.repos.AuthEvent.ListPendingAccountWideWipes(adminCtx, time.Time{})
		if err != nil {
			return err
		}
		for _, wipe := range pending {
			if _, already := seen[wipe.AccountID]; already {
				continue
			}
			reason := wipe.Reason
			if !isAccountWideRevocation(reason) {
				reason = "administrative_revoke"
			}
			if err := s.finishScheduledAccountWideWipe(adminCtx, wipe.AccountID, reason, "", "", true); err != nil {
				return err
			}
		}
		if s.repos.PushSubscription == nil {
			return nil
		}
		return s.repos.PushSubscription.DeleteOrphanedSubscriptions(adminCtx)
	})
}

// CleanupExpiredPasswordResetTokens removes expired password reset tokens
func (s *Service) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	count, err := s.repos.PasswordResetToken.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired password reset tokens", Err: err}
	}
	return count, nil
}

// CleanupExpiredRateLimits purges stale password reset rate limit windows.
func (s *Service) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	if s.repos.PasswordResetRateLimit == nil {
		return 0, nil
	}

	count, err := s.repos.PasswordResetRateLimit.CleanupExpired(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup password reset rate limits", Err: err}
	}

	s.getLogger().Info("password reset rate limit cleanup completed",
		slog.Int("records_deleted", count))
	return count, nil
}

// RevokeAllTokens revokes all tokens for an account
func (s *Service) RevokeAllTokens(ctx context.Context, accountID int) error {
	return s.RevokeAllTokensWithReason(ctx, accountID, "administrative_revoke")
}

func (s *Service) RevokeAllTokensWithReason(ctx context.Context, accountID int, reason string) error {
	if isAccountWideRevocation(reason) {
		if err := s.scheduleAccountWideRevoke(ctx, int64(accountID), reason, "", ""); err != nil {
			return &AuthError{Op: "revoke all tokens", Err: err}
		}
		return nil
	}
	var revoked []*auth.Token
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		tokens, txErr := s.deleteAccountTokensWithAudit(txCtx, int64(accountID), reason, "", "")
		if txErr != nil {
			return txErr
		}
		revoked = tokens
		return nil
	})
	if err != nil {
		return &AuthError{Op: "revoke all tokens", Err: err}
	}
	s.queuePushCleanup(ctx, int64(accountID), revoked, reason)
	return nil
}

// RevokeTokensByTenantID deletes all refresh tokens for a given tenant.
// Used during soft-delete to immediately cut off session refresh for all users of the school.
func (s *Service) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	count := 0
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		deleted, err := s.repos.Token.DeleteByTenantIDReturning(txCtx, tenantID)
		if err != nil {
			return err
		}
		if err := s.auditRevokedTokens(txCtx, deleted, "tenant_deleted", "", ""); err != nil {
			return err
		}
		count = len(deleted)
		return nil
	})
	if err != nil {
		return 0, &AuthError{Op: "revoke tokens by tenant", Err: err}
	}
	return count, nil
}

// GetActiveTokens retrieves all active tokens for an account
func (s *Service) GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error) {
	filters := map[string]interface{}{
		"account_id": int64(accountID),
		"active":     true,
	}

	tokens, err := s.repos.Token.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "get active tokens", Err: err}
	}
	return tokens, nil
}

// logAuthEvent logs an authentication event for audit purposes
func (s *Service) logAuthEvent(ctx context.Context, accountID int64, eventType string, success bool, ipAddress, userAgent string, errorMessage string) error {
	event := audit.NewAuthEvent(accountID, eventType, success, ipAddress)

	// Login/refresh/logout are public routes — tenant.FromContext is 0.
	// Resolve from account_tenants so audit events get the correct tenant.
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 && accountID > 0 {
		tenantID, _, _ = s.resolveAccountTenant(ctx, accountID, "")
	}
	if tenantID > 0 {
		event.SetTenantID(tenantID)
	}
	event.UserAgent = userAgent
	if errorMessage != "" {
		event.ErrorMessage = errorMessage
	}

	if tenantID <= 0 {
		return fmt.Errorf("audit auth event: tenant is required")
	}
	if s.audit == nil {
		return fmt.Errorf("audit auth event: command is not configured")
	}
	if hasAmbientTx(ctx) {
		return s.audit.Append(ctx, event)
	}
	if s.db == nil {
		return fmt.Errorf("audit auth event: database is not configured")
	}
	auditCtx := tenant.WithTenantID(s.withTenantRuntime(ctx), tenantID)
	return tenant.WithTenantTx(auditCtx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.audit.Append(txCtx, event)
	})
}
