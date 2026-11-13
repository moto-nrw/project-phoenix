package application

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
)

func (s *Service) FindInstanceStaff(ctx context.Context, id int64) (result timetable.InstanceStaff, err error) {
	if id <= 0 {
		return timetable.InstanceStaff{}, s.reject("find_instance_staff", timetable.ErrInvalidInstanceStaffQuery)
	}

	err = s.run(ctx, "find_instance_staff", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindInstanceStaff(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrInstanceStaffNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListInstanceStaff(ctx context.Context, filter timetable.InstanceStaffFilter) (result []timetable.InstanceStaff, err error) {
	if !validInstanceStaffFilter(filter) {
		return nil, s.reject("list_instance_staff", timetable.ErrInvalidInstanceStaffQuery)
	}

	err = s.run(ctx, "list_instance_staff", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListInstanceStaff(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (result map[int64]int, err error) {
	if hasInvalidID(instanceIDs) {
		return nil, s.reject("count_non_absent_instance_staff", timetable.ErrInvalidInstanceStaffQuery)
	}

	err = s.run(ctx, "count_non_absent_instance_staff", func(ctx context.Context, stats *domain.OperationStats) error {
		counts, queryStats, countErr := s.store.CountNonAbsentInstanceStaff(ctx, instanceIDs)
		stats.Add(queryStats)
		result = counts
		return countErr
	})
	return result, err
}

func (s *Service) CreateInstanceStaff(ctx context.Context, fields timetable.InstanceStaffInput) (result timetable.InstanceStaff, err error) {
	if !validInstanceStaff(fields) {
		return timetable.InstanceStaff{}, s.reject("create_instance_staff", timetable.ErrInvalidInstanceStaff)
	}

	err = s.runWrite(ctx, "create_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockStaffAssignment(txCtx, fields.StaffID); err != nil {
			return err
		}
		if err := s.lockInstanceForStaffAssignment(txCtx, fields.InstanceID, stats); err != nil {
			return err
		}
		value, queryStats, createErr := s.store.CreateInstanceStaff(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateInstanceStaff(ctx context.Context, id int64, fields timetable.InstanceStaffInput) (result timetable.InstanceStaff, err error) {
	if id <= 0 || !validInstanceStaff(fields) {
		return timetable.InstanceStaff{}, s.reject("update_instance_staff", timetable.ErrInvalidInstanceStaff)
	}

	err = s.runWrite(ctx, "update_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockStaffAssignment(txCtx, fields.StaffID); err != nil {
			return err
		}
		if err := s.lockInstanceForStaffAssignment(txCtx, fields.InstanceID, stats); err != nil {
			return err
		}
		value, found, queryStats, updateErr := s.store.UpdateInstanceStaff(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrInstanceStaffNotFound
		}
		result = value
		return nil
	})
	return result, err
}

var _ timetable.InstanceStaffLocks = (*Service)(nil)

// lockStaffRows takes the staff rows of a day writer's roster through the
// instance staff capability before the writer's first block row (#6583). A
// capability without the locks fails the write instead of letting it take
// the rows later, against the lock order.
func lockStaffRows(ctx context.Context, staff timetable.InstanceStaffCommand, staffIDs []int64) error {
	if len(staffIDs) == 0 {
		return nil
	}
	locks, ok := staff.(timetable.InstanceStaffLocks)
	if !ok {
		return errors.New("timetable: the instance staff capability takes no staff row locks")
	}
	return locks.LockStaffAssignments(ctx, staffIDs)
}

// LockStaffAssignments takes the staff row lock of every staff member,
// ascending and once each, in the caller's unit of work. A day writer calls it
// after its day locks and before its first block row, so it never holds a
// block while it waits for a staff row that offboarding holds (#6583).
func (s *Service) LockStaffAssignments(ctx context.Context, staffIDs []int64) error {
	if hasInvalidID(staffIDs) {
		return s.reject("lock_staff_assignments", timetable.ErrInvalidInstanceStaff)
	}
	if len(staffIDs) == 0 {
		return nil
	}
	ids := slices.Sorted(slices.Values(staffIDs))
	return s.runWrite(ctx, "lock_staff_assignments", true, func(txCtx context.Context, _ *domain.OperationStats) error {
		return s.lockStaffAssignments(txCtx, slices.Compact(ids))
	})
}

func (s *Service) lockInstanceForStaffAssignment(ctx context.Context, instanceID int64, stats *domain.OperationStats) error {
	_, found, queryStats, err := s.store.LockActivityInstance(ctx, instanceID, false)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found {
		return timetable.ErrActivityInstanceNotFound
	}
	return nil
}

func (s *Service) PatchInstanceStaff(ctx context.Context, id int64, fields timetable.InstanceStaffInput, columns []string) (result int64, err error) {
	if id <= 0 || !validInstanceStaffColumns(columns) {
		return 0, s.reject("patch_instance_staff", timetable.ErrInvalidInstanceStaff)
	}

	err = s.runWrite(ctx, "patch_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.PatchInstanceStaff(txCtx, id, fields, columns)
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

func (s *Service) DeleteInstanceStaff(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_instance_staff", timetable.ErrInvalidInstanceStaff)
	}

	return s.runWrite(ctx, "delete_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStaff(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return s.reject("delete_instance_staff_by_instance", timetable.ErrInvalidInstanceStaff)
	}

	return s.runWrite(ctx, "delete_instance_staff_by_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStaffByInstance(txCtx, instanceID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) deleteInstanceStaffByStaff(ctx context.Context, staffID int64) (result int64, err error) {
	err = s.runWrite(ctx, "delete_instance_staff_by_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteInstanceStaffByStaff(txCtx, staffID)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

func (s *Service) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (result int64, err error) {
	if staffID <= 0 || !timetable.ValidDate(after) {
		return 0, s.reject("delete_upcoming_instance_staff", timetable.ErrInvalidInstanceStaff)
	}

	err = s.runWrite(ctx, "delete_upcoming_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		// A block of the day that Student Presence already runs or ended is
		// history and keeps its staff (#2762); only unstarted plans go.
		sameDay, listStats, listErr := s.store.ListInstanceStaff(txCtx, timetable.InstanceStaffFilter{StaffIDs: []int64{staffID}, Date: &after})
		stats.Add(listStats)
		if listErr != nil {
			return listErr
		}
		instanceIDs := make([]int64, 0, len(sameDay))
		for _, row := range sameDay {
			instanceIDs = append(instanceIDs, row.InstanceID)
		}
		started, factsErr := s.startedInstanceIDs(txCtx, sortedUniqueIDs(instanceIDs))
		if factsErr != nil {
			return factsErr
		}
		excluded := make([]int64, 0, len(started))
		for id := range started {
			excluded = append(excluded, id)
		}
		rows, queryStats, deleteErr := s.store.DeleteUpcomingInstanceStaff(txCtx, staffID, after, sortedUniqueIDs(excluded))
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

func (s *Service) lockStaffAssignment(ctx context.Context, staffID int64) error {
	return s.lockStaffAssignments(ctx, []int64{staffID})
}

// skipHeldStaffRows leaves out staff row locks the unit of work already holds
// (#6583): a day writer takes the staff rows of its roster up front, so every
// later assignment write of the same person would only repeat the locked,
// already validated read.
func skipHeldStaffRows(lock func(context.Context, []int64) error) func(context.Context, []int64) error {
	return func(ctx context.Context, staffIDs []int64) error {
		pending := make([]int64, 0, len(staffIDs))
		for _, staffID := range staffIDs {
			if !tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(staffID)) {
				pending = append(pending, staffID)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		return lock(ctx, pending)
	}
}
