package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// PickupInfo reads today's pickup time and notes of a student card without
// opening or closing anything. Refreshing the device session heartbeat is
// best-effort.
func (s *Service) PickupInfo(ctx context.Context, rfidTag string) (*devicescan.PickupInfo, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()

	person, err := s.resolvePickupPerson(ctx, device, rfidTag)
	if err != nil {
		return nil, err
	}
	student, err := s.resolvePickupStudent(ctx, person)
	if err != nil {
		return nil, err
	}

	info := &devicescan.PickupInfo{
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		ProcessedAt: now,
	}
	if s.pickups != nil {
		pickup, err := s.pickups.Effective(ctx, student.ID, s.clock.Day(now))
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to get pickup info during pickup query",
				slog.Int64("student_id", student.ID),
				slog.String("error", err.Error()),
			)
			return nil, devicescan.Internal(err.Error(), err)
		}
		if pickup != nil {
			if pickup.Time != nil {
				info.PickupTime = pickup.Time.Format("15:04")
			}
			info.PickupNote = selectPickupNote(pickup)
		}
	}

	if session, err := s.sessions.Current(ctx, device.ID); err == nil && session != nil {
		if err := s.sessions.Touch(ctx, session.ID); err != nil {
			s.logger.WarnContext(ctx, "failed to update session activity during pickup query",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()),
			)
		}
	}
	return info, nil
}

// resolvePickupPerson keeps the pickup-query error contract: an unknown card
// is 404 with an unregistered-scan record, a lookup failure 500, and an
// unassigned card 404.
func (s *Service) resolvePickupPerson(ctx context.Context, device *ports.Device, tag string) (*ports.Person, error) {
	person, err := s.people.FindPersonByTag(ctx, tag)
	if err != nil {
		if isPersonNotFound(err) {
			s.recordUnregisteredScan(ctx, device, tag)
			s.logger.WarnContext(ctx, "RFID tag not found during pickup query", slog.String("rfid", tag))
			return nil, devicescan.NotFoundWithCode(devicescan.MessageRFIDTagNotFound, devicescan.CodeRFIDTagNotFound)
		}
		s.logger.ErrorContext(ctx, "failed to lookup RFID tag during pickup query",
			slog.String("rfid", tag),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(devicescan.MessageInternalServerError, err)
	}
	if person == nil || !person.HasTag {
		s.logger.WarnContext(ctx, "RFID tag not assigned to any person during pickup query", slog.String("rfid", tag))
		return nil, devicescan.NotFound(devicescan.MessageRFIDTagUnassigned)
	}
	return person, nil
}

// resolvePickupStudent resolves the student of a pickup query. A staff card
// is a 400, a card of neither a 404, and a lookup failure a 500.
func (s *Service) resolvePickupStudent(ctx context.Context, person *ports.Person) (*ports.Student, error) {
	student, err := s.people.FindStudentByPerson(ctx, person.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to lookup student during pickup query",
			slog.Int64("person_id", person.ID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(err.Error(), err)
	}
	if student != nil && !student.Alumnus {
		return student, nil
	}
	staff, err := s.people.FindStaffByPerson(ctx, person.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to lookup staff during pickup query",
			slog.Int64("person_id", person.ID),
			slog.String("error", err.Error()),
		)
		return nil, devicescan.Internal(err.Error(), err)
	}
	if staff != nil {
		return nil, devicescan.InvalidRequest(devicescan.MessageStudentRFIDRequiredForPickup)
	}
	return nil, devicescan.NotFound(devicescan.MessageRFIDTagNotStudentOrStaff)
}

// selectPickupNote prefers the day-specific notes over the recurring
// weekday note; blank day notes fall back to the recurring text.
func selectPickupNote(pickup *ports.Pickup) string {
	if pickup == nil {
		return ""
	}
	if len(pickup.DayNotes) > 0 {
		contents := make([]string, 0, len(pickup.DayNotes))
		for _, note := range pickup.DayNotes {
			if trimmed := strings.TrimSpace(note); trimmed != "" {
				contents = append(contents, trimmed)
			}
		}
		if len(contents) > 0 {
			return strings.Join(contents, "\n")
		}
	}
	return strings.TrimSpace(pickup.Notes)
}

// applyPickupTime looks up today's pickup time for the scan result. It is
// non-blocking: a lookup error is logged and skipped. It runs inside the
// request transaction for RLS visibility; a read cannot abort it.
func (s *Service) applyPickupTime(ctx context.Context, result *devicescan.ScanResult, studentID int64, now time.Time) {
	if s.pickups == nil {
		return
	}
	pickup, err := s.pickups.Effective(ctx, studentID, s.clock.Day(now))
	if err != nil {
		s.logger.WarnContext(ctx, "failed to get pickup time",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if pickup != nil && pickup.Time != nil {
		formatted := pickup.Time.Format("15:04")
		result.PickupTime = &formatted
	}
}
