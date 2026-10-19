package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

func (s *service) GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error) {
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.findAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	// A soft-deleted appointment is gone from every interactive surface, so its
	// overview is not reachable even with a retained appointment ID.
	if appointment == nil || appointment.DeletedAt != nil {
		return nil, ErrNotFound
	}
	recipients, err := s.cfg.Appointments.FindAppointmentRecipients(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	_, recipientID := staffRecipientStatus(recipients, staff.ID)
	if appointment.OrganizerStaffID != staff.ID && recipientID == nil {
		return nil, ErrNotFound
	}
	if !canStaffViewOverview(appointment, staff.ID, recipientID != nil) {
		return nil, ErrForbidden
	}
	return s.buildAppointmentOverview(ctx, appointment, recipients)
}

func (s *service) buildAppointmentOverview(ctx context.Context, appointment *appointmentcap.Appointment, recipients []*appointmentcap.AppointmentRecipient) (*AppointmentOverview, error) {
	staffIDs, guardianIDs := recipientPersonIDs(recipients)
	staffByID, err := s.cfg.StaffRepo.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	guardiansByID, err := s.cfg.GuardianProfileRepo.FindByIDs(ctx, guardianIDs)
	if err != nil {
		return nil, err
	}

	attendees := make([]AppointmentAttendee, 0, len(recipients))
	for _, recipient := range recipients {
		attendees = append(attendees, AppointmentAttendee{
			RecipientID:   formatID(recipient.ID),
			RecipientType: recipient.RecipientType,
			Name:          attendeeName(recipient, staffByID, guardiansByID),
			Status:        recipient.Status,
			RespondedAt:   recipient.RespondedAt,
		})
	}
	sort.SliceStable(attendees, func(i, j int) bool {
		if attendees[i].RecipientType != attendees[j].RecipientType {
			return attendees[i].RecipientType < attendees[j].RecipientType
		}
		return strings.ToLower(attendees[i].Name) < strings.ToLower(attendees[j].Name)
	})

	return &AppointmentOverview{
		AppointmentID:      formatID(appointment.ID),
		DeliveryMode:       appointment.DeliveryMode,
		OverviewVisibility: appointment.OverviewVisibility,
		Attendees:          attendees,
	}, nil
}

// recipientPersonIDs splits the recipients into the staff and guardian
// profile IDs whose names the overview resolves.
func recipientPersonIDs(recipients []*appointmentcap.AppointmentRecipient) ([]int64, []int64) {
	staffIDs := make([]int64, 0)
	guardianIDs := make([]int64, 0)
	for _, recipient := range recipients {
		if recipient.StaffID != nil {
			staffIDs = append(staffIDs, *recipient.StaffID)
		}
		if recipient.GuardianProfileID != nil {
			guardianIDs = append(guardianIDs, *recipient.GuardianProfileID)
		}
	}
	return staffIDs, guardianIDs
}

// attendeeName names a recipient by its staff member or guardian, falling
// back to a numbered placeholder when no name is known.
func attendeeName(recipient *appointmentcap.AppointmentRecipient, staffByID map[int64]*ports.Staff, guardiansByID map[int64]*ports.GuardianProfile) string {
	name := ""
	if recipient.StaffID != nil {
		name = staffName(staffByID[*recipient.StaffID])
	}
	if recipient.GuardianProfileID != nil {
		name = guardianName(guardiansByID[*recipient.GuardianProfileID])
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Empfänger %d", recipient.ID)
	}
	return name
}

func canStaffViewOverview(appointment *appointmentcap.Appointment, staffID int64, isRecipient bool) bool {
	if appointment == nil {
		return false
	}
	if appointment.OrganizerStaffID == staffID {
		return true
	}
	switch appointment.OverviewVisibility {
	case appointmentcap.OverviewVisibilityStaff, appointmentcap.OverviewVisibilityAll:
		return isRecipient
	default:
		return false
	}
}

func canParentViewOverview(appointment *appointmentcap.Appointment, isRecipient bool) bool {
	return appointment != nil && isRecipient && appointment.OverviewVisibility == appointmentcap.OverviewVisibilityAll
}
