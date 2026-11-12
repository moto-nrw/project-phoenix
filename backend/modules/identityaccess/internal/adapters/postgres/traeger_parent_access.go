package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The parents-portal access of an account at a Träger (ADR 0216) lives in
// auth.traeger_parent_access and carries no tenant: only the administrative
// transactions of the account setup, the parent sign-in and its refresh and
// validation reach it.

const listTraegerParentAccessQuery = `SELECT organization_id FROM auth.traeger_parent_access
	WHERE account_id = ?
	ORDER BY created_at, organization_id`

// ListTraegerParentAccess returns the Träger the account holds parents-portal
// access at, oldest access first.
func (s *Store) ListTraegerParentAccess(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	organizationIDs := []int64{}
	started := time.Now()
	err = db.NewRaw(listTraegerParentAccessQuery, accountID).Scan(ctx, &organizationIDs)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(organizationIDs))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list Träger parent access: %w", err)
	}
	return organizationIDs, stats, nil
}

// HasTraegerParentAccess reports the account's access at the Träger;
// forShare pins the row until the caller's transaction ends, so a removal
// cannot commit under a mint or a refresh.
func (s *Store) HasTraegerParentAccess(ctx context.Context, accountID, organizationID int64, forShare bool) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewSelect().
		ColumnExpr("1").
		TableExpr("auth.traeger_parent_access").
		Where("account_id = ?", accountID).
		Where("organization_id = ?", organizationID)
	if forShare {
		query = query.For("SHARE")
	}
	var ones []int
	started := time.Now()
	err = query.Scan(ctx, &ones)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ones))}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: find Träger parent access: %w", err)
	}
	return len(ones) > 0, stats, nil
}

const grantTraegerParentAccessQuery = `INSERT INTO auth.traeger_parent_access (account_id, organization_id)
	VALUES (?, ?)
	ON CONFLICT (account_id, organization_id) DO NOTHING`

// GrantTraegerParentAccess records the access once; a held access stays
// unchanged.
func (s *Store) GrantTraegerParentAccess(ctx context.Context, accountID, organizationID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(grantTraegerParentAccessQuery, accountID, organizationID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: grant Träger parent access: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}
