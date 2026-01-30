package sessions

import (
	"errors"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
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
type SupervisorInfo struct {
	StaffID     int64  `json:"staff_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// SessionStartResponse represents the response when starting an activity session
type SessionStartResponse struct {
	ActiveGroupID int64                 `json:"active_group_id"`
	ActivityID    int64                 `json:"activity_id"`
	DeviceID      int64                 `json:"device_id"`
	StartTime     time.Time             `json:"start_time"`
	ConflictInfo  *ConflictInfoResponse `json:"conflict_info,omitempty"`
	Supervisors   []SupervisorInfo      `json:"supervisors,omitempty"`
	Status        string                `json:"status"`
	Message       string                `json:"message"`
}

// ConflictInfoResponse represents conflict information for API responses
type ConflictInfoResponse struct {
	HasConflict       bool   `json:"has_conflict"`
	ConflictingDevice *int64 `json:"conflicting_device,omitempty"`
	ConflictMessage   string `json:"conflict_message"`
	CanOverride       bool   `json:"can_override"`
}

// SessionTimeoutResponse represents the result of processing a session timeout
type SessionTimeoutResponse struct {
	SessionID          int64     `json:"session_id"`
	ActivityID         int64     `json:"activity_id"`
	StudentsCheckedOut int       `json:"students_checked_out"`
	TimeoutAt          time.Time `json:"timeout_at"`
	Status             string    `json:"status"`
	Message            string    `json:"message"`
}

// SessionTimeoutConfig represents timeout configuration for devices
type SessionTimeoutConfig struct {
	TimeoutMinutes       int `json:"timeout_minutes"`
	WarningMinutes       int `json:"warning_minutes"`
	CheckIntervalSeconds int `json:"check_interval_seconds"`
}

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

// SessionTimeoutInfoResponse provides comprehensive timeout information
type SessionTimeoutInfoResponse struct {
	SessionID               int64     `json:"session_id"`
	ActivityID              int64     `json:"activity_id"`
	StartTime               time.Time `json:"start_time"`
	LastActivity            time.Time `json:"last_activity"`
	TimeoutMinutes          int       `json:"timeout_minutes"`
	InactivitySeconds       int       `json:"inactivity_seconds"`
	TimeUntilTimeoutSeconds int       `json:"time_until_timeout_seconds"`
	IsTimedOut              bool      `json:"is_timed_out"`
	ActiveStudentCount      int       `json:"active_student_count"`
}

// SessionCurrentResponse represents the current session information
type SessionCurrentResponse struct {
	ActiveGroupID  *int64           `json:"active_group_id,omitempty"`
	ActivityID     *int64           `json:"activity_id,omitempty"`
	ActivityName   *string          `json:"activity_name,omitempty"`
	RoomID         *int64           `json:"room_id,omitempty"`
	RoomName       *string          `json:"room_name,omitempty"`
	DeviceID       int64            `json:"device_id"`
	StartTime      *time.Time       `json:"start_time,omitempty"`
	Duration       *string          `json:"duration,omitempty"`
	IsActive       bool             `json:"is_active"`
	ActiveStudents *int             `json:"active_students,omitempty"`
	Supervisors    []SupervisorInfo `json:"supervisors,omitempty"`
}

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
type UpdateSupervisorsResponse struct {
	ActiveGroupID int64            `json:"active_group_id"`
	Supervisors   []SupervisorInfo `json:"supervisors"`
	Status        string           `json:"status"`
	Message       string           `json:"message"`
}
