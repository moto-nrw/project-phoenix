package checkin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// getAttendanceStatus handles getting a student's attendance status by RFID
func (rs *AttendanceResource) getAttendanceStatus(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get RFID from URL parameter and normalize it
	rfid := chi.URLParam(r, "rfid")
	if rfid == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("RFID parameter is required")))
		return
	}

	normalizedRFID := users.NormalizeTagID(rfid)

	// Find and validate student by RFID
	student, person, ok := rs.findStudentByRFID(w, r, normalizedRFID)
	if !ok {
		return
	}

	// Get attendance status from service
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Get optional group information
	groupInfo := rs.getStudentGroupInfo(r.Context(), student)

	// Build and return response
	response := AttendanceStatusResponse{
		Student: AttendanceStudentInfo{
			ID:        student.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
			Group:     groupInfo,
		},
		Attendance: AttendanceInfo{
			Status:       attendanceStatus.Status,
			Date:         attendanceStatus.Date,
			CheckInTime:  attendanceStatus.CheckInTime,
			CheckOutTime: attendanceStatus.CheckOutTime,
			CheckedInBy:  attendanceStatus.CheckedInBy,
			CheckedOutBy: attendanceStatus.CheckedOutBy,
		},
	}

	common.Respond(w, r, http.StatusOK, response, "Student attendance status retrieved successfully")
}

// toggleAttendance handles toggling a student's attendance state via RFID tag
func (rs *AttendanceResource) toggleAttendance(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse request body
	req := &AttendanceToggleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Handle "cancel" action by returning cancelled response
	if req.Action == "cancel" {
		rs.handleCancelAction(w, r)
		return
	}

	normalizedRFID := users.NormalizeTagID(req.RFID)

	// Handle "confirm_daily_checkout" action - process the deferred daily checkout
	if req.Action == "confirm_daily_checkout" {
		rs.handleDailyCheckout(w, r, normalizedRFID, req, deviceCtx.ID)
		return
	}

	// Handle normal toggle (confirm action)
	rs.handleNormalToggle(w, r, normalizedRFID)
}

// Helper functions

// findStudentByRFID finds a student by RFID tag and returns the student, person, and success status
func (rs *AttendanceResource) findStudentByRFID(w http.ResponseWriter, r *http.Request, normalizedRFID string) (*users.Student, *users.Person, bool) {
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedRFID)
	if err != nil {
		shared.RecordUnregisteredTagScan(r.Context(), rs.UnregisteredTagScans, slog.Default(), normalizedRFID)
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgRFIDTagNotFound)))
		return nil, nil, false
	}

	if person == nil || person.TagID == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil, nil, false
	}

	student, err := rs.UsersService.GetStudentByPersonID(r.Context(), person.ID)
	if err != nil || student == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgPersonNotStudent)))
		return nil, nil, false
	}

	return student, person, true
}

// getStudentGroupInfo gets optional group information for a student
func (rs *AttendanceResource) getStudentGroupInfo(ctx context.Context, student *users.Student) *AttendanceGroupInfo {
	if student.GroupID == nil {
		return nil
	}

	group, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil || group == nil {
		return nil
	}

	return &AttendanceGroupInfo{
		ID:   group.ID,
		Name: group.Name,
	}
}

// handleCancelAction handles the "cancel" action for attendance toggle
func (rs *AttendanceResource) handleCancelAction(w http.ResponseWriter, r *http.Request) {
	response := AttendanceToggleResponse{
		Action:  "cancelled",
		Message: "Attendance tracking cancelled",
	}
	common.Respond(w, r, http.StatusOK, response, "Attendance tracking cancelled")
}

// handleDailyCheckout handles the "confirm_daily_checkout" action.
// Normally the student's visit was already ended by the checkin handler
// (student is "unterwegs") and this endpoint only updates the attendance
// record when the student confirms "nach Hause". If a visit is still open,
// CheckOutStudent ends it in the same request transaction (issue #895 — see
// services/active.performCheckOut).
func (rs *AttendanceResource) handleDailyCheckout(w http.ResponseWriter, r *http.Request, normalizedRFID string, req *AttendanceToggleRequest, deviceID int64) {
	// Find person by RFID tag
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedRFID)
	if err != nil || person == nil {
		shared.RecordUnregisteredTagScan(r.Context(), rs.UnregisteredTagScans, slog.Default(), normalizedRFID)
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgRFIDTagNotFound)))
		return
	}

	// Get student from person
	student, err := rs.UsersService.GetStudentByPersonID(r.Context(), person.ID)
	if err != nil || student == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgPersonNotStudent)))
		return
	}

	slog.Default().InfoContext(r.Context(), "confirming daily checkout",
		slog.Int64("student_id", student.ID),
		slog.String("destination", *req.Destination),
	)

	// Verify the student has an attendance record for today.
	// Without one, daily checkout makes no sense — the student was never checked in.
	currentStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		slog.Default().ErrorContext(r.Context(), "failed to get attendance status",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if currentStatus.Status != "checked_in" && currentStatus.Status != "checked_out" {
		slog.Default().ErrorContext(r.Context(), "student has no attendance record for today",
			slog.Int64("student_id", student.ID),
			slog.String("status", currentStatus.Status),
		)
		common.RenderError(w, r, common.ErrorNotFound(
			errors.New("student has no attendance record for today")))
		return
	}

	// Only update attendance when student is going home ("zuhause").
	// "unterwegs" means they stay in transit — no attendance change needed.
	if *req.Destination == "zuhause" {
		// Guard against a race where attendance was already changed
		// (e.g., teacher manually checked out) between scan and "nach Hause" click.
		switch currentStatus.Status {
		case "checked_out":
			slog.Default().DebugContext(r.Context(), "student already checked out, skipping attendance toggle",
				slog.Int64("student_id", student.ID),
			)
		case "checked_in":
			// Set attendance to "checked_out" — sets check_out_time on today's record.
			// The active service resolves the device's active supervisor as the
			// auditable checked_out_by principal before writing attendance.
			_, err := rs.ActiveService.CheckOutStudentFromDevice(r.Context(), student.ID, deviceID)
			if err != nil {
				slog.Default().ErrorContext(r.Context(), "failed to update attendance for daily checkout",
					slog.Int64("student_id", student.ID),
					slog.String("error", err.Error()),
				)
				common.RenderError(w, r, shared.ErrorRenderer(err))
				return
			}

			// Broadcast SSE event so the OGS Groups page updates in real time
			rs.ActiveService.BroadcastDailyCheckout(r.Context(), student.ID)
		}
	}

	// Determine action and message based on destination
	action := "checked_out_daily"
	message := "Tschüss " + person.FirstName + "!"
	if req.Destination != nil && *req.Destination == "unterwegs" {
		action = "checked_out"
		message = "Viel Spaß!"
	}

	slog.Default().InfoContext(r.Context(), "daily checkout confirmed",
		slog.Int64("student_id", student.ID),
		slog.String("action", action),
		slog.String("destination", *req.Destination),
	)

	// Resolve feedback_enabled setting so PyrePortal knows whether to show the feedback modal
	var feedbackEnabled *bool
	if rs.SettingsService != nil {
		if val, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyFeedbackEnabled); err == nil {
			feedbackEnabled = &val
		}
	}

	response := AttendanceToggleResponse{
		Action:  action,
		Message: message,
		Student: AttendanceStudentInfo{
			ID:        student.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
		},
		FeedbackEnabled: feedbackEnabled,
	}
	common.Respond(w, r, http.StatusOK, response, "Daily checkout confirmed")
}

// handleNormalToggle handles the normal "confirm" action for attendance toggle
func (rs *AttendanceResource) handleNormalToggle(w http.ResponseWriter, r *http.Request, normalizedRFID string) {
	// Find person by RFID tag
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedRFID)
	if err != nil {
		shared.RecordUnregisteredTagScan(r.Context(), rs.UnregisteredTagScans, slog.Default(), normalizedRFID)
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgRFIDTagNotFound)))
		return
	}

	if person == nil || person.TagID == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return
	}

	// Get student from person
	student := rs.lookupStudent(w, r, person.ID)
	if student == nil {
		return // Error already handled by lookupStudent
	}

	// Get device and staff context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffID := rs.getStaffIDFromContext(r.Context())

	// Toggle attendance
	result, err := rs.ActiveService.ToggleStudentAttendance(r.Context(), student.ID, staffID, deviceCtx.ID, false)
	if err != nil {
		slog.Default().ErrorContext(r.Context(), "failed to toggle attendance",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build and send response
	rs.sendToggleResponse(w, r, student, person, result)
}

// lookupStudent gets student from person ID with error handling
func (rs *AttendanceResource) lookupStudent(w http.ResponseWriter, r *http.Request, personID int64) *users.Student {
	student, err := rs.UsersService.GetStudentByPersonID(r.Context(), personID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgPersonNotStudent)))
		return nil
	}
	if student == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(shared.ErrMsgPersonNotStudent)))
		return nil
	}
	return student
}

// getStaffIDFromContext extracts staff ID from device context
func (rs *AttendanceResource) getStaffIDFromContext(ctx context.Context) int64 {
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		return staffCtx.ID
	}
	return 0
}

// sendToggleResponse builds and sends the attendance toggle response
func (rs *AttendanceResource) sendToggleResponse(w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, result *activeSvc.AttendanceResult) {
	// Get updated attendance status
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Load student's group info
	groupInfo := rs.getStudentGroupInfo(r.Context(), student)

	// Determine user-friendly message
	message := rs.buildAttendanceMessage(result.Action, person.FirstName)

	// Build and return response
	response := AttendanceToggleResponse{
		Action: result.Action,
		Student: AttendanceStudentInfo{
			ID:        student.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
			Group:     groupInfo,
		},
		Attendance: AttendanceInfo{
			Status:       attendanceStatus.Status,
			Date:         attendanceStatus.Date,
			CheckInTime:  attendanceStatus.CheckInTime,
			CheckOutTime: attendanceStatus.CheckOutTime,
			CheckedInBy:  attendanceStatus.CheckedInBy,
			CheckedOutBy: attendanceStatus.CheckedOutBy,
		},
		Message: message,
	}

	common.Respond(w, r, http.StatusOK, response, fmt.Sprintf("Student %s successfully", result.Action))
}

// buildAttendanceMessage creates user-friendly message for attendance action
func (rs *AttendanceResource) buildAttendanceMessage(action, firstName string) string {
	switch action {
	case "checked_in":
		return fmt.Sprintf("Hallo %s!", firstName)
	case "checked_out":
		return fmt.Sprintf("Tschüss %s!", firstName)
	default:
		return fmt.Sprintf("Attendance %s for %s", action, firstName)
	}
}
