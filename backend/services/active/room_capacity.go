package active

import "context"

func (s *service) ensureRoomCapacity(ctx context.Context, roomID int64, incoming int) error {
	if incoming < 0 || s.RoomRepo == nil {
		return nil
	}

	room, err := s.RoomRepo.FindByIDForUpdate(ctx, roomID)
	if err != nil || room == nil {
		return &ActiveError{Op: "EnsureRoomCapacity", Err: ErrDatabaseOperation}
	}
	if room.Capacity == nil || *room.Capacity <= 0 {
		return nil
	}

	currentOccupancy, err := s.VisitRepo.CountActiveByRoomID(ctx, roomID)
	if err != nil {
		return &ActiveError{Op: "EnsureRoomCapacity", Err: ErrDatabaseOperation}
	}
	if currentOccupancy+incoming <= *room.Capacity {
		return nil
	}

	return &RoomCapacityError{
		RoomID:           room.ID,
		RoomName:         room.Name,
		CurrentOccupancy: currentOccupancy,
		MaxCapacity:      *room.Capacity,
	}
}
