package application

import (
	"context"
	"fmt"

	appointmentcap "github.com/moto-ogs/project-phoenix/modules/appointments"
)

func (s *service) RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	if status != appointmentcap.ResponseStatusAccepted && status != appointmentcap.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
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
	return s.cfg.Appointments.UpdateAppointmentRecipientResponse(ctx, recipientID, status)
}

func staffRecipientStatus(recipients []*appointmentcap.AppointmentRecipient, staffID int64) (*string, *int64) {
	for _, recipient := range recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			status := recipient.Status
			id := recipient.ID
			return &status, &id
		}
	}
	return nil, nil
}

func guardianRecipientStatusForStudents(recipients []*appointmentcap.AppointmentRecipient, guardianProfileIDs []int64, studentIDs []int64) (*string, *int64) {
	guardianRecipients := recipientsOfGuardians(recipients, int64Set(guardianProfileIDs))
	allowedStudents := int64Set(studentIDs)
	for _, recipient := range guardianRecipients {
		if recipientHasStudent(recipient, allowedStudents) {
			status := recipient.Status
			id := recipient.ID
			return &status, &id
		}
	}
	return nil, nil
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
