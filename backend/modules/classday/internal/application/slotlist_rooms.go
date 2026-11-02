package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// slotRoomNames resolves only selectable occurrences, including unselected
// options. Missing rooms retain an empty name (the "Ohne Raum" bucket).
func (s *service) slotRoomNames(ctx context.Context, instances []*slotInstance, kind classday.ListKind) (map[int64]string, error) {
	ids := make([]int64, 0, len(instances))
	seen := make(map[int64]struct{}, len(instances))
	for _, inst := range instances {
		if !selectableSlot(inst, kind) || inst.RoomID == 0 {
			continue
		}
		if _, ok := seen[inst.RoomID]; !ok {
			seen[inst.RoomID] = struct{}{}
			ids = append(ids, inst.RoomID)
		}
	}
	names := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	rooms, err := s.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("lookup room %d: %w", ids[0], err)
	}
	for _, room := range rooms {
		names[room.ID] = room.Name
	}
	return names, nil
}
