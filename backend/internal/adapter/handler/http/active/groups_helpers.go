package active

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
)

// parseActiveGroupQueryParams parses query parameters for active groups.
func (rs *Resource) parseActiveGroupQueryParams(r *http.Request) *base.QueryOptions {
	queryOptions := base.NewQueryOptions()

	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		if isActive {
			queryOptions.Filter.IsNull("end_time")
		} else {
			queryOptions.Filter.IsNotNull("end_time")
		}
	}

	return queryOptions
}

// loadActiveGroupRelations loads rooms and supervisors for active groups.
func (rs *Resource) loadActiveGroupRelations(r *http.Request, groups []*active.Group) {
	roomMap := rs.loadRoomsMap(r, groups)
	supervisorMap := rs.loadActiveSupervisorsMap(r, groups)

	for _, group := range groups {
		if supervisors, ok := supervisorMap[group.ID]; ok {
			group.Supervisors = supervisors
		}
		if room, ok := roomMap[group.RoomID]; ok {
			group.Room = room
		}
	}
}

// loadRoomsMap loads rooms and returns a map of room ID to room.
func (rs *Resource) loadRoomsMap(r *http.Request, groups []*active.Group) map[int64]*facilities.Room {
	roomIDs := rs.collectUniqueRoomIDs(groups)
	if len(roomIDs) == 0 {
		return make(map[int64]*facilities.Room)
	}

	roomMap, err := rs.FacilitiesService.GetRoomsByIDs(r.Context(), roomIDs)
	if err != nil {
		logger.Logger.WithError(err).Warn("Error loading rooms")
		return make(map[int64]*facilities.Room)
	}

	return roomMap
}

// collectUniqueRoomIDs collects unique room IDs from groups.
func (rs *Resource) collectUniqueRoomIDs(groups []*active.Group) []int64 {
	roomIDs := make([]int64, 0, len(groups))
	roomIDMap := make(map[int64]bool)

	for _, group := range groups {
		if group.RoomID > 0 && !roomIDMap[group.RoomID] {
			roomIDs = append(roomIDs, group.RoomID)
			roomIDMap[group.RoomID] = true
		}
	}

	return roomIDs
}

// loadActiveSupervisorsMap loads supervisors and returns a map of group ID to active supervisors.
func (rs *Resource) loadActiveSupervisorsMap(r *http.Request, groups []*active.Group) map[int64][]*active.GroupSupervisor {
	groupIDs := make([]int64, len(groups))
	for i, group := range groups {
		groupIDs[i] = group.ID
	}

	allSupervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupIDs(r.Context(), groupIDs)
	if err != nil {
		logger.Logger.WithError(err).Warn("Error loading supervisors")
		return make(map[int64][]*active.GroupSupervisor)
	}

	activeSupervisors := make([]*active.GroupSupervisor, 0, len(allSupervisors))
	for _, supervisor := range allSupervisors {
		if supervisor.IsActive() {
			activeSupervisors = append(activeSupervisors, supervisor)
		}
	}

	supervisorMap := make(map[int64][]*active.GroupSupervisor)
	for _, supervisor := range activeSupervisors {
		supervisorMap[supervisor.GroupID] = append(supervisorMap[supervisor.GroupID], supervisor)
	}

	return supervisorMap
}

// extractStaffFromRequest extracts staff information from JWT claims.
func (rs *Resource) extractStaffFromRequest(w http.ResponseWriter, r *http.Request) (*users.Staff, error) {
	claims := jwt.ClaimsFromCtx(r.Context())

	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(claims.ID))
	if err != nil || person == nil {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("account not found")))
		return nil, errors.New("account not found")
	}

	staff, err := rs.PersonService.GetStaffByPersonID(r.Context(), person.ID)
	if err != nil || staff == nil {
		common.RenderError(w, r, ErrorForbidden(errors.New("user is not a staff member")))
		return nil, errors.New("user is not a staff member")
	}

	return staff, nil
}

// verifyStaffSupervisionAccess verifies staff has permission to view an active group.
func (rs *Resource) verifyStaffSupervisionAccess(w http.ResponseWriter, r *http.Request, staffID int64, activeGroupID int64) error {
	supervisions, err := rs.ActiveService.GetStaffActiveSupervisions(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return err
	}

	hasPermission := false
	for _, supervision := range supervisions {
		if supervision.GroupID == activeGroupID {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		common.RenderError(w, r, ErrorForbidden(errors.New("not authorized to view this group")))
		return errors.New("not authorized")
	}

	_, err = rs.ActiveService.GetActiveGroup(r.Context(), activeGroupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return err
	}

	return nil
}

// convertVisitsToDisplayResponses converts service visit data to API responses.
func (rs *Resource) convertVisitsToDisplayResponses(visits []activeSvc.VisitWithDisplayData) []VisitWithDisplayDataResponse {
	responses := make([]VisitWithDisplayDataResponse, 0, len(visits))
	for _, v := range visits {
		studentName := v.FirstName + " " + v.LastName
		responses = append(responses, VisitWithDisplayDataResponse{
			ID:            v.VisitID,
			StudentID:     v.StudentID,
			ActiveGroupID: v.ActiveGroupID,
			CheckInTime:   v.EntryTime,
			CheckOutTime:  v.ExitTime,
			IsActive:      v.ExitTime == nil,
			StudentName:   studentName,
			SchoolClass:   v.SchoolClass,
			GroupName:     v.OGSGroupName,
			CreatedAt:     v.CreatedAt,
			UpdatedAt:     v.UpdatedAt,
		})
	}
	return responses
}
