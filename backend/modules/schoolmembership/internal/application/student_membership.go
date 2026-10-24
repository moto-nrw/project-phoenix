package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Service) Enroll(ctx context.Context, input schoolmembership.StudentEnrollment) (result int64, err error) {
	if input.Status == "" {
		input.Status = "active"
	}
	if err := validateStudentEnrollment(input, false); err != nil {
		return 0, err
	}
	err = s.runWrite(ctx, "enroll_student", func(txCtx context.Context, stats *domain.OperationStats) error {
		return s.countingWrite(txCtx, stats, func(writeCtx context.Context) error {
			id, queryStats, writeErr := s.store.EnrollStudent(writeCtx, input)
			stats.Add(queryStats)
			if writeErr == nil && id <= 0 {
				return fmt.Errorf("school membership: enrollment returned no membership identity")
			}
			result = id
			return writeErr
		})
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

// RenewEnrollment counts only when the renewal brings back a child that did
// not count before; renewing a counted child always passes.
func (s *Service) RenewEnrollment(ctx context.Context, input schoolmembership.StudentEnrollment) (result int64, err error) {
	if input.Status == "" {
		input.Status = "active"
	}
	if err := validateStudentEnrollment(input, true); err != nil {
		return 0, err
	}
	err = s.runWrite(ctx, "renewenrollment_student", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		return s.countingWrite(txCtx, stats, func(writeCtx context.Context) error {
			id, queryStats, writeErr := s.store.RenewStudentEnrollment(writeCtx, input)
			stats.Add(queryStats)
			result = id
			return writeErr
		})
	})
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *Service) AssignGroup(ctx context.Context, studentID int64, groupID *int64) (result bool, err error) {
	if studentID <= 0 || (groupID != nil && *groupID <= 0) {
		return false, invalid("student and group IDs must be positive")
	}
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
