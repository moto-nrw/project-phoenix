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

// getSessionDisplayNames extracts activity and room names from session for display
func (rs *Resource) getSessionDisplayNames(session *active.Group) (string, string) {
	activityName := "Unknown Activity"
	roomName := "Unknown Room"

	if session.ActualGroup != nil && session.ActualGroup.Name != "" {
		activityName = session.ActualGroup.Name
	}

	if session.Room != nil && session.Room.Name != "" {
		roomName = session.Room.Name
	}

	return activityName, roomName
}

// extractSupervisorIDsAndCheckDuplicate extracts active supervisor IDs and checks if staff is already a supervisor
func (rs *Resource) extractSupervisorIDsAndCheckDuplicate(supervisorsGroup *active.Group, staffID int64, sessionID int64) ([]int64, bool) {
	supervisorIDs := make([]int64, 0)
	isDuplicate := false

	if supervisorsGroup.Supervisors != nil {
		for _, sup := range supervisorsGroup.Supervisors {
			if sup.StaffID == staffID && sup.EndDate == nil {
				isDuplicate = true
				log.Printf("[SUPERVISOR_AUTH] Supervisor %d already assigned to session %d (idempotent)", staffID, sessionID)
			}
			if sup.EndDate == nil {
				supervisorIDs = append(supervisorIDs, sup.StaffID)
			}
		}
	}

	return supervisorIDs, isDuplicate
}

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

	conflictInfo, conflictErr := rs.ActiveService.CheckActivityConflict(r.Context(), activityID, deviceID)
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
	} else {
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
		} else {
		}
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
