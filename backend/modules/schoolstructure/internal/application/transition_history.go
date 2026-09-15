package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

var errTenantRequired = errors.New("school structure: tenant is required")

func (s *Service) CountStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (result int, err error) {
	err = s.run("count_student_transition_history", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		count, queryStats, countErr := s.store.CountStudentTransitionHistory(ctx, tenantID, studentID)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

func (s *Service) AnonymizeStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (result int64, err error) {
	err = s.run("anonymize_student_transition_history", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		rows, writeStats, writeErr := s.store.AnonymizeStudentTransitionHistory(ctx, tenantID, studentID)
		stats.Add(writeStats)
		result = rows
		return writeErr
	})
	return result, err
}
