package checkin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	activeService "github.com/moto-nrw/project-phoenix/internal/core/service/active"
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

// processCheckout handles the checkout logic for a student with an active visit
// Returns: visitID, previousRoomName, error
func (rs *Resource) processCheckout(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, currentVisit *active.Visit) (*int64, string, error) {
	logger.Logger.WithFields(map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"visit_id":     currentVisit.ID,
	}).Info("[CHECKIN] Performing CHECKOUT")

	// Get previous room name
	var previousRoomName string
	if currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		previousRoomName = currentVisit.ActiveGroup.Room.Name
		logger.Logger.WithFields(map[string]interface{}{
			"previous_room": previousRoomName,
			"room_id":       currentVisit.ActiveGroup.RoomID,
		}).Debug("[CHECKIN] Previous room from active group")
	} else {
		logger.Logger.WithFields(map[string]interface{}{
			"has_active_group": currentVisit.ActiveGroup != nil,
			"has_room":         currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil,
		}).Warn("[CHECKIN] Could not get previous room name")
	}

	// End current visit with attendance sync (ensures daily checkout updates attendance record)
	if err := rs.ActiveService.EndVisit(activeService.WithAttendanceAutoSync(ctx), currentVisit.ID); err != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"visit_id":   currentVisit.ID,
			"student_id": student.ID,
		}).WithError(err).Error("[CHECKIN] Failed to end visit")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to end visit record")))
		return nil, "", err
	}

	logger.Logger.WithFields(map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"visit_id":     currentVisit.ID,
	}).Info("[CHECKIN] Student checked out successfully")

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
	logger.Logger.WithFields(map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"room_id":      roomID,
	}).Info("[CHECKIN] Performing CHECK-IN")

	// Get room information for capacity check
	room, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err != nil {
		logger.Logger.WithField("room_id", roomID).WithError(err).Error("[CHECKIN] Failed to get room")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get room information")))
		return nil, "", err
	}

	// Check room capacity if set
	if room != nil && room.Capacity != nil {
		currentOccupancy, countErr := rs.countRoomOccupancy(ctx, roomID)
		if countErr != nil {
			logger.Logger.WithField("room_id", roomID).WithError(countErr).Error("[CHECKIN] Failed to count room occupancy")
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to check room capacity")))
			return nil, "", countErr
		}

		if currentOccupancy >= *room.Capacity {
			logger.Logger.WithFields(map[string]interface{}{
				"room_id":   roomID,
				"room_name": room.Name,
				"occupancy": currentOccupancy,
				"capacity":  *room.Capacity,
			}).Error("[CHECKIN] Room is at capacity")
			iotCommon.RenderError(w, r, iotCommon.ErrorRoomCapacityExceeded(roomID, room.Name, currentOccupancy, *room.Capacity))
			return nil, "", iotCommon.ErrRoomCapacityExceeded
		}

		logger.Logger.WithFields(map[string]interface{}{
			"room_name": room.Name,
			"occupancy": currentOccupancy,
			"capacity":  *room.Capacity,
		}).Debug("[CHECKIN] Room capacity check passed")
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

	logger.Logger.WithFields(map[string]interface{}{
		"student_id":      student.ID,
		"active_group_id": activeGroupID,
	}).Debug("[CHECKIN] Creating visit")
	if err := rs.ActiveService.CreateVisit(ctx, newVisit); err != nil {
		logger.Logger.WithField("student_id", student.ID).WithError(err).Error("[CHECKIN] Failed to create visit")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create visit record")))
		return nil, "", err
	}

	logger.Logger.WithFields(map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"visit_id":     newVisit.ID,
		"room_name":    roomName,
	}).Info("[CHECKIN] Student checked in successfully")

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
			logger.Logger.WithField("active_group_id", group.ID).WithError(visitErr).Warn("[CHECKIN] Failed to count visits for active group")
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
		logger.Logger.WithField("active_group_id", activeGroupID).WithError(err).Error("[CHECKIN] Failed to get active group")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get active group")))
		return err
	}

	// Get the activity group to check MaxParticipants
	activityGroup, err := rs.ActivitiesService.GetGroup(ctx, activeGroup.GroupID)
	if err != nil {
		logger.Logger.WithField("group_id", activeGroup.GroupID).WithError(err).Error("[CHECKIN] Failed to get activity group")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get activity information")))
		return err
	}

	// Check activity capacity
	currentOccupancy, countErr := rs.countActiveGroupOccupancy(ctx, activeGroupID)
	if countErr != nil {
		logger.Logger.WithField("active_group_id", activeGroupID).WithError(countErr).Error("[CHECKIN] Failed to count activity occupancy")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to check activity capacity")))
		return countErr
	}

	if currentOccupancy >= activityGroup.MaxParticipants {
		logger.Logger.WithFields(map[string]interface{}{
			"activity_id":   activityGroup.ID,
			"activity_name": activityGroup.Name,
			"occupancy":     currentOccupancy,
			"max":           activityGroup.MaxParticipants,
		}).Error("[CHECKIN] Activity is at capacity")
		iotCommon.RenderError(w, r, iotCommon.ErrorActivityCapacityExceeded(activityGroup.ID, activityGroup.Name, currentOccupancy, activityGroup.MaxParticipants))
		return iotCommon.ErrActivityCapacityExceeded
	}

	logger.Logger.WithFields(map[string]interface{}{
		"activity_name": activityGroup.Name,
		"occupancy":     currentOccupancy,
		"max":           activityGroup.MaxParticipants,
	}).Debug("[CHECKIN] Activity capacity check passed")

	return nil
}

// findOrCreateActiveGroupForRoom finds an existing active group or creates one for Schulhof
// Returns: activeGroupID, roomName, error
func (rs *Resource) findOrCreateActiveGroupForRoom(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID int64) (int64, string, error) {
	logger.Logger.WithField("room_id", roomID).Debug("[CHECKIN] Looking for active groups in room")

	activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		logger.Logger.WithField("room_id", roomID).WithError(err).Error("[CHECKIN] Failed to find active groups in room")
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
	logger.Logger.WithFields(map[string]interface{}{
		"room_id":         roomID,
		"active_group_id": activeGroupID,
		"group_count":     len(activeGroups),
	}).Debug("[CHECKIN] Using existing active group")

	roomName := rs.roomNameByID(ctx, activeGroups[0].Room, roomID)
	return activeGroupID, roomName, nil
}

// createSchulhofActiveGroupIfNeeded creates a Schulhof active group if the room is Schulhof
func (rs *Resource) createSchulhofActiveGroupIfNeeded(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID int64) (int64, string, error) {
	room, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err != nil || room == nil || room.Name != activities.SchulhofRoomName {
		logger.Logger.WithField("room_id", roomID).Error("[CHECKIN] No active groups found in room")
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("no active groups in specified room")))
		return 0, "", errors.New("no active groups in specified room")
	}

	logger.Logger.WithField("room_id", roomID).Info("[CHECKIN] No active group in Schulhof room, auto-creating")

	schulhofActivity, err := rs.schulhofActivityGroup(ctx)
	if err != nil {
		logger.Logger.WithError(err).Error("[CHECKIN] Failed to find Schulhof activity")
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
		logger.Logger.WithError(err).Error("[CHECKIN] Failed to create Schulhof active group")
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create Schulhof session")))
		return 0, "", err
	}

	logger.Logger.WithField("active_group_id", newActiveGroup.ID).Info("[CHECKIN] Auto-created Schulhof active group")
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
		logger.Logger.Error("[CHECKIN] Room ID is required for check-in")
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
			logger.Logger.WithFields(map[string]interface{}{
				"student_name":  studentName,
				"previous_room": input.PreviousRoomName,
				"current_room":  input.RoomName,
			}).Info("[CHECKIN] Student transferred")
		} else {
			// Same room or previous room unknown
			result.Action = activeService.StatusCheckedIn
			result.GreetingMsg = "Hallo " + input.Person.FirstName + "!"
			logger.Logger.WithFields(map[string]interface{}{
				"student_name":  studentName,
				"previous_room": input.PreviousRoomName,
				"current_room":  input.RoomName,
			}).Debug("[CHECKIN] Student re-entered room")
		}
		result.VisitID = input.NewVisitID
	} else if input.CheckedOut {
		// Only checked out
		// Note: Daily checkout upgrade happens in deviceCheckin() via shouldUpgradeToDailyCheckout()
		// which has access to EducationService for room matching
		result.Action = activeService.StatusCheckedOut
		result.GreetingMsg = "Tschüss " + input.Person.FirstName + "!"
		result.VisitID = input.CheckoutVisitID
	} else if input.NewVisitID != nil {
		// Only checked in
		result.Action = activeService.StatusCheckedIn
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
				logger.Logger.WithField("group_id", group.ID).WithError(updateErr).Warn("[CHECKIN] Failed to update session activity")
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
