package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Account refresh sessions (auth.tokens) join the caller's transaction and
// otherwise run on the root connection: login, refresh, switch and logout open
// the administrative transaction their rotation and audit evidence must
// commit in, and tenant-scoped callers already hold their tenant transaction.
// The tenant filter the store applies comes from the caller's context, so no
// operation here opens a tenant transaction of its own.

func (s *Service) FindAccountSession(ctx context.Context, token string) (domain.AccountSession, error) {
	return s.findAccountSession(ctx, "find_account_session", token, false)
}

func (s *Service) FindAccountSessionForUpdate(ctx context.Context, token string) (domain.AccountSession, error) {
	return s.findAccountSession(ctx, "find_account_session_for_update", token, true)
}

func (s *Service) findAccountSession(ctx context.Context, operation, token string, forUpdate bool) (result domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return domain.ErrAccountSessionNotFound
		}
		session, found, queryStats, findErr := s.sessions.FindAccountSessionByToken(txCtx, token, forUpdate)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrAccountSessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (s *Service) LatestAccountSessionInFamily(ctx context.Context, familyID string) (result domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "latest_account_session_in_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		if familyID == "" {
			return domain.ErrAccountSessionNotFound
		}
		session, found, queryStats, findErr := s.sessions.LatestAccountSessionInFamily(txCtx, familyID)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrAccountSessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (s *Service) ListAccountSessions(ctx context.Context, filter domain.AccountSessionFilter) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "list_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		sessions, queryStats, listErr := s.sessions.ListAccountSessions(txCtx, filter, time.Now())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = sessions
		return nil
	})
	return result, err
}

func (s *Service) CountExpiredAccountSessions(ctx context.Context) (result int, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "count_expired_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := s.sessions.CountExpiredAccountSessions(txCtx, time.Now())
		stats.Add(queryStats)
		if countErr != nil {
			return countErr
		}
		result = count
		return nil
	})
	return result, err
}

func (s *Service) ListInactiveAccountIDsWithLiveSessions(ctx context.Context) (result []int64, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "list_inactive_accounts_with_live_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, queryStats, listErr := s.sessions.ListInactiveAccountIDsWithLiveSessions(txCtx, time.Now())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = ids
		return nil
	})
	return result, err
}

func (s *Service) HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (result bool, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "has_live_account_sessions_created_after", func(txCtx context.Context, stats *domain.OperationStats) error {
		exists, queryStats, existsErr := s.sessions.HasLiveAccountSessionsCreatedAfter(txCtx, accountID, since, time.Now())
		stats.Add(queryStats)
		if existsErr != nil {
			return existsErr
		}
		result = exists
		return nil
	})
	return result, err
}

// CreateAccountSession validates the session and pins it to the caller's
// tenant when the caller did not name one. Login resolves the tenant from
// the account's mappings before it mints, so the row is always attributable
// to one school.
func (s *Service) CreateAccountSession(ctx context.Context, session domain.AccountSession) (result domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "create_account_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := session.Validate(); validateErr != nil {
			return validateErr
		}
		if session.TenantID == 0 {
			session.TenantID = s.tenantOf(txCtx)
		}
		stored, queryStats, insertErr := s.sessions.InsertAccountSession(txCtx, session)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

// MarkAccountSessionRotated records the hand-off exactly once: a session that
// was already rotated (or never existed) is reported, never silently
// re-rotated, because a second hand-off is the replay signal the refresh
// flow revokes the whole family on.
func (s *Service) MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return s.run(ctx, s.tx.RunPlatform, "mark_account_session_rotated", func(txCtx context.Context, stats *domain.OperationStats) error {
		rotated, queryStats, err := s.sessions.MarkAccountSessionRotated(txCtx, id, replacementToken, recoveryProofHash, rotatedAt)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !rotated {
			return domain.ErrAccountSessionRotated
		}
		return nil
	})
}

func (s *Service) DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error {
	return s.run(ctx, s.tx.RunPlatform, "delete_expired_rotated_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.sessions.DeleteExpiredRotatedAccountSessions(txCtx, accountID, now)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error {
	return s.run(ctx, s.tx.RunPlatform, "retire_account_session_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.sessions.RetireAccountSessionFamily(txCtx, accountID, familyID, expiry)
		stats.Add(queryStats)
		return err
	})
}

// EnforceAccountSessionCap keeps at most keep live sessions in the portal
// group of portalScope and returns the sessions it evicted, closest to
// expiry first, so the caller can record its audit evidence in the same
// transaction. Rotated hand-offs and expired rows have their own lifecycle
// and are never counted or evicted here.
func (s *Service) EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "enforce_account_session_cap", func(txCtx context.Context, stats *domain.OperationStats) error {
		live, listStats, listErr := s.sessions.ListLiveAccountSessionsForCap(txCtx, accountID, domain.CapPortalScopes(portalScope), time.Now())
		stats.Add(listStats)
		if listErr != nil {
			return listErr
		}
		if keep < 0 {
			keep = 0
		}
		if len(live) <= keep {
			return nil
		}
		ids := make([]int64, 0, len(live)-keep)
		for _, session := range live[keep:] {
			ids = append(ids, session.ID)
		}
		deleted, deleteStats, deleteErr := s.sessions.DeleteAccountSessionsByID(txCtx, ids)
		stats.Add(deleteStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) DeleteAccountSession(ctx context.Context, id int64) error {
	return s.run(ctx, s.tx.RunPlatform, "delete_account_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.sessions.DeleteAccountSession(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RevokeAccountSessionFamily(ctx context.Context, familyID string) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_account_session_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByFamily(txCtx, familyID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

// RevokeAccountSessionsInTenant deletes the account's sessions at the
// caller's school only. An administrative caller must not widen this to
// other schools; account-wide wipes use RevokeAllAccountSessions.
func (s *Service) RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_account_sessions_in_tenant", func(txCtx context.Context, stats *domain.OperationStats) error {
		if s.tenantOf(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByAccount(txCtx, accountID, true, time.Time{})
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) RevokeAllAccountSessions(ctx context.Context, accountID int64) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_all_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByAccount(txCtx, accountID, false, time.Time{})
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_account_sessions_created_at_or_before", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByAccount(txCtx, accountID, false, cutoff)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) RevokeTenantAccountSessions(ctx context.Context, tenantID int64) (result []domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "revoke_tenant_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return domain.ErrTenantRequired
		}
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByTenant(txCtx, tenantID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}

func (s *Service) DeleteExpiredAccountSessions(ctx context.Context) (result int, err error) {
	err = s.run(ctx, s.tx.RunPlatform, "delete_expired_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteExpiredAccountSessions(txCtx, time.Now())
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	return result, err
}
