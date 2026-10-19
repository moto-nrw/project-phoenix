package application

import (
	"context"
	"fmt"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

func (s *service) parentChildren(ctx context.Context, accountID int64) ([]*ports.ChildSummary, error) {
	var children []*ports.ChildSummary
	if err := s.cfg.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		rows, err := s.cfg.ChildRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		children = rows
		return nil
	}); err != nil {
		return nil, err
	}
	return children, nil
}

func groupChildrenByTenant(children []*ports.ChildSummary) map[int64][]*ports.ChildSummary {
	out := make(map[int64][]*ports.ChildSummary)
	for _, child := range children {
		out[child.TenantID] = append(out[child.TenantID], child)
	}
	return out
}

func distinctGuardianProfileIDs(children []*ports.ChildSummary) []int64 {
	seen := map[int64]struct{}{}
	out := []int64{}
	for _, child := range children {
		if _, ok := seen[child.GuardianProfileID]; ok {
			continue
		}
		seen[child.GuardianProfileID] = struct{}{}
		out = append(out, child.GuardianProfileID)
	}
	return out
}

func distinctChildStudentIDs(children []*ports.ChildSummary) []int64 {
	seen := map[int64]struct{}{}
	out := []int64{}
	for _, child := range children {
		if _, ok := seen[child.StudentID]; ok {
			continue
		}
		seen[child.StudentID] = struct{}{}
		out = append(out, child.StudentID)
	}
	return out
}

// validateParentAppointmentRequest checks the identifiers of a parent request
// for one appointment: the account first (forbidden), then the appointment
// (invalid request).
func validateParentAppointmentRequest(accountID, appointmentID int64) error {
	if accountID <= 0 {
		return fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if appointmentID <= 0 {
		return fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	return nil
}

// withinParentTenants runs visit once per tenant of the parent's children,
// each inside that tenant's transaction, until visit reports done or fails.
func (s *service) withinParentTenants(ctx context.Context, children []*ports.ChildSummary, visit func(ctx context.Context, tenantID int64, tenantChildren []*ports.ChildSummary) (bool, error)) error {
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		var done bool
		if err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			var err error
			done, err = visit(txCtx, tenantID, tenantChildren)
			return err
		}); err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	return nil
}

// parentVisibleAppointment loads an appointment inside one tenant for a
// parent. It returns a nil appointment when the appointment is missing,
// soft-deleted, or not addressed to the parent through one of the tenant's
// children; otherwise it also returns the appointment's recipients.
func (s *service) parentVisibleAppointment(ctx context.Context, appointmentID int64, tenantChildren []*ports.ChildSummary) (*appointmentcap.Appointment, []*appointmentcap.AppointmentRecipient, error) {
	appointment, err := s.findAppointment(ctx, appointmentID)
	if err != nil {
		return nil, nil, err
	}
	if appointment == nil || appointment.DeletedAt != nil {
		return nil, nil, nil
	}
	recipients, err := s.cfg.Appointments.FindAppointmentRecipients(ctx, appointment.ID)
	if err != nil {
		return nil, nil, err
	}
	guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
	studentIDs := distinctChildStudentIDs(tenantChildren)
	_, recipientID, err := s.guardianRecipientStatusForStudents(ctx, recipients, guardianProfileIDs, studentIDs)
	if err != nil {
		return nil, nil, err
	}
	if recipientID == nil {
		return nil, nil, nil
	}
	return appointment, recipients, nil
}
