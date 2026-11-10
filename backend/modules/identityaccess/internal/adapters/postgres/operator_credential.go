package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// operatorCurrentCredential admits an operator code or remembered device only
// while the operator's credential generation is the one it was issued under:
// every password write ends it (#6308).
const operatorCurrentCredential = `credential_generation = (SELECT credential_operator.credential_generation FROM platform.operators AS credential_operator WHERE credential_operator.id = operator_id)`

// WriteOperatorCredential is the one statement that changes the password of
// an operator (#6308): platform.operators.password_hash and the credential
// generation every artifact of a password check is bound to. Every write
// moves the generation on in the same statement, so the e-mail codes,
// enrollment and challenge tokens and remembered devices issued under the old
// password end with it. TestOperatorCredentialHasOneWriter keeps it the only
// writer; the operator's first password arrives with its insert, at the
// generation a new row starts with.
//
// The expectations make a write a compare-and-swap: written=false when the
// row is gone or holds another hash or generation by now. An operator has no
// first-credential write and no write without a password (domain.CredentialWrite
// Initial and an empty PasswordHash are refused).
func (s *Store) WriteOperatorCredential(ctx context.Context, operatorID int64, write domain.CredentialWrite) (bool, domain.OperationStats, error) {
	if write.Initial || write.PasswordHash == "" {
		return false, domain.OperationStats{}, fmt.Errorf("identity access postgres: write operator credential: an operator credential write names a new password")
	}
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	expectedHash, expectedGeneration := any(nil), any(nil)
	if write.ExpectedPasswordHash != "" {
		expectedHash = write.ExpectedPasswordHash
	}
	if write.ExpectedGeneration != nil {
		expectedGeneration = *write.ExpectedGeneration
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE platform.operators AS operator
	SET password_hash = ?, credential_generation = operator.credential_generation + 1, updated_at = NOW()
	WHERE operator.id = ?
		AND (?::text IS NULL OR operator.password_hash = ?::text)
		AND (?::bigint IS NULL OR operator.credential_generation = ?::bigint)`,
		write.PasswordHash, operatorID, expectedHash, expectedHash, expectedGeneration, expectedGeneration).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: write operator credential: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}
