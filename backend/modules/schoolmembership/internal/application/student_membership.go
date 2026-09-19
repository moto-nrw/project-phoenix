package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Service) EnrollStudent(ctx context.Context, input domain.StudentEnrollment) (result int64, err error) {
	err = s.runWrite(ctx, "enroll_student", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.EnrollStudent(txCtx, input)
		stats.Add(queryStats)

		if err == nil && result <= 0 {
			return fmt.Errorf("school membership: enrollment returned no membership identity")
		}

		return err
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *Service) RenewStudentEnrollment(ctx context.Context, input domain.StudentEnrollment) (result int64, err error) {
	err = s.runWrite(ctx, "renewenrollment_student", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.RenewStudentEnrollment(txCtx, input)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) AssignStudentGroup(ctx context.Context, studentID int64, groupID *int64) (result bool, err error) {
	err = s.runWrite(ctx, "assigngroup_student", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.AssignStudentGroup(txCtx, studentID, groupID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
