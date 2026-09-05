package appointments

import (
	"context"
	"fmt"
	"time"
)

// An all-day occurrence is due at 08:00 Berlin time, not at midnight.
const allDayReminderHour = 8

type DueReminder struct {
	Appointment *Appointment
	Effective   *Appointment
	Occurrence  Date
}

type reminderPlanner struct{ query Query }

// FindDueGuardianReminders locks and rechecks eligible occurrences in [from,to).
// The caller must hold a tenant transaction for the complete delivery workflow.
func FindDueGuardianReminders(ctx context.Context, query Query, from, to time.Time) ([]DueReminder, error) {
	if !to.After(from) {
		return nil, nil
	}
	fromDate := Date(from.In(berlin).Format(dateLayout))
	toDate := Date(to.In(berlin).Format(dateLayout))
	values, err := query.ListGuardianReminderCandidates(ctx, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("calendar: list reminder candidates: %w", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return (&reminderPlanner{query: query}).loadDueAppointmentReminders(ctx, values, fromDate, toDate, from, to)
}

func (s *reminderPlanner) loadReminderCandidateRelations(
	ctx context.Context,
	appointments []*Appointment,
	from, to Date,
) (reminderCandidateRelations, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.query.FindRecurrenceRules(ctx, ids)
	if err != nil {
		return reminderCandidateRelations{}, fmt.Errorf("calendar: load reminder recurrences: %w", err)
	}
	recurrenceByAppointment := make(map[int64]*RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	movedOverrides, err := s.query.FindOccurrenceOverridesByStartDates(ctx, ids, dateRange(from, to))
	if err != nil {
		return reminderCandidateRelations{}, fmt.Errorf("calendar: load moved reminder overrides: %w", err)
	}
	movedByAppointment := make(map[int64][]*AppointmentOccurrenceOverride, len(movedOverrides))
	for _, override := range movedOverrides {
		movedByAppointment[override.AppointmentID] = append(movedByAppointment[override.AppointmentID], override)
	}
	return reminderCandidateRelations{recurrences: recurrenceByAppointment, moved: movedByAppointment}, nil
}

func eligibleReminderAppointments(
	appointments []*Appointment,
	relations reminderCandidateRelations,
	from, to Date,
) []*Appointment {
	eligible := appointments[:0]
	for _, appointment := range appointments {
		rule := relations.recurrences[appointment.ID]
		if rule != nil && rule.OccurrenceCount != nil && !HasOccurrenceInWindow(appointment, rule, from, to) && len(relations.moved[appointment.ID]) == 0 {
			continue
		}
		eligible = append(eligible, appointment)
	}
	return eligible
}

func (s *reminderPlanner) loadDueAppointmentReminders(
	ctx context.Context,
	appointments []*Appointment,
	fromDate, toDate Date,
	from, to time.Time,
) ([]DueReminder, error) {
	relations, err := s.loadReminderCandidateRelations(ctx, appointments, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	appointments = eligibleReminderAppointments(appointments, relations, fromDate, toDate)
	if len(appointments) == 0 {
		return nil, nil
	}
	occurrencesByAppointment := reminderOccurrencesByAppointment(appointments, relations.recurrences, relations.moved, fromDate, toDate)
	current, err := s.loadLockedReminderCandidates(ctx, appointments, occurrencesByAppointment)
	if err != nil {
		return nil, err
	}
	return collectDueAppointmentReminders(appointments, occurrencesByAppointment, current, from, to), nil
}

func collectDueAppointmentReminders(
	appointments []*Appointment,
	occurrences map[int64][]Date,
	current lockedReminderCandidates,
	from, to time.Time,
) []DueReminder {
	due := make([]DueReminder, 0)
	for _, appointment := range appointments {
		locked := current.appointments[appointment.ID]
		if locked == nil {
			continue
		}
		for _, occurrence := range occurrences[appointment.ID] {
			if reminder := dueAppointmentForOccurrence(locked, occurrence, current, from, to); reminder != nil {
				due = append(due, *reminder)
			}
		}
	}
	return due
}

func dueAppointmentForOccurrence(
	appointment *Appointment,
	occurrence Date,
	current lockedReminderCandidates,
	from, to time.Time,
) *DueReminder {
	rule := current.recurrences[appointment.ID]
	if (rule == nil && appointment.StartDate != Date(occurrence)) || (rule != nil && !OccurrenceExists(appointment, rule, occurrence)) {
		return nil
	}
	override := current.overrides[appointmentOccurrenceKey(appointment.ID, occurrence)]
	if override != nil && override.Cancelled {
		return nil
	}
	effective := EffectiveOccurrence(appointment, occurrence, override)
	startsAt := OccurrenceStartInstant(effective)
	if startsAt.Before(from) || !startsAt.Before(to) {
		return nil
	}
	return &DueReminder{Appointment: appointment, Effective: effective, Occurrence: occurrence}
}

func (s *reminderPlanner) loadLockedReminderCandidates(
	ctx context.Context,
	appointments []*Appointment,
	occurrences map[int64][]Date,
) (lockedReminderCandidates, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	locked, err := s.query.FindReminderCandidatesForUpdate(ctx, ids)
	if err != nil {
		return lockedReminderCandidates{}, fmt.Errorf("calendar: lock reminder candidates: %w", err)
	}
	lockedIDs := make([]int64, 0, len(locked))
	byID := make(map[int64]*Appointment, len(locked))
	for _, appointment := range locked {
		lockedIDs = append(lockedIDs, appointment.ID)
		byID[appointment.ID] = appointment
	}
	rules, err := s.query.FindRecurrenceRules(ctx, lockedIDs)
	if err != nil {
		return lockedReminderCandidates{}, fmt.Errorf("calendar: reload reminder recurrences: %w", err)
	}
	rulesByID := make(map[int64]*RecurrenceRule, len(rules))
	for _, rule := range rules {
		rulesByID[rule.AppointmentID] = rule
	}
	dates := distinctReminderOccurrenceDates(lockedIDs, occurrences)
	overrides, err := s.query.FindOccurrenceOverrides(ctx, lockedIDs, dates)
	if err != nil {
		return lockedReminderCandidates{}, fmt.Errorf("calendar: reload reminder overrides: %w", err)
	}
	overridesByKey := make(map[string]*AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overridesByKey[appointmentOccurrenceKey(override.AppointmentID, Date(override.OccurrenceDate))] = override
	}
	return lockedReminderCandidates{appointments: byID, recurrences: rulesByID, overrides: overridesByKey}, nil
}

func reminderOccurrencesByAppointment(
	appointments []*Appointment,
	rules map[int64]*RecurrenceRule,
	moved map[int64][]*AppointmentOccurrenceOverride,
	from, to Date,
) map[int64][]Date {
	result := make(map[int64][]Date, len(appointments))
	for _, appointment := range appointments {
		seen := make(map[Date]bool)
		for _, occurrence := range reminderOccurrences(appointment, rules[appointment.ID], from, to) {
			if !seen[occurrence] {
				seen[occurrence] = true
				result[appointment.ID] = append(result[appointment.ID], occurrence)
			}
		}
		for _, override := range moved[appointment.ID] {
			occurrence := Date(override.OccurrenceDate)
			if !seen[occurrence] {
				seen[occurrence] = true
				result[appointment.ID] = append(result[appointment.ID], occurrence)
			}
		}
	}
	return result
}

func distinctReminderOccurrenceDates(ids []int64, occurrences map[int64][]Date) []Date {
	seen := make(map[Date]bool)
	dates := make([]Date, 0)
	for _, id := range ids {
		for _, date := range occurrences[id] {
			if !seen[date] {
				seen[date] = true
				dates = append(dates, date)
			}
		}
	}
	return dates
}

func appointmentOccurrenceKey(appointmentID int64, occurrence Date) string {
	return fmt.Sprintf("%d:%s", appointmentID, occurrence.String())
}

func dateRange(from, to Date) []Date {
	dates := make([]Date, 0, from.DaysUntil(to)+1)
	for date := from; !date.After(to); date = date.AddDays(1) {
		dates = append(dates, date)
	}
	return dates
}

type reminderCandidateRelations struct {
	recurrences map[int64]*RecurrenceRule
	moved       map[int64][]*AppointmentOccurrenceOverride
}

type lockedReminderCandidates struct {
	appointments map[int64]*Appointment
	recurrences  map[int64]*RecurrenceRule
	overrides    map[string]*AppointmentOccurrenceOverride
}

// reminderOccurrences lists the occurrence dates of one appointment inside the
// window — the single date of a one-off, or the expanded recurrence.
func reminderOccurrences(appointment *Appointment, rule *RecurrenceRule, from, to Date) []Date {
	if rule == nil {
		if !dateRangesOverlap(Date(appointment.StartDate), Date(appointment.EndDate), from, to) {
			return nil
		}
		return []Date{Date(appointment.StartDate)}
	}
	return ExpandOccurrences(appointment, rule, from, to)
}

// EffectiveOccurrence returns a copy of the appointment as this occurrence
// actually happens: moved to the occurrence date and carrying whatever the
// single-occurrence override changed. The copy is what the mail text and the
// due-instant are computed from, so an occurrence someone moved to another time
// is reminded about at its real time and reads correctly in the mail.
func EffectiveOccurrence(appointment *Appointment, occurrence Date, override *AppointmentOccurrenceOverride) *Appointment {
	effective := *appointment
	span := appointment.StartDate.DaysUntil(appointment.EndDate)
	effective.StartDate = Date(occurrence)
	effective.EndDate = Date(occurrence.AddDays(span))

	if override == nil {
		return &effective
	}
	if override.Title != nil {
		effective.Title = *override.Title
	}
	if override.Location != nil {
		effective.Location = override.Location
	}
	if override.AllDay != nil {
		effective.AllDay = *override.AllDay
	}
	if override.StartTime != nil {
		effective.StartTime = *override.StartTime
	}
	if override.EndTime != nil {
		effective.EndTime = *override.EndTime
	}
	if override.StartDate != nil {
		effective.StartDate = *override.StartDate
	}
	if override.EndDate != nil {
		effective.EndDate = *override.EndDate
	}
	return &effective
}

// OccurrenceStartInstant turns an occurrence into the actual moment it begins,
// in Berlin wall-clock terms. TIME columns carry no date and no zone, so the
// clock components are lifted onto the occurrence's local midnight — which is
// what makes the result DST-correct instead of an hour off twice a year.
func OccurrenceStartInstant(appointment *Appointment) time.Time {
	midnight := appointment.StartDate.BerlinMidnight()
	if appointment.AllDay {
		return time.Date(
			midnight.Year(), midnight.Month(), midnight.Day(),
			allDayReminderHour, 0, 0, 0,
			midnight.Location(),
		)
	}
	clock := appointment.StartTime
	return time.Date(
		midnight.Year(), midnight.Month(), midnight.Day(),
		clock.Hour(), clock.Minute(), 0, 0,
		midnight.Location(),
	)
}
