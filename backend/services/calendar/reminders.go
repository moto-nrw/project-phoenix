package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
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
// cannot produce a second mail for the same occurrence revision.
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
	movedOverrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndStartDates(ctx, ids, dateRange(fromDate, toDate))
	if err != nil {
		return 0, fmt.Errorf("calendar: load moved reminder overrides: %w", err)
	}
	movedByAppointment := make(map[int64][]*calModels.AppointmentOccurrenceOverride, len(movedOverrides))
	for _, override := range movedOverrides {
		movedByAppointment[override.AppointmentID] = append(movedByAppointment[override.AppointmentID], override)
	}
	// The repository's coarse recurrence predicate cannot express the final
	// date of every count-bounded rule (weekly and monthly rules may carry
	// several selected days). A moved occurrence can still be due after the
	// base count is exhausted, so retain it when its effective start is in the
	// window; bounded recurrence expansion is capped at 366.
	activeAppointments := appointments[:0]
	for _, appointment := range appointments {
		rule := recurrenceByAppointment[appointment.ID]
		if rule != nil && rule.OccurrenceCount != nil && !hasOccurrenceInWindow(appointment, rule, fromDate, toDate) && len(movedByAppointment[appointment.ID]) == 0 {
			continue
		}
		activeAppointments = append(activeAppointments, appointment)
	}
	appointments = activeAppointments
	if len(appointments) == 0 {
		return 0, nil
	}
	ids = ids[:0]
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}

	queued := 0
	for _, appointment := range appointments {
		occurrences := reminderOccurrences(appointment, recurrenceByAppointment[appointment.ID], fromDate, toDate)
		for _, override := range movedByAppointment[appointment.ID] {
			occurrences = append(occurrences, override.OccurrenceDate)
		}
		seenOccurrences := make(map[timezone.Date]struct{}, len(occurrences))
		for _, occurrence := range occurrences {
			if _, seen := seenOccurrences[occurrence]; seen {
				continue
			}
			seenOccurrences[occurrence] = struct{}{}
			// The initial scan is deliberately broad and may have observed an
			// appointment before an edit, cancellation, or occurrence cancellation
			// committed. Re-read it under a row lock before creating an outbox row.
			// Lifecycle writes take the same lock; whichever transaction wins, the
			// loser sees the committed state and cannot leave a stale reminder.
			current, err := s.cfg.AppointmentRepo.LockReminderCandidate(ctx, appointment.ID)
			if err != nil {
				return queued, fmt.Errorf("calendar: lock reminder candidate: %w", err)
			}
			if current == nil {
				continue
			}
			currentRule, err := s.cfg.RecurrenceRepo.FindByAppointmentID(ctx, current.ID)
			if err != nil {
				return queued, fmt.Errorf("calendar: reload reminder recurrence: %w", err)
			}
			if (currentRule == nil && current.StartDate != occurrence) ||
				(currentRule != nil && !occurrenceExists(current, currentRule, occurrence)) {
				continue
			}
			currentOverrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(ctx, []int64{current.ID}, []timezone.Date{occurrence})
			if err != nil {
				return queued, fmt.Errorf("calendar: reload reminder override: %w", err)
			}
			var override *calModels.AppointmentOccurrenceOverride
			if len(currentOverrides) > 0 {
				override = currentOverrides[0]
			}
			if override != nil && override.Cancelled {
				continue
			}
			effective := appointmentWithOverride(current, occurrence, override)
			startsAt := occurrenceStartInstant(effective)
			if startsAt.Before(from) || !startsAt.Before(to) {
				continue
			}
			count, _, err := s.enqueueAppointmentReminder(ctx, current, effective, occurrence)
			if err != nil {
				return queued, err
			}
			queued += count
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

func dateRange(from, to timezone.Date) []timezone.Date {
	dates := make([]timezone.Date, 0, from.DaysUntil(to)+1)
	for date := from; !date.After(to); date = date.AddDays(1) {
		dates = append(dates, date)
	}
	return dates
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
		return time.Date(
			midnight.Year(), midnight.Month(), midnight.Day(),
			allDayReminderHour, 0, 0, 0,
			midnight.Location(),
		)
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
) (int, []int64, error) {
	guardianIDs, profiles, err := s.reachableGuardianRecipients(ctx, appointment.ID)
	if err != nil {
		return 0, nil, fmt.Errorf("calendar: resolve reminder recipients: %w", err)
	}
	if len(guardianIDs) == 0 {
		return 0, nil, nil
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
	pushProfilesByAccount := make(map[int64][]int64, len(guardianIDs))
	seen := make(map[int64]struct{}, len(guardianIDs))
	for _, id := range guardianIDs {
		if _, done := seen[id]; done {
			continue
		}
		seen[id] = struct{}{}
		profile, ok := profiles[id]
		if !ok {
			continue
		}
		if profile.Email != nil && *profile.Email != "" {
			if profile.AccountID != nil && *profile.AccountID > 0 && s.cfg.Preferences != nil {
				optedIn, err := s.cfg.Preferences.FilterOptedIn(ctx, notifications.TypeParentAppointmentReminder, []int64{*profile.AccountID})
				if err != nil {
					return queued, nil, fmt.Errorf("calendar: filter reminder e-mail preferences: %w", err)
				}
				if len(optedIn) == 0 {
					continue
				}
			}
			row, err := s.cfg.Outbox.Enqueue(ctx, platformService.EnqueueRequest{
				Kind: platformModels.EmailKindAppointmentReminder,
				Payload: map[string]any{
					apptPayloadRecipient: *profile.Email, apptPayloadFirstName: profile.FirstName, apptPayloadLastName: profile.LastName,
					apptPayloadTitle: effective.Title, apptPayloadWhen: whenText, apptPayloadLocation: location,
					apptPayloadSchoolName: schoolName, apptPayloadPortalURL: s.cfg.ParentsURL, apptPayloadLogoURL: logoURL, apptPayloadMotoLogoURL: motoLogoURL,
				},
				IdempotencyKey: appointmentReminderKey(appointment.ID, appointment.Revision, occurrence, id), RelatedEntityType: platformModels.EmailRelatedTypeAppointment, RelatedEntityID: appointment.ID,
			})
			if err != nil {
				return queued, nil, fmt.Errorf("calendar: enqueue appointment reminder: %w", err)
			}
			if row.ID != 0 {
				queued++
			}
		}
		if profile.AccountID != nil && *profile.AccountID > 0 && s.cfg.ReminderNotifier != nil && s.cfg.Preferences != nil {
			optedIn, err := s.cfg.Preferences.FilterOptedIn(ctx, notifications.TypeParentAppointmentReminder, []int64{*profile.AccountID})
			if err != nil {
				return queued, nil, fmt.Errorf("calendar: filter reminder push preferences: %w", err)
			}
			if len(optedIn) > 0 {
				pushProfilesByAccount[*profile.AccountID] = append(pushProfilesByAccount[*profile.AccountID], id)
			}
		}
	}
	for accountID, profileIDs := range pushProfilesByAccount {
		claimedProfileIDs := profileIDs[:0]
		for _, profileID := range profileIDs {
			claimed, err := s.cfg.RecipientRepo.ClaimReminderPush(ctx, appointment.ID, appointment.Revision, occurrence, profileID)
			if err != nil {
				return queued, nil, fmt.Errorf("calendar: claim reminder push delivery: %w", err)
			}
			if claimed {
				claimedProfileIDs = append(claimedProfileIDs, profileID)
			}
		}
		if len(claimedProfileIDs) == 0 {
			continue
		}
		dispatched, err := s.dispatchGuardianAccountReminderDevices(ctx, appointment, []int64{accountID})
		if err != nil || !dispatched {
			s.logger().Warn("calendar: appointment reminder push failed",
				slog.Int64("appointment_id", appointment.ID),
				slog.Int64("guardian_account_id", accountID),
			)
			if err != nil {
				s.logger().Warn("calendar: appointment reminder push error",
					slog.Int64("appointment_id", appointment.ID),
					slog.Int64("guardian_account_id", accountID),
					slog.String("error", err.Error()),
				)
			}
			for _, profileID := range claimedProfileIDs {
				if releaseErr := s.cfg.RecipientRepo.ReleaseReminderPush(ctx, appointment.ID, appointment.Revision, occurrence, profileID); releaseErr != nil {
					return queued, nil, fmt.Errorf("calendar: release reminder push delivery: %w", releaseErr)
				}
			}
			if errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
				continue
			}
			if err != nil {
				return queued, nil, fmt.Errorf("calendar: dispatch appointment reminder push: %w", err)
			}
			return queued, nil, fmt.Errorf("calendar: appointment reminder push was not accepted")
		}
	}

	s.logger().Info("appointment reminders queued",
		slog.Int64("appointment_id", appointment.ID),
		slog.String("occurrence_date", occurrence.String()),
		slog.Int("recipient_count", len(pushProfilesByAccount)),
	)
	return queued, nil, nil
}

func appointmentReminderKey(appointmentID int64, revision int, occurrence timezone.Date, guardianProfileID int64) string {
	return fmt.Sprintf("%s:%d:%d:%s:%d",
		platformModels.EmailKindAppointmentReminder,
		appointmentID,
		revision,
		occurrence.String(),
		guardianProfileID,
	)
}
