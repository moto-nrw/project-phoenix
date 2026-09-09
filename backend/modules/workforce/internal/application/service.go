// Package application orchestrates the Workforce work-time capability: it
// validates template and schedule input, runs every multi-table write in one
// unit of work, and records one observation per call.
package application

import (
	"slices"
	"time"

	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// Service is the single entry point behind the Workforce facade.
type Service struct {
	store               ports.Store
	transaction         ports.Transaction
	assignments         ports.StaffAssignments
	lockStaffAssignment func(context.Context, int64) error
	clock               ports.Clock
	observe             ports.Observer
}

func New(
	store ports.Store,
	transaction ports.Transaction,
	assignments ports.StaffAssignments,
	lockStaffAssignment func(context.Context, int64) error,
	clock ports.Clock,
	observe ports.Observer,
) *Service {
	if store == nil || transaction == nil || assignments == nil || lockStaffAssignment == nil || clock == nil || observe == nil {
		panic("workforce application: all dependencies are required")
	}
	return &Service{store: store, transaction: transaction, assignments: assignments, lockStaffAssignment: lockStaffAssignment, clock: clock, observe: observe}
}

// --- work-time templates ---

func (s *Service) lockAssignmentStaff(ctx context.Context, ids []int64) error {
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		if err := s.lockStaffAssignment(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListWorkTimeModels(ctx context.Context) (result []domain.WorkTimeModel, err error) {
	err = s.run("list_work_time_models", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListWorkTimeModels(ctx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindWorkTimeModel(ctx context.Context, id int64) (result domain.WorkTimeModel, err error) {
	err = s.run("find_work_time_model", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindWorkTimeModel(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkTimeModelNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListWorkTimeModelsByIDs(ctx context.Context, ids []int64) (result []domain.WorkTimeModel, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	err = s.run("list_work_time_models_by_ids", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListWorkTimeModelsByIDs(ctx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateWorkTimeModel(ctx context.Context, fields domain.WorkTimeModelFields) (result domain.WorkTimeModel, err error) {
	if validationErr := domain.ValidateWorkTimeModelFields(fields); validationErr != nil {
		return domain.WorkTimeModel{}, validationErr
	}
	err = s.run("create_work_time_model", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateWorkTimeModel(txCtx, fields)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// UpdateWorkTimeModel replaces the template and, in the same unit of work,
// rewrites the schedule snapshots of every staff member bound to it. The two
// writes are inseparable: a template whose entries moved but whose assignees
// still carry the old Soll is a wrong time account, so they must commit or
// roll back together.
func (s *Service) UpdateWorkTimeModel(ctx context.Context, id int64, fields domain.WorkTimeModelFields) (result domain.WorkTimeModel, err error) {
	if validationErr := domain.ValidateWorkTimeModelFields(fields); validationErr != nil {
		return domain.WorkTimeModel{}, validationErr
	}
	err = s.run("update_work_time_model", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateWorkTimeModel(txCtx, id, fields)
			stats.Add(writeStats)
			if err != nil {
				return err
			}
			if !found {
				return domain.ErrWorkTimeModelNotFound
			}
			return s.refreshAssignedSchedules(txCtx, result, stats)
		})
	})
	return result, err
}

// DeleteWorkTimeModel removes a template that no staff member is bound to.
// The assignment check and the delete share the write transaction, so a
// concurrent assignment cannot slip past the guard and leave a staff member
// pointing at a deleted template.
func (s *Service) DeleteWorkTimeModel(ctx context.Context, id int64) error {
	return s.run("delete_work_time_model", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			assigned, err := s.assignments.AssignedStaffIDs(txCtx, id)
			if err != nil {
				return err
			}
			if len(assigned) > 0 {
				return domain.ErrWorkTimeModelAssigned
			}
			found, deleteStats, err := s.store.DeleteWorkTimeModel(txCtx, id)
			stats.Add(deleteStats)
			if err != nil {
				return err
			}
			if !found {
				return domain.ErrWorkTimeModelNotFound
			}
			return nil
		})
	})
}

// refreshAssignedSchedules rewrites the schedule snapshots of every staff
// member bound to the template. Running versions are closed at today and the
// new ones carry the template's own anchor, so a template edit can never
// re-parity a week that has already been accounted for. It runs inside the
// caller's write transaction and must never be given its own.
func (s *Service) refreshAssignedSchedules(ctx context.Context, model domain.WorkTimeModel, stats *domain.OperationStats) error {
	staffIDs, err := s.assignments.AssignedStaffIDs(ctx, model.ID)
	if err != nil {
		return err
	}
	if len(staffIDs) == 0 {
		return nil
	}
	// Sorted so concurrent refreshes take the per-staff locks in the same
	// order and cannot deadlock against each other.
	slices.Sort(staffIDs)
	for _, staffID := range staffIDs {
		if err := s.transaction.LockStaffBalance(ctx, staffID); err != nil {
			return err
		}
	}

	today := s.clock.Today()
	closeStats, err := s.store.CloseStaffSchedules(ctx, staffIDs, today)
	stats.Add(closeStats)
	if err != nil {
		return err
	}

	// The staff rows belong to School Membership: the anchor is stamped
	// through that capability, never by a foreign join.
	if _, err := s.assignments.RebaseAnchor(ctx, model.ID, model.RotationAnchorDate); err != nil {
		return err
	}

	if len(model.Entries) == 0 {
		return nil
	}
	rows := make([]domain.StaffWorkSchedule, 0, len(staffIDs)*len(model.Entries))
	for _, staffID := range staffIDs {
		for _, entry := range model.Entries {
			rows = append(rows, domain.StaffWorkSchedule{
				StaffID:            staffID,
				WeekIndex:          entry.WeekIndex,
				RotationLength:     model.RotationLength,
				DayOfWeek:          entry.DayOfWeek,
				TargetMinutes:      entry.TargetMinutes,
				StartTime:          entry.StartTime,
				RotationAnchorDate: model.RotationAnchorDate,
				ValidFrom:          today,
			})
		}
	}
	insertStats, err := s.store.InsertStaffSchedules(ctx, rows)
	stats.Add(insertStats)
	return err
}

// --- staff schedules ---

// ReplaceStaffSchedule closes the staff member's running schedule versions at
// today and writes the given entries as the new current version. A zero anchor
// leaves the per-version anchor NULL: a one-week rotation has no parity.
func (s *Service) ReplaceStaffSchedule(ctx context.Context, staffID int64, entries []domain.StaffWorkScheduleFields, anchor string) error {
	if err := domain.ValidateDate(anchor, "rotation_anchor_date"); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := domain.ValidateStaffScheduleFields(entry); err != nil {
			return err
		}
	}
	return s.run("replace_staff_schedule", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if err := s.transaction.LockStaffBalance(txCtx, staffID); err != nil {
				return err
			}
			today := s.clock.Today()
			closeStats, err := s.store.CloseStaffSchedules(txCtx, []int64{staffID}, today)
			stats.Add(closeStats)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return nil
			}
			rows := make([]domain.StaffWorkSchedule, 0, len(entries))
			for _, entry := range entries {
				rows = append(rows, domain.StaffWorkSchedule{
					StaffID:            staffID,
					WeekIndex:          entry.WeekIndex,
					RotationLength:     entry.RotationLength,
					DayOfWeek:          entry.DayOfWeek,
					TargetMinutes:      entry.TargetMinutes,
					StartTime:          entry.StartTime,
					RotationAnchorDate: anchor,
					ValidFrom:          today,
				})
			}
			insertStats, err := s.store.InsertStaffSchedules(txCtx, rows)
			stats.Add(insertStats)
			return err
		})
	})
}

func (s *Service) CurrentStaffSchedule(ctx context.Context, staffID int64) (result []domain.StaffWorkSchedule, err error) {
	err = s.run("current_staff_schedule", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CurrentStaffSchedule(ctx, staffID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) StaffScheduleOn(ctx context.Context, staffID int64, date string) (result []domain.StaffWorkSchedule, err error) {
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("staff_schedule_on", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.StaffScheduleOn(ctx, staffID, date)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) (result []domain.StaffWorkSchedule, err error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return nil, validationErr
	}
	if validationErr := domain.ValidateDate(to, "to"); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("staff_schedules_in_range", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.StaffSchedulesInRange(ctx, staffIDs, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) HasStaffScheduleHistory(ctx context.Context, staffID int64) (result bool, err error) {
	err = s.run("has_staff_schedule_history", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.HasStaffScheduleHistory(ctx, staffID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) StaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (result map[int64]bool, err error) {
	if len(staffIDs) == 0 {
		return map[int64]bool{}, nil
	}
	err = s.run("staff_ids_with_schedule_history", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.StaffIDsWithScheduleHistory(ctx, staffIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// --- plumbing ---

func (s *Service) run(operation string, fn func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(&stats)
	if err != nil {
		stats.Rows = 0
	}
	return err
}
