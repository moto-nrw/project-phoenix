package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) ClassifySchoolRoles(ctx context.Context, tenantID int64, accountIDs []int64) ([]domain.SchoolRoleClass, domain.OperationStats, error) {
	if len(accountIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		AccountID   int64 `bun:"account_id"`
		IsAdmin     bool  `bun:"is_admin"`
		IsLehrkraft bool  `bun:"is_lehrkraft"`
	}
	started := time.Now()
	err = db.NewRaw(`SELECT ar.account_id,
		COALESCE(bool_or(r.base_role = 'admin' OR (r.is_system AND lower(btrim(r.name)) = 'admin')), false) AS is_admin,
		COALESCE(bool_or(r.is_system AND lower(btrim(r.name)) = 'lehrkraft'), false) AS is_lehrkraft
		FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id
		WHERE ar.tenant_id = ? AND ar.account_id IN (?)
		GROUP BY ar.account_id`,
		tenantID, bun.List(accountIDs)).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("classify school roles: %w", err)
	}
	result := make([]domain.SchoolRoleClass, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.SchoolRoleClass{AccountID: row.AccountID, IsAdmin: row.IsAdmin, IsLehrkraft: row.IsLehrkraft})
	}
	return result, stats, nil
}
