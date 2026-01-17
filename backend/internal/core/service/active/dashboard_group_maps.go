package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	educationModels "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// dashboardGroupData holds group-related mappings
type dashboardGroupData struct {
	educationGroupsMap  map[int64]bool
	educationGroupRooms map[int64]bool  // room IDs that belong to educational groups
	studentHomeRoomMap  map[int64]int64 // studentID -> home room ID
}

// buildEducationGroupMaps creates group-related lookup structures
func (s *service) buildEducationGroupMaps(ctx context.Context, activeGroups []*active.Group, allEducationGroups []*educationModels.Group, studentsWithGroups []*userModels.Student) (*dashboardGroupData, error) {
	data := &dashboardGroupData{
		educationGroupsMap:  make(map[int64]bool),
		educationGroupRooms: make(map[int64]bool),
		studentHomeRoomMap:  make(map[int64]int64),
	}

	// Load education groups for active groups
	s.loadEducationGroupsForActive(ctx, activeGroups, data)

	// Build education group rooms set
	buildEducationGroupRoomsSet(allEducationGroups, data)

	// Build student home room map
	buildStudentHomeRoomMap(studentsWithGroups, allEducationGroups, data)

	return data, nil
}

// loadEducationGroupsForActive loads and marks education groups that have active sessions
func (s *service) loadEducationGroupsForActive(ctx context.Context, activeGroups []*active.Group, data *dashboardGroupData) {
	groupIDs := collectGroupIDs(activeGroups)
	if len(groupIDs) == 0 {
		return
	}

	eduGroupsMap, err := s.educationGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return
	}

	for id := range eduGroupsMap {
		data.educationGroupsMap[id] = true
	}
}

// collectGroupIDs extracts group IDs from active groups
func collectGroupIDs(activeGroups []*active.Group) []int64 {
	groupIDs := make([]int64, 0, len(activeGroups))
	for _, group := range activeGroups {
		groupIDs = append(groupIDs, group.GroupID)
	}
	return groupIDs
}

// buildEducationGroupRoomsSet populates the set of room IDs belonging to education groups
func buildEducationGroupRoomsSet(allEducationGroups []*educationModels.Group, data *dashboardGroupData) {
	for _, eduGroup := range allEducationGroups {
		if eduGroup.RoomID != nil && *eduGroup.RoomID > 0 {
			data.educationGroupRooms[*eduGroup.RoomID] = true
		}
	}
}

// buildStudentHomeRoomMap creates a mapping of student IDs to their home room IDs
func buildStudentHomeRoomMap(studentsWithGroups []*userModels.Student, allEducationGroups []*educationModels.Group, data *dashboardGroupData) {
	// Pre-build group ID to room ID lookup for O(1) access
	groupToRoom := buildGroupToRoomLookup(allEducationGroups)

	for _, student := range studentsWithGroups {
		if student.GroupID == nil {
			continue
		}
		if roomID, ok := groupToRoom[*student.GroupID]; ok {
			data.studentHomeRoomMap[student.ID] = roomID
		}
	}
}

// buildGroupToRoomLookup creates a map from group ID to room ID
func buildGroupToRoomLookup(allEducationGroups []*educationModels.Group) map[int64]int64 {
	lookup := make(map[int64]int64)
	for _, eduGroup := range allEducationGroups {
		if eduGroup.RoomID != nil {
			lookup[eduGroup.ID] = *eduGroup.RoomID
		}
	}
	return lookup
}
