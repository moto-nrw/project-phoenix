package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) ListGuardianSchoolIDs(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewRaw(`SELECT mapping.tenant_id FROM auth.account_tenants AS mapping
		WHERE mapping.account_id = ? AND mapping.status = 'active'
		AND EXISTS (
			SELECT 1 FROM auth.account_roles AS assignment
			INNER JOIN auth.roles AS role ON role.id = assignment.role_id
			WHERE assignment.account_id = mapping.account_id
			AND assignment.tenant_id = mapping.tenant_id
			AND LOWER(role.name) = 'guardian'
		) ORDER BY mapping.created_at ASC`, accountID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list guardian schools: %w", err)
	}
	return ids, stats, nil
}
