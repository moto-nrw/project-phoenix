package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) FindParentCalendarFeedAccount(ctx context.Context, id int64) (domain.ParentCalendarFeedAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ParentCalendarFeedAccount{}, false, domain.OperationStats{}, err
	}
	return scanParentCalendarFeedAccount(ctx, db.NewRaw(`SELECT id, email, active, COALESCE(calendar_feed_token, '') AS token_hash FROM auth.accounts WHERE id = ?`, id))
}

func (s *Store) FindParentCalendarFeedOwner(ctx context.Context, hash string) (domain.ParentCalendarFeedAccount, bool, domain.OperationStats, error) {
	if hash == "" {
		return domain.ParentCalendarFeedAccount{}, false, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.ParentCalendarFeedAccount{}, false, domain.OperationStats{}, err
	}
	return scanParentCalendarFeedAccount(ctx, db.NewRaw(`SELECT id, email, active, COALESCE(calendar_feed_token, '') AS token_hash FROM auth.accounts WHERE calendar_feed_token = ?`, hash))
}

func scanParentCalendarFeedAccount(ctx context.Context, query *bun.RawQuery) (domain.ParentCalendarFeedAccount, bool, domain.OperationStats, error) {
	var account domain.ParentCalendarFeedAccount
	started := time.Now()
	err := query.Scan(ctx, &account)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ParentCalendarFeedAccount{}, false, stats, nil
	}
	if err != nil {
		return domain.ParentCalendarFeedAccount{}, false, stats, fmt.Errorf("read parent calendar feed account: %w", err)
	}
	stats.Rows = 1
	return account, true, stats, nil
}

// EnsureParentCalendarFeedToken claims only an unset credential. Concurrent
// losers read the winning hash without overwriting it.
func (s *Store) EnsureParentCalendarFeedToken(ctx context.Context, id int64, hash string) (string, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return "", domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.accounts SET calendar_feed_token = ? WHERE id = ? AND (calendar_feed_token IS NULL OR calendar_feed_token = '')`, hash, id).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return "", stats, fmt.Errorf("ensure parent calendar feed token: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr == nil && rows > 0 {
		stats.Rows = rows
		return hash, stats, nil
	}
	account, _, readStats, err := s.FindParentCalendarFeedAccount(ctx, id)
	stats.Add(readStats)
	return account.TokenHash, stats, err
}

func (s *Store) RotateParentCalendarFeedToken(ctx context.Context, id int64, hash string) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw("UPDATE auth.accounts SET calendar_feed_token = ? WHERE id = ?", hash, id).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("rotate parent calendar feed token: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
