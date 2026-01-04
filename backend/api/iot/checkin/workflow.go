package checkin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// checkinResult holds the result of processing a checkin request
type checkinResult struct {
	Action           string
	VisitID          *int64
	RoomName         string
	PreviousRoomName string
	GreetingMsg      string
}

// checkinResultInput holds the input parameters for building a checkin result.
// This struct reduces the parameter count of buildCheckinResult for better maintainability.
type checkinResultInput struct {
	Student          *users.Student
	Person           *users.Person
	CheckedOut       bool
	NewVisitID       *int64
	CheckoutVisitID  *int64
	RoomName         string
	PreviousRoomName string
	CurrentVisit     *active.Visit
}

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
		log.Printf("[CHECKIN] ERROR: Invalid request from device %s: %v", deviceID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
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
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return nil
	}

	if person == nil || person.TagID == nil {
		log.Printf("[CHECKIN] ERROR: RFID tag %s not assigned to any person", rfid)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil
	}

	log.Printf("[CHECKIN] RFID tag %s belongs to person: %s %s (ID: %d)",
		rfid, person.FirstName, person.LastName, person.ID)
	return person
}

// lookupStudentFromPerson attempts to find a student from a person record.
// Returns nil if person is not a student or if lookup fails (errors are logged).
func (rs *Resource) lookupStudentFromPerson(ctx context.Context, personID int64) *users.Student {
	studentRepo := rs.UsersService.StudentRepository()
	student, err := studentRepo.FindByPersonID(ctx, personID)
	if err != nil {
		// Log error but continue - person may be staff instead of student
		log.Printf("[CHECKIN] Student lookup for person %d: %v", personID, err)
		return nil
	}
	return student
}

// handleStaffScan checks if person is staff and handles supervisor authentication
// Returns true if the request was handled (either successfully or with error)
func (rs *Resource) handleStaffScan(w http.ResponseWriter, r *http.Request, _ *iot.Device, person *users.Person) bool {
	log.Printf("[CHECKIN] Person %d is not a student, checking if staff...", person.ID)

	staffRepo := rs.UsersService.StaffRepository()
	staff, err := staffRepo.FindByPersonID(r.Context(), person.ID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to lookup staff for person %d: %v", person.ID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
		return true
	}

	if staff != nil {
		log.Printf("[CHECKIN] Found staff: ID %d - staff RFID authentication via checkin endpoint not supported", staff.ID)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("staff RFID authentication must be done via session management endpoints")))
		return true
	}

	// Neither student nor staff
	log.Printf("[CHECKIN] ERROR: Person %d is neither student nor staff", person.ID)
	iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
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

	// End current visit with attendance sync (ensures daily checkout updates attendance record)
	if err := rs.ActiveService.EndVisit(activeService.WithAttendanceAutoSync(ctx), currentVisit.ID); err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to end visit %d for student %d: %v",
			currentVisit.ID, student.ID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to end visit record")))
		return nil, "", err
	}

	log.Printf("[CHECKIN] SUCCESS: Checked out student %s %s (ID: %d), ended visit %d",
		person.FirstName, person.LastName, student.ID, currentVisit.ID)

	visitID := currentVisit.ID
	return &visitID, previousRoomName, nil
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

	// Get room information for capacity check
	room, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to get room %d: %v", roomID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get room information")))
		return nil, "", err
	}

	// Check room capacity if set
	if room != nil && room.Capacity != nil {
		currentOccupancy, countErr := rs.countRoomOccupancy(ctx, roomID)
		if countErr != nil {
			log.Printf("[CHECKIN] ERROR: Failed to count room occupancy for room %d: %v", roomID, countErr)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to check room capacity")))
			return nil, "", countErr
		}

		if currentOccupancy >= *room.Capacity {
			log.Printf("[CHECKIN] ERROR: Room %s (ID: %d) is at capacity: %d/%d",
				room.Name, roomID, currentOccupancy, *room.Capacity)
			iotCommon.RenderError(w, r, iotCommon.ErrorRoomCapacityExceeded(roomID, room.Name, currentOccupancy, *room.Capacity))
			return nil, "", iotCommon.ErrRoomCapacityExceeded
		}

		log.Printf("[CHECKIN] Room %s capacity check passed: %d/%d",
			room.Name, currentOccupancy, *room.Capacity)
	}

	// Find or create active group for the room
	activeGroupID, roomName, err := rs.findOrCreateActiveGroupForRoom(ctx, w, r, roomID)
	if err != nil {
		return nil, "", err
	}

	// Check activity capacity
	if capacityErr := rs.checkActivityCapacity(ctx, w, r, activeGroupID); capacityErr != nil {
		return nil, "", capacityErr
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
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create visit record")))
		return nil, "", err
	}

	log.Printf("[CHECKIN] SUCCESS: Checked in student %s %s (ID: %d), created visit %d in room %s",
		person.FirstName, person.LastName, student.ID, newVisit.ID, roomName)

	return &newVisit.ID, roomName, nil
}

// countRoomOccupancy counts the number of active visits in a room.
// Active visits are those where exit_time IS NULL in active groups that are in the specified room.
func (rs *Resource) countRoomOccupancy(ctx context.Context, roomID int64) (int, error) {
	// Find all active groups in the room
	activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return 0, fmt.Errorf("failed to find active groups in room %d: %w", roomID, err)
	}

	if len(activeGroups) == 0 {
		return 0, nil
	}

	// Count active visits across all active groups in this room
	totalOccupancy := 0
	for _, group := range activeGroups {
		visits, visitErr := rs.ActiveService.FindVisitsByActiveGroupID(ctx, group.ID)
		if visitErr != nil {
			log.Printf("[CHECKIN] Warning: Failed to count visits for active group %d: %v", group.ID, visitErr)
			continue
		}

		// Count only active visits (exit_time IS NULL)
		for _, visit := range visits {
			if visit.ExitTime == nil {
				totalOccupancy++
			}
		}
	}

	return totalOccupancy, nil
}

// countActiveGroupOccupancy counts the number of active visits in a specific active group.
// Active visits are those where exit_time IS NULL.
func (rs *Resource) countActiveGroupOccupancy(ctx context.Context, activeGroupID int64) (int, error) {
	visits, err := rs.ActiveService.FindVisitsByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return 0, fmt.Errorf("failed to find visits for active group %d: %w", activeGroupID, err)
	}

	// Count only active visits (exit_time IS NULL)
	count := 0
	for _, visit := range visits {
		if visit.ExitTime == nil {
			count++
		}
	}

	return count, nil
}

// checkActivityCapacity validates that the activity has capacity for another student.
// Returns nil if capacity is available, error otherwise.
func (rs *Resource) checkActivityCapacity(ctx context.Context, w http.ResponseWriter, r *http.Request, activeGroupID int64) error {
	// Get the active group to find the activity group ID
	activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, activeGroupID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to get active group %d: %v", activeGroupID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get active group")))
		return err
	}

	// Get the activity group to check MaxParticipants
	activityGroup, err := rs.ActivitiesService.GetGroup(ctx, activeGroup.GroupID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to get activity group %d: %v", activeGroup.GroupID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get activity information")))
		return err
	}

	// Check activity capacity
	currentOccupancy, countErr := rs.countActiveGroupOccupancy(ctx, activeGroupID)
	if countErr != nil {
		log.Printf("[CHECKIN] ERROR: Failed to count activity occupancy for active group %d: %v", activeGroupID, countErr)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to check activity capacity")))
		return countErr
	}

	if currentOccupancy >= activityGroup.MaxParticipants {
		log.Printf("[CHECKIN] ERROR: Activity %s (ID: %d) is at capacity: %d/%d",
			activityGroup.Name, activityGroup.ID, currentOccupancy, activityGroup.MaxParticipants)
		iotCommon.RenderError(w, r, iotCommon.ErrorActivityCapacityExceeded(activityGroup.ID, activityGroup.Name, currentOccupancy, activityGroup.MaxParticipants))
		return iotCommon.ErrActivityCapacityExceeded
	}

	log.Printf("[CHECKIN] Activity %s capacity check passed: %d/%d",
		activityGroup.Name, currentOccupancy, activityGroup.MaxParticipants)

	return nil
}

// findOrCreateActiveGroupForRoom finds an existing active group or creates one for Schulhof
// Returns: activeGroupID, roomName, error
func (rs *Resource) findOrCreateActiveGroupForRoom(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID int64) (int64, string, error) {
	log.Printf("[CHECKIN] Looking for active groups in room %d", roomID)

	activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to find active groups in room %d: %v", roomID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("error finding active groups in room")))
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

	roomName := rs.roomNameByID(ctx, activeGroups[0].Room, roomID)
	return activeGroupID, roomName, nil
}

// createSchulhofActiveGroupIfNeeded creates a Schulhof active group if the room is Schulhof
func (rs *Resource) createSchulhofActiveGroupIfNeeded(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID int64) (int64, string, error) {
	room, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err != nil || room == nil || room.Name != constants.SchulhofRoomName {
		log.Printf("[CHECKIN] ERROR: No active groups found in room %d", roomID)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("no active groups in specified room")))
		return 0, "", errors.New("no active groups in specified room")
	}

	log.Printf("[CHECKIN] No active group in Schulhof room %d, auto-creating...", roomID)

	schulhofActivity, err := rs.schulhofActivityGroup(ctx)
	if err != nil {
		log.Printf("[CHECKIN] ERROR: Failed to find Schulhof activity: %v", err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("schulhof activity not configured")))
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
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create Schulhof session")))
		return 0, "", err
	}

	log.Printf("[CHECKIN] SUCCESS: Auto-created Schulhof active group %d", newActiveGroup.ID)
	return newActiveGroup.ID, room.Name, nil
}

// roomNameByID resolves the room name from a room object or by ID lookup
func (rs *Resource) roomNameByID(ctx context.Context, room *facilities.Room, roomID int64) string {
	if room != nil {
		return room.Name
	}

	loadedRoom, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err == nil && loadedRoom != nil {
		return loadedRoom.Name
	}

	return fmt.Sprintf("Room %d", roomID)
}

// roomNameForResponse resolves the room name for a checkin response
func (rs *Resource) roomNameForResponse(ctx context.Context, currentVisit *active.Visit, roomID *int64) string {
	if currentVisit != nil && currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		return currentVisit.ActiveGroup.Room.Name
	}
	if roomID != nil {
		return rs.roomNameByID(ctx, nil, *roomID)
	}
	return ""
}

// checkinProcessingInput holds the inputs for processing a student checkin
type checkinProcessingInput struct {
	RoomID       *int64
	SkipCheckin  bool
	CheckedOut   bool
	CurrentVisit *active.Visit
}

// checkinProcessingResult holds the result of checkin processing
type checkinProcessingResult struct {
	NewVisitID *int64
	RoomName   string
	Error      error
}

// processStudentCheckin handles the checkin logic based on room and skip conditions.
// This helper reduces cognitive complexity by encapsulating the branching logic.
func (rs *Resource) processStudentCheckin(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, input *checkinProcessingInput) *checkinProcessingResult {
	result := &checkinProcessingResult{}

	switch {
	case input.RoomID != nil && !input.SkipCheckin:
		// Normal checkin case
		visitID, roomName, err := rs.processCheckin(ctx, w, r, student, person, *input.RoomID)
		if err != nil {
			result.Error = err
			return result
		}
		result.NewVisitID = visitID
		result.RoomName = roomName

	case input.RoomID != nil && input.SkipCheckin:
		// Skipped checkin - just get room name for response
		result.RoomName = rs.roomNameForResponse(ctx, input.CurrentVisit, input.RoomID)

	case !input.CheckedOut:
		// No room_id provided and no previous checkout - error
		log.Printf("[CHECKIN] ERROR: Room ID is required for check-in")
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("room_id is required for check-in")))
		result.Error = errors.New("room_id is required for check-in")
	}

	return result
}

// buildCheckinResult builds the result message based on what actions occurred
func buildCheckinResult(input *checkinResultInput) *checkinResult {
	studentName := input.Person.FirstName + " " + input.Person.LastName
	result := &checkinResult{}

	if input.CheckedOut && input.NewVisitID != nil {
		// Student checked out and checked in
		if input.PreviousRoomName != "" && input.PreviousRoomName != input.RoomName {
			// Actual room transfer
			result.Action = "transferred"
			result.GreetingMsg = fmt.Sprintf("Gewechselt von %s zu %s!", input.PreviousRoomName, input.RoomName)
			log.Printf("[CHECKIN] Student %s transferred from %s to %s", studentName, input.PreviousRoomName, input.RoomName)
		} else {
			// Same room or previous room unknown
			result.Action = "checked_in"
			result.GreetingMsg = "Hallo " + input.Person.FirstName + "!"
			log.Printf("[CHECKIN] Student %s re-entered room (previous: '%s', current: '%s')",
				studentName, input.PreviousRoomName, input.RoomName)
		}
		result.VisitID = input.NewVisitID
	} else if input.CheckedOut {
		// Only checked out
		// Note: Daily checkout upgrade happens in deviceCheckin() via shouldUpgradeToDailyCheckout()
		// which has access to EducationService for room matching
		result.Action = "checked_out"
		result.GreetingMsg = "Tschüss " + input.Person.FirstName + "!"
		result.VisitID = input.CheckoutVisitID
	} else if input.NewVisitID != nil {
		// Only checked in
		result.Action = "checked_in"
		result.GreetingMsg = "Hallo " + input.Person.FirstName + "!"
		result.VisitID = input.NewVisitID
	}

	result.RoomName = input.RoomName
	result.PreviousRoomName = input.PreviousRoomName
	return result
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
