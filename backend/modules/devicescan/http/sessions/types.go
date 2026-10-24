package sessions

import (
	"errors"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// SessionStartRequest represents a request to start an activity session
type SessionStartRequest struct {
	ActivityID    int64   `json:"activity_id"`
	RoomID        *int64  `json:"room_id,omitempty"`        // Optional: Override the activity's planned room
	SupervisorIDs []int64 `json:"supervisor_ids,omitempty"` // Multiple supervisors support
	Force         bool    `json:"force,omitempty"`
}

// Bind implements render.Binder interface for SessionStartRequest
func (req *SessionStartRequest) Bind(_ *http.Request) error {
	// Validate request
	if req.ActivityID <= 0 {
		return errors.New("activity_id is required")
	}

	return nil
}

// SupervisorInfo represents information about a supervisor
type SupervisorInfo = devicescan.SupervisorInfo

// SessionStartResponse represents the response when starting an activity session.
// ActivityID is nullable (WP-B6): an API client that triggers a spontaneous
// session still receives a response, but with `activity_id: null`. The
// existing IoT start flow always carries a template id, so `null` never
// appears for today's NFC path.
type SessionStartResponse = devicescan.SessionStartResponse

// ConflictInfoResponse represents conflict information for API responses
type ConflictInfoResponse = devicescan.ConflictInfoResponse

// SessionTimeoutResponse represents the result of processing a session timeout.
// ActivityID is nullable (WP-B6): a timed-out spontaneous session has no parent
// template. Serialized as `null` on the wire (no omitempty) so clients must
// handle both shapes explicitly.
type SessionTimeoutResponse = devicescan.SessionTimeoutResponse

// SessionActivityRequest represents a session activity update request
type SessionActivityRequest struct {
	ActivityType string    `json:"activity_type"` // "rfid_scan", "button_press", "ui_interaction"
	Timestamp    time.Time `json:"timestamp"`
}

// Bind validates the session activity request
func (req *SessionActivityRequest) Bind(_ *http.Request) error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.ActivityType, validation.Required, validation.In("rfid_scan", "button_press", "ui_interaction")),
	); err != nil {
		return err
	}

	// Set timestamp to now if not provided
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	return nil
}

// TimeoutValidationRequest represents a timeout validation request
type TimeoutValidationRequest struct {
	TimeoutMinutes int       `json:"timeout_minutes"`
	LastActivity   time.Time `json:"last_activity"`
}

// Bind validates the timeout validation request
func (req *TimeoutValidationRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.TimeoutMinutes, validation.Required, validation.Min(1), validation.Max(480)),
		validation.Field(&req.LastActivity, validation.Required),
	)
}

// SessionTimeoutInfoResponse provides comprehensive timeout information.
// ActivityID is nullable per WP-B6 — see SessionTimeoutResponse above.
type SessionTimeoutInfoResponse = devicescan.SessionTimeoutInfoResponse

// SessionCurrentResponse represents the current session information
type SessionCurrentResponse = devicescan.SessionCurrentResponse

// UpdateSupervisorsRequest represents a request to update supervisors for an active session
type UpdateSupervisorsRequest struct {
	SupervisorIDs []int64 `json:"supervisor_ids"`
}

// Bind validates the update supervisors request
func (req *UpdateSupervisorsRequest) Bind(_ *http.Request) error {
	if len(req.SupervisorIDs) == 0 {
		return errors.New("at least one supervisor is required")
	}
	return nil
}

// UpdateSupervisorsResponse represents the response when updating supervisors
type UpdateSupervisorsResponse = devicescan.UpdateSupervisorsResponse
