package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// The planned exits of #4809 live on the membership row. Reads go through
// the staff queries; every write locks the live member first and decides on
// the locked row. Every call joins the caller's transaction.

// SavePlannedExit enters or moves the planned exit of a live member. A moved
// day clears the scheduler's progress, so the reminders and attempts start
// over; re-entering the same day keeps them.
func (s *Service) SavePlannedExit(ctx context.Context, plan schoolmembership.PlanExit) (result schoolmembership.PlannedExit, err error) {
	err = s.runWrite(ctx, "save_planned_exit", func(txCtx context.Context, stats *domain.OperationStats) error {
		staff, found, lockStats, err := s.store.FindStaff(txCtx, plan.StaffID, "UPDATE", false)
		stats.Add(lockStats)
		if err = liveStaff(found, err); err != nil {
			return err
		}
		keepProgress := staff.PlannedExit != nil && staff.PlannedExit.LastWorkingDay == plan.LastWorkingDay
		saved, saveStats, err := s.store.SavePlannedExit(txCtx, plan, keepProgress)
		stats.Add(saveStats)
		if err == nil {
			result = *saved.PlannedExit
		}
		return err
	})
	return result, err
}

// DeletePlannedExit withdraws the planned exit of a live member.
func (s *Service) DeletePlannedExit(ctx context.Context, staffID int64) error {
	return s.runWrite(ctx, "delete_planned_exit", func(txCtx context.Context, stats *domain.OperationStats) error {
		staff, found, lockStats, err := s.store.FindStaff(txCtx, staffID, "UPDATE", false)
		stats.Add(lockStats)
		if err = liveStaff(found, err); err != nil {
			return err
		}
		if staff.PlannedExit == nil {
			return schoolmembership.ErrPlannedExitNotFound
		}
		clearStats, err := s.store.ClearPlannedExit(txCtx, staffID)
		stats.Add(clearStats)
		return err
	})
}

// RecordPlannedExitProgress stores the scheduler's progress for the day it
// worked on. A planned exit the Leitung moved or withdrew since is left
// alone: the progress belonged to the old day.
func (s *Service) RecordPlannedExitProgress(ctx context.Context, progress schoolmembership.PlannedExitProgress) error {
	return s.runWrite(ctx, "record_planned_exit_progress", func(txCtx context.Context, stats *domain.OperationStats) error {
		staff, found, lockStats, err := s.store.FindStaff(txCtx, progress.StaffID, "UPDATE", false)
		stats.Add(lockStats)
		if err = liveStaff(found, err); err != nil {
			return err
		}
		if staff.PlannedExit == nil || staff.PlannedExit.LastWorkingDay != progress.LastWorkingDay {
			return schoolmembership.ErrPlannedExitNotFound
		}
		recordStats, err := s.store.RecordPlannedExitProgress(txCtx, progress)
		stats.Add(recordStats)
		return err
	})
}

// liveStaff maps a missing live member onto ErrStaffNotFound.
func liveStaff(found bool, err error) error {
	if err == nil && !found {
		return schoolmembership.ErrStaffNotFound
	}
	return err
}

// NewPlannedExitsFacade binds the planned exits onto the service: reads over
// the staff queries, writes over the locked commands above.
func NewPlannedExitsFacade(service *Service) *schoolmembership.PlannedExits {
	return schoolmembership.NewPlannedExits(schoolmembership.PlannedExitStore{
		FindForMutation: func(ctx context.Context, staffID int64) (schoolmembership.PlannedExit, error) {
			staff, err := service.FindStaffForMutation(ctx, staffID)
			if err != nil {
				return schoolmembership.PlannedExit{}, err
			}
			if staff.PlannedExit == nil {
				return schoolmembership.PlannedExit{}, schoolmembership.ErrPlannedExitNotFound
			}
			return *staff.PlannedExit, nil
		},
		List: func(ctx context.Context) ([]schoolmembership.PlannedExit, error) {
			staff, err := service.ListStaff(ctx, schoolmembership.StaffFilter{PlannedExitOnly: true, MembershipOnly: true})
			result := make([]schoolmembership.PlannedExit, 0, len(staff))
			for _, member := range staff {
				result = append(result, *member.PlannedExit)
			}
			return result, err
		},
		Save: service.SavePlannedExit, Delete: service.DeletePlannedExit, Record: service.RecordPlannedExitProgress,
	})
}
