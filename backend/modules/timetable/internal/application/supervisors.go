package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindPlannedSupervisor(ctx context.Context, id int64) (result domain.PlannedSupervisor, err error) {
	err = s.run("find_planned_supervisor", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindPlannedSupervisor(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrPlannedSupervisorNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListPlannedSupervisors(ctx context.Context, filter domain.PlannedSupervisorFilter) (result []domain.PlannedSupervisor, err error) {
	err = s.run("list_planned_supervisors", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedSupervisors(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListPlannedSupervisionBlockers(ctx context.Context, staffID int64) (result []domain.PlannedSupervisionBlocker, err error) {
	err = s.run("list_planned_supervision_blockers", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedSupervisionBlockers(ctx, staffID)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreatePlannedSupervisor(ctx context.Context, fields domain.PlannedSupervisorFields) (result domain.PlannedSupervisor, err error) {
	if fields.ValidFrom == "" {
		fields.ValidFrom = s.today()
	}
	err = s.runWrite(ctx, "create_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreatePlannedSupervisor(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdatePlannedSupervisor(ctx context.Context, id int64, fields domain.PlannedSupervisorFields) (result domain.PlannedSupervisor, err error) {
	if fields.ValidFrom == "" {
		fields.ValidFrom = s.today()
	}
	err = s.runWrite(ctx, "update_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdatePlannedSupervisor(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrPlannedSupervisorNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeletePlannedSupervisor(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePlannedSupervisor(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "set_primary_planned_supervisor", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.SetPrimaryPlannedSupervisor(txCtx, id)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPlannedSupervisorNotFound
		}
		return nil
	})
}

func (s *Service) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (result int64, err error) {
	err = s.runWrite(ctx, "delete_planned_supervisors_by_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeletePlannedSupervisorsByStaff(txCtx, staffID)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

func (s *Service) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (result int64, err error) {
	err = s.runWrite(ctx, "cap_active_planned_supervisors", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, capErr := s.store.CapActivePlannedSupervisors(txCtx, groupID, validUntil)
		stats.Add(queryStats)
		result = rows
		return capErr
	})
	return result, err
}

func (s *Service) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) error {
	return s.runWrite(ctx, "set_planned_supervisor_valid_until", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.SetPlannedSupervisorValidUntil(txCtx, id, validUntil)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPlannedSupervisorNotFound
		}
		return nil
	})
}

func (s *Service) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	return s.runWrite(ctx, "close_open_planned_supervisors", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.CloseOpenPlannedSupervisors(txCtx, groupID, periodID, validUntil)
		stats.Add(queryStats)
		return err
	})
}
