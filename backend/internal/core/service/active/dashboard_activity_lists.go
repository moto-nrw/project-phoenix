package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	facilityModels "github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
)

// buildRecentActivity builds the recent activity list
func (s *service) buildRecentActivity(ctx context.Context, activeGroups []*active.Group, roomData *dashboardRoomData) []RecentActivity {
	recentActivity := []RecentActivity{}

	for i, group := range activeGroups {
		if i >= 3 { // Limit to 3 recent activities
			break
		}

		if time.Since(group.StartTime) >= 30*time.Minute || !group.IsActive() {
			continue
		}

		groupName := s.resolveGroupName(ctx, group.GroupID)
		roomName := s.resolveRoomName(group.RoomID, roomData.roomByID)

		// Count unique students
		visitCount := 0
		if studentSet, ok := roomData.roomStudentsMap[group.RoomID]; ok {
			visitCount = len(studentSet)
		}

		activity := RecentActivity{
			Type:      "group_start",
			GroupName: groupName,
			RoomName:  roomName,
			Count:     visitCount,
			Timestamp: group.StartTime,
		}
		recentActivity = append(recentActivity, activity)
	}

	return recentActivity
}

// buildCurrentActivities builds the current activities list
func (s *service) buildCurrentActivities(ctx context.Context, activeGroups []*active.Group, roomData *dashboardRoomData) []CurrentActivity {
	currentActivities := []CurrentActivity{}

	activityGroups, err := s.activityGroupRepo.List(ctx, nil)
	if err != nil {
		return currentActivities
	}

	for i, actGroup := range activityGroups {
		if i >= 2 { // Limit to 2 current activities
			break
		}

		hasActiveSession, participantCount := s.findActiveSessionForActivity(actGroup.ID, activeGroups, roomData)
		if !hasActiveSession {
			continue
		}

		categoryName := "Sonstiges"
		if actGroup.Category != nil {
			categoryName = actGroup.Category.Name
		}

		status := s.determineActivityStatus(participantCount, actGroup.MaxParticipants)

		activity := CurrentActivity{
			Name:         actGroup.Name,
			Category:     categoryName,
			Participants: participantCount,
			MaxCapacity:  actGroup.MaxParticipants,
			Status:       status,
		}
		currentActivities = append(currentActivities, activity)
	}

	return currentActivities
}

// findActiveSessionForActivity checks if an activity has an active session and returns participant count
func (s *service) findActiveSessionForActivity(activityID int64, activeGroups []*active.Group, roomData *dashboardRoomData) (bool, int) {
	for _, group := range activeGroups {
		if group.IsActive() && group.GroupID == activityID {
			participantCount := 0
			if studentSet, ok := roomData.roomStudentsMap[group.RoomID]; ok {
				participantCount = len(studentSet)
			}
			return true, participantCount
		}
	}
	return false, 0
}

// determineActivityStatus returns the status string based on capacity
func (s *service) determineActivityStatus(participants, maxCapacity int) string {
	if participants >= maxCapacity {
		return "full"
	}
	if participants > int(float64(maxCapacity)*0.8) {
		return "ending_soon"
	}
	return "active"
}

// buildActiveGroupsSummary builds the active groups summary list
func (s *service) buildActiveGroupsSummary(ctx context.Context, activeGroups []*active.Group, roomData *dashboardRoomData) []ActiveGroupInfo {
	summary := []ActiveGroupInfo{}

	for i, group := range activeGroups {
		if i >= 2 || !group.IsActive() { // Limit to 2 groups
			break
		}

		groupName, groupType := s.resolveGroupNameAndType(ctx, group.GroupID)
		location := s.resolveRoomName(group.RoomID, roomData.roomByID)

		studentCount := 0
		if studentSet, ok := roomData.roomStudentsMap[group.RoomID]; ok {
			studentCount = len(studentSet)
		}

		groupInfo := ActiveGroupInfo{
			Name:         groupName,
			Type:         groupType,
			StudentCount: studentCount,
			Location:     location,
			Status:       "active",
		}
		summary = append(summary, groupInfo)
	}

	return summary
}

// resolveGroupName gets the display name for a group
func (s *service) resolveGroupName(ctx context.Context, groupID int64) string {
	if actGroup, err := s.activityGroupRepo.FindByID(ctx, groupID); err == nil && actGroup != nil {
		return actGroup.Name
	}
	if eduGroup, err := s.educationGroupRepo.FindByID(ctx, groupID); err == nil && eduGroup != nil {
		return eduGroup.Name
	}
	return fmt.Sprintf("Gruppe %d", groupID)
}

// resolveGroupNameAndType gets the display name and type for a group
func (s *service) resolveGroupNameAndType(ctx context.Context, groupID int64) (string, string) {
	if eduGroup, err := s.educationGroupRepo.FindByID(ctx, groupID); err == nil && eduGroup != nil {
		return eduGroup.Name, "ogs_group"
	}
	return fmt.Sprintf("Gruppe %d", groupID), "activity"
}

// resolveRoomName gets the display name for a room
func (s *service) resolveRoomName(roomID int64, roomByID map[int64]*facilityModels.Room) string {
	if room, ok := roomByID[roomID]; ok {
		return room.Name
	}
	return fmt.Sprintf("Raum %d", roomID)
}
