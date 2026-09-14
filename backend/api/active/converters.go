package active

import (
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// activeGroupDisplayName renders a display string for an active.Group. For
// template-backed sessions it emits "<prefix><template_id>"; for spontaneous
// sessions (no template, WP-B6) it emits "<prefix>spontaneous" so the name
// remains distinguishable and never collides with a real template id.
func activeGroupDisplayName(g *active.Group) string {
	if g == nil {
		return ""
	}
	if templateID, ok := g.TemplateID(); ok {
		return displayGroupPrefix + strconv.FormatInt(templateID, 10)
	}
	return displayGroupPrefix + "spontaneous"
}

// ===== Conversion Functions =====

// newActiveGroupResponse converts an active group model to a response object
func newActiveGroupResponse(group *active.Group) ActiveGroupResponse {
	response := ActiveGroupResponse{
		ID:        group.ID,
		GroupID:   group.GroupID,
		RoomID:    group.RoomID,
		StartTime: group.StartTime,
		EndTime:   group.EndTime,
		IsActive:  group.IsActive(),
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}

	if group.Supervisors != nil {
		response.Supervisors = newActiveSupervisorResponses(group.Supervisors)
		response.SupervisorCount = len(response.Supervisors)
	}

	// Add room info if available
	if group.Room != nil {
		response.Room = &RoomSimple{
			ID:    group.Room.ID,
			Name:  group.Room.Name,
			Color: group.Room.Color,
		}
	}

	return response
}

func newActiveSupervisorResponses(supervisors []*active.GroupSupervisor) []GroupSupervisorSimple {
	now := time.Now()
	responses := make([]GroupSupervisorSimple, 0, len(supervisors))
	for _, supervisor := range supervisors {
		if activeService.IsSupervisorActive(supervisor, now) {
			responses = append(responses, GroupSupervisorSimple{StaffID: supervisor.StaffID, Role: supervisor.Role})
		}
	}
	return responses
}

// newSupervisorResponse converts a group supervisor model to a response object
func newSupervisorResponse(supervisor *active.GroupSupervisor) SupervisorResponse {
	// The supervisor wire shape keeps start_time/end_time as ISO timestamps
	// (UTC midnight of the DATE), matching how the DATE columns serialized
	// before the timezone.Date migration.
	var endTime *time.Time
	if supervisor.EndDate != nil {
		t := supervisor.EndDate.UTCMidnight()
		endTime = &t
	}
	response := SupervisorResponse{
		ID:            supervisor.ID,
		StaffID:       supervisor.StaffID,
		ActiveGroupID: supervisor.GroupID,
		StartTime:     supervisor.StartDate.UTCMidnight(),
		EndTime:       endTime,
		IsActive:      activeService.IsSupervisorActive(supervisor, time.Now()),
		CreatedAt:     supervisor.CreatedAt,
		UpdatedAt:     supervisor.UpdatedAt,
	}

	// Add related information if available
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		response.StaffName = supervisor.Staff.Person.GetFullName()
	}
	if supervisor.ActiveGroup != nil {
		response.ActiveGroupName = activeGroupDisplayName(supervisor.ActiveGroup)
	}

	return response
}

// newCombinedGroupResponse converts a combined group model to a response object
func newCombinedGroupResponse(group *activeService.CombinedGroupDetails) CombinedGroupResponse {
	response := newPresenceCombinationResponse(group.CombinedGroup)
	if group.ActiveGroups != nil {
		response.GroupCount = len(group.ActiveGroups)
	}
	return response
}

func newPresenceLiveGroupResponse(group studentpresence.LiveGroup) ActiveGroupResponse {
	return ActiveGroupResponse{
		ID: group.ID, GroupID: group.ActivityGroupID, RoomID: group.RoomID,
		StartTime: group.StartTime, EndTime: group.EndTime, IsActive: group.IsOpen(),
		CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
	}
}

func newPresenceCombinationResponse(group studentpresence.CombinedGroup) CombinedGroupResponse {
	response := CombinedGroupResponse{
		ID:          group.ID,
		Name:        "Combined Group #" + strconv.FormatInt(group.ID, 10), // Using ID as name since the model doesn't have name
		Description: "",                                                   // Using empty description since the model doesn't have description
		RoomID:      0,                                                    // Using default value since the model doesn't have roomID
		StartTime:   group.StartTime,
		EndTime:     group.EndTime,
		IsActive:    group.EndTime == nil || time.Now().Before(*group.EndTime),
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}

	return response
}
