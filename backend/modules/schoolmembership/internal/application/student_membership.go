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
		return s.countingWriteAfterClassGate(txCtx, stats, func(writeCtx context.Context) error {
			id, queryStats, writeErr := s.store.RenewStudentEnrollment(writeCtx, input, s.today())
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

// MovePendingWindow moves the window of a still pending child (#6505). The
// row lock comes first, so a status change committed meanwhile is seen and
// the child is left alone. No class gate is taken: the move writes no class,
// and a caller holding the recurrence gate must not wait for it afterwards.
// The Kontingentzahl cannot rise: the counted pending children are those
// whose start lies ahead, and the move keeps or ends that.
func (s *Service) MovePendingWindow(ctx context.Context, move schoolmembership.PendingWindowMove) (moved bool, err error) {
	if move.StudentID <= 0 {
		return false, invalid("student ID is required")
	}
	err = s.runWrite(ctx, "move_pending_student_window", func(txCtx context.Context, stats *domain.OperationStats) error {
		current, found, queryStats, readErr := s.store.LockStudentWindow(txCtx, move.StudentID)
		stats.Add(queryStats)
		if readErr != nil || !found || current.Status != "pending" {
			return readErr
		}
		moved = true
		next := current
		if current.EnrolledFrom == move.FromBefore {
			next.EnrolledFrom = move.FromAfter
		}
		if current.EnrolledUntil == move.UntilBefore {
			next.EnrolledUntil = move.UntilAfter
		}
		if next.EnrolledFrom == current.EnrolledFrom && next.EnrolledUntil == current.EnrolledUntil {
			return nil
		}
		if next.EnrolledFrom != "" && next.EnrolledUntil != "" && next.EnrolledFrom > next.EnrolledUntil {
			return invalid("the moved enrollment window would end before it starts")
		}
		_, writeStats, writeErr := s.store.RenewStudentEnrollment(txCtx, next, s.today())
		stats.Add(writeStats)
		return writeErr
	})
	if err != nil {
		return false, err
	}
	return moved, nil
}
