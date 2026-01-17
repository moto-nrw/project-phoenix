package checkin

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
)

// devicePing handles ping requests from RFID devices
// This endpoint keeps both the device AND any active session alive
func (rs *Resource) devicePing(w http.ResponseWriter, r *http.Request) {
	recordEventAction(r.Context(), "device_ping")

	// Get authenticated device from context (no staff context needed with global PIN)
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		recordEventError(r.Context(), "device_ping", "device_unauthorized", device.ErrMissingAPIKey)
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Update device last seen time (already done in middleware, but let's be explicit)
	if err := rs.IoTService.PingDevice(r.Context(), deviceCtx.DeviceID); err != nil {
		recordEventError(r.Context(), "device_ping", "device_ping_failed", err)
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Also update session activity if device has an active session
	// This keeps the session alive as long as the device is pinging
	sessionActive := false
	if session, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID); err == nil && session != nil {
		sessionActive = true // Session exists - set immediately regardless of update success
		if err := rs.ActiveService.UpdateSessionActivity(r.Context(), session.ID); err != nil {
			recordEventError(r.Context(), "device_ping", "session_activity_update_failed", err)
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
	recordEventAction(r.Context(), "device_status")

	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		recordEventError(r.Context(), "device_status", "device_unauthorized", device.ErrMissingAPIKey)
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

// deviceCheckin handles student check-in/check-out requests from RFID devices
func (rs *Resource) deviceCheckin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	recordEventAction(ctx, "check_in")

	// Step 1: Validate device context
	deviceCtx := validateDeviceContext(w, r)
	if deviceCtx == nil {
		return
	}

	// Step 2: Parse and validate request
	req := parseCheckinRequest(w, r, deviceCtx.DeviceID)
	if req == nil {
		return
	}
	if req.RoomID != nil {
		recordEventRoomID(ctx, *req.RoomID)
	}

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
	recordEventStudentID(ctx, student.ID)
	student.Person = person

	// Step 5: Load current visit with room information
	currentVisit := rs.loadCurrentVisitWithRoom(ctx, student.ID)

	// Step 6: Process checkout if student has active visit
	var checkoutVisitID *int64
	var previousRoomName string
	var checkedOut bool

	// Step 6: Check for pending daily checkout (must check BEFORE processing checkout)
	if currentVisit != nil && rs.isPendingDailyCheckoutScenario(ctx, student, currentVisit) {
		handlePendingDailyCheckoutResponse(w, r, student, person, currentVisit)
		return
	}

	// Step 6b: Process checkout if student has active visit
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
	}

	// Step 8: Process checkin if room_id provided and not skipping
	checkinResult := rs.processStudentCheckin(ctx, w, r, student, person, &checkinProcessingInput{
		RoomID:       req.RoomID,
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
		recordEventError(ctx, "checkin", "no_action", errors.New("no action determined"))
		result.Action = "no_action"
		result.GreetingMsg = "Keine Aktion durchgeführt"
	}

	// Step 10: Check daily checkout with education group
	if rs.shouldUpgradeToDailyCheckout(ctx, result.Action, student, currentVisit) {
		result.Action = "checked_out_daily"
	}

	// Step 11: Update session activity for device monitoring
	if req.RoomID != nil {
		rs.updateSessionActivityForDevice(ctx, *req.RoomID, deviceCtx.ID)
	}

	// Step 12: Build and send response
	response := buildCheckinResponse(student, result, now)
	recordEventAction(ctx, result.Action)
	sendCheckinResponse(w, r, response, result.Action)
}
