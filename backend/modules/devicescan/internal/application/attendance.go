package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

func isPersonNotFound(err error) bool { return errors.Is(err, ports.ErrPersonNotFound) }

// AttendanceStatus answers today's attendance of a student card.
func (s *Service) AttendanceStatus(ctx context.Context, rfidTag string) (*devicescan.AttendanceStatus, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	if rfidTag == "" {
		return nil, devicescan.InvalidRequest(devicescan.MessageRFIDParameterRequired)
	}
	tag := s.people.NormalizeTag(rfidTag)
	student, person, err := s.findAttendanceStudent(ctx, device, tag)
	if err != nil {
		return nil, err
	}
	state, err := s.attendance.Status(ctx, student.ID)
	if err != nil {
		return nil, devicescan.Internal(err.Error(), err)
	}
	return &devicescan.AttendanceStatus{
		Student:    s.attendanceStudent(ctx, student, person),
		Attendance: publicAttendanceState(state),
	}, nil
}

// ToggleAttendance confirms, cancels or finalizes today's attendance.
func (s *Service) ToggleAttendance(ctx context.Context, command devicescan.AttendanceToggleCommand) (*devicescan.AttendanceToggleResult, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.unit.RequireTransaction(ctx); err != nil {
		return nil, devicescan.Internal(err.Error(), nil)
	}
	if command.Action == devicescan.AttendanceActionCancel {
		return &devicescan.AttendanceToggleResult{
			Action:  devicescan.AttendanceActionCancelled,
			Message: "Attendance tracking cancelled",
		}, nil
	}
	tag := s.people.NormalizeTag(command.RFIDTag)
	if command.Action == devicescan.AttendanceActionDailyCheckout {
		return s.confirmDailyCheckout(ctx, device, tag, command.Destination)
	}
	return s.toggleAttendance(ctx, device, tag)
}

// findAttendanceStudent resolves a card for the attendance flows. Alumni are
// never valid; care-ended children continue so an open presence can close.
func (s *Service) findAttendanceStudent(ctx context.Context, device *ports.Device, tag string) (*ports.Student, *ports.Person, error) {
	person, err := s.people.FindPersonByTag(ctx, tag)
	if err != nil {
		if !isPersonNotFound(err) {
			return nil, nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
		}
		s.recordUnregisteredScan(ctx, device, tag)
		return nil, nil, devicescan.NotFound(devicescan.MessageRFIDTagNotFound)
	}
	if person == nil || !person.HasTag {
		return nil, nil, devicescan.NotFound(devicescan.MessageRFIDTagUnassigned)
	}
	student, err := s.people.FindStudentByPerson(ctx, person.ID)
	if err != nil || student == nil || student.Alumnus {
		return nil, nil, devicescan.NotFound(devicescan.MessagePersonNotStudent)
	}
	return student, person, nil
}

// attendanceStudent projects the child with the education group the kiosk
// shows; a missing group only means no group info.
func (s *Service) attendanceStudent(ctx context.Context, student *ports.Student, person *ports.Person) devicescan.AttendanceStudent {
	result := devicescan.AttendanceStudent{ID: student.ID, FirstName: person.FirstName, LastName: person.LastName}
	if student.GroupID == nil || s.groups == nil {
		return result
	}
	group, err := s.groups.Find(ctx, *student.GroupID)
	if err != nil || group == nil {
		return result
	}
	result.Group = &devicescan.AttendanceGroup{ID: group.ID, Name: group.Name}
	return result
}

func publicAttendanceState(state *ports.AttendanceState) devicescan.AttendanceState {
	if state == nil {
		return devicescan.AttendanceState{}
	}
	return devicescan.AttendanceState{
		Status: state.Status, Date: state.Date.String(), CheckInTime: state.CheckInTime, CheckOutTime: state.CheckOutTime,
		CheckedInBy: state.CheckedInBy, CheckedOutBy: state.CheckedOutBy,
	}
}

// confirmDailyCheckout resolves the deferred "nach Hause" decision. The
// visit was normally already ended by the scan (the child is "unterwegs");
// this only updates the attendance row when the child confirms going home.
func (s *Service) confirmDailyCheckout(ctx context.Context, device *ports.Device, tag, destination string) (*devicescan.AttendanceToggleResult, error) {
	person, err := s.people.FindPersonByTag(ctx, tag)
	if err != nil && !isPersonNotFound(err) {
		return nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
	}
	if err != nil || person == nil {
		s.recordUnregisteredScan(ctx, device, tag)
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagNotFound)
	}
	student, err := s.people.FindStudentByPerson(ctx, person.ID)
	if err != nil || student == nil || student.Alumnus {
		return nil, devicescan.NotFound(devicescan.MessagePersonNotStudent)
	}

	action, err := s.attendance.ConfirmDailyCheckout(ctx, student.ID, device.ID, destination)
	if err != nil {
		if errors.Is(err, ports.ErrNoAttendanceRecord) {
			return nil, devicescan.NotFound(err.Error())
		}
		return nil, classifyPresenceFailure(err)
	}

	message := "Tschüss " + person.FirstName + "!"
	if destination == devicescan.DestinationTransit {
		message = "Viel Spaß!"
	}
	result := &devicescan.AttendanceToggleResult{
		Action:  action,
		Message: message,
		Student: devicescan.AttendanceStudent{ID: student.ID, FirstName: person.FirstName, LastName: person.LastName},
	}
	if enabled, err := s.settings.FeedbackEnabled(ctx); err == nil {
		result.FeedbackEnabled = &enabled
	}
	return result, nil
}

// toggleAttendance flips today's row. Staff attribution comes from the
// verified account PIN; a plain device scan stays device-attributed.
func (s *Service) toggleAttendance(ctx context.Context, device *ports.Device, tag string) (*devicescan.AttendanceToggleResult, error) {
	person, err := s.people.FindPersonByTag(ctx, tag)
	if err != nil {
		if !isPersonNotFound(err) {
			return nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
		}
		s.recordUnregisteredScan(ctx, device, tag)
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagNotFound)
	}
	if person == nil || !person.HasTag {
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagUnassigned)
	}
	student, err := s.people.FindStudentByPerson(ctx, person.ID)
	if err != nil || student == nil || student.Alumnus {
		return nil, devicescan.NotFound(devicescan.MessagePersonNotStudent)
	}

	var staffID int64
	if staff, ok := s.principals.Staff(ctx); ok && staff != nil {
		staffID = staff.ID
	}
	action, err := s.attendance.Toggle(ctx, student.ID, staffID, device.ID, false)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to toggle attendance",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		return nil, classifyPresenceFailure(err)
	}

	state, err := s.attendance.Status(ctx, student.ID)
	if err != nil {
		return nil, devicescan.Internal(err.Error(), err)
	}
	attendance := publicAttendanceState(state)
	return &devicescan.AttendanceToggleResult{
		Action:     action,
		Student:    s.attendanceStudent(ctx, student, person),
		Attendance: &attendance,
		Message:    attendanceMessage(action, person.FirstName),
	}, nil
}

// classifyPresenceFailure maps a classified presence refusal to the kiosk
// contract, keeping the retained service's wording on the wire.
func classifyPresenceFailure(err error) error {
	switch {
	case errors.Is(err, ports.ErrStudentNotInCare):
		return devicescan.NotFound(devicescan.MessagePersonNotStudent)
	case errors.Is(err, ports.ErrConflict):
		return devicescan.Conflict(err.Error())
	case errors.Is(err, ports.ErrNotFound):
		return devicescan.NotFound(err.Error())
	case errors.Is(err, ports.ErrInvalid):
		return devicescan.InvalidRequest(err.Error())
	default:
		return devicescan.Internal(err.Error(), err)
	}
}

// attendanceMessage is the kiosk confirmation of an attendance action.
func attendanceMessage(action, firstName string) string {
	switch action {
	case devicescan.ScanActionCheckedIn:
		return fmt.Sprintf("Hallo %s!", firstName)
	case devicescan.ScanActionCheckedOut:
		return fmt.Sprintf("Tschüss %s!", firstName)
	default:
		return fmt.Sprintf("Attendance %s for %s", action, firstName)
	}
}
