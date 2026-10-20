package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

var errTenantRequired = errors.New("school structure: tenant is required")

// The ledger commands run only inside the deletion workflow's tenant
// transaction. They take the tenant from context like ListGroups does and
// never open a transaction of their own.

func (s *Service) CountStudentTransitionHistory(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, schoolstructure.ErrInvalidStudent
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	var result int
	err = s.run("count_student_transition_history", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		count, queryStats, countErr := s.store.CountStudentTransitionHistory(ctx, tenantID.Int64(), studentID)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

func (s *Service) AnonymizeStudentTransitionHistory(ctx context.Context, studentID int64) (int64, error) {
	if studentID <= 0 {
		return 0, schoolstructure.ErrInvalidStudent
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	var result int64
	err = s.run("anonymize_student_transition_history", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		rows, writeStats, writeErr := s.store.AnonymizeStudentTransitionHistory(ctx, tenantID.Int64(), studentID)
		stats.Add(writeStats)
		result = rows
		return writeErr
	})
	return result, err
}
