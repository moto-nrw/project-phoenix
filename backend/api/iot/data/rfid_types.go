package data

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

const invalidStaffIDMessage = "invalid staff ID"

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
type RFIDAssignmentResponse = devicescan.TagAssignmentChange
