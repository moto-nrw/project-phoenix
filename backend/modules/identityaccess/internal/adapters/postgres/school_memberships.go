package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) FindActiveSchoolMemberships(ctx context.Context, accountIDs, schoolIDs []int64) (map[int64][]int64, domain.OperationStats, error) {
	result := make(map[int64][]int64)
	if len(accountIDs) == 0 || len(schoolIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		AccountID int64 `bun:"account_id"`
		TenantID  int64 `bun:"tenant_id"`
	}
	started := time.Now()
	err = db.NewRaw(`SELECT account_id, tenant_id FROM auth.account_tenants
		WHERE status = 'active' AND account_id IN (?) AND tenant_id IN (?)
		ORDER BY account_id, tenant_id`, bun.List(accountIDs), bun.List(schoolIDs)).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find active school memberships: %w", err)
	}
	for _, row := range rows {
		result[row.AccountID] = append(result[row.AccountID], row.TenantID)
	}
	return result, stats, nil
}
