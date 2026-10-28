package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// The Träger-Büro account management (#4650) keeps its invitations in
// auth.traeger_invitations. Like the Träger roles the table carries no
// tenant: only the administrative transaction of the account management
// reaches it. The token column holds the hex SHA-256 of the link token only
// (#4902).

type traegerInvitationRow struct {
	ID             int64      `bun:"id"`
	OrganizationID int64      `bun:"organization_id"`
	Email          string     `bun:"email"`
	Roles          []string   `bun:"roles,array"`
	ExpiresAt      time.Time  `bun:"expires_at"`
	UsedAt         *time.Time `bun:"used_at"`
	RevokedAt      *time.Time `bun:"revoked_at"`
	CreatedBy      *int64     `bun:"created_by"`
	CreatedAt      time.Time  `bun:"created_at"`
}

func (r traegerInvitationRow) toDomain() domain.TraegerInvitation {
	roles := r.Roles
	if roles == nil {
		roles = []string{}
	}
	return domain.TraegerInvitation{
		ID: r.ID, OrganizationID: r.OrganizationID, Email: r.Email, Roles: roles, ExpiresAt: r.ExpiresAt,
		UsedAt: r.UsedAt, RevokedAt: r.RevokedAt, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt,
	}
}

const traegerInvitationColumns = `id, organization_id, email, roles, expires_at, used_at, revoked_at, created_by, created_at`

const insertTraegerInvitationQuery = `INSERT INTO auth.traeger_invitations
		(organization_id, email, token, roles, expires_at, created_by)
	VALUES (?, ?, ?, ?, ?, ?)
	RETURNING ` + traegerInvitationColumns

// InsertTraegerInvitation stores the invitation with the hash of its token.
func (s *Store) InsertTraegerInvitation(ctx context.Context, invitation domain.TraegerInvitation) (domain.TraegerInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.TraegerInvitation{}, domain.OperationStats{}, err
	}
	var rows []traegerInvitationRow
	started := time.Now()
	err = db.NewRaw(insertTraegerInvitationQuery, invitation.OrganizationID, invitation.Email, domain.HashToken(invitation.Token),
		pgdialect.Array(invitation.Roles), invitation.ExpiresAt, invitation.CreatedBy).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.TraegerInvitation{}, stats, fmt.Errorf("identity access postgres: insert Träger invitation: %w", err)
	}
	if len(rows) != 1 {
		return domain.TraegerInvitation{}, stats, fmt.Errorf("identity access postgres: insert Träger invitation returned %d rows", len(rows))
	}
	created := rows[0].toDomain()
	created.Token = invitation.Token
	return created, stats, nil
}

const revokeOpenTraegerInvitationsForEmailQuery = `UPDATE auth.traeger_invitations
	SET revoked_at = ?
	WHERE organization_id = ? AND email = ? AND used_at IS NULL AND revoked_at IS NULL`

// RevokeOpenTraegerInvitationsForEmail takes the address lock of the
// Träger, then spends the open invitations of the address. The lock holds
// until the caller's transaction ends, so two invitations of the same
// address queue instead of both inserting an open one; it also holds when
// the Träger-Büro has nobody whose Verwaltung row could be locked (#5144).
func (s *Store) RevokeOpenTraegerInvitationsForEmail(ctx context.Context, organizationID int64, email string, at time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	lockKey := fmt.Sprintf("traeger_invitation:%d:%s", organizationID, email)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey); err != nil {
		return domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, fmt.Errorf("identity access postgres: lock Träger invitation address: %w", err)
	}
	result, err := db.NewRaw(revokeOpenTraegerInvitationsForEmailQuery, at, organizationID, email).Exec(ctx)
	stats := domain.OperationStats{Queries: 2, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: revoke open Träger invitations: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

const revokeTraegerInvitationQuery = `UPDATE auth.traeger_invitations
	SET revoked_at = ?
	WHERE id = ? AND organization_id = ? AND used_at IS NULL AND revoked_at IS NULL
	RETURNING ` + traegerInvitationColumns

// RevokeTraegerInvitation spends one open invitation of the Träger.
func (s *Store) RevokeTraegerInvitation(ctx context.Context, organizationID, id int64, at time.Time) (domain.TraegerInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.TraegerInvitation{}, false, domain.OperationStats{}, err
	}
	var rows []traegerInvitationRow
	started := time.Now()
	err = db.NewRaw(revokeTraegerInvitationQuery, at, id, organizationID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.TraegerInvitation{}, false, stats, fmt.Errorf("identity access postgres: revoke Träger invitation: %w", err)
	}
	if len(rows) == 0 {
		return domain.TraegerInvitation{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

const listOpenTraegerInvitationsQuery = `SELECT ` + traegerInvitationColumns + `
	FROM auth.traeger_invitations
	WHERE organization_id = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?
	ORDER BY created_at DESC, id DESC`

// ListOpenTraegerInvitations returns the open, unexpired invitations.
func (s *Store) ListOpenTraegerInvitations(ctx context.Context, organizationID int64, now time.Time) ([]domain.TraegerInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []traegerInvitationRow
	started := time.Now()
	err = db.NewRaw(listOpenTraegerInvitationsQuery, organizationID, now).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list open Träger invitations: %w", err)
	}
	invitations := make([]domain.TraegerInvitation, 0, len(rows))
	for _, row := range rows {
		invitations = append(invitations, row.toDomain())
	}
	return invitations, stats, nil
}

const findTraegerInvitationByTokenQuery = `SELECT ` + traegerInvitationColumns + `
	FROM auth.traeger_invitations
	WHERE token = ?`

const lockTraegerInvitationByTokenQuery = findTraegerInvitationByTokenQuery + `
	FOR UPDATE`

// FindTraegerInvitationByToken reads the invitation behind a raw token.
func (s *Store) FindTraegerInvitationByToken(ctx context.Context, token string, forUpdate bool) (domain.TraegerInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.TraegerInvitation{}, false, domain.OperationStats{}, err
	}
	read := db.NewRaw(findTraegerInvitationByTokenQuery, domain.HashToken(token))
	if forUpdate {
		read = db.NewRaw(lockTraegerInvitationByTokenQuery, domain.HashToken(token))
	}
	var rows []traegerInvitationRow
	started := time.Now()
	err = read.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.TraegerInvitation{}, false, stats, fmt.Errorf("identity access postgres: find Träger invitation: %w", err)
	}
	if len(rows) == 0 {
		return domain.TraegerInvitation{}, false, stats, nil
	}
	invitation := rows[0].toDomain()
	invitation.Token = token
	return invitation, true, stats, nil
}

const spendTraegerInvitationQuery = `UPDATE auth.traeger_invitations
	SET used_at = ?
	WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL`

// SpendTraegerInvitation marks an open invitation used.
func (s *Store) SpendTraegerInvitation(ctx context.Context, id int64, at time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(spendTraegerInvitationQuery, at, id).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: spend Träger invitation: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

const deleteOtherTraegerRolesQuery = `DELETE FROM auth.organization_account_roles
	WHERE organization_id = ? AND account_id = ? AND NOT (role = ANY (?))`

const insertTraegerRolesQuery = `INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
	SELECT ?, ?, granted FROM unnest(?::VARCHAR(32)[]) AS granted
	ON CONFLICT ON CONSTRAINT uq_organization_account_roles_role DO NOTHING`

// ReplaceTraegerRoles makes roles the account's exact role set at the Träger.
func (s *Store) ReplaceTraegerRoles(ctx context.Context, organizationID, accountID int64, roles []string) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if roles == nil {
		roles = []string{}
	}
	started := time.Now()
	deleted, err := db.NewRaw(deleteOtherTraegerRolesQuery, organizationID, accountID, pgdialect.Array(roles)).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: remove Träger roles: %w", err)
	}
	removed, _ := deleted.RowsAffected()
	stats.Rows += removed
	if len(roles) == 0 {
		return stats, nil
	}
	inserted, err := db.NewRaw(insertTraegerRolesQuery, organizationID, accountID, pgdialect.Array(roles)).Exec(ctx)
	stats.Queries++
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: grant Träger roles: %w", err)
	}
	added, _ := inserted.RowsAffected()
	stats.Rows += added
	return stats, nil
}

// The account's Träger sessions and the school sessions it opened through
// the Träger (ADR 0084 section 5): both name the Träger in organization_id.
const deleteTraegerAccountSessionsQuery = `DELETE FROM auth.tokens
	WHERE account_id = ? AND organization_id = ?`

// DeleteTraegerAccountSessions ends every session of the account at the
// Träger, the school sessions of its Absprung included (#4654).
func (s *Store) DeleteTraegerAccountSessions(ctx context.Context, organizationID, accountID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(deleteTraegerAccountSessionsQuery, accountID, organizationID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete Träger sessions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

const lockTraegerAdministratorsQuery = `SELECT "role".account_id FROM auth.organization_account_roles AS "role"
	JOIN auth.accounts AS "account" ON "account".id = "role".account_id
	WHERE "role".organization_id = ? AND "role".role = 'administration'
		AND "account".active AND "account".personal_data_erased_at IS NULL
	ORDER BY "role".account_id
	FOR UPDATE OF "role"`

// LockTraegerAdministrators locks the Verwaltung role rows of the Träger
// whose accounts can sign in and returns those accounts. A concurrent
// removal of one of them is skipped once it commits, so the answer is the
// set that remains.
func (s *Store) LockTraegerAdministrators(ctx context.Context, organizationID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	started := time.Now()
	err = db.NewRaw(lockTraegerAdministratorsQuery, organizationID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: lock Träger administrators: %w", err)
	}
	return ids, stats, nil
}

const deleteExpiredTraegerInvitationsQuery = `DELETE FROM auth.traeger_invitations WHERE expires_at < ?`

// DeleteExpiredTraegerInvitations deletes every expired invitation, open or
// spent: the ledger keeps who invited whom. A Träger invitation belongs to
// no school, so a cleanup scoped to one school deletes none; the hourly
// cleanup runs administratively and reaches them.
func (s *Store) DeleteExpiredTraegerInvitations(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	if !s.scope(ctx).AdminTransaction {
		return 0, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(deleteExpiredTraegerInvitationsQuery, now).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: delete expired Träger invitations: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return int(stats.Rows), stats, nil
}
