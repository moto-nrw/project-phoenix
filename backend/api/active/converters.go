package active

import (
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
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

	// Add counts if available
	if group.Visits != nil {
		response.VisitCount = len(group.Visits)
	}
	if group.Supervisors != nil {
		// Only expose currently active supervisors
		now := time.Now()
		activeSupervisors := make([]*active.GroupSupervisor, 0, len(group.Supervisors))
		for _, supervisor := range group.Supervisors {
			if activeService.IsSupervisorActive(supervisor, now) {
				activeSupervisors = append(activeSupervisors, supervisor)
			}
		}

		response.SupervisorCount = len(activeSupervisors)
		// Add supervisor details
		response.Supervisors = make([]GroupSupervisorSimple, 0, len(activeSupervisors))
		for _, supervisor := range activeSupervisors {
			response.Supervisors = append(response.Supervisors, GroupSupervisorSimple{
				StaffID: supervisor.StaffID,
				Role:    supervisor.Role,
			})
		}
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

// newVisitResponse converts a visit model to a response object
func newVisitResponse(visit *active.Visit) VisitResponse {
	response := VisitResponse{
		ID:            visit.ID,
		StudentID:     visit.StudentID,
		ActiveGroupID: visit.ActiveGroupID,
		CheckInTime:   visit.EntryTime,
		CheckOutTime:  visit.ExitTime,
		IsActive:      visit.IsActive(),
		CreatedAt:     visit.CreatedAt,
		UpdatedAt:     visit.UpdatedAt,
	}

	// Add related information if available
	if visit.Student != nil && visit.Student.Person != nil {
		response.StudentName = visit.Student.Person.GetFullName()
	}
	if visit.ActiveGroup != nil {
		response.ActiveGroupName = activeGroupDisplayName(visit.ActiveGroup)
	}

	return response
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
func newCombinedGroupResponse(group *active.CombinedGroup) CombinedGroupResponse {
	response := CombinedGroupResponse{
		ID:          group.ID,
		Name:        "Combined Group #" + strconv.FormatInt(group.ID, 10), // Using ID as name since the model doesn't have name
		Description: "",                                                   // Using empty description since the model doesn't have description
		RoomID:      0,                                                    // Using default value since the model doesn't have roomID
		StartTime:   group.StartTime,
		EndTime:     group.EndTime,
		IsActive:    activeService.IsCombinedGroupActive(group, time.Now()),
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}

	// Add group count if available
	if group.ActiveGroups != nil {
		response.GroupCount = len(group.ActiveGroups)
	}

	return response
}

// newGroupMappingResponse converts a group mapping model to a response object
func newGroupMappingResponse(mapping *active.GroupMapping) GroupMappingResponse {
	response := GroupMappingResponse{
		ID:              mapping.ID,
		ActiveGroupID:   mapping.ActiveGroupID,
		CombinedGroupID: mapping.ActiveCombinedGroupID,
	}

	// Add related information if available
	if mapping.ActiveGroup != nil {
		response.GroupName = activeGroupDisplayName(mapping.ActiveGroup)
	}
	if mapping.CombinedGroup != nil {
		response.CombinedName = "Combined Group #" + strconv.FormatInt(mapping.CombinedGroup.ID, 10)
	}

	return response
}
