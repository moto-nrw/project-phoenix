package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// TransitionStatus is the scheduler's compare-and-set. Graduation and
// reactivation are separate commands and cannot be requested through it.
func (s *Service) TransitionStatus(ctx context.Context, id int64, expected, next string) (changed bool, err error) {
	if id <= 0 {
		return false, invalid("student ID is required")
	}
	for _, value := range []string{expected, next} {
		if !lifecycleStatus(value) {
			return false, invalid("status transition requires non-alumni lifecycle states")
		}
	}
	err = s.runWrite(ctx, "transition_student_status", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		transition := func(writeCtx context.Context) (bool, error) {
			moved, queryStats, writeErr := s.store.TransitionStudentStatus(writeCtx, id, expected, next)
			stats.Add(queryStats)
			changed = moved
			return moved, writeErr
		}
		if expected == "pending" && next == "active" {
			return s.countingTransition(txCtx, stats, transition)
		}
		_, err := transition(txCtx)
		return err
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// SetStatus is the unconditional status change. Unlike the scheduler's
// compare-and-set it can bring an inactive child back, so it is checked
// against the Kinderkontingent (#3567).
func (s *Service) SetStatus(ctx context.Context, id int64, status string) (changed bool, err error) {
	if id <= 0 {
		return false, invalid("student ID is required")
	}
	if !lifecycleStatus(status) {
		return false, invalid("status change requires a non-alumni lifecycle state")
	}
	err = s.runWrite(ctx, "set_student_status", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		return s.countingWrite(txCtx, stats, func(writeCtx context.Context) error {
			moved, queryStats, writeErr := s.store.TransitionStudentStatus(writeCtx, id, "", status)
			stats.Add(queryStats)
			changed = moved
			return writeErr
		})
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// ResumeCare changes only an ended, non-alumni care interval. On is the
// caller's frozen calendar day, not the database server's timezone.
func (s *Service) ResumeCare(ctx context.Context, id int64, from, status, on string) (changed bool, err error) {
	if id <= 0 {
		return false, invalid("student ID is required")
	}
	if from == "" || on == "" {
		return false, invalid("care start and current day are required")
	}
	if err := validateDate(from, "care start"); err != nil {
		return false, err
	}
	if err := validateDate(on, "current day"); err != nil {
		return false, err
	}
	if !lifecycleStatus(status) {
		return false, invalid("care resume requires a non-alumni lifecycle status")
	}
	err = s.runWrite(ctx, "resume_student_care", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		return s.countingWrite(txCtx, stats, func(writeCtx context.Context) error {
			resumed, queryStats, writeErr := s.store.ResumeStudentCare(writeCtx, id, from, status, on)
			stats.Add(queryStats)
			changed = resumed
			return writeErr
		})
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (s *Service) EndCare(ctx context.Context, ids []int64, until string) (changed int64, err error) {
	ids = uniquePositive(ids)
	if err := validateDate(until, "care end"); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "end_student_care", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.EndStudentCare(txCtx, ids, until)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

// Reactivate brings graduated children back. The whole batch is checked
// against the Kinderkontingent as one write unless the caller reverts a grade
// transition and says so (#3567).
func (s *Service) Reactivate(ctx context.Context, ids []int64, status string, check schoolmembership.ChildQuotaCheck) (changed []int64, err error) {
	ids = uniquePositive(ids)
	status = strings.TrimSpace(status)
	if !lifecycleStatus(status) {
		return nil, invalid("reactivation requires a non-alumni lifecycle status")
	}
	if !validChildQuotaCheck(check) {
		return nil, invalid("reactivation requires an explicit child quota check")
	}
	if len(ids) == 0 {
		return []int64{}, nil
	}
	enforceChildQuota := check == schoolmembership.EnforceChildQuota
	err = s.runWrite(ctx, "reactivate_students", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		reactivate := func(writeCtx context.Context) error {
			reactivated, queryStats, writeErr := s.store.ReactivateStudents(writeCtx, ids, status)
			stats.Add(queryStats)
			changed = reactivated
			return writeErr
		}
		if !enforceChildQuota {
			return reactivate(txCtx)
		}
		return s.countingWrite(txCtx, stats, reactivate)
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

func (s *Service) Graduate(ctx context.Context, ids []int64) (changed int64, err error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "graduate_students", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.GraduateStudents(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

// ChangeClass compares the current class before changing it, making retry
// and grade-transition reversal conditional on the state the caller
// observed.
func (s *Service) ChangeClass(ctx context.Context, ids []int64, from, to string) (changed int64, err error) {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return 0, invalid("source and destination class are required")
	}
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "change_student_class", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.ChangeStudentClass(txCtx, ids, from, to)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

// LockStudentClassWrites fences membership writers before any student row
// lock. Grade transition takes it exclusively; individual writers share it.
func (s *Service) LockStudentClassWrites(ctx context.Context, exclusive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := s.observeRun(ctx, "lock_student_class_writes", func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockStudentClassWrites(txCtx, exclusive)
		stats.Add(queryStats)
		return err
	})

	if err != nil {
		return fmt.Errorf("school membership: lock class writes: %w", err)
	}
	return nil
}
