package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// allDayReminderHour anchors the reminder for an all-day appointment. An all-day
// row carries no meaningful start_time, and taking midnight would put a 24-hour
// reminder in the middle of the night before. 08:00 local is when the school day
// starts, so "tomorrow" reaches parents over breakfast.
const allDayReminderHour = 8

const maxConcurrentReminderPushDispatches = 8

type reminderPushDelivery struct {
	appointment *calModels.Appointment
	occurrence  timezone.Date
	accountID   int64
	profileIDs  []int64
}

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
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, errors.New("calendar: tenant id is required for appointment reminders")
	}

	// The scheduler already iterates tenants in a transaction, but reminder
	// e-mails, delivery claims, and their after-commit push dispatch must share
	// one transaction of their own. Otherwise a later scan error could roll back
	// an e-mail while a separately committed claim suppresses its push retry.
	var queued int
	var dispatchErr error
	var deliveries []reminderPushDelivery
	independentCtx := tenant.ContextWithoutAfterCommitHooks(modelBase.ContextWithoutTx(ctx))
	err := tenant.WithTenantTx(independentCtx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var enqueueErr error
		queued, enqueueErr = s.enqueueDueAppointmentReminders(txCtx, from, to, &deliveries)
		if enqueueErr == nil && len(deliveries) > 0 {
			postCommitCtx := tenant.ContextWithoutAfterCommitHooks(modelBase.ContextWithoutTx(txCtx))
			tenant.RegisterAfterCommit(txCtx, func() {
				dispatchErr = s.dispatchReminderPushes(postCommitCtx, deliveries)
			})
		}
		return enqueueErr
	})
	if err != nil {
		return queued, err
	}
	return queued, dispatchErr
}

func (s *service) enqueueDueAppointmentReminders(ctx context.Context, from, to time.Time, deliveries *[]reminderPushDelivery) (int, error) {
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
			count, _, err := s.enqueueAppointmentReminder(ctx, current, effective, occurrence, deliveries)
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
	deliveries *[]reminderPushDelivery,
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
		sendEmail := profile.Email != nil && *profile.Email != ""
		if sendEmail && profile.AccountID != nil && *profile.AccountID > 0 && s.cfg.Preferences != nil {
			optedIn, err := s.cfg.Preferences.FilterNotOptedOut(ctx, notifications.TypeParentAppointmentReminder, []int64{*profile.AccountID})
			if err != nil {
				return queued, nil, fmt.Errorf("calendar: filter reminder e-mail preferences: %w", err)
			}
			sendEmail = len(optedIn) > 0
		}
		if sendEmail {
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
			claimed, err := s.claimReminderPush(ctx, appointment, occurrence, profileID)
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
		appointmentCopy := *appointment
		*deliveries = append(*deliveries, reminderPushDelivery{
			appointment: &appointmentCopy,
			occurrence:  occurrence,
			accountID:   accountID,
			profileIDs:  append([]int64(nil), claimedProfileIDs...),
		})
	}

	s.logger().Info("appointment reminders queued",
		slog.Int64("appointment_id", appointment.ID),
		slog.String("occurrence_date", occurrence.String()),
		slog.Int("recipient_count", len(pushProfilesByAccount)),
	)
	return queued, nil, nil
}

func (s *service) dispatchReminderPushes(ctx context.Context, deliveries []reminderPushDelivery) error {
	sem := make(chan struct{}, maxConcurrentReminderPushDispatches)
	// A failed preparation can report both the original failure and a failed
	// claim release; cancellation can do the same for every pending delivery.
	errs := make(chan error, len(deliveries)*2+1)
	var wg sync.WaitGroup
	for i, delivery := range deliveries {
		if ctx.Err() != nil {
			for _, pending := range deliveries[i:] {
				if err := s.releaseReminderPushDelivery(ctx, pending); err != nil {
					errs <- err
				}
			}
			errs <- ctx.Err()
			break
		}
		wg.Add(1)
		go func(delivery reminderPushDelivery) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				if err := s.releaseReminderPushDelivery(ctx, delivery); err != nil {
					errs <- err
				}
				errs <- ctx.Err()
				return
			}
			defer func() { <-sem }()
			appointment, profileIDs, err := s.prepareReminderPushDispatch(ctx, delivery)
			if err != nil {
				if releaseErr := s.releaseReminderPushDelivery(ctx, delivery); releaseErr != nil {
					errs <- releaseErr
				}
				errs <- fmt.Errorf("calendar: prepare appointment reminder push: %w", err)
				return
			}
			if len(profileIDs) == 0 {
				if err := s.releaseReminderPushDelivery(ctx, delivery); err != nil {
					errs <- err
				}
				return
			}
			staleProfiles := withoutReminderProfiles(delivery.profileIDs, profileIDs)
			if len(staleProfiles) > 0 {
				staleDelivery := delivery
				staleDelivery.profileIDs = staleProfiles
				if err := s.releaseReminderPushDelivery(ctx, staleDelivery); err != nil {
					errs <- err
					return
				}
			}
			delivery.profileIDs = profileIDs
			dispatched, err := s.dispatchGuardianAccountReminderDevices(ctx, appointment, []int64{delivery.accountID})
			if err == nil && dispatched {
				return
			}
			if errors.Is(err, notifications.ErrNoWebPushSubscribers) || errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
				return
			}
			s.logger().Warn("calendar: appointment reminder push failed",
				slog.Int64("appointment_id", delivery.appointment.ID),
				slog.Int64("guardian_account_id", delivery.accountID),
			)
			if releaseErr := s.releaseReminderPushDelivery(ctx, delivery); releaseErr != nil {
				errs <- releaseErr
				return
			}
			if err != nil {
				errs <- fmt.Errorf("calendar: dispatch appointment reminder push: %w", err)
			} else {
				errs <- errors.New("calendar: appointment reminder push was not accepted")
			}
		}(delivery)
	}
	wg.Wait()
	close(errs)
	var joined []error
	for err := range errs {
		joined = append(joined, err)
	}
	return errors.Join(joined...)
}

// prepareReminderPushDispatch takes a short tenant transaction to ensure the
// claim still belongs to a live, guardian-notified appointment and that the
// guardian still consents to reminders. The transaction commits before the
// synchronous Web Push request begins, so network latency never holds a DB
// connection or a lifecycle row lock.
func (s *service) prepareReminderPushDispatch(ctx context.Context, delivery reminderPushDelivery) (*calModels.Appointment, []int64, error) {
	var appointment *calModels.Appointment
	var profileIDs []int64
	err := tenant.WithTenantTx(ctx, s.cfg.DB, delivery.appointment.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		current, err := s.cfg.AppointmentRepo.LockReminderCandidate(txCtx, delivery.appointment.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return nil
		}
		if current.Revision != delivery.appointment.Revision {
			return nil
		}
		rule, err := s.cfg.RecurrenceRepo.FindByAppointmentID(txCtx, current.ID)
		if err != nil {
			return fmt.Errorf("reload reminder recurrence: %w", err)
		}
		if (rule == nil && current.StartDate != delivery.occurrence) ||
			(rule != nil && !occurrenceExists(current, rule, delivery.occurrence)) {
			return nil
		}
		overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(txCtx, []int64{current.ID}, []timezone.Date{delivery.occurrence})
		if err != nil {
			return fmt.Errorf("reload reminder override: %w", err)
		}
		if len(overrides) > 0 && overrides[0].Cancelled {
			return nil
		}
		if s.cfg.Preferences == nil {
			return nil
		}
		optedIn, err := s.cfg.Preferences.FilterOptedIn(txCtx, notifications.TypeParentAppointmentReminder, []int64{delivery.accountID})
		if err != nil {
			return fmt.Errorf("filter opted-in guardians: %w", err)
		}
		if len(optedIn) == 0 {
			return nil
		}
		reachable, profiles, err := s.reachableGuardianRecipients(txCtx, current.ID)
		if err != nil {
			return err
		}
		reachableSet := make(map[int64]struct{}, len(reachable))
		for _, profileID := range reachable {
			profile, ok := profiles[profileID]
			if ok && profile.AccountID != nil && *profile.AccountID == delivery.accountID {
				reachableSet[profileID] = struct{}{}
			}
		}
		for _, profileID := range delivery.profileIDs {
			if _, ok := reachableSet[profileID]; ok {
				profileIDs = append(profileIDs, profileID)
			}
		}
		if len(profileIDs) == 0 {
			return nil
		}
		appointment = current
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return appointment, profileIDs, nil
}

func withoutReminderProfiles(claimed, reachable []int64) []int64 {
	reachableSet := make(map[int64]struct{}, len(reachable))
	for _, profileID := range reachable {
		reachableSet[profileID] = struct{}{}
	}
	stale := make([]int64, 0, len(claimed))
	for _, profileID := range claimed {
		if _, ok := reachableSet[profileID]; !ok {
			stale = append(stale, profileID)
		}
	}
	return stale
}

// Reminder delivery claims are created in the same transaction as the e-mail
// outbox row. The push is dispatched by an after-commit hook, so a durable
// claim can never outlive a rolled-back reminder.
func (s *service) claimReminderPush(ctx context.Context, appointment *calModels.Appointment, occurrence timezone.Date, profileID int64) (bool, error) {
	return s.cfg.RecipientRepo.ClaimReminderPush(ctx, appointment.ID, appointment.Revision, occurrence, profileID)
}

func (s *service) releaseReminderPush(ctx context.Context, appointment *calModels.Appointment, occurrence timezone.Date, profileID int64) error {
	cleanupCtx := context.WithoutCancel(modelBase.ContextWithoutTx(ctx))
	return tenant.WithTenantTx(cleanupCtx, s.cfg.DB, appointment.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.cfg.RecipientRepo.ReleaseReminderPush(txCtx, appointment.ID, appointment.Revision, occurrence, profileID)
	})
}

func (s *service) releaseReminderPushDelivery(ctx context.Context, delivery reminderPushDelivery) error {
	for _, profileID := range delivery.profileIDs {
		if err := s.releaseReminderPush(ctx, delivery.appointment, delivery.occurrence, profileID); err != nil {
			return fmt.Errorf("calendar: release reminder push delivery: %w", err)
		}
	}
	return nil
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
