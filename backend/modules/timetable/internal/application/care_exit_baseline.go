package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// CountPlannedRosterForCareExit measures the roster a care exit after `after`
// takes away against the pre-exit baseline: the live planned rows plus the
// rows an earlier exit of the same children removed and changing that exit
// puts back first. The restorable half skips a row somebody re-planned by
// hand since, because the live half already counted it.
func (s *Service) CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string, restorable []domain.CareExitRosterRow) (counts map[int64]int, err error) {
	counts = map[int64]int{}
	if len(studentIDs) == 0 {
		return counts, nil
	}
	err = s.run("count_planned_roster_for_care_exit", func(stats *domain.OperationStats) error {
		live, measured, err := s.store.CountPlannedRosterForCareExit(ctx, studentIDs, after)
		stats.Add(measured)
		if err != nil {
			return err
		}
		addCareExitCounts(counts, live)
		if len(restorable) == 0 {
			return nil
		}
		restored, measured, err := s.store.CountRestorableRosterForCareExit(ctx, studentIDs, after, restorable)
		stats.Add(measured)
		addCareExitCounts(counts, restored)
		return err
	})
	return counts, err
}

// CountRunningEnrollmentsForCareExit measures the bookings a care exit ending
// on validUntil (exclusive) takes away against the same baseline. A booking
// the earlier exit capped counts with its previous end; one it deleted counts
// only while it has not been re-created.
func (s *Service) CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string, restorable []domain.CareExitEnrollmentRemoval) (counts map[int64]int, err error) {
	counts = map[int64]int{}
	if len(studentIDs) == 0 {
		return counts, nil
	}
	capped, deleted := splitCareExitEnrollmentRemovals(restorable)
	err = s.run("count_running_enrollments_for_care_exit", func(stats *domain.OperationStats) error {
		live, measured, err := s.store.CountRunningEnrollmentsForCareExit(ctx, studentIDs, validUntil, capped)
		stats.Add(measured)
		if err != nil {
			return err
		}
		addCareExitCounts(counts, live)
		if len(deleted) == 0 {
			return nil
		}
		restored, measured, err := s.store.CountRestorableEnrollmentsForCareExit(ctx, studentIDs, validUntil, deleted)
		stats.Add(measured)
		addCareExitCounts(counts, restored)
		return err
	})
	return counts, err
}

func splitCareExitEnrollmentRemovals(removals []domain.CareExitEnrollmentRemoval) (capped, deleted []domain.CareExitEnrollmentRemoval) {
	for _, removal := range removals {
		if removal.WasDeleted {
			deleted = append(deleted, removal)
		} else {
			capped = append(capped, removal)
		}
	}
	return capped, deleted
}

func addCareExitCounts(total, part map[int64]int) {
	for studentID, count := range part {
		total[studentID] += count
	}
}
