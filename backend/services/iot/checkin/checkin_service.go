package checkin

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// CheckinService holds the RFID check-in business logic extracted from the
// api/iot/checkin handler (issue #575 B8). It orchestrates the active, users,
// facilities and activities services; it performs NO HTTP rendering. Methods
// that used to render an error mid-flow return one of the typed errors in
// checkin_errors.go, which the handler maps back to the exact renderer.
type CheckinService struct {
	active     activeSvc.Service
	users      usersSvc.PersonService
	facilities facilitiesSvc.Service
	activities activitiesSvc.ActivityService
	settings   configSvc.SettingsService
	pickup     scheduleSvc.PickupScheduleService
	education  educationSvc.Service
	logger     *slog.Logger
	now        func() time.Time
}

// CheckinServiceDeps groups the collaborators for NewCheckinService.
type CheckinServiceDeps struct {
	Active     activeSvc.Service
	Users      usersSvc.PersonService
	Facilities facilitiesSvc.Service
	Activities activitiesSvc.ActivityService
	Settings   configSvc.SettingsService
	Pickup     scheduleSvc.PickupScheduleService
	Education  educationSvc.Service
	Logger     *slog.Logger
}

// NewCheckinService constructs a CheckinService.
func NewCheckinService(deps CheckinServiceDeps) *CheckinService {
	return &CheckinService{
		active:     deps.Active,
		users:      deps.Users,
		facilities: deps.Facilities,
		activities: deps.Activities,
		settings:   deps.Settings,
		pickup:     deps.Pickup,
		education:  deps.Education,
		logger:     deps.Logger,
	}
}

// getLogger returns a nil-safe logger, falling back to slog.Default().
func (s *CheckinService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *CheckinService) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// SelectedActiveGroup is the active group chosen (or created) for a check-in,
// along with the resolved room name and whether the selection was scoped to the
// scanning device.
type SelectedActiveGroup struct {
	Group        *active.Group
	RoomName     string
	DeviceScoped bool
	created      bool
}

// CheckinResult holds the outcome of processing a check-in request. It is built
// by BuildCheckinResult and then enriched by the handler (daily-checkout flag,
// feedback flag, active-student count, pickup time).
type CheckinResult struct {
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

// CheckinResultInput holds the inputs for BuildCheckinResult.
type CheckinResultInput struct {
	Student          *users.Student
	Person           *users.Person
	CheckedOut       bool
	NewVisitID       *int64
	CheckoutVisitID  *int64
	RoomName         string
	PreviousRoomName string
	CurrentVisit     *active.Visit
}

// CheckinProcessingInput holds the inputs for ProcessStudentCheckin.
type CheckinProcessingInput struct {
	RoomID       *int64
	DeviceID     int64
	SkipCheckin  bool
	CheckedOut   bool
	CurrentVisit *active.Visit
}

// CheckinProcessingResult holds the result of check-in processing.
type CheckinProcessingResult struct {
	NewVisitID       *int64
	RoomName         string
	ActiveGroupID    *int64
	SelectedGroup    *active.Group
	DeviceScopedRoom bool
}

// SupervisorScanResult carries the session context the handler needs to build
// the 200 supervisor-authenticated response.
type SupervisorScanResult struct {
	RoomName     string
	ActivityName string
}

// ResolvePersonByRFID finds a person by RFID without rendering. It preserves
// usersSvc.ErrPersonNotFound so the handler can distinguish "unknown tag" from
// backend failures.
func (s *CheckinService) ResolvePersonByRFID(ctx context.Context, rfid string) (*users.Person, error) {
	person, err := s.users.FindByTagID(ctx, rfid)
	if err != nil {
		return nil, err
	}
	if person == nil {
		return nil, usersSvc.ErrPersonNotFound
	}
	return person, nil
}

// ResolveStudentFromPerson finds a student from a person record. A missing
// student returns (nil, nil); repository failures are returned.
//
// A graduated (alumnus) student is soft-deleted by the grade-transition flow:
// treat them as "no active student" so the primary kiosk check-in
// (POST /api/iot/checkin) and pickup-query flows reject the scan exactly like
// an unknown tag. Without this, a graduate's RFID could still open a room visit
// (detailed mode) or an attendance row (binary mode), because neither this
// resolver nor lookupStudentFromPerson checked the status (#405).
func (s *CheckinService) ResolveStudentFromPerson(ctx context.Context, personID int64) (*users.Student, error) {
	student, err := s.users.GetStudentByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if student != nil && student.Status == users.StudentStatusAlumnus {
		return nil, nil
	}
	// A child whose care has ended is off every kiosk roster from the day
	// after their last care day (#2487). Reported as an unknown tag, exactly
	// like a graduate, so PyrePortal needs no new error mapping.
	if student.CareEndedOn(timezone.TodayDate()) {
		return nil, nil
	}
	return student, nil
}

// ResolveStaffFromPerson finds a staff record from a person record. A missing
// staff member returns (nil, nil); repository failures are returned.
func (s *CheckinService) ResolveStaffFromPerson(ctx context.Context, personID int64) (*users.Staff, error) {
	staff, err := s.users.GetStaffByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return staff, nil
}

// LoadCurrentVisitWithRoom loads the current active visit and its room. Returns
// nil when there is no active visit or the lookup fails (logged at Debug).
func (s *CheckinService) LoadCurrentVisitWithRoom(ctx context.Context, studentID int64) *active.Visit {
	currentVisit, err := s.active.GetStudentCurrentVisitWithRoom(ctx, studentID)
	if err != nil {
		s.getLogger().DebugContext(ctx, "error checking current visit",
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

// buildStudentAlreadyActiveConflict best-effort-loads the existing visit so the
// kiosk can show "Bereits angemeldet in Raum X". The lookup is deliberately
// tolerant: if it fails, only StudentID is populated and the kiosk degrades to
// a generic message rather than upgrading the 409 to a 500.
func (s *CheckinService) buildStudentAlreadyActiveConflict(ctx context.Context, studentID int64) *StudentAlreadyActiveConflict {
	existing := s.LoadCurrentVisitWithRoom(ctx, studentID)
	if existing == nil {
		return &StudentAlreadyActiveConflict{StudentID: studentID}
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
	return &StudentAlreadyActiveConflict{
		StudentID:       studentID,
		ExistingVisitID: existing.ID,
		EntryTime:       &entryTime,
		RoomID:          roomID,
		RoomName:        roomName,
	}
}

// ProcessCheckout ends the student's current room visit WITHOUT attendance sync
// (leaving a room means "unterwegs", not "zuhause"). Returns the ended visit ID
// and the previous room name.
func (s *CheckinService) ProcessCheckout(ctx context.Context, student *users.Student, person *users.Person, currentVisit *active.Visit) (*int64, string, error) {
	s.getLogger().DebugContext(ctx, "student has active visit, performing checkout",
		slog.String("student_name", person.FirstName+" "+person.LastName),
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", currentVisit.ID),
	)

	var previousRoomName string
	if currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		previousRoomName = currentVisit.ActiveGroup.Room.Name
		s.getLogger().DebugContext(ctx, "previous room from active group",
			slog.String("room_name", previousRoomName),
			slog.Int64("room_id", currentVisit.ActiveGroup.RoomID),
		)
	} else {
		s.getLogger().WarnContext(ctx, "could not get previous room name",
			slog.Bool("has_active_group", currentVisit.ActiveGroup != nil),
			slog.Bool("has_room", currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil),
		)
	}

	if err := s.active.EndVisit(ctx, currentVisit.ID); err != nil {
		s.getLogger().ErrorContext(ctx, "failed to end visit",
			slog.Int64("visit_id", currentVisit.ID),
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		return nil, "", newInternalError(checkinErrEndVisit)
	}

	s.getLogger().InfoContext(ctx, "checked out student",
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", currentVisit.ID),
	)

	visitID := currentVisit.ID
	return &visitID, previousRoomName, nil
}

// ShouldSkipCheckin reports whether check-in should be skipped (student scanned
// the same room they are already in). A visit from a previous day's session is
// a rollover recovery, not a same-session toggle: after checkout the scan must
// continue into today's group.
func ShouldSkipCheckin(roomID *int64, checkedOut bool, currentVisit *active.Visit, now time.Time) bool {
	if roomID == nil || !checkedOut || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}
	if !currentVisit.ActiveGroup.StartTime.IsZero() &&
		timezone.DateFromTime(currentVisit.ActiveGroup.StartTime) != timezone.DateFromTime(now) {
		return false
	}
	return currentVisit.ActiveGroup.RoomID == *roomID
}

// processCheckin handles the check-in logic for a student. Returns the new
// visit ID and the selected active group.
func (s *CheckinService) processCheckin(ctx context.Context, student *users.Student, person *users.Person, roomID int64, deviceID int64) (*int64, *SelectedActiveGroup, error) {
	s.getLogger().DebugContext(ctx, "performing check-in to room",
		slog.String("student_name", person.FirstName+" "+person.LastName),
		slog.Int64("student_id", student.ID),
		slog.Int64("room_id", roomID),
	)

	room, err := s.facilities.GetRoom(ctx, roomID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to get room",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
		return nil, nil, newInternalError(checkinErrGetRoom)
	}
	if err := s.checkRoomCapacity(ctx, room); err != nil {
		return nil, nil, err
	}

	selection, err := s.findOrCreateActiveGroupForRoom(ctx, room, deviceID)
	if err != nil {
		return nil, nil, err
	}

	if capacityErr := s.checkActivityCapacity(ctx, selection.Group); capacityErr != nil {
		return nil, nil, capacityErr
	}

	newVisit := &active.Visit{
		StudentID:     student.ID,
		ActiveGroupID: selection.Group.ID,
		EntryTime:     time.Now(),
	}

	s.getLogger().DebugContext(ctx, "creating visit for student",
		slog.Int64("student_id", student.ID),
		slog.Int64("active_group_id", selection.Group.ID),
	)
	if err := s.active.CreateVisit(ctx, newVisit); err != nil {
		var roomCapacityErr *activeSvc.RoomCapacityError
		if errors.As(err, &roomCapacityErr) {
			s.deleteEmptyCreatedGroup(ctx, selection)
			return nil, nil, roomCapacityErr
		}
		// Duplicate active visit — race between two concurrent scans, or a
		// previous checkout that didn't fully commit. Surface as a 409
		// conflict carrying the existing visit details.
		if errors.Is(err, activeSvc.ErrStudentAlreadyActive) {
			s.getLogger().InfoContext(ctx, "duplicate active visit rejected",
				slog.Int64("student_id", student.ID),
				slog.String("error", err.Error()),
			)
			return nil, nil, s.buildStudentAlreadyActiveConflict(ctx, student.ID)
		}
		// Graduation committed between resolving this student (as active) and
		// creating the visit — a race with a concurrent grade-transition apply.
		// CreateVisit's guard returns ErrStudentGraduated; surface it as the same
		// 404 "not a student" the pre-resolution path already returns for an
		// alumnus, instead of letting the generic wrapper below turn it into a 500
		// that tells the kiosk to retry (#405).
		if errors.Is(err, activeSvc.ErrStudentGraduated) {
			s.getLogger().InfoContext(ctx, "check-in rejected: student graduated mid-scan",
				slog.Int64("student_id", student.ID),
			)
			return nil, nil, newNotFoundError(checkinErrPersonNotStudent)
		}
		s.getLogger().ErrorContext(ctx, "failed to create visit",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		return nil, nil, newInternalError(checkinErrCreateVisit)
	}

	s.getLogger().InfoContext(ctx, "checked in student",
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", newVisit.ID),
		slog.String("room", selection.RoomName),
	)

	return &newVisit.ID, selection, nil
}

func (s *CheckinService) deleteEmptyCreatedGroup(ctx context.Context, selection *SelectedActiveGroup) {
	if selection == nil || !selection.created || selection.Group == nil {
		return
	}
	if err := s.active.DeleteActiveGroup(ctx, selection.Group.ID); err != nil {
		s.getLogger().WarnContext(ctx, "failed to remove empty active group after rejected check-in",
			slog.Int64("active_group_id", selection.Group.ID),
			slog.String("error", err.Error()),
		)
	}
}

func (s *CheckinService) checkRoomCapacity(ctx context.Context, room *facilities.Room) error {
	if room.Capacity == nil || *room.Capacity <= 0 {
		return nil
	}

	currentOccupancy, err := s.active.CountActiveVisitsByRoomID(ctx, room.ID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to count room occupancy",
			slog.Int64("room_id", room.ID),
			slog.String("error", err.Error()),
		)
		return newInternalError(checkinErrCheckRoomCapacity)
	}
	if currentOccupancy >= *room.Capacity {
		return &activeSvc.RoomCapacityError{
			RoomID:           room.ID,
			RoomName:         room.Name,
			CurrentOccupancy: currentOccupancy,
			MaxCapacity:      *room.Capacity,
		}
	}

	return nil
}

// countActiveGroupOccupancy counts active visits (exit_time IS NULL) in an
// active group.
func (s *CheckinService) countActiveGroupOccupancy(ctx context.Context, activeGroupID int64) (int, error) {
	count, err := s.active.CountActiveVisitsByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return 0, fmt.Errorf("failed to find visits for active group %d: %w", activeGroupID, err)
	}

	return count, nil
}

// checkActivityCapacity validates the activity has room for another student.
// Spontaneous sessions (no template) are unsupported on this path.
func (s *CheckinService) checkActivityCapacity(ctx context.Context, activeGroup *active.Group) error {
	activityGroup := activeGroup.ActualGroup
	if activityGroup == nil {
		templateID, ok := activeGroup.TemplateID()
		if !ok {
			s.getLogger().ErrorContext(ctx, "spontaneous active group reached IoT capacity check — unsupported in this flow",
				slog.Int64("active_group_id", activeGroup.ID),
			)
			return newInternalError(checkinErrSpontaneous)
		}
		var err error
		activityGroup, err = s.activities.GetGroup(ctx, templateID)
		if err != nil {
			s.getLogger().ErrorContext(ctx, "failed to get activity group",
				slog.Int64("activity_group_id", templateID),
				slog.String("error", err.Error()),
			)
			return newInternalError(checkinErrGetActivity)
		}
	}
	if !activityGroup.HasParticipantLimit() {
		return nil
	}

	currentOccupancy, countErr := s.countActiveGroupOccupancy(ctx, activeGroup.ID)
	if countErr != nil {
		s.getLogger().ErrorContext(ctx, "failed to count activity occupancy",
			slog.Int64("active_group_id", activeGroup.ID),
			slog.String("error", countErr.Error()),
		)
		return newInternalError(checkinErrCheckActivityCap)
	}

	if currentOccupancy >= activityGroup.MaxParticipants {
		s.getLogger().WarnContext(ctx, "activity is at capacity",
			slog.String("activity_name", activityGroup.Name),
			slog.Int64("activity_id", activityGroup.ID),
			slog.Int("current_occupancy", currentOccupancy),
			slog.Int("max_participants", activityGroup.MaxParticipants),
		)
		return &ActivityCapacityError{
			ActivityID:       activityGroup.ID,
			ActivityName:     activityGroup.Name,
			CurrentOccupancy: currentOccupancy,
			MaxCapacity:      activityGroup.MaxParticipants,
		}
	}

	s.getLogger().DebugContext(ctx, "activity capacity check passed",
		slog.String("activity_name", activityGroup.Name),
		slog.Int("current_occupancy", currentOccupancy),
		slog.Int("max_participants", activityGroup.MaxParticipants),
	)

	return nil
}

// findOrCreateActiveGroupForRoom finds an existing active group or creates one
// for a special room (Schulhof, WC). When multiple active groups exist, it
// prefers the one linked to deviceID.
func (s *CheckinService) findOrCreateActiveGroupForRoom(ctx context.Context, room *facilities.Room, deviceID int64) (*SelectedActiveGroup, error) {
	s.getLogger().DebugContext(ctx, "looking for active groups in room",
		slog.Int64("room_id", room.ID),
		slog.Int64("device_id", deviceID),
	)

	activeGroups, err := s.active.FindActiveGroupsByRoomID(ctx, room.ID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to find active groups in room",
			slog.Int64("room_id", room.ID),
			slog.String("error", err.Error()),
		)
		return nil, newInternalError(checkinErrFindActiveGroups)
	}

	// The same-day filter is limited to the special rooms that auto-create
	// sessions: only there does endPreviousDayActiveGroups close the stale
	// session that the filter hides. For regular rooms no such cleanup runs
	// here — a prior-day session that outlived the session-end cutoff must
	// stay reusable, or scans would 404 until the next EndDailySessions run.
	currentGroups := activeGroups
	if room.Name == constants.SchulhofRoomName || constants.IsWCRoomName(room.Name) {
		now := s.currentTime()
		if err := s.endPreviousDayActiveGroups(ctx, activeGroups, now); err != nil {
			s.getLogger().ErrorContext(ctx, "failed to end stale special-room session",
				slog.Int64("room_id", room.ID),
				slog.String("room_name", room.Name),
				slog.String("error", err.Error()),
			)
			return nil, specialRoomCreationError(room.Name)
		}
		currentGroups = activeGroupsStartedToday(activeGroups, now)
	}
	if len(currentGroups) > 0 {
		return s.useExistingActiveGroup(ctx, currentGroups, room, deviceID), nil
	}

	return s.createSpecialRoomActiveGroupIfNeeded(ctx, room, deviceID)
}

func activeGroupsStartedToday(groups []*active.Group, now time.Time) []*active.Group {
	today := timezone.DateFromTime(now)
	current := make([]*active.Group, 0, len(groups))
	for _, group := range groups {
		if group == nil || group.EndTime != nil || timezone.DateFromTime(group.StartTime) != today {
			continue
		}
		current = append(current, group)
	}
	return current
}

func (s *CheckinService) endPreviousDayActiveGroups(ctx context.Context, groups []*active.Group, now time.Time) error {
	today := timezone.DateFromTime(now)
	for _, group := range groups {
		if group == nil || group.EndTime != nil {
			continue
		}
		if groupDate := timezone.DateFromTime(group.StartTime); !groupDate.Before(today) {
			continue
		}

		err := s.active.EndActiveGroupSession(ctx, group.ID)
		if err != nil && !errors.Is(err, activeSvc.ErrActiveGroupAlreadyEnded) {
			return fmt.Errorf("end stale active group %d: %w", group.ID, err)
		}
	}
	return nil
}

// useExistingActiveGroup selects an active group in the room, preferring one
// linked to deviceID and falling back to the group with the newest start_time.
// The newest-wins fallback is deliberate: since Schulhof became a regular
// plannable room (#2161) several open groups can coexist in one room (planned
// block + spontaneous/IoT fallback session), and the repository query carries
// no ORDER BY — picking the first row would make the scan target a coin flip.
// Newest-wins lands kiosk scans in the just-started planned block and matches
// the Schulhof status read model's selection.
func (s *CheckinService) useExistingActiveGroup(ctx context.Context, activeGroups []*active.Group, room *facilities.Room, deviceID int64) *SelectedActiveGroup {
	selected := activeGroups[0]
	deviceMatched := false
	for _, g := range activeGroups {
		if g.DeviceID != nil && *g.DeviceID == deviceID {
			selected = g
			deviceMatched = true
			break
		}
		if g.StartTime.After(selected.StartTime) {
			selected = g
		}
	}

	s.getLogger().DebugContext(ctx, "selected active group in room",
		slog.Int("group_count", len(activeGroups)),
		slog.Int64("room_id", room.ID),
		slog.Int64("device_id", deviceID),
		slog.Int64("active_group_id", selected.ID),
		slog.Bool("device_matched", selected.DeviceID != nil && *selected.DeviceID == deviceID),
	)

	if selected.Room == nil {
		selected.Room = room
	}
	return &SelectedActiveGroup{
		Group:        selected,
		RoomName:     room.Name,
		DeviceScoped: deviceMatched,
	}
}

// createSpecialRoomActiveGroupIfNeeded creates an active group if the room is a
// special room (Schulhof, WC).
func (s *CheckinService) createSpecialRoomActiveGroupIfNeeded(ctx context.Context, room *facilities.Room, deviceID int64) (*SelectedActiveGroup, error) {
	var activityGroup *activities.Group
	var err error
	switch {
	case room.Name == constants.SchulhofRoomName:
		s.getLogger().InfoContext(ctx, "auto-creating Schulhof active group",
			slog.Int64("room_id", room.ID),
		)
		activityGroup, err = s.schulhofActivityGroup(ctx)
		if err != nil {
			s.getLogger().ErrorContext(ctx, "failed to find Schulhof activity",
				slog.String("error", err.Error()),
			)
			return nil, newInternalError(checkinErrSchulhofNotConfig)
		}
	case constants.IsWCRoomName(room.Name):
		s.getLogger().InfoContext(ctx, "auto-creating WC active group",
			slog.Int64("room_id", room.ID),
		)
		activityGroup, err = s.wcActivityGroup(ctx)
		if err != nil {
			s.getLogger().ErrorContext(ctx, "failed to find WC activity",
				slog.String("error", err.Error()),
			)
			errMsg := checkinErrWCNotConfig
			if strings.Contains(err.Error(), "staff context") {
				errMsg = checkinErrWCNeedsStaff
			}
			return nil, newInternalError(errMsg)
		}
	default:
		s.getLogger().WarnContext(ctx, "no active groups found in room",
			slog.Int64("room_id", room.ID),
			slog.String("room_name", room.Name),
		)
		return nil, newNotFoundError(checkinErrNoGroupsInRoom)
	}

	// IoT check-in fallback always starts a template-backed session.
	activityGroupID := activityGroup.ID
	newActiveGroup := &active.Group{
		GroupID:      &activityGroupID,
		RoomID:       room.ID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
	}

	if err := s.active.CreateActiveGroup(ctx, newActiveGroup); err != nil {
		s.getLogger().ErrorContext(ctx, "failed to create active group for special room",
			slog.String("room_name", room.Name),
			slog.String("error", err.Error()),
		)
		return nil, specialRoomCreationError(room.Name)
	}

	newActiveGroup.Room = room
	newActiveGroup.ActualGroup = activityGroup
	s.getLogger().InfoContext(ctx, "auto-created active group for special room",
		slog.String("room_name", room.Name),
		slog.Int64("active_group_id", newActiveGroup.ID),
	)
	return &SelectedActiveGroup{
		Group:        newActiveGroup,
		RoomName:     room.Name,
		DeviceScoped: false,
		created:      true,
	}, nil
}

func specialRoomCreationError(roomName string) *CheckinError {
	switch {
	case roomName == constants.SchulhofRoomName:
		return newInternalError(checkinErrCreateSchulhof)
	case constants.IsWCRoomName(roomName):
		return newInternalError(checkinErrCreateWC)
	default:
		return newInternalError(checkinErrCreateSession)
	}
}

// roomNameByID resolves the room name from a room object or by ID lookup.
func (s *CheckinService) roomNameByID(ctx context.Context, room *facilities.Room, roomID int64) string {
	if room != nil {
		return room.Name
	}

	loadedRoom, err := s.facilities.GetRoom(ctx, roomID)
	if err == nil && loadedRoom != nil {
		return loadedRoom.Name
	}

	return fmt.Sprintf("Room %d", roomID)
}

// roomNameForResponse resolves the room name for a check-in response.
func (s *CheckinService) roomNameForResponse(ctx context.Context, currentVisit *active.Visit, roomID *int64) string {
	if currentVisit != nil && currentVisit.ActiveGroup != nil && currentVisit.ActiveGroup.Room != nil {
		return currentVisit.ActiveGroup.Room.Name
	}
	if roomID != nil {
		return s.roomNameByID(ctx, nil, *roomID)
	}
	return ""
}

// ProcessStudentCheckin handles the check-in logic based on room and skip
// conditions.
func (s *CheckinService) ProcessStudentCheckin(ctx context.Context, student *users.Student, person *users.Person, input *CheckinProcessingInput) (*CheckinProcessingResult, error) {
	result := &CheckinProcessingResult{}

	switch {
	case input.RoomID != nil && !input.SkipCheckin:
		// Normal checkin case
		visitID, selection, err := s.processCheckin(ctx, student, person, *input.RoomID, input.DeviceID)
		if err != nil {
			return nil, err
		}
		result.NewVisitID = visitID
		result.RoomName = selection.RoomName
		result.SelectedGroup = selection.Group
		result.ActiveGroupID = &selection.Group.ID
		result.DeviceScopedRoom = selection.DeviceScoped

	case input.RoomID != nil && input.SkipCheckin:
		// Skipped checkin - just get room name for response
		result.RoomName = s.roomNameForResponse(ctx, input.CurrentVisit, input.RoomID)

	case !input.CheckedOut:
		// No room_id provided and no previous checkout - error
		s.getLogger().ErrorContext(ctx, "room ID is required for check-in")
		return nil, newInvalidRequestError(checkinErrRoomIDRequired)
	}

	return result, nil
}

// BuildCheckinResult builds the result message based on what actions occurred.
func BuildCheckinResult(input *CheckinResultInput) *CheckinResult {
	result := &CheckinResult{}

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
		// Note: Daily checkout upgrade happens in the handler via
		// shouldUpgradeToDailyCheckout() which has EducationService for room matching.
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

// GetDeviceActiveGroupInRoom returns the active group in a room linked to the
// scanning device, or nil on error (logged at Warn).
func (s *CheckinService) GetDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) *active.Group {
	group, err := s.active.FindDeviceActiveGroupInRoom(ctx, roomID, deviceID)
	if err != nil {
		s.getLogger().WarnContext(ctx, "failed to find active group for room and device",
			slog.Int64("room_id", roomID),
			slog.Int64("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return group
}

// GetActiveStudentCountForRoom returns the active-visit count for a room, or nil
// on error (logged at Warn).
func (s *CheckinService) GetActiveStudentCountForRoom(ctx context.Context, roomID int64) *int {
	count, err := s.active.CountActiveVisitsByRoomID(ctx, roomID)
	if err != nil {
		s.getLogger().WarnContext(ctx, "failed to count active students for room",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &count
}

// GetActiveStudentCountForGroup returns the active-visit count for an active
// group, or nil on error (logged at Warn).
func (s *CheckinService) GetActiveStudentCountForGroup(ctx context.Context, activeGroupID int64) *int {
	count, err := s.active.CountActiveVisitsByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		s.getLogger().WarnContext(ctx, "failed to count active students for group",
			slog.Int64("active_group_id", activeGroupID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &count
}

// ResolveActiveStudentCount refreshes the session heartbeat for the group tied
// to the scan and returns the active-student count to surface to the kiosk. It
// scopes the heartbeat and count to the scanning device's session where
// possible, falling back to a room-wide count for rooms without a device-linked
// active group.
func (s *CheckinService) ResolveActiveStudentCount(ctx context.Context, result *CheckinProcessingResult, roomID int64, deviceID int64) *int {
	if result.ActiveGroupID != nil && result.DeviceScopedRoom {
		s.UpdateSessionActivity(ctx, *result.ActiveGroupID)
		return s.GetActiveStudentCountForGroup(ctx, *result.ActiveGroupID)
	}

	if deviceGroup := s.GetDeviceActiveGroupInRoom(ctx, roomID, deviceID); deviceGroup != nil {
		s.UpdateSessionActivity(ctx, deviceGroup.ID)
		return s.GetActiveStudentCountForGroup(ctx, deviceGroup.ID)
	}

	return s.GetActiveStudentCountForRoom(ctx, roomID)
}

// UpdateSessionActivity refreshes the session heartbeat for the given active
// group (best-effort; failures are logged at Warn).
func (s *CheckinService) UpdateSessionActivity(ctx context.Context, activeGroupID int64) {
	if updateErr := s.active.UpdateSessionActivity(ctx, activeGroupID); updateErr != nil {
		s.getLogger().WarnContext(ctx, "failed to update session activity for group",
			slog.Int64("group_id", activeGroupID),
			slog.String("error", updateErr.Error()),
		)
	}
}

// AddStaffAsSupervisor processes RFID-based supervisor authentication: staff who
// scan at a device with an active session are added as supervisors (idempotent).
// Returns the session's room/activity name for the response.
func (s *CheckinService) AddStaffAsSupervisor(ctx context.Context, deviceID int64, deviceIDStr string, staffID int64) (*SupervisorScanResult, error) {
	// Step 1: Get current session for this device
	session, err := s.active.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil || session == nil {
		s.getLogger().InfoContext(ctx, "no active session for supervisor scan",
			slog.String("device_id", deviceIDStr),
			slog.Int64("device_db_id", deviceID),
		)
		return nil, newNotFoundError(checkinErrNoActiveSession)
	}

	s.getLogger().InfoContext(ctx, "processing supervisor authentication",
		slog.String("device_id", deviceIDStr),
		slog.Int64("session_id", session.ID),
		slog.Int64("staff_id", staffID),
	)

	// Step 2: Load current supervisors to check for duplicates
	groupWithSupervisors, err := s.active.GetActiveGroupWithSupervisors(ctx, session.ID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to load supervisors for session",
			slog.Int64("session_id", session.ID),
			slog.String("error", err.Error()),
		)
		return nil, newInternalWrapError(checkinErrLoadSupervisors, err)
	}

	// Step 3: Build supervisor ID list (existing active + new)
	var supervisorIDs []int64
	alreadySupervisor := false
	for _, sup := range groupWithSupervisors.Supervisors {
		if sup.EndDate == nil { // Only include active supervisors
			supervisorIDs = append(supervisorIDs, sup.StaffID)
			if sup.StaffID == staffID {
				alreadySupervisor = true
			}
		}
	}

	// Step 4: Add new supervisor if not already present
	if !alreadySupervisor {
		supervisorIDs = append(supervisorIDs, staffID)
		if _, err := s.active.UpdateActiveGroupSupervisors(ctx, session.ID, supervisorIDs); err != nil {
			s.getLogger().ErrorContext(ctx, "failed to add supervisor to session",
				slog.Int64("staff_id", staffID),
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()),
			)
			return nil, newInternalWrapError(checkinErrUpdateSupervisors, err)
		}
		s.getLogger().InfoContext(ctx, "added staff as supervisor to session",
			slog.Int64("staff_id", staffID),
			slog.Int64("session_id", session.ID),
		)
	} else {
		s.getLogger().DebugContext(ctx, "staff is already a supervisor (idempotent)",
			slog.Int64("staff_id", staffID),
			slog.Int64("session_id", session.ID),
		)
	}

	// Step 5: Resolve response context
	result := &SupervisorScanResult{}
	if session.Room != nil {
		result.RoomName = session.Room.Name
	}
	if session.ActualGroup != nil {
		result.ActivityName = session.ActualGroup.Name
	}
	return result, nil
}
