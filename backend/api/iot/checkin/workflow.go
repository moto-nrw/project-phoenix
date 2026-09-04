package checkin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	checkinSvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
)

// This file holds the thin HTTP shells for the RFID check-in flow. The
// business logic lives in services/iot/checkin.CheckinService (issue #575 B8);
// these functions parse the request, delegate to the service, and map the
// service's typed errors back to the exact renderers the flow emitted before —
// preserving the PyrePortal wire contract byte-for-byte.

// validateDeviceContext validates the device context and returns an error response if invalid
func validateDeviceContext(w http.ResponseWriter, r *http.Request) *iot.Device {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		common.RenderError(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey))
		return nil
	}
	return deviceCtx
}

// parseCheckinRequest parses and validates the checkin request
func parseCheckinRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, logger *slog.Logger, deviceID string) *CheckinRequest {
	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		logger.ErrorContext(ctx, "invalid checkin request",
			slog.String("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil
	}
	return req
}

// renderCheckinError maps a CheckinService typed error to the exact renderer the
// pre-extraction handler emitted, keeping the PyrePortal wire contract intact.
// The capacity 409s carry their `details` object only when the tenant's
// checkin.*_capacity_details_enabled setting allows it (issue #1879); when
// off, the NoDetails variant is rendered and the kiosk shows its generic
// German fallback.
func (rs *Resource) renderCheckinError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()

	var roomCap *checkinSvc.RoomCapacityError
	if errors.As(err, &roomCap) {
		if rs.capacityDetailsEnabled(ctx, configModel.KeyCheckinRoomCapacityDetailsEnabled, true) {
			common.RenderError(w, r, ErrorRoomCapacityExceeded(roomCap.RoomID, roomCap.RoomName, roomCap.CurrentOccupancy, roomCap.MaxCapacity))
		} else {
			common.RenderError(w, r, ErrorRoomCapacityExceededNoDetails())
		}
		return
	}

	var actCap *checkinSvc.ActivityCapacityError
	if errors.As(err, &actCap) {
		if rs.capacityDetailsEnabled(ctx, configModel.KeyCheckinActivityCapacityDetailsEnabled, false) {
			common.RenderError(w, r, ErrorActivityCapacityExceeded(actCap.ActivityID, actCap.ActivityName, actCap.CurrentOccupancy, actCap.MaxCapacity))
		} else {
			common.RenderError(w, r, ErrorActivityCapacityExceededNoDetails())
		}
		return
	}

	var conflict *checkinSvc.StudentAlreadyActiveConflict
	if errors.As(err, &conflict) {
		common.RenderError(w, r, ErrorStudentAlreadyActive(conflict.StudentID, conflict.ExistingVisitID, conflict.EntryTime, conflict.RoomID, conflict.RoomName))
		return
	}

	var ce *checkinSvc.CheckinError
	if errors.As(err, &ce) {
		switch ce.Kind {
		case checkinSvc.KindNotFound:
			common.RenderError(w, r, common.ErrorNotFound(errors.New(ce.Message)))
		case checkinSvc.KindInvalidRequest:
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(ce.Message)))
		case checkinSvc.KindInternalWrap:
			common.RenderError(w, r, common.ErrorInternalServerWrap(ce.Message, ce.Cause))
		default: // KindInternal
			common.RenderError(w, r, common.ErrorInternalServer(errors.New(ce.Message)))
		}
		return
	}

	// Fallback — no typed error matched (should not happen). Render a generic 500.
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// capacityDetailsEnabled resolves a capacity-detail disclosure setting,
// failing safe to `fallback` when the settings service is absent
// (bare-struct tests) or resolution errors. The fallback must match the
// registry default for the key (activity=false, room=true) so the fail-safe
// path preserves default behavior instead of surfacing a 500.
func (rs *Resource) capacityDetailsEnabled(ctx context.Context, key string, fallback bool) bool {
	if rs.SettingsService == nil {
		return fallback
	}
	val, err := rs.SettingsService.ResolveBool(ctx, key)
	if err != nil {
		rs.getLogger().WarnContext(ctx, "failed to resolve capacity detail setting; using fallback",
			slog.String("setting_key", key),
			slog.Bool("fallback", fallback),
			slog.String("error", err.Error()),
		)
		return fallback
	}
	return val
}

// lookupPersonByRFID finds a person by RFID tag and validates the assignment
func (rs *Resource) lookupPersonByRFID(ctx context.Context, w http.ResponseWriter, r *http.Request, rfid string) *users.Person {
	rs.getLogger().DebugContext(ctx, "looking up RFID tag", slog.String("rfid", rfid))
	person, err := rs.Checkin.ResolvePersonByRFID(ctx, rfid)
	if err != nil {
		shared.RecordUnregisteredTagScan(ctx, rs.UnregisteredTagScans, rs.getLogger(), rfid)
		rs.getLogger().WarnContext(ctx, "RFID tag not found",
			slog.String("rfid", rfid),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, common.ErrorNotFoundWithCode(errors.New(shared.ErrMsgRFIDTagNotFound), shared.ErrCodeRFIDTagNotFound))
		return nil
	}

	if person == nil || person.TagID == nil {
		rs.getLogger().WarnContext(ctx, "RFID tag not assigned to any person",
			slog.String("rfid", rfid),
		)
		common.RenderError(w, r, common.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil
	}

	rs.getLogger().DebugContext(ctx, "RFID tag belongs to person",
		slog.String("rfid", rfid),
		slog.String("person_name", person.FirstName+" "+person.LastName),
		slog.Int64("person_id", person.ID),
	)
	return person
}

// lookupStudentFromPerson attempts to find a student from a person record.
// Returns nil if person is not a student or if lookup fails (errors are logged).
func (rs *Resource) lookupStudentFromPerson(ctx context.Context, personID int64) *users.Student {
	student, err := rs.Checkin.ResolveStudentFromPerson(ctx, personID)
	if err != nil {
		// Log error but continue - person may be staff instead of student
		rs.getLogger().DebugContext(ctx, "student lookup for person",
			slog.Int64("person_id", personID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return student
}

// handleStaffScan checks if person is staff and handles supervisor authentication
// Returns true if the request was handled (either successfully or with error)
func (rs *Resource) handleStaffScan(w http.ResponseWriter, r *http.Request, deviceCtx *iot.Device, person *users.Person) bool {
	rs.getLogger().DebugContext(r.Context(), "person is not a student, checking if staff",
		slog.Int64("person_id", person.ID),
	)

	staff, err := rs.UsersService.GetStaffByPersonID(r.Context(), person.ID)
	if err != nil {
		rs.getLogger().ErrorContext(r.Context(), "failed to lookup staff for person",
			slog.Int64("person_id", person.ID),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, common.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
		return true
	}

	if staff != nil {
		rs.getLogger().InfoContext(r.Context(), "found staff, attempting supervisor authentication",
			slog.Int64("staff_id", staff.ID),
		)
		rs.handleSupervisorScan(w, r, deviceCtx, staff, person)
		return true
	}

	// Neither student nor staff
	rs.getLogger().WarnContext(r.Context(), "person is neither student nor staff",
		slog.Int64("person_id", person.ID),
	)
	common.RenderError(w, r, common.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
	return true
}

// handleSupervisorScan processes RFID-based supervisor authentication for staff.
// The business core (session lookup + supervisor add) lives in the CheckinService;
// this shell builds the 200 response, preserving the exact wire body.
func (rs *Resource) handleSupervisorScan(w http.ResponseWriter, r *http.Request, deviceCtx *iot.Device, staff *users.Staff, person *users.Person) {
	ctx := r.Context()

	result, err := rs.Checkin.AddStaffAsSupervisor(ctx, deviceCtx.ID, deviceCtx.DeviceID, staff.ID)
	if err != nil {
		rs.renderCheckinError(w, r, err)
		return
	}

	staffName := person.FirstName + " " + person.LastName
	message := "Supervisor authenticated"
	if result.ActivityName != "" {
		message = "Supervisor authenticated for " + result.ActivityName
	}

	response := map[string]interface{}{
		"student_id":   staff.ID,
		"student_name": staffName,
		"action":       "supervisor_authenticated",
		"room_name":    result.RoomName,
		"processed_at": time.Now(),
		"message":      message,
		"status":       "success",
	}

	common.Respond(w, r, http.StatusOK, response, "Supervisor authenticated")
}
