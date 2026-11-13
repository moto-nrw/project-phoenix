package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// RemoveShiftType deletes an unreferenced type and archives a referenced one
// (#6845). The FKs of staff_shifts and staff_shift_series would set the
// type to NULL on delete and erase the classification of planned and past
// shifts. The type row stays locked from the count to the write, so a
// concurrent plan write naming the type either commits first and is counted
// or waits until the removal is done.
func (s *Service) RemoveShiftType(ctx context.Context, id int64) (workforce.ShiftTypeRemoval, error) {
	if id <= 0 {
		return "", workforce.ErrShiftTypeNotFound
	}
	var removal workforce.ShiftTypeRemoval
	// One unit of work: s.run opens or joins it, so the lock holds until the
	// archive or delete commits.
	err := s.run(ctx, "remove_shift_type", func(ctx context.Context, stats *domain.OperationStats) error {
		found, lockStats, err := s.store.LockShiftType(ctx, id)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrShiftTypeNotFound
		}
		references, countStats, err := s.store.CountShiftTypeReferences(ctx, id)
		stats.Add(countStats)
		if err != nil {
			return err
		}
		if references.Referenced() {
			removal = workforce.ShiftTypeRemovalArchived
			_, _, archiveStats, archiveErr := s.store.ArchiveShiftType(ctx, id)
			stats.Add(archiveStats)
			return archiveErr
		}
		removal = workforce.ShiftTypeRemovalDeleted
		deleteStats, deleteErr := s.store.DeleteShiftType(ctx, id)
		stats.Add(deleteStats)
		return deleteErr
	})
	if err != nil {
		return "", mapShiftError(err)
	}
	return removal, nil
}

// ShiftPlanReferences counts the plan's references per shift type and the
// series per calendar period in one read.
func (s *Service) ShiftPlanReferences(ctx context.Context) (workforce.ShiftPlanReferences, error) {
	result := workforce.ShiftPlanReferences{
		ByShiftType:            map[int64]workforce.ShiftTypeReferences{},
		SeriesByCalendarPeriod: map[int64]int{},
	}
	err := s.run(ctx, "shift_plan_references", func(ctx context.Context, stats *domain.OperationStats) error {
		counts, queryStats, err := s.store.CountShiftPlanReferences(ctx)
		stats.Add(queryStats)
		for id, entry := range counts.ByShiftType {
			result.ByShiftType[id] = workforce.ShiftTypeReferences(entry)
		}
		for id, count := range counts.SeriesByCalendarPeriod {
			result.SeriesByCalendarPeriod[id] = count
		}
		return err
	})
	if err != nil {
		return workforce.ShiftPlanReferences{}, mapShiftError(err)
	}
	return result, nil
}
