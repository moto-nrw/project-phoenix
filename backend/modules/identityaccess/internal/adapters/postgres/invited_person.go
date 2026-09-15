package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) FindInvitedPersonIDs(ctx context.Context, email string, tenantID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewRaw(`SELECT person_id FROM auth.invitation_tokens
		WHERE LOWER(email) = ? AND tenant_id = ? AND used_at IS NULL AND person_id IS NOT NULL`, email, tenantID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: find invited people: %w", err)
	}
	return ids, stats, nil
}
