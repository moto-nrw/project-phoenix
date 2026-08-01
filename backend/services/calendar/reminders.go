package calendar

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// allDayReminderHour anchors the reminder for an all-day appointment. An all-day
// row carries no meaningful start_time, and taking midnight would put a 24-hour
// reminder in the middle of the night before. 08:00 local is when the school day
// starts, so "tomorrow" reaches parents over breakfast.
const allDayReminderHour = 8

// EnqueueDueAppointmentReminders queues the guardian reminder e-mail for every
// appointment occurrence starting in [from, to) and dispatches the matching
// push. It is called per tenant by the scheduler, which derives the window from
// the school's configured lead time, and returns how many e-mails it queued.
//
// The window is half-open on purpose: consecutive ticks pass adjacent windows,
// so an occurrence lands in exactly one of them. Duplicate delivery is
// nevertheless impossible — every row carries an idempotency key of
// (appointment, occurrence, guardian) and the outbox insert is ON CONFLICT DO
// NOTHING, so a re-run, an overlapping window, or a second scheduler process
// cannot produce a second mail for the same occurrence.
func (s *service) EnqueueDueAppointmentReminders(ctx context.Context, from, to time.Time) (int, error) {
	if s.cfg.Outbox == nil || !to.After(from) {
		return 0, nil
	}

	// The DB window is date-granular and the instant window is not: an
	// occurrence at 08:00 on day X is only due when the [from, to) instants
	// straddle it. Scan the full days the window touches and filter by instant
	// below.
	fromDate := timezone.DateFromTime(from)
	toDate := timezone.DateFromTime(to)

	appointments, err := s.cfg.AppointmentRepo.ListGuardianReminderCandidates(ctx, fromDate, toDate)
	if err != nil {
		return 0, fmt.Errorf("calendar: list reminder candidates: %w", err)
	}
	if len(appointments) == 0 {
		return 0, nil
	}

	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("calendar: load reminder recurrences: %w", err)
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}

	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, fromDate, toDate)
	overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(ctx, ids, occurrenceDates)
	if err != nil {
		return 0, fmt.Errorf("calendar: load reminder overrides: %w", err)
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	queued := 0
	for _, appointment := range appointments {
		for _, occurrence := range reminderOccurrences(appointment, recurrenceByAppointment[appointment.ID], fromDate, toDate) {
			override := overrideByAppointmentDate[fmt.Sprintf("%d:%s", appointment.ID, occurrence.String())]
			if override != nil && override.Cancelled {
				continue
			}
			effective := appointmentWithOverride(appointment, occurrence, override)
			startsAt := occurrenceStartInstant(effective)
			if startsAt.Before(from) || !startsAt.Before(to) {
				continue
			}
			count, err := s.enqueueAppointmentReminder(ctx, appointment, effective, occurrence)
			if err != nil {
				return queued, err
			}
			queued += count
			s.notifyGuardianDevices(ctx, appointment, platformModels.EmailKindAppointmentReminder)
		}
	}
	return queued, nil
}

// reminderOccurrences lists the occurrence dates of one appointment inside the
// window — the single date of a one-off, or the expanded recurrence.
func reminderOccurrences(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) []timezone.Date {
	if rule == nil {
		if !dateRangesOverlap(appointment.StartDate, appointment.EndDate, from, to) {
			return nil
		}
		return []timezone.Date{appointment.StartDate}
	}
	return expandOccurrences(appointment, rule, from, to)
}

// appointmentWithOverride returns a copy of the appointment as this occurrence
// actually happens: moved to the occurrence date and carrying whatever the
// single-occurrence override changed. The copy is what the mail text and the
// due-instant are computed from, so an occurrence someone moved to another time
// is reminded about at its real time and reads correctly in the mail.
func appointmentWithOverride(appointment *calModels.Appointment, occurrence timezone.Date, override *calModels.AppointmentOccurrenceOverride) *calModels.Appointment {
	effective := *appointment
	span := appointment.StartDate.DaysUntil(appointment.EndDate)
	effective.StartDate = occurrence
	effective.EndDate = occurrence.AddDays(span)

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

// occurrenceStartInstant turns an occurrence into the actual moment it begins,
// in Berlin wall-clock terms. TIME columns carry no date and no zone, so the
// clock components are lifted onto the occurrence's local midnight — which is
// what makes the result DST-correct instead of an hour off twice a year.
func occurrenceStartInstant(appointment *calModels.Appointment) time.Time {
	midnight := appointment.StartDate.BerlinMidnight()
	if appointment.AllDay {
		return midnight.Add(allDayReminderHour * time.Hour)
	}
	clock := timezone.WallClock(appointment.StartTime)
	return time.Date(
		midnight.Year(), midnight.Month(), midnight.Day(),
		clock.Hour(), clock.Minute(), 0, 0,
		midnight.Location(),
	)
}

// enqueueAppointmentReminder queues one reminder mail per reachable guardian of
// the appointment and reports how many rows it wrote.
func (s *service) enqueueAppointmentReminder(
	ctx context.Context,
	appointment *calModels.Appointment,
	effective *calModels.Appointment,
	occurrence timezone.Date,
) (int, error) {
	guardianIDs, profiles, err := s.reachableGuardianRecipients(ctx, appointment.ID)
	if err != nil {
		return 0, fmt.Errorf("calendar: resolve reminder recipients: %w", err)
	}
	if len(guardianIDs) == 0 {
		return 0, nil
	}

	schoolName := s.resolveSchoolName(ctx, appointment.TenantID)
	logoURL := s.resolveSchoolLogo(ctx, appointment.TenantID)
	motoLogoURL := emailbranding.MotoLogoURL(s.cfg.ParentsURL)
	whenText := appointmentWhenText(effective)
	location := ""
	if effective.Location != nil {
		location = *effective.Location
	}

	queued := 0
	seen := make(map[int64]struct{}, len(guardianIDs))
	for _, id := range guardianIDs {
		if _, done := seen[id]; done {
			continue
		}
		seen[id] = struct{}{}
		profile, ok := profiles[id]
		if !ok || profile.Email == nil || *profile.Email == "" {
			continue
		}
		if _, err := s.cfg.Outbox.Enqueue(ctx, platformService.EnqueueRequest{
			Kind: platformModels.EmailKindAppointmentReminder,
			Payload: map[string]any{
				apptPayloadRecipient:   *profile.Email,
				apptPayloadFirstName:   profile.FirstName,
				apptPayloadLastName:    profile.LastName,
				apptPayloadTitle:       effective.Title,
				apptPayloadWhen:        whenText,
				apptPayloadLocation:    location,
				apptPayloadSchoolName:  schoolName,
				apptPayloadPortalURL:   s.cfg.ParentsURL,
				apptPayloadLogoURL:     logoURL,
				apptPayloadMotoLogoURL: motoLogoURL,
			},
			// One reminder per occurrence per guardian, forever. The key also
			// carries the guardian so two parents of the same child both get one.
			IdempotencyKey:    appointmentReminderKey(appointment.ID, occurrence, id),
			RelatedEntityType: platformModels.EmailRelatedTypeAppointment,
			RelatedEntityID:   appointment.ID,
		}); err != nil {
			return queued, fmt.Errorf("calendar: enqueue appointment reminder: %w", err)
		}
		queued++
	}

	s.logger().Info("appointment reminders queued",
		slog.Int64("appointment_id", appointment.ID),
		slog.String("occurrence_date", occurrence.String()),
		slog.Int("recipient_count", queued),
	)
	return queued, nil
}

func appointmentReminderKey(appointmentID int64, occurrence timezone.Date, guardianProfileID int64) string {
	return fmt.Sprintf("%s:%d:%s:%d",
		platformModels.EmailKindAppointmentReminder,
		appointmentID,
		occurrence.String(),
		guardianProfileID,
	)
}
