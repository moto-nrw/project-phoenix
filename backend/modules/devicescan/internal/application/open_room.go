package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

var _ devicescan.OpenRoomBooking = (*Service)(nil)

// BookOpenRoom records the child behind the card as an independent stay in a
// released room, chosen at the device the child leaves (#3067). The move is
// the open-room move the phone uses: it requires the release, provisions the
// room's own session, and never books the child into an activity running in
// that room. The destination needs no device, no second scan and no
// supervision.
func (s *Service) BookOpenRoom(ctx context.Context, command devicescan.OpenRoomCommand) (*devicescan.OpenRoomResult, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.unit.RequireTransaction(ctx); err != nil {
		return nil, devicescan.Internal(err.Error(), nil)
	}
	if command.RoomID <= 0 {
		return nil, devicescan.InvalidRequest(devicescan.MessageOpenRoomIDRequired)
	}
	person, student, err := s.openRoomStudent(ctx, device, command.RFIDTag)
	if err != nil {
		return nil, err
	}

	outcome, err := s.openRooms.MoveToOpenRoom(ctx, devicescan.OpenRoomMove{StudentID: student.ID, RoomID: command.RoomID})
	if err != nil {
		// A refused move must not leave a half-written stay behind in the
		// request transaction.
		s.unit.MarkRollback(ctx)
		return nil, s.classifyOpenRoomFailure(ctx, err)
	}

	roomName := ""
	if room, roomErr := s.rooms.FindRoom(ctx, command.RoomID); roomErr == nil {
		roomName = room.Name
	}
	s.logger.InfoContext(ctx, "open room stay booked at device",
		slog.String("device_id", device.DeviceID),
		slog.Int64("student_id", student.ID),
		slog.Int64("room_id", command.RoomID),
		slog.Int64("room_session_id", outcome.RoomSessionID),
		slog.Bool("moved", outcome.Moved),
	)
	return &devicescan.OpenRoomResult{
		StudentID:     student.ID,
		StudentName:   person.FirstName + " " + person.LastName,
		RoomID:        command.RoomID,
		RoomName:      roomName,
		RoomSessionID: outcome.RoomSessionID,
		Moved:         outcome.Moved,
		Action:        devicescan.ScanActionOpenRoomStay,
		Message:       openRoomMessage(person.FirstName, roomName),
	}, nil
}

// openRoomStudent resolves the child behind the card and checks that the
// tenant can book open rooms at all.
func (s *Service) openRoomStudent(ctx context.Context, device *ports.Device, tag string) (*ports.Person, *ports.Student, error) {
	person, err := s.resolvePerson(ctx, device, tag)
	if err != nil {
		return nil, nil, err
	}
	student := s.lookupStudent(ctx, person.ID)
	if student == nil {
		return nil, nil, devicescan.NotFound(devicescan.MessagePersonNotStudent)
	}
	// Binary-mode tenants track attendance only: there is no room a child
	// could stay in, so the destination choice does not exist for them.
	mode, err := s.settings.PresenceMode(ctx)
	if err != nil {
		return nil, nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
	}
	if mode == presenceModeBinary {
		return nil, nil, &devicescan.Failure{Kind: devicescan.FailureConflict, Message: devicescan.MessageOpenRoomBinaryMode, Code: devicescan.CodeOpenRoomBinaryMode}
	}
	if s.openRooms == nil {
		return nil, nil, devicescan.Internal(devicescan.MessageOpenRoomUnavailable, errors.New("open room mover not composed"))
	}
	return person, student, nil
}

// classifyOpenRoomFailure turns the mover's sentinels into the refusals the
// kiosk renders. A full room keeps the capacity body the check-in uses.
func (s *Service) classifyOpenRoomFailure(ctx context.Context, err error) error {
	if capacity, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err); ok {
		capacity.Details = s.capacityDetailsDisclosed(ctx, ports.CapacityRoom, true)
		return capacity
	}
	if _, ok := errors.AsType[*devicescan.StudentAlreadyActiveError](err); ok {
		return err
	}
	switch {
	case errors.Is(err, devicescan.ErrOpenRoomNotFound):
		return devicescan.NotFoundWithCode(devicescan.MessageOpenRoomNotFound, devicescan.CodeOpenRoomNotFound)
	case errors.Is(err, devicescan.ErrOpenRoomNotReleased):
		return &devicescan.Failure{Kind: devicescan.FailureConflict, Message: devicescan.MessageOpenRoomNotReleased, Code: devicescan.CodeOpenRoomNotReleased}
	case errors.Is(err, devicescan.ErrOpenRoomStudentNotPresent):
		return &devicescan.Failure{Kind: devicescan.FailureConflict, Message: devicescan.MessageStudentNotPresent, Code: devicescan.CodeStudentNotPresent}
	}
	return devicescan.Internal(devicescan.MessageCreateVisitFailed, err)
}

func openRoomMessage(firstName, roomName string) string {
	if roomName == "" {
		return fmt.Sprintf("%s ist jetzt im offenen Raum.", firstName)
	}
	return fmt.Sprintf("%s ist jetzt in %s.", firstName, roomName)
}
