package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *Service) FindPlannedSupervisor(ctx context.Context, id int64) (result timetable.PlannedSupervisor, err error) {
	if id <= 0 {
		return timetable.PlannedSupervisor{}, s.reject("find_planned_supervisor", timetable.ErrInvalidPlannedSupervisorQuery)
	}

	err = s.run(ctx, "find_planned_supervisor", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindPlannedSupervisor(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrPlannedSupervisorNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListPlannedSupervisors(ctx context.Context, filter timetable.PlannedSupervisorFilter) (result []timetable.PlannedSupervisor, err error) {
	if hasInvalidID(filter.GroupIDs) || (filter.StaffID != nil && *filter.StaffID <= 0) {
		return nil, s.reject("list_planned_supervisors", timetable.ErrInvalidPlannedSupervisorQuery)
	}

	err = s.run(ctx, "list_planned_supervisors", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedSupervisors(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) (result []timetable.PlannedSupervisionBlocker, err error) {
	if staffID <= 0 || tenantID <= 0 {
		return nil, s.reject("list_planned_supervision_blockers", timetable.ErrInvalidPlannedSupervisorQuery)
	}
	contextTenant, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if contextTenant.Int64() != tenantID {
		return nil, errors.New("tenant context does not match requested tenant")
	}
	err = s.run(ctx, "list_planned_supervision_blockers", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedSupervisionBlockers(ctx, staffID)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreatePlannedSupervisor(ctx context.Context, fields timetable.PlannedSupervisorInput) (result timetable.PlannedSupervisor, err error) {
	if !normalizePlannedSupervisor(&fields) {
		return timetable.PlannedSupervisor{}, s.reject("create_planned_supervisor", timetable.ErrInvalidPlannedSupervisor)
	}

	if fields.ValidFrom == "" {
		fields.ValidFrom = s.today()
	}
	err = s.runWrite(ctx, "create_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockStaffAssignment(txCtx, fields.StaffID); err != nil {
			return err
		}
		value, queryStats, createErr := s.store.CreatePlannedSupervisor(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdatePlannedSupervisor(ctx context.Context, id int64, fields timetable.PlannedSupervisorInput) (result timetable.PlannedSupervisor, err error) {
	if id <= 0 || !normalizePlannedSupervisor(&fields) {
		return timetable.PlannedSupervisor{}, s.reject("update_planned_supervisor", timetable.ErrInvalidPlannedSupervisor)
	}

	if fields.ValidFrom == "" {
		fields.ValidFrom = s.today()
	}
	err = s.runWrite(ctx, "update_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockStaffAssignment(txCtx, fields.StaffID); err != nil {
			return err
		}
		value, found, queryStats, updateErr := s.store.UpdatePlannedSupervisor(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrPlannedSupervisorNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeletePlannedSupervisor(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_planned_supervisor", timetable.ErrInvalidPlannedSupervisor)
	}

	return s.runWrite(ctx, "delete_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePlannedSupervisor(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("set_primary_planned_supervisor", timetable.ErrInvalidPlannedSupervisor)
	}

	return s.runWrite(ctx, "set_primary_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.SetPrimaryPlannedSupervisor(txCtx, id)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return timetable.ErrPlannedSupervisorNotFound
		}
		return nil
	})
}

func (s *Service) LockPlannedSupervisors(ctx context.Context) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("timetable: planned supervisor locks require a transaction")
	}
	return s.observeOperation(ctx, "lock_planned_supervisors", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockPlannedSupervisors(ctx)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (result int64, err error) {
	if staffID <= 0 {
		return 0, s.reject("delete_planned_supervisors_by_staff", timetable.ErrInvalidPlannedSupervisor)
	}

	err = s.runWrite(ctx, "delete_planned_supervisors_by_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeletePlannedSupervisorsByStaff(txCtx, staffID)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

func (s *Service) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (result int64, err error) {
	if groupID <= 0 || !timetable.ValidDate(validUntil) {
		return 0, s.reject("cap_active_planned_supervisors", timetable.ErrInvalidPlannedSupervisor)
	}

	err = s.runWrite(ctx, "cap_active_planned_supervisors", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, capErr := s.store.CapActivePlannedSupervisors(txCtx, groupID, validUntil)
		stats.Add(queryStats)
		result = rows
		return capErr
	})
	return result, err
}

func (s *Service) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) error {
	if id <= 0 || !timetable.ValidDate(validUntil) {
		return s.reject("set_planned_supervisor_valid_until", timetable.ErrInvalidPlannedSupervisor)
	}

	return s.runWrite(ctx, "set_planned_supervisor_valid_until", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.SetPlannedSupervisorValidUntil(txCtx, id, validUntil)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return timetable.ErrPlannedSupervisorNotFound
		}
		return nil
	})
}

func (s *Service) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	if groupID <= 0 || (periodID != nil && *periodID <= 0) || !timetable.ValidDate(validUntil) {
		return s.reject("close_open_planned_supervisors", timetable.ErrInvalidPlannedSupervisor)
	}

	return s.runWrite(ctx, "close_open_planned_supervisors", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.CloseOpenPlannedSupervisors(txCtx, groupID, periodID, validUntil)
		stats.Add(queryStats)
		return err
	})
}
