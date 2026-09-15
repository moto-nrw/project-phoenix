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

// operatorRow mirrors platform.operators. The default tags keep the insert
// semantics of the retained model: a zero Active value stores the column
// default and zero timestamps take the database clock.
type operatorRow struct {
	bun.BaseModel  `bun:"table:platform.operators,alias:operator"`
	ID             int64      `bun:"id,pk,autoincrement"`
	Email          string     `bun:"email,notnull"`
	DisplayName    string     `bun:"display_name,notnull"`
	PasswordHash   string     `bun:"password_hash,notnull"`
	Active         bool       `bun:"active,notnull,default:true"`
	LastLogin      *time.Time `bun:"last_login"`
	MFAAttempts    int        `bun:"mfa_attempts,default:0"`
	MFALockedUntil *time.Time `bun:"mfa_locked_until"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorRow) toDomain() domain.Operator {
	return domain.Operator{
		ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, PasswordHash: r.PasswordHash, Active: r.Active,
		LastLogin: r.LastLogin, MFAAttempts: r.MFAAttempts, MFALockedUntil: r.MFALockedUntil,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func operatorRowFromDomain(operator domain.Operator) operatorRow {
	return operatorRow{
		ID: operator.ID, Email: operator.Email, DisplayName: operator.DisplayName, PasswordHash: operator.PasswordHash, Active: operator.Active,
		LastLogin: operator.LastLogin, MFAAttempts: operator.MFAAttempts, MFALockedUntil: operator.MFALockedUntil,
		CreatedAt: operator.CreatedAt, UpdatedAt: operator.UpdatedAt,
	}
}

// operatorSessionRow mirrors platform.operator_refresh_tokens.
type operatorSessionRow struct {
	bun.BaseModel     `bun:"table:platform.operator_refresh_tokens,alias:operator_refresh_token"`
	ID                int64      `bun:"id,pk,autoincrement"`
	OperatorID        int64      `bun:"operator_id,notnull"`
	Token             string     `bun:"token,notnull"`
	Expiry            time.Time  `bun:"expiry,notnull"`
	FamilyID          string     `bun:"family_id,notnull"`
	Generation        int        `bun:"generation,notnull,default:0"`
	RotatedAt         *time.Time `bun:"rotated_at"`
	ReplacementToken  *string    `bun:"replacement_token"`
	RecoveryProofHash []byte     `bun:"recovery_proof_hash"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r operatorSessionRow) toDomain() domain.OperatorSession {
	return domain.OperatorSession{
		ID: r.ID, OperatorID: r.OperatorID, Token: r.Token, Expiry: r.Expiry, FamilyID: r.FamilyID, Generation: r.Generation,
		RotatedAt: r.RotatedAt, ReplacementToken: r.ReplacementToken, RecoveryProofHash: r.RecoveryProofHash,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func operatorSessionRowFromDomain(session domain.OperatorSession) operatorSessionRow {
	return operatorSessionRow{
		ID: session.ID, OperatorID: session.OperatorID, Token: session.Token, Expiry: session.Expiry, FamilyID: session.FamilyID, Generation: session.Generation,
		RotatedAt: session.RotatedAt, ReplacementToken: session.ReplacementToken, RecoveryProofHash: session.RecoveryProofHash,
		CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	}
}

func operatorSessionsToDomain(rows []operatorSessionRow) []domain.OperatorSession {
	result := make([]domain.OperatorSession, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result
}

func (s *Store) FindOperator(ctx context.Context, id int64, forUpdate bool) (domain.Operator, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Operator{}, false, domain.OperationStats{}, err
	}
	var row operatorRow
	query := db.NewSelect().Model(&row).Where(`"operator".id = ?`, id)
	if forUpdate {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Operator{}, false, stats, nil
	}
	if err != nil {
		return domain.Operator{}, false, stats, fmt.Errorf("identity access postgres: find operator: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) FindOperatorByEmail(ctx context.Context, email string) (domain.Operator, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Operator{}, false, domain.OperationStats{}, err
	}
	var row operatorRow
	started := time.Now()
	err = db.NewSelect().Model(&row).Where(`"operator".email = ?`, email).Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Operator{}, false, stats, nil
	}
	if err != nil {
		return domain.Operator{}, false, stats, fmt.Errorf("identity access postgres: find operator by email: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) ListOperators(ctx context.Context) ([]domain.Operator, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []operatorRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).OrderExpr(`"operator".display_name ASC`).Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list operators: %w", err)
	}
	result := make([]domain.Operator, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, stats, nil
}

func (s *Store) InsertOperator(ctx context.Context, operator domain.Operator) (domain.Operator, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Operator{}, domain.OperationStats{}, err
	}
	row := operatorRowFromDomain(operator)
	row.ID = 0
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.Operator{}, stats, fmt.Errorf("identity access postgres: insert operator: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

// UpdateOperator overwrites every column except the identity, matching the
// retained generic update that wrote the whole model back.
func (s *Store) UpdateOperator(ctx context.Context, operator domain.Operator) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	row := operatorRowFromDomain(operator)
	started := time.Now()
	result, err := db.NewUpdate().Model(&row).WherePK().Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: update operator: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows > 0, stats, nil
}

func (s *Store) DeleteOperator(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Model((*operatorRow)(nil)).Where(`"operator".id = ?`, id).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete operator: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) RecordOperatorLogin(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorRow)(nil)).
		Set("last_login = ?", at).
		Where(`"operator".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: record operator login: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// IncrementOperatorMFAAttempts is the SQL-level compare-and-set that keeps
// concurrent failed verifications from collapsing into one counted attempt.
func (s *Store) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockedUntil time.Time) (domain.OperatorMFAAttempts, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorMFAAttempts{}, false, domain.OperationStats{}, err
	}
	var row struct {
		MFAAttempts    int        `bun:"mfa_attempts"`
		MFALockedUntil *time.Time `bun:"mfa_locked_until"`
	}
	started := time.Now()
	err = db.NewUpdate().Model((*operatorRow)(nil)).
		Set("mfa_attempts = mfa_attempts + 1").
		Set("mfa_locked_until = CASE WHEN mfa_attempts + 1 >= ? THEN ? ELSE mfa_locked_until END", threshold, lockedUntil).
		Where(`"operator".id = ?`, id).
		Returning("mfa_attempts, mfa_locked_until").
		Scan(ctx, &row)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorMFAAttempts{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorMFAAttempts{}, false, stats, fmt.Errorf("identity access postgres: increment operator mfa attempts: %w", err)
	}
	stats.Rows = 1
	return domain.OperatorMFAAttempts{Attempts: row.MFAAttempts, LockedUntil: row.MFALockedUntil}, true, stats, nil
}

func (s *Store) ResetOperatorMFAAttempts(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorRow)(nil)).
		Set("mfa_attempts = 0").
		Set("mfa_locked_until = NULL").
		Where(`"operator".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: reset operator mfa attempts: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) FindOperatorSessionByToken(ctx context.Context, token string, forUpdate bool) (domain.OperatorSession, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorSession{}, false, domain.OperationStats{}, err
	}
	var row operatorSessionRow
	query := db.NewSelect().Model(&row).Where(`"operator_refresh_token".token = ?`, token)
	if forUpdate {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorSession{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorSession{}, false, stats, fmt.Errorf("identity access postgres: find operator session: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (domain.OperatorSession, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorSession{}, false, domain.OperationStats{}, err
	}
	var row operatorSessionRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Where(`"operator_refresh_token".family_id = ?`, familyID).
		OrderExpr(`"operator_refresh_token".generation DESC`).
		Limit(1).
		Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperatorSession{}, false, stats, nil
	}
	if err != nil {
		return domain.OperatorSession{}, false, stats, fmt.Errorf("identity access postgres: latest operator session in family: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) InsertOperatorSession(ctx context.Context, session domain.OperatorSession) (domain.OperatorSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperatorSession{}, domain.OperationStats{}, err
	}
	row := operatorSessionRowFromDomain(session)
	row.ID = 0
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.OperatorSession{}, stats, fmt.Errorf("identity access postgres: insert operator session: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Model((*operatorSessionRow)(nil)).
		Set("rotated_at = ?", rotatedAt).
		Set("replacement_token = ?", replacementToken).
		Set("recovery_proof_hash = ?", recoveryProofHash).
		Where(`"operator_refresh_token".id = ?`, id).
		Where(`"operator_refresh_token".rotated_at IS NULL`).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: mark operator session rotated: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

// DeleteExpiredRotatedOperatorSessions removes predecessor rows only after
// their refresh JWTs expire, preserving family evidence for replay detection
// until then.
func (s *Store) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	candidates := db.NewSelect().Model((*operatorSessionRow)(nil)).
		ColumnExpr(`"operator_refresh_token".id`).
		Where(`"operator_refresh_token".family_id = ?`, familyID).
		Where(`"operator_refresh_token".rotated_at IS NOT NULL`).
		Where(`"operator_refresh_token".expiry <= ?`, now).
		For("UPDATE SKIP LOCKED")
	started := time.Now()
	result, err := db.NewDelete().Model((*operatorSessionRow)(nil)).
		Where(`"operator_refresh_token".id IN (?)`, candidates).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete expired rotated operator sessions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) DeleteOperatorSession(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Model((*operatorSessionRow)(nil)).Where(`"operator_refresh_token".id = ?`, id).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete operator session: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// DeleteOperatorSessionsByOperator returns the deleted rows so revocation and
// its per-family audit evidence commit in the caller's one transaction.
func (s *Store) DeleteOperatorSessionsByOperator(ctx context.Context, operatorID int64) ([]domain.OperatorSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []operatorSessionRow
	started := time.Now()
	err = db.NewDelete().Model((*operatorSessionRow)(nil)).
		Where(`"operator_refresh_token".operator_id = ?`, operatorID).
		Returning("*").
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: delete operator sessions by operator: %w", err)
	}
	stats.Rows = int64(len(rows))
	return operatorSessionsToDomain(rows), stats, nil
}

func (s *Store) DeleteOperatorSessionsByFamily(ctx context.Context, familyID string) ([]domain.OperatorSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []operatorSessionRow
	started := time.Now()
	err = db.NewDelete().Model((*operatorSessionRow)(nil)).
		Where(`"operator_refresh_token".family_id = ?`, familyID).
		Returning("*").
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: delete operator session family: %w", err)
	}
	stats.Rows = int64(len(rows))
	return operatorSessionsToDomain(rows), stats, nil
}

func (s *Store) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Model((*operatorSessionRow)(nil)).
		Where(`"operator_refresh_token".expiry < ?`, now).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: delete expired operator sessions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return int(stats.Rows), stats, nil
}
