package sessions

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
)

func (rs *Resource) startActivitySession(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	req := &SessionStartRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err, "")
		return
	}
	response, err := rs.Lifecycle.StartSession(r.Context(), devicescan.StartSessionCommand{ActivityID: req.ActivityID, RoomID: req.RoomID, SupervisorIDs: req.SupervisorIDs, Force: req.Force})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	if response.Status == "conflict" {
		rs.runtime.Success(w, r, http.StatusConflict, response, "Session conflict detected")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Activity session started successfully")
}

func (rs *Resource) endActivitySession(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	current, err := rs.Lifecycle.SessionToEnd(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	// The workflow joins the request transaction. Every failure must roll it
	// back, including 4xx outcomes, so no partial close or event can commit.
	ended, err := rs.SessionEnd.EndSession(r.Context(), current.ID)
	if err != nil {
		rs.runtime.MarkRollback(r.Context())
		status, message := http.StatusInternalServerError, "failed to end activity session"
		switch {
		case errors.Is(err, sessionend.ErrSessionNotFound):
			status, message = http.StatusNotFound, ""
		case errors.Is(err, sessionend.ErrSessionAlreadyEnded):
			status, message = http.StatusBadRequest, ""
		}
		rs.runtime.Failure(w, r, status, err, message)
		return
	}
	response := map[string]interface{}{
		"active_group_id": current.ID, "activity_id": current.ActivityID, "device_id": current.DeviceID,
		"ended_at": ended.EndedAt, "duration": ended.EndedAt.Sub(current.StartTime).String(),
		"status": "ended", "message": "Activity session ended successfully",
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Activity session ended successfully")
}

func (rs *Resource) getCurrentSession(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	response, err := rs.Lifecycle.CurrentSession(r.Context())
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	message := "Current session retrieved successfully"
	if !response.IsActive {
		message = "No active session"
	}
	rs.runtime.Success(w, r, http.StatusOK, response, message)
}

func (rs *Resource) updateSessionSupervisors(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	sessionID, err := rs.runtime.ParseID(r, "sessionId")
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("invalid session ID"), "")
		return
	}
	req := &UpdateSupervisorsRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err, "")
		return
	}
	response, err := rs.Lifecycle.ReplaceSessionSupervisors(r.Context(), sessionID, req.SupervisorIDs)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Supervisors updated successfully")
}

func (rs *Resource) checkSessionConflict(w http.ResponseWriter, r *http.Request) {
	if !rs.requireDevice(w, r) {
		return
	}
	req := &SessionStartRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err, "")
		return
	}
	response, err := rs.Lifecycle.CheckSessionConflict(r.Context(), req.ActivityID)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	status, message := http.StatusOK, "No conflicts detected"
	if response.HasConflict {
		status, message = http.StatusConflict, "Conflict detected"
	}
	rs.runtime.Success(w, r, status, response, message)
}
