package rfid

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
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
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Get the staff member
	staffRepo := rs.UsersService.StaffRepository()
	staff, err := staffRepo.FindByID(r.Context(), staffID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff not found")))
		return
	}

	// Get person details for the staff member
	person, err := rs.UsersService.Get(r.Context(), staff.PersonID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get person data for staff")))
		return
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
	log.Printf("RFID tag assignment: device=%s, staff=%d, tag=%s, previous_tag=%v",
		deviceCtx.DeviceID, staffID, req.RFIDTag, previousTag)

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
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Get the staff member
	staffRepo := rs.UsersService.StaffRepository()
	staff, err := staffRepo.FindByID(r.Context(), staffID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff not found")))
		return
	}

	// Get person details for the staff member
	person, err := rs.UsersService.Get(r.Context(), staff.PersonID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get person data for staff")))
		return
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
	log.Printf("RFID tag unassignment: device=%s, staff=%d, tag=%s",
		deviceCtx.DeviceID, staffID, removedTag)

	common.Respond(w, r, http.StatusOK, response, response.Message)
}
