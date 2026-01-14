package attendance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// getAttendanceStatus handles getting a student's attendance status by RFID
func (rs *Resource) getAttendanceStatus(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get RFID from URL parameter and normalize it
	rfid := chi.URLParam(r, "rfid")
	if rfid == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("RFID parameter is required")))
		return
	}

	normalizedRFID := iotCommon.NormalizeTagID(rfid)

	// Find and validate student by RFID
	student, person, ok := rs.findStudentByRFID(w, r, normalizedRFID)
	if !ok {
		return
	}

	// Get attendance status from service
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
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
func (rs *Resource) toggleAttendance(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse request body
	req := &AttendanceToggleRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Handle "cancel" action by returning cancelled response
	if req.Action == "cancel" {
		rs.handleCancelAction(w, r)
		return
	}

	normalizedRFID := iotCommon.NormalizeTagID(req.RFID)

	// Handle "confirm_daily_checkout" action - process the deferred daily checkout
	if req.Action == "confirm_daily_checkout" {
		rs.handleDailyCheckout(w, r, normalizedRFID, req)
		return
	}

	// Handle normal toggle (confirm action)
	rs.handleNormalToggle(w, r, normalizedRFID)
}

// Helper functions

// findStudentByRFID finds a student by RFID tag and returns the student, person, and success status
func (rs *Resource) findStudentByRFID(w http.ResponseWriter, r *http.Request, normalizedRFID string) (*users.Student, *users.Person, bool) {
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedRFID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return nil, nil, false
	}

	if person == nil || person.TagID == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil, nil, false
	}

	student, err := rs.UsersService.StudentRepository().FindByPersonID(r.Context(), person.ID)
	if err != nil || student == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgPersonNotStudent)))
		return nil, nil, false
	}

	return student, person, true
}

// getStudentGroupInfo gets optional group information for a student
func (rs *Resource) getStudentGroupInfo(ctx context.Context, student *users.Student) *AttendanceGroupInfo {
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
func (rs *Resource) handleCancelAction(w http.ResponseWriter, r *http.Request) {
	response := AttendanceToggleResponse{
		Action:  "cancelled",
		Message: "Attendance tracking cancelled",
	}
	common.Respond(w, r, http.StatusOK, response, "Attendance tracking cancelled")
}

// handleDailyCheckout handles the "confirm_daily_checkout" action
func (rs *Resource) handleDailyCheckout(w http.ResponseWriter, r *http.Request, normalizedRFID string, req *AttendanceToggleRequest) {
	// Find person by RFID tag
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedRFID)
	if err != nil || person == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return
	}

	// Get student from person
	studentRepo := rs.UsersService.StudentRepository()
	student, err := studentRepo.FindByPersonID(r.Context(), person.ID)
	if err != nil || student == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgPersonNotStudent)))
		return
	}

	// Find the student's active visit
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID)
	if err != nil {
		log.Printf("[DAILY_CHECKOUT] ERROR: Failed to get current visit for student %d: %v", student.ID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}
	if currentVisit == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("no active visit found for student")))
		return
	}

	log.Printf("[DAILY_CHECKOUT] Confirming daily checkout for student %s %s (ID: %d), destination: %s",
		person.FirstName, person.LastName, student.ID, *req.Destination)

	// End the visit - only sync attendance if student is going home ("zuhause")
	ctx := r.Context()
	if *req.Destination == "zuhause" {
		ctx = activeSvc.WithAttendanceAutoSync(ctx)
	}

	if err := rs.ActiveService.EndVisit(ctx, currentVisit.ID); err != nil {
		log.Printf("[DAILY_CHECKOUT] ERROR: Failed to end visit %d: %v", currentVisit.ID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	// Determine action and message based on destination
	action := "checked_out_daily"
	message := "Tschüss " + person.FirstName + "!"
	if req.Destination != nil && *req.Destination == "unterwegs" {
		action = "checked_out"
		message = "Viel Spaß!"
	}

	log.Printf("[DAILY_CHECKOUT] SUCCESS: Student %s %s checked out, action=%s, destination=%s",
		person.FirstName, person.LastName, action, *req.Destination)

	response := AttendanceToggleResponse{
		Action:  action,
		Message: message,
		Student: AttendanceStudentInfo{
			ID:        student.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
		},
	}
	common.Respond(w, r, http.StatusOK, response, "Daily checkout confirmed")
}

// handleNormalToggle handles the normal "confirm" action for attendance toggle
func (rs *Resource) handleNormalToggle(w http.ResponseWriter, r *http.Request, normalizedRFID string) {
	// Find person by RFID tag
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedRFID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return
	}

	if person == nil || person.TagID == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
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
		log.Printf("[ATTENDANCE_TOGGLE] ERROR: Failed to toggle attendance for student %d: %v", student.ID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build and send response
	rs.sendToggleResponse(w, r, student, person, result)
}

// lookupStudent gets student from person ID with error handling
func (rs *Resource) lookupStudent(w http.ResponseWriter, r *http.Request, personID int64) *users.Student {
	studentRepo := rs.UsersService.StudentRepository()
	student, err := studentRepo.FindByPersonID(r.Context(), personID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgPersonNotStudent)))
		return nil
	}
	if student == nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgPersonNotStudent)))
		return nil
	}
	return student
}

// getStaffIDFromContext extracts staff ID from device context
func (rs *Resource) getStaffIDFromContext(ctx context.Context) int64 {
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		return staffCtx.ID
	}
	return 0
}

// sendToggleResponse builds and sends the attendance toggle response
func (rs *Resource) sendToggleResponse(w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, result *activeSvc.AttendanceResult) {
	// Get updated attendance status
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
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
func (rs *Resource) buildAttendanceMessage(action, firstName string) string {
	switch action {
	case "checked_in":
		return fmt.Sprintf("Hallo %s!", firstName)
	case "checked_out":
		return fmt.Sprintf("Tschüss %s!", firstName)
	default:
		return fmt.Sprintf("Attendance %s for %s", action, firstName)
	}
}

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// GetAttendanceStatusHandler returns the getAttendanceStatus handler
func (rs *Resource) GetAttendanceStatusHandler() http.HandlerFunc { return rs.getAttendanceStatus }

// ToggleAttendanceHandler returns the toggleAttendance handler
func (rs *Resource) ToggleAttendanceHandler() http.HandlerFunc { return rs.toggleAttendance }
