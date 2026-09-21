package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) AnonymizeAccountForDeletion(ctx context.Context, accountID int64, email string) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw("UPDATE auth.accounts SET email = ?, username = NULL WHERE id = ?", email, accountID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("anonymize account for deletion: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
