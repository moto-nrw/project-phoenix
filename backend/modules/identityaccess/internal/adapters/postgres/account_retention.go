package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// EraseUnusedAccounts calls the database workflow of #4043 (ADR 0065). The
// function decides which accounts no school needs and tombstones them; only
// phoenix_admin may call it, so it fails outside the administrative
// transaction of the token cleanup.
func (s *Store) EraseUnusedAccounts(ctx context.Context) ([]domain.ErasedAccount, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		AccountID int64          `bun:"account_id"`
		Avatar    sql.NullString `bun:"avatar"`
	}
	started := time.Now()
	err = db.NewRaw(`SELECT account_id, avatar FROM auth.erase_unused_accounts()`).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: erase unused accounts: %w", err)
	}
	var erased []domain.ErasedAccount
	index := map[int64]int{}
	for _, row := range rows {
		position, seen := index[row.AccountID]
		if !seen {
			position = len(erased)
			index[row.AccountID] = position
			erased = append(erased, domain.ErasedAccount{ID: row.AccountID})
		}
		if row.Avatar.Valid {
			erased[position].AvatarFiles = append(erased[position].AvatarFiles, row.Avatar.String)
		}
	}
	stats.Rows = int64(len(erased))
	return erased, stats, nil
}

// PendingAvatarCleanups locks one bounded batch until the caller commits.
// Failed files move behind unattempted entries so they cannot starve them.
func (s *Store) PendingAvatarCleanups(ctx context.Context) ([]domain.AvatarCleanup, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var pending []domain.AvatarCleanup
	err = db.NewRaw(`SELECT id, avatar FROM auth.account_avatar_file_cleanup
		ORDER BY attempted_at, id LIMIT 100 FOR UPDATE SKIP LOCKED`).Scan(ctx, &pending)
	return pending, err
}

func (s *Store) FinishAvatarCleanup(ctx context.Context, id int64, removed bool) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	if removed {
		_, err = db.NewRaw(`DELETE FROM auth.account_avatar_file_cleanup WHERE id = ?`, id).Exec(ctx)
	} else {
		_, err = db.NewRaw(`UPDATE auth.account_avatar_file_cleanup
			SET attempted_at = clock_timestamp() WHERE id = ?`, id).Exec(ctx)
	}
	return err
}
