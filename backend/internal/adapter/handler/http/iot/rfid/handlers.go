package rfid

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// assignStaffRFIDTag handles assigning an RFID tag to a staff member (device-authenticated endpoint)
func (rs *Resource) assignStaffRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse staff ID from URL
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Get staff and person details
	staff, person, ok := rs.getStaffAndPerson(w, r, staffID)
	if !ok {
		return // Error already handled by getStaffAndPerson
	}

	// Store previous tag for response
	var previousTag *string
	if person.TagID != nil {
		previousTag = person.TagID
	}

	// Assign the RFID tag (this handles unlinking old assignments automatically)
	if err := rs.UsersService.LinkToRFIDCard(r.Context(), person.ID, req.RFIDTag); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	// Create response (reuse student response type with staff data)
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   staff.ID, // Field name is StudentID but holds staff_id
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     req.RFIDTag,
		PreviousTag: previousTag,
		Message:     "RFID tag assigned successfully",
	}

	if previousTag != nil {
		response.Message = "RFID tag assigned successfully (previous tag replaced)"
	}

	// Log assignment for audit trail
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id":    deviceCtx.DeviceID,
			"staff_id":     staffID,
			"tag":          req.RFIDTag,
			"previous_tag": previousTag,
		}).Info("RFID tag assignment")
	}

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// unassignStaffRFIDTag handles removing an RFID tag from a staff member (device-authenticated endpoint)
func (rs *Resource) unassignStaffRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse staff ID from URL
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return
	}

	// Get staff and person details
	staff, person, ok := rs.getStaffAndPerson(w, r, staffID)
	if !ok {
		return // Error already handled by getStaffAndPerson
	}

	// Check if staff has an RFID tag assigned
	if person.TagID == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff has no RFID tag assigned")))
		return
	}

	// Store removed tag for response
	removedTag := *person.TagID

	// Unlink the RFID tag
	if err := rs.UsersService.UnlinkFromRFIDCard(r.Context(), person.ID); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	// Create response (reuse student response type with staff data)
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   staff.ID, // Field name is StudentID but holds staff_id
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     removedTag,
		Message:     "RFID tag unassigned successfully",
	}

	// Log unassignment for audit trail
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id": deviceCtx.DeviceID,
			"staff_id":  staffID,
			"tag":       removedTag,
		}).Info("RFID tag unassignment")
	}

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// Helper functions

// getStaffAndPerson retrieves staff and person details by staff ID with error handling
// Returns staff, person, and success status. If ok=false, error response already sent
func (rs *Resource) getStaffAndPerson(w http.ResponseWriter, r *http.Request, staffID int64) (*users.Staff, *users.Person, bool) {
	// Get the staff member with person details
	staff, err := rs.UsersService.GetStaffWithPerson(r.Context(), staffID)
	if err != nil || staff == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff not found")))
		return nil, nil, false
	}

	if staff.Person == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get person data for staff")))
		return nil, nil, false
	}

	return staff, staff.Person, true
}
