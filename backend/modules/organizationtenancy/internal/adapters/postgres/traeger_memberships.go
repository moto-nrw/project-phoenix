package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// Träger memberships (ADR 0084, #4649). platform.organization_accounts has no
// tenant: only the administrative transaction of the Träger sign-in and the
// operator reaches it.

type traegerMembershipRow struct {
	OrganizationID int64      `bun:"organization_id"`
	AccountID      int64      `bun:"account_id"`
	Status         string     `bun:"status"`
	ActivatedAt    *time.Time `bun:"activated_at"`
	CreatedAt      time.Time  `bun:"created_at"`
}

func (r traegerMembershipRow) toDomain() organizationtenancy.TraegerMembership {
	return organizationtenancy.TraegerMembership(r)
}

const listActiveAccountTraegerIDsQuery = `SELECT organization_id FROM platform.organization_accounts
	WHERE account_id = ? AND status = 'active'
	ORDER BY organization_id`

// ListActiveAccountTraegerIDs returns the Träger where the account's
// membership is active, whatever the state of the Träger itself.
func (s *Store) ListActiveAccountTraegerIDs(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	started := time.Now()
	err = db.NewRaw(listActiveAccountTraegerIDsQuery, accountID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("organization tenancy postgres: list Träger of account: %w", err)
	}
	return ids, stats, nil
}

const listTraegerSchoolIDsQuery = `SELECT id FROM platform.schools
	WHERE organization_id = ? AND deleted_at IS NULL
	ORDER BY id`

// ListTraegerSchoolIDs returns the undeleted schools of the Träger.
func (s *Store) ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	started := time.Now()
	err = db.NewRaw(listTraegerSchoolIDsQuery, organizationID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("organization tenancy postgres: list Träger schools: %w", err)
	}
	return ids, stats, nil
}

const listTraegerMembershipsQuery = `SELECT organization_id, account_id, status, activated_at, created_at
	FROM platform.organization_accounts
	WHERE organization_id = ?
	ORDER BY created_at, id`

// ListTraegerMemberships returns every membership of the Träger.
func (s *Store) ListTraegerMemberships(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerMembership, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []traegerMembershipRow
	started := time.Now()
	err = db.NewRaw(listTraegerMembershipsQuery, organizationID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("organization tenancy postgres: list Träger memberships: %w", err)
	}
	memberships := make([]organizationtenancy.TraegerMembership, 0, len(rows))
	for _, row := range rows {
		memberships = append(memberships, row.toDomain())
	}
	return memberships, stats, nil
}

const upsertActiveTraegerMembershipQuery = `INSERT INTO platform.organization_accounts
		(organization_id, account_id, status, activated_at)
	VALUES (?, ?, 'active', CURRENT_TIMESTAMP)
	ON CONFLICT ON CONSTRAINT uq_organization_accounts_member DO UPDATE
		SET status = 'active',
			activated_at = CASE WHEN platform.organization_accounts.status = 'active'
				THEN platform.organization_accounts.activated_at ELSE CURRENT_TIMESTAMP END,
			deactivated_at = NULL
	RETURNING organization_id, account_id, status, activated_at, created_at`

// UpsertActiveTraegerMembership creates the membership active or reactivates
// it; an active membership keeps its activation time.
func (s *Store) UpsertActiveTraegerMembership(ctx context.Context, organizationID, accountID int64) (organizationtenancy.TraegerMembership, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return organizationtenancy.TraegerMembership{}, domain.OperationStats{}, err
	}
	var row traegerMembershipRow
	started := time.Now()
	err = db.NewRaw(upsertActiveTraegerMembershipQuery, organizationID, accountID).Scan(ctx, &row)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: 1}
	if err != nil {
		return organizationtenancy.TraegerMembership{}, stats, fmt.Errorf("organization tenancy postgres: activate Träger membership: %w", err)
	}
	return row.toDomain(), stats, nil
}
