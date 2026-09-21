package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) ListSchoolAccountRoleNames(ctx context.Context, accountID, tenantID int64) ([]string, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var names []string
	started := time.Now()
	err = db.NewRaw(`SELECT role.name FROM auth.roles AS role
		JOIN auth.account_roles AS assignment ON assignment.role_id = role.id
		WHERE assignment.account_id = ? AND assignment.tenant_id = ?
		ORDER BY assignment.created_at ASC, assignment.id ASC`, accountID, tenantID).Scan(ctx, &names)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(names))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list school account role names: %w", err)
	}
	return names, stats, nil
}

func (s *Store) FindSystemRoleID(ctx context.Context, name string) (int64, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, false, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewRaw(`SELECT id FROM auth.roles WHERE tenant_id IS NULL AND is_system = TRUE AND LOWER(name) = LOWER(?) LIMIT 1`, name).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return 0, false, stats, fmt.Errorf("identity access postgres: find system role: %w", err)
	}
	if len(ids) == 0 {
		return 0, false, stats, nil
	}
	return ids[0], true, stats, nil
}
