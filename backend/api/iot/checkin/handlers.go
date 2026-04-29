package checkin

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

const errStudentRFIDRequiredForPickupQuery = "student RFID tag required for pickup query"

// devicePing handles ping requests from RFID devices
// This endpoint keeps both the device AND any active session alive
func (rs *Resource) devicePing(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context (no staff context needed with global PIN)
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Update device last seen time (already done in middleware, but let's be explicit)
	if err := rs.IoTService.PingDevice(r.Context(), deviceCtx.DeviceID); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Also update session activity if device has an active session
	// This keeps the session alive as long as the device is pinging
	sessionActive := false
	if session, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID); err == nil && session != nil {
		sessionActive = true // Session exists - set immediately regardless of update success
		if err := rs.ActiveService.UpdateSessionActivity(r.Context(), session.ID); err != nil {
			rs.getLogger().WarnContext(r.Context(), "failed to update session activity during ping",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Return device status (no staff info with global PIN)
	response := map[string]interface{}{
		"device_id":      deviceCtx.DeviceID,
		"device_name":    deviceCtx.Name,
		"status":         deviceCtx.Status,
		"last_seen":      deviceCtx.LastSeen,
		"is_online":      deviceCtx.IsOnline(),
		"ping_time":      time.Now(),
		"session_active": sessionActive,
	}

	common.Respond(w, r, http.StatusOK, response, "Device ping successful")
}

// deviceStatus handles status requests from RFID devices
func (rs *Resource) deviceStatus(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Return detailed device status
	response := map[string]interface{}{
		"device": map[string]interface{}{
			"id":          deviceCtx.ID,
			"device_id":   deviceCtx.DeviceID,
			"device_type": deviceCtx.DeviceType,
			"name":        deviceCtx.Name,
			"status":      deviceCtx.Status,
			"last_seen":   deviceCtx.LastSeen,
			"is_online":   deviceCtx.IsOnline(),
			"is_active":   deviceCtx.IsActive(),
		},
		"authenticated_at": time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Device status retrieved")
}

// devicePickupQuery handles read-only pickup information lookups for students.
func (rs *Resource) devicePickupQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	deviceCtx := validateDeviceContext(w, r)
	if deviceCtx == nil {
		return
	}

	req := &PickupQueryRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.getLogger().ErrorContext(ctx, "invalid pickup query request",
			slog.String("device_id", deviceCtx.DeviceID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	person, err := rs.resolvePersonByRFID(ctx, req.StudentRFID)
	if err != nil {
		if errors.Is(err, usersSvc.ErrPersonNotFound) {
			rs.getLogger().WarnContext(ctx, "RFID tag not found during pickup query",
				slog.String("rfid", req.StudentRFID),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
			return
		}
		rs.getLogger().ErrorContext(ctx, "failed to lookup RFID tag during pickup query",
			slog.String("rfid", req.StudentRFID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	if person == nil || person.TagID == nil {
		rs.getLogger().WarnContext(ctx, "RFID tag not assigned to any person during pickup query",
			slog.String("rfid", req.StudentRFID),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return
	}

	student, err := rs.resolveStudentFromPerson(ctx, person.ID)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to lookup student during pickup query",
			slog.Int64("person_id", person.ID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	if student == nil {
		staff, err := rs.resolveStaffFromPerson(ctx, person.ID)
		if err != nil {
			rs.getLogger().ErrorContext(ctx, "failed to lookup staff during pickup query",
				slog.Int64("person_id", person.ID),
				slog.String("error", err.Error()),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
			return
		}
		if staff != nil {
			iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errStudentRFIDRequiredForPickupQuery)))
			return
		}
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
		return
	}

	response := map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"action":       "pickup_info",
		"processed_at": now,
		"status":       "success",
	}

	if rs.PickupScheduleService != nil {
		effectivePickup, err := rs.PickupScheduleService.GetEffectivePickupTimeForDate(ctx, student.ID, now)
		if err != nil {
			rs.getLogger().ErrorContext(ctx, "failed to get pickup info during pickup query",
				slog.Int64("student_id", student.ID),
				slog.String("error", err.Error()),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
			return
		} else {
			attachPickupInfoToResponse(response, effectivePickup)
		}
	}

	if session, err := rs.ActiveService.GetDeviceCurrentSession(ctx, deviceCtx.ID); err == nil && session != nil {
		if err := rs.ActiveService.UpdateSessionActivity(ctx, session.ID); err != nil {
			rs.getLogger().WarnContext(ctx, "failed to update session activity during pickup query",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	common.Respond(w, r, http.StatusOK, response, "Pickup information retrieved successfully")
}

// deviceCheckin handles student check-in/check-out requests from RFID devices
func (rs *Resource) deviceCheckin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Step 1: Validate device context
	deviceCtx := validateDeviceContext(w, r)
	if deviceCtx == nil {
		return
	}
	rs.getLogger().DebugContext(ctx, "starting checkin process",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int64("device_db_id", deviceCtx.ID),
	)

	// Step 2: Parse and validate request
	req := parseCheckinRequest(ctx, w, r, rs.getLogger(), deviceCtx.DeviceID)
	if req == nil {
		return
	}
	rs.getLogger().DebugContext(ctx, "checkin request details",
		slog.String("action", req.Action),
		slog.String("student_rfid", req.StudentRFID),
		slog.Any("room_id", req.RoomID),
	)

	// Step 3: Lookup person by RFID
	person := rs.lookupPersonByRFID(ctx, w, r, req.StudentRFID)
	if person == nil {
		return
	}

	// Step 4: Check if person is a student
	student := rs.lookupStudentFromPerson(ctx, person.ID)
	if student == nil {
		// Not a student - attempt staff scan handling (always return after)
		rs.handleStaffScan(w, r, deviceCtx, person)
		return
	}
	rs.getLogger().InfoContext(ctx, "found student",
		slog.Int64("student_id", student.ID),
		slog.String("class", student.SchoolClass),
	)
	student.Person = person

	// Binary-mode tenants track attendance only — no visits, no rooms, no
	// active groups. Short-circuit the detailed-mode flow and toggle the
	// attendance row directly. This keeps PyrePortal's existing response
	// contract intact (same CheckinResponse shape) while skipping all the
	// visit/room/supervision complexity that doesn't apply here.
	if rs.ActiveService.GetPresenceMode(ctx) == "binary" {
		rs.processBinaryModeCheckin(w, r, student, deviceCtx, now)
		return
	}

	// Step 5: Load current visit with room information
	currentVisit := rs.loadCurrentVisitWithRoom(ctx, student.ID)

	// Step 6: Process checkout if student has active visit
	var checkoutVisitID *int64
	var previousRoomName string
	var checkedOut bool

	// Step 6b: Process checkout if student has active visit.
	// Checkout always produces "unterwegs" state — attendance is NOT synced here.
	// Daily attendance checkout is confirmed separately via confirm_daily_checkout
	// when the student selects "nach Hause" on the device.
	if currentVisit != nil {
		var err error
		checkoutVisitID, previousRoomName, err = rs.processCheckout(ctx, w, r, student, person, currentVisit)
		if err != nil {
			return
		}
		checkedOut = true
	}

	// Step 7: Determine if checkin should be skipped (same room scenario)
	skipCheckin := shouldSkipCheckin(req.RoomID, checkedOut, currentVisit)
	if skipCheckin {
		rs.getLogger().DebugContext(ctx, "skipping re-checkin to same room",
			slog.Int64("room_id", *req.RoomID),
		)
	}

	// Step 8: Process checkin if room_id provided and not skipping
	checkinResult := rs.processStudentCheckin(ctx, w, r, student, person, &checkinProcessingInput{
		RoomID:       req.RoomID,
		DeviceID:     deviceCtx.ID,
		SkipCheckin:  skipCheckin,
		CheckedOut:   checkedOut,
		CurrentVisit: currentVisit,
	})
	if checkinResult.Error != nil {
		return
	}
	newVisitID := checkinResult.NewVisitID
	roomName := checkinResult.RoomName

	// Step 9: Check for daily checkout scenario
	result := buildCheckinResult(&checkinResultInput{
		Student:          student,
		Person:           person,
		CheckedOut:       checkedOut,
		NewVisitID:       newVisitID,
		CheckoutVisitID:  checkoutVisitID,
		RoomName:         roomName,
		PreviousRoomName: previousRoomName,
		CurrentVisit:     currentVisit,
	})
	if result.Action == "" {
		// No action occurred - shouldn't happen but handle gracefully
		rs.getLogger().WarnContext(ctx, "no action determined for student",
			slog.Int64("student_id", student.ID),
		)
		result.Action = "no_action"
		result.GreetingMsg = "Keine Aktion durchgeführt"
	}

	// Step 10: Check daily checkout with education group
	if rs.shouldUpgradeToDailyCheckout(ctx, result.Action, student, currentVisit) {
		result.Action = "checked_out_daily"
	}

	// Compute daily_checkout_available flag for frontend "nach Hause" button
	// and feedback_enabled setting — only relevant for checkout actions
	if currentVisit != nil && (result.Action == "checked_out" || result.Action == "checked_out_daily") {
		result.DailyCheckoutAvailable = rs.shouldShowDailyCheckoutWithGroup(ctx, student, currentVisit)

		// Resolve feedback_enabled so PyrePortal knows whether to show the feedback modal
		result.FeedbackEnabled = false // default: opt-in (GDPR)
		if rs.SettingsService != nil {
			if val, err := rs.SettingsService.ResolveBool(ctx, configModel.KeyFeedbackEnabled); err == nil {
				result.FeedbackEnabled = val
			}
		}
	}

	// Step 11: Keep heartbeat and active_students scoped to the scanning device session.
	if req.RoomID != nil {
		switch {
		case checkinResult.ActiveGroupID != nil && checkinResult.DeviceScopedRoom:
			rs.updateSessionActivity(ctx, *checkinResult.ActiveGroupID)
			result.ActiveStudents = rs.getActiveStudentCountForGroup(ctx, *checkinResult.ActiveGroupID)
		default:
			deviceGroup := rs.getDeviceActiveGroupInRoom(ctx, *req.RoomID, deviceCtx.ID)
			if deviceGroup != nil {
				rs.updateSessionActivity(ctx, deviceGroup.ID)
				result.ActiveStudents = rs.getActiveStudentCountForGroup(ctx, deviceGroup.ID)
			} else {
				// Preserve the legacy fallback for rooms without a device-linked active group.
				result.ActiveStudents = rs.getActiveStudentCountForRoom(ctx, *req.RoomID)
			}
		}
	}

	// Step 12: Lookup today's pickup time for the student (non-blocking: log and skip on error).
	// Runs inside the existing tenant tx for RLS visibility. A read-only SELECT on the
	// schedule tables won't abort the tx unless the DB itself is down.
	if rs.PickupScheduleService != nil {
		pickupTime, err := rs.PickupScheduleService.GetEffectivePickupTimeForDate(ctx, student.ID, now)
		if err != nil {
			rs.getLogger().WarnContext(ctx, "failed to get pickup time",
				slog.Int64("student_id", student.ID),
				slog.String("error", err.Error()),
			)
		} else if pickupTime != nil && pickupTime.PickupTime != nil {
			formatted := pickupTime.PickupTime.Format("15:04")
			result.PickupTime = &formatted
		}
	}

	// Step 13: Build and send response
	response := buildCheckinResponse(student, result, now)
	rs.getLogger().InfoContext(ctx, "checkin complete",
		slog.String("action", result.Action),
		slog.Int64("student_id", student.ID),
		slog.String("message", result.GreetingMsg),
		slog.Any("visit_id", result.VisitID),
		slog.String("room", result.RoomName),
	)

	sendCheckinResponse(w, r, response, result.Action)
}

// processBinaryModeCheckin toggles active.attendance for the student and sends
// a CheckinResponse-compatible reply. Skips the visit/room/active-group path
// entirely — binary-mode tenants don't populate those tables.
//
// Contract note for PyrePortal: the response shape matches detailed-mode's
// CheckinResponse (same field names), just with room_name empty and visit_id
// omitted. Kiosks that read those fields defensively keep working; see the
// PyrePortal issue for the UX branch (single-tap vs room-selector).
func (rs *Resource) processBinaryModeCheckin(
	w http.ResponseWriter,
	r *http.Request,
	student *users.Student,
	deviceCtx *iot.Device,
	now time.Time,
) {
	ctx := r.Context()

	// Staff ID for CheckedInBy / CheckedOutBy is carried by device auth
	// (staff PIN). No active-group lookup is needed because binary-mode
	// kiosks don't open activity sessions.
	var staffID int64
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		staffID = staffCtx.ID
	}
	if staffID == 0 {
		rs.getLogger().WarnContext(ctx, "binary mode checkin without staff context",
			slog.Int64("student_id", student.ID),
			slog.Int64("device_id", deviceCtx.ID),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("staff PIN required for binary-mode attendance toggle")))
		return
	}

	// skipAuthCheck=true: the device + staff-PIN layer already authorized
	// this scan. The default IoT path (skipAuthCheck=false) would also
	// require an active session supervisor, which doesn't exist in binary.
	result, err := rs.ActiveService.ToggleStudentAttendance(ctx, student.ID, staffID, deviceCtx.ID, true)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "binary mode attendance toggle failed",
			slog.Int64("student_id", student.ID),
			slog.Int64("staff_id", staffID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	studentName := student.Person.FirstName + " " + student.Person.LastName
	message := binaryModeGreeting(result.Action, student.Person.FirstName)

	response := map[string]interface{}{
		"student_id":               student.ID,
		"student_name":             studentName,
		"action":                   result.Action, // "checked_in" | "checked_out"
		"room_name":                "",            // binary has no rooms
		"processed_at":             now,
		"message":                  message,
		"status":                   "success",
		"daily_checkout_available": false, // daily-checkout semantics belong to the detailed flow
		"feedback_enabled":         false,
	}

	rs.getLogger().InfoContext(ctx, "binary mode checkin complete",
		slog.String("action", result.Action),
		slog.Int64("student_id", student.ID),
		slog.Int64("staff_id", staffID),
	)

	sendCheckinResponse(w, r, response, result.Action)
}

// binaryModeGreeting returns the German kiosk message shown on the confirmation
// screen. Deliberately terse — kiosks have short attention windows and the
// detailed flow's greeting helpers assume room context we don't have here.
func binaryModeGreeting(action, firstName string) string {
	switch action {
	case "checked_in":
		return "Willkommen, " + firstName + "!"
	case "checked_out":
		return "Tschüss, " + firstName + "!"
	default:
		return "Anwesenheit aktualisiert"
	}
}

func attachPickupInfoToResponse(response map[string]interface{}, effectivePickup *scheduleSvc.EffectivePickupTime) {
	if effectivePickup == nil {
		return
	}

	if effectivePickup.PickupTime != nil {
		response["pickup_time"] = effectivePickup.PickupTime.Format("15:04")
	}

	if pickupNote := selectPickupNote(effectivePickup); pickupNote != "" {
		response["pickup_note"] = pickupNote
	}
}

func selectPickupNote(effectivePickup *scheduleSvc.EffectivePickupTime) string {
	if effectivePickup == nil {
		return ""
	}

	// Day-specific notes are an override for the recurring weekday note in the kiosk flow.
	// If no usable day note exists, fall back to the recurring note text.
	if len(effectivePickup.DayNotes) > 0 {
		contents := make([]string, 0, len(effectivePickup.DayNotes))
		for _, note := range effectivePickup.DayNotes {
			if trimmed := strings.TrimSpace(note.Content); trimmed != "" {
				contents = append(contents, trimmed)
			}
		}
		if len(contents) > 0 {
			return strings.Join(contents, "\n")
		}
	}

	return strings.TrimSpace(effectivePickup.Notes)
}
