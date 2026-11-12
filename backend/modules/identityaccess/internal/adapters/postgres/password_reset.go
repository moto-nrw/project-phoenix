package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// Identity-owned rows behind the password reset flows (#2722). The tables
// carry no tenant; the flows run before authentication. The token column
// holds the hex SHA-256 of the link token only (#4902).

const passwordResetWindowLength = "1 hour"

type passwordResetTokenRow struct {
	bun.BaseModel   `bun:"table:auth.password_reset_tokens,alias:password_reset_token"`
	ID              int64      `bun:"id,pk,autoincrement"`
	AccountID       int64      `bun:"account_id,notnull"`
	TokenHash       string     `bun:"token,notnull"`
	Expiry          time.Time  `bun:"expiry,notnull"`
	Used            bool       `bun:"used,notnull"`
	EmailSentAt     *time.Time `bun:"email_sent_at"`
	EmailError      *string    `bun:"email_error"`
	EmailRetryCount int        `bun:"email_retry_count,notnull"`
	// CredentialGeneration is the account's credential generation the link
	// was issued under; it redeems only while that is current (#6289).
	CredentialGeneration int64     `bun:"credential_generation,notnull"`
	CreatedAt            time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r passwordResetTokenRow) toDomain() domain.PasswordResetToken {
	return domain.PasswordResetToken{
		ID: r.ID, AccountID: r.AccountID, Expiry: r.Expiry, Used: r.Used,
		Delivery:  domain.TokenDelivery{SentAt: r.EmailSentAt, Error: r.EmailError, RetryCount: r.EmailRetryCount},
		CreatedAt: r.CreatedAt, CredentialGeneration: r.CredentialGeneration,
	}
}

type passwordResetWindowRow struct {
	Attempts int       `bun:"attempts"`
	RetryAt  time.Time `bun:"retry_at"`
}

func (s *Store) PasswordResetWindow(ctx context.Context, email string) (domain.PasswordResetWindow, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.PasswordResetWindow{}, false, domain.OperationStats{}, err
	}
	var rows []passwordResetWindowRow
	started := time.Now()
	err = db.NewRaw(`SELECT attempts, window_start + INTERVAL '`+passwordResetWindowLength+`' AS retry_at
		FROM auth.password_reset_rate_limits WHERE email = ?`, email).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.PasswordResetWindow{}, false, stats, fmt.Errorf("identity access postgres: read password reset window: %w", err)
	}
	if len(rows) == 0 {
		return domain.PasswordResetWindow{}, false, stats, nil
	}
	return domain.PasswordResetWindow(rows[0]), true, stats, nil
}

// CountPasswordResetRequest counts the request inside the rolling window in
// one upsert, so concurrent requests cannot collapse into one count.
func (s *Store) CountPasswordResetRequest(ctx context.Context, email string) (domain.PasswordResetWindow, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.PasswordResetWindow{}, domain.OperationStats{}, err
	}
	var row passwordResetWindowRow
	started := time.Now()
	err = db.NewRaw(`INSERT INTO auth.password_reset_rate_limits AS rate_limit (email, attempts, window_start)
		VALUES (?, 1, NOW())
		ON CONFLICT (email) DO UPDATE SET
			attempts = CASE WHEN rate_limit.window_start > NOW() - INTERVAL '`+passwordResetWindowLength+`'
				THEN rate_limit.attempts + 1 ELSE 1 END,
			window_start = CASE WHEN rate_limit.window_start > NOW() - INTERVAL '`+passwordResetWindowLength+`'
				THEN rate_limit.window_start ELSE NOW() END
		RETURNING attempts, window_start + INTERVAL '`+passwordResetWindowLength+`' AS retry_at`, email).Scan(ctx, &row)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.PasswordResetWindow{}, stats, fmt.Errorf("identity access postgres: count password reset request: %w", err)
	}
	stats.Rows = 1
	return domain.PasswordResetWindow(row), stats, nil
}

func (s *Store) DeleteStalePasswordResetWindows(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete stale password reset windows", func(db bun.IDB) executor {
		return db.NewDelete().TableExpr("auth.password_reset_rate_limits").Where("window_start < ?", before)
	})
	return int(rows), stats, err
}

func (s *Store) RevokePasswordResetTokens(ctx context.Context, accountID int64) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "revoke password reset tokens", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*passwordResetTokenRow)(nil)).
			Set("used = TRUE").
			Where(`"password_reset_token".account_id = ?`, accountID)
	})
	return stats, err
}

func (s *Store) InsertPasswordResetToken(ctx context.Context, token domain.PasswordResetToken) (domain.PasswordResetToken, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.PasswordResetToken{}, domain.OperationStats{}, err
	}
	row := passwordResetTokenRow{AccountID: token.AccountID, TokenHash: domain.HashToken(token.Token), Expiry: token.Expiry}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).
		Value("credential_generation", "(SELECT mfa_reset_generation FROM auth.accounts WHERE id = ?)", token.AccountID).
		Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.PasswordResetToken{}, stats, fmt.Errorf("identity access postgres: insert password reset token: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	created := row.toDomain()
	created.Token = token.Token
	return created, stats, nil
}

func (s *Store) FindRedeemablePasswordResetToken(ctx context.Context, token string, now time.Time) (domain.PasswordResetToken, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.PasswordResetToken{}, false, domain.OperationStats{}, err
	}
	var row passwordResetTokenRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"password_reset_token".token = ?`, domain.HashToken(token)).
		Where(`"password_reset_token".expiry > ?`, now).
		Where(`"password_reset_token".used = FALSE`).
		Limit(1).
		Scan(ctx)
	found, stats, err := scanOne("find redeemable password reset token", started, err)
	redeemable := row.toDomain()
	redeemable.Token = token
	return redeemable, found, stats, err
}

func (s *Store) RedeemPasswordResetToken(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "redeem password reset token", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*passwordResetTokenRow)(nil)).
			Set("used = TRUE").
			Where(`"password_reset_token".id = ?`, id).
			Where(`"password_reset_token".used = FALSE`)
	})
	return rows == 1, stats, err
}

func (s *Store) RecordPasswordResetDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "record password reset delivery", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*passwordResetTokenRow)(nil)).
			Set("email_sent_at = ?", delivery.SentAt).
			Set("email_error = ?", delivery.Error).
			Set("email_retry_count = ?", delivery.RetryCount).
			Where(`"password_reset_token".id = ?`, id)
	})
	return stats, err
}

func (s *Store) DeleteSpentPasswordResetTokens(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete spent password reset tokens", func(db bun.IDB) executor {
		return db.NewDelete().Model((*passwordResetTokenRow)(nil)).
			WhereGroup(" AND ", func(q *bun.DeleteQuery) *bun.DeleteQuery {
				return q.Where(`"password_reset_token".expiry < ?`, now).
					WhereOr(`"password_reset_token".used = TRUE`)
			})
	})
	return int(rows), stats, err
}

// MigratePasswordResetWindow preserves active windows from before hashed keys.
// The delete and upsert share the caller's limiter transaction. A mixed rollout
// merges live counts conservatively rather than granting another quota.
func (s *Store) MigratePasswordResetWindow(ctx context.Context, address, key string) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `WITH legacy AS (
 DELETE FROM auth.password_reset_rate_limits WHERE email = ? AND window_start > NOW() - INTERVAL '1 hour'
 RETURNING attempts, window_start
 ) INSERT INTO auth.password_reset_rate_limits AS counter (email, attempts, window_start)
 SELECT ?, attempts, window_start FROM legacy
 ON CONFLICT (email) DO UPDATE SET
 attempts = CASE WHEN counter.window_start > NOW() - INTERVAL '1 hour'
 THEN LEAST(counter.attempts + EXCLUDED.attempts, ?) ELSE EXCLUDED.attempts END,
 window_start = GREATEST(counter.window_start, EXCLUDED.window_start)`, address, key, domain.PasswordResetRateLimitThreshold)
	return err
}
