package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	if len(studentIDs) == 0 {
		return nil
	}
	return s.runWrite(ctx, "lock_planned_roster_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		measured, err := s.store.LockPlannedRosterForCareExit(txCtx, studentIDs, after)
		stats.Add(measured)
		return err
	})
}
func (s *Service) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) (rows []domain.CareExitRosterRow, err error) {
	rows = []domain.CareExitRosterRow{}
	if len(studentIDs) == 0 {
		return rows, nil
	}
	err = s.runWrite(ctx, "remove_planned_roster_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		values, measured, writeErr := s.store.RemovePlannedRosterForCareExit(txCtx, studentIDs, after)
		stats.Add(measured)
		rows = values
		return writeErr
	})
	return rows, err
}
func (s *Service) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []domain.CareExitRosterRow) (count int, err error) {
	if len(studentIDs) == 0 || len(rows) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "restore_roster_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		values, err := s.validateCareExitRosterReferences(txCtx, rows, stats)
		if err != nil {
			return err
		}
		restored, measured, err := s.store.RestoreRosterForCareExit(txCtx, studentIDs, values)
		stats.Add(measured)
		count = int(restored)
		return err
	})
	return count, err
}

func (s *Service) validateCareExitRosterReferences(ctx context.Context, rows []domain.CareExitRosterRow, stats *domain.OperationStats) ([]domain.CareExitRosterRow, error) {
	var roomIDs, pickupIDs, statusIDs []int64
	for _, row := range rows {
		if row.RoomID != nil {
			roomIDs = append(roomIDs, *row.RoomID)
		}
		if row.PickupExceptionID != nil {
			pickupIDs = append(pickupIDs, *row.PickupExceptionID)
		}
		if row.StudentStatusDayID != nil {
			statusIDs = append(statusIDs, *row.StudentStatusDayID)
		}
	}
	rooms := []domain.RoomRef{}
	if len(roomIDs) > 0 {
		values, measured, err := s.rooms.LockRoomsByID(ctx, sortedUniqueIDs(roomIDs))
		stats.Add(measured)
		if err != nil {
			return nil, err
		}
		rooms = values
	}
	pickups := map[int64]bool{}
	if len(pickupIDs) > 0 {
		values, err := s.carePlan.ListPickupExceptions(ctx, domain.PickupExceptionFilter{IDs: sortedUniqueIDs(pickupIDs)})
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			pickups[value.ID] = true
		}
	}
	statuses := map[int64]bool{}
	if len(statusIDs) > 0 {
		values, err := s.carePlan.ListStudentStatusDays(ctx, domain.StudentStatusDayFilter{IDs: sortedUniqueIDs(statusIDs)})
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			statuses[value.ID] = true
		}
	}
	validRooms := validRosterRooms(rooms)
	result := make([]domain.CareExitRosterRow, 0, len(rows))
	for _, row := range rows {
		if row.RoomID != nil && validRooms[*row.RoomID] != row.TenantID {
			row.RoomID = nil
		}
		if row.PickupExceptionID != nil && !pickups[*row.PickupExceptionID] {
			row.PickupExceptionID = nil
		}
		if row.StudentStatusDayID != nil && !statuses[*row.StudentStatusDayID] {
			row.StudentStatusDayID = nil
		}
		result = append(result, row)
	}
	return result, nil
}
