package active

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	facilityModels "github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
)

// dashboardRoomData holds room-related lookup maps
type dashboardRoomData struct {
	roomByID          map[int64]*facilityModels.Room
	roomCapacityTotal int
	occupiedRooms     map[int64]bool
	roomStudentsMap   map[int64]map[int64]struct{} // roomID -> set of unique student IDs
}

// buildRoomLookupMaps creates room-related lookup structures
func (s *service) buildRoomLookupMaps(allRooms []*facilityModels.Room) *dashboardRoomData {
	data := &dashboardRoomData{
		roomByID:        make(map[int64]*facilityModels.Room),
		occupiedRooms:   make(map[int64]bool),
		roomStudentsMap: make(map[int64]map[int64]struct{}),
	}

	for _, room := range allRooms {
		data.roomByID[room.ID] = room
		if room.Capacity != nil && *room.Capacity > 0 {
			data.roomCapacityTotal += *room.Capacity
		}
	}

	return data
}

// isPlaygroundRoom checks if a room is a playground/outdoor area
func isPlaygroundRoom(room *facilityModels.Room) bool {
	if room == nil || room.Category == nil {
		return false
	}
	switch *room.Category {
	case "Schulhof", "Playground", "school_yard":
		return true
	}
	return false
}

// buildActiveGroupRoomLookup creates a lookup of active group IDs to non-playground room status
func buildActiveGroupRoomLookup(activeGroups []*active.Group, roomData *dashboardRoomData) map[int64]bool {
	lookup := make(map[int64]bool)
	for _, group := range activeGroups {
		if !group.IsActive() {
			continue
		}
		room, ok := roomData.roomByID[group.RoomID]
		if ok && !isPlaygroundRoom(room) {
			lookup[group.ID] = true
		}
	}
	return lookup
}
