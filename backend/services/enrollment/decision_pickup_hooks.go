package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// An approved weekly Gehzeit plan (applyTargetedFields) changes the same
// pickup baseline a staff weekly edit does. These hooks take the student lock
// first, compare the plan before and after, and re-derive the coupled auto
// excusals in the caller's transaction. They are nil-safe for focused tests.

func (s *decisionService) lockPickupStudents(ctx context.Context, studentIDs []int64) error {
	if s.LockPickupStudents == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := s.LockPickupStudents(ctx, studentIDs); err != nil {
		return fmt.Errorf("lock students for pickup reset: %w", err)
	}
	return nil
}

func (s *decisionService) resyncPickupAutoExcusals(ctx context.Context, studentIDs []int64) error {
	if s.ResyncPickupAutoExcusals == nil || len(studentIDs) == 0 {
		return nil
	}
	if err := s.ResyncPickupAutoExcusals(ctx, studentIDs); err != nil {
		return fmt.Errorf("resync pickup auto excusals: %w", err)
	}
	return nil
}

func (s *decisionService) snapshotPickupWeekdayChanges(
	ctx context.Context, studentID int64, date timezone.Date,
) (map[int]string, error) {
	if s.SnapshotPickupWeekdayChanges == nil {
		return nil, nil
	}
	before, err := s.SnapshotPickupWeekdayChanges(ctx, studentID, date)
	if err != nil {
		return nil, fmt.Errorf("snapshot pickup weekday changes: %w", err)
	}
	return before, nil
}

func (s *decisionService) recordPickupWeekdayChanges(
	ctx context.Context, studentID int64, date timezone.Date, before map[int]string,
) error {
	if s.RecordPickupWeekdayChanges == nil || before == nil {
		return nil
	}
	if err := s.RecordPickupWeekdayChanges(ctx, studentID, date, before); err != nil {
		return fmt.Errorf("record pickup weekday changes: %w", err)
	}
	return nil
}
