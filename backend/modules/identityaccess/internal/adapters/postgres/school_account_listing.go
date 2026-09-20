package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// ListSchoolAccountListings includes inactive mappings and accounts for the
// operator dashboard. Only an active mapping suppresses a pending invitation.
func (s *Store) ListSchoolAccountListings(ctx context.Context, schoolIDs []int64) ([]domain.SchoolAccountListing, domain.OperationStats, error) {
	if len(schoolIDs) == 0 {
		return []domain.SchoolAccountListing{}, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	started := time.Now()
	accounts, err := listSchoolAccountMappings(ctx, db, schoolIDs)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(accounts)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	started = time.Now()
	invitations, err := listSchoolAccountInvitations(ctx, db, schoolIDs)
	stats.Add(domain.OperationStats{Queries: 1, Rows: int64(len(invitations)), StatementDuration: time.Since(started)})
	if err != nil {
		return nil, stats, err
	}
	return append(accounts, invitations...), stats, nil
}

func listSchoolAccountMappings(ctx context.Context, db bun.IDB, schoolIDs []int64) ([]domain.SchoolAccountListing, error) {
	var rows []domain.SchoolAccountListing
	err := db.NewSelect().
		ColumnExpr("at.tenant_id AS school_id, at.account_id, a.email, a.active, at.status").
		ColumnExpr("COALESCE(string_agg(DISTINCT r.name, ', ' ORDER BY r.name), '') AS role_name").
		ColumnExpr("COALESCE(bool_or(LOWER(r.name) = 'admin'), false) AS has_admin_role").
		ColumnExpr("COALESCE(bool_or(LOWER(r.name) = 'user'), false) AS has_user_role").
		TableExpr("auth.account_tenants AS at").
		Join("JOIN auth.accounts AS a ON a.id = at.account_id").
		Join("LEFT JOIN auth.account_roles AS ar ON ar.account_id = at.account_id AND ar.tenant_id = at.tenant_id").
		Join("LEFT JOIN auth.roles AS r ON r.id = ar.role_id").
		Where("at.tenant_id IN (?)", bun.List(schoolIDs)).
		GroupExpr("at.tenant_id, at.account_id, a.email, a.active, at.status").
		OrderExpr("at.tenant_id ASC, at.account_id ASC").
		Scan(ctx, &rows)
	return rows, err
}

func listSchoolAccountInvitations(ctx context.Context, db bun.IDB, schoolIDs []int64) ([]domain.SchoolAccountListing, error) {
	var rows []domain.SchoolAccountListing
	err := db.NewSelect().
		ColumnExpr("inv.tenant_id AS school_id, inv.email").
		ColumnExpr("COALESCE(inv.first_name, '') AS first_name, COALESCE(inv.last_name, '') AS last_name").
		ColumnExpr("COALESCE(r.name, '') AS role_name, 'invited' AS status").
		TableExpr("auth.invitation_tokens AS inv").
		Join("LEFT JOIN auth.roles AS r ON r.id = inv.role_id").
		Where("inv.tenant_id IN (?)", bun.List(schoolIDs)).
		Where("inv.used_at IS NULL").
		Where("inv.expires_at > NOW()").
		Where(`NOT EXISTS (
 SELECT 1 FROM auth.account_tenants AS existing
 JOIN auth.accounts AS ea ON ea.id = existing.account_id
 WHERE existing.tenant_id = inv.tenant_id AND existing.status = 'active'
 AND LOWER(ea.email) = LOWER(inv.email)
 )`).
		Scan(ctx, &rows)
	return rows, err
}
