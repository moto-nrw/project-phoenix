package checkin

import (
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
)

// CheckinRequest represents a student check-in request from RFID devices
type CheckinRequest struct {
	StudentRFID string `json:"student_rfid"`
	Action      string `json:"action"` // "checkin" or "checkout"
	RoomID      *int64 `json:"room_id,omitempty"`
}

// CheckinResponse represents the response to a student check-in request
type CheckinResponse struct {
	StudentID   int64     `json:"student_id"`
	StudentName string    `json:"student_name"`
	Action      string    `json:"action"`
	VisitID     *int64    `json:"visit_id,omitempty"`
	RoomName    string    `json:"room_name,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
}

// Bind validates the checkin request
func (req *CheckinRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StudentRFID, validation.Required),
		// Note: Action field is ignored in logic but still required for API compatibility
		validation.Field(&req.Action, validation.Required, validation.In("checkin", "checkout")),
	)
}
