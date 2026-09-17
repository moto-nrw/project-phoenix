package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// auth.invitation_tokens as the school invitation flows (#2722) read and
// write it. The table is tenant-scoped: the statements apply the tenant the
// runtime scoped the caller to, so an administrative transaction — the
// public accept and validate routes run in one — reaches the invitation of
// every school while a tenant-scoped caller stays inside its own.

type schoolInvitationRow struct {
	bun.BaseModel    `bun:"table:auth.invitation_tokens,alias:invitation_token"`
	ID               int64      `bun:"id,pk,autoincrement"`
	TenantID         int64      `bun:"tenant_id,notnull"`
	Email            string     `bun:"email,notnull"`
	Token            string     `bun:"token,notnull"`
	RoleID           int64      `bun:"role_id,notnull"`
	ExpiresAt        time.Time  `bun:"expires_at,notnull"`
	UsedAt           *time.Time `bun:"used_at"`
	CreatedBy        *int64     `bun:"created_by"`
	FirstName        *string    `bun:"first_name"`
	LastName         *string    `bun:"last_name"`
	Position         *string    `bun:"position"`
	CaregiverEnabled bool       `bun:"caregiver_enabled,notnull"`
	PersonID         *int64     `bun:"person_id"`
	EmailSentAt      *time.Time `bun:"email_sent_at"`
	EmailError       *string    `bun:"email_error"`
	EmailRetryCount  int        `bun:"email_retry_count,notnull"`
	CreatedAt        time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r schoolInvitationRow) toDomain() domain.SchoolInvitation {
	return domain.SchoolInvitation{
		ID: r.ID, TenantID: r.TenantID, Email: r.Email, Token: r.Token, RoleID: r.RoleID, ExpiresAt: r.ExpiresAt,
		UsedAt: r.UsedAt, CreatedBy: r.CreatedBy, FirstName: r.FirstName, LastName: r.LastName, Position: r.Position,
		CaregiverEnabled: r.CaregiverEnabled, PersonID: r.PersonID,
		Delivery:  domain.TokenDelivery{SentAt: r.EmailSentAt, Error: r.EmailError, RetryCount: r.EmailRetryCount},
		CreatedAt: r.CreatedAt,
	}
}

func invitationTenantFilter[Q tenantFilterable[Q]](scope TenantScope, query Q) Q {
	if scope.TenantID > 0 {
		return query.Where(`"invitation_token".tenant_id = ?`, scope.TenantID)
	}
	return query
}

func (s *Store) InsertSchoolInvitation(ctx context.Context, invitation domain.SchoolInvitation) (domain.SchoolInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.SchoolInvitation{}, domain.OperationStats{}, err
	}
	row := schoolInvitationRow{
		TenantID: invitation.TenantID, Email: invitation.Email, Token: invitation.Token, RoleID: invitation.RoleID,
		ExpiresAt: invitation.ExpiresAt, CreatedBy: invitation.CreatedBy, FirstName: invitation.FirstName,
		LastName: invitation.LastName, Position: invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled,
		PersonID: invitation.PersonID,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.SchoolInvitation{}, stats, fmt.Errorf("identity access postgres: insert school invitation: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) FindSchoolInvitation(ctx context.Context, id int64) (domain.SchoolInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.SchoolInvitation{}, false, domain.OperationStats{}, err
	}
	var row schoolInvitationRow
	started := time.Now()
	err = invitationTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"invitation_token".id = ?`, id)).Scan(ctx)
	found, stats, err := scanOne("find school invitation", started, err)
	return row.toDomain(), found, stats, err
}

func (s *Store) FindSchoolInvitationByToken(ctx context.Context, token string) (domain.SchoolInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.SchoolInvitation{}, false, domain.OperationStats{}, err
	}
	var row schoolInvitationRow
	started := time.Now()
	err = invitationTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"invitation_token".token = ?`, token)).Scan(ctx)
	found, stats, err := scanOne("find school invitation by token", started, err)
	return row.toDomain(), found, stats, err
}

func (s *Store) ListRedeemableSchoolInvitations(ctx context.Context, now time.Time) ([]domain.SchoolInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []schoolInvitationRow
	started := time.Now()
	query := invitationTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"invitation_token".used_at IS NULL`).
		Where(`"invitation_token".expires_at > ?`, now)).
		OrderExpr(`"invitation_token".created_at DESC`)
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list redeemable school invitations: %w", err)
	}
	result := make([]domain.SchoolInvitation, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *Store) RedeemSchoolInvitation(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	scope := s.scope(ctx)
	rows, stats, err := s.exec(ctx, "redeem school invitation", func(db bun.IDB) executor {
		return invitationTenantFilter(scope, db.NewUpdate().Model((*schoolInvitationRow)(nil)).
			Set("used_at = NOW()").
			Where(`"invitation_token".id = ?`, id).
			Where(`"invitation_token".used_at IS NULL`))
	})
	return rows == 1, stats, err
}

func (s *Store) RevokeSchoolInvitationsForEmail(ctx context.Context, email string) (int, domain.OperationStats, error) {
	scope := s.scope(ctx)
	rows, stats, err := s.exec(ctx, "revoke school invitations for email", func(db bun.IDB) executor {
		return invitationTenantFilter(scope, db.NewUpdate().Model((*schoolInvitationRow)(nil)).
			Set("used_at = NOW()").
			Where(`LOWER("invitation_token".email) = LOWER(?)`, email).
			Where(`"invitation_token".used_at IS NULL`))
	})
	return int(rows), stats, err
}

func (s *Store) RevokeSchoolInvitationsForTenant(ctx context.Context, tenantID int64) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "revoke school invitations for tenant", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*schoolInvitationRow)(nil)).
			Set("used_at = NOW()").
			Where(`"invitation_token".tenant_id = ?`, tenantID).
			Where(`"invitation_token".used_at IS NULL`)
	})
	return int(rows), stats, err
}

func (s *Store) ExtendSchoolInvitation(ctx context.Context, id int64, expiresAt, now time.Time) (bool, domain.OperationStats, error) {
	scope := s.scope(ctx)
	rows, stats, err := s.exec(ctx, "extend school invitation", func(db bun.IDB) executor {
		return invitationTenantFilter(scope, db.NewUpdate().Model((*schoolInvitationRow)(nil)).
			Set("expires_at = ?", expiresAt).
			Where(`"invitation_token".id = ?`, id).
			Where(`"invitation_token".used_at IS NULL`).
			Where(`"invitation_token".expires_at > ?`, now))
	})
	return rows == 1, stats, err
}

func (s *Store) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	scope := s.scope(ctx)
	_, stats, err := s.exec(ctx, "record school invitation delivery", func(db bun.IDB) executor {
		return invitationTenantFilter(scope, db.NewUpdate().Model((*schoolInvitationRow)(nil)).
			Set("email_sent_at = ?", delivery.SentAt).
			Set("email_error = ?", delivery.Error).
			Set("email_retry_count = ?", delivery.RetryCount).
			Where(`"invitation_token".id = ?`, id))
	})
	return stats, err
}

func (s *Store) DeleteExpiredSchoolInvitations(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired school invitations", func(db bun.IDB) executor {
		return db.NewDelete().Model((*schoolInvitationRow)(nil)).
			Where(`"invitation_token".expires_at < ?`, now)
	})
	return int(rows), stats, err
}

func (s *Store) InsertAccount(ctx context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.LoginAccount{}, domain.OperationStats{}, err
	}
	var rows []loginAccountRow
	started := time.Now()
	err = db.NewRaw(`INSERT INTO auth.accounts (email, password_hash, active)
		VALUES (?, NULLIF(?, ''), TRUE)
		RETURNING id, email, username, password_hash, active, updated_at`, email, passwordHash).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.LoginAccount{}, stats, fmt.Errorf("identity access postgres: insert account: %w", err)
	}
	if len(rows) == 0 {
		return domain.LoginAccount{}, stats, fmt.Errorf("identity access postgres: insert account: no row returned")
	}
	return rows[0].toDomain(), stats, nil
}

// EnsureAccountTenant activates the mapping and reactivates a deactivated
// one, so accepting a re-invitation after offboarding restores access.
func (s *Store) EnsureAccountTenant(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "ensure account tenant", func(db bun.IDB) executor {
		return db.NewRaw(`INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, deactivated_at, created_at, updated_at)
			VALUES (?, ?, 'active', NOW(), NULL, NOW(), NOW())
			ON CONFLICT (account_id, tenant_id) DO UPDATE
			SET status = 'active', activated_at = NOW(), deactivated_at = NULL, updated_at = NOW()`,
			accountID, tenantID)
	})
	return stats, err
}
