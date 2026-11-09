package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// WriteAccountCredential is the one statement that writes the credential of
// an account (#6289): auth.accounts.password_hash, the one-time-password flag
// beside it and the credential generation every artifact of a password check
// is bound to. Every write moves the generation on in the same statement, so
// the MFA codes, enrollment and challenge tokens, remembered devices, reset
// links and sessions issued under the old credential end with it, at every
// school. TestAccountCredentialHasOneWriter keeps it the only writer.
//
// A write without a password hash moves only the generation (an MFA reset).
// The first credential of an account just created (Initial) replaces none:
// it lands only on a row without a password, and the generation stays where
// the row started, as nothing can be bound to an earlier credential. The
// expectations make a write a compare-and-swap: written=false when the row
// is gone or erased, or holds another hash or generation by now.
func (s *Store) WriteAccountCredential(ctx context.Context, accountID int64, write domain.CredentialWrite) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(`auth.accounts AS "account"`).
		Set(`updated_at = NOW()`).
		Where(`"account".id = ?`, accountID).
		Where(liveAccountSQL)
	if write.Initial {
		query = query.Where(`"account".password_hash IS NULL`)
	} else {
		query = query.Set(`mfa_reset_generation = "account".mfa_reset_generation + 1`)
	}
	if write.PasswordHash != "" {
		query = query.Set(`password_hash = ?`, write.PasswordHash).Set(`is_password_otp = FALSE`)
	}
	if write.ExpectedPasswordHash != "" {
		query = query.Where(`"account".password_hash = ?`, write.ExpectedPasswordHash)
	}
	if write.ExpectedGeneration != nil {
		query = query.Where(`"account".mfa_reset_generation = ?`, *write.ExpectedGeneration)
	}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: write account credential: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}
