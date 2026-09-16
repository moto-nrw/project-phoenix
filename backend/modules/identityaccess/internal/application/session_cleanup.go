package application

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CountExpiredTokens reports how many refresh sessions a cleanup would
// remove, across every school.
func (s *AccountAuthentication) CountExpiredTokens(ctx context.Context) (int, error) {
	count, err := s.sessions.CountExpiredAccountSessions(ctx)
	if err != nil {
		return 0, failed("count expired tokens", err)
	}
	return count, nil
}

// CleanupExpiredTokens removes expired refresh sessions and then reconciles
// revocations whose after-commit follow-up did not complete.
func (s *AccountAuthentication) CleanupExpiredTokens(ctx context.Context) (int, error) {
	count, err := s.sessions.DeleteExpiredAccountSessions(ctx)
	if err != nil {
		return 0, failed("cleanup expired tokens", err)
	}
	if recErr := s.reconcileRevokedSessions(ctx); recErr != nil {
		s.logger.Warn("revocation follow-up reconciliation failed",
			slog.Any("error", recErr))
	}
	return count, nil
}

// reconcileRevokedSessions wipes deactivated accounts that still hold live
// sessions, completes recorded account-wide wipes and drops orphaned push
// subscriptions, in one administrative transaction.
func (s *AccountAuthentication) reconcileRevokedSessions(ctx context.Context) error {
	return s.runtime.WithAdminTx(s.runtime.WithoutTransaction(ctx), func(adminCtx context.Context) error {
		seen := map[int64]struct{}{}
		inactive, err := s.sessions.ListInactiveAccountIDsWithLiveSessions(adminCtx)
		if err != nil {
			return err
		}
		for _, id := range inactive {
			if id > 0 {
				seen[id] = struct{}{}
			}
		}
		for accountID := range seen {
			if err := s.wipeAccountWideIndependently(adminCtx, accountID, "account_deactivated", "", ""); err != nil {
				return err
			}
		}
		pending, err := s.audit.ListPendingAccountWideWipes(adminCtx)
		if err != nil {
			return err
		}
		for _, wipe := range pending {
			if _, already := seen[wipe.AccountID]; already {
				continue
			}
			reason := wipe.Reason
			if !domain.IsAccountWideRevocation(reason) {
				reason = "administrative_revoke"
			}
			if err := s.finishScheduledAccountWideWipe(adminCtx, wipe.AccountID, reason, "", "", true); err != nil {
				return err
			}
		}
		return s.push.DeleteOrphaned(adminCtx)
	})
}

// ListActiveSessions returns the account's live refresh sessions in the
// caller's tenant scope.
func (s *AccountAuthentication) ListActiveSessions(ctx context.Context, accountID int64) ([]domain.AccountSession, error) {
	sessions, err := s.sessions.ListAccountSessions(ctx, domain.AccountSessionFilter{AccountID: accountID, Liveness: domain.AccountSessionsLive})
	if err != nil {
		return nil, failed("get active tokens", err)
	}
	return sessions, nil
}

// ListSessionIDs returns the identifiers of every session of the account in
// the caller's tenant scope, for revision fingerprints.
func (s *AccountAuthentication) ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error) {
	sessions, err := s.sessions.ListAccountSessions(ctx, domain.AccountSessionFilter{AccountID: accountID})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	return ids, nil
}
