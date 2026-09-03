package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
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
	independentCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	err := tenant.WithTenantTx(independentCtx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var enqueueErr error
		queued, enqueueErr = s.enqueueDueAppointmentReminders(txCtx, from, to, &deliveries)
		if enqueueErr == nil && len(deliveries) > 0 {
			postCommitCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(txCtx))
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
	if !to.After(from) {
		return 0, nil
	}
	// E-mail and push are independent channels. A deployment without an outbox
	// still owes its guardians the push, so only the enqueue itself is gated on
	// the outbox — never the scan. Scanning is pointless only when neither
	// channel can produce anything.
	if s.cfg.Outbox == nil && !s.reminderPushConfigured() {
		return 0, nil
	}

	// The DB window is date-granular and the instant window is not: an
	// occurrence at 08:00 on day X is only due when the [from, to) instants
	// straddle it. Scan the full days the window touches and filter by instant
	// below.
	fromDate := timezone.DateFromTime(from)
	toDate := timezone.DateFromTime(to)
	appointments, err := s.listGuardianReminderCandidates(ctx, toCalendarDate(fromDate), toCalendarDate(toDate))
	if err != nil {
		return 0, fmt.Errorf("calendar: list reminder candidates: %w", err)
	}
	if len(appointments) == 0 {
		return 0, nil
	}
	due, err := s.loadDueAppointmentReminders(ctx, appointments, fromDate, toDate, from, to)
	if err != nil || len(due) == 0 {
		return 0, err
	}
	return s.enqueueReminderBatch(ctx, due, from, to, deliveries)
}

type reminderCandidateRelations struct {
	recurrences map[int64]*calModels.RecurrenceRule
	moved       map[int64][]*calModels.AppointmentOccurrenceOverride
}

func (s *service) loadReminderCandidateRelations(
	ctx context.Context,
	appointments []*calModels.Appointment,
	from, to timezone.Date,
) (reminderCandidateRelations, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, ids)
	if err != nil {
		return reminderCandidateRelations{}, fmt.Errorf("calendar: load reminder recurrences: %w", err)
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	movedOverrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndStartDates(ctx, ids, toCalendarDates(dateRange(from, to)))
	if err != nil {
		return reminderCandidateRelations{}, fmt.Errorf("calendar: load moved reminder overrides: %w", err)
	}
	movedByAppointment := make(map[int64][]*calModels.AppointmentOccurrenceOverride, len(movedOverrides))
	for _, override := range movedOverrides {
		movedByAppointment[override.AppointmentID] = append(movedByAppointment[override.AppointmentID], override)
	}
	return reminderCandidateRelations{recurrences: recurrenceByAppointment, moved: movedByAppointment}, nil
}

func eligibleReminderAppointments(
	appointments []*calModels.Appointment,
	relations reminderCandidateRelations,
	from, to timezone.Date,
) []*calModels.Appointment {
	eligible := appointments[:0]
	for _, appointment := range appointments {
		rule := relations.recurrences[appointment.ID]
		if rule != nil && rule.OccurrenceCount != nil && !hasOccurrenceInWindow(appointment, rule, from, to) && len(relations.moved[appointment.ID]) == 0 {
			continue
		}
		eligible = append(eligible, appointment)
	}
	return eligible
}

func (s *service) loadDueAppointmentReminders(
	ctx context.Context,
	appointments []*calModels.Appointment,
	fromDate, toDate timezone.Date,
	from, to time.Time,
) ([]dueAppointmentReminder, error) {
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

func (s *service) enqueueReminderBatch(
	ctx context.Context,
	due []dueAppointmentReminder,
	from, to time.Time,
	deliveries *[]reminderPushDelivery,
) (int, error) {
	dueIDs := dueReminderAppointmentIDs(due)
	recipients, err := s.reachableGuardianRecipientsByAppointment(ctx, dueIDs)
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
	appointment *calModels.Appointment
	effective   *calModels.Appointment
	occurrence  timezone.Date
}

func collectDueAppointmentReminders(
	appointments []*calModels.Appointment,
	occurrences map[int64][]timezone.Date,
	current lockedReminderCandidates,
	from, to time.Time,
) []dueAppointmentReminder {
	due := make([]dueAppointmentReminder, 0)
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
	appointment *calModels.Appointment,
	occurrence timezone.Date,
	current lockedReminderCandidates,
	from, to time.Time,
) *dueAppointmentReminder {
	rule := current.recurrences[appointment.ID]
	if (rule == nil && appointment.StartDate != toCalendarDate(occurrence)) || (rule != nil && !occurrenceExists(appointment, rule, occurrence)) {
		return nil
	}
	override := current.overrides[appointmentOccurrenceKey(appointment.ID, occurrence)]
	if override != nil && override.Cancelled {
		return nil
	}
	effective := appointmentWithOverride(appointment, occurrence, override)
	startsAt := occurrenceStartInstant(effective)
	if startsAt.Before(from) || !startsAt.Before(to) {
		return nil
	}
	return &dueAppointmentReminder{appointment: appointment, effective: effective, occurrence: occurrence}
}

func dueReminderAppointmentIDs(reminders []dueAppointmentReminder) []int64 {
	ids := make([]int64, 0, len(reminders))
	seen := make(map[int64]bool)
	for _, reminder := range reminders {
		appendDistinctID(&ids, seen, reminder.appointment.ID)
	}
	return ids
}

type lockedReminderCandidates struct {
	appointments map[int64]*calModels.Appointment
	recurrences  map[int64]*calModels.RecurrenceRule
	overrides    map[string]*calModels.AppointmentOccurrenceOverride
}

func (s *service) loadLockedReminderCandidates(
	ctx context.Context,
	appointments []*calModels.Appointment,
	occurrences map[int64][]timezone.Date,
) (lockedReminderCandidates, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	locked, err := s.findReminderCandidatesForUpdate(ctx, ids)
	if err != nil {
		return lockedReminderCandidates{}, fmt.Errorf("calendar: lock reminder candidates: %w", err)
	}
	lockedIDs := make([]int64, 0, len(locked))
	byID := make(map[int64]*calModels.Appointment, len(locked))
	for _, appointment := range locked {
		lockedIDs = append(lockedIDs, appointment.ID)
		byID[appointment.ID] = appointment
	}
	rules, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, lockedIDs)
	if err != nil {
		return lockedReminderCandidates{}, fmt.Errorf("calendar: reload reminder recurrences: %w", err)
	}
	rulesByID := make(map[int64]*calModels.RecurrenceRule, len(rules))
	for _, rule := range rules {
		rulesByID[rule.AppointmentID] = rule
	}
	dates := toCalendarDates(distinctReminderOccurrenceDates(lockedIDs, occurrences))
	overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(ctx, lockedIDs, dates)
	if err != nil {
		return lockedReminderCandidates{}, fmt.Errorf("calendar: reload reminder overrides: %w", err)
	}
	overridesByKey := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overridesByKey[appointmentOccurrenceKey(override.AppointmentID, toTimezoneDate(override.OccurrenceDate))] = override
	}
	return lockedReminderCandidates{appointments: byID, recurrences: rulesByID, overrides: overridesByKey}, nil
}

func reminderOccurrencesByAppointment(
	appointments []*calModels.Appointment,
	rules map[int64]*calModels.RecurrenceRule,
	moved map[int64][]*calModels.AppointmentOccurrenceOverride,
	from, to timezone.Date,
) map[int64][]timezone.Date {
	result := make(map[int64][]timezone.Date, len(appointments))
	for _, appointment := range appointments {
		seen := make(map[timezone.Date]bool)
		for _, occurrence := range reminderOccurrences(appointment, rules[appointment.ID], from, to) {
			if !seen[occurrence] {
				seen[occurrence] = true
				result[appointment.ID] = append(result[appointment.ID], occurrence)
			}
		}
		for _, override := range moved[appointment.ID] {
			occurrence := toTimezoneDate(override.OccurrenceDate)
			if !seen[occurrence] {
				seen[occurrence] = true
				result[appointment.ID] = append(result[appointment.ID], occurrence)
			}
		}
	}
	return result
}

func distinctReminderOccurrenceDates(ids []int64, occurrences map[int64][]timezone.Date) []timezone.Date {
	seen := make(map[timezone.Date]bool)
	dates := make([]timezone.Date, 0)
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

func appointmentOccurrenceKey(appointmentID int64, occurrence timezone.Date) string {
	return fmt.Sprintf("%d:%s", appointmentID, occurrence.String())
}

// reminderPushConfigured reports whether the push half of the reminder can run
// at all. Without a notifier there is no channel, and without the consent
// service there is no permission to address anybody — in both cases the scan
// would only ever produce e-mail.
func (s *service) reminderPushConfigured() bool {
	return s.cfg.ReminderNotifier != nil && s.cfg.Preferences != nil
}

// reminderOccurrences lists the occurrence dates of one appointment inside the
// window — the single date of a one-off, or the expanded recurrence.
func reminderOccurrences(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) []timezone.Date {
	if rule == nil {
		if !dateRangesOverlap(toTimezoneDate(appointment.StartDate), toTimezoneDate(appointment.EndDate), from, to) {
			return nil
		}
		return []timezone.Date{toTimezoneDate(appointment.StartDate)}
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
	effective.StartDate = toCalendarDate(occurrence)
	effective.EndDate = toCalendarDate(occurrence.AddDays(span))

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
	clock := timezone.NormalizeWallClock(appointment.StartTime)
	return time.Date(
		midnight.Year(), midnight.Month(), midnight.Day(),
		clock.Hour(), clock.Minute(), 0, 0,
		midnight.Location(),
	)
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

func (s *service) loadReminderBrand(ctx context.Context, tenantID int64) reminderBrand {
	return reminderBrand{
		schoolName:  s.resolveSchoolName(ctx, tenantID),
		logoURL:     s.resolveSchoolLogo(ctx, tenantID),
		motoLogoURL: emailbranding.MotoLogoURL(s.cfg.ParentsURL),
	}
}

func (s *service) loadReminderAudiencePreferences(ctx context.Context, recipients map[int64]guardianRecipients) (reminderAudiencePreferences, error) {
	result := reminderAudiencePreferences{emailUnfiltered: s.cfg.Preferences == nil, emailAccounts: make(map[int64]bool), pushAccounts: make(map[int64]bool)}
	accountIDs := reminderRecipientAccountIDs(recipients)
	if s.cfg.Preferences == nil || len(accountIDs) == 0 {
		return result, nil
	}
	emailAccounts, err := s.cfg.Preferences.FilterNotOptedOut(ctx, notifications.TypeParentAppointmentReminder, accountIDs)
	if err != nil {
		return result, fmt.Errorf("calendar: filter reminder e-mail preferences: %w", err)
	}
	result.emailAccounts = int64BoolSet(emailAccounts)
	if s.reminderPushConfigured() {
		pushAccounts, err := s.cfg.Preferences.FilterOptedIn(ctx, notifications.TypeParentAppointmentReminder, accountIDs)
		if err != nil {
			return result, fmt.Errorf("calendar: filter reminder push preferences: %w", err)
		}
		result.pushAccounts = int64BoolSet(pushAccounts)
	}
	return result, nil
}

func reminderRecipientAccountIDs(recipients map[int64]guardianRecipients) []int64 {
	ids := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, recipientSet := range recipients {
		for _, profile := range recipientSet.profiles {
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
func (s *service) enqueueAppointmentReminder(
	ctx context.Context,
	reminder dueAppointmentReminder,
	recipients guardianRecipients,
	preferences reminderAudiencePreferences,
	brand reminderBrand,
	from, to time.Time,
	deliveries *[]reminderPushDelivery,
) (int, error) {
	if len(recipients.guardianIDs) == 0 {
		return 0, nil
	}
	queued, audience, err := s.enqueueReminderAudience(ctx, reminder, recipients, preferences, brand)
	if err != nil {
		return queued, err
	}
	if err := s.claimReminderPushAudience(ctx, reminder, audience, from, to, deliveries); err != nil {
		return queued, err
	}
	s.logger().Info("appointment reminders queued",
		slog.Int64("appointment_id", reminder.appointment.ID),
		slog.String("occurrence_date", reminder.occurrence.String()),
		slog.Int("recipient_count", len(audience.profilesByAccount)),
	)
	return queued, nil
}

func (s *service) enqueueReminderAudience(
	ctx context.Context,
	reminder dueAppointmentReminder,
	recipients guardianRecipients,
	preferences reminderAudiencePreferences,
	brand reminderBrand,
) (int, reminderPushAudience, error) {
	queued := 0
	audience := reminderPushAudience{
		profilesByAccount: make(map[int64][]int64, len(recipients.guardianIDs)),
		localeByAccount:   make(map[int64]string, len(recipients.guardianIDs)),
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

func uniqueReminderGuardianProfiles(recipients guardianRecipients) []*userModels.GuardianProfile {
	profiles := make([]*userModels.GuardianProfile, 0, len(recipients.guardianIDs))
	seen := make(map[int64]bool, len(recipients.guardianIDs))
	for _, id := range recipients.guardianIDs {
		if profile := recipients.profiles[id]; profile != nil && !seen[id] {
			seen[id] = true
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

func (s *service) enqueueReminderEmail(
	ctx context.Context,
	reminder dueAppointmentReminder,
	recipients guardianRecipients,
	profile *userModels.GuardianProfile,
	preferences reminderAudiencePreferences,
	brand reminderBrand,
) (bool, error) {
	if s.cfg.Outbox == nil || profile.Email == nil || *profile.Email == "" || !preferences.emailAllowed(profile.AccountID) {
		return false, nil
	}
	location := ""
	if reminder.effective.Location != nil {
		location = *reminder.effective.Location
	}
	payload := map[string]any{
		apptPayloadRecipient: *profile.Email, apptPayloadFirstName: profile.FirstName, apptPayloadLastName: profile.LastName,
		apptPayloadTitle: reminder.effective.Title, apptPayloadWhen: appointmentWhenText(reminder.effective), apptPayloadLocation: location,
		apptPayloadSchoolName: brand.schoolName, apptPayloadPortalURL: s.cfg.ParentsURL, apptPayloadLogoURL: brand.logoURL, apptPayloadMotoLogoURL: brand.motoLogoURL,
	}
	addGuardianDeliveryScope(payload, recipients, profile.ID)
	row, err := s.cfg.Outbox.Enqueue(ctx, platformService.EnqueueRequest{
		Kind: platformModels.EmailKindAppointmentReminder, Payload: payload,
		IdempotencyKey:    appointmentReminderKey(reminder.appointment.ID, reminder.appointment.Revision, reminder.occurrence, profile.ID),
		RelatedEntityType: platformModels.EmailRelatedTypeAppointment, RelatedEntityID: reminder.appointment.ID,
	})
	if err != nil {
		return false, fmt.Errorf("calendar: enqueue appointment reminder: %w", err)
	}
	return row.ID != 0, nil
}

func (s *service) claimReminderPushAudience(
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
			dispatched, err := s.dispatchGuardianAccountReminderDevicesLocalized(ctx, appointment, []int64{delivery.accountID}, studentIDs, delivery.locale)
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
			if errors.Is(err, notifications.ErrNoWebPushSubscribers) || errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
				if releaseErr := s.releaseReminderPushDelivery(ctx, delivery); releaseErr != nil {
					errs <- releaseErr
				}
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
func (s *service) prepareReminderPushDispatch(ctx context.Context, delivery reminderPushDelivery) (*calModels.Appointment, []int64, []int64, error) {
	var appointment *calModels.Appointment
	var profileIDs []int64
	var studentIDs []int64
	err := tenant.WithTenantTx(ctx, s.cfg.DB, delivery.appointment.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		current, err := s.findReminderCandidateForUpdate(txCtx, delivery.appointment.ID)
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
		rule, err := s.cfg.RecurrenceRepo.FindByAppointmentID(txCtx, current.ID)
		if err != nil {
			return fmt.Errorf("reload reminder recurrence: %w", err)
		}
		if (rule == nil && current.StartDate != toCalendarDate(delivery.occurrence)) ||
			(rule != nil && !occurrenceExists(current, rule, delivery.occurrence)) {
			return nil
		}
		overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(txCtx, []int64{current.ID}, []calModels.Date{toCalendarDate(delivery.occurrence)})
		if err != nil {
			return fmt.Errorf("reload reminder override: %w", err)
		}
		var override *calModels.AppointmentOccurrenceOverride
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
		if !delivery.stillDue(occurrenceStartInstant(appointmentWithOverride(current, delivery.occurrence, override))) {
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
		recipients, err := s.reachableGuardianRecipients(txCtx, current.ID)
		if err != nil {
			return err
		}
		reachableSet := make(map[int64]struct{}, len(recipients.guardianIDs))
		for _, profileID := range recipients.guardianIDs {
			profile, ok := recipients.profiles[profileID]
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
				claimed, err := s.cfg.RecipientRepo.ClaimReminderPush(txCtx, current.ID, current.Revision, toCalendarDate(delivery.occurrence), profileID)
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
		studentIDs = guardianStudentIDs(recipients, []int64{delivery.accountID})
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
func (s *service) claimReminderPush(ctx context.Context, appointment *calModels.Appointment, occurrence timezone.Date, profileID int64) (bool, error) {
	return s.cfg.RecipientRepo.ClaimReminderPush(ctx, appointment.ID, appointment.Revision, toCalendarDate(occurrence), profileID)
}

func (s *service) releaseReminderPush(ctx context.Context, appointment *calModels.Appointment, occurrence timezone.Date, profileID int64) error {
	cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
	cleanupCtx = tenant.ContextWithoutAfterCommitHooks(cleanupCtx)
	return tenant.WithTenantTx(cleanupCtx, s.cfg.DB, appointment.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.cfg.RecipientRepo.ReleaseReminderPush(txCtx, appointment.ID, appointment.Revision, toCalendarDate(occurrence), profileID)
	})
}

// releaseReminderPushProfiles drops the claims one appointment revision holds
// for these guardians, inside the caller's transaction. Used where a claim is
// superseded rather than abandoned — the caller commits the release together
// with whatever replaces it.
func (s *service) releaseReminderPushProfiles(ctx context.Context, appointment *calModels.Appointment, occurrence timezone.Date, profileIDs []int64) error {
	for _, profileID := range profileIDs {
		if err := s.cfg.RecipientRepo.ReleaseReminderPush(ctx, appointment.ID, appointment.Revision, toCalendarDate(occurrence), profileID); err != nil {
			return fmt.Errorf("calendar: release superseded reminder push delivery: %w", err)
		}
	}
	return nil
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
