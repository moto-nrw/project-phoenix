package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// EraseNextRetiredStaff takes the next staff member above afterID who left
// before the cutoff (#3963): it marks the tombstone erased and deletes the
// Membership records that still name them, the teacher record and its group
// assignments, the guest profile and the class assignments. The marking
// comes first and locks the tombstone, so a concurrent run never takes the
// same staff member. found is false when none is due.
func (s *Service) EraseNextRetiredStaff(ctx context.Context, retiredBefore time.Time, afterID int64) (result schoolmembership.RetiredStaffErasure, found bool, err error) {
	if retiredBefore.IsZero() || afterID < 0 {
		return schoolmembership.RetiredStaffErasure{}, false, errors.New("school membership: cutoff and cursor are required")
	}
	err = s.runWrite(ctx, "erase_next_retired_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var markStats domain.OperationStats
		result, found, markStats, err = s.store.MarkNextRetiredStaffErased(txCtx, retiredBefore, afterID)
		stats.Add(markStats)
		if err != nil || !found {
			return err
		}
		for _, erase := range []struct {
			count *int64
			run   func(context.Context, int64) (domain.OperationStats, error)
		}{
			{&result.GroupAssignments, s.store.DeleteGroupAssignmentsByStaff},
			{&result.Teachers, s.store.DeleteTeachersByStaff},
			{&result.Guests, s.store.DeleteGuestsByStaff},
			{&result.ClassAssignments, s.store.DeleteClassAssignmentsByStaff},
		} {
			deleteStats, err := erase.run(txCtx, result.StaffID)
			stats.Add(deleteStats)
			if err != nil {
				return err
			}
			*erase.count = deleteStats.Rows
		}
		return nil
	})
	if err != nil {
		return schoolmembership.RetiredStaffErasure{}, false, err
	}
	return result, found, nil
}
