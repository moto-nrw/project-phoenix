package application

import (
	"context"
	"fmt"

	appointmentcap "github.com/moto-ogs/project-phoenix/modules/appointments"
)

func (s *service) RespondToStaffInvitation(ctx context.Context, recipientID int64, answer InvitationAnswer) error {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	if err := validateAnswerStatus(answer.Status); err != nil {
		return err
	}

	recipient, err := s.cfg.Appointments.FindAppointmentRecipient(ctx, recipientID)
	if err != nil {
		return err
	}
	if recipient == nil || recipient.StaffID == nil || *recipient.StaffID != staff.ID {
		return ErrNotFound
	}
	if recipient.Status == appointmentcap.ResponseStatusInfo {
		return fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
	}
	appointment, err := s.findAppointment(ctx, recipient.AppointmentID)
	if err != nil {
		return err
	}
	// A soft-deleted appointment can no longer be answered — it is gone from every
	// interactive surface, so a retained recipient ID must not reach the RSVP.
	if appointment == nil || appointment.DeletedAt != nil {
		return ErrNotFound
	}
	if appointment.CancelledAt != nil {
		return fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
	}
	return s.recordAnswer(ctx, appointment, recipientID, answer)
}

func validateAnswerStatus(status string) error {
	if status != appointmentcap.ResponseStatusAccepted && status != appointmentcap.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}
	return nil
}

// recordAnswer stores the answer for the whole appointment or, when it names
// a date, for that date of the series only (#6987).
func (s *service) recordAnswer(ctx context.Context, appointment *appointmentcap.Appointment, recipientID int64, answer InvitationAnswer) error {
	occurrenceDate, err := s.answeredOccurrence(ctx, appointment, toCalendarDate(answer.OccurrenceDate))
	if err != nil {
		return err
	}
	return s.cfg.Appointments.UpdateAppointmentRecipientResponse(ctx, appointmentcap.RecipientResponse{
		RecipientID: recipientID, OccurrenceDate: occurrenceDate, Status: answer.Status,
	})
}

// answeredOccurrence checks the date an answer names and returns the date
// the answer is stored for. The zero date answers the whole appointment: no
// date, or the date of an appointment that does not recur. A date of a series
// must be one of its dates and not cancelled.
func (s *service) answeredOccurrence(ctx context.Context, appointment *appointmentcap.Appointment, date appointmentcap.Date) (appointmentcap.Date, error) {
	if date.IsZero() {
		return "", nil
	}
	recurrence, err := s.cfg.Appointments.FindRecurrenceRule(ctx, appointment.ID)
	if err != nil {
		return "", err
	}
	if recurrence == nil {
		if date == appointment.StartDate {
			return "", nil
		}
		return "", fmt.Errorf("%w: the appointment does not take place on %s", ErrOccurrenceUnavailable, date)
	}
	if !occurrenceExists(appointment, recurrence, date) {
		return "", fmt.Errorf("%w: the series has no date %s", ErrOccurrenceUnavailable, date)
	}
	overrides, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, []int64{appointment.ID}, []appointmentcap.Date{date})
	if err != nil {
		return "", err
	}
	for _, override := range overrides {
		if override.OccurrenceDate == date && override.Cancelled {
			return "", fmt.Errorf("%w: the date %s is cancelled", ErrOccurrenceUnavailable, date)
		}
	}
	return date, nil
}

// staffRecipient is the staff member's recipient row, nil when the staff
// member is not invited.
func staffRecipient(recipients []*appointmentcap.AppointmentRecipient, staffID int64) *appointmentcap.AppointmentRecipient {
	for _, recipient := range recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			return recipient
		}
	}
	return nil
}

// guardianRecipientForStudents is the first recipient row of the guardians
// invited through one of the students, nil when there is none.
func guardianRecipientForStudents(recipients []*appointmentcap.AppointmentRecipient, guardianProfileIDs []int64, studentIDs []int64) *appointmentcap.AppointmentRecipient {
	guardianRecipients := recipientsOfGuardians(recipients, int64Set(guardianProfileIDs))
	allowedStudents := int64Set(studentIDs)
	for _, recipient := range guardianRecipients {
		if recipientHasStudent(recipient, allowedStudents) {
			return recipient
		}
	}
	return nil
}

// recipientHasStudent reports whether the recipient was invited through one of
// the allowed students.
func recipientHasStudent(recipient *appointmentcap.AppointmentRecipient, allowedStudents map[int64]struct{}) bool {
	for _, studentID := range recipient.StudentIDs {
		if _, ok := allowedStudents[studentID]; ok {
			return true
		}
	}
	return false
}

// recipientsOfGuardians keeps, in order, the guardian recipients whose profile
// is one of the allowed guardians.
func recipientsOfGuardians(recipients []*appointmentcap.AppointmentRecipient, allowedGuardians map[int64]struct{}) []*appointmentcap.AppointmentRecipient {
	out := make([]*appointmentcap.AppointmentRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		if _, ok := allowedGuardians[*recipient.GuardianProfileID]; ok {
			out = append(out, recipient)
		}
	}
	return out
}

func (s *service) recipientHasVisibleStudent(ctx context.Context, recipientID int64, allowedStudentIDs map[int64]struct{}) (bool, error) {
	links, err := s.cfg.Appointments.FindAppointmentRecipientStudents(ctx, []int64{recipientID})
	if err != nil {
		return false, err
	}
	for _, link := range links {
		if _, ok := allowedStudentIDs[link.StudentID]; ok {
			return true, nil
		}
	}
	return false, nil
}
