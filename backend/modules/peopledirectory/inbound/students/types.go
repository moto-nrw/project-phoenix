package students

import (
	errors "errors"
	http "net/http"
)

// RFIDAssignmentRequest is the RFID tag assignment payload. The kiosk sends
// the same body to the staff tag route of api/iot/data, which declares its
// own copy; the two validate the tag identically.
type RFIDAssignmentRequest struct {
	RFIDTag string `json:"rfid_tag"`
}

// Bind validates the RFID assignment request.
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

// RFIDAssignmentResponse represents an RFID tag assignment response
type RFIDAssignmentResponse struct {
	Success     bool    `json:"success"`
	StudentID   int64   `json:"student_id"`
	StudentName string  `json:"student_name"`
	RFIDTag     string  `json:"rfid_tag"`
	PreviousTag *string `json:"previous_tag,omitempty"`
	Message     string  `json:"message"`
}
