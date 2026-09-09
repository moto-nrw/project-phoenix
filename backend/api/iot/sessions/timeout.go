package sessions

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

// processSessionTimeout handles device timeout notification
func (rs *Resource) processSessionTimeout(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}

	response, err := rs.Lifecycle.TimeoutSession(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, "Session timeout processed successfully")
}

// updateSessionActivity handles activity updates for timeout tracking
func (rs *Resource) updateSessionActivity(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}

	var req SessionActivityRequest
	if err := render.Bind(r, &req); err != nil {
		rs.renderError(w, r, err)
		return
	}

	sessionID, err := rs.Lifecycle.TouchSession(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}

	response := map[string]interface{}{
		"session_id":    sessionID,
		"activity_type": req.ActivityType,
		"updated_at":    time.Now(),
		"last_activity": time.Now(),
	}

	rs.runtime.Success(w, r, http.StatusOK, response, "Session activity updated")
}

// validateSessionTimeout validates if a timeout request is legitimate
func (rs *Resource) validateSessionTimeout(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}

	var req TimeoutValidationRequest
	if err := render.Bind(r, &req); err != nil {
		rs.renderError(w, r, err)
		return
	}

	// Validate the timeout request
	if err := rs.Lifecycle.ValidateTimeout(r.Context(), req.TimeoutMinutes); err != nil {
		rs.renderError(w, r, err)
		return
	}

	response := map[string]interface{}{
		"valid":           true,
		"timeout_minutes": req.TimeoutMinutes,
		"last_activity":   req.LastActivity,
		"validated_at":    time.Now(),
	}

	rs.runtime.Success(w, r, http.StatusOK, response, "Timeout validation successful")
}

// getSessionTimeoutInfo provides comprehensive timeout information
func (rs *Resource) getSessionTimeoutInfo(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}

	response, err := rs.Lifecycle.SessionTimeoutInfo(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, "Session timeout information retrieved")
}
