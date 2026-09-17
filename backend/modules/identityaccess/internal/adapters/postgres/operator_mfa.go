package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// operatorMFACredentialRow mirrors platform.operator_mfa_credentials.
type operatorMFACredentialRow struct {
	bun.BaseModel `bun:"table:platform.operator_mfa_credentials,alias:operator_mfa_credential"`
	ID            int64      `bun:"id,pk,autoincrement"`
	OperatorID    int64      `bun:"operator_id,notnull"`
	Method        string     `bun:"method,notnull"`
	EnrolledAt    time.Time  `bun:"enrolled_at,nullzero,notnull,default:current_timestamp"`
	LastUsedAt    *time.Time `bun:"last_used_at"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorMFACredentialRow) toDomain() domain.OperatorMFACredential {
	return domain.OperatorMFACredential{
		ID: r.ID, OperatorID: r.OperatorID, Method: r.Method, EnrolledAt: r.EnrolledAt, LastUsedAt: r.LastUsedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// operatorMFAChallengeRow mirrors platform.operator_mfa_email_challenges.
type operatorMFAChallengeRow struct {
	bun.BaseModel `bun:"table:platform.operator_mfa_email_challenges,alias:operator_mfa_email_challenge"`
	ID            int64      `bun:"id,pk,autoincrement"`
	OperatorID    int64      `bun:"operator_id,notnull"`
	CodeHash      string     `bun:"code_hash,notnull"`
	ExpiresAt     time.Time  `bun:"expires_at,notnull"`
	ConsumedAt    *time.Time `bun:"consumed_at"`
	IPAddress     net.IP     `bun:"ip_address,type:inet,nullzero"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorMFAChallengeRow) toDomain() domain.OperatorMFAChallenge {
	return domain.OperatorMFAChallenge{
		ID: r.ID, OperatorID: r.OperatorID, CodeHash: r.CodeHash, ExpiresAt: r.ExpiresAt, ConsumedAt: r.ConsumedAt,
		IPAddress: r.IPAddress, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// operatorTrustedDeviceRow mirrors platform.operator_mfa_trusted_devices.
type operatorTrustedDeviceRow struct {
	bun.BaseModel `bun:"table:platform.operator_mfa_trusted_devices,alias:operator_mfa_trusted_device"`
	ID            int64      `bun:"id,pk,autoincrement"`
	OperatorID    int64      `bun:"operator_id,notnull"`
	TokenHash     string     `bun:"token_hash,notnull"`
	UserAgent     *string    `bun:"user_agent"`
	IPAddress     net.IP     `bun:"ip_address,type:inet,nullzero"`
	ExpiresAt     time.Time  `bun:"expires_at,notnull"`
	LastUsedAt    *time.Time `bun:"last_used_at"`
	RevokedAt     *time.Time `bun:"revoked_at"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorTrustedDeviceRow) toDomain() domain.OperatorTrustedDevice {
	return domain.OperatorTrustedDevice{
		ID: r.ID, OperatorID: r.OperatorID, TokenHash: r.TokenHash, UserAgent: r.UserAgent, IPAddress: r.IPAddress,
		ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Store) FindOperatorMFACredential(ctx context.Context, operatorID int64) (domain.OperatorMFACredential, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorMFACredential{}, false, domain.OperationStats{}, err
	}
	var row operatorMFACredentialRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_mfa_credential".operator_id = ?`, operatorID).
		Limit(1).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorMFACredential{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorMFACredential{}, false, stats, fmt.Errorf("identity access postgres: find operator mfa credential: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) InsertOperatorMFACredential(ctx context.Context, credential domain.OperatorMFACredential) (domain.OperatorMFACredential, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorMFACredential{}, domain.OperationStats{}, err
	}
	row := operatorMFACredentialRow{
		OperatorID: credential.OperatorID, Method: credential.Method, EnrolledAt: credential.EnrolledAt, LastUsedAt: credential.LastUsedAt,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorMFACredential{}, stats, fmt.Errorf("identity access postgres: insert operator mfa credential: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorMFACredentialRow)(nil)).
		Set("last_used_at = ?", usedAt).
		Where(`"operator_mfa_credential".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: touch operator mfa credential: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Model((*operatorMFACredentialRow)(nil)).
		Where(`"operator_mfa_credential".operator_id = ?`, operatorID).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete operator mfa credentials: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) InsertOperatorMFAChallenge(ctx context.Context, challenge domain.OperatorMFAChallenge) (domain.OperatorMFAChallenge, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorMFAChallenge{}, domain.OperationStats{}, err
	}
	row := operatorMFAChallengeRow{
		OperatorID: challenge.OperatorID, CodeHash: challenge.CodeHash, ExpiresAt: challenge.ExpiresAt,
		ConsumedAt: challenge.ConsumedAt, IPAddress: challenge.IPAddress,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorMFAChallenge{}, stats, fmt.Errorf("identity access postgres: insert operator mfa challenge: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64, now time.Time) (domain.OperatorMFAChallenge, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorMFAChallenge{}, false, domain.OperationStats{}, err
	}
	var row operatorMFAChallengeRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_mfa_email_challenge".operator_id = ?`, operatorID).
		Where(`"operator_mfa_email_challenge".consumed_at IS NULL`).
		Where(`"operator_mfa_email_challenge".expires_at > ?`, now).
		OrderExpr(`"operator_mfa_email_challenge".expires_at DESC`).
		Limit(1).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorMFAChallenge{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorMFAChallenge{}, false, stats, fmt.Errorf("identity access postgres: find active operator mfa challenge: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().Model((*operatorMFAChallengeRow)(nil)).
		Where(`"operator_mfa_email_challenge".operator_id = ?`, operatorID).
		Where(`"operator_mfa_email_challenge".created_at >= ?`, since).
		Count(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: count operator mfa challenges: %w", err)
	}
	return count, stats, nil
}

func (s *Store) ActivateOperatorMFAChallenge(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorMFAChallengeRow)(nil)).
		Set("consumed_at = NULL").
		Where(`"operator_mfa_email_challenge".id = ?`, id).
		Where(`"operator_mfa_email_challenge".consumed_at IS NOT NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: activate operator mfa challenge: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorMFAChallengeRow)(nil)).
		Set("consumed_at = ?", consumedAt).
		Where(`"operator_mfa_email_challenge".id = ?`, id).
		Where(`"operator_mfa_email_challenge".consumed_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: consume operator mfa challenge: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) InsertOperatorTrustedDevice(ctx context.Context, device domain.OperatorTrustedDevice) (domain.OperatorTrustedDevice, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorTrustedDevice{}, domain.OperationStats{}, err
	}
	row := operatorTrustedDeviceRow{
		OperatorID: device.OperatorID, TokenHash: device.TokenHash, UserAgent: device.UserAgent, IPAddress: device.IPAddress,
		ExpiresAt: device.ExpiresAt, LastUsedAt: device.LastUsedAt, RevokedAt: device.RevokedAt,
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorTrustedDevice{}, stats, fmt.Errorf("identity access postgres: insert operator trusted device: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string, now time.Time) (domain.OperatorTrustedDevice, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorTrustedDevice{}, false, domain.OperationStats{}, err
	}
	var row operatorTrustedDeviceRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_mfa_trusted_device".operator_id = ?`, operatorID).
		Where(`"operator_mfa_trusted_device".token_hash = ?`, tokenHash).
		Where(`"operator_mfa_trusted_device".revoked_at IS NULL`).
		Where(`"operator_mfa_trusted_device".expires_at > ?`, now).
		Limit(1).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorTrustedDevice{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorTrustedDevice{}, false, stats, fmt.Errorf("identity access postgres: find active operator trusted device: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64, now time.Time) ([]domain.OperatorTrustedDevice, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []operatorTrustedDeviceRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where(`"operator_mfa_trusted_device".operator_id = ?`, operatorID).
		Where(`"operator_mfa_trusted_device".revoked_at IS NULL`).
		Where(`"operator_mfa_trusted_device".expires_at > ?`, now).
		OrderExpr(`"operator_mfa_trusted_device".last_used_at DESC NULLS LAST`).
		OrderExpr(`"operator_mfa_trusted_device".created_at DESC`).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list active operator trusted devices: %w", err)
	}
	result := make([]domain.OperatorTrustedDevice, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *Store) TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorTrustedDeviceRow)(nil)).
		Set("last_used_at = ?", usedAt).
		Where(`"operator_mfa_trusted_device".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: touch operator trusted device: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorTrustedDeviceRow)(nil)).
		Set("revoked_at = ?", revokedAt).
		Where(`"operator_mfa_trusted_device".id = ?`, id).
		Where(`"operator_mfa_trusted_device".revoked_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: revoke operator trusted device: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

func (s *Store) RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorTrustedDeviceRow)(nil)).
		Set("revoked_at = ?", revokedAt).
		Where(`"operator_mfa_trusted_device".operator_id = ?`, operatorID).
		Where(`"operator_mfa_trusted_device".revoked_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: revoke operator trusted devices: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}
