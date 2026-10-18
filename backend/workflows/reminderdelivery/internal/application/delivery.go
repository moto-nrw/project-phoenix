package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

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
	if s.deps.Outbox == nil {
		return queued, nil, errors.New("calendar: reminder push outbox is required")
	}
	ids := make([]int64, 0, len(deliveries))
	tenantID := deliveries[0].appointment.TenantID
	for _, delivery := range deliveries {
		snapshot := delivery.snapshot()
		slices.Sort(snapshot.ProfileIDs)
		key := appointmentReminderKey(snapshot.Appointment.ID, snapshot.Appointment.Revision, snapshot.Occurrence, snapshot.ProfileIDs[0])
		id, err := s.deps.Outbox.Enqueue(ctx, key, snapshot)
		if err != nil {
			return queued, nil, err
		}
		ids = append(ids, id)
	}
	return queued, func(dispatchCtx context.Context) error {
		var failures []error
		for index, id := range ids {
			if err := s.deps.Outbox.Deliver(dispatchCtx, tenantID, id); err != nil {
				// A stored terminal result does not call the provider, so it cannot
				// release the legacy audience claim acquired by this scan itself.
				failures = append(failures, errors.Join(err, s.releaseReminderPushDelivery(dispatchCtx, deliveries[index])))
			}
		}
		return errors.Join(failures...)
	}, nil
}

func (d reminderPushDelivery) snapshot() ports.ReminderPushSnapshot {
	return ports.ReminderPushSnapshot{Appointment: *d.appointment, Occurrence: d.occurrence,
		AccountID: d.accountID, ProfileIDs: append([]int64(nil), d.profileIDs...), Locale: d.locale,
		From: d.from, To: d.to, MaxAttempts: 1,
	}
}

// NewPushDelivery restores and revalidates a committed reminder. Both targeted
// synchronous dispatch and crash recovery call this same capability.
func NewPushDelivery(deps ports.DeliveryDependencies, runtime ports.CommandRuntime) func(context.Context, []byte) error {
	s := NewPreparation(deps, runtime).(*preparation)
	return func(ctx context.Context, raw []byte) error {
		var value ports.ReminderPushSnapshot
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		delivery := reminderPushDelivery{appointment: &value.Appointment, occurrence: value.Occurrence,
			accountID: value.AccountID, profileIDs: value.ProfileIDs, locale: value.Locale, from: value.From, to: value.To}
		return s.dispatchReminderPush(ctx, delivery)
	}
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

func (s *preparation) dispatchReminderPush(ctx context.Context, delivery reminderPushDelivery) error {
	appointment, profileIDs, studentIDs, err := s.prepareReminderPushDispatch(ctx, delivery)
	if err != nil {
		return errors.Join(err, s.releaseReminderPushDelivery(ctx, delivery))
	}
	if len(profileIDs) == 0 {
		if err := s.releaseReminderPushDelivery(ctx, delivery); err != nil {
			return err
		}
		return ports.ErrReminderSkipped
	}
	staleProfiles := withoutReminderProfiles(delivery.profileIDs, profileIDs)
	if len(staleProfiles) > 0 {
		stale := delivery
		stale.profileIDs = staleProfiles
		if err := s.releaseReminderPushDelivery(ctx, stale); err != nil {
			return err
		}
	}
	delivery.appointment, delivery.profileIDs = appointment, profileIDs
	dispatched, err := s.deps.Push(ctx, appointment.TenantID, []int64{delivery.accountID}, studentIDs, delivery.locale)
	if err == nil && dispatched {
		return nil
	}
	if releaseErr := s.releaseReminderPushDelivery(ctx, delivery); releaseErr != nil {
		return errors.Join(err, releaseErr)
	}
	if err == nil || s.deps.NoDelivery(err) {
		return ports.ErrReminderAudienceUnavailable
	}
	s.deps.Logger.Warn("calendar: appointment reminder push failed",
		slog.Int64("appointment_id", appointment.ID),
		slog.Int64("guardian_account_id", delivery.accountID))
	return fmt.Errorf("calendar: dispatch appointment reminder push: %w", err)
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
		var prepareErr error
		appointment, profileIDs, studentIDs, prepareErr = s.prepareReminderPushInTx(txCtx, delivery)
		return prepareErr
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
// outbox row. The leased push is dispatched after commit, so a durable
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
	if deps.Now == nil {
		deps.Now = time.Now
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
