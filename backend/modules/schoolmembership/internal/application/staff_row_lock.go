package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// findStaffRow is the one read of a staff row and, with a lock, the one
// acquisition of a staff row lock (#6583). The lock is admitted through the
// unit of work's lock order first (tenant.LockStaffRows): staff rows come
// after the day locks, ascending by id, and before every block row. A staff
// row lock after a later staff row, or a gate or day after a staff row, is
// refused before it waits, so the path fails instead of closing a deadlock
// cycle with offboarding.
func (s *Service) findStaffRow(ctx context.Context, id int64, lock string, includeDeleted bool) (schoolmembership.Staff, bool, domain.OperationStats, error) {
	if lock != "" {
		if err := tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(id)); err != nil {
			return schoolmembership.Staff{}, false, domain.OperationStats{}, err
		}
	}
	return s.store.FindStaff(ctx, id, lock, includeDeleted)
}

func (s *Service) listStaffForMutation(ctx context.Context, filter schoolmembership.StaffFilter) (result []schoolmembership.Staff, err error) {
	if len(filter.IDs) == 0 {
		return nil, invalid("staff IDs are required for mutation")
	}
	slices.Sort(filter.IDs)
	err = s.runWrite(ctx, "find_staff_by_ids_for_mutation", func(txCtx context.Context, stats *domain.OperationStats) error {
		for _, id := range filter.IDs {
			if err := tenant.OrderLock(txCtx, tenant.LockStaffRows, tenant.LockIDKey(id)); err != nil {
				return err
			}
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaff(txCtx, filter)
		stats.Add(queryStats)
		if err != nil || filter.MembershipOnly {
			return err
		}
		return s.composeEmployment(txCtx, result, stats)
	})
	return result, err
}
