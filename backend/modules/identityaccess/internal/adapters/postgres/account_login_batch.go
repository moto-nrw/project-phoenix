package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// LockLoginAccounts takes only the account locks needed before a planned
// exit's school gates. Access snapshots are read later by offboarding.
func (s *Store) LockLoginAccounts(ctx context.Context, ids []int64) ([]int64, domain.OperationStats, error) {
	if len(ids) == 0 {
		return []int64{}, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var locked []int64
	started := time.Now()
	err = db.NewSelect().TableExpr(`auth.accounts AS "account"`).ColumnExpr(`"account".id`).
		Where(`"account".id IN (?)`, bun.List(ids)).
		Where(`"account".personal_data_erased_at IS NULL`).
		OrderExpr(`"account".id ASC`).For("UPDATE OF account").Scan(ctx, &locked)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(locked))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: lock login accounts: %w", err)
	}
	return locked, stats, nil
}
