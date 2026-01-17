package checkin

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// validateDeviceContext validates the device context and returns an error response if invalid
func validateDeviceContext(w http.ResponseWriter, r *http.Request) *iot.Device {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		recordEventError(r.Context(), "checkin", "device_unauthorized", device.ErrMissingAPIKey)
		iotCommon.RenderError(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey))
		return nil
	}
	return deviceCtx
}

// parseCheckinRequest parses and validates the checkin request
func parseCheckinRequest(w http.ResponseWriter, r *http.Request, _ string) *CheckinRequest {
	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		recordEventError(r.Context(), "checkin", "invalid_request", err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return nil
	}
	return req
}

// lookupPersonByRFID finds a person by RFID tag and validates the assignment
func (rs *Resource) lookupPersonByRFID(ctx context.Context, w http.ResponseWriter, r *http.Request, rfid string) *users.Person {
	person, err := rs.UsersService.FindByTagID(ctx, rfid)
	if err != nil {
		recordEventError(ctx, "checkin", "rfid_not_found", err)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return nil
	}

	if person == nil || person.TagID == nil {
		recordEventError(ctx, "checkin", "rfid_unassigned", errors.New("rfid tag not assigned to any person"))
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil
	}

	return person
}

// lookupStudentFromPerson attempts to find a student from a person record.
// Returns nil if person is not a student or if lookup fails (errors are logged).
func (rs *Resource) lookupStudentFromPerson(ctx context.Context, personID int64) *users.Student {
	student, err := rs.UsersService.GetStudentByPersonID(ctx, personID)
	if err != nil {
		return nil
	}
	return student
}

// handleStaffScan checks if person is staff and handles supervisor authentication
// Returns true if the request was handled (either successfully or with error)
func (rs *Resource) handleStaffScan(w http.ResponseWriter, r *http.Request, _ *iot.Device, person *users.Person) bool {
	staff, err := rs.UsersService.GetStaffByPersonID(r.Context(), person.ID)
	if err != nil {
		recordEventError(r.Context(), "checkin", "staff_lookup_failed", err)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
		return true
	}

	if staff != nil {
		recordEventError(r.Context(), "checkin", "staff_not_supported", errors.New("staff rfid auth not supported"))
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff RFID authentication must be done via session management endpoints")))
		return true
	}

	// Neither student nor staff
	recordEventError(r.Context(), "checkin", "person_not_student_or_staff", errors.New("person not student or staff"))
	iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
	return true
}

// loadCurrentVisitWithRoom loads the current visit and its room information
func (rs *Resource) loadCurrentVisitWithRoom(ctx context.Context, studentID int64) *active.Visit {
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		return nil
	}

	if currentVisit == nil || currentVisit.ExitTime != nil {
		return nil
	}

	// Load the active group with room information
	activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err == nil && activeGroup != nil {
		currentVisit.ActiveGroup = activeGroup
		if activeGroup.RoomID > 0 {
			room, roomErr := rs.FacilityService.GetRoom(ctx, activeGroup.RoomID)
			if roomErr == nil && room != nil {
				activeGroup.Room = room
			}
		}
	}

	return currentVisit
}
