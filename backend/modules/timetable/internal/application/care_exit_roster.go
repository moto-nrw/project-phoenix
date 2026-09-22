package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// plannedCareExitRoster resolves the participants a care exit after `after`
// takes away: the students' rows on not cancelled instances after that day,
// minus the ones Student Presence already observed or whose block ended.
// A manually set future status is still a plan and stays in the set.
func (s *Service) plannedCareExitRoster(ctx context.Context, studentIDs []int64, after string, stats *domain.OperationStats) ([]domain.CareExitRosterRow, error) {
	candidates, measured, err := s.store.ListPlannedRosterForCareExit(ctx, studentIDs, after)
	stats.Add(measured)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	participantIDs := make([]int64, 0, len(candidates))
	instanceIDs := make([]int64, 0, len(candidates))
	for _, row := range candidates {
		participantIDs = append(participantIDs, row.ParticipantID)
		instanceIDs = append(instanceIDs, row.InstanceID)
	}
	observed, err := s.observedParticipantIDs(ctx, participantIDs)
	if err != nil {
		return nil, err
	}
	completed, err := s.completedInstanceIDs(ctx, sortedUniqueIDs(instanceIDs))
	if err != nil {
		return nil, err
	}
	result := make([]domain.CareExitRosterRow, 0, len(candidates))
	for _, row := range candidates {
		if observed[row.ParticipantID] || completed[row.InstanceID] {
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

func careExitParticipantIDs(rows []domain.CareExitRosterRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ParticipantID)
	}
	return ids
}

func (s *Service) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	if len(studentIDs) == 0 {
		return nil
	}
	return s.runWrite(ctx, "lock_planned_roster_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, err := s.plannedCareExitRoster(txCtx, studentIDs, after, stats)
		if err != nil {
			return err
		}
		measured, err := s.store.LockInstanceStudentsByID(txCtx, careExitParticipantIDs(rows))
		stats.Add(measured)
		return err
	})
}

// PreviewPlannedRosterForCareExit lists what RemovePlannedRosterForCareExit
// would remove, so the caller can read the attendance those participants
// carry before the rows are gone.
func (s *Service) PreviewPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) (rows []domain.CareExitRosterRow, err error) {
	rows = []domain.CareExitRosterRow{}
	if len(studentIDs) == 0 {
		return rows, nil
	}
	err = s.run("preview_planned_roster_for_care_exit", func(stats *domain.OperationStats) error {
		values, err := s.plannedCareExitRoster(ctx, studentIDs, after, stats)
		if err != nil {
			return err
		}
		rows = append(rows, values...)
		return nil
	})
	return rows, err
}

func (s *Service) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) (rows []domain.CareExitRosterRow, err error) {
	rows = []domain.CareExitRosterRow{}
	if len(studentIDs) == 0 {
		return rows, nil
	}
	err = s.runWrite(ctx, "remove_planned_roster_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		planned, err := s.plannedCareExitRoster(txCtx, studentIDs, after, stats)
		if err != nil || len(planned) == 0 {
			return err
		}
		values, measured, writeErr := s.store.RemovePlannedRosterForCareExit(txCtx, careExitParticipantIDs(planned))
		stats.Add(measured)
		rows = values
		return writeErr
	})
	return rows, err
}

// RestoreRosterForCareExit puts removed participants back onto rosters that
// have not ended or been cancelled and returns the restored rows with their
// new participant ids.
func (s *Service) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []domain.CareExitRosterRow) (restored []domain.CareExitRosterRow, err error) {
	restored = []domain.CareExitRosterRow{}
	if len(studentIDs) == 0 || len(rows) == 0 {
		return restored, nil
	}
	err = s.runWrite(ctx, "restore_roster_for_care_exit", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		values, err := s.validateCareExitRosterRooms(txCtx, rows, stats)
		if err != nil {
			return err
		}
		instanceIDs := make([]int64, 0, len(values))
		for _, row := range values {
			instanceIDs = append(instanceIDs, row.InstanceID)
		}
		completed, err := s.completedInstanceIDs(txCtx, sortedUniqueIDs(instanceIDs))
		if err != nil {
			return err
		}
		excluded := make([]int64, 0, len(completed))
		for id := range completed {
			excluded = append(excluded, id)
		}
		result, measured, err := s.store.RestoreRosterForCareExit(txCtx, studentIDs, values, sortedUniqueIDs(excluded))
		stats.Add(measured)
		restored = result
		return err
	})
	return restored, err
}

func (s *Service) validateCareExitRosterRooms(ctx context.Context, rows []domain.CareExitRosterRow, stats *domain.OperationStats) ([]domain.CareExitRosterRow, error) {
	var roomIDs []int64
	for _, row := range rows {
		if row.RoomID != nil {
			roomIDs = append(roomIDs, *row.RoomID)
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
	validRooms := validRosterRooms(rooms)
	result := make([]domain.CareExitRosterRow, 0, len(rows))
	for _, row := range rows {
		if row.RoomID != nil && validRooms[*row.RoomID] != row.TenantID {
			row.RoomID = nil
		}
		result = append(result, row)
	}
	return result, nil
}
