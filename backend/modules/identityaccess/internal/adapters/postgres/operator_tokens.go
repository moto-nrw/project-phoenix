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

// operatorInvitationRow mirrors platform.operator_invitation_tokens.
type operatorInvitationRow struct {
	bun.BaseModel   `bun:"table:platform.operator_invitation_tokens,alias:operator_invitation_token"`
	ID              int64      `bun:"id,pk,autoincrement"`
	Email           string     `bun:"email,notnull"`
	Token           string     `bun:"token,notnull"`
	ExpiresAt       time.Time  `bun:"expires_at,notnull"`
	UsedAt          *time.Time `bun:"used_at"`
	CreatedBy       int64      `bun:"created_by,notnull"`
	DisplayName     *string    `bun:"display_name"`
	EmailSentAt     *time.Time `bun:"email_sent_at"`
	EmailError      *string    `bun:"email_error"`
	EmailRetryCount int        `bun:"email_retry_count,notnull"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorInvitationRow) toDomain() domain.OperatorInvitation {
	return domain.OperatorInvitation{
		ID: r.ID, Email: r.Email, Token: r.Token, ExpiresAt: r.ExpiresAt, UsedAt: r.UsedAt, CreatedBy: r.CreatedBy,
		DisplayName: r.DisplayName,
		Delivery:    domain.TokenDelivery{SentAt: r.EmailSentAt, Error: r.EmailError, RetryCount: r.EmailRetryCount},
		CreatedAt:   r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// operatorEmailChangeRow mirrors platform.operator_email_change_tokens.
type operatorEmailChangeRow struct {
	bun.BaseModel   `bun:"table:platform.operator_email_change_tokens,alias:operator_email_change_token"`
	ID              int64      `bun:"id,pk,autoincrement"`
	OperatorID      int64      `bun:"operator_id,notnull"`
	NewEmail        string     `bun:"new_email,notnull"`
	Token           string     `bun:"token,notnull"`
	Expiry          time.Time  `bun:"expiry,notnull"`
	Used            bool       `bun:"used,notnull"`
	EmailSentAt     *time.Time `bun:"email_sent_at"`
	EmailError      *string    `bun:"email_error"`
	EmailRetryCount int        `bun:"email_retry_count,notnull"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorEmailChangeRow) toDomain() domain.OperatorEmailChange {
	return domain.OperatorEmailChange{
		ID: r.ID, OperatorID: r.OperatorID, NewEmail: r.NewEmail, Token: r.Token, Expiry: r.Expiry, Used: r.Used,
		Delivery:  domain.TokenDelivery{SentAt: r.EmailSentAt, Error: r.EmailError, RetryCount: r.EmailRetryCount},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// executor is a built write statement.
type executor interface {
	Exec(ctx context.Context, dest ...any) (sql.Result, error)
}

// exec runs one write statement and records its duration and affected rows.
func (s *Store) exec(ctx context.Context, operation string, build func(bun.IDB) executor) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := build(db).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows, stats, nil
}

// scanOne classifies the outcome of a statement that yields at most one row.
func scanOne(operation string, started time.Time, err error) (bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return false, stats, nil
	}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	return true, stats, nil
}

func (s *Store) InsertOperatorInvitation(ctx context.Context, invitation domain.OperatorInvitation) (domain.OperatorInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorInvitation{}, domain.OperationStats{}, err
	}
	row := operatorInvitationRow{
		Email: invitation.Email, Token: invitation.Token, ExpiresAt: invitation.ExpiresAt, CreatedBy: invitation.CreatedBy,
		DisplayName: invitation.DisplayName,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorInvitation{}, stats, fmt.Errorf("identity access postgres: insert operator invitation: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) FindOperatorInvitation(ctx context.Context, id int64) (domain.OperatorInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorInvitation{}, false, domain.OperationStats{}, err
	}
	var row operatorInvitationRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_invitation_token".id = ?`, id).
		Scan(ctx)
	found, stats, err := scanOne("find operator invitation", started, err)
	return row.toDomain(), found, stats, err
}

func (s *Store) FindRedeemableOperatorInvitation(ctx context.Context, token string, now time.Time) (domain.OperatorInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorInvitation{}, false, domain.OperationStats{}, err
	}
	var row operatorInvitationRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_invitation_token".token = ?`, token).
		Where(`"operator_invitation_token".expires_at > ?`, now).
		Where(`"operator_invitation_token".used_at IS NULL`).
		Scan(ctx)
	found, stats, err := scanOne("find redeemable operator invitation", started, err)
	return row.toDomain(), found, stats, err
}

func (s *Store) ListRedeemableOperatorInvitations(ctx context.Context, now time.Time) ([]domain.OperatorInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []operatorInvitationRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where(`"operator_invitation_token".used_at IS NULL`).
		Where(`"operator_invitation_token".expires_at > ?`, now).
		OrderExpr(`"operator_invitation_token".created_at DESC`).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list redeemable operator invitations: %w", err)
	}
	result := make([]domain.OperatorInvitation, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *Store) CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().Model((*operatorInvitationRow)(nil)).
		Where(`"operator_invitation_token".created_by = ?`, createdBy).
		Where(`"operator_invitation_token".created_at > ?`, since).
		Count(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: count operator invitations: %w", err)
	}
	return count, stats, nil
}

func (s *Store) RedeemOperatorInvitation(ctx context.Context, token string, now time.Time) (domain.OperatorInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorInvitation{}, false, domain.OperationStats{}, err
	}
	var row operatorInvitationRow
	started := time.Now()
	err = db.NewUpdate().Model(&row).
		Set("used_at = ?", now).
		Where(`"operator_invitation_token".token = ?`, token).
		Where(`"operator_invitation_token".expires_at > ?`, now).
		Where(`"operator_invitation_token".used_at IS NULL`).
		Returning("*").
		Scan(ctx)
	found, stats, err := scanOne("redeem operator invitation", started, err)
	if found {
		stats.Rows = 1
	}
	return row.toDomain(), found, stats, err
}

func (s *Store) RevokeOperatorInvitation(ctx context.Context, id int64, now time.Time) (bool, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "revoke operator invitation", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorInvitationRow)(nil)).
			Set("used_at = ?", now).
			Where(`"operator_invitation_token".id = ?`, id).
			Where(`"operator_invitation_token".used_at IS NULL`)
	})
	return rows == 1, stats, err
}

func (s *Store) RevokeOperatorInvitationsForEmail(ctx context.Context, email string, now time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "revoke operator invitations for email", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorInvitationRow)(nil)).
			Set("used_at = ?", now).
			Where(`"operator_invitation_token".email = ?`, email).
			Where(`"operator_invitation_token".used_at IS NULL`)
	})
	return int(rows), stats, err
}

func (s *Store) ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt, now time.Time) (bool, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "extend operator invitation", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorInvitationRow)(nil)).
			Set("expires_at = ?", expiresAt).
			Where(`"operator_invitation_token".id = ?`, id).
			Where(`"operator_invitation_token".used_at IS NULL`).
			Where(`"operator_invitation_token".expires_at > ?`, now)
	})
	return rows == 1, stats, err
}

func (s *Store) RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "record operator invitation delivery", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorInvitationRow)(nil)).
			Set("email_sent_at = ?", delivery.SentAt).
			Set("email_error = ?", delivery.Error).
			Set("email_retry_count = ?", delivery.RetryCount).
			Where(`"operator_invitation_token".id = ?`, id)
	})
	return stats, err
}

func (s *Store) DeleteExpiredOperatorInvitations(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired operator invitations", func(db bun.IDB) executor {
		return db.NewDelete().Model((*operatorInvitationRow)(nil)).
			Where(`"operator_invitation_token".expires_at < ?`, now)
	})
	return int(rows), stats, err
}

func (s *Store) InsertOperatorEmailChange(ctx context.Context, change domain.OperatorEmailChange) (domain.OperatorEmailChange, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorEmailChange{}, domain.OperationStats{}, err
	}
	row := operatorEmailChangeRow{
		OperatorID: change.OperatorID, NewEmail: change.NewEmail, Token: change.Token, Expiry: change.Expiry, Used: change.Used,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		// idx_email_change_tokens_one_active_per_operator is the second half
		// of the rate limit: it catches two requests that both passed the
		// counted one. The SQLSTATE stays in the adapter that produced it.
		if isUniqueViolation(err) {
			return domain.OperatorEmailChange{}, stats, domain.ErrOperatorEmailChangeActive
		}
		return domain.OperatorEmailChange{}, stats, fmt.Errorf("identity access postgres: insert operator email change: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().Model((*operatorEmailChangeRow)(nil)).
		Where(`"operator_email_change_token".operator_id = ?`, operatorID).
		Where(`"operator_email_change_token".created_at > ?`, since).
		Count(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: count operator email changes: %w", err)
	}
	return count, stats, nil
}

func (s *Store) RedeemOperatorEmailChange(ctx context.Context, token string, now time.Time) (domain.OperatorEmailChange, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorEmailChange{}, false, domain.OperationStats{}, err
	}
	var row operatorEmailChangeRow
	started := time.Now()
	err = db.NewUpdate().Model(&row).
		Set("used = TRUE").
		Where(`"operator_email_change_token".token = ?`, token).
		Where(`"operator_email_change_token".expiry > ?`, now).
		Where(`"operator_email_change_token".used = FALSE`).
		Returning("*").
		Scan(ctx)
	found, stats, err := scanOne("redeem operator email change", started, err)
	if found {
		stats.Rows = 1
	}
	return row.toDomain(), found, stats, err
}

func (s *Store) RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "revoke operator email changes", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorEmailChangeRow)(nil)).
			Set("used = TRUE").
			Where(`"operator_email_change_token".operator_id = ?`, operatorID).
			Where(`"operator_email_change_token".used = FALSE`)
	})
	return stats, err
}

func (s *Store) RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "record operator email change delivery", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorEmailChangeRow)(nil)).
			Set("email_sent_at = ?", delivery.SentAt).
			Set("email_error = ?", delivery.Error).
			Set("email_retry_count = ?", delivery.RetryCount).
			Where(`"operator_email_change_token".id = ?`, id)
	})
	return stats, err
}

func (s *Store) RevokeExpiredOperatorEmailChanges(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "revoke expired operator email changes", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*operatorEmailChangeRow)(nil)).
			Set("used = TRUE").
			Where(`"operator_email_change_token".expiry < ?`, now).
			Where(`"operator_email_change_token".used = FALSE`)
	})
	return int(rows), stats, err
}

func (s *Store) DeleteStaleOperatorEmailChanges(ctx context.Context, createdBefore, now time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete stale operator email changes", func(db bun.IDB) executor {
		return db.NewDelete().Model((*operatorEmailChangeRow)(nil)).
			Where(`"operator_email_change_token".created_at < ?`, createdBefore).
			WhereGroup(" AND ", func(q *bun.DeleteQuery) *bun.DeleteQuery {
				return q.Where(`"operator_email_change_token".expiry < ?`, now).
					WhereOr(`"operator_email_change_token".used = TRUE`)
			})
	})
	return int(rows), stats, err
}
