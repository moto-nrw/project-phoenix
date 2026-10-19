package application

import (
	"context"
	"fmt"
	"sort"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

func (s *service) ListMyStaffEvents(ctx context.Context, from, to appointmentcap.Date) ([]Event, error) {
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}

	appointments, err := s.listAppointmentsVisibleToStaff(ctx, staff.ID, toCalendarDate(from), toCalendarDate(to))
	if err != nil {
		return nil, err
	}
	appointmentEvents, err := s.expandAppointmentEvents(ctx, appointments, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	timetableEvents, err := s.staffTimetableEvents(ctx, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	shiftEvents, err := s.staffShiftEvents(ctx, staff.ID, from, to)
	if err != nil {
		return nil, err
	}

	events := append(appointmentEvents, timetableEvents...)
	events = append(events, shiftEvents...)
	sortEvents(events)
	return events, nil
}

func (s *service) ListMyParentEvents(ctx context.Context, accountID int64, from, to appointmentcap.Date) ([]Event, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}

	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	events := []Event{}
	if err := s.withinParentTenants(ctx, children, func(txCtx context.Context, tenantID int64, tenantChildren []*ports.ChildSummary) (bool, error) {
		tenantEvents, err := s.parentTenantEvents(txCtx, tenantID, tenantChildren, from, to)
		if err != nil {
			return false, err
		}
		events = append(events, tenantEvents...)
		return false, nil
	}); err != nil {
		return nil, err
	}
	sortEvents(events)
	return events, nil
}

// parentTenantEvents lists the appointment events of one tenant that are
// addressed to the parent through the tenant's children, labelled with the
// tenant and school.
func (s *service) parentTenantEvents(ctx context.Context, tenantID int64, tenantChildren []*ports.ChildSummary, from, to appointmentcap.Date) ([]Event, error) {
	guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
	studentIDs := distinctChildStudentIDs(tenantChildren)
	appointments, err := s.listAppointmentsVisibleToGuardians(ctx, guardianProfileIDs, studentIDs, toCalendarDate(from), toCalendarDate(to))
	if err != nil {
		return nil, err
	}
	appointmentEvents, err := s.expandGuardianAppointmentEvents(ctx, appointments, guardianProfileIDs, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	schoolName := ""
	tenantIDString := formatID(tenantID)
	if len(tenantChildren) > 0 {
		schoolName = tenantChildren[0].SchoolName
	}
	for i := range appointmentEvents {
		appointmentEvents[i].TenantID = &tenantIDString
		if schoolName != "" {
			appointmentEvents[i].SchoolName = &schoolName
		}
	}
	return appointmentEvents, nil
}

func validateWindow(from, to appointmentcap.Date) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("%w: from and to are required", ErrInvalidRequest)
	}
	if to.Before(from) {
		return fmt.Errorf("%w: to must be on or after from", ErrInvalidRequest)
	}
	if from.DaysUntil(to)+1 > maxCalendarWindowDays {
		return fmt.Errorf("%w: range cannot exceed %d days", ErrInvalidRequest, maxCalendarWindowDays)
	}
	return nil
}

func (s *service) listAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]*appointmentcap.Appointment, error) {
	values, err := s.cfg.Appointments.ListAppointmentsVisibleToStaff(ctx, staffID, from, to)
	return values, err
}

func (s *service) listAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to appointmentcap.Date) ([]*appointmentcap.Appointment, error) {
	values, err := s.cfg.Appointments.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, from, to)
	return values, err
}

// recipientsByAppointment loads the recipients of every listed appointment in
// one read; the list views used to issue one read per appointment (#2940).
func (s *service) recipientsByAppointment(ctx context.Context, appointmentIDs []int64) (map[int64][]*appointmentcap.AppointmentRecipient, error) {
	recipients, err := s.cfg.Appointments.FindAppointmentRecipientsByAppointmentIDs(ctx, appointmentIDs)
	if err != nil {
		return nil, err
	}
	byAppointment := make(map[int64][]*appointmentcap.AppointmentRecipient, len(appointmentIDs))
	for _, recipient := range recipients {
		byAppointment[recipient.AppointmentID] = append(byAppointment[recipient.AppointmentID], recipient)
	}
	return byAppointment, nil
}

func (s *service) expandAppointmentEvents(ctx context.Context, appointments []*appointmentcap.Appointment, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	expansion, err := s.loadAppointmentExpansion(ctx, appointments, from, to)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		status, recipientID := staffRecipientStatus(expansion.recipientsByAppointment[appointment.ID], staffID)
		events = append(events, expansion.events(appointment, from, to, func(occurrence appointmentcap.Date) Event {
			return appointmentEvent(appointment, occurrence, status, recipientID, staffID)
		})...)
	}
	return events, nil
}

func (s *service) expandGuardianAppointmentEvents(ctx context.Context, appointments []*appointmentcap.Appointment, guardianProfileIDs []int64, studentIDs []int64, from, to appointmentcap.Date) ([]Event, error) {
	expansion, err := s.loadAppointmentExpansion(ctx, appointments, from, to)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		status, recipientID, err := s.guardianRecipientStatusForStudents(ctx, expansion.recipientsByAppointment[appointment.ID], guardianProfileIDs, studentIDs)
		if err != nil {
			return nil, err
		}
		events = append(events, expansion.events(appointment, from, to, func(occurrence appointmentcap.Date) Event {
			event := appointmentEvent(appointment, occurrence, status, recipientID, 0)
			event.CanViewOverview = canParentViewOverview(appointment, recipientID != nil)
			return event
		})...)
	}
	return events, nil
}

// appointmentExpansion holds the batch reads that turn a list of appointments
// into calendar events for one window: recurrence rules, the occurrence
// overrides inside the window, and the recipients.
type appointmentExpansion struct {
	recurrenceByAppointment   map[int64]*appointmentcap.RecurrenceRule
	overrideByAppointmentDate map[string]*appointmentcap.AppointmentOccurrenceOverride
	recipientsByAppointment   map[int64][]*appointmentcap.AppointmentRecipient
}

func (s *service) loadAppointmentExpansion(ctx context.Context, appointments []*appointmentcap.Appointment, from, to appointmentcap.Date) (appointmentExpansion, error) {
	ids := appointmentIDs(appointments)
	recurrenceByAppointment, err := s.recurrenceRulesByAppointment(ctx, ids)
	if err != nil {
		return appointmentExpansion{}, err
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, ids, toCalendarDates(occurrenceDates))
	if err != nil {
		return appointmentExpansion{}, err
	}
	overrideByAppointmentDate := make(map[string]*appointmentcap.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[occurrenceKey(override.AppointmentID, override.OccurrenceDate)] = override
	}
	recipientsByAppointment, err := s.recipientsByAppointment(ctx, ids)
	if err != nil {
		return appointmentExpansion{}, err
	}
	return appointmentExpansion{
		recurrenceByAppointment:   recurrenceByAppointment,
		overrideByAppointmentDate: overrideByAppointmentDate,
		recipientsByAppointment:   recipientsByAppointment,
	}, nil
}

// events expands one appointment inside the window: a single event when it
// does not recur and overlaps the window, otherwise one event per occurrence
// that is not cancelled, with its override applied. build creates the event of
// an occurrence date.
func (e appointmentExpansion) events(appointment *appointmentcap.Appointment, from, to appointmentcap.Date, build func(appointmentcap.Date) Event) []Event {
	recurrence := e.recurrenceByAppointment[appointment.ID]
	if recurrence == nil {
		if !dateRangesOverlap(toTimezoneDate(appointment.StartDate), toTimezoneDate(appointment.EndDate), from, to) {
			return nil
		}
		return []Event{build(toTimezoneDate(appointment.StartDate))}
	}
	var events []Event
	for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
		override := e.overrideByAppointmentDate[occurrenceKey(appointment.ID, occurrence)]
		if override != nil && override.Cancelled {
			continue
		}
		event := build(occurrence)
		event.Recurring = true
		applyOverride(&event, override)
		events = append(events, event)
	}
	return events
}

func occurrenceKey(appointmentID int64, occurrenceDate appointmentcap.Date) string {
	return fmt.Sprintf("%d:%s", appointmentID, occurrenceDate.String())
}

func appointmentIDs(appointments []*appointmentcap.Appointment) []int64 {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	return ids
}

// recurrenceRulesByAppointment loads the recurrence rules of the listed
// appointments in one read, keyed by appointment ID.
func (s *service) recurrenceRulesByAppointment(ctx context.Context, ids []int64) (map[int64]*appointmentcap.RecurrenceRule, error) {
	recurrences, err := s.cfg.Appointments.FindRecurrenceRules(ctx, ids)
	if err != nil {
		return nil, err
	}
	byAppointment := make(map[int64]*appointmentcap.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		byAppointment[recurrence.AppointmentID] = recurrence
	}
	return byAppointment, nil
}

func appointmentEvent(appointment *appointmentcap.Appointment, occurrenceDate appointmentcap.Date, responseStatus *string, recipientID *int64, staffID int64) Event {
	appointmentID := formatID(appointment.ID)
	deliveryMode := appointment.DeliveryMode
	occurrence := occurrenceDate.String()
	endDate := occurrenceDate.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate))
	isStaffRecipient := recipientID != nil
	var recipientIDString *string
	if recipientID != nil {
		value := formatID(*recipientID)
		recipientIDString = &value
	}
	organizerStaffID := formatID(appointment.OrganizerStaffID)
	return Event{
		ID:               fmt.Sprintf("appointment:%d:%s", appointment.ID, occurrence),
		Source:           EventSourceAppointment,
		AppointmentID:    &appointmentID,
		OccurrenceDate:   &occurrence,
		Title:            appointment.Title,
		Description:      appointment.Description,
		Location:         appointment.Location,
		StartDate:        occurrenceDate.String(),
		EndDate:          endDate.String(),
		StartTime:        formatClock(appointment.StartTime),
		EndTime:          formatClock(appointment.EndTime),
		AllDay:           appointment.AllDay,
		Cancelled:        appointment.CancelledAt != nil,
		DeliveryMode:     &deliveryMode,
		ResponseStatus:   responseStatus,
		RecipientID:      recipientIDString,
		OrganizerStaffID: &organizerStaffID,
		// Stay respondable for any real (non-informational) recipient, including
		// already accepted/declined ones — the respond endpoints allow changing
		// an existing RSVP, so users can correct an accidental answer. Only
		// informational recipients (and non-recipients) cannot respond, and a
		// cancelled appointment freezes RSVP entirely (matching the server-side
		// rejection in RespondTo*Invitation).
		CanRespond:      appointment.CancelledAt == nil && recipientID != nil && responseStatus != nil && *responseStatus != appointmentcap.ResponseStatusInfo,
		CanEdit:         appointment.OrganizerStaffID == staffID,
		CanViewOverview: canStaffViewOverview(appointment, staffID, isStaffRecipient),
	}
}

func applyOverride(event *Event, override *appointmentcap.AppointmentOccurrenceOverride) {
	if override == nil {
		return
	}
	if override.Title != nil {
		event.Title = *override.Title
	}
	if override.Description != nil {
		event.Description = override.Description
	}
	if override.Location != nil {
		event.Location = override.Location
	}
	if override.StartDate != nil {
		event.StartDate = override.StartDate.String()
	}
	if override.EndDate != nil {
		event.EndDate = override.EndDate.String()
	}
	if override.StartTime != nil {
		event.StartTime = formatClock(*override.StartTime)
	}
	if override.EndTime != nil {
		event.EndTime = formatClock(*override.EndTime)
	}
	if override.AllDay != nil {
		event.AllDay = *override.AllDay
	}
}

func occurrenceDatesForAppointments(appointments []*appointmentcap.Appointment, recurrenceByAppointment map[int64]*appointmentcap.RecurrenceRule, from, to appointmentcap.Date) []appointmentcap.Date {
	seen := map[string]struct{}{}
	dates := []appointmentcap.Date{}
	for _, appointment := range appointments {
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			continue
		}
		for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
			key := occurrence.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			dates = append(dates, occurrence)
		}
	}
	return dates
}

func dateRangesOverlap(aStart, aEnd, bStart, bEnd appointmentcap.Date) bool {
	return !aEnd.Before(bStart) && !aStart.After(bEnd)
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].StartDate != events[j].StartDate {
			return events[i].StartDate < events[j].StartDate
		}
		if events[i].StartTime != events[j].StartTime {
			return events[i].StartTime < events[j].StartTime
		}
		return events[i].ID < events[j].ID
	})
}
