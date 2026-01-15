package sessions

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
)

// processSessionTimeout handles device timeout notification
func (rs *Resource) processSessionTimeout(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	// Process timeout via device ID
	result, err := rs.ActiveService.ProcessSessionTimeout(r.Context(), deviceCtx.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	response := SessionTimeoutResponse{
		SessionID:          result.SessionID,
		ActivityID:         result.ActivityID,
		StudentsCheckedOut: result.StudentsCheckedOut,
		TimeoutAt:          result.TimeoutAt,
		Status:             "completed",
		Message:            fmt.Sprintf("Session ended due to timeout. %d students checked out.", result.StudentsCheckedOut),
	}

	common.Respond(w, r, http.StatusOK, response, "Session timeout processed successfully")
}

// getSessionTimeoutConfig returns timeout configuration for the requesting device
func (rs *Resource) getSessionTimeoutConfig(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	settings, err := rs.ConfigService.GetDeviceTimeoutSettings(r.Context(), deviceCtx.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	config := SessionTimeoutConfig{
		TimeoutMinutes:       settings.GetEffectiveTimeoutMinutes(),
		WarningMinutes:       settings.WarningThresholdMinutes,
		CheckIntervalSeconds: settings.CheckIntervalSeconds,
	}

	common.Respond(w, r, http.StatusOK, config, "Timeout configuration retrieved")
}

// updateSessionActivity handles activity updates for timeout tracking
func (rs *Resource) updateSessionActivity(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	var req SessionActivityRequest
	if err := render.Bind(r, &req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Get current session for this device
	session, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Update session activity
	if err := rs.ActiveService.UpdateSessionActivity(r.Context(), session.ID); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	response := map[string]interface{}{
		"session_id":    session.ID,
		"activity_type": req.ActivityType,
		"updated_at":    time.Now(),
		"last_activity": time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Session activity updated")
}

// validateSessionTimeout validates if a timeout request is legitimate
func (rs *Resource) validateSessionTimeout(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	var req TimeoutValidationRequest
	if err := render.Bind(r, &req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Validate the timeout request
	if err := rs.ActiveService.ValidateSessionTimeout(r.Context(), deviceCtx.ID, req.TimeoutMinutes); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	response := map[string]interface{}{
		"valid":           true,
		"timeout_minutes": req.TimeoutMinutes,
		"last_activity":   req.LastActivity,
		"validated_at":    time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Timeout validation successful")
}

// getSessionTimeoutInfo provides comprehensive timeout information
func (rs *Resource) getSessionTimeoutInfo(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	info, err := rs.ActiveService.GetSessionTimeoutInfo(r.Context(), deviceCtx.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	response := SessionTimeoutInfoResponse{
		SessionID:               info.SessionID,
		ActivityID:              info.ActivityID,
		StartTime:               info.StartTime,
		LastActivity:            info.LastActivity,
		TimeoutMinutes:          info.TimeoutMinutes,
		InactivitySeconds:       int(info.InactivityDuration.Seconds()),
		TimeUntilTimeoutSeconds: int(info.TimeUntilTimeout.Seconds()),
		IsTimedOut:              info.IsTimedOut,
		ActiveStudentCount:      info.ActiveStudentCount,
	}

	common.Respond(w, r, http.StatusOK, response, "Session timeout information retrieved")
}
