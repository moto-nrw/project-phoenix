package sessions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// startSession starts an activity session with proper validation and logging
func (rs *Resource) startSession(ctx context.Context, req *SessionStartRequest, deviceCtx *iot.Device) (*active.Group, error) {
	log.Printf("Session start request: ActivityID=%d, SupervisorIDs=%v, Force=%v", req.ActivityID, req.SupervisorIDs, req.Force)

	if len(req.SupervisorIDs) == 0 {
		log.Printf("No supervisor IDs provided in request")
		return nil, errors.New("at least one supervisor ID is required")
	}

	log.Printf("Using multi-supervisor methods with %d supervisors", len(req.SupervisorIDs))

	if req.Force {
		log.Printf("Calling ForceStartActivitySessionWithSupervisors with supervisors: %v", req.SupervisorIDs)
		return rs.ActiveService.ForceStartActivitySessionWithSupervisors(ctx, req.ActivityID, deviceCtx.ID, req.SupervisorIDs, req.RoomID)
	}

	log.Printf("Calling StartActivitySessionWithSupervisors with supervisors: %v", req.SupervisorIDs)
	return rs.ActiveService.StartActivitySessionWithSupervisors(ctx, req.ActivityID, deviceCtx.ID, req.SupervisorIDs, req.RoomID)
}

// handleSessionConflictError handles session conflict errors and returns true if error was handled
func (rs *Resource) handleSessionConflictError(w http.ResponseWriter, r *http.Request, err error, activityID, deviceID int64) bool {
	if !errors.Is(err, activeSvc.ErrSessionConflict) && !errors.Is(err, activeSvc.ErrDeviceAlreadyActive) {
		return false
	}

	conflictInfo, conflictErr := rs.ActiveService.CheckActivityConflict(r.Context(), deviceID)
	if conflictErr != nil || !conflictInfo.HasConflict {
		return false
	}

	response := SessionStartResponse{
		Status:  "conflict",
		Message: conflictInfo.ConflictMessage,
		ConflictInfo: &ConflictInfoResponse{
			HasConflict:     conflictInfo.HasConflict,
			ConflictMessage: conflictInfo.ConflictMessage,
			CanOverride:     conflictInfo.CanOverride,
		},
	}

	if conflictInfo.ConflictingDevice != nil {
		if deviceID, parseErr := strconv.ParseInt(*conflictInfo.ConflictingDevice, 10, 64); parseErr == nil {
			response.ConflictInfo.ConflictingDevice = &deviceID
		}
	}

	common.Respond(w, r, http.StatusConflict, response, "Session conflict detected")
	return true
}

// buildSessionStartResponse builds the success response with supervisor information
func (rs *Resource) buildSessionStartResponse(ctx context.Context, activeGroup *active.Group, deviceCtx *iot.Device) SessionStartResponse {
	response := SessionStartResponse{
		ActiveGroupID: activeGroup.ID,
		ActivityID:    activeGroup.GroupID,
		DeviceID:      deviceCtx.ID,
		StartTime:     activeGroup.StartTime,
		Status:        "started",
		Message:       "Activity session started successfully",
	}

	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)

	if err == nil && len(supervisors) > 0 {
		response.Supervisors = rs.buildSupervisorInfos(ctx, supervisors)
	}

	return response
}

// buildSupervisorInfos builds supervisor information from supervisor list
func (rs *Resource) buildSupervisorInfos(ctx context.Context, supervisors []*active.GroupSupervisor) []SupervisorInfo {
	supervisorInfos := make([]SupervisorInfo, 0, len(supervisors))
	staffRepo := rs.UsersService.StaffRepository()

	for _, supervisor := range supervisors {
		staff, err := staffRepo.FindWithPerson(ctx, supervisor.StaffID)

		if err == nil && staff != nil && staff.Person != nil {
			supervisorInfos = append(supervisorInfos, SupervisorInfo{
				StaffID:     supervisor.StaffID,
				FirstName:   staff.Person.FirstName,
				LastName:    staff.Person.LastName,
				DisplayName: fmt.Sprintf("%s %s", staff.Person.FirstName, staff.Person.LastName),
				Role:        supervisor.Role,
			})
		}
		// Skip supervisors where staff or person data is missing
	}

	return supervisorInfos
}

// filterActiveSupervisors filters for active supervisors (no end date)
func (rs *Resource) filterActiveSupervisors(supervisors []*active.GroupSupervisor) []*active.GroupSupervisor {
	active := make([]*active.GroupSupervisor, 0, len(supervisors))
	for _, sup := range supervisors {
		if sup.EndDate == nil && sup.StaffID > 0 {
			active = append(active, sup)
		}
	}
	return active
}

// countActiveStudents counts visits without an exit time (active students in session)
func countActiveStudents(visits []*active.Visit) int {
	count := 0
	for _, visit := range visits {
		if visit.ExitTime == nil {
			count++
		}
	}
	return count
}
