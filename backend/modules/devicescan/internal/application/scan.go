package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// Presence modes of a tenant.
const (
	presenceModeDetailed = "detailed"
	presenceModeBinary   = "binary"
)

// Scan processes one card scan. A student card runs the detailed room stay
// transition or, for a binary-mode tenant, the attendance toggle; a staff
// card joins the device session as supervisor. Every write of one scan runs
// in the request transaction, so a refused check-in after a checkout is
// rolled back as a whole.
func (s *Service) Scan(ctx context.Context, command devicescan.ScanCommand) (*devicescan.ScanResult, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.DebugContext(ctx, "starting checkin process",
		slog.String("device_id", device.DeviceID),
		slog.Int64("device_db_id", device.ID),
	)
	s.logger.DebugContext(ctx, "checkin request details",
		slog.String("student_rfid", command.RFIDTag),
		slog.Any("room_id", command.RoomID),
	)

	person, err := s.resolvePerson(ctx, device, command.RFIDTag)
	if err != nil {
		return nil, err
	}

	student := s.lookupStudent(ctx, person.ID)
	if student == nil {
		return s.scanStaff(ctx, device, person)
	}
	// Debug, not Info: the school class is an attribute of the child and has
	// no operational value in retained logs (#2062).
	s.logger.DebugContext(ctx, "found student",
		slog.Int64("student_id", student.ID),
		slog.String("class", student.SchoolClass),
	)

	// Binary-mode tenants track attendance only: no visits, no rooms, no
	// sessions. The response keeps the detailed shape so older kiosks keep
	// working.
	mode, err := s.settings.PresenceMode(ctx)
	if err != nil {
		return nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
	}
	if mode == presenceModeBinary {
		return s.scanBinary(ctx, device, student, person)
	}
	return s.scanVisit(ctx, device, student, person, command)
}

// resolvePerson turns the card into a person, records unknown cards for
// the operator review, and keeps the pinned wire errors of the scan.
func (s *Service) resolvePerson(ctx context.Context, device *ports.Device, tag string) (*ports.Person, error) {
	s.logger.DebugContext(ctx, "looking up RFID tag", slog.String("rfid", tag))
	person, err := s.people.FindPersonByTag(ctx, tag)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			s.recordUnregisteredScan(ctx, device, tag)
			s.logger.WarnContext(ctx, "RFID tag not found", slog.String("rfid", tag))
			return nil, devicescan.NotFoundWithCode(devicescan.MessageRFIDTagNotFound, devicescan.CodeRFIDTagNotFound)
		}
		s.logger.ErrorContext(ctx, "failed to lookup RFID tag",
			slog.String("rfid", tag),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
	}
	if person == nil || !person.HasTag {
		s.logger.WarnContext(ctx, "RFID tag not assigned to any person", slog.String("rfid", tag))
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagUnassigned)
	}
	s.logger.DebugContext(ctx, "RFID tag belongs to person",
		slog.String("rfid", tag),
		slog.String("person_name", person.FirstName+" "+person.LastName),
		slog.Int64("person_id", person.ID),
	)
	return person, nil
}

// lookupStudent resolves the student behind a person. A lookup failure is
// logged and treated as "no student", so the scan continues as a staff scan.
func (s *Service) lookupStudent(ctx context.Context, personID int64) *ports.Student {
	student, err := s.people.FindStudentByPerson(ctx, personID)
	if err != nil {
		s.logger.DebugContext(ctx, "student lookup for person",
			slog.Int64("person_id", personID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	// A graduated alumnus is soft-deleted by the grade transition: treat the
	// card like an unknown one so it opens neither a visit nor an attendance
	// row (#405). Care-ended children stay resolvable so an open stay can end.
	if student != nil && student.Alumnus {
		return nil
	}
	return student
}

// scanStaff joins a staff card to the device session as supervisor.
func (s *Service) scanStaff(ctx context.Context, device *ports.Device, person *ports.Person) (*devicescan.ScanResult, error) {
	s.logger.DebugContext(ctx, "person is not a student, checking if staff",
		slog.Int64("person_id", person.ID),
	)
	staff, err := s.people.FindStaffByPerson(ctx, person.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to lookup staff for person",
			slog.Int64("person_id", person.ID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagNotStudentOrStaff)
	}
	if staff == nil {
		s.logger.WarnContext(ctx, "person is neither student nor staff",
			slog.Int64("person_id", person.ID),
		)
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagNotStudentOrStaff)
	}

	s.logger.InfoContext(ctx, "found staff, attempting supervisor authentication",
		slog.Int64("staff_id", staff.ID),
	)
	session, err := s.addSupervisor(ctx, device, staff.ID)
	if err != nil {
		return nil, err
	}
	message := "Supervisor authenticated"
	if session.ActivityName != "" {
		message = "Supervisor authenticated for " + session.ActivityName
	}
	return &devicescan.ScanResult{
		Outcome:     devicescan.ScanOutcomeSupervisor,
		PersonID:    staff.ID,
		PersonName:  person.FirstName + " " + person.LastName,
		Action:      devicescan.ScanActionSupervisorAuthenticated,
		RoomName:    session.RoomName,
		Message:     message,
		ProcessedAt: s.now(),
	}, nil
}

// addSupervisor adds the staff member to the device's running session
// (idempotent) and returns the session context for the response.
func (s *Service) addSupervisor(ctx context.Context, device *ports.Device, staffID int64) (*ports.Session, error) {
	session, err := s.sessions.Current(ctx, device.ID)
	if err != nil || session == nil {
		s.logger.InfoContext(ctx, "no active session for supervisor scan",
			slog.String("device_id", device.DeviceID),
			slog.Int64("device_db_id", device.ID),
		)
		return nil, devicescan.NotFound(devicescan.MessageNoActiveSession)
	}
	s.logger.InfoContext(ctx, "processing supervisor authentication",
		slog.String("device_id", device.DeviceID),
		slog.Int64("session_id", session.ID),
		slog.Int64("staff_id", staffID),
	)

	supervisors, err := s.sessions.Supervisors(ctx, session.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load supervisors for session",
			slog.Int64("session_id", session.ID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(devicescan.MessageLoadSupervisorsFailed, err)
	}

	var supervisorIDs []int64
	alreadySupervisor := false
	for _, supervisor := range supervisors {
		if supervisor.Ended {
			continue
		}
		supervisorIDs = append(supervisorIDs, supervisor.StaffID)
		if supervisor.StaffID == staffID {
			alreadySupervisor = true
		}
	}

	if alreadySupervisor {
		s.logger.DebugContext(ctx, "staff is already a supervisor (idempotent)",
			slog.Int64("staff_id", staffID),
			slog.Int64("session_id", session.ID),
		)
		return session, nil
	}
	supervisorIDs = append(supervisorIDs, staffID)
	if err := s.sessions.ReplaceSupervisors(ctx, session.ID, supervisorIDs); err != nil {
		s.logger.ErrorContext(ctx, "failed to add supervisor to session",
			slog.Int64("staff_id", staffID),
			slog.Int64("session_id", session.ID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(devicescan.MessageUpdateSupervisorsFailed, err)
	}
	s.logger.InfoContext(ctx, "added staff as supervisor to session",
		slog.Int64("staff_id", staffID),
		slog.Int64("session_id", session.ID),
	)
	return session, nil
}

// scanBinary toggles today's attendance row for a binary-mode tenant.
// Legacy clients only authenticate the device and shared OGS PIN; those
// scans stay device-attributed instead of trusting a caller-controlled staff
// id, so the staff comes from the verified account PIN alone.
func (s *Service) scanBinary(ctx context.Context, device *ports.Device, student *ports.Student, person *ports.Person) (*devicescan.ScanResult, error) {
	var staffID int64
	if staff, ok := s.principals.Staff(ctx); ok && staff != nil {
		staffID = staff.ID
	}

	// skipAuthCheck: the device and staff-PIN layer already authorized this
	// scan; the default path would also require a session supervisor, which
	// binary mode never has.
	action, err := s.attendance.Toggle(ctx, student.ID, staffID, device.ID, true)
	if err != nil {
		// A graduation or care exit that committed between the lookup and
		// the write is the same 404 every attendance path returns (#405,
		// #2487): the kiosk has one mapping for "not a child we care for".
		if errors.Is(err, ports.ErrStudentNotInCare) {
			s.logger.InfoContext(ctx, "binary mode checkin rejected: student not in care",
				slog.Int64("student_id", student.ID),
				slog.Int64("staff_id", staffID),
			)
			return nil, devicescan.NotFound(devicescan.MessagePersonNotStudent)
		}
		s.logger.ErrorContext(ctx, "binary mode attendance toggle failed",
			slog.Int64("student_id", student.ID),
			slog.Int64("staff_id", staffID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(err.Error(), err)
	}

	s.logger.InfoContext(ctx, "binary mode checkin complete",
		slog.String("action", action),
		slog.Int64("student_id", student.ID),
		slog.Int64("staff_id", staffID),
	)
	return &devicescan.ScanResult{
		Outcome:     devicescan.ScanOutcomeAttendance,
		PersonID:    student.ID,
		PersonName:  person.FirstName + " " + person.LastName,
		Action:      action,
		Message:     binaryModeGreeting(action, person.FirstName),
		ProcessedAt: s.now(),
	}, nil
}

// binaryModeGreeting is the German confirmation of a binary-mode scan.
func binaryModeGreeting(action, firstName string) string {
	switch action {
	case devicescan.ScanActionCheckedIn:
		return "Willkommen, " + firstName + "!"
	case devicescan.ScanActionCheckedOut:
		return "Tschüss, " + firstName + "!"
	default:
		return "Anwesenheit aktualisiert"
	}
}

// scanVisit runs the detailed presence transition: close the current room
// stay, open the requested one, and render the kiosk projection.
func (s *Service) scanVisit(ctx context.Context, device *ports.Device, student *ports.Student, person *ports.Person, command devicescan.ScanCommand) (*devicescan.ScanResult, error) {
	now := s.now()

	checkout, err := s.checkoutCurrentVisit(ctx, student, person)
	if err != nil {
		return nil, err
	}

	skipCheckin := s.shouldSkipCheckin(command.RoomID, checkout, now)
	if skipCheckin {
		s.logger.DebugContext(ctx, "skipping re-checkin to same room",
			slog.Int64("room_id", *command.RoomID),
		)
	}

	checkin, err := s.processStudentCheckin(ctx, student, person, checkinInput{
		RoomID: command.RoomID, DeviceID: device.ID, SkipCheckin: skipCheckin, Checkout: checkout,
	})
	if err != nil {
		if shouldRollbackDestinationCheckin(checkout.CheckedOut, err) {
			s.unit.MarkRollback(ctx)
		}
		return nil, err
	}

	result := buildScanResult(person, checkout, checkin)
	if result.Action == "" {
		s.logger.WarnContext(ctx, "no action determined for student",
			slog.Int64("student_id", student.ID),
		)
		result.Action = devicescan.ScanActionNoAction
		result.Message = "Keine Aktion durchgeführt"
	}

	if s.shouldUpgradeToDailyCheckout(ctx, result.Action, student, checkout.Visit) {
		result.Action = devicescan.ScanActionCheckedOutDaily
	}

	s.applyCheckoutFlags(ctx, result, student, checkout.Visit)
	if command.RoomID != nil {
		result.ActiveStudents = s.resolveActiveStudentCount(ctx, checkin, *command.RoomID, device.ID)
	}
	s.applyPickupTime(ctx, result, student.ID, now)

	result.PersonID = student.ID
	result.PersonName = person.FirstName + " " + person.LastName
	result.ProcessedAt = now
	// The greeting embeds the child's first name: kiosk response only, never
	// a retained log line (#2062).
	s.logger.InfoContext(ctx, "checkin complete",
		slog.String("action", result.Action),
		slog.Int64("student_id", student.ID),
		slog.Any("visit_id", result.VisitID),
		slog.String("room", result.RoomName),
	)
	return result, nil
}

// shouldRollbackDestinationCheckin reports whether a refused destination
// must undo the source checkout that already happened in this request.
func shouldRollbackDestinationCheckin(checkedOut bool, err error) bool {
	if !checkedOut {
		return false
	}
	var roomCapacity *devicescan.RoomCapacityExceededError
	var activityCapacity *devicescan.ActivityCapacityExceededError
	return errors.As(err, &roomCapacity) || errors.As(err, &activityCapacity)
}

// buildScanResult names the transition from what happened.
func buildScanResult(person *ports.Person, checkout currentVisitCheckout, checkin *checkinResult) *devicescan.ScanResult {
	result := &devicescan.ScanResult{Outcome: devicescan.ScanOutcomeVisit}
	switch {
	case checkout.CheckedOut && checkin.NewVisitID != nil:
		if checkout.PreviousRoomName != "" && checkout.PreviousRoomName != checkin.RoomName {
			result.Action = devicescan.ScanActionTransferred
			result.Message = "Gewechselt von " + checkout.PreviousRoomName + " zu " + checkin.RoomName + "!"
		} else {
			result.Action = devicescan.ScanActionCheckedIn
			result.Message = "Hallo " + person.FirstName + "!"
		}
		result.VisitID = checkin.NewVisitID
	case checkout.CheckedOut:
		result.Action = devicescan.ScanActionCheckedOut
		result.Message = "Tschüss " + person.FirstName + "!"
		result.VisitID = checkout.VisitID
	case checkin.NewVisitID != nil:
		result.Action = devicescan.ScanActionCheckedIn
		result.Message = "Hallo " + person.FirstName + "!"
		result.VisitID = checkin.NewVisitID
	}
	result.RoomName = checkin.RoomName
	result.PreviousRoomName = checkout.PreviousRoomName
	return result
}

// applyCheckoutFlags sets the daily-checkout-available and feedback flags.
// Both only mean something when the child just left a room, so they keep
// their defaults otherwise. Feedback is opt-in (GDPR).
func (s *Service) applyCheckoutFlags(ctx context.Context, result *devicescan.ScanResult, student *ports.Student, currentVisit *ports.CurrentVisit) {
	if currentVisit == nil || (result.Action != devicescan.ScanActionCheckedOut && result.Action != devicescan.ScanActionCheckedOutDaily) {
		return
	}
	result.DailyCheckoutAvailable = s.shouldShowDailyCheckoutWithGroup(ctx, student, currentVisit)
	if enabled, err := s.settings.FeedbackEnabled(ctx); err == nil {
		result.FeedbackEnabled = enabled
	}
}
