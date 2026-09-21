package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) FindActiveGuardianMemberships(ctx context.Context, accountIDs []int64) (map[int64][]int64, domain.OperationStats, error) {
	result := make(map[int64][]int64)
	if len(accountIDs) == 0 {
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
	err = db.NewRaw(`SELECT DISTINCT ar.account_id, ar.tenant_id
		FROM auth.account_roles ar
		JOIN auth.roles r ON r.id = ar.role_id
		JOIN auth.accounts a ON a.id = ar.account_id AND a.active = TRUE
		JOIN auth.account_tenants at ON at.account_id = ar.account_id AND at.tenant_id = ar.tenant_id AND at.status = 'active'
		WHERE ar.account_id IN (?) AND LOWER(r.name) = 'guardian'
		ORDER BY ar.account_id, ar.tenant_id`, bun.List(accountIDs)).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find active guardian memberships: %w", err)
	}
	for _, row := range rows {
		result[row.AccountID] = append(result[row.AccountID], row.TenantID)
	}
	return result, stats, nil
}
