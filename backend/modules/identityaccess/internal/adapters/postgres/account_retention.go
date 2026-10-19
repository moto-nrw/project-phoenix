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
