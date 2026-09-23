package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// RoleStore serves role administration on the same database runtime as the other identity flows.
// Its managed-role view is separate from the smaller role facts used by account access.
type RoleStore struct{ *Store }

func NewRoleStore(store *Store) *RoleStore { return &RoleStore{Store: store} }

type managedRoleRow struct {
	bun.BaseModel `bun:"table:auth.roles,alias:role"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      *int64    `bun:"tenant_id"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
	IsSystem      bool      `bun:"is_system"`
	BaseRole      *string   `bun:"base_role"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func managedRoleRecord(role domain.ManagedRole) managedRoleRow {
	return managedRoleRow{ID: role.ID, TenantID: role.TenantID, Name: role.Name, Description: role.Description,
		IsSystem: role.IsSystem, BaseRole: role.BaseRole, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

func (role managedRoleRow) domain() domain.ManagedRole {
	return domain.ManagedRole{ID: role.ID, TenantID: role.TenantID, Name: role.Name, Description: role.Description,
		IsSystem: role.IsSystem, BaseRole: role.BaseRole, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

func (s *RoleStore) CreateRole(ctx context.Context, role domain.ManagedRole) (domain.ManagedRole, error) {
	if err := role.Validate(); err != nil {
		return domain.ManagedRole{}, err
	}
	if role.TenantID == nil || *role.TenantID == 0 {
		if tenantID := s.scope(ctx).TenantID; tenantID != 0 {
			role.TenantID = &tenantID
		}
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedRole{}, err
	}
	row := managedRoleRecord(role)
	_, err = db.NewInsert().Model(&row).ModelTableExpr("auth.roles").Returning("id, created_at, updated_at").Exec(ctx)
	if err != nil {
		return domain.ManagedRole{}, fmt.Errorf("create role: %w", err)
	}
	return row.domain(), nil
}

func (s *RoleStore) FindRole(ctx context.Context, id int64) (domain.ManagedRole, bool, error) {
	return s.findRole(ctx, id, s.scope(ctx).TenantID, false)
}

func (s *RoleStore) FindRoleIgnoringTenant(ctx context.Context, id int64) (domain.ManagedRole, bool, error) {
	return s.findRole(ctx, id, 0, false)
}

func (s *RoleStore) FindRoleForUpdate(ctx context.Context, id int64) (domain.ManagedRole, bool, error) {
	return s.findRole(ctx, id, s.scope(ctx).TenantID, true)
}

func (s *RoleStore) findRole(ctx context.Context, id, tenantID int64, lock bool) (domain.ManagedRole, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedRole{}, false, err
	}
	row := new(managedRoleRow)
	query := db.NewSelect().Model(row).ModelTableExpr("auth.roles AS role").Where("role.id = ?", id)
	if tenantID > 0 {
		query = query.Where("(role.tenant_id = ? OR role.tenant_id IS NULL)", tenantID)
	}
	if lock {
		query = query.For("UPDATE")
	}
	err = query.Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ManagedRole{}, false, nil
	}
	if err != nil {
		return domain.ManagedRole{}, false, fmt.Errorf("find role: %w", err)
	}
	return row.domain(), true, nil
}

func (s *RoleStore) FindSystemRoleByName(ctx context.Context, name string) (domain.ManagedRole, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedRole{}, false, err
	}
	row := new(managedRoleRow)
	err = db.NewSelect().Model(row).ModelTableExpr("auth.roles AS role").
		Where("role.tenant_id IS NULL AND role.is_system = TRUE AND LOWER(role.name) = LOWER(?)", name).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ManagedRole{}, false, nil
	}
	if err != nil {
		return domain.ManagedRole{}, false, fmt.Errorf("find system role: %w", err)
	}
	return row.domain(), true, nil
}

func (s *RoleStore) UpdateRole(ctx context.Context, role domain.ManagedRole) error {
	if err := role.Validate(); err != nil {
		return err
	}
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	row := managedRoleRecord(role)
	query := db.NewUpdate().Model(&row).ModelTableExpr("auth.roles AS role").Where("role.id = ?", role.ID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("role.tenant_id = ?", tenantID)
	}
	result, err := query.Exec(ctx)
	return roleMutationResult(result, err)
}

func (s *RoleStore) DeleteRole(ctx context.Context, id int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().TableExpr("auth.roles AS role").Where("role.id = ?", id)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("role.tenant_id = ?", tenantID)
	}
	result, err := query.Exec(ctx)
	return roleMutationResult(result, err)
}

func roleMutationResult(result sql.Result, err error) error {
	if err != nil {
		return fmt.Errorf("mutate role: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

func (s *RoleStore) ListRoles(ctx context.Context, filter domain.RoleFilter) ([]domain.ManagedRole, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []managedRoleRow
	query := db.NewSelect().Model(&rows).ModelTableExpr("auth.roles AS role")
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("(role.tenant_id = ? OR role.tenant_id IS NULL)", tenantID)
	}
	if filter.Name != "" {
		query = query.Where("LOWER(role.name) = LOWER(?)", filter.Name)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return managedRoles(rows), nil
}

func managedRoles(rows []managedRoleRow) []domain.ManagedRole {
	if rows == nil {
		return nil
	}
	roles := make([]domain.ManagedRole, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, row.domain())
	}
	return roles
}

func (s *RoleStore) ListAccountRoles(ctx context.Context, accountID int64) ([]domain.ManagedRole, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []managedRoleRow
	query := db.NewSelect().Model(&rows).ModelTableExpr("auth.roles AS role").
		Join("JOIN auth.account_roles ar ON ar.role_id = role.id").Where("ar.account_id = ?", accountID).
		OrderExpr("ar.created_at ASC, ar.id ASC")
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("ar.tenant_id = ?", tenantID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find roles by account ID: %w", err)
	}
	return managedRoles(rows), nil
}

func (s *RoleStore) ListAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	result := make(map[int64]string)
	if len(accountIDs) == 0 {
		return result, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		AccountID int64
		RoleName  string
	}
	query := db.NewSelect().TableExpr("auth.account_roles AS ar").
		ColumnExpr("ar.account_id, role.name AS role_name").
		Join("JOIN auth.roles AS role ON role.id = ar.role_id").
		Where("ar.account_id IN (?)", bun.List(accountIDs)).OrderExpr("ar.created_at ASC, ar.id ASC")
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("ar.tenant_id = ?", tenantID)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("find role names by account IDs: %w", err)
	}
	for _, row := range rows {
		if _, exists := result[row.AccountID]; !exists {
			result[row.AccountID] = row.RoleName
		}
	}
	return result, nil
}

func (s *RoleStore) AccountHoldsRole(ctx context.Context, accountID, roleID int64) (bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewSelect().TableExpr("auth.account_roles AS account_role").
		Where("account_role.account_id = ? AND account_role.role_id = ?", accountID, roleID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("account_role.tenant_id = ?", tenantID)
	}
	return query.Exists(ctx)
}

func (s *RoleStore) CreateAccountRole(ctx context.Context, accountID, roleID, tenantID int64) error {
	if accountID <= 0 {
		return errors.New("account ID is required")
	}
	if roleID <= 0 {
		return errors.New("role ID is required")
	}
	if tenantID == 0 {
		tenantID = s.scope(ctx).TenantID
	}
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", accountID, roleID, tenantID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create account role: %w", err)
	}
	return nil
}

func (s *RoleStore) DeleteAccountRole(ctx context.Context, accountID, roleID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().TableExpr("auth.account_roles AS account_role").
		Where("account_role.account_id = ? AND account_role.role_id = ?", accountID, roleID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("account_role.tenant_id = ?", tenantID)
	}
	_, err = query.Exec(ctx)
	return err
}

func (s *RoleStore) DeleteRoleAssignments(ctx context.Context, roleID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().TableExpr("auth.account_roles AS account_role").Where("account_role.role_id = ?", roleID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("account_role.tenant_id = ?", tenantID)
	}
	_, err = query.Exec(ctx)
	return err
}

func (s *RoleStore) LockAccount(ctx context.Context, accountID int64) (bool, error) {
	_, found, _, err := s.FindLoginAccount(ctx, accountID, true)
	return found, err
}

func (s *RoleStore) HasTenantMembership(ctx context.Context, accountID, tenantID int64, share bool) (bool, error) {
	if share {
		found, _, err := s.LockActiveTenantMappingShared(ctx, accountID, tenantID)
		return found, err
	}
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().TableExpr("auth.account_tenants").
		Where("account_id = ? AND tenant_id = ? AND status = 'active'", accountID, tenantID).Exists(ctx)
}

func (s *RoleStore) ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	emails, _, err := s.Store.ListAccountEmails(ctx, accountIDs)
	return emails, err
}

func (s *RoleStore) ListAccountAvatars(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	result := make(map[int64]string)
	if len(accountIDs) == 0 {
		return result, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID     int64
		Avatar string
	}
	err = db.NewSelect().TableExpr("auth.accounts").Column("id", "avatar").
		Where("id IN (?)", bun.List(accountIDs)).Where("avatar IS NOT NULL AND avatar != ''").Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("find avatars by account IDs: %w", err)
	}
	for _, row := range rows {
		result[row.ID] = row.Avatar
	}
	return result, nil
}
