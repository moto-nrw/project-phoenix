package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) HasActiveAccountTenant(ctx context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	exists, err := db.NewSelect().TableExpr("auth.account_tenants").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("status = ?", "active").Exists(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: find active account mapping: %w", err)
	}
	return exists, stats, nil
}

func (s *Store) ListAccountRoleIDs(ctx context.Context, accountID, tenantID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().TableExpr("auth.account_roles").
		ColumnExpr("role_id").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		OrderExpr("role_id ASC").
		Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account roles: %w", err)
	}
	return ids, stats, nil
}

func (s *Store) ListActiveAccountIDs(ctx context.Context, tenantID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().TableExpr("auth.account_tenants").
		ColumnExpr("account_id").
		Where("tenant_id = ?", tenantID).
		Where("status = ?", "active").
		OrderExpr("account_id ASC").
		Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list active accounts: %w", err)
	}
	return ids, stats, nil
}
