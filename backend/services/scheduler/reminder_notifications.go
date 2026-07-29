// Reminder → notification bridge (epic "persönliche Benachrichtigungen"): the
// producer that turns the #1457 reminder list into notifications for the people
// it actually concerns.
//
// Every minute, per tenant, the tick asks who agreed to hear about which kind
// of reminder, computes those people's personal reminder lists in one batch,
// and dispatches one event per person and kind. Nobody is addressed who did not
// switch that kind on in their own profile.
//
// The producer lives in the scheduler rather than in services/notifications
// because it needs both packages, and those two deliberately do not know each
// other (see TestNotificationTypeKeysMatchReminders). It knows nothing about
// channels either — it builds events and calls NotifyBatch, so every channel the
// abstraction grows applies here automatically.
//
// GDPR: reminder rows carry student names, the dispatched notifications never
// do. Title and body only say how many reminders of which kind are due; the deep
// link leads to /reminders, where the names are shown to the authenticated,
// permission-filtered viewer.
//
// Re-fire guard: an in-memory sync.Map keyed by (tenant, account, reminder
// identity), rotated on civil-date rollover — the same shape as the
// instance-overdue tick. A mid-day restart re-fires currently-due reminders
// once; toasts are ephemeral, that cost is acceptable for zero disk state.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// reminderNotificationKey identifies one dispatched reminder occurrence for one
// person. The account is part of the key because the same pickup legitimately
// reaches several people, and each of them must hear about it once.
type reminderNotificationKey struct {
	tenantID  int64
	accountID int64
	reminder  string // type + entity id + due time
}

// The three reads the producer needs beyond its two services. Narrow interfaces
// rather than the full repositories: they say exactly what this tick reads, and
// a test double stays three methods long.
type (
	staffAccountReader interface {
		ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error)
	}
	adminAccountReader interface {
		ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
	}
	dutyReader interface {
		GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
	}
	// consentFilter is the one method this producer uses of the consent
	// service; notifications.PreferenceService satisfies it.
	consentFilter interface {
		FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
	}
)

// ReminderNotificationDeps bundles what the tick needs. All of it or none: a
// partially wired producer would resolve a narrower audience for a reason
// nobody intended, so an incomplete set disables the task instead.
type ReminderNotificationDeps struct {
	Computer     reminders.BatchComputer
	Notifier     notifications.BatchNotifier
	Preferences  consentFilter
	Staff        staffAccountReader
	Accounts     adminAccountReader
	WorkSessions dutyReader
}

func (d ReminderNotificationDeps) complete() bool {
	return d.Computer != nil && d.Notifier != nil && d.Preferences != nil &&
		d.Staff != nil && d.Accounts != nil && d.WorkSessions != nil
}

// personalReminderTypes are the notification types this producer can send, in
// the order the consent lookup runs.
var personalReminderTypes = []string{
	notifications.TypePickupUpcoming,
	notifications.TypePickupOverdue,
	notifications.TypeActivityStart,
	notifications.TypeActivityOverdue,
	notifications.TypeMyActivityStarting,
}

// SetReminderNotificationDeps wires the reminder-notification tick.
func (s *Scheduler) SetReminderNotificationDeps(deps ReminderNotificationDeps) {
	s.reminderNotifications = deps
}

// scheduleReminderNotificationTask registers the minute tick when the producer
// is completely wired.
func (s *Scheduler) scheduleReminderNotificationTask() {
	if !s.reminderNotifications.complete() {
		s.getLogger().Info("reminder notification tick not configured (incomplete dependencies)")
		return
	}
	s.registerTask("reminder-notifications", "1m-poll", s.runReminderNotificationTaskPolling)
}

func (s *Scheduler) runReminderNotificationTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in reminder notification task",
		"reminder notification tick using minute-polling",
		s.checkAndRunReminderNotifications)
}

// checkAndRunReminderNotifications rotates the day cache, then iterates
// active tenants. The per-tenant work lives in
// runReminderNotificationsForTenant so unit tests can drive it directly.
func (s *Scheduler) checkAndRunReminderNotifications(task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	s.rotateReminderNotificationCacheIfNewDay(time.Now())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "reminder-notifications", func(tenantCtx context.Context, tenantID int64) error {
		// Cheap gate first: skip the whole resolution while the notification
		// feature flag is off for this tenant. NotifyBatch would also refuse
		// (ErrDisabled), but there is no point resolving anybody. That mirroring
		// only holds while both sides read the flag the same way, hence
		// notificationDispatchEnabled instead of the scheduler's
		// literal-default helpers.
		if !s.notificationDispatchEnabled(tenantCtx, tenantID) {
			return nil
		}
		s.runReminderNotificationsForTenant(tenantCtx, tenantID, time.Now())
		return nil
	})
}

// notificationDispatchEnabled answers the notification feature flag exactly the
// way the notification router does (services/notifications/service.go): the
// tenant override when one exists, otherwise the value registered in the
// settings registry. ResolveBool already returns the registry default, so this
// gate carries no default of its own. A literal here would be a second source of
// truth beside the registry: a registry default of "on" against a literal "off"
// silenced the whole tick for every school without an explicit override.
//
// Fails closed when the flag cannot be read and when no settings service is
// wired: without one the router cannot dispatch either.
func (s *Scheduler) notificationDispatchEnabled(ctx context.Context, tenantID int64) bool {
	if s.settings == nil {
		return false
	}
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyNotificationsDispatchEnabled)
	if err != nil {
		s.getLogger().Warn("reminder notification tick: notification feature flag unreadable, skipping tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return enabled
}

// reminderRecipient is one person the tick may address, with the kinds of
// reminder they agreed to hear about.
type reminderRecipient struct {
	staffID   int64
	accountID int64
	isAdmin   bool
	consent   map[string]struct{}
}

func (r reminderRecipient) resultKey() int64 {
	if r.staffID > 0 {
		return r.staffID
	}
	// Account and staff IDs come from different sequences. Keeping staff-less
	// admins in the negative half makes their opaque batch keys collision-free.
	return -r.accountID
}

// runReminderNotificationsForTenant resolves the audience, computes everybody's
// personal list in one batch and dispatches what is new. `now` is injected for
// deterministic tests.
func (s *Scheduler) runReminderNotificationsForTenant(ctx context.Context, tenantID int64, now time.Time) {
	deps := s.reminderNotifications

	recipients, err := s.resolveReminderRecipients(ctx)
	if err != nil {
		s.getLogger().Warn("reminder notification tick: audience resolution failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(recipients) == 0 {
		return
	}

	scopes := make([]reminders.BatchScope, 0, len(recipients))
	for _, r := range recipients {
		_, includeAssignedStart := r.consent[notifications.TypeMyActivityStarting]
		scopes = append(scopes, reminders.BatchScope{
			Scope:                        reminders.Scope{IsAdmin: r.isAdmin, StaffID: r.staffID},
			ResultKey:                    r.resultKey(),
			IncludeAssignedActivityStart: includeAssignedStart,
		})
	}

	results, err := deps.Computer.ComputeBatch(ctx, scopes)
	if err != nil {
		s.getLogger().Warn("reminder notification tick: compute failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return
	}

	events, freshKeys := buildPersonalReminderEvents(tenantID, recipients, results, func(key reminderNotificationKey) bool {
		_, seen := s.reminderNotified.Load(key)
		return seen
	})
	if len(events) == 0 {
		return
	}

	if err := deps.Notifier.NotifyBatch(ctx, events); err != nil {
		// A closed delivery window and a flipped feature flag are ordinary
		// answers, not failures: quiet hours would otherwise produce one warning
		// per tenant per minute all evening. Either way the occurrences stay
		// unmarked and fire on the next tick once dispatch is possible again.
		if !errors.Is(err, notifications.ErrDisabled) && !errors.Is(err, notifications.ErrOutsideActiveWindow) {
			s.getLogger().Warn("reminder notification tick: notify failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	// NotifyBatch registered delivery first, so this hook runs after delivery
	// and only after the surrounding tenant transaction commits. A rollback or
	// commit failure drops both hooks and leaves the occurrences eligible for
	// the next tick.
	tenant.RegisterAfterCommit(ctx, func() {
		for _, key := range freshKeys {
			s.reminderNotified.Store(key, now)
		}
		s.getLogger().Info("personal reminder notifications dispatched",
			slog.Int64("tenant_id", tenantID),
			slog.Int("recipient_count", len(recipients)),
			slog.Int("event_count", len(events)),
		)
	})
}

// resolveReminderRecipients builds the audience: every staff member with a
// login plus every effective admin account, narrowed by their own consent and,
// when the school asks for it, by whether staff-backed recipients are on duty.
//
// The order is the one the consent service prescribes — a relation first, then
// consent narrowing it. Consent is never the source of the audience, it is the
// filter on top; that is also why an empty consent result ends the tick before
// any reminder is computed.
func (s *Scheduler) resolveReminderRecipients(ctx context.Context) ([]reminderRecipient, error) {
	deps := s.reminderNotifications

	accountsByStaff, err := deps.Staff.ListAllStaffAccountIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list staff accounts: %w", err)
	}

	adminIDs, err := deps.Accounts.ListEffectiveAdminAccountIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list admin accounts: %w", err)
	}

	staffByAccount := make(map[int64]int64, len(accountsByStaff))
	accountSet := make(map[int64]struct{}, len(accountsByStaff)+len(adminIDs))
	for staffID, accountID := range accountsByStaff {
		if accountID <= 0 {
			continue
		}
		// An account should map to one staff row. Choosing the smaller ID keeps
		// resolution deterministic if inconsistent legacy data says otherwise.
		current, exists := staffByAccount[accountID]
		if !exists || staffID < current {
			staffByAccount[accountID] = staffID
		}
		accountSet[accountID] = struct{}{}
	}
	admins := make(map[int64]struct{}, len(adminIDs))
	for _, accountID := range adminIDs {
		if accountID <= 0 {
			continue
		}
		admins[accountID] = struct{}{}
		accountSet[accountID] = struct{}{}
	}
	if len(accountSet) == 0 {
		return nil, nil
	}

	accountIDs := make([]int64, 0, len(accountSet))
	for accountID := range accountSet {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })

	// FilterOptedIn also applies each type's school-wide gate (the matching
	// reminders.* setting), so a reminder kind the school switched off reaches
	// nobody without this tick having to know which setting that is.
	consentByAccount := make(map[int64]map[string]struct{}, len(accountIDs))
	for _, notificationType := range personalReminderTypes {
		optedIn, ferr := deps.Preferences.FilterOptedIn(ctx, notificationType, accountIDs)
		if ferr != nil {
			return nil, fmt.Errorf("filter opted-in accounts for %s: %w", notificationType, ferr)
		}
		for _, accountID := range optedIn {
			if consentByAccount[accountID] == nil {
				consentByAccount[accountID] = make(map[string]struct{}, len(personalReminderTypes))
			}
			consentByAccount[accountID][notificationType] = struct{}{}
		}
	}
	if len(consentByAccount) == 0 {
		return nil, nil
	}

	onDuty, err := s.resolveOnDutyStaff(ctx)
	if err != nil {
		return nil, err
	}

	recipients := make([]reminderRecipient, 0, len(consentByAccount))
	for _, accountID := range accountIDs {
		consent := consentByAccount[accountID]
		if len(consent) == 0 {
			continue
		}
		staffID := staffByAccount[accountID]
		_, isAdmin := admins[accountID]
		if staffID <= 0 && !isAdmin {
			continue
		}
		if onDuty != nil {
			if staffID <= 0 {
				continue
			}
			if _, working := onDuty[staffID]; !working {
				continue
			}
		}
		recipients = append(recipients, reminderRecipient{
			staffID:   staffID,
			accountID: accountID,
			isAdmin:   isAdmin,
			consent:   consent,
		})
	}
	// Map iteration is random; a stable order keeps the dispatched batch (and
	// therefore the logs and the tests) reproducible.
	sort.Slice(recipients, func(i, j int) bool { return recipients[i].accountID < recipients[j].accountID })

	return recipients, nil
}

// resolveOnDutyStaff answers who may be addressed under
// notifications.on_duty_only. A nil map means "no restriction"; an empty,
// non-nil map means nobody is currently on duty.
func (s *Scheduler) resolveOnDutyStaff(ctx context.Context) (map[int64]struct{}, error) {
	if !s.resolveBoolSetting(ctx, configModel.KeyNotificationsOnDutyOnly, "", true) {
		return nil, nil
	}

	presence, err := s.reminderNotifications.WorkSessions.GetTodayPresenceMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("load staff presence: %w", err)
	}
	onDuty := make(map[int64]struct{}, len(presence))
	for staffID, status := range presence {
		// Matched positively against the two real statuses: the map also carries
		// a synthetic "checked out" value, and an unknown future status must not
		// silently count as working.
		if status == activeModel.WorkSessionStatusPresent || status == activeModel.WorkSessionStatusHomeOffice {
			onDuty[staffID] = struct{}{}
		}
	}
	return onDuty, nil
}

// buildPersonalReminderEvents turns the batch results into one event per person
// and notification type, skipping everything the person did not agree to and
// everything they already heard about today.
//
// Pure apart from the injected seen-check, so the whole mapping — consent,
// classification, aggregation — is testable without a scheduler.
func buildPersonalReminderEvents(
	tenantID int64,
	recipients []reminderRecipient,
	results map[int64]*reminders.Result,
	seen func(reminderNotificationKey) bool,
) ([]notifications.Event, []reminderNotificationKey) {
	var (
		events    []notifications.Event
		freshKeys []reminderNotificationKey
	)

	for _, r := range recipients {
		result := results[r.resultKey()]
		if result == nil || !result.Enabled || len(result.Reminders) == 0 {
			continue
		}

		counts := make(map[string]int, len(personalReminderTypes))
		order := make([]string, 0, len(personalReminderTypes))

		for _, reminder := range result.Reminders {
			notificationType, wanted := notificationTypeFor(reminder, result.AssignedActivityInstanceIDs, r.consent)
			if !wanted {
				continue
			}
			key := reminderNotificationKey{
				tenantID:  tenantID,
				accountID: r.accountID,
				reminder:  reminderIdentity(reminder),
			}
			if seen(key) {
				continue
			}
			if counts[notificationType] == 0 {
				order = append(order, notificationType)
			}
			counts[notificationType]++
			freshKeys = append(freshKeys, key)
		}

		for _, notificationType := range order {
			events = append(events, buildPersonalReminderEvent(
				tenantID, r.accountID, notificationType, counts[notificationType]))
		}
	}

	return events, freshKeys
}

// notificationTypeFor decides which notification type a reminder belongs to for
// one person, and whether that person wants it.
//
// The only non-obvious case is an activity the person is personally planned on.
// It is reported as "Ihr nächster Einsatz" even when they also happen to
// supervise the room, because that is what the two catalogue descriptions
// promise: the room type is about "a room you are watching", the assignment type
// about your own slot "auch wenn Sie gerade keine Aufsicht führen". Each switch
// therefore means exactly what it says, and the same activity never arrives
// twice.
func notificationTypeFor(
	reminder reminders.Reminder,
	assigned map[string]struct{},
	consent map[string]struct{},
) (string, bool) {
	notificationType := reminder.Type
	if reminder.Type == reminders.TypeActivityStart && reminder.ActivityInstanceID != nil {
		if _, mine := assigned[*reminder.ActivityInstanceID]; mine {
			notificationType = notifications.TypeMyActivityStarting
		}
	}
	if _, agreed := consent[notificationType]; !agreed {
		return "", false
	}
	return notificationType, true
}

// reminderIdentity builds the dedup identity of one reminder occurrence:
// type + entity + due time. A rescheduled pickup (new DueTime) is a new
// occurrence on purpose — the plan changed, staff should hear about it again.
func reminderIdentity(r reminders.Reminder) string {
	entity := ""
	switch {
	case r.StudentID != nil:
		entity = "s" + *r.StudentID
	case r.ActivityInstanceID != nil:
		entity = "a" + *r.ActivityInstanceID
	}
	return r.Type + "|" + entity + "|" + r.DueTime
}

// personalReminderCopy is the display-safe German wording per notification
// type. Counts only: these strings end up on a lock screen, and the reminder
// rows they summarise carry child names.
type personalReminderCopy struct {
	title    string
	singular string
	plural   string // one %d
	urgent   bool
}

var reminderCopyByType = map[string]personalReminderCopy{
	notifications.TypePickupUpcoming: {
		title:    "Abholung steht an",
		singular: "Ein Kind aus Ihrer Aufsicht wird bald abgeholt.",
		plural:   "%d Kinder aus Ihrer Aufsicht werden bald abgeholt.",
	},
	notifications.TypePickupOverdue: {
		title:    "Abholung überfällig",
		singular: "Ein Kind ist noch da, obwohl die Abholzeit vorbei ist.",
		plural:   "%d Kinder sind noch da, obwohl ihre Abholzeit vorbei ist.",
		urgent:   true,
	},
	notifications.TypeActivityStart: {
		title:    "Aktivität beginnt",
		singular: "In einem Raum Ihrer Aufsicht beginnt gleich eine Aktivität.",
		plural:   "In Ihren Räumen beginnen gleich %d Aktivitäten.",
	},
	notifications.TypeActivityOverdue: {
		title:    "Aktivität nicht gestartet",
		singular: "Eine geplante Aktivität ist überfällig und wurde noch nicht gestartet.",
		plural:   "%d geplante Aktivitäten sind überfällig und wurden noch nicht gestartet.",
		urgent:   true,
	},
	notifications.TypeMyActivityStarting: {
		title:    "Ihr Einsatz beginnt",
		singular: "Einer Ihrer geplanten Einsätze beginnt gleich.",
		plural:   "%d Ihrer geplanten Einsätze beginnen gleich.",
	},
}

// buildPersonalReminderEvent renders one person's due count of one kind.
func buildPersonalReminderEvent(tenantID, accountID int64, notificationType string, count int) notifications.Event {
	copyFor, known := reminderCopyByType[notificationType]
	if !known {
		// Unreachable while personalReminderTypes and the copy table agree,
		// which a test pins. A generic wording still beats an empty title,
		// which validation would reject and which would drop the whole batch.
		copyFor = personalReminderCopy{
			title:    "Erinnerung",
			singular: "Eine Erinnerung ist fällig.",
			plural:   "%d Erinnerungen sind fällig.",
		}
	}

	body := copyFor.singular
	if count > 1 {
		body = fmt.Sprintf(copyFor.plural, count)
	}

	priority := notifications.PriorityNormal
	if copyFor.urgent {
		priority = notifications.PriorityHigh
	}

	return notifications.Event{
		Type: notificationType,
		Audience: notifications.Audience{
			TenantID:        tenantID,
			Scope:           notifications.ScopeStaff,
			StaffAccountIDs: []int64{accountID},
		},
		Priority: priority,
		Title:    copyFor.title,
		Body:     body,
		DeepLink: "/reminders",
		Data:     map[string]string{"count": strconv.Itoa(count)},
	}
}

// rotateReminderNotificationCacheIfNewDay clears the re-fire guard on civil-
// date rollover, mirroring rotateOverdueCacheIfNewDay. Reminder identities
// embed only wall-clock due times, so yesterday's keys would otherwise
// suppress today's identical schedule.
func (s *Scheduler) rotateReminderNotificationCacheIfNewDay(now time.Time) {
	today := timezone.DateFromTime(now)
	s.reminderNotifiedDayMu.Lock()
	defer s.reminderNotifiedDayMu.Unlock()
	if s.reminderNotifiedDay != today {
		s.reminderNotified.Range(func(k, _ any) bool {
			s.reminderNotified.Delete(k)
			return true
		})
		s.reminderNotifiedDay = today
	}
}
