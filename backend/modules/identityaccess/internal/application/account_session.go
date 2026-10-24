package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Account refresh sessions (auth.tokens) join the caller's transaction and
// otherwise run on the root connection: login, refresh, switch and logout open
// the administrative transaction their rotation and audit evidence must
// commit in, and tenant-scoped callers already hold their tenant transaction.
// The tenant filter the store applies comes from the caller's context, so no
// operation here opens a tenant transaction of its own.

func (s *Service) FindAccountSession(ctx context.Context, token string) (identityaccess.AccountSession, error) {
	session, err := s.findAccountSessionByToken(ctx, token)
	if err != nil {
		return identityaccess.AccountSession{}, fmt.Errorf("identity access: find account session: %w", MapError(err))
	}
	return identityaccess.AccountSession(session), nil
}

func (s *Service) FindAccountSessionForUpdate(ctx context.Context, token string) (identityaccess.AccountSession, error) {
	session, err := s.findAccountSessionForUpdate(ctx, token)
	if err != nil {
		return identityaccess.AccountSession{}, fmt.Errorf("identity access: find account session for update: %w", MapError(err))
	}
	return identityaccess.AccountSession(session), nil
}

func (s *Service) findAccountSessionByToken(ctx context.Context, token string) (domain.AccountSession, error) {
	return s.findAccountSession(ctx, "find_account_session", token, false)
}

func (s *Service) findAccountSessionForUpdate(ctx context.Context, token string) (domain.AccountSession, error) {
	return s.findAccountSession(ctx, "find_account_session_for_update", token, true)
}

func (s *Service) findAccountSession(ctx context.Context, operation, token string, forUpdate bool) (result domain.AccountSession, err error) {
	err = s.run(ctx, s.tx.RunPlatform, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return identityaccess.ErrAccountSessionNotFound
		}
		session, found, queryStats, findErr := s.sessions.FindAccountSessionByToken(txCtx, token, forUpdate)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrAccountSessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (s *Service) LatestAccountSessionInFamily(ctx context.Context, familyID string) (identityaccess.AccountSession, error) {
	var result domain.AccountSession
	err := s.run(ctx, s.tx.RunPlatform, "latest_account_session_in_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		if familyID == "" {
			return identityaccess.ErrAccountSessionNotFound
		}
		session, found, queryStats, findErr := s.sessions.LatestAccountSessionInFamily(txCtx, familyID)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrAccountSessionNotFound
		}
		result = session
		return nil
	})
	if err != nil {
		return identityaccess.AccountSession{}, fmt.Errorf("identity access: latest account session in family: %w", MapError(err))
	}
	return identityaccess.AccountSession(result), nil
}

func (s *Service) ListAccountSessions(ctx context.Context, public identityaccess.AccountSessionFilter) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	filter := domain.AccountSessionFilter{
		AccountID: public.AccountID, FamilyID: public.FamilyID, Mobile: public.Mobile, Liveness: domain.AccountSessionLiveness(public.Liveness),
	}
	err := s.run(ctx, s.tx.RunPlatform, "list_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		sessions, queryStats, listErr := s.sessions.ListAccountSessions(txCtx, filter, time.Now())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = sessions
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: list account sessions: %w", MapError(err))
	}
	return accountSessions(result), nil
}

func (s *Service) CountExpiredAccountSessions(ctx context.Context) (int, error) {
	var result int
	err := s.run(ctx, s.tx.RunPlatform, "count_expired_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := s.sessions.CountExpiredAccountSessions(txCtx, time.Now())
		stats.Add(queryStats)
		if countErr != nil {
			return countErr
		}
		result = count
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("identity access: count expired account sessions: %w", MapError(err))
	}
	return result, nil
}

func (s *Service) ListInactiveAccountIDsWithLiveSessions(ctx context.Context) ([]int64, error) {
	var result []int64
	err := s.run(ctx, s.tx.RunPlatform, "list_inactive_accounts_with_live_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, queryStats, listErr := s.sessions.ListInactiveAccountIDsWithLiveSessions(txCtx, time.Now())
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = ids
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: list inactive accounts with live sessions: %w", MapError(err))
	}
	return result, nil
}

func (s *Service) HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error) {
	var result bool
	err := s.run(ctx, s.tx.RunPlatform, "has_live_account_sessions_created_after", func(txCtx context.Context, stats *domain.OperationStats) error {
		exists, queryStats, existsErr := s.sessions.HasLiveAccountSessionsCreatedAfter(txCtx, accountID, since, time.Now())
		stats.Add(queryStats)
		if existsErr != nil {
			return existsErr
		}
		result = exists
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("identity access: has live account sessions created after: %w", MapError(err))
	}
	return result, nil
}

// CreateAccountSession validates the session and pins it to the caller's
// tenant when the caller did not name one. Login resolves the tenant from
// the account's mappings before it mints, so the row is always attributable
// to one school.
func (s *Service) CreateAccountSession(ctx context.Context, public identityaccess.AccountSession) (identityaccess.AccountSession, error) {
	var result domain.AccountSession
	session := domain.AccountSession(public)
	err := s.run(ctx, s.tx.RunPlatform, "create_account_session", func(txCtx context.Context, stats *domain.OperationStats) error {
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
	if err != nil {
		return identityaccess.AccountSession{}, fmt.Errorf("identity access: create account session: %w", MapError(err))
	}
	return identityaccess.AccountSession(result), nil
}

// MarkAccountSessionRotated records the hand-off exactly once: a session that
// was already rotated (or never existed) is reported, never silently
// re-rotated, because a second hand-off is the replay signal the refresh
// flow revokes the whole family on.
func (s *Service) MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	err := s.run(ctx, s.tx.RunPlatform, "mark_account_session_rotated", func(txCtx context.Context, stats *domain.OperationStats) error {
		rotated, queryStats, err := s.sessions.MarkAccountSessionRotated(txCtx, id, replacementToken, recoveryProofHash, rotatedAt)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !rotated {
			return identityaccess.ErrAccountSessionRotated
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("identity access: mark account session rotated: %w", MapError(err))
	}
	return nil
}

func (s *Service) DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error {
	err := s.run(ctx, s.tx.RunPlatform, "delete_expired_rotated_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.sessions.DeleteExpiredRotatedAccountSessions(txCtx, accountID, now)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: delete expired rotated account sessions: %w", MapError(err))
	}
	return nil
}

func (s *Service) RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error {
	err := s.run(ctx, s.tx.RunPlatform, "retire_account_session_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.sessions.RetireAccountSessionFamily(txCtx, accountID, familyID, expiry)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: retire account session family: %w", MapError(err))
	}
	return nil
}

// EnforceAccountSessionCap keeps at most keep live sessions in the portal
// group of portalScope and returns the sessions it evicted, closest to
// expiry first, so the caller can record its audit evidence in the same
// transaction. Rotated hand-offs and expired rows have their own lifecycle
// and are never counted or evicted here.
func (s *Service) EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	err := s.run(ctx, s.tx.RunPlatform, "enforce_account_session_cap", func(txCtx context.Context, stats *domain.OperationStats) error {
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
	if err != nil {
		return nil, fmt.Errorf("identity access: enforce account session cap: %w", MapError(err))
	}
	return accountSessions(result), nil
}

func (s *Service) DeleteAccountSession(ctx context.Context, id int64) error {
	err := s.run(ctx, s.tx.RunPlatform, "delete_account_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.sessions.DeleteAccountSession(txCtx, id)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: delete account session: %w", MapError(err))
	}
	return nil
}

func (s *Service) RevokeAccountSessionFamily(ctx context.Context, familyID string) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	err := s.run(ctx, s.tx.RunPlatform, "revoke_account_session_family", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByFamily(txCtx, familyID)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke account session family: %w", MapError(err))
	}
	return accountSessions(result), nil
}

// RevokeAccountSessionsInTenant deletes the account's sessions at the
// caller's school only. An administrative caller must not widen this to
// other schools; account-wide wipes use RevokeAllAccountSessions.
func (s *Service) RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	err := s.run(ctx, s.tx.RunPlatform, "revoke_account_sessions_in_tenant", func(txCtx context.Context, stats *domain.OperationStats) error {
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
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke account sessions in tenant: %w", MapError(err))
	}
	return accountSessions(result), nil
}

func (s *Service) RevokeAllAccountSessions(ctx context.Context, accountID int64) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	err := s.run(ctx, s.tx.RunAdmin, "revoke_all_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByAccount(txCtx, accountID, false, time.Time{})
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke all account sessions: %w", MapError(err))
	}
	return accountSessions(result), nil
}

func (s *Service) RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	err := s.run(ctx, s.tx.RunAdmin, "revoke_account_sessions_created_at_or_before", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteAccountSessionsByAccount(txCtx, accountID, false, cutoff)
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke account sessions created at or before: %w", MapError(err))
	}
	return accountSessions(result), nil
}

func (s *Service) RevokeTenantAccountSessions(ctx context.Context, tenantID int64) ([]identityaccess.AccountSession, error) {
	var result []domain.AccountSession
	err := s.run(ctx, s.tx.RunPlatform, "revoke_tenant_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
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
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke tenant account sessions: %w", MapError(err))
	}
	return accountSessions(result), nil
}

func (s *Service) DeleteExpiredAccountSessions(ctx context.Context) (int, error) {
	var result int
	err := s.run(ctx, s.tx.RunPlatform, "delete_expired_account_sessions", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := s.sessions.DeleteExpiredAccountSessions(txCtx, time.Now())
		stats.Add(queryStats)
		if deleteErr != nil {
			return deleteErr
		}
		result = deleted
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired account sessions: %w", MapError(err))
	}
	return result, nil
}

func accountSessions(values []domain.AccountSession) []identityaccess.AccountSession {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.AccountSession, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.AccountSession(value))
	}
	return result
}

// domainAccountSessions hands the public sessions back to the session flows,
// whose rotation and audit rules read the domain rows.
func domainAccountSessions(values []identityaccess.AccountSession) []domain.AccountSession {
	if values == nil {
		return nil
	}
	result := make([]domain.AccountSession, 0, len(values))
	for _, value := range values {
		result = append(result, domain.AccountSession(value))
	}
	return result
}
