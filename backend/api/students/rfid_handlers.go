package students

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/iot"
)

// RFIDAssignmentRequest represents an RFID tag assignment request
type RFIDAssignmentRequest struct {
	RFIDTag string `json:"rfid_tag"`
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

// checkDeviceAuth verifies device authentication and returns the device
// Returns the device and true if successful, or renders an error and returns nil, false
func (rs *Resource) checkDeviceAuth(w http.ResponseWriter, r *http.Request) (*iot.Device, bool) {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		renderError(w, r, ErrorUnauthorized(errors.New("device authentication required")))
		return nil, false
	}
	return deviceCtx, true
}

// assignRFIDTag handles assigning an RFID tag to a student (device-authenticated endpoint)
func (rs *Resource) assignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx, ok := rs.checkDeviceAuth(w, r)
	if !ok {
		return
	}

	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get person details for the student
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// With global PIN authentication, we trust the device to assign tags to any student
	// No need to check teacher supervision rights

	// Store previous tag for response
	var previousTag *string
	if person.TagID != nil {
		previousTag = person.TagID
	}

	// Assign the RFID tag (this handles unlinking old assignments automatically)
	if err := rs.PersonService.LinkToRFIDCard(r.Context(), person.ID, req.RFIDTag); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Create response
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     req.RFIDTag,
		PreviousTag: previousTag,
		Message:     "RFID tag assigned successfully",
	}

	if previousTag != nil {
		response.Message = "RFID tag assigned successfully (previous tag replaced)"
	}

	// Log assignment for audit trail
	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"device_id":    deviceCtx.DeviceID,
			"student_id":   student.ID,
			"tag":          req.RFIDTag,
			"previous_tag": previousTag,
		}).Info("RFID tag assigned")
	}

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// unassignRFIDTag handles removing an RFID tag from a student (device-authenticated endpoint)
func (rs *Resource) unassignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx, ok := rs.checkDeviceAuth(w, r)
	if !ok {
		return
	}

	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Get person details for the student
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Check if student has an RFID tag assigned
	if person.TagID == nil {
		renderError(w, r, ErrorNotFound(errors.New("student has no RFID tag assigned")))
		return
	}

	// Store removed tag for response
	removedTag := *person.TagID

	// Unlink the RFID tag
	if err := rs.PersonService.UnlinkFromRFIDCard(r.Context(), person.ID); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Create response
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     removedTag,
		Message:     "RFID tag unassigned successfully",
	}

	// Log unassignment for audit trail
	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"device_id":  deviceCtx.DeviceID,
			"student_id": student.ID,
			"tag":        removedTag,
		}).Info("RFID tag unassigned")
	}

	common.Respond(w, r, http.StatusOK, response, response.Message)
}
