package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appointmentcap "github.com/moto-ogs/project-phoenix/modules/appointments"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/collation"
)

func (s *service) GetStaffAppointmentOverview(ctx context.Context, appointmentID int64, occurrenceDate appointmentcap.Date) (*AppointmentOverview, error) {
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
	isRecipient := staffRecipient(recipients, staff.ID) != nil
	if appointment.OrganizerStaffID != staff.ID && !isRecipient {
		return nil, ErrNotFound
	}
	if !canStaffViewOverview(appointment, staff.ID, isRecipient) {
		return nil, ErrForbidden
	}
	occurrenceDate, err = s.answeredOccurrence(ctx, appointment, occurrenceDate)
	if err != nil {
		return nil, err
	}
	return s.buildAppointmentOverview(ctx, appointment, occurrenceDate, recipients, recipients)
}

// buildAppointmentOverview counts the answers of all recipients and names only
// the listed ones, so names of unlisted recipients are never loaded. With an
// occurrence date it shows the answers for that date of a series (#6987),
// without one the answers for the whole series. The time of an answer is
// kept for the whole series only: a row stores one time for all its answers.
func (s *service) buildAppointmentOverview(ctx context.Context, appointment *appointmentcap.Appointment, occurrenceDate appointmentcap.Date, recipients, listed []*appointmentcap.AppointmentRecipient) (*AppointmentOverview, error) {
	staffIDs, guardianIDs := recipientPersonIDs(listed)
	staffByID, err := s.cfg.StaffRepo.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	guardiansByID, err := s.cfg.GuardianProfileRepo.FindByIDs(ctx, guardianIDs)
	if err != nil {
		return nil, err
	}

	attendees := make([]AppointmentAttendee, 0, len(listed))
	for _, recipient := range listed {
		attendee := AppointmentAttendee{
			RecipientID:   formatID(recipient.ID),
			RecipientType: recipient.RecipientType,
			Name:          attendeeName(recipient, staffByID, guardiansByID),
			Status:        recipient.StatusOn(occurrenceDate),
		}
		if occurrenceDate.IsZero() {
			attendee.RespondedAt = recipient.RespondedAt
		}
		attendees = append(attendees, attendee)
	}
	sort.SliceStable(attendees, func(i, j int) bool {
		if attendees[i].RecipientType != attendees[j].RecipientType {
			return attendees[i].RecipientType < attendees[j].RecipientType
		}
		return collation.CompareGerman(attendees[i].Name, attendees[j].Name) < 0
	})

	return &AppointmentOverview{
		AppointmentID:      formatID(appointment.ID),
		DeliveryMode:       appointment.DeliveryMode,
		OverviewVisibility: appointment.OverviewVisibility,
		OccurrenceDate:     occurrenceDate,
		Counts:             countResponses(recipients, occurrenceDate),
		Attendees:          attendees,
	}, nil
}

func countResponses(recipients []*appointmentcap.AppointmentRecipient, occurrenceDate appointmentcap.Date) AppointmentResponseCounts {
	var counts AppointmentResponseCounts
	for _, recipient := range recipients {
		switch recipient.StatusOn(occurrenceDate) {
		case appointmentcap.ResponseStatusAccepted:
			counts.Accepted++
		case appointmentcap.ResponseStatusDeclined:
			counts.Declined++
		case appointmentcap.ResponseStatusPending:
			counts.Pending++
		}
	}
	return counts
}

// parentListedRecipients keeps the recipients a parent may see by name: every
// staff member and the parent's own guardian entries for their visible
// children. Other parents stay in the counts only (#6846).
func parentListedRecipients(recipients []*appointmentcap.AppointmentRecipient, guardianProfileIDs, studentIDs []int64) []*appointmentcap.AppointmentRecipient {
	visibleStudents := int64Set(studentIDs)
	listed := make([]*appointmentcap.AppointmentRecipient, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.StaffID != nil {
			listed = append(listed, recipient)
		}
	}
	for _, recipient := range recipientsOfGuardians(recipients, int64Set(guardianProfileIDs)) {
		if recipientHasStudent(recipient, visibleStudents) {
			listed = append(listed, recipient)
		}
	}
	return listed
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
