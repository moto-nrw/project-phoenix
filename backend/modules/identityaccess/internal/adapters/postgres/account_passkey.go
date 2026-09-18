package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// accountPasskeyRow mirrors auth.passkey_credentials.
type accountPasskeyRow struct {
	bun.BaseModel  `bun:"table:auth.passkey_credentials,alias:passkey_credential"`
	ID             int64           `bun:"id,pk,autoincrement"`
	AccountID      int64           `bun:"account_id,notnull"`
	UserHandle     []byte          `bun:"user_handle,notnull"`
	CredentialID   []byte          `bun:"credential_id,notnull"`
	CredentialJSON json.RawMessage `bun:"credential_json,type:jsonb,notnull"`
	Name           string          `bun:"name,notnull"`
	LastUsedAt     *time.Time      `bun:"last_used_at"`
	RevokedAt      *time.Time      `bun:"revoked_at"`
	CreatedAt      time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r accountPasskeyRow) toDomain() domain.AccountPasskeyCredential {
	return domain.AccountPasskeyCredential{
		ID: r.ID, AccountID: r.AccountID, UserHandle: r.UserHandle, CredentialID: r.CredentialID, CredentialJSON: r.CredentialJSON,
		Name: r.Name, LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// accountPasskeySessionRow mirrors auth.passkey_sessions.
type accountPasskeySessionRow struct {
	bun.BaseModel  `bun:"table:auth.passkey_sessions,alias:passkey_session"`
	ID             string          `bun:"id,pk"`
	AccountID      *int64          `bun:"account_id"`
	TenantID       *int64          `bun:"tenant_id"`
	Purpose        string          `bun:"purpose,notnull"`
	RPID           string          `bun:"rp_id,notnull"`
	ExpectedOrigin string          `bun:"expected_origin,notnull"`
	SessionJSON    json.RawMessage `bun:"session_json,type:jsonb,notnull"`
	ExpiresAt      time.Time       `bun:"expires_at,notnull"`
	ConsumedAt     *time.Time      `bun:"consumed_at"`
	CreatedAt      time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r accountPasskeySessionRow) toDomain() domain.AccountPasskeySession {
	return domain.AccountPasskeySession{
		ID: r.ID, AccountID: r.AccountID, TenantID: r.TenantID, Purpose: r.Purpose, RPID: r.RPID, ExpectedOrigin: r.ExpectedOrigin,
		SessionJSON: r.SessionJSON, ExpiresAt: r.ExpiresAt, ConsumedAt: r.ConsumedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Store) InsertAccountPasskey(ctx context.Context, credential domain.AccountPasskeyCredential) (domain.AccountPasskeyCredential, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountPasskeyCredential{}, domain.OperationStats{}, err
	}
	row := accountPasskeyRow{
		AccountID: credential.AccountID, UserHandle: credential.UserHandle, CredentialID: credential.CredentialID,
		CredentialJSON: credential.CredentialJSON, Name: credential.Name, LastUsedAt: credential.LastUsedAt, RevokedAt: credential.RevokedAt,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.AccountPasskeyCredential{}, stats, fmt.Errorf("identity access postgres: insert account passkey: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) ListActiveAccountPasskeys(ctx context.Context, accountID int64) ([]domain.AccountPasskeyCredential, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountPasskeyRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where(`"passkey_credential".account_id = ?`, accountID).
		Where(`"passkey_credential".revoked_at IS NULL`).
		OrderExpr(`"passkey_credential".created_at ASC`).
		OrderExpr(`"passkey_credential".id ASC`).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list active account passkeys: %w", err)
	}
	result := make([]domain.AccountPasskeyCredential, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *Store) FindActiveAccountPasskey(ctx context.Context, credentialID, userHandle []byte) (domain.AccountPasskeyCredential, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountPasskeyCredential{}, false, domain.OperationStats{}, err
	}
	var row accountPasskeyRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"passkey_credential".credential_id = ?`, credentialID).
		Where(`"passkey_credential".user_handle = ?`, userHandle).
		Where(`"passkey_credential".revoked_at IS NULL`).
		Limit(1).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountPasskeyCredential{}, false, stats, nil
	}
	if err != nil {
		return domain.AccountPasskeyCredential{}, false, stats, fmt.Errorf("identity access postgres: find active account passkey: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) UpdateAccountPasskeyAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*accountPasskeyRow)(nil)).
		Set("credential_json = ?", json.RawMessage(credentialJSON)).
		Set("last_used_at = ?", usedAt).
		Where(`"passkey_credential".id = ?`, id).
		Where(`"passkey_credential".revoked_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: update account passkey after use: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) RevokeAccountPasskey(ctx context.Context, accountID, id int64, revokedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*accountPasskeyRow)(nil)).
		Set("revoked_at = ?", revokedAt).
		Where(`"passkey_credential".id = ?`, id).
		Where(`"passkey_credential".account_id = ?`, accountID).
		Where(`"passkey_credential".revoked_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: revoke account passkey: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) InsertAccountPasskeySession(ctx context.Context, session domain.AccountPasskeySession) (domain.AccountPasskeySession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountPasskeySession{}, domain.OperationStats{}, err
	}
	row := accountPasskeySessionRow{
		ID: session.ID, AccountID: session.AccountID, TenantID: session.TenantID, Purpose: session.Purpose, RPID: session.RPID,
		ExpectedOrigin: session.ExpectedOrigin, SessionJSON: session.SessionJSON, ExpiresAt: session.ExpiresAt, ConsumedAt: session.ConsumedAt,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.AccountPasskeySession{}, stats, fmt.Errorf("identity access postgres: insert account passkey session: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) ConsumeAccountPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (domain.AccountPasskeySession, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountPasskeySession{}, false, domain.OperationStats{}, err
	}
	var rows []accountPasskeySessionRow
	started := time.Now()
	err = db.NewUpdate().Model((*accountPasskeySessionRow)(nil)).
		Set("consumed_at = ?", consumedAt).
		Where(`"passkey_session".id = ?`, id).
		Where(`"passkey_session".purpose = ?`, purpose).
		Where(`"passkey_session".consumed_at IS NULL`).
		Where(`"passkey_session".expires_at > ?`, consumedAt).
		Returning("*").
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.AccountPasskeySession{}, false, stats, fmt.Errorf("identity access postgres: consume account passkey session: %w", err)
	}
	if len(rows) == 0 {
		return domain.AccountPasskeySession{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}
