package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Service) FindInvitedPersonIDs(ctx context.Context, email string) ([]int64, error) {
	var ids []int64
	err := s.run(ctx, s.withinSchoolRead, "find_invited_person_ids", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		ids, queryStats, queryErr = s.store.FindInvitedPersonIDs(txCtx, strings.ToLower(strings.TrimSpace(email)), s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return nil, fmt.Errorf("identity access: find invited people: %w", MapError(err))
	}
	return ids, nil
}

func (s *Service) CountStudentGuardianInvitations(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, fmt.Errorf("identity access: count student guardian invitations: student ID is required")
	}
	var count int
	err := s.run(ctx, s.withinSchoolRead, "count_student_guardian_invitations", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		count, queryStats, queryErr = s.store.CountStudentGuardianInvitations(txCtx, studentID, s.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return 0, fmt.Errorf("identity access: count student guardian invitations: %w", MapError(err))
	}
	return count, nil
}
