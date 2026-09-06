package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

const maxConcurrentReminderPushDispatches = 8

type reminderPushDelivery struct {
	appointment *appointments.Appointment
	occurrence  appointments.Date
	accountID   int64
	profileIDs  []int64
	locale      string

	// from and to are the scan window this occurrence fell due in. Preparation
	// re-measures the occurrence against them, because an edit between claim and
	// dispatch can move it: one that is still due delivers now, one that moved
	// away releases its claim and waits for the tick that reaches its new moment.
	from time.Time
	to   time.Time
}

// stillDue reports whether the occurrence starts inside the window this delivery
// was scanned in.
func (d reminderPushDelivery) stillDue(startsAt time.Time) bool {
	return !startsAt.Before(d.from) && startsAt.Before(d.to)
}

// PrepareAppointmentReminders evaluates and claims reminders inside the
// workflow-owned transaction. The returned continuation rechecks permissions,
// edits, and preferences after commit before dispatching push notifications.
func (s *preparation) PrepareAppointmentReminders(ctx context.Context, from, to time.Time) (int, func(context.Context) error, error) {
	var deliveries []reminderPushDelivery
	queued, err := s.enqueueDueAppointmentReminders(ctx, from, to, &deliveries)
	if err != nil || len(deliveries) == 0 {
		return queued, nil, err
	}
	return queued, func(dispatchCtx context.Context) error {
		return s.dispatchReminderPushes(dispatchCtx, deliveries)
	}, nil
}

func (s *preparation) enqueueDueAppointmentReminders(ctx context.Context, from, to time.Time, deliveries *[]reminderPushDelivery) (int, error) {
	if !to.After(from) {
		return 0, nil
	}
	// E-mail and push are independent channels. A deployment without an outbox
	// still owes its guardians the push, so only the enqueue itself is gated on
	// the outbox — never the scan. Scanning is pointless only when neither
	// channel can produce anything.
	if s.deps.Email == nil && !s.reminderPushConfigured() {
		return 0, nil
	}

	values, err := appointments.FindDueGuardianReminders(ctx, s.deps.Appointments, from, to)
	if err != nil || len(values) == 0 {
		return 0, err
	}
	due := make([]dueAppointmentReminder, 0, len(values))
	for _, value := range values {
		due = append(due, dueAppointmentReminder{appointment: value.Appointment, effective: value.Effective, occurrence: value.Occurrence})
	}
	return s.enqueueReminderBatch(ctx, due, from, to, deliveries)
}

func (s *preparation) enqueueReminderBatch(
	ctx context.Context,
	due []dueAppointmentReminder,
	from, to time.Time,
	deliveries *[]reminderPushDelivery,
) (int, error) {
	dueIDs := dueReminderAppointmentIDs(due)
	recipients, err := s.deps.Audiences(ctx, dueIDs)
	if err != nil {
		return 0, fmt.Errorf("calendar: resolve reminder recipients: %w", err)
	}
	preferences, err := s.loadReminderAudiencePreferences(ctx, recipients)
	if err != nil {
		return 0, err
	}
	brand := s.loadReminderBrand(ctx, due[0].appointment.TenantID)
	queued := 0
	for _, reminder := range due {
		count, err := s.enqueueAppointmentReminder(ctx, reminder, recipients[reminder.appointment.ID], preferences, brand, from, to, deliveries)
		if err != nil {
			return queued, err
		}
		queued += count
	}
	return queued, nil
}

type dueAppointmentReminder struct {
	appointment *appointments.Appointment
	effective   *appointments.Appointment
	occurrence  appointments.Date
}

func dueReminderAppointmentIDs(reminders []dueAppointmentReminder) []int64 {
	ids := make([]int64, 0, len(reminders))
	seen := make(map[int64]bool)
	for _, reminder := range reminders {
		appendDistinctID(&ids, seen, reminder.appointment.ID)
	}
	return ids
}

// reminderPushConfigured reports whether the push half of the reminder can run
// at all. Without a notifier there is no channel, and without the consent
// service there is no permission to address anybody — in both cases the scan
// would only ever produce e-mail.
func (s *preparation) reminderPushConfigured() bool {
	return s.deps.Push != nil && s.deps.FilterPush != nil
}

type reminderBrand struct {
	schoolName  string
	logoURL     string
	motoLogoURL string
}

type reminderAudiencePreferences struct {
	emailUnfiltered bool
	emailAccounts   map[int64]bool
	pushAccounts    map[int64]bool
}

func (s *preparation) loadReminderBrand(ctx context.Context, tenantID int64) reminderBrand {
	schoolName, logoURL, motoLogoURL := s.deps.Brand(ctx, tenantID)
	return reminderBrand{schoolName: schoolName, logoURL: logoURL, motoLogoURL: motoLogoURL}
}

func (s *preparation) loadReminderAudiencePreferences(ctx context.Context, recipients map[int64]ports.GuardianAudience) (reminderAudiencePreferences, error) {
	result := reminderAudiencePreferences{emailUnfiltered: s.deps.FilterEmail == nil, emailAccounts: make(map[int64]bool), pushAccounts: make(map[int64]bool)}
	accountIDs := reminderRecipientAccountIDs(recipients)
	if len(accountIDs) == 0 {
		return result, nil
	}
	if s.deps.FilterEmail != nil {
		emailAccounts, err := s.deps.FilterEmail(ctx, accountIDs)
		if err != nil {
			return result, fmt.Errorf("calendar: filter reminder e-mail preferences: %w", err)
		}
		result.emailAccounts = int64BoolSet(emailAccounts)
	}
	if s.reminderPushConfigured() {
		pushAccounts, err := s.deps.FilterPush(ctx, accountIDs)
		if err != nil {
			return result, fmt.Errorf("calendar: filter reminder push preferences: %w", err)
		}
		result.pushAccounts = int64BoolSet(pushAccounts)
	}
	return result, nil
}

func reminderRecipientAccountIDs(recipients map[int64]ports.GuardianAudience) []int64 {
	ids := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, recipientSet := range recipients {
		for _, profile := range recipientSet.Profiles {
			if profile.AccountID != nil && *profile.AccountID > 0 {
				appendDistinctID(&ids, seen, *profile.AccountID)
			}
		}
	}
	return ids
}

func int64BoolSet(ids []int64) map[int64]bool {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

func (p reminderAudiencePreferences) emailAllowed(accountID *int64) bool {
	return accountID == nil || *accountID <= 0 || p.emailUnfiltered || p.emailAccounts[*accountID]
}

type reminderPushAudience struct {
	profilesByAccount map[int64][]int64
	localeByAccount   map[int64]string
}

// enqueueAppointmentReminder queues one reminder mail per reachable guardian of
// the appointment and reports how many rows it wrote.
func (s *preparation) enqueueAppointmentReminder(
	ctx context.Context,
	reminder dueAppointmentReminder,
	recipients ports.GuardianAudience,
	preferences reminderAudiencePreferences,
	brand reminderBrand,
	from, to time.Time,
	deliveries *[]reminderPushDelivery,
) (int, error) {
	if len(recipients.GuardianIDs) == 0 {
		return 0, nil
	}
	queued, audience, err := s.enqueueReminderAudience(ctx, reminder, recipients, preferences, brand)
	if err != nil {
		return queued, err
	}
	if err := s.claimReminderPushAudience(ctx, reminder, audience, from, to, deliveries); err != nil {
		return queued, err
	}
	s.deps.Logger.Info("appointment reminders queued",
		slog.Int64("appointment_id", reminder.appointment.ID),
		slog.String("occurrence_date", reminder.occurrence.String()),
		slog.Int("recipient_count", len(audience.profilesByAccount)),
	)
	return queued, nil
}

func (s *preparation) enqueueReminderAudience(
	ctx context.Context,
	reminder dueAppointmentReminder,
	recipients ports.GuardianAudience,
	preferences reminderAudiencePreferences,
	brand reminderBrand,
) (int, reminderPushAudience, error) {
	queued := 0
	audience := reminderPushAudience{
		profilesByAccount: make(map[int64][]int64, len(recipients.GuardianIDs)),
		localeByAccount:   make(map[int64]string, len(recipients.GuardianIDs)),
	}
	for _, profile := range uniqueReminderGuardianProfiles(recipients) {
		inserted, err := s.enqueueReminderEmail(ctx, reminder, recipients, profile, preferences, brand)
		if err != nil {
			return queued, audience, err
		}
		if inserted {
			queued++
		}
		if profile.AccountID != nil && *profile.AccountID > 0 && preferences.pushAccounts[*profile.AccountID] {
			audience.profilesByAccount[*profile.AccountID] = append(audience.profilesByAccount[*profile.AccountID], profile.ID)
			if profile.PortalLocale != nil {
				audience.localeByAccount[*profile.AccountID] = *profile.PortalLocale
			}
		}
	}
	return queued, audience, nil
}

func uniqueReminderGuardianProfiles(recipients ports.GuardianAudience) []*ports.GuardianProfile {
	profiles := make([]*ports.GuardianProfile, 0, len(recipients.GuardianIDs))
	seen := make(map[int64]bool, len(recipients.GuardianIDs))
	for _, id := range recipients.GuardianIDs {
		if profile := recipients.Profiles[id]; profile != nil && !seen[id] {
			seen[id] = true
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

func (s *preparation) enqueueReminderEmail(
	ctx context.Context,
	reminder dueAppointmentReminder,
	recipients ports.GuardianAudience,
	profile *ports.GuardianProfile,
	preferences reminderAudiencePreferences,
	brand reminderBrand,
) (bool, error) {
	if s.deps.Email == nil || profile.Email == nil || *profile.Email == "" || !preferences.emailAllowed(profile.AccountID) {
		return false, nil
	}
	location := ""
	if reminder.effective.Location != nil {
		location = *reminder.effective.Location
	}
	payload := map[string]any{
		apptPayloadRecipient: *profile.Email, apptPayloadFirstName: profile.FirstName, apptPayloadLastName: profile.LastName,
		apptPayloadTitle: reminder.effective.Title, apptPayloadWhen: s.deps.WhenText(reminder.effective), apptPayloadLocation: location,
		apptPayloadSchoolName: brand.schoolName, apptPayloadPortalURL: s.deps.ParentsURL, apptPayloadLogoURL: brand.logoURL, apptPayloadMotoLogoURL: brand.motoLogoURL,
	}
	if profile.AccountID != nil && *profile.AccountID > 0 && len(recipients.StudentsByGuardian[profile.ID]) > 0 {
		payload[apptPayloadGuardianAccountID] = *profile.AccountID
		payload[apptPayloadStudentIDs] = append([]int64(nil), recipients.StudentsByGuardian[profile.ID]...)
	}
	inserted, err := s.deps.Email(ctx, appointmentReminderKey(reminder.appointment.ID, reminder.appointment.Revision, reminder.occurrence, profile.ID), reminder.appointment.ID, payload)
	if err != nil {
		return false, fmt.Errorf("calendar: enqueue appointment reminder: %w", err)
	}
	return inserted, nil
}

func (s *preparation) claimReminderPushAudience(
	ctx context.Context,
	reminder dueAppointmentReminder,
	audience reminderPushAudience,
	from, to time.Time,
	deliveries *[]reminderPushDelivery,
) error {
	for accountID, profileIDs := range audience.profilesByAccount {
		claimedProfileIDs := profileIDs[:0]
		for _, profileID := range profileIDs {
			claimed, err := s.claimReminderPush(ctx, reminder.appointment, reminder.occurrence, profileID)
			if err != nil {
				return fmt.Errorf("calendar: claim reminder push delivery: %w", err)
			}
			if claimed {
				claimedProfileIDs = append(claimedProfileIDs, profileID)
			}
		}
		if len(claimedProfileIDs) == 0 {
			continue
		}
		appointmentCopy := *reminder.appointment
		*deliveries = append(*deliveries, reminderPushDelivery{
			appointment: &appointmentCopy,
			occurrence:  reminder.occurrence,
			accountID:   accountID,
			profileIDs:  append([]int64(nil), claimedProfileIDs...),
			locale:      audience.localeByAccount[accountID],
			from:        from,
			to:          to,
		})
	}
	return nil
}

func (s *preparation) dispatchReminderPushes(ctx context.Context, deliveries []reminderPushDelivery) error {
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
			appointment, profileIDs, studentIDs, err := s.prepareReminderPushDispatch(ctx, delivery)
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
			// Claims are keyed by revision, and preparation may have re-taken them
			// under a newer one. Every release from here on has to name the revision
			// the claims are actually held at, not the one the scan started with.
			delivery.appointment = appointment
			delivery.profileIDs = profileIDs
			dispatched, err := s.deps.Push(ctx, appointment.TenantID, []int64{delivery.accountID}, studentIDs, delivery.locale)
			if err == nil && dispatched {
				return
			}
			// None of these three delivered anything: no device was registered or
			// no VAPID key was configured, the school switched notifications off,
			// or the delivery window was closed. All three are states rather than
			// failures, so the tick stays quiet and the scan window closes
			// normally — a school's quiet hours drop a push here exactly as they
			// do for every other producer, because the router itself never queues
			// an event it may not deliver now. The reminder's primary channel is
			// unaffected: the e-mail is already committed.
			//
			// The release is therefore not a deferral, it keeps the claim honest:
			// a claim records that a push went out, so a window that gets scanned
			// again anyway — an unrelated failure in the same tenant pass kept it
			// open, or the school raised the lead time — can still deliver. A
			// genuine dispatch failure below takes that path deliberately.
			if s.deps.NoDelivery(err) {
				if releaseErr := s.releaseReminderPushDelivery(ctx, delivery); releaseErr != nil {
					errs <- releaseErr
				}
				return
			}
			s.deps.Logger.Warn("calendar: appointment reminder push failed",
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
// claim still belongs to a live, guardian-notified appointment that is still due
// in this delivery's scan window, and that the guardian still consents to
// reminders. The transaction commits before the synchronous Web Push request
// begins, so network latency never holds a DB connection or a lifecycle row lock.
//
// It also reports the children this account is still let through by, so the
// device lookup can ask the access question once more — that lookup runs in yet
// another transaction, and the payload it produces is rendered on a lock screen.
//
// The appointment it returns is the one the claims are now held under, which is
// not necessarily the one the scan started with: see the revision handling below.
func (s *preparation) prepareReminderPushDispatch(ctx context.Context, delivery reminderPushDelivery) (*appointments.Appointment, []int64, []int64, error) {
	var appointment *appointments.Appointment
	var profileIDs []int64
	var studentIDs []int64
	err := s.runtime.WithinTenant(ctx, delivery.appointment.TenantID, func(txCtx context.Context) error {
		current, err := s.deps.Appointments.FindReminderCandidateForUpdate(txCtx, delivery.appointment.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return nil
		}
		// An edit between claim and dispatch bumps the revision, which invalidates
		// the claim this delivery holds — nothing can ever match it again, so it is
		// released here. Dropping the reminder with it would lose it for good: the
		// scan boundary has already moved past this occurrence and no later tick
		// offers it again. So the claim is re-taken under the new revision instead,
		// and only the checks below decide whether the reminder is still warranted.
		revised := current.Revision != delivery.appointment.Revision
		if revised {
			if err := s.releaseReminderPushProfiles(txCtx, delivery.appointment, delivery.occurrence, delivery.profileIDs); err != nil {
				return err
			}
		}
		rule, err := s.deps.Appointments.FindRecurrenceRule(txCtx, current.ID)
		if err != nil {
			return fmt.Errorf("reload reminder recurrence: %w", err)
		}
		if (rule == nil && current.StartDate != delivery.occurrence) ||
			(rule != nil && !appointments.OccurrenceExists(current, rule, delivery.occurrence)) {
			return nil
		}
		overrides, err := s.deps.Appointments.FindOccurrenceOverrides(txCtx, []int64{current.ID}, []appointments.Date{delivery.occurrence})
		if err != nil {
			return fmt.Errorf("reload reminder override: %w", err)
		}
		var override *appointments.AppointmentOccurrenceOverride
		if len(overrides) > 0 {
			override = overrides[0]
		}
		if override != nil && override.Cancelled {
			return nil
		}
		// An edit can also have moved the occurrence. A reminder is only worth
		// sending at the moment it was due for, so one that moved away is not sent
		// now — returning nothing releases the claim (below), which leaves the tick
		// that reaches the occurrence's new moment free to take it.
		if !delivery.stillDue(appointments.OccurrenceStartInstant(appointments.EffectiveOccurrence(current, delivery.occurrence, override))) {
			return nil
		}
		if s.deps.FilterPush == nil {
			return nil
		}
		optedIn, err := s.deps.FilterPush(txCtx, []int64{delivery.accountID})
		if err != nil {
			return fmt.Errorf("filter opted-in guardians: %w", err)
		}
		if len(optedIn) == 0 {
			return nil
		}
		audiences, err := s.deps.Audiences(txCtx, []int64{current.ID})
		if err != nil {
			return err
		}
		recipients := audiences[current.ID]
		reachableSet := make(map[int64]struct{}, len(recipients.GuardianIDs))
		for _, profileID := range recipients.GuardianIDs {
			profile, ok := recipients.Profiles[profileID]
			if ok && profile.AccountID != nil && *profile.AccountID == delivery.accountID {
				reachableSet[profileID] = struct{}{}
			}
		}
		for _, profileID := range delivery.profileIDs {
			if _, ok := reachableSet[profileID]; !ok {
				continue
			}
			// The claim released above has to be replaced before the push goes out,
			// or a re-scan of this window could send the reminder a second time.
			if revised {
				claimed, err := s.deps.Appointments.ClaimReminderPushDelivery(txCtx, current.ID, current.Revision, delivery.occurrence, profileID)
				if err != nil {
					return fmt.Errorf("claim revised reminder push delivery: %w", err)
				}
				if !claimed {
					continue
				}
			}
			profileIDs = append(profileIDs, profileID)
		}
		if len(profileIDs) == 0 {
			return nil
		}
		studentIDs = recipients.StudentIDs([]int64{delivery.accountID})
		appointment = current
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return appointment, profileIDs, studentIDs, nil
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
func (s *preparation) claimReminderPush(ctx context.Context, appointment *appointments.Appointment, occurrence appointments.Date, profileID int64) (bool, error) {
	return s.deps.Appointments.ClaimReminderPushDelivery(ctx, appointment.ID, appointment.Revision, occurrence, profileID)
}

func (s *preparation) releaseReminderPush(ctx context.Context, appointment *appointments.Appointment, occurrence appointments.Date, profileID int64) error {
	cleanupCtx := context.WithoutCancel(s.runtime.Detached(ctx))
	return s.runtime.WithinTenant(cleanupCtx, appointment.TenantID, func(txCtx context.Context) error {
		return s.deps.Appointments.ReleaseReminderPushDelivery(txCtx, appointment.ID, appointment.Revision, occurrence, profileID)
	})
}

// releaseReminderPushProfiles drops the claims one appointment revision holds
// for these guardians, inside the caller's transaction. Used where a claim is
// superseded rather than abandoned — the caller commits the release together
// with whatever replaces it.
func (s *preparation) releaseReminderPushProfiles(ctx context.Context, appointment *appointments.Appointment, occurrence appointments.Date, profileIDs []int64) error {
	for _, profileID := range profileIDs {
		if err := s.deps.Appointments.ReleaseReminderPushDelivery(ctx, appointment.ID, appointment.Revision, occurrence, profileID); err != nil {
			return fmt.Errorf("calendar: release superseded reminder push delivery: %w", err)
		}
	}
	return nil
}

func (s *preparation) releaseReminderPushDelivery(ctx context.Context, delivery reminderPushDelivery) error {
	for _, profileID := range delivery.profileIDs {
		if err := s.releaseReminderPush(ctx, delivery.appointment, delivery.occurrence, profileID); err != nil {
			return fmt.Errorf("calendar: release reminder push delivery: %w", err)
		}
	}
	return nil
}

func appointmentReminderKey(appointmentID int64, revision int, occurrence appointments.Date, guardianProfileID int64) string {
	return fmt.Sprintf("appointment_reminder:%d:%d:%s:%d",
		appointmentID,
		revision,
		occurrence.String(),
		guardianProfileID,
	)
}

const (
	apptPayloadRecipient         = "recipient_email"
	apptPayloadFirstName         = "first_name"
	apptPayloadLastName          = "last_name"
	apptPayloadTitle             = "title"
	apptPayloadWhen              = "when_text"
	apptPayloadLocation          = "location"
	apptPayloadSchoolName        = "school_name"
	apptPayloadPortalURL         = "portal_url"
	apptPayloadLogoURL           = "logo_url"
	apptPayloadMotoLogoURL       = "moto_logo_url"
	apptPayloadGuardianAccountID = "guardian_account_id"
	apptPayloadStudentIDs        = "student_ids"
)

type preparation struct {
	deps    ports.DeliveryDependencies
	runtime ports.CommandRuntime
}

func NewPreparation(deps ports.DeliveryDependencies, runtime ports.CommandRuntime) ports.GuardianPreparation {
	if deps.Appointments == nil || deps.Audiences == nil || deps.Brand == nil || deps.WhenText == nil || deps.NoDelivery == nil {
		panic("reminder preparation: required inputs missing")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &preparation{deps: deps, runtime: runtime}
}
func appendDistinctID(ids *[]int64, seen map[int64]bool, id int64) {
	if !seen[id] {
		seen[id] = true
		*ids = append(*ids, id)
	}
}
