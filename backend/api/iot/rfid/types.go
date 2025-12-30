package rfid

import (
	"errors"
	"net/http"
)

// RFIDAssignmentRequest represents an RFID tag assignment request
type RFIDAssignmentRequest struct {
	RFIDTag string `json:"rfid_tag"`
}

// Bind validates the RFID assignment request
func (req *RFIDAssignmentRequest) Bind(_ *http.Request) error {
	if req.RFIDTag == "" {
		return errors.New("rfid_tag is required")
	}
	if len(req.RFIDTag) < 8 {
		return errors.New("rfid_tag must be at least 8 characters")
	}
	if len(req.RFIDTag) > 64 {
		return errors.New("rfid_tag must be at most 64 characters")
	}
	return nil
}

// RFIDAssignmentResponse represents an RFID tag assignment response (for students and staff)
type RFIDAssignmentResponse struct {
	Success     bool    `json:"success"`
	StudentID   int64   `json:"student_id"`   // For students: student_id, for staff: staff_id
	StudentName string  `json:"student_name"` // For students: student name, for staff: staff name
	RFIDTag     string  `json:"rfid_tag"`
	PreviousTag *string `json:"previous_tag,omitempty"`
	Message     string  `json:"message"`
}
