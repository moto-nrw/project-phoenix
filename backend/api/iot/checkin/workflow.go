package checkin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// checkinResult holds the result of processing a checkin request
type checkinResult struct {
	Action                 string
	VisitID                *int64
	RoomName               string
	PreviousRoomName       string
	GreetingMsg            string
	DailyCheckoutAvailable bool
	FeedbackEnabled        bool
	ActiveStudents         *int
	PickupTime             *string
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

type selectedActiveGroup struct {
	Group        *active.Group
	RoomName     string
	DeviceScoped bool
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
func parseCheckinRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, logger *slog.Logger, deviceID string) *CheckinRequest {
	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		logger.ErrorContext(ctx, "invalid checkin request",
			slog.String("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return nil
	}
	return req
}

// lookupPersonByRFID finds a person by RFID tag and validates the assignment
func (rs *Resource) lookupPersonByRFID(ctx context.Context, w http.ResponseWriter, r *http.Request, rfid string) *users.Person {
	rs.getLogger().DebugContext(ctx, "looking up RFID tag", slog.String("rfid", rfid))
	person, err := rs.resolvePersonByRFID(ctx, rfid)
	if err != nil {
		rs.getLogger().WarnContext(ctx, "RFID tag not found",
			slog.String("rfid", rfid),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New(iotCommon.ErrMsgRFIDTagNotFound)))
		return nil
	}

	if person == nil || person.TagID == nil {
		rs.getLogger().WarnContext(ctx, "RFID tag not assigned to any person",
			slog.String("rfid", rfid),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to any person")))
		return nil
	}

	rs.getLogger().DebugContext(ctx, "RFID tag belongs to person",
		slog.String("rfid", rfid),
		slog.String("person_name", person.FirstName+" "+person.LastName),
		slog.Int64("person_id", person.ID),
	)
	return person
}

// resolvePersonByRFID finds a person by RFID without rendering an HTTP response.
// It preserves ErrPersonNotFound so callers can distinguish "unknown tag" from backend failures.
func (rs *Resource) resolvePersonByRFID(ctx context.Context, rfid string) (*users.Person, error) {
	person, err := rs.UsersService.FindByTagID(ctx, rfid)
	if err != nil {
		return nil, err
	}
	if person == nil {
		return nil, usersSvc.ErrPersonNotFound
	}
	return person, nil
}

// lookupStudentFromPerson attempts to find a student from a person record.
// Returns nil if person is not a student or if lookup fails (errors are logged).
func (rs *Resource) lookupStudentFromPerson(ctx context.Context, personID int64) *users.Student {
	student, err := rs.resolveStudentFromPerson(ctx, personID)
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

// resolveStudentFromPerson finds a student from a person record without rendering an HTTP response.
// A missing student returns (nil, nil); repository failures are returned to the caller.
func (rs *Resource) resolveStudentFromPerson(ctx context.Context, personID int64) (*users.Student, error) {
	studentRepo := rs.UsersService.StudentRepository()
	student, err := studentRepo.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return student, nil
}

// resolveStaffFromPerson finds a staff record from a person record without rendering an HTTP response.
// A missing staff member returns (nil, nil); repository failures are returned to the caller.
func (rs *Resource) resolveStaffFromPerson(ctx context.Context, personID int64) (*users.Staff, error) {
	staffRepo := rs.UsersService.StaffRepository()
	staff, err := staffRepo.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return staff, nil
}

// handleStaffScan checks if person is staff and handles supervisor authentication
// Returns true if the request was handled (either successfully or with error)
func (rs *Resource) handleStaffScan(w http.ResponseWriter, r *http.Request, deviceCtx *iot.Device, person *users.Person) bool {
	rs.getLogger().DebugContext(r.Context(), "person is not a student, checking if staff",
		slog.Int64("person_id", person.ID),
	)

	staffRepo := rs.UsersService.StaffRepository()
	staff, err := staffRepo.FindByPersonID(r.Context(), person.ID)
	if err != nil {
		rs.getLogger().ErrorContext(r.Context(), "failed to lookup staff for person",
			slog.Int64("person_id", person.ID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
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
	iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("RFID tag not assigned to student or staff")))
	return true
}

// handleSupervisorScan processes RFID-based supervisor authentication for staff.
// When staff scan their RFID at a device with an active session, they are
// automatically added as supervisors to that session.
func (rs *Resource) handleSupervisorScan(w http.ResponseWriter, r *http.Request, deviceCtx *iot.Device, staff *users.Staff, person *users.Person) {
	ctx := r.Context()

	// Step 1: Get current session for this device
	session, err := rs.ActiveService.GetDeviceCurrentSession(ctx, deviceCtx.ID)
	if err != nil || session == nil {
		rs.getLogger().InfoContext(ctx, "no active session for supervisor scan",
			slog.String("device_id", deviceCtx.DeviceID),
			slog.Int64("device_db_id", deviceCtx.ID),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(
			errors.New("no active session - please start an activity first")))
		return
	}

	rs.getLogger().InfoContext(ctx, "processing supervisor authentication",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int64("session_id", session.ID),
		slog.Int64("staff_id", staff.ID),
	)

	// Step 2: Load current supervisors to check for duplicates
	groupWithSupervisors, err := rs.ActiveService.GetActiveGroupWithSupervisors(ctx, session.ID)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to load supervisors for session",
			slog.Int64("session_id", session.ID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServerWrap("failed to load session supervisors", err))
		return
	}

	// Step 3: Build supervisor ID list (existing active + new)
	var supervisorIDs []int64
	alreadySupervisor := false
	for _, sup := range groupWithSupervisors.Supervisors {
		if sup.EndDate == nil { // Only include active supervisors
			supervisorIDs = append(supervisorIDs, sup.StaffID)
			if sup.StaffID == staff.ID {
				alreadySupervisor = true
			}
		}
	}

	// Step 4: Add new supervisor if not already present
	if !alreadySupervisor {
		supervisorIDs = append(supervisorIDs, staff.ID)
		if _, err := rs.ActiveService.UpdateActiveGroupSupervisors(ctx, session.ID, supervisorIDs); err != nil {
			rs.getLogger().ErrorContext(ctx, "failed to add supervisor to session",
				slog.Int64("staff_id", staff.ID),
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServerWrap("failed to update session supervisors", err))
			return
		}
		rs.getLogger().InfoContext(ctx, "added staff as supervisor to session",
			slog.Int64("staff_id", staff.ID),
			slog.Int64("session_id", session.ID),
		)
	} else {
		rs.getLogger().DebugContext(ctx, "staff is already a supervisor (idempotent)",
			slog.Int64("staff_id", staff.ID),
			slog.Int64("session_id", session.ID),
		)
	}

	// Step 5: Build and send response
	staffName := person.FirstName + " " + person.LastName
	var roomName, activityName string
	if session.Room != nil {
		roomName = session.Room.Name
	}
	if session.ActualGroup != nil {
		activityName = session.ActualGroup.Name
	}

	message := "Supervisor authenticated"
	if activityName != "" {
		message = "Supervisor authenticated for " + activityName
	}

	response := map[string]interface{}{
		"student_id":   staff.ID,
		"student_name": staffName,
		"action":       "supervisor_authenticated",
		"room_name":    roomName,
		"processed_at": time.Now(),
		"message":      message,
		"status":       "success",
	}

	common.Respond(w, r, http.StatusOK, response, "Supervisor authenticated")
}

// loadCurrentVisitWithRoom loads the current visit and its room information
func (rs *Resource) loadCurrentVisitWithRoom(ctx context.Context, studentID int64) *active.Visit {
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisitWithRoom(ctx, studentID)
	if err != nil {
		rs.getLogger().DebugContext(ctx, "error checking current visit",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if currentVisit == nil || currentVisit.ExitTime != nil {
		return nil
	}

	return currentVisit
}

// buildStudentAlreadyActiveResponse builds the 409 Conflict response body
// for a duplicate-checkin attempt. It best-effort-loads the existing visit
// so the kiosk can show "Bereits angemeldet in Raum X". The lookup is
// deliberately tolerant: if it fails (race window between INSERT failure
// and the response build, or the visit was just closed by another path),
// we return the response with only StudentID set rather than upgrading the
// 409 to a 500. The kiosk degrades to a generic "already checked in"
// message in that case, which is still strictly better than the old 500.
func (rs *Resource) buildStudentAlreadyActiveResponse(ctx context.Context, studentID int64) render.Renderer {
	existing := rs.loadCurrentVisitWithRoom(ctx, studentID)
	if existing == nil {
		return iotCommon.ErrorStudentAlreadyActive(studentID, 0, nil, nil, "")
	}

	var roomID *int64
	var roomName string
	if existing.ActiveGroup != nil {
		rid := existing.ActiveGroup.RoomID
		roomID = &rid
		if existing.ActiveGroup.Room != nil {
			roomName = existing.ActiveGroup.Room.Name
		}
	}
	entryTime := existing.EntryTime
	return iotCommon.ErrorStudentAlreadyActive(studentID, existing.ID, &entryTime, roomID, roomName)
}

// processCheckout handles the checkout logic for a student with an active visit
// Returns: visitID, previousRoomName, error
func (rs *Resource) processCheckout(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, currentVisit *active.Visit) (*int64, string, error) {
	rs.getLogger().DebugContext(ctx, "student has active visit, performing checkout",
		slog.String("student_name", person.FirstName+" "+person.LastName),
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", currentVisit.ID),
	)

	// Get previous room name
	var previousRoomName string
	if currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		previousRoomName = currentVisit.ActiveGroup.Room.Name
		rs.getLogger().DebugContext(ctx, "previous room from active group",
			slog.String("room_name", previousRoomName),
			slog.Int64("room_id", currentVisit.ActiveGroup.RoomID),
		)
	} else {
		rs.getLogger().WarnContext(ctx, "could not get previous room name",
			slog.Bool("has_active_group", currentVisit.ActiveGroup != nil),
			slog.Bool("has_room", currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil),
		)
	}

	// End current room visit WITHOUT attendance sync - leaving a room doesn't mean leaving the building.
	// The student should become "Unterwegs" (in transit), not "Zuhause" (at home).
	// Daily attendance checkout is handled via the confirm_daily_checkout action from the frontend.
	if err := rs.ActiveService.EndVisit(ctx, currentVisit.ID); err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to end visit",
			slog.Int64("visit_id", currentVisit.ID),
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to end visit record")))
		return nil, "", err
	}

	rs.getLogger().InfoContext(ctx, "checked out student",
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", currentVisit.ID),
	)

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

// processCheckin handles the checkin logic for a student.
// Returns: visitID, roomName, activeGroupID, error
func (rs *Resource) processCheckin(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, roomID int64, deviceID int64) (*int64, *selectedActiveGroup, error) {
	rs.getLogger().DebugContext(ctx, "performing check-in to room",
		slog.String("student_name", person.FirstName+" "+person.LastName),
		slog.Int64("student_id", student.ID),
		slog.Int64("room_id", roomID),
	)

	// Get room information for capacity check
	room, err := rs.FacilityService.GetRoom(ctx, roomID)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to get room",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get room information")))
		return nil, nil, err
	}

	// Check room capacity if set
	if room != nil && room.Capacity != nil {
		currentOccupancy, countErr := rs.ActiveService.CountActiveVisitsByRoomID(ctx, roomID)
		if countErr != nil {
			rs.getLogger().ErrorContext(ctx, "failed to count room occupancy",
				slog.Int64("room_id", roomID),
				slog.String("error", countErr.Error()),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to check room capacity")))
			return nil, nil, countErr
		}

		if currentOccupancy >= *room.Capacity {
			rs.getLogger().WarnContext(ctx, "room is at capacity",
				slog.String("room_name", room.Name),
				slog.Int64("room_id", roomID),
				slog.Int("current_occupancy", currentOccupancy),
				slog.Int("capacity", *room.Capacity),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorRoomCapacityExceeded(roomID, room.Name, currentOccupancy, *room.Capacity))
			return nil, nil, iotCommon.ErrRoomCapacityExceeded
		}

		rs.getLogger().DebugContext(ctx, "room capacity check passed",
			slog.String("room_name", room.Name),
			slog.Int("current_occupancy", currentOccupancy),
			slog.Int("capacity", *room.Capacity),
		)
	}

	// Find or create active group for the room
	selection, err := rs.findOrCreateActiveGroupForRoom(ctx, w, r, room, deviceID)
	if err != nil {
		return nil, nil, err
	}

	// Check activity capacity
	if capacityErr := rs.checkActivityCapacity(ctx, w, r, selection.Group); capacityErr != nil {
		return nil, nil, capacityErr
	}

	// Create new visit
	newVisit := &active.Visit{
		StudentID:     student.ID,
		ActiveGroupID: selection.Group.ID,
		EntryTime:     time.Now(),
	}

	rs.getLogger().DebugContext(ctx, "creating visit for student",
		slog.Int64("student_id", student.ID),
		slog.Int64("active_group_id", selection.Group.ID),
	)
	if err := rs.ActiveService.CreateVisit(ctx, newVisit); err != nil {
		// Duplicate active visit — race between two concurrent scans, or a
		// previous checkout that didn't fully commit. Surface as 409 with
		// the existing visit details so the kiosk can show "Bereits
		// angemeldet in Raum X" instead of a generic error. The DB-level
		// guard is migration 1.15.47's partial unique index; the
		// application-level guard is ensureStudentHasNoActiveVisit in the
		// active service. Either path translates to ErrStudentAlreadyActive.
		if errors.Is(err, activeSvc.ErrStudentAlreadyActive) {
			rs.getLogger().InfoContext(ctx, "duplicate active visit rejected",
				slog.Int64("student_id", student.ID),
				slog.String("error", err.Error()),
			)
			iotCommon.RenderError(w, r, rs.buildStudentAlreadyActiveResponse(ctx, student.ID))
			return nil, nil, err
		}
		rs.getLogger().ErrorContext(ctx, "failed to create visit",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create visit record")))
		return nil, nil, err
	}

	rs.getLogger().InfoContext(ctx, "checked in student",
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", newVisit.ID),
		slog.String("room", selection.RoomName),
	)

	return &newVisit.ID, selection, nil
}

// countActiveGroupOccupancy counts the number of active visits in a specific active group.
// Active visits are those where exit_time IS NULL.
func (rs *Resource) countActiveGroupOccupancy(ctx context.Context, activeGroupID int64) (int, error) {
	count, err := rs.ActiveService.CountActiveVisitsByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return 0, fmt.Errorf("failed to find visits for active group %d: %w", activeGroupID, err)
	}

	return count, nil
}

// checkActivityCapacity validates that the activity has capacity for another student.
// Returns nil if capacity is available, error otherwise.
//
// Spontaneous sessions (no parent template in activities.groups, WP-B6) have
// no capacity row to consult. Today's IoT check-in flow only interacts with
// template-backed sessions, so a nil GroupID here is unexpected and we fail
// the capacity check rather than silently admitting the student.
func (rs *Resource) checkActivityCapacity(ctx context.Context, w http.ResponseWriter, r *http.Request, activeGroup *active.Group) error {
	// Get the activity group to check MaxParticipants
	activityGroup := activeGroup.ActualGroup
	if activityGroup == nil {
		templateID, ok := activeGroup.TemplateID()
		if !ok {
			rs.getLogger().ErrorContext(ctx, "spontaneous active group reached IoT capacity check — unsupported in this flow",
				slog.Int64("active_group_id", activeGroup.ID),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("spontaneous sessions are not supported on this check-in path")))
			return errors.New("spontaneous session in IoT capacity check")
		}
		var err error
		activityGroup, err = rs.ActivitiesService.GetGroup(ctx, templateID)
		if err != nil {
			rs.getLogger().ErrorContext(ctx, "failed to get activity group",
				slog.Int64("activity_group_id", templateID),
				slog.String("error", err.Error()),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to get activity information")))
			return err
		}
	}

	// Check activity capacity
	currentOccupancy, countErr := rs.countActiveGroupOccupancy(ctx, activeGroup.ID)
	if countErr != nil {
		rs.getLogger().ErrorContext(ctx, "failed to count activity occupancy",
			slog.Int64("active_group_id", activeGroup.ID),
			slog.String("error", countErr.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to check activity capacity")))
		return countErr
	}

	if currentOccupancy >= activityGroup.MaxParticipants {
		rs.getLogger().WarnContext(ctx, "activity is at capacity",
			slog.String("activity_name", activityGroup.Name),
			slog.Int64("activity_id", activityGroup.ID),
			slog.Int("current_occupancy", currentOccupancy),
			slog.Int("max_participants", activityGroup.MaxParticipants),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorActivityCapacityExceeded(activityGroup.ID, activityGroup.Name, currentOccupancy, activityGroup.MaxParticipants))
		return iotCommon.ErrActivityCapacityExceeded
	}

	rs.getLogger().DebugContext(ctx, "activity capacity check passed",
		slog.String("activity_name", activityGroup.Name),
		slog.Int("current_occupancy", currentOccupancy),
		slog.Int("max_participants", activityGroup.MaxParticipants),
	)

	return nil
}

// findOrCreateActiveGroupForRoom finds an existing active group or creates one for Schulhof.
// When multiple active groups exist in the room, it prefers the one linked to deviceID.
// Returns: activeGroupID, roomName, error
func (rs *Resource) findOrCreateActiveGroupForRoom(ctx context.Context, w http.ResponseWriter, r *http.Request, room *facilities.Room, deviceID int64) (*selectedActiveGroup, error) {
	rs.getLogger().DebugContext(ctx, "looking for active groups in room",
		slog.Int64("room_id", room.ID),
		slog.Int64("device_id", deviceID),
	)

	activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(ctx, room.ID)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to find active groups in room",
			slog.Int64("room_id", room.ID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("error finding active groups in room")))
		return nil, err
	}

	if len(activeGroups) > 0 {
		return rs.useExistingActiveGroup(ctx, activeGroups, room, deviceID), nil
	}

	// No active groups - check if this is a special room (Schulhof, WC)
	return rs.createSpecialRoomActiveGroupIfNeeded(ctx, w, r, room, deviceID)
}

// useExistingActiveGroup selects an active group in the room, preferring one linked to deviceID.
// Falls back to the first group when no device match is found (single-device rooms, legacy sessions).
func (rs *Resource) useExistingActiveGroup(ctx context.Context, activeGroups []*active.Group, room *facilities.Room, deviceID int64) *selectedActiveGroup {
	selected := activeGroups[0] // default fallback
	deviceMatched := false
	for _, g := range activeGroups {
		if g.DeviceID != nil && *g.DeviceID == deviceID {
			selected = g
			deviceMatched = true
			break
		}
	}

	rs.getLogger().DebugContext(ctx, "selected active group in room",
		slog.Int("group_count", len(activeGroups)),
		slog.Int64("room_id", room.ID),
		slog.Int64("device_id", deviceID),
		slog.Int64("active_group_id", selected.ID),
		slog.Bool("device_matched", selected.DeviceID != nil && *selected.DeviceID == deviceID),
	)

	if selected.Room == nil {
		selected.Room = room
	}
	return &selectedActiveGroup{
		Group:        selected,
		RoomName:     room.Name,
		DeviceScoped: deviceMatched,
	}
}

// createSpecialRoomActiveGroupIfNeeded creates an active group if the room is a special room (Schulhof, WC)
func (rs *Resource) createSpecialRoomActiveGroupIfNeeded(ctx context.Context, w http.ResponseWriter, r *http.Request, room *facilities.Room, deviceID int64) (*selectedActiveGroup, error) {
	// Determine which activity group to use based on room name
	var activityGroup *activities.Group
	var err error
	switch {
	case room.Name == constants.SchulhofRoomName:
		rs.getLogger().InfoContext(ctx, "auto-creating Schulhof active group",
			slog.Int64("room_id", room.ID),
		)
		activityGroup, err = rs.schulhofActivityGroup(ctx)
		if err != nil {
			rs.getLogger().ErrorContext(ctx, "failed to find Schulhof activity",
				slog.String("error", err.Error()),
			)
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("schulhof activity not configured")))
			return nil, err
		}
	case constants.IsWCRoomName(room.Name):
		rs.getLogger().InfoContext(ctx, "auto-creating WC active group",
			slog.Int64("room_id", room.ID),
		)
		activityGroup, err = rs.wcActivityGroup(ctx)
		if err != nil {
			rs.getLogger().ErrorContext(ctx, "failed to find WC activity",
				slog.String("error", err.Error()),
			)
			errMsg := "WC activity not configured"
			if strings.Contains(err.Error(), "staff context") {
				errMsg = "WC activity auto-create requires staff context"
			}
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New(errMsg)))
			return nil, err
		}
	default:
		rs.getLogger().WarnContext(ctx, "no active groups found in room",
			slog.Int64("room_id", room.ID),
			slog.String("room_name", room.Name),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("no active groups in specified room")))
		return nil, errors.New("no active groups in specified room")
	}

	// IoT check-in fallback always starts a template-backed session.
	activityGroupID := activityGroup.ID
	newActiveGroup := &active.Group{
		GroupID:      &activityGroupID,
		RoomID:       room.ID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}

	if err := rs.ActiveService.CreateActiveGroup(ctx, newActiveGroup); err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to create active group for special room",
			slog.String("room_name", room.Name),
			slog.String("error", err.Error()),
		)
		switch {
		case room.Name == constants.SchulhofRoomName:
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create Schulhof session")))
		case constants.IsWCRoomName(room.Name):
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create WC session")))
		default:
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(errors.New("failed to create session")))
		}
		return nil, err
	}

	newActiveGroup.Room = room
	newActiveGroup.ActualGroup = activityGroup
	rs.getLogger().InfoContext(ctx, "auto-created active group for special room",
		slog.String("room_name", room.Name),
		slog.Int64("active_group_id", newActiveGroup.ID),
	)
	return &selectedActiveGroup{
		Group:        newActiveGroup,
		RoomName:     room.Name,
		DeviceScoped: false,
	}, nil
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
	DeviceID     int64
	SkipCheckin  bool
	CheckedOut   bool
	CurrentVisit *active.Visit
}

// checkinProcessingResult holds the result of checkin processing
type checkinProcessingResult struct {
	NewVisitID       *int64
	RoomName         string
	ActiveGroupID    *int64
	SelectedGroup    *active.Group
	DeviceScopedRoom bool
	Error            error
}

// processStudentCheckin handles the checkin logic based on room and skip conditions.
// This helper reduces cognitive complexity by encapsulating the branching logic.
func (rs *Resource) processStudentCheckin(ctx context.Context, w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, input *checkinProcessingInput) *checkinProcessingResult {
	result := &checkinProcessingResult{}

	switch {
	case input.RoomID != nil && !input.SkipCheckin:
		// Normal checkin case
		visitID, selection, err := rs.processCheckin(ctx, w, r, student, person, *input.RoomID, input.DeviceID)
		if err != nil {
			result.Error = err
			return result
		}
		result.NewVisitID = visitID
		result.RoomName = selection.RoomName
		result.SelectedGroup = selection.Group
		result.ActiveGroupID = &selection.Group.ID
		result.DeviceScopedRoom = selection.DeviceScoped

	case input.RoomID != nil && input.SkipCheckin:
		// Skipped checkin - just get room name for response
		result.RoomName = rs.roomNameForResponse(ctx, input.CurrentVisit, input.RoomID)

	case !input.CheckedOut:
		// No room_id provided and no previous checkout - error
		rs.getLogger().ErrorContext(ctx, "room ID is required for check-in")
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New("room_id is required for check-in")))
		result.Error = errors.New("room_id is required for check-in")
	}

	return result
}

// buildCheckinResult builds the result message based on what actions occurred
func buildCheckinResult(input *checkinResultInput) *checkinResult {
	result := &checkinResult{}

	if input.CheckedOut && input.NewVisitID != nil {
		// Student checked out and checked in
		if input.PreviousRoomName != "" && input.PreviousRoomName != input.RoomName {
			// Actual room transfer
			result.Action = "transferred"
			result.GreetingMsg = fmt.Sprintf("Gewechselt von %s zu %s!", input.PreviousRoomName, input.RoomName)
		} else {
			// Same room or previous room unknown
			result.Action = "checked_in"
			result.GreetingMsg = "Hallo " + input.Person.FirstName + "!"
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

func (rs *Resource) getDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) *active.Group {
	group, err := rs.ActiveService.FindDeviceActiveGroupInRoom(ctx, roomID, deviceID)
	if err != nil {
		rs.getLogger().WarnContext(ctx, "failed to find active group for room and device",
			slog.Int64("room_id", roomID),
			slog.Int64("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return group
}

func (rs *Resource) getActiveStudentCountForRoom(ctx context.Context, roomID int64) *int {
	count, err := rs.ActiveService.CountActiveVisitsByRoomID(ctx, roomID)
	if err != nil {
		rs.getLogger().WarnContext(ctx, "failed to count active students for room",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &count
}

func (rs *Resource) getActiveStudentCountForGroup(ctx context.Context, activeGroupID int64) *int {
	count, err := rs.ActiveService.CountActiveVisitsByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		rs.getLogger().WarnContext(ctx, "failed to count active students for group",
			slog.Int64("active_group_id", activeGroupID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &count
}

// countActiveStudentsInVisits counts visits without an exit time (active students).
// Kept for tests and utility callers that already have visit data in memory.
func countActiveStudentsInVisits(visits []*active.Visit) int {
	count := 0
	for _, visit := range visits {
		if visit.ExitTime == nil {
			count++
		}
	}
	return count
}

// updateSessionActivity updates session activity for the already-selected active group.
func (rs *Resource) updateSessionActivity(ctx context.Context, activeGroupID int64) {
	if updateErr := rs.ActiveService.UpdateSessionActivity(ctx, activeGroupID); updateErr != nil {
		rs.getLogger().WarnContext(ctx, "failed to update session activity for group",
			slog.Int64("group_id", activeGroupID),
			slog.String("error", updateErr.Error()),
		)
	}
}

// buildCheckinResponse builds the final checkin response map
func buildCheckinResponse(student *users.Student, result *checkinResult, now time.Time) map[string]interface{} {
	studentName := student.Person.FirstName + " " + student.Person.LastName

	response := map[string]interface{}{
		"student_id":               student.ID,
		"student_name":             studentName,
		"action":                   result.Action,
		"visit_id":                 result.VisitID,
		"room_name":                result.RoomName,
		"processed_at":             now,
		"message":                  result.GreetingMsg,
		"status":                   "success",
		"daily_checkout_available": result.DailyCheckoutAvailable,
		"feedback_enabled":         result.FeedbackEnabled,
	}

	if result.Action == "transferred" && result.PreviousRoomName != "" {
		response["previous_room"] = result.PreviousRoomName
	}

	if result.ActiveStudents != nil {
		response["active_students"] = *result.ActiveStudents
	}

	if result.PickupTime != nil {
		response["pickup_time"] = *result.PickupTime
	}

	return response
}

// sendCheckinResponse sends the final response
func sendCheckinResponse(w http.ResponseWriter, r *http.Request, response map[string]interface{}, action string) {
	common.Respond(w, r, http.StatusOK, response, "Student "+action+" successfully")
}
