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

// operatorPasskeyRow mirrors platform.operator_passkey_credentials.
type operatorPasskeyRow struct {
	bun.BaseModel  `bun:"table:platform.operator_passkey_credentials,alias:operator_passkey_credential"`
	ID             int64           `bun:"id,pk,autoincrement"`
	OperatorID     int64           `bun:"operator_id,notnull"`
	UserHandle     []byte          `bun:"user_handle,notnull"`
	CredentialID   []byte          `bun:"credential_id,notnull"`
	CredentialJSON json.RawMessage `bun:"credential_json,type:jsonb,notnull"`
	Name           string          `bun:"name,notnull"`
	LastUsedAt     *time.Time      `bun:"last_used_at"`
	RevokedAt      *time.Time      `bun:"revoked_at"`
	CreatedAt      time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorPasskeyRow) toDomain() domain.OperatorPasskeyCredential {
	return domain.OperatorPasskeyCredential{
		ID: r.ID, OperatorID: r.OperatorID, UserHandle: r.UserHandle, CredentialID: r.CredentialID, CredentialJSON: r.CredentialJSON,
		Name: r.Name, LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// operatorPasskeySessionRow mirrors platform.operator_passkey_sessions.
type operatorPasskeySessionRow struct {
	bun.BaseModel  `bun:"table:platform.operator_passkey_sessions,alias:operator_passkey_session"`
	ID             string          `bun:"id,pk"`
	OperatorID     *int64          `bun:"operator_id"`
	Purpose        string          `bun:"purpose,notnull"`
	RPID           string          `bun:"rp_id,notnull"`
	ExpectedOrigin string          `bun:"expected_origin,notnull"`
	SessionJSON    json.RawMessage `bun:"session_json,type:jsonb,notnull"`
	ExpiresAt      time.Time       `bun:"expires_at,notnull"`
	ConsumedAt     *time.Time      `bun:"consumed_at"`
	CreatedAt      time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorPasskeySessionRow) toDomain() domain.OperatorPasskeySession {
	return domain.OperatorPasskeySession{
		ID: r.ID, OperatorID: r.OperatorID, Purpose: r.Purpose, RPID: r.RPID, ExpectedOrigin: r.ExpectedOrigin,
		SessionJSON: r.SessionJSON, ExpiresAt: r.ExpiresAt, ConsumedAt: r.ConsumedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Store) InsertOperatorPasskey(ctx context.Context, credential domain.OperatorPasskeyCredential) (domain.OperatorPasskeyCredential, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, domain.OperationStats{}, err
	}
	row := operatorPasskeyRow{
		OperatorID: credential.OperatorID, UserHandle: credential.UserHandle, CredentialID: credential.CredentialID,
		CredentialJSON: credential.CredentialJSON, Name: credential.Name, LastUsedAt: credential.LastUsedAt, RevokedAt: credential.RevokedAt,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorPasskeyCredential{}, stats, fmt.Errorf("identity access postgres: insert operator passkey: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) ListActiveOperatorPasskeys(ctx context.Context, operatorID int64) ([]domain.OperatorPasskeyCredential, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []operatorPasskeyRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where(`"operator_passkey_credential".operator_id = ?`, operatorID).
		Where(`"operator_passkey_credential".revoked_at IS NULL`).
		OrderExpr(`"operator_passkey_credential".created_at ASC`).
		OrderExpr(`"operator_passkey_credential".id ASC`).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list active operator passkeys: %w", err)
	}
	result := make([]domain.OperatorPasskeyCredential, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *Store) FindActiveOperatorPasskey(ctx context.Context, credentialID, userHandle []byte) (domain.OperatorPasskeyCredential, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, false, domain.OperationStats{}, err
	}
	var row operatorPasskeyRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_passkey_credential".credential_id = ?`, credentialID).
		Where(`"operator_passkey_credential".user_handle = ?`, userHandle).
		Where(`"operator_passkey_credential".revoked_at IS NULL`).
		Limit(1).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorPasskeyCredential{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorPasskeyCredential{}, false, stats, fmt.Errorf("identity access postgres: find active operator passkey: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) UpdateOperatorPasskeyAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorPasskeyRow)(nil)).
		Set("credential_json = ?", json.RawMessage(credentialJSON)).
		Set("last_used_at = ?", usedAt).
		Where(`"operator_passkey_credential".id = ?`, id).
		Where(`"operator_passkey_credential".revoked_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: update operator passkey after use: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) RevokeOperatorPasskey(ctx context.Context, operatorID, id int64, revokedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorPasskeyRow)(nil)).
		Set("revoked_at = ?", revokedAt).
		Where(`"operator_passkey_credential".id = ?`, id).
		Where(`"operator_passkey_credential".operator_id = ?`, operatorID).
		Where(`"operator_passkey_credential".revoked_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: revoke operator passkey: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) InsertOperatorPasskeySession(ctx context.Context, session domain.OperatorPasskeySession) (domain.OperatorPasskeySession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorPasskeySession{}, domain.OperationStats{}, err
	}
	row := operatorPasskeySessionRow{
		ID: session.ID, OperatorID: session.OperatorID, Purpose: session.Purpose, RPID: session.RPID,
		ExpectedOrigin: session.ExpectedOrigin, SessionJSON: session.SessionJSON, ExpiresAt: session.ExpiresAt, ConsumedAt: session.ConsumedAt,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorPasskeySession{}, stats, fmt.Errorf("identity access postgres: insert operator passkey session: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) ConsumeOperatorPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (domain.OperatorPasskeySession, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorPasskeySession{}, false, domain.OperationStats{}, err
	}
	var rows []operatorPasskeySessionRow
	started := time.Now()
	err = db.NewUpdate().Model((*operatorPasskeySessionRow)(nil)).
		Set("consumed_at = ?", consumedAt).
		Where(`"operator_passkey_session".id = ?`, id).
		Where(`"operator_passkey_session".purpose = ?`, purpose).
		Where(`"operator_passkey_session".consumed_at IS NULL`).
		Where(`"operator_passkey_session".expires_at > ?`, consumedAt).
		Returning("*").
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.OperatorPasskeySession{}, false, stats, fmt.Errorf("identity access postgres: consume operator passkey session: %w", err)
	}
	if len(rows) == 0 {
		return domain.OperatorPasskeySession{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}
