package active

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/models/active"
)

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
		activeSupervisors := make([]*active.GroupSupervisor, 0, len(group.Supervisors))
		for _, supervisor := range group.Supervisors {
			if supervisor.IsActive() {
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
			ID:   group.Room.ID,
			Name: group.Room.Name,
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
		response.ActiveGroupName = displayGroupPrefix + strconv.FormatInt(visit.ActiveGroup.GroupID, 10)
	}

	return response
}

// newSupervisorResponse converts a group supervisor model to a response object
func newSupervisorResponse(supervisor *active.GroupSupervisor) SupervisorResponse {
	response := SupervisorResponse{
		ID:            supervisor.ID,
		StaffID:       supervisor.StaffID,
		ActiveGroupID: supervisor.GroupID,
		StartTime:     supervisor.StartDate,
		EndTime:       supervisor.EndDate,
		IsActive:      supervisor.IsActive(),
		CreatedAt:     supervisor.CreatedAt,
		UpdatedAt:     supervisor.UpdatedAt,
	}

	// Add related information if available
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		response.StaffName = supervisor.Staff.Person.GetFullName()
	}
	if supervisor.ActiveGroup != nil {
		response.ActiveGroupName = displayGroupPrefix + strconv.FormatInt(supervisor.ActiveGroup.GroupID, 10)
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
		IsActive:    group.IsActive(),
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
		response.GroupName = displayGroupPrefix + strconv.FormatInt(mapping.ActiveGroup.GroupID, 10)
	}
	if mapping.CombinedGroup != nil {
		response.CombinedName = "Combined Group #" + strconv.FormatInt(mapping.CombinedGroup.ID, 10)
	}

	return response
}
