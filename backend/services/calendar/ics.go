package calendar

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/ical"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StaffAppointmentICS renders an appointment the current staff member can see
// (organizer or invited) as an iCalendar document for import into an external
// calendar. Returns a download filename and the text/calendar body.
func (s *service) StaffAppointmentICS(ctx context.Context, appointmentID int64) (string, string, error) {
	if appointmentID <= 0 {
		return "", "", fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return "", "", fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.cfg.AppointmentRepo.FindByID(ctx, appointmentID)
	if err != nil {
		return "", "", err
	}
	// A soft-deleted appointment is gone from every interactive surface; only the
	// subscription feed still exports it (as a tombstone).
	if appointment == nil || appointment.DeletedAt != nil {
		return "", "", ErrNotFound
	}
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return "", "", err
	}
	_, recipientID := staffRecipientStatus(recipients, staff.ID)
	if appointment.OrganizerStaffID != staff.ID && recipientID == nil {
		return "", "", ErrNotFound
	}
	return s.renderAppointmentICS(ctx, appointment)
}

// ParentAppointmentICS renders an appointment a guardian can see (their linked
// child is a recipient) as an iCalendar document. Scans the parent's tenants
// exactly like the parent overview path.
func (s *service) ParentAppointmentICS(ctx context.Context, accountID, appointmentID int64) (string, string, error) {
	if accountID <= 0 {
		return "", "", fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if appointmentID <= 0 {
		return "", "", fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	var filename, content string
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		tenantID := tenantID
		tenantChildren := tenantChildren
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			appointment, err := s.cfg.AppointmentRepo.FindByID(txCtx, appointmentID)
			if err != nil {
				return err
			}
			// A soft-deleted appointment is downloadable only as a feed tombstone.
			if appointment == nil || appointment.DeletedAt != nil {
				return nil
			}
			recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(txCtx, appointment.ID)
			if err != nil {
				return err
			}
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			studentIDs := distinctChildStudentIDs(tenantChildren)
			_, recipientID, err := s.guardianRecipientStatusForStudents(txCtx, recipients, guardianProfileIDs, studentIDs)
			if err != nil {
				return err
			}
			if recipientID == nil {
				return nil
			}
			filename, content, err = s.renderAppointmentICS(txCtx, appointment)
			return err
		}); err != nil {
			return "", "", err
		}
		if content != "" {
			return filename, content, nil
		}
	}
	return "", "", ErrNotFound
}

func (s *service) renderAppointmentICS(ctx context.Context, appointment *calModels.Appointment) (string, string, error) {
	recurrence, err := s.cfg.RecurrenceRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return "", "", err
	}
	var cancelled []*calModels.AppointmentOccurrenceOverride
	if recurrence != nil {
		cancelled, err = s.cfg.OverrideRepo.FindCancelledByAppointmentIDs(ctx, []int64{appointment.ID})
		if err != nil {
			return "", "", err
		}
	}
	event := appointmentICSEvent(appointment, recurrence, cancelled)
	content := ical.Render(appointment.Title, []ical.Event{event})
	return icsFilename(appointment.Title), content, nil
}

func appointmentICSEvent(appointment *calModels.Appointment, recurrence *calModels.RecurrenceRule, cancelledOverrides []*calModels.AppointmentOccurrenceOverride) ical.Event {
	description := ""
	if appointment.Description != nil {
		description = *appointment.Description
	}
	location := ""
	if appointment.Location != nil {
		location = *appointment.Location
	}
	event := ical.Event{
		// Include the tenant so the UID is globally unique: a parent's feed
		// aggregates appointments across schools, and appointment IDs repeat per
		// tenant — a tenant-local UID would let one school's event overwrite
		// another's in the subscriber's calendar.
		UID:         fmt.Sprintf("appointment-%d-%d@moto-app.de", appointment.TenantID, appointment.ID),
		Summary:     appointment.Title,
		Description: description,
		Location:    location,
		StartDate:   appointment.StartDate,
		EndDate:     appointment.EndDate,
		StartClock:  appointment.StartTime,
		EndClock:    appointment.EndTime,
		AllDay:      appointment.AllDay,
		// A deleted appointment survives only as a feed tombstone; export it (like a
		// cancelled one) as STATUS:CANCELLED so subscribers purge it.
		Cancelled: appointment.CancelledAt != nil || appointment.DeletedAt != nil,
		Sequence:  appointment.Revision,
		Stamp:     appointment.UpdatedAt,
		// LAST-MODIFIED + SEQUENCE together tell subscribers this VEVENT is a newer
		// revision, so edits and cancellations are honoured rather than ignored.
		LastModified: appointment.UpdatedAt,
	}
	// Only emit an RRULE when the rule actually produces an occurrence. A rule
	// that generates none (e.g. weekly weekdays outside its EndsOn window) is
	// rejected at create/update, so this guard is defensive: it prevents a
	// phantom recurring series from ever being exported.
	first, ok := firstRecurrenceOccurrence(appointment, recurrence)
	if !ok {
		return event
	}
	// DTSTART must be the first real occurrence, not the raw StartDate: for a
	// weekly rule whose weekdays exclude StartDate's weekday the app starts on the
	// first matching weekday, so anchor the exported series there too (and shift
	// EndDate by the same span to preserve each occurrence's duration).
	//
	// The RRULE is exported UNCLAMPED — with the series' real UNTIL/COUNT (or
	// open-ended). A subscription feed is stateless and re-rendered on every poll,
	// so clamping the horizon to a moving "today + N days" UNTIL would advance the
	// window without bumping SEQUENCE/LAST-MODIFIED (the appointment itself is
	// unchanged). Clients treat the same UID+SEQUENCE as unchanged and keep the
	// FIRST horizon they saw, silently dropping later occurrences. Emitting the
	// true recurrence keeps the event stable across polls; the window only governs
	// which appointments are INCLUDED (applyAppointmentWindow + hasOccurrenceInWindow).
	if first != appointment.StartDate {
		event.StartDate = first
		event.EndDate = first.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate))
	}
	event.Recurrence = &ical.Recurrence{
		Freq:      recurrence.Frequency,
		Interval:  recurrence.IntervalCount,
		Weekdays:  recurrence.Weekdays,
		MonthDays: recurrence.MonthDays,
		Until:     recurrence.EndsOn,
		Count:     recurrence.OccurrenceCount,
	}
	// Single occurrences cancelled via "Nur diesen Termin" become EXDATEs so
	// subscribed external calendars drop them from the RRULE expansion.
	for _, override := range cancelledOverrides {
		event.ExDates = append(event.ExDates, override.OccurrenceDate)
	}
	return event
}

// icsFilename builds a safe download filename from the appointment title.
func icsFilename(title string) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_':
			return '-'
		default:
			return -1
		}
	}, title)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "termin"
	}
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return slug + ".ics"
}
