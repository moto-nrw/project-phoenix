package application

import (
	"context"
	"fmt"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
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

func (s *service) RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error {
	if accountID <= 0 {
		return fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if status != appointmentcap.ResponseStatusAccepted && status != appointmentcap.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}

	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return err
	}
	// recipientID is globally unique (calendar.appointment_recipients.id is a
	// BIGSERIAL PRIMARY KEY, not per-tenant), so it exists in exactly one
	// tenant. Scanning the parent's tenants is safe: RLS inside each
	// WithTenantTx returns the row only in its owning tenant, and allowedProfiles
	// then confirms it belongs to THIS parent there. There is no cross-tenant ID
	// collision — a foreign recipient id simply resolves to nil or fails the
	// allowedProfiles check.
	var updated bool
	err = s.withinParentTenants(ctx, children, func(txCtx context.Context, _ int64, tenantChildren []*ports.ChildSummary) (bool, error) {
		var err error
		updated, err = s.respondAsGuardianInTenant(txCtx, recipientID, status, tenantChildren)
		return updated, err
	})
	if err != nil {
		return err
	}
	if updated {
		return nil
	}
	return ErrNotFound
}

// respondAsGuardianInTenant records the RSVP when the recipient belongs to one
// of the tenant's guardian profiles through a visible child. It reports false
// (and no error) when the recipient is not this parent's in the tenant.
func (s *service) respondAsGuardianInTenant(ctx context.Context, recipientID int64, status string, tenantChildren []*ports.ChildSummary) (bool, error) {
	allowedProfiles := int64Set(distinctGuardianProfileIDs(tenantChildren))
	allowedStudentIDs := int64Set(distinctChildStudentIDs(tenantChildren))
	recipient, err := s.cfg.Appointments.FindAppointmentRecipient(ctx, recipientID)
	if err != nil {
		return false, err
	}
	if recipient == nil || recipient.GuardianProfileID == nil {
		return false, nil
	}
	if _, ok := allowedProfiles[*recipient.GuardianProfileID]; !ok {
		return false, nil
	}
	visible, err := s.recipientHasVisibleStudent(ctx, recipient.ID, allowedStudentIDs)
	if err != nil || !visible {
		return false, err
	}
	if recipient.Status == appointmentcap.ResponseStatusInfo {
		return false, fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
	}
	appointment, err := s.findAppointment(ctx, recipient.AppointmentID)
	if err != nil {
		return false, err
	}
	// A soft-deleted appointment is unanswerable — treat as not found.
	if appointment == nil || appointment.DeletedAt != nil {
		return false, nil
	}
	if appointment.CancelledAt != nil {
		return false, fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
	}
	if err := s.cfg.Appointments.UpdateAppointmentRecipientResponse(ctx, recipientID, status); err != nil {
		return false, err
	}
	return true, nil
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

func (s *service) guardianRecipientStatusForStudents(ctx context.Context, recipients []*appointmentcap.AppointmentRecipient, guardianProfileIDs []int64, studentIDs []int64) (*string, *int64, error) {
	allowedStudents := int64Set(studentIDs)
	guardianRecipients := recipientsOfGuardians(recipients, int64Set(guardianProfileIDs))
	recipientIDs := make([]int64, 0, len(recipients))
	for _, recipient := range guardianRecipients {
		recipientIDs = append(recipientIDs, recipient.ID)
	}
	links, err := s.cfg.Appointments.FindAppointmentRecipientStudents(ctx, recipientIDs)
	if err != nil {
		return nil, nil, err
	}
	visibleRecipients := map[int64]struct{}{}
	for _, link := range links {
		if _, ok := allowedStudents[link.StudentID]; ok {
			visibleRecipients[link.RecipientID] = struct{}{}
		}
	}
	for _, recipient := range guardianRecipients {
		if _, ok := visibleRecipients[recipient.ID]; ok {
			status := recipient.Status
			id := recipient.ID
			return &status, &id, nil
		}
	}
	return nil, nil, nil
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
