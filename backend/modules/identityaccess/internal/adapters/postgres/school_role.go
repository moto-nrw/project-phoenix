package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type schoolRoleRow struct {
	ID       int64   `bun:"id"`
	TenantID *int64  `bun:"tenant_id"`
	Name     string  `bun:"name"`
	IsSystem bool    `bun:"is_system"`
	BaseRole *string `bun:"base_role"`
}

func (r schoolRoleRow) value() *domain.SchoolRole {
	return &domain.SchoolRole{ID: r.ID, TenantID: r.TenantID, Name: r.Name, IsSystem: r.IsSystem, BaseRole: r.BaseRole}
}

func (s *Store) ListSchoolRoles(ctx context.Context, tenantID int64) ([]*domain.SchoolRole, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []schoolRoleRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, tenant_id, name, is_system, base_role FROM auth.roles
		WHERE tenant_id = ? OR tenant_id IS NULL`, tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list school roles: %w", err)
	}
	roles := make([]*domain.SchoolRole, len(rows))
	for i, row := range rows {
		roles[i] = row.value()
	}
	return roles, stats, nil
}

func (s *Store) FindSchoolRoleByName(ctx context.Context, name string, tenantID int64) (*domain.SchoolRole, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []schoolRoleRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, tenant_id, name, is_system, base_role FROM auth.roles
		WHERE LOWER(name) = ? AND (tenant_id = ? OR tenant_id IS NULL)
		ORDER BY tenant_id IS NULL ASC LIMIT 1`, name, tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: find school role: %w", err)
	}
	if len(rows) == 0 {
		return nil, stats, nil
	}
	return rows[0].value(), stats, nil
}

func (s *Store) FindRolePermissions(ctx context.Context, roleID, tenantID int64) ([]string, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var names []string
	started := time.Now()
	err = db.NewRaw(`SELECT p.name FROM auth.permissions p
		JOIN auth.role_permissions rp ON rp.permission_id = p.id
		JOIN auth.roles r ON r.id = rp.role_id
		WHERE rp.role_id = ? AND (r.tenant_id = ? OR r.tenant_id IS NULL)`, roleID, tenantID).Scan(ctx, &names)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: find role permissions: %w", err)
	}
	return names, stats, nil
}
