package sessions

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
)

// startActivitySession handles starting an activity session on a device
func (rs *Resource) startActivitySession(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse request
	req := &SessionStartRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Additional debug - check what we got after binding
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"activity_id":    req.ActivityID,
			"supervisor_ids": req.SupervisorIDs,
			"force":          req.Force,
		}).Debug("Session start request after binding")
	}

	// Start the activity session
	activeGroup, err := rs.startSession(r.Context(), req, deviceCtx)
	if err != nil {
		// Handle conflict errors with detailed response
		if rs.handleSessionConflictError(w, r, err, req.ActivityID, deviceCtx.ID) {
			return
		}
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build success response with supervisor information
	response := rs.buildSessionStartResponse(r.Context(), activeGroup, deviceCtx)

	common.Respond(w, r, http.StatusOK, response, "Activity session started successfully")
}

// endActivitySession handles ending the current activity session on a device
func (rs *Resource) endActivitySession(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get current session for this device
	currentSession, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID)
	if err != nil {
		if errors.Is(err, activeSvc.ErrNoActiveSession) {
			iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("no active session to end")))
			return
		}
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// End the session
	if err := rs.ActiveService.EndActivitySession(r.Context(), currentSession.ID); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	response := map[string]interface{}{
		"active_group_id": currentSession.ID,
		"activity_id":     currentSession.GroupID,
		"device_id":       deviceCtx.ID,
		"ended_at":        time.Now(),
		"duration":        time.Since(currentSession.StartTime).String(),
		"status":          "ended",
		"message":         "Activity session ended successfully",
	}

	common.Respond(w, r, http.StatusOK, response, "Activity session ended successfully")
}

// getCurrentSession handles getting the current session information for a device
// This endpoint also keeps the session alive (updates last_activity and device.last_seen)
func (rs *Resource) getCurrentSession(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Update device last seen time (best-effort - don't fail request if this fails)
	// This keeps the device marked as "online" while it's actively polling session/current
	if err := rs.IoTService.PingDevice(r.Context(), deviceCtx.DeviceID); err != nil {
		if logger.Logger != nil {
			logger.Logger.WithField("device_id", deviceCtx.DeviceID).WithError(err).Warn("Failed to update device last seen")
		}
	}

	// Get current session for this device
	currentSession, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID)

	response := SessionCurrentResponse{
		DeviceID: deviceCtx.ID,
		IsActive: false,
	}

	if err != nil {
		if errors.Is(err, activeSvc.ErrNoActiveSession) {
			// No active session - return empty response with IsActive: false
			common.Respond(w, r, http.StatusOK, response, "No active session")
			return
		}
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Update session activity to keep the session alive
	// This allows devices polling this endpoint to prevent session timeout
	if updateErr := rs.ActiveService.UpdateSessionActivity(r.Context(), currentSession.ID); updateErr != nil {
		// Log but don't fail - the main purpose is to return session info
		if logger.Logger != nil {
			logger.Logger.WithField("session_id", currentSession.ID).WithError(updateErr).Warn("Failed to update session activity")
		}
	}

	// Session found - populate response
	response.IsActive = true
	response.ActiveGroupID = &currentSession.ID
	response.ActivityID = &currentSession.GroupID
	response.RoomID = &currentSession.RoomID
	response.StartTime = &currentSession.StartTime
	duration := time.Since(currentSession.StartTime).String()
	response.Duration = &duration

	// Add activity name if available
	if currentSession.ActualGroup != nil {
		response.ActivityName = &currentSession.ActualGroup.Name
	}

	// Add room name if available
	if currentSession.Room != nil {
		response.RoomName = &currentSession.Room.Name
	}

	// Get active student count for this session
	activeVisits, err := rs.ActiveService.FindVisitsByActiveGroupID(r.Context(), currentSession.ID)
	if err != nil {
		// Log error but don't fail the request - student count is optional info
		if logger.Logger != nil {
			logger.Logger.WithField("session_id", currentSession.ID).WithError(err).Warn("Failed to get active student count")
		}
	} else {
		activeCount := countActiveStudents(activeVisits)
		response.ActiveStudents = &activeCount
	}

	common.Respond(w, r, http.StatusOK, response, "Current session retrieved successfully")
}

// updateSessionSupervisors handles updating the supervisors for an active session
func (rs *Resource) updateSessionSupervisors(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get session ID from URL parameters
	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	// Parse request
	req := &UpdateSupervisorsRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Update supervisors
	updatedGroup, err := rs.ActiveService.UpdateActiveGroupSupervisors(r.Context(), sessionID, req.SupervisorIDs)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Filter active supervisors and build response details
	activeSupervisors := rs.filterActiveSupervisors(updatedGroup.Supervisors)
	supervisors := rs.buildSupervisorInfos(r.Context(), activeSupervisors)

	// Build response
	response := UpdateSupervisorsResponse{
		ActiveGroupID: updatedGroup.ID,
		Supervisors:   supervisors,
		Status:        "success",
		Message:       "Supervisors updated successfully",
	}

	common.Respond(w, r, http.StatusOK, response, "Supervisors updated successfully")
}

// checkSessionConflict handles checking for conflicts before starting a session
func (rs *Resource) checkSessionConflict(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse request
	req := &SessionStartRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Check for conflicts
	conflictInfo, err := rs.ActiveService.CheckActivityConflict(r.Context(), req.ActivityID, deviceCtx.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	response := ConflictInfoResponse{
		HasConflict:     conflictInfo.HasConflict,
		ConflictMessage: conflictInfo.ConflictMessage,
		CanOverride:     conflictInfo.CanOverride,
	}

	if conflictInfo.ConflictingDevice != nil {
		if deviceID, parseErr := strconv.ParseInt(*conflictInfo.ConflictingDevice, 10, 64); parseErr == nil {
			response.ConflictingDevice = &deviceID
		}
	}

	statusCode := http.StatusOK
	message := "No conflicts detected"
	if conflictInfo.HasConflict {
		statusCode = http.StatusConflict
		message = "Conflict detected"
	}

	common.Respond(w, r, statusCode, response, message)
}
