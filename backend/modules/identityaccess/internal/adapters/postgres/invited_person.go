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

func (s *Store) CountStudentGuardianInvitations(ctx context.Context, studentID, tenantID int64) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	started := time.Now()
	err = db.NewRaw(`SELECT COUNT(*)::int FROM auth.guardian_invitations
		WHERE tenant_id = ? AND student_id = ?`, tenantID, studentID).Scan(ctx, &count)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: count student guardian invitations: %w", err)
	}
	stats.Rows = 1
	return count, stats, nil
}
