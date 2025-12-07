package iot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// checkinResult holds the result of processing a checkin request
type checkinResult struct {
	Action           string
	VisitID          *int64
	RoomName         string
	PreviousRoomName string
	GreetingMsg      string
}

// validateDeviceContext validates the device context and returns an error response if invalid
func validateDeviceContext(w http.ResponseWriter, r *http.Request) *iot.Device {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return nil
	}
	return deviceCtx
}

// parseCheckinRequest parses and validates the checkin request
func parseCheckinRequest(w http.ResponseWriter, r *http.Request, deviceID string) *CheckinRequest {
	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		log.Printf("[CHECKIN] ERROR: Invalid request from device %s: %v", deviceID, err)
		if renderErr := render.Render(w, r, ErrorInvalidRequest(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return nil
	}
	return req
}

// lookupPersonByRFID finds a person by RFID tag and validates the assignment
func (rs *Resource) lookupPersonByRFID(ctx context.Context, w http.ResponseWriter, r *http.Request, rfid string) *users.Person {
	log.Printf("[CHECKIN] Looking up RFID tag: %s", rfid)
	person, err := rs.UsersService.FindByTagID(ctx, rfid)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: RFID tag %s not found: %v", rfid, err)
		if renderErr := render.Render(w, r, ErrorNotFound(errors.New("RFID tag not found"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return nil
	}

	if person == nil || person.TagID == nil {
		log.Printf("[CHECKIN] ERROR: RFID tag %s not assigned to any person", rfid)
		if renderErr := render.Render(w, r, ErrorNotFound(errors.New("RFID tag not assigned to any person"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return nil
	}

	log.Printf("[CHECKIN] RFID tag %s belongs to person: %s %s (ID: %d)",
		rfid, person.FirstName, person.LastName, person.ID)
	return person
}

// lookupStudentFromPerson attempts to find a student from a person record
func (rs *Resource) lookupStudentFromPerson(ctx context.Context, personID int64) *users.Student {
	studentRepo := rs.UsersService.StudentRepository()
	student, _ := studentRepo.FindByPersonID(ctx, personID)
	return student
}

// handleStaffScan checks if person is staff and handles supervisor authentication
// Returns true if the request was handled (either successfully or with error)
func (rs *Resource) handleStaffScan(w http.ResponseWriter, r *http.Request, deviceCtx *iot.Device, person *users.Person) bool {
	log.Printf("[CHECKIN] Person %d is not a student, checking if staff...", person.ID)

	staffRepo := rs.UsersService.StaffRepository()
	staff, err := staffRepo.FindByPersonID(r.Context(), person.ID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to lookup staff for person %d: %v", person.ID, err)
		if renderErr := render.Render(w, r, ErrorNotFound(errors.New("RFID tag not assigned to student or staff"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return true
	}

	if staff != nil {
		log.Printf("[CHECKIN] Found staff: ID %d, routing to supervisor authentication", staff.ID)
		rs.handleSupervisorScan(w, r, deviceCtx, staff, person)
		return true
	}

	// Neither student nor staff
	log.Printf("[CHECKIN] ERROR: Person %d is neither student nor staff", person.ID)
	if renderErr := render.Render(w, r, ErrorNotFound(errors.New("RFID tag not assigned to student or staff"))); renderErr != nil {
		log.Printf("Render error: %v", renderErr)
	}
	return true
}

// loadCurrentVisitWithRoom loads the current visit and its room information
func (rs *Resource) loadCurrentVisitWithRoom(ctx context.Context, studentID int64) *active.Visit {
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		log.Printf("Error checking current visit: %v", err)
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

// processCheckout handles the checkout logic for a student with an active visit
// Returns: visitID, previousRoomName, error
func (rs *Resource) processCheckout(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, currentVisit *active.Visit) (*int64, string, error) {
	log.Printf("[CHECKIN] Student %s %s (ID: %d) has active visit %d - performing CHECKOUT",
		person.FirstName, person.LastName, student.ID, currentVisit.ID)

	// Get previous room name
	var previousRoomName string
	if currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		previousRoomName = currentVisit.ActiveGroup.Room.Name
		log.Printf("[CHECKIN] Previous room name from active group: %s (Room ID: %d)",
			previousRoomName, currentVisit.ActiveGroup.RoomID)
	} else {
		log.Printf("[CHECKIN] Warning: Could not get previous room name - ActiveGroup: %v, Room: %v",
			currentVisit.ActiveGroup != nil,
			currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil)
	}

	// End current visit
	if err := rs.ActiveService.EndVisit(ctx, currentVisit.ID); err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to end visit %d for student %d: %v",
			currentVisit.ID, student.ID, err)
		if renderErr := render.Render(w, r, ErrorInternalServer(errors.New("failed to end visit record"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return nil, "", err
	}

	// Cancel any pending scheduled checkout
	rs.cancelPendingScheduledCheckout(ctx, student.ID)

	log.Printf("[CHECKIN] SUCCESS: Checked out student %s %s (ID: %d), ended visit %d",
		person.FirstName, person.LastName, student.ID, currentVisit.ID)

	visitID := currentVisit.ID
	return &visitID, previousRoomName, nil
}

// cancelPendingScheduledCheckout cancels any pending scheduled checkout for the student
func (rs *Resource) cancelPendingScheduledCheckout(ctx context.Context, studentID int64) {
	pendingCheckout, err := rs.ActiveService.GetPendingScheduledCheckout(ctx, studentID)
	if err != nil {
		log.Printf("[CHECKIN] Warning: Failed to check for pending scheduled checkout: %v", err)
		return
	}
	if pendingCheckout == nil {
		return
	}

	// Get staff ID from context if available
	var cancelledBy int64 = 1 // Default to admin ID
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		cancelledBy = staffCtx.ID
	}

	if err := rs.ActiveService.CancelScheduledCheckout(ctx, pendingCheckout.ID, cancelledBy); err != nil {
		log.Printf("[CHECKIN] Warning: Failed to cancel scheduled checkout %d: %v", pendingCheckout.ID, err)
	} else {
		log.Printf("[CHECKIN] Cancelled pending scheduled checkout %d for student %d", pendingCheckout.ID, studentID)
	}
}

// shouldSkipCheckin determines if checkin should be skipped (same room scenario)
func shouldSkipCheckin(roomID *int64, checkedOut bool, currentVisit *active.Visit) bool {
	if roomID == nil || !checkedOut || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}
	return currentVisit.ActiveGroup.RoomID == *roomID
}

// processCheckin handles the checkin logic for a student
// Returns: visitID, roomName, error
func (rs *Resource) processCheckin(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, roomID int64) (*int64, string, error) {
	log.Printf("[CHECKIN] Student %s %s (ID: %d) - performing CHECK-IN to room %d",
		person.FirstName, person.LastName, student.ID, roomID)

	// Find or create active group for the room
	activeGroupID, roomName, err := rs.findOrCreateActiveGroupForRoom(ctx, w, r, roomID)
	if err != nil {
		return nil, "", err
	}

	// Create new visit
	newVisit := &active.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroupID,
		EntryTime:     time.Now(),
	}

	log.Printf("[CHECKIN] Creating visit for student %d in active group %d", student.ID, activeGroupID)
	if err := rs.ActiveService.CreateVisit(ctx, newVisit); err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to create visit for student %d: %v", student.ID, err)
		if renderErr := render.Render(w, r, ErrorInternalServer(errors.New("failed to create visit record"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return nil, "", err
	}

	log.Printf("[CHECKIN] SUCCESS: Checked in student %s %s (ID: %d), created visit %d in room %s",
		person.FirstName, person.LastName, student.ID, newVisit.ID, roomName)

	return &newVisit.ID, roomName, nil
}

// findOrCreateActiveGroupForRoom finds an existing active group or creates one for Schulhof
// Returns: activeGroupID, roomName, error
func (rs *Resource) findOrCreateActiveGroupForRoom(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID int64) (int64, string, error) {
	log.Printf("[CHECKIN] Looking for active groups in room %d", roomID)

	activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to find active groups in room %d: %v", roomID, err)
		if renderErr := render.Render(w, r, ErrorInternalServer(errors.New("error finding active groups in room"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return 0, "", err
	}

	if len(activeGroups) > 0 {
		return rs.useExistingActiveGroup(ctx, activeGroups, roomID)
	}

	// No active groups - check if this is Schulhof
	return rs.createSchulhofActiveGroupIfNeeded(ctx, w, r, roomID)
}

// useExistingActiveGroup uses an existing active group in the room
func (rs *Resource) useExistingActiveGroup(ctx context.Context, activeGroups []*active.Group, roomID int64) (int64, string, error) {
	activeGroupID := activeGroups[0].ID
	log.Printf("[CHECKIN] Found %d active groups in room %d, using group %d",
		len(activeGroups), roomID, activeGroupID)

	roomName := rs.getRoomName(ctx, activeGroups[0].Room, roomID)
	return activeGroupID, roomName, nil
}

// createSchulhofActiveGroupIfNeeded creates a Schulhof active group if the room is Schulhof
func (rs *Resource) createSchulhofActiveGroupIfNeeded(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID int64) (int64, string, error) {
	room, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err != nil || room == nil || room.Name != constants.SchulhofRoomName {
		log.Printf("[CHECKIN] ERROR: No active groups found in room %d", roomID)
		if renderErr := render.Render(w, r, ErrorNotFound(errors.New("no active groups in specified room"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return 0, "", errors.New("no active groups in specified room")
	}

	log.Printf("[CHECKIN] No active group in Schulhof room %d, auto-creating...", roomID)

	schulhofActivity, err := rs.schulhofActivityGroup(ctx)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to find Schulhof activity: %v", err)
		if renderErr := render.Render(w, r, ErrorInternalServer(errors.New("schulhof activity not configured"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return 0, "", err
	}

	newActiveGroup := &active.Group{
		GroupID:      schulhofActivity.ID,
		RoomID:       roomID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}

	if err := rs.ActiveService.CreateActiveGroup(ctx, newActiveGroup); err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to create Schulhof active group: %v", err)
		if renderErr := render.Render(w, r, ErrorInternalServer(errors.New("failed to create Schulhof session"))); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return 0, "", err
	}

	log.Printf("[CHECKIN] SUCCESS: Auto-created Schulhof active group %d", newActiveGroup.ID)
	return newActiveGroup.ID, room.Name, nil
}

// getRoomName gets the room name from the room object or by ID lookup
func (rs *Resource) getRoomName(ctx context.Context, room *facilities.Room, roomID int64) string {
	if room != nil {
		return room.Name
	}

	loadedRoom, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err == nil && loadedRoom != nil {
		return loadedRoom.Name
	}

	return fmt.Sprintf("Room %d", roomID)
}

// getRoomNameForCheckinResponse gets the room name for the checkin response
func (rs *Resource) getRoomNameForCheckinResponse(ctx context.Context, currentVisit *active.Visit, roomID *int64) string {
	if currentVisit != nil && currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		return currentVisit.ActiveGroup.Room.Name
	}
	if roomID != nil {
		return rs.getRoomName(ctx, nil, *roomID)
	}
	return ""
}

// buildCheckinResult builds the result message based on what actions occurred
func buildCheckinResult(student *users.Student, person *users.Person, checkedOut bool, newVisitID *int64, checkoutVisitID *int64, roomName, previousRoomName string, currentVisit *active.Visit) *checkinResult {
	studentName := person.FirstName + " " + person.LastName
	result := &checkinResult{}

	if checkedOut && newVisitID != nil {
		// Student checked out and checked in
		if previousRoomName != "" && previousRoomName != roomName {
			// Actual room transfer
			result.Action = "transferred"
			result.GreetingMsg = fmt.Sprintf("Gewechselt von %s zu %s!", previousRoomName, roomName)
			log.Printf("[CHECKIN] Student %s transferred from %s to %s", studentName, previousRoomName, roomName)
		} else {
			// Same room or previous room unknown
			result.Action = "checked_in"
			result.GreetingMsg = "Hallo " + person.FirstName + "!"
			log.Printf("[CHECKIN] Student %s re-entered room (previous: '%s', current: '%s')",
				studentName, previousRoomName, roomName)
		}
		result.VisitID = newVisitID
	} else if checkedOut {
		// Only checked out
		result.Action = "checked_out"
		result.GreetingMsg = "Tschüss " + person.FirstName + "!"
		result.VisitID = checkoutVisitID

		// Check for daily checkout
		if shouldShowDailyCheckout(student, currentVisit) {
			result.Action = "checked_out_daily"
		}
	} else if newVisitID != nil {
		// Only checked in
		result.Action = "checked_in"
		result.GreetingMsg = "Hallo " + person.FirstName + "!"
		result.VisitID = newVisitID
	}

	result.RoomName = roomName
	result.PreviousRoomName = previousRoomName
	return result
}

// shouldShowDailyCheckout checks if daily checkout message should be shown
func shouldShowDailyCheckout(student *users.Student, currentVisit *active.Visit) bool {
	if student.GroupID == nil || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}

	checkoutTime, err := getStudentDailyCheckoutTime()
	if err != nil || !time.Now().After(checkoutTime) {
		return false
	}

	// Would need to check education group room match
	// Simplified for now - full implementation would fetch education group
	return false
}

// updateSessionActivityForDevice updates session activity when student scans
func (rs *Resource) updateSessionActivityForDevice(ctx context.Context, roomID int64, deviceID int64) {
	activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return
	}

	for _, group := range activeGroups {
		if group.DeviceID != nil && *group.DeviceID == deviceID {
			if updateErr := rs.ActiveService.UpdateSessionActivity(ctx, group.ID); updateErr != nil {
				log.Printf("Warning: Failed to update session activity for group %d: %v", group.ID, updateErr)
			}
			break
		}
	}
}

// buildCheckinResponse builds the final checkin response map
func buildCheckinResponse(student *users.Student, result *checkinResult, now time.Time) map[string]interface{} {
	studentName := student.Person.FirstName + " " + student.Person.LastName

	response := map[string]interface{}{
		"student_id":   student.ID,
		"student_name": studentName,
		"action":       result.Action,
		"visit_id":     result.VisitID,
		"room_name":    result.RoomName,
		"processed_at": now,
		"message":      result.GreetingMsg,
		"status":       "success",
	}

	if result.Action == "transferred" && result.PreviousRoomName != "" {
		response["previous_room"] = result.PreviousRoomName
	}

	return response
}

// sendCheckinResponse sends the final response
func sendCheckinResponse(w http.ResponseWriter, r *http.Request, response map[string]interface{}, action string) {
	common.Respond(w, r, http.StatusOK, response, "Student "+action+" successfully")
}
