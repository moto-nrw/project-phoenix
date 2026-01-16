package checkin

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// validateDeviceContext validates the device context and returns an error response if invalid
func validateDeviceContext(w http.ResponseWriter, r *http.Request) *iot.Device {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		iotCommon.RenderError(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey))
		return nil
	}
	return deviceCtx
}

// parseCheckinRequest parses and validates the checkin request
func parseCheckinRequest(w http.ResponseWriter, r *http.Request, deviceID string) *CheckinRequest {
	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		logger.Logger.WithField("device_id", deviceID).WithError(err).Error("[CHECKIN] Invalid request")
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return nil
	}
	return req
}

// lookupPersonByRFID finds a person by RFID tag and validates the assignment
func (rs *Resource) lookupPersonByRFID(ctx context.Context, w http.ResponseWriter, r *http.Request, rfid string) *users.Person {
	logger.Logger.WithField("rfid", rfid).Debug("[CHECKIN] Looking up RFID tag")
	person, err := rs.UsersService.FindByTagID(ctx, rfid)
	if err != nil {
		logger.Logger.WithField("rfid", rfid).WithError(err).Error("[CHECKIN] RFID tag not found")
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return nil
	}

	if person == nil || person.TagID == nil {
		logger.Logger.WithField("rfid", rfid).Error("[CHECKIN] RFID tag not assigned to any person")
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil
	}

	logger.Logger.WithFields(map[string]interface{}{
		"rfid":       rfid,
		"person_id":  person.ID,
		"first_name": person.FirstName,
		"last_name":  person.LastName,
	}).Debug("[CHECKIN] RFID tag resolved to person")
	return person
}

// lookupStudentFromPerson attempts to find a student from a person record.
// Returns nil if person is not a student or if lookup fails (errors are logged).
func (rs *Resource) lookupStudentFromPerson(ctx context.Context, personID int64) *users.Student {
	student, err := rs.UsersService.GetStudentByPersonID(ctx, personID)
	if err != nil {
		// Log error but continue - person may be staff instead of student
		logger.Logger.WithField("person_id", personID).WithError(err).Debug("[CHECKIN] Student lookup")
		return nil
	}
	return student
}

// handleStaffScan checks if person is staff and handles supervisor authentication
// Returns true if the request was handled (either successfully or with error)
func (rs *Resource) handleStaffScan(w http.ResponseWriter, r *http.Request, _ *iot.Device, person *users.Person) bool {
	logger.Logger.WithField("person_id", person.ID).Debug("[CHECKIN] Person is not a student, checking if staff")

	staff, err := rs.UsersService.GetStaffByPersonID(r.Context(), person.ID)
	if err != nil {
		logger.Logger.WithField("person_id", person.ID).WithError(err).Error("[CHECKIN] Failed to lookup staff")
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
		return true
	}

	if staff != nil {
		logger.Logger.WithField("staff_id", staff.ID).Warn("[CHECKIN] Staff RFID auth via checkin endpoint not supported")
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff RFID authentication must be done via session management endpoints")))
		return true
	}

	// Neither student nor staff
	logger.Logger.WithField("person_id", person.ID).Error("[CHECKIN] Person is neither student nor staff")
	iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
	return true
}

// loadCurrentVisitWithRoom loads the current visit and its room information
func (rs *Resource) loadCurrentVisitWithRoom(ctx context.Context, studentID int64) *active.Visit {
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		logger.Logger.WithField("student_id", studentID).WithError(err).Debug("[CHECKIN] Error checking current visit")
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
