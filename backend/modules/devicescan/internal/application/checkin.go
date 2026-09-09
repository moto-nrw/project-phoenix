package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// currentVisitCheckout is what closing the student's current room stay
// produced: the visit that was open, its id and room, and whether it was
// closed by this scan.
type currentVisitCheckout struct {
	Visit            *ports.CurrentVisit
	VisitID          *int64
	PreviousRoomName string
	CheckedOut       bool
}

// checkinInput drives processStudentCheckin.
type checkinInput struct {
	RoomID      *int64
	DeviceID    int64
	SkipCheckin bool
	Checkout    currentVisitCheckout
}

// checkinResult is what opening the requested room stay produced.
type checkinResult struct {
	NewVisitID       *int64
	RoomName         string
	SessionID        *int64
	DeviceScopedRoom bool
}

// selectedSession is the session chosen (or provisioned) for a check-in.
type selectedSession struct {
	Session      ports.Session
	RoomName     string
	DeviceScoped bool
	created      bool
}

// loadCurrentVisit returns nil only when no open visit exists. A lookup
// failure stops the scan instead of being read as a new arrival.
func (s *Service) loadCurrentVisit(ctx context.Context, studentID int64) (*ports.CurrentVisit, error) {
	visit, err := s.visits.Current(ctx, studentID)
	if err != nil {
		return nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
	}
	return visit, nil
}

// checkoutCurrentVisit resolves and closes the room stay before a new
// check-in. A failed lookup or checkout prevents the scan from proceeding.
func (s *Service) checkoutCurrentVisit(ctx context.Context, student *ports.Student, person *ports.Person) (currentVisitCheckout, error) {
	visit, err := s.loadCurrentVisit(ctx, student.ID)
	if err != nil || visit == nil {
		return currentVisitCheckout{}, err
	}
	s.logger.DebugContext(ctx, "student has active visit, performing checkout",
		slog.String("student_name", person.FirstName+" "+person.LastName),
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", visit.ID),
	)

	var previousRoomName string
	if visit.Session != nil && visit.Session.Room != nil {
		previousRoomName = visit.Session.Room.Name
		s.logger.DebugContext(ctx, "previous room from active group",
			slog.String("room_name", previousRoomName),
			slog.Int64("room_id", visit.Session.RoomID),
		)
	} else {
		s.logger.WarnContext(ctx, "could not get previous room name",
			slog.Bool("has_active_group", visit.Session != nil),
			slog.Bool("has_room", visit.Session != nil && visit.Session.Room != nil),
		)
	}

	if err := s.visits.End(ctx, visit.ID); err != nil {
		s.logger.ErrorContext(ctx, "failed to end visit",
			slog.Int64("visit_id", visit.ID),
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		return currentVisitCheckout{}, devicescan.Internal(devicescan.MessageEndVisitFailed, nil)
	}
	s.logger.InfoContext(ctx, "checked out student",
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", visit.ID),
	)
	visitID := visit.ID
	return currentVisitCheckout{Visit: visit, VisitID: &visitID, PreviousRoomName: previousRoomName, CheckedOut: true}, nil
}

// shouldSkipCheckin reports a scan of the room the child just left: the
// checkout stands and no re-entry happens. A visit from a previous day's
// session is a rollover recovery, not a same-session toggle, so after that
// checkout the scan continues into today's session.
func (s *Service) shouldSkipCheckin(roomID *int64, checkout currentVisitCheckout, now time.Time) bool {
	if roomID == nil || !checkout.CheckedOut || checkout.Visit == nil || checkout.Visit.Session == nil {
		return false
	}
	session := checkout.Visit.Session
	if !session.StartTime.IsZero() && s.clock.Day(session.StartTime) != s.clock.Day(now) {
		return false
	}
	return session.RoomID == *roomID
}

// processStudentCheckin opens the requested room stay, or only resolves the
// room name when the check-in is skipped. Without a room and without a
// preceding checkout the scan asked for nothing.
func (s *Service) processStudentCheckin(ctx context.Context, student *ports.Student, person *ports.Person, input checkinInput) (*checkinResult, error) {
	result := &checkinResult{}
	switch {
	case input.RoomID != nil && !input.SkipCheckin:
		visitID, selection, err := s.processCheckin(ctx, student, person, *input.RoomID, input.DeviceID)
		if err != nil {
			return nil, err
		}
		result.NewVisitID = visitID
		result.RoomName = selection.RoomName
		sessionID := selection.Session.ID
		result.SessionID = &sessionID
		result.DeviceScopedRoom = selection.DeviceScoped
	case input.RoomID != nil && input.SkipCheckin:
		result.RoomName = s.roomNameForResponse(ctx, input.Checkout.Visit, input.RoomID)
	case !input.Checkout.CheckedOut:
		s.logger.ErrorContext(ctx, "room ID is required for check-in")
		return nil, devicescan.InvalidRequest(devicescan.MessageRoomIDRequired)
	}
	return result, nil
}

// processCheckin opens a visit in the room's session and returns the visit
// id with the session that received it.
func (s *Service) processCheckin(ctx context.Context, student *ports.Student, person *ports.Person, roomID, deviceID int64) (*int64, *selectedSession, error) {
	s.logger.DebugContext(ctx, "performing check-in to room",
		slog.String("student_name", person.FirstName+" "+person.LastName),
		slog.Int64("student_id", student.ID),
		slog.Int64("room_id", roomID),
	)

	room, err := s.rooms.FindRoom(ctx, roomID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get room",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
		return nil, nil, devicescan.Internal(devicescan.MessageGetRoomFailed, nil)
	}
	if err := s.checkRoomCapacity(ctx, room); err != nil {
		return nil, nil, err
	}

	selection, err := s.findOrCreateSessionForRoom(ctx, room, deviceID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkActivityCapacity(ctx, &selection.Session); err != nil {
		return nil, nil, err
	}

	s.logger.DebugContext(ctx, "creating visit for student",
		slog.Int64("student_id", student.ID),
		slog.Int64("active_group_id", selection.Session.ID),
	)
	visitID, err := s.visits.Record(ctx, student.ID, selection.Session.ID)
	if err != nil {
		return nil, nil, s.classifyRecordFailure(ctx, student.ID, selection, err)
	}

	s.logger.InfoContext(ctx, "checked in student",
		slog.Int64("student_id", student.ID),
		slog.Int64("visit_id", visitID),
		slog.String("room", selection.RoomName),
	)
	return &visitID, selection, nil
}

// classifyRecordFailure maps a refused visit to the kiosk contract: a full
// room drops the session this scan provisioned, a duplicate scan carries
// the existing stay, a departed child is "not a student", and everything
// else is the pinned create failure.
func (s *Service) classifyRecordFailure(ctx context.Context, studentID int64, selection *selectedSession, err error) error {
	var roomCapacity *ports.RoomCapacityExceeded
	if errors.As(err, &roomCapacity) {
		s.deleteEmptyCreatedSession(ctx, selection)
		return s.roomCapacityExceeded(ctx, roomCapacity.RoomID, roomCapacity.RoomName, roomCapacity.CurrentOccupancy, roomCapacity.MaxCapacity)
	}
	if errors.Is(err, ports.ErrStudentAlreadyActive) {
		s.logger.InfoContext(ctx, "duplicate active visit rejected",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return s.buildStudentAlreadyActive(ctx, studentID)
	}
	if errors.Is(err, ports.ErrStudentNotInCare) {
		s.logger.InfoContext(ctx, "check-in rejected: student not in care",
			slog.Int64("student_id", studentID),
		)
		return devicescan.NotFound(devicescan.MessagePersonNotStudent)
	}
	s.logger.ErrorContext(ctx, "failed to create visit",
		slog.Int64("student_id", studentID),
		slog.String("error", err.Error()),
	)
	return devicescan.Internal(devicescan.MessageCreateVisitFailed, nil)
}

// buildStudentAlreadyActive loads the details of a rejected duplicate scan.
// A concurrently closed visit needs no details; a failed lookup remains an
// error.
func (s *Service) buildStudentAlreadyActive(ctx context.Context, studentID int64) error {
	existing, err := s.loadCurrentVisit(ctx, studentID)
	if err != nil {
		return err
	}
	if existing == nil {
		return &devicescan.StudentAlreadyActiveError{StudentID: studentID}
	}
	conflict := &devicescan.StudentAlreadyActiveError{StudentID: studentID, ExistingVisitID: existing.ID}
	entryTime := existing.EntryTime
	conflict.EntryTime = &entryTime
	if existing.Session != nil {
		roomID := existing.Session.RoomID
		conflict.RoomID = &roomID
		if existing.Session.Room != nil {
			conflict.RoomName = existing.Session.Room.Name
		}
	}
	return conflict
}

func (s *Service) deleteEmptyCreatedSession(ctx context.Context, selection *selectedSession) {
	if selection == nil || !selection.created {
		return
	}
	if err := s.sessions.Delete(ctx, selection.Session.ID); err != nil {
		s.logger.WarnContext(ctx, "failed to remove empty active group after rejected check-in",
			slog.Int64("active_group_id", selection.Session.ID),
			slog.String("error", err.Error()),
		)
	}
}

// roomCapacityExceeded builds the room conflict with the tenant's
// disclosure decision. The registry default shows room details.
func (s *Service) roomCapacityExceeded(ctx context.Context, roomID int64, roomName string, occupancy, capacity int) error {
	return &devicescan.RoomCapacityExceededError{
		RoomID: roomID, RoomName: roomName, CurrentOccupancy: occupancy, MaxCapacity: capacity,
		Details: s.capacityDetailsDisclosed(ctx, ports.CapacityRoom, true),
	}
}

// capacityDetailsDisclosed resolves a disclosure setting, failing safe to
// the registry default so a settings outage never becomes a 500.
func (s *Service) capacityDetailsDisclosed(ctx context.Context, kind ports.CapacityKind, fallback bool) bool {
	disclosed, err := s.settings.CapacityDetailsDisclosed(ctx, kind)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to resolve capacity detail setting; using fallback",
			slog.Int("capacity_kind", int(kind)),
			slog.Bool("fallback", fallback),
			slog.String("error", err.Error()),
		)
		return fallback
	}
	return disclosed
}

func (s *Service) checkRoomCapacity(ctx context.Context, room facilities.Room) error {
	if room.Capacity == nil || *room.Capacity <= 0 {
		return nil
	}
	occupancy, err := s.presence.CountOpenVisitsInRoom(ctx, room.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to count room occupancy",
			slog.Int64("room_id", room.ID),
			slog.String("error", err.Error()),
		)
		return devicescan.Internal(devicescan.MessageCheckRoomCapacityFailed, nil)
	}
	if occupancy >= *room.Capacity {
		return s.roomCapacityExceeded(ctx, room.ID, room.Name, occupancy, *room.Capacity)
	}
	return nil
}

// checkActivityCapacity validates that the session's activity has room for
// another child. Spontaneous sessions (no template) are unsupported here.
func (s *Service) checkActivityCapacity(ctx context.Context, session *ports.Session) error {
	activity := session.Activity
	if activity == nil {
		if session.TemplateID == nil {
			s.logger.ErrorContext(ctx, "spontaneous active group reached IoT capacity check — unsupported in this flow",
				slog.Int64("active_group_id", session.ID),
			)
			return devicescan.Internal(devicescan.MessageSpontaneousUnsupported, nil)
		}
		if s.activities == nil {
			return devicescan.Internal(devicescan.MessageGetActivityFailed, nil)
		}
		loaded, err := s.activities.Find(ctx, *session.TemplateID)
		if err != nil || loaded == nil {
			s.logger.ErrorContext(ctx, "failed to get activity group",
				slog.Int64("activity_group_id", *session.TemplateID),
				slog.String("error", errorText(err)),
			)
			return devicescan.Internal(devicescan.MessageGetActivityFailed, nil)
		}
		activity = loaded
	}
	if !activity.HasParticipantLimit() {
		return nil
	}

	occupancy, err := s.presence.CountOpenVisitsInGroup(ctx, session.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to count activity occupancy",
			slog.Int64("active_group_id", session.ID),
			slog.String("error", err.Error()),
		)
		return devicescan.Internal(devicescan.MessageCheckActivityCapFailed, nil)
	}
	if occupancy >= activity.MaxParticipants {
		s.logger.WarnContext(ctx, "activity is at capacity",
			slog.String("activity_name", activity.Name),
			slog.Int64("activity_id", activity.ID),
			slog.Int("current_occupancy", occupancy),
			slog.Int("max_participants", activity.MaxParticipants),
		)
		return &devicescan.ActivityCapacityExceededError{
			ActivityID: activity.ID, ActivityName: activity.Name, CurrentOccupancy: occupancy, MaxCapacity: activity.MaxParticipants,
			Details: s.capacityDetailsDisclosed(ctx, ports.CapacityActivity, false),
		}
	}
	s.logger.DebugContext(ctx, "activity capacity check passed",
		slog.String("activity_name", activity.Name),
		slog.Int("current_occupancy", occupancy),
		slog.Int("max_participants", activity.MaxParticipants),
	)
	return nil
}

func errorText(err error) string {
	if err == nil {
		return "activity not found"
	}
	return err.Error()
}

// findOrCreateSessionForRoom finds a running session in the room or
// provisions one for a special room (released Schulhof, WC). With several
// candidates it prefers the one linked to the scanning device.
func (s *Service) findOrCreateSessionForRoom(ctx context.Context, room facilities.Room, deviceID int64) (*selectedSession, error) {
	s.logger.DebugContext(ctx, "looking for active groups in room",
		slog.Int64("room_id", room.ID),
		slog.Int64("device_id", deviceID),
	)
	sessions, err := s.sessions.ListOpenInRoom(ctx, room.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to find active groups in room",
			slog.Int64("room_id", room.ID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(devicescan.MessageFindActiveGroupsFailed, nil)
	}

	// The same-day filter is limited to the rooms that provision their own
	// session: only there is closing yesterday's stale session safe. For
	// regular rooms a prior-day session that outlived the session-end cutoff
	// must stay reusable, or scans would 404 until the next daily cleanup.
	current := sessions
	if isSelfProvisioningRoom(room) {
		now := s.now()
		if err := s.endPreviousDaySessions(ctx, sessions, now); err != nil {
			s.logger.ErrorContext(ctx, "failed to end stale special-room session",
				slog.Int64("room_id", room.ID),
				slog.String("room_name", room.Name),
				slog.String("error", err.Error()),
			)
			return nil, specialRoomCreationError(room.Name)
		}
		current = s.sessionsStartedToday(sessions, now)
	}
	if len(current) > 0 {
		return s.useExistingSession(ctx, current, room, deviceID), nil
	}
	return s.createSpecialRoomSession(ctx, room)
}

func (s *Service) sessionsStartedToday(sessions []ports.Session, now time.Time) []ports.Session {
	today := s.clock.Day(now)
	current := make([]ports.Session, 0, len(sessions))
	for _, session := range sessions {
		if session.EndTime != nil || s.clock.Day(session.StartTime) != today {
			continue
		}
		current = append(current, session)
	}
	return current
}

func (s *Service) endPreviousDaySessions(ctx context.Context, sessions []ports.Session, now time.Time) error {
	today := s.clock.Day(now)
	for _, session := range sessions {
		if session.EndTime != nil || !s.clock.Day(session.StartTime).Before(today) {
			continue
		}
		if err := s.sessions.End(ctx, session.ID); err != nil {
			return fmt.Errorf("end stale active group %d: %w", session.ID, err)
		}
	}
	return nil
}

// useExistingSession selects a session in the room, preferring one linked
// to the scanning device and otherwise the newest start. Newest-wins is
// deliberate: since the Schulhof became a plannable room several open
// sessions can coexist in one room, and the lookup carries no order, so the
// first row would make the scan target a coin flip.
func (s *Service) useExistingSession(ctx context.Context, sessions []ports.Session, room facilities.Room, deviceID int64) *selectedSession {
	selected := sessions[0]
	deviceMatched := false
	for _, candidate := range sessions {
		if candidate.DeviceID != nil && *candidate.DeviceID == deviceID {
			selected = candidate
			deviceMatched = true
			break
		}
		if candidate.StartTime.After(selected.StartTime) {
			selected = candidate
		}
	}
	s.logger.DebugContext(ctx, "selected active group in room",
		slog.Int("group_count", len(sessions)),
		slog.Int64("room_id", room.ID),
		slog.Int64("device_id", deviceID),
		slog.Int64("active_group_id", selected.ID),
		slog.Bool("device_matched", deviceMatched),
	)
	return &selectedSession{Session: selected, RoomName: room.Name, DeviceScoped: deviceMatched}
}

// isReleasedSchulhofRoom reports the canonical Schulhof while it is still
// released by the administration (#3064). The release is what earns the
// yard its permanent kiosk journey; removing it puts the ordinary room rules
// back, server-side.
func isReleasedSchulhofRoom(room facilities.Room) bool {
	return room.Name == facilities.SchulhofRoomName && room.IsOpenRoom
}

// isSelfProvisioningRoom reports a room that provisions its own kiosk
// session on scan, which is what makes closing yesterday's session safe.
func isSelfProvisioningRoom(room facilities.Room) bool {
	return isReleasedSchulhofRoom(room) || facilities.IsToiletRoomName(room.Name)
}

// createSpecialRoomSession provisions the session of a special room
// (released Schulhof, WC). The session belongs to the room, never to the
// scanning device: a device link would hijack the device's own session.
func (s *Service) createSpecialRoomSession(ctx context.Context, room facilities.Room) (*selectedSession, error) {
	var activity *ports.Activity
	var err error
	switch {
	case isReleasedSchulhofRoom(room):
		s.logger.InfoContext(ctx, "auto-creating Schulhof active group", slog.Int64("room_id", room.ID))
		activity, err = s.schulhofActivity(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to find Schulhof activity", slog.String("error", err.Error()))
			return nil, devicescan.Internal(devicescan.MessageSchulhofNotConfigured, nil)
		}
	case facilities.IsToiletRoomName(room.Name):
		s.logger.InfoContext(ctx, "auto-creating WC active group", slog.Int64("room_id", room.ID))
		activity, err = s.wcActivity(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to find WC activity", slog.String("error", err.Error()))
			message := devicescan.MessageWCNotConfigured
			if strings.Contains(err.Error(), "staff context") {
				message = devicescan.MessageWCNeedsStaff
			}
			return nil, devicescan.Internal(message, nil)
		}
	default:
		s.logger.WarnContext(ctx, "no active groups found in room",
			slog.Int64("room_id", room.ID),
			slog.String("room_name", room.Name),
		)
		return nil, devicescan.NotFound(devicescan.MessageNoGroupsInRoom)
	}

	session, err := s.sessions.Start(ctx, ports.NewSession{ActivityID: activity.ID, RoomID: room.ID})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create active group for special room",
			slog.String("room_name", room.Name),
			slog.String("error", err.Error()),
		)
		return nil, specialRoomCreationError(room.Name)
	}
	session.Activity = activity
	s.logger.InfoContext(ctx, "auto-created active group for special room",
		slog.String("room_name", room.Name),
		slog.Int64("active_group_id", session.ID),
	)
	return &selectedSession{Session: session, RoomName: room.Name, created: true}, nil
}

func specialRoomCreationError(roomName string) error {
	switch {
	case roomName == facilities.SchulhofRoomName:
		return devicescan.Internal(devicescan.MessageCreateSchulhofFailed, nil)
	case facilities.IsToiletRoomName(roomName):
		return devicescan.Internal(devicescan.MessageCreateWCFailed, nil)
	default:
		return devicescan.Internal(devicescan.MessageCreateSessionFailed, nil)
	}
}

// roomNameByID resolves a room name by lookup, falling back to a
// placeholder that names the id.
func (s *Service) roomNameByID(ctx context.Context, roomID int64) string {
	if room, err := s.rooms.FindRoom(ctx, roomID); err == nil {
		return room.Name
	}
	return fmt.Sprintf("Room %d", roomID)
}

// roomNameForResponse names the room of a skipped check-in.
func (s *Service) roomNameForResponse(ctx context.Context, currentVisit *ports.CurrentVisit, roomID *int64) string {
	if currentVisit != nil && currentVisit.Session != nil && currentVisit.Session.Room != nil {
		return currentVisit.Session.Room.Name
	}
	if roomID != nil {
		return s.roomNameByID(ctx, *roomID)
	}
	return ""
}

// resolveActiveStudentCount refreshes the heartbeat of the session tied to
// the scan and returns the count the kiosk shows: scoped to the scanning
// device's session where possible, otherwise the whole room.
func (s *Service) resolveActiveStudentCount(ctx context.Context, checkin *checkinResult, roomID, deviceID int64) *int {
	if checkin.SessionID != nil && checkin.DeviceScopedRoom {
		s.touchSession(ctx, *checkin.SessionID)
		return s.activeStudentCountForSession(ctx, *checkin.SessionID)
	}
	if session := s.deviceSessionInRoom(ctx, roomID, deviceID); session != nil {
		s.touchSession(ctx, session.ID)
		return s.activeStudentCountForSession(ctx, session.ID)
	}
	return s.activeStudentCountForRoom(ctx, roomID)
}

func (s *Service) deviceSessionInRoom(ctx context.Context, roomID, deviceID int64) *ports.Session {
	session, err := s.sessions.FindDeviceSessionInRoom(ctx, roomID, deviceID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to find active group for room and device",
			slog.Int64("room_id", roomID),
			slog.Int64("device_id", deviceID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return session
}

func (s *Service) activeStudentCountForRoom(ctx context.Context, roomID int64) *int {
	count, err := s.presence.CountOpenVisitsInRoom(ctx, roomID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to count active students for room",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &count
}

func (s *Service) activeStudentCountForSession(ctx context.Context, sessionID int64) *int {
	count, err := s.presence.CountOpenVisitsInGroup(ctx, sessionID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to count active students for group",
			slog.Int64("active_group_id", sessionID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return &count
}

// touchSession refreshes the session heartbeat; failures only log.
func (s *Service) touchSession(ctx context.Context, sessionID int64) {
	if err := s.sessions.Touch(ctx, sessionID); err != nil {
		s.logger.WarnContext(ctx, "failed to update session activity for group",
			slog.Int64("group_id", sessionID),
			slog.String("error", err.Error()),
		)
	}
}
