package active

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
)

// locationMetrics holds calculated location-based metrics
type locationMetrics struct {
	studentsOnPlayground  int
	studentsInGroupRooms  int
	studentsInHomeRoom    int
	studentsInIndoorRooms int
}

// processActiveGroups calculates metrics from active groups
func (s *service) processActiveGroups(activeGroups []*active.Group, visitsByGroupID map[int64][]*active.Visit, groupData *dashboardGroupData, roomData *dashboardRoomData) (int, int, map[int64]struct{}) {
	ogsGroupsCount := 0
	uniqueStudentsInRoomsOverall := make(map[int64]struct{})

	for _, group := range activeGroups {
		roomData.occupiedRooms[group.RoomID] = true

		// Initialize room student set if not exists
		if roomData.roomStudentsMap[group.RoomID] == nil {
			roomData.roomStudentsMap[group.RoomID] = make(map[int64]struct{})
		}

		// Count unique students for this group
		if groupVisits, ok := visitsByGroupID[group.ID]; ok {
			for _, visit := range groupVisits {
				roomData.roomStudentsMap[group.RoomID][visit.StudentID] = struct{}{}
				uniqueStudentsInRoomsOverall[visit.StudentID] = struct{}{}
			}
		}

		// Check if this is an OGS group
		if groupData.educationGroupsMap[group.GroupID] {
			ogsGroupsCount++
		}
	}

	return len(activeGroups), ogsGroupsCount, uniqueStudentsInRoomsOverall
}

// calculateLocationMetrics calculates student location-based metrics
func (s *service) calculateLocationMetrics(roomData *dashboardRoomData, groupData *dashboardGroupData, activeVisits []*active.Visit, activeGroups []*active.Group) *locationMetrics {
	metrics := &locationMetrics{}

	// Process each room's student set
	for roomID, studentSet := range roomData.roomStudentsMap {
		s.processRoomForLocationMetrics(roomID, studentSet, roomData, groupData, metrics)
	}

	// Calculate students in indoor rooms (excluding playground)
	metrics.studentsInIndoorRooms = s.countStudentsInIndoorRooms(activeVisits, activeGroups, roomData)

	return metrics
}

// processRoomForLocationMetrics updates metrics based on a single room's students
func (s *service) processRoomForLocationMetrics(roomID int64, studentSet map[int64]struct{}, roomData *dashboardRoomData, groupData *dashboardGroupData, metrics *locationMetrics) {
	room, ok := roomData.roomByID[roomID]
	if !ok {
		return
	}

	uniqueStudentCount := len(studentSet)

	if isPlaygroundRoom(room) {
		metrics.studentsOnPlayground += uniqueStudentCount
	}

	if !groupData.educationGroupRooms[roomID] {
		return
	}

	metrics.studentsInGroupRooms += uniqueStudentCount
	metrics.studentsInHomeRoom += countStudentsInHomeRoom(studentSet, roomID, groupData.studentHomeRoomMap)
}

// countStudentsInHomeRoom counts how many students in the set are in their home room
func countStudentsInHomeRoom(studentSet map[int64]struct{}, roomID int64, studentHomeRoomMap map[int64]int64) int {
	count := 0
	for studentID := range studentSet {
		if homeRoomID, ok := studentHomeRoomMap[studentID]; ok && homeRoomID == roomID {
			count++
		}
	}
	return count
}

// countStudentsInIndoorRooms counts unique students in rooms excluding playground areas
func (s *service) countStudentsInIndoorRooms(activeVisits []*active.Visit, activeGroups []*active.Group, roomData *dashboardRoomData) int {
	// Build group ID to room lookup for O(1) access
	groupToRoom := buildActiveGroupRoomLookup(activeGroups, roomData)

	uniqueStudentsInRooms := make(map[int64]struct{})
	for _, visit := range activeVisits {
		if !visit.IsActive() {
			continue
		}
		if _, isIndoor := groupToRoom[visit.ActiveGroupID]; isIndoor {
			uniqueStudentsInRooms[visit.StudentID] = struct{}{}
		}
	}

	return len(uniqueStudentsInRooms)
}
