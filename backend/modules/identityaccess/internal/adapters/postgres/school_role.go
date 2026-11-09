package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type schoolRoleRow struct {
	ID           int64   `bun:"id"`
	TenantID     *int64  `bun:"tenant_id"`
	Name         string  `bun:"name"`
	IsSystem     bool    `bun:"is_system"`
	BaseRole     *string `bun:"base_role"`
	SchoolPortal bool    `bun:"school_portal"`
	NameKey      string  `bun:"name_key"`
}

func (r schoolRoleRow) value() *domain.SchoolRole {
	return &domain.SchoolRole{ID: r.ID, TenantID: r.TenantID, Name: r.Name, IsSystem: r.IsSystem, BaseRole: r.BaseRole, SchoolPortal: r.SchoolPortal, NameKey: r.NameKey}
}

func (s *Store) ListSchoolRoles(ctx context.Context, tenantID int64) ([]*domain.SchoolRole, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []schoolRoleRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, tenant_id, name, is_system, base_role, school_portal, LOWER(name) AS name_key FROM auth.roles
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
	err = db.NewRaw(`SELECT id, tenant_id, name, is_system, base_role, school_portal FROM auth.roles
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

// FindRolePermissionsByRoleIDs reads the permissions of every given role in
// one statement. It joins the same three tables with the same tenant filter
// as the former single-role read, so it waits for the same relation locks as
// one such read did and takes no row locks.
func (s *Store) FindRolePermissionsByRoleIDs(ctx context.Context, roleIDs []int64, tenantID int64) (map[int64][]string, domain.OperationStats, error) {
	permissions := make(map[int64][]string, len(roleIDs))
	if len(roleIDs) == 0 {
		return permissions, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		RoleID int64  `bun:"role_id"`
		Name   string `bun:"name"`
	}
	started := time.Now()
	err = db.NewRaw(`SELECT rp.role_id, p.name FROM auth.permissions p
		JOIN auth.role_permissions rp ON rp.permission_id = p.id
		JOIN auth.roles r ON r.id = rp.role_id
		WHERE rp.role_id = ANY(?) AND (r.tenant_id = ? OR r.tenant_id IS NULL)`, pgdialect.Array(roleIDs), tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: find role permissions: %w", err)
	}
	for _, row := range rows {
		permissions[row.RoleID] = append(permissions[row.RoleID], row.Name)
	}
	return permissions, stats, nil
}
